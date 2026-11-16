// Package gui is LazyTrackIt's Wails v3 desktop front end: a tray icon plus
// a window whose HTML/JS frontend calls the exported methods of Service.
// All business logic lives in internal/service; this package only adapts it
// to JSON-friendly types and to the desktop (events, tray, autostart).
package gui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
	"github.com/ahmed-habbachi/lazytrackit/internal/service"
	"github.com/ahmed-habbachi/lazytrackit/internal/version"
)

// Events emitted to the frontend.
const (
	EventLoginPrompt = "login:prompt" // payload: provider.LoginPrompt
	EventAttendance  = "attendance"   // payload: *AttendanceSession (nil = checked out)
	EventAutostart   = "autostart"    // payload: bool
	EventUpdate      = "update"       // payload: UpdateStatus
	EventNavigate    = "navigate"     // payload: view name, e.g. "attendance"
)

// Error texts the frontend recognizes instead of showing them:
//   - StaleMessage: the call raced a provider switch; ignore it (see
//     service.ErrStale).
//   - ReconnectMessage: the login expired or the profile isn't loaded, so
//     the frontend runs Start again, which logs in afresh.
const (
	StaleMessage     = "stale"
	ReconnectMessage = "reconnect"
)

// Host is what Service needs from the desktop shell. The Wails app
// implements it; tests substitute a fake.
type Host interface {
	Emit(name string, data any)
	OpenURL(url string)
	AutostartEnabled() (bool, error)
	SetAutostart(enabled bool) error
}

// Service is bound to the frontend: every exported method is callable from
// JavaScript. It is safe for concurrent use, since the frontend may have
// several calls in flight at once.
type Service struct {
	host         Host
	session      *service.Session // nil when SetupMessage is set
	setupMessage string
	// updatesDisabled mirrors the config's check_for_updates: false.
	updatesDisabled bool
	// reminderAt is the config's check_in_reminder (time of day);
	// remindersOn is false when it's "off".
	reminderAt  time.Duration
	remindersOn bool
	// reminder, when running, is told about every attendance update.
	reminder *reminder

	mu sync.Mutex
	// entries caches the most recently requested week, by ID, so Delete can
	// send the provider the full entry. weekSeq orders Week requests: a
	// response only replaces the cache if no newer request (or provider
	// switch) has started since, so it always matches what the UI shows.
	entries map[string]provider.TimeEntry
	weekSeq uint64

	// openSession is the check-in state. attSeq orders writes to it: user
	// actions (check in/out) and provider switches bump it, and a status
	// fetch only applies its result if nothing bumped it while it ran, so a
	// slow fetch can't resurrect a session the user just closed.
	openSession *provider.AttendanceSession
	attSeq      uint64

	updateNotice string
	updateReady  string // version installed and waiting for a restart

	// restart starts the installed update and quits; nil in tests and
	// when the executable's path couldn't be found.
	restart func() error

	// Hooks for the tray, set once at startup.
	onAttendance func(open *provider.AttendanceSession, enabled bool)
	// onUpdateReady offers "Restart to update" in the tray (nil: no tray).
	onUpdateReady func(version string)
	onAutostart   func(enabled bool)
}

// NewService builds a Service over session.
func NewService(host Host, session *service.Session) *Service {
	return &Service{host: host, session: session, entries: map[string]provider.TimeEntry{}}
}

// NewSetupService builds a Service for when there's no usable config yet:
// the frontend only shows message (e.g. where a starter config was written).
func NewSetupService(host Host, message string) *Service {
	return &Service{host: host, setupMessage: message, entries: map[string]provider.TimeEntry{}}
}

// publicError maps errors the frontend handles itself (see StaleMessage
// and ReconnectMessage); others pass through to be shown.
func publicError(err error) error {
	switch {
	case errors.Is(err, service.ErrStale):
		return errors.New(StaleMessage)
	case errors.Is(err, provider.ErrReauthRequired), errors.Is(err, service.ErrNoProfile):
		return errors.New(ReconnectMessage)
	}
	return err
}

var errSetup = errors.New("LazyTrackIt isn't configured yet")

func (s *Service) scope() (service.Scope, error) {
	if s.session == nil {
		return service.Scope{}, errSetup
	}
	return s.session.Current(), nil
}

// State returns the app-level status.
func (s *Service) State() State {
	st := State{Version: version.Version, SetupMessage: s.setupMessage}
	if on, err := s.host.AutostartEnabled(); err == nil {
		st.Autostart = on
	}
	s.mu.Lock()
	st.Update = UpdateStatus{Notice: s.updateNotice, Ready: s.updateReady, CanRestart: s.restart != nil}
	s.mu.Unlock()
	if s.session == nil {
		return st
	}

	sc := s.session.Current()
	st.Providers = s.session.ProviderNames()
	st.Active = sc.ProviderName()
	st.NeedsLogin = sc.NeedsLogin()
	member, ok := sc.Member()
	st.HasProfile = ok
	st.Member = member.DisplayName()
	st.Attendance = sc.Attendance() != nil
	return st
}

// Start logs in to the active provider if needed, including again if the
// saved login has expired (emitting EventLoginPrompt and opening the browser
// when the user has to approve something), then loads the member and
// projects.
func (s *Service) Start(ctx context.Context) ([]Project, error) {
	sc, err := s.scope()
	if err != nil {
		return nil, err
	}
	sc, err = sc.Connect(ctx, func(p provider.LoginPrompt) {
		s.host.Emit(EventLoginPrompt, p)
		if safeLoginURL(p.URL) {
			s.host.OpenURL(p.URL)
		}
	})
	if err != nil {
		// Never answer Start itself with ReconnectMessage (the frontend
		// would just call Start again): show what went wrong instead.
		if errors.Is(err, service.ErrStale) {
			return nil, errors.New(StaleMessage)
		}
		return nil, err
	}
	go s.refreshAttendance(sc)
	return toProjects(sc.Projects()), nil
}

// warmUp loads what the tray shows (the check-in state) without any user
// interaction, for a launch that starts hidden in the tray: the frontend
// only calls Start once the window is opened. It never starts an
// interactive login.
func (s *Service) warmUp() {
	sc, err := s.scope()
	if err != nil || sc.NeedsLogin() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), service.RequestTimeout)
	defer cancel()
	if sc, err = sc.LoadProfile(ctx); err == nil {
		s.refreshAttendance(sc)
	}
}

// safeLoginURL reports whether a provider-supplied login URL may be opened
// in the browser: only web URLs, never file: or custom-scheme ones a
// misbehaving provider could use to launch other programs.
func safeLoginURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

// ProviderType is a backend type that can be added (see
// service.ProviderTypes), for the "add provider" form.
type ProviderType struct {
	ID              string   `json:"id"`
	Label           string   `json:"label"`
	Fields          []string `json:"fields"`
	BaseURLOptional bool     `json:"baseURLOptional"`
}

// ProviderTypes lists the types a provider can be added for.
func (s *Service) ProviderTypes() []ProviderType {
	out := make([]ProviderType, len(service.ProviderTypes))
	for i, t := range service.ProviderTypes {
		out[i] = ProviderType{ID: t.ID, Label: t.Label, Fields: t.Fields, BaseURLOptional: t.BaseURLOptional}
	}
	return out
}

// NewProviderDraft returns the defaults for a new provider of type typ.
func (s *Service) NewProviderDraft(typ string) service.ProviderDraft {
	return service.NewProviderDraft(typ)
}

// AddProvider saves a new provider to config.yaml and returns its name. The
// frontend then switches to it (SwitchProvider + Start).
func (s *Service) AddProvider(draft service.ProviderDraft) (string, error) {
	if s.session == nil {
		return "", errSetup
	}
	return s.session.AddProvider(draft)
}

// BackendVersion returns the active provider's server version, or "" if
// its type can't report one.
func (s *Service) BackendVersion(ctx context.Context) (string, error) {
	sc, err := s.scope()
	if err != nil {
		return "", err
	}
	v, err := sc.BackendVersion(ctx)
	if err != nil {
		return "", publicError(err)
	}
	return v, nil
}

// ProviderSummary is one row of the Settings dialog's provider list.
type ProviderSummary struct {
	Name   string `json:"name"`
	Type   string `json:"type"` // the type's label, e.g. "Toggl Track"
	Active bool   `json:"active"`
}

// ProviderList lists the configured providers with their types.
func (s *Service) ProviderList() ([]ProviderSummary, error) {
	if s.session == nil {
		return nil, errSetup
	}
	active := s.session.ActiveProviderName()
	var out []ProviderSummary
	for _, name := range s.session.ProviderNames() {
		row := ProviderSummary{Name: name, Active: name == active}
		if d, err := service.ProviderDraftFor(name); err == nil {
			row.Type = d.Type
			if t, ok := service.LookupProviderType(d.Type); ok {
				row.Type = t.Label
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// ProviderDraft returns provider name's settings, to edit them.
func (s *Service) ProviderDraft(name string) (service.ProviderDraft, error) {
	if s.session == nil {
		return service.ProviderDraft{}, errSetup
	}
	return service.ProviderDraftFor(name)
}

// UpdateProvider saves new settings for an existing provider. It reports
// whether that's the active provider; if so the frontend reloads (Start).
func (s *Service) UpdateProvider(draft service.ProviderDraft) (bool, error) {
	if s.session == nil {
		return false, errSetup
	}
	active, err := s.session.UpdateProvider(draft)
	if err != nil {
		return false, err
	}
	if active {
		s.resetProviderState()
	}
	return active, nil
}

// RemoveProvider deletes a provider. If it was the active one, it returns
// the provider now active and the frontend reloads (Start).
func (s *Service) RemoveProvider(name string) (string, error) {
	if s.session == nil {
		return "", errSetup
	}
	next, err := s.session.RemoveProvider(name)
	if err != nil {
		return "", err
	}
	if next != "" {
		s.resetProviderState()
	}
	return next, nil
}

// SwitchProvider makes name the active provider. The frontend calls Start
// again afterwards.
func (s *Service) SwitchProvider(name string) error {
	if s.session == nil {
		return errSetup
	}
	if err := s.session.SwitchProvider(name); err != nil {
		return err
	}
	s.resetProviderState()
	return nil
}

// resetProviderState drops what was cached for the previous active
// provider (or the previous settings of this one).
func (s *Service) resetProviderState() {
	s.mu.Lock()
	s.entries = map[string]provider.TimeEntry{}
	s.weekSeq++
	s.attSeq++
	s.openSession = nil
	s.mu.Unlock()
	s.publishAttendance(nil, s.session.Current().Attendance() != nil)
}

// Week returns the entries of the week containing day ("YYYY-MM-DD"; ""
// means the current week).
func (s *Service) Week(ctx context.Context, day string) (Week, error) {
	sc, err := s.scope()
	if err != nil {
		return Week{}, err
	}
	now := time.Now()
	weekStart, err := parseWeekStart(day, now)
	if err != nil {
		return Week{}, err
	}

	s.mu.Lock()
	s.weekSeq++
	seq := s.weekSeq
	s.mu.Unlock()

	entries, err := sc.ListEntries(ctx, weekStart)
	if err != nil {
		return Week{}, publicError(err)
	}

	byID := make(map[string]provider.TimeEntry, len(entries))
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
		out = append(out, toEntry(e))
	}
	s.mu.Lock()
	if seq == s.weekSeq {
		s.entries = byID
	}
	s.mu.Unlock()

	return Week{
		Start:           weekStart.Format(dateLayout),
		End:             service.WeekEnd(weekStart).Format(dateLayout),
		Today:           now.Format(dateLayout),
		Entries:         out,
		LoggedMinutes:   int(service.TotalDuration(entries).Minutes()),
		ExpectedMinutes: int(service.ExpectedWeekDuration(weekStart, now).Minutes()),
	}, nil
}

// NewDraft returns the defaults for a new entry in the week containing day.
func (s *Service) NewDraft(day string) (Draft, error) {
	if s.session == nil {
		return Draft{}, errSetup
	}
	now := time.Now()
	weekStart, err := parseWeekStart(day, now)
	if err != nil {
		return Draft{}, err
	}
	s.mu.Lock()
	entries := make([]provider.TimeEntry, 0, len(s.entries))
	for _, e := range s.entries {
		entries = append(entries, e)
	}
	s.mu.Unlock()
	return toDraft(service.NewEntryDraft(weekStart, now, s.session.LastDefaults(), entries)), nil
}

// Tags returns the tags usable on projectID.
func (s *Service) Tags(ctx context.Context, projectID string) ([]Tag, error) {
	sc, err := s.scope()
	if err != nil {
		return nil, err
	}
	tags, err := sc.ListTags(ctx, projectID)
	if err != nil {
		return nil, publicError(err)
	}
	return toTags(tags), nil
}

// Save validates and creates (empty ID) or updates an entry.
func (s *Service) Save(ctx context.Context, d Draft) error {
	sc, err := s.scope()
	if err != nil {
		return err
	}
	draft, err := d.toService()
	if err != nil {
		return err
	}
	if draft.IsEdit() {
		if e, ok := s.cachedEntry(draft.ID); ok && !toEntry(e).Editable {
			return errors.New("this entry can't be edited here (it's locked, or spans midnight)")
		}
	}
	_, err = sc.SaveDraft(ctx, draft, nil)
	return publicError(err)
}

// Delete removes the entry with id from the last listed week.
func (s *Service) Delete(ctx context.Context, id string) error {
	sc, err := s.scope()
	if err != nil {
		return err
	}
	e, ok := s.cachedEntry(id)
	if !ok {
		return fmt.Errorf("entry %s is no longer listed; refresh and try again", id)
	}
	if e.Locked {
		return errors.New("this entry is locked and cannot be deleted")
	}
	return publicError(sc.DeleteEntry(ctx, e))
}

func (s *Service) cachedEntry(id string) (provider.TimeEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	return e, ok
}

// AttendanceStatus returns the open check-in session, or nil if checked out.
func (s *Service) AttendanceStatus(ctx context.Context) (*AttendanceSession, error) {
	sc, err := s.scope()
	if err != nil {
		return nil, err
	}
	open, err := s.fetchAttendance(ctx, sc)
	if err != nil {
		return nil, publicError(err)
	}
	return toSession(open), nil
}

// CheckInOptions returns the work locations to offer.
func (s *Service) CheckInOptions(ctx context.Context) (CheckInOptions, error) {
	sc, err := s.scope()
	if err != nil {
		return CheckInOptions{}, err
	}
	opts, err := sc.CheckInOptions(ctx)
	if err != nil {
		return CheckInOptions{}, publicError(err)
	}
	return toCheckInOptions(opts), nil
}

// CheckIn opens a session at locationID.
func (s *Service) CheckIn(ctx context.Context, locationID, note string) (*AttendanceSession, error) {
	sc, err := s.scope()
	if err != nil {
		return nil, err
	}
	s.bumpAttendance()
	sess, err := sc.CheckIn(ctx, locationID, note)
	if err != nil {
		return nil, publicError(err)
	}
	s.applyAttendance(&sess, true)
	return toSession(&sess), nil
}

// CheckOut closes the open session.
func (s *Service) CheckOut(ctx context.Context, note string) error {
	sc, err := s.scope()
	if err != nil {
		return err
	}
	s.mu.Lock()
	open := s.openSession
	s.mu.Unlock()
	if open == nil {
		return errors.New("not checked in")
	}
	s.bumpAttendance()
	if _, err := sc.CheckOut(ctx, *open, note); err != nil {
		return publicError(err)
	}
	s.applyAttendance(nil, true)
	return nil
}

// SetAutostart turns launching at login on or off, and tells the tray and
// window so both checkboxes agree whichever one was used.
func (s *Service) SetAutostart(enabled bool) error {
	if err := s.host.SetAutostart(enabled); err != nil {
		return err
	}
	s.mu.Lock()
	hook := s.onAutostart
	s.mu.Unlock()
	if hook != nil {
		hook(enabled)
	}
	s.host.Emit(EventAutostart, enabled)
	return nil
}

// refreshAttendance loads the check-in state in the background (e.g. so the
// tray shows it even before the window is opened).
func (s *Service) refreshAttendance(sc service.Scope) {
	ctx, cancel := context.WithTimeout(context.Background(), service.RequestTimeout)
	defer cancel()
	_, _ = s.fetchAttendance(ctx, sc)
}

// fetchAttendance reads the check-in state and applies it, unless a check
// in/out or provider switch happened while it was in flight (see attSeq).
func (s *Service) fetchAttendance(ctx context.Context, sc service.Scope) (*provider.AttendanceSession, error) {
	if sc.Attendance() == nil {
		return nil, service.ErrAttendanceDisabled
	}
	s.mu.Lock()
	seq := s.attSeq
	s.mu.Unlock()

	open, err := sc.AttendanceStatus(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	current := seq == s.attSeq
	if current {
		s.openSession = open
	}
	s.mu.Unlock()
	if current {
		s.publishAttendance(open, true)
	}
	return open, nil
}

// bumpAttendance invalidates status fetches in flight before a user action
// changes the check-in state.
func (s *Service) bumpAttendance() {
	s.mu.Lock()
	s.attSeq++
	s.mu.Unlock()
}

// applyAttendance records the outcome of a user action (always current).
func (s *Service) applyAttendance(open *provider.AttendanceSession, enabled bool) {
	s.mu.Lock()
	s.attSeq++
	s.openSession = open
	s.mu.Unlock()
	s.publishAttendance(open, enabled)
}

// publishAttendance tells the tray and frontend about the check-in state.
// enabled is false when the active provider has no check-in/out, so the
// tray hides it. It holds no lock while calling out; ordering is already
// settled by attSeq before it's called.
func (s *Service) publishAttendance(open *provider.AttendanceSession, enabled bool) {
	s.mu.Lock()
	hook := s.onAttendance
	rem := s.reminder
	s.mu.Unlock()
	if hook != nil {
		hook(open, enabled)
	}
	if rem != nil {
		rem.observe(open)
	}
	s.host.Emit(EventAttendance, toSession(open))
}

// setUpdateNotice records a notice about self-update and tells the frontend.
func (s *Service) setUpdateNotice(notice string) {
	s.mu.Lock()
	s.updateNotice = notice
	st := UpdateStatus{Notice: notice, Ready: s.updateReady}
	s.mu.Unlock()
	s.host.Emit(EventUpdate, st)
}

// setUpdateReady records that version is installed and runs after a
// restart, and tells the frontend (which offers "Restart now").
func (s *Service) setUpdateReady(version string) {
	s.mu.Lock()
	s.updateReady = version
	s.updateNotice = fmt.Sprintf("LazyTrackIt %s is installed. Restart to start using it.", version)
	st := UpdateStatus{Notice: s.updateNotice, Ready: version, CanRestart: s.restart != nil}
	hook := s.onUpdateReady
	s.mu.Unlock()
	s.host.Emit(EventUpdate, st)
	if hook != nil && st.CanRestart {
		hook(version)
	}
}

// RestartToUpdate starts the installed update and quits this instance.
func (s *Service) RestartToUpdate() error {
	s.mu.Lock()
	ready, restart := s.updateReady, s.restart
	s.mu.Unlock()
	if ready == "" {
		return errors.New("no update is waiting to be installed")
	}
	if restart == nil {
		return errors.New("LazyTrackIt can't restart itself here; quit and start it again")
	}
	return restart()
}

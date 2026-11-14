package gui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
	"github.com/ahmed-habbachi/lazytrackit/internal/service"
)

type fakeHost struct {
	mu        sync.Mutex
	events    map[string][]any
	opened    []string
	autostart bool
}

func newFakeHost() *fakeHost { return &fakeHost{events: map[string][]any{}} }

func (h *fakeHost) Emit(name string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events[name] = append(h.events[name], data)
}
func (h *fakeHost) OpenURL(url string)              { h.opened = append(h.opened, url) }
func (h *fakeHost) AutostartEnabled() (bool, error) { return h.autostart, nil }
func (h *fakeHost) SetAutostart(on bool) error      { h.autostart = on; return nil }

type fakeProv struct {
	mu            sync.Mutex
	name          string
	authenticated bool
	prompt        *provider.LoginPrompt
	expired       bool // next Me reports the saved login as expired
	entries       []provider.TimeEntry
	created       []provider.TimeEntry
	deleted       []provider.TimeEntry
}

func (f *fakeProv) Name() string { return f.name }
func (f *fakeProv) IsAuthenticated() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.authenticated
}
func (f *fakeProv) Login(_ context.Context, onPrompt func(provider.LoginPrompt)) error {
	if f.prompt != nil {
		onPrompt(*f.prompt)
	}
	f.mu.Lock()
	f.authenticated = true
	f.mu.Unlock()
	return nil
}
func (f *fakeProv) Me(context.Context) (provider.Member, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.expired {
		// The saved login was ended server-side: drop it, ask for a new one.
		f.expired = false
		f.authenticated = false
		return provider.Member{}, fmt.Errorf("refreshing: %w", provider.ErrReauthRequired)
	}
	return provider.Member{ID: "m1", FullName: "Sam Rivera"}, nil
}
func (f *fakeProv) ListProjects(context.Context) ([]provider.Project, error) {
	return []provider.Project{{ID: "p1", Name: "Atlas", ClientName: "Northwind", Color: "#123"}}, nil
}
func (f *fakeProv) ListTags(context.Context, string) ([]provider.Tag, error) {
	return []provider.Tag{{ID: "t1", Name: "billable"}}, nil
}
func (f *fakeProv) ListTimeEntries(context.Context, string, time.Time, time.Time) ([]provider.TimeEntry, error) {
	return f.entries, nil
}
func (f *fakeProv) CreateTimeEntry(_ context.Context, e provider.TimeEntry) (provider.TimeEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, e)
	return e, nil
}
func (f *fakeProv) UpdateTimeEntry(_ context.Context, e provider.TimeEntry) (provider.TimeEntry, error) {
	return e, nil
}
func (f *fakeProv) DeleteTimeEntry(_ context.Context, e provider.TimeEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, e)
	return nil
}

type memStore struct{}

func (memStore) LoadLastEntryDefaults(string) (config.LastEntryDefaults, error) {
	return config.LastEntryDefaults{}, errors.New("none")
}
func (memStore) SaveLastEntryDefaults(string, config.LastEntryDefaults) error { return nil }
func (memStore) SaveLastProvider(string) error                                { return nil }

func newTestService(t *testing.T, provs ...*fakeProv) (*Service, *fakeHost) {
	t.Helper()
	m := map[string]provider.Provider{}
	var names []string
	for _, p := range provs {
		m[p.name] = p
		names = append(names, p.name)
	}
	sess, err := service.NewSession(m, names, provs[0].name, memStore{})
	if err != nil {
		t.Fatal(err)
	}
	host := newFakeHost()
	return NewService(host, sess), host
}

func TestSetupServiceRefusesCalls(t *testing.T) {
	svc := NewSetupService(newFakeHost(), "write your config")
	st := svc.State()
	if st.SetupMessage != "write your config" || st.Active != "" {
		t.Errorf("state = %+v", st)
	}
	if _, err := svc.Start(context.Background()); err == nil {
		t.Error("Start should fail in setup mode")
	}
	if _, err := svc.Week(context.Background(), ""); err == nil {
		t.Error("Week should fail in setup mode")
	}
}

func TestStartForwardsLoginPromptAndOpensBrowser(t *testing.T) {
	p := &fakeProv{name: "a", prompt: &provider.LoginPrompt{URL: "https://auth.example/device", Code: "WXYZ"}}
	svc, host := newTestService(t, p)

	projects, err := svc.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Client != "Northwind" {
		t.Errorf("projects = %+v", projects)
	}
	if len(host.events[EventLoginPrompt]) != 1 || len(host.opened) != 1 {
		t.Errorf("events=%v opened=%v", host.events, host.opened)
	}
	st := svc.State()
	if st.Member != "Sam Rivera" || !st.HasProfile || st.NeedsLogin {
		t.Errorf("state = %+v", st)
	}
}

func TestWeekAndSaveRoundTrip(t *testing.T) {
	start := time.Date(2026, 11, 10, 9, 15, 0, 0, time.Local)
	p := &fakeProv{name: "a", authenticated: true, entries: []provider.TimeEntry{
		{ID: "e1", ProjectID: "p1", ProjectName: "Atlas", Start: start, End: start.Add(90 * time.Minute)},
	}}
	svc, _ := newTestService(t, p)
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	week, err := svc.Week(context.Background(), "2026-11-12")
	if err != nil {
		t.Fatal(err)
	}
	if week.Start != "2026-11-09" || week.End != "2026-11-15" {
		t.Errorf("range = %s..%s", week.Start, week.End)
	}
	if len(week.Entries) != 1 || week.LoggedMinutes != 90 {
		t.Fatalf("week = %+v", week)
	}
	e := week.Entries[0]
	if e.Date != "2026-11-10" || e.From != 9*60+15 || e.To != 10*60+45 {
		t.Errorf("entry = %+v", e)
	}

	err = svc.Save(context.Background(), Draft{ProjectID: "p1", Date: "2026-11-12", From: 600, To: 660, Description: " x ", Tags: []string{"t1"}})
	if err != nil {
		t.Fatal(err)
	}
	got := p.created[0]
	if want := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC); !got.Start.Equal(want) || got.Description != "x" || got.TagIDs[0] != "t1" {
		t.Errorf("created = %+v", got)
	}

	if err := svc.Save(context.Background(), Draft{ProjectID: "p1", Date: "nope", From: 1, To: 2}); err == nil {
		t.Error("expected invalid date error")
	}
	if err := svc.Save(context.Background(), Draft{ProjectID: "p1", Date: "2026-11-12", From: 600, To: 500}); !errors.Is(err, service.ErrEndNotAfter) {
		t.Errorf("err = %v, want ErrEndNotAfter", err)
	}
}

func TestDeleteUsesCachedEntryAndRefusesLocked(t *testing.T) {
	start := time.Date(2026, 11, 10, 9, 0, 0, 0, time.Local)
	p := &fakeProv{name: "a", authenticated: true, entries: []provider.TimeEntry{
		{ID: "open", ProjectID: "p1", Start: start, End: start.Add(time.Hour)},
		{ID: "locked", ProjectID: "p1", Start: start, End: start.Add(time.Hour), Locked: true},
	}}
	svc, _ := newTestService(t, p)
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), "open"); err == nil || !strings.Contains(err.Error(), "no longer listed") {
		t.Errorf("delete before listing: err = %v", err)
	}
	if _, err := svc.Week(context.Background(), "2026-11-10"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), "locked"); err == nil {
		t.Error("locked entry was deleted")
	}
	if err := svc.Save(context.Background(), Draft{ID: "locked", ProjectID: "p1", Date: "2026-11-10", From: 1, To: 2}); err == nil {
		t.Error("locked entry was edited")
	}
	if err := svc.Delete(context.Background(), "open"); err != nil {
		t.Fatal(err)
	}
	if len(p.deleted) != 1 || p.deleted[0].ID != "open" {
		t.Errorf("deleted = %+v", p.deleted)
	}
}

func TestStaleErrorsAreMapped(t *testing.T) {
	if err := publicError(service.ErrStale); err == nil || err.Error() != StaleMessage {
		t.Errorf("publicError(ErrStale) = %v", err)
	}
	other := errors.New("boom")
	if err := publicError(other); err != other {
		t.Errorf("publicError changed unrelated error: %v", err)
	}
}

func TestSwitchProviderClearsCacheAndAttendance(t *testing.T) {
	start := time.Date(2026, 11, 10, 9, 0, 0, 0, time.Local)
	a := &fakeProv{name: "a", authenticated: true, entries: []provider.TimeEntry{{ID: "e1", ProjectID: "p1", Start: start, End: start.Add(time.Hour)}}}
	b := &fakeProv{name: "b", authenticated: true}
	svc, host := newTestService(t, a, b)
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Week(context.Background(), "2026-11-10"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SwitchProvider("b"); err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.cachedEntry("e1"); ok {
		t.Error("entry cache survived provider switch")
	}
	if n := len(host.events[EventAttendance]); n == 0 {
		t.Error("switch didn't publish the reset attendance state")
	}
	if svc.State().Active != "b" {
		t.Error("active provider not switched")
	}
}

func TestParseWeekStart(t *testing.T) {
	now := time.Date(2026, 11, 14, 15, 0, 0, 0, time.Local) // Saturday
	got, err := parseWeekStart("", now)
	if err != nil || got.Format(dateLayout) != "2026-11-09" {
		t.Errorf("current week = %v, %v", got, err)
	}
	got, err = parseWeekStart("2026-11-01", now) // a Sunday
	if err != nil || got.Format(dateLayout) != "2026-10-26" {
		t.Errorf("sunday's week = %v, %v", got, err)
	}
	if _, err := parseWeekStart("11/01/2026", now); err == nil {
		t.Error("expected error for bad format")
	}
}

func TestAttendanceLabels(t *testing.T) {
	status, action, _ := attendanceLabels(nil, time.Now())
	if status != "Checked out" || action != "Check in…" {
		t.Errorf("checked out labels = %q %q", status, action)
	}
	in := time.Date(2026, 11, 14, 8, 30, 0, 0, time.Local)
	status, action, tip := attendanceLabels(&provider.AttendanceSession{WorkLocationName: "Office", CheckIn: in}, in.Add(95*time.Minute))
	if status != "At Office since 08:30" || action != "Check out…" || !strings.Contains(tip, "1h35m") {
		t.Errorf("checked in labels = %q %q %q", status, action, tip)
	}
}

// gatedProv blocks ListTimeEntries / AttendanceStatus on per-call gates so
// tests can force responses to arrive out of order.
type gatedProv struct {
	fakeProv
	listGates chan chan struct{}
	attGate   chan struct{}
	attOpen   *provider.AttendanceSession
}

func (g *gatedProv) ListTimeEntries(_ context.Context, _ string, from, _ time.Time) ([]provider.TimeEntry, error) {
	gate := <-g.listGates
	<-gate
	return []provider.TimeEntry{{ID: from.Format(dateLayout), ProjectID: "p1", Start: from.Add(9 * time.Hour), End: from.Add(10 * time.Hour)}}, nil
}
func (g *gatedProv) AttendanceEnabled() bool { return true }
func (g *gatedProv) ListWorkLocations(context.Context) ([]provider.WorkLocation, error) {
	return []provider.WorkLocation{{ID: "office", Name: "Office"}}, nil
}
func (g *gatedProv) LastWorkLocation(context.Context) (string, error) { return "", nil }
func (g *gatedProv) AttendanceStatus(ctx context.Context) (*provider.AttendanceSession, error) {
	if g.attGate != nil {
		<-g.attGate
	}
	return g.attOpen, nil
}
func (g *gatedProv) CheckIn(_ context.Context, id, note string) (provider.AttendanceSession, error) {
	return provider.AttendanceSession{WorkLocationID: id, WorkLocationName: "Office", CheckIn: time.Now()}, nil
}
func (g *gatedProv) CheckOut(_ context.Context, s provider.AttendanceSession, _ string) (provider.AttendanceSession, error) {
	s.CheckOut = time.Now()
	return s, nil
}

func newGatedService(t *testing.T, g *gatedProv) *Service {
	t.Helper()
	sess, err := service.NewSession(map[string]provider.Provider{g.name: g}, []string{g.name}, g.name, memStore{})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(newFakeHost(), sess)
	if _, err := sess.Current().LoadProfile(context.Background()); err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestWeekCacheKeepsNewestRequestNotLastResponse(t *testing.T) {
	g := &gatedProv{fakeProv: fakeProv{name: "a", authenticated: true}, listGates: make(chan chan struct{}, 2)}
	svc := newGatedService(t, g)
	gateA, gateB := make(chan struct{}), make(chan struct{})
	g.listGates <- gateA
	doneA := make(chan struct{})
	go func() { defer close(doneA); _, _ = svc.Week(context.Background(), "2026-11-02") }()
	for len(g.listGates) != 0 {
		time.Sleep(time.Millisecond)
	}
	g.listGates <- gateB
	doneB := make(chan struct{})
	go func() { defer close(doneB); _, _ = svc.Week(context.Background(), "2026-11-09") }()
	for len(g.listGates) != 0 {
		time.Sleep(time.Millisecond)
	}

	close(gateB) // newer request answers first...
	<-doneB
	close(gateA) // ...then the older one
	<-doneA

	if _, ok := svc.cachedEntry("2026-11-09"); !ok {
		t.Error("cache lost the newest week's entries")
	}
	if _, ok := svc.cachedEntry("2026-11-02"); ok {
		t.Error("late response for an older week overwrote the cache")
	}
}

func TestSlowStatusFetchCannotUndoCheckOut(t *testing.T) {
	open := &provider.AttendanceSession{WorkLocationName: "Office", CheckIn: time.Now().Add(-time.Hour)}
	g := &gatedProv{fakeProv: fakeProv{name: "a", authenticated: true}, attOpen: open}
	svc := newGatedService(t, g)
	if _, err := svc.AttendanceStatus(context.Background()); err != nil {
		t.Fatal(err)
	}

	g.attGate = make(chan struct{})
	done := make(chan struct{})
	go func() { defer close(done); _, _ = svc.AttendanceStatus(context.Background()) }() // reads "open"
	time.Sleep(20 * time.Millisecond)
	if err := svc.CheckOut(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	close(g.attGate)
	<-done

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.openSession != nil {
		t.Error("stale status fetch re-opened a session the user checked out of")
	}
}

func TestAttendanceHookAndSwitchHideIt(t *testing.T) {
	g := &gatedProv{fakeProv: fakeProv{name: "a", authenticated: true}}
	b := &fakeProv{name: "b", authenticated: true}
	sess, err := service.NewSession(map[string]provider.Provider{"a": g, "b": b}, []string{"a", "b"}, "a", memStore{})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(newFakeHost(), sess)
	var mu sync.Mutex
	var last struct {
		open    *provider.AttendanceSession
		enabled bool
	}
	svc.onAttendance = func(open *provider.AttendanceSession, enabled bool) {
		mu.Lock()
		defer mu.Unlock()
		last.open, last.enabled = open, enabled
	}

	if _, err := svc.CheckIn(context.Background(), "office", ""); err != nil {
		t.Fatal(err)
	}
	if last.open == nil || !last.enabled {
		t.Fatalf("after check-in hook got %+v", last)
	}
	if err := svc.SwitchProvider("b"); err != nil {
		t.Fatal(err)
	}
	if last.open != nil || last.enabled {
		t.Errorf("after switching to a provider without attendance hook got %+v", last)
	}
}

func TestSetAutostartNotifiesTrayAndWindow(t *testing.T) {
	svc, host := newTestService(t, &fakeProv{name: "a", authenticated: true})
	var got []bool
	svc.onAutostart = func(on bool) { got = append(got, on) }
	if err := svc.SetAutostart(true); err != nil {
		t.Fatal(err)
	}
	if !host.autostart || len(got) != 1 || !got[0] || len(host.events[EventAutostart]) != 1 {
		t.Errorf("autostart=%v hook=%v events=%v", host.autostart, got, host.events[EventAutostart])
	}
}

func TestToEntryMidnightHandling(t *testing.T) {
	day := time.Date(2026, 11, 10, 0, 0, 0, 0, time.Local)
	cases := []struct {
		name         string
		start, end   time.Time
		wantTo       int
		wantEditable bool
	}{
		{"same day", day.Add(9 * time.Hour), day.Add(17 * time.Hour), 17 * 60, true},
		{"ends at midnight", day.Add(23 * time.Hour), day.AddDate(0, 0, 1), 24 * 60, true},
		{"crosses midnight", day.Add(22 * time.Hour), day.AddDate(0, 0, 1).Add(2 * time.Hour), 26 * 60, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := toEntry(provider.TimeEntry{ID: "x", Start: c.start, End: c.end})
			if e.To != c.wantTo || e.Editable != c.wantEditable {
				t.Errorf("To=%d Editable=%v, want %d %v", e.To, e.Editable, c.wantTo, c.wantEditable)
			}
		})
	}
	if toEntry(provider.TimeEntry{Start: day, End: day.Add(time.Hour), Locked: true}).Editable {
		t.Error("locked entry reported editable")
	}
}

func TestSafeColorAndLoginURL(t *testing.T) {
	for in, want := range map[string]string{"#abc": "#abc", "#A1B2C3": "#A1B2C3", "red": "", "url(https://x/y)": "", "#12345": "", "": ""} {
		if got := safeColor(in); got != want {
			t.Errorf("safeColor(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]bool{
		"https://auth.example/device": true, "http://localhost:8080/x": true,
		"file:///etc/passwd": false, "ms-settings:privacy": false, "javascript:alert(1)": false, "https://": false, "": false,
	} {
		if got := safeLoginURL(in); got != want {
			t.Errorf("safeLoginURL(%q) = %v, want %v", in, got, want)
		}
	}
}

// The bug a user hit: a saved TrackIt login the server had ended looked
// valid, so Start failed and the week then showed "profile not loaded yet"
// with no way to log in again. Start must now prompt a fresh login.
func TestStartRecoversFromExpiredSavedLogin(t *testing.T) {
	p := &fakeProv{name: "a", authenticated: true, expired: true,
		prompt: &provider.LoginPrompt{URL: "https://auth.example/device", Code: "NEW1"}}
	svc, host := newTestService(t, p)

	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(host.events[EventLoginPrompt]) != 1 {
		t.Errorf("login prompts = %d, want 1", len(host.events[EventLoginPrompt]))
	}
	if st := svc.State(); !st.HasProfile || st.Member != "Sam Rivera" {
		t.Errorf("state after recovery = %+v", st)
	}
}

func TestWeekWithoutProfileAsksFrontendToReconnect(t *testing.T) {
	svc, _ := newTestService(t, &fakeProv{name: "a", authenticated: true})
	_, err := svc.Week(context.Background(), "")
	if err == nil || err.Error() != ReconnectMessage {
		t.Fatalf("err = %v, want %q", err, ReconnectMessage)
	}
	if err := publicError(fmt.Errorf("x: %w", provider.ErrReauthRequired)); err.Error() != ReconnectMessage {
		t.Errorf("ErrReauthRequired mapped to %v", err)
	}
}

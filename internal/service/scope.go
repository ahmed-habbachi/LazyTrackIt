package service

import (
	"context"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// Scope is an immutable snapshot of a Session's active provider and what
// had been loaded for it when the snapshot was taken (see Session.Current).
// Every call made through a Scope targets that one provider, with that
// member and those projects: if the Session has switched provider since,
// writes are refused and reads are discarded with ErrStale, and calls
// already in flight are cancelled.
type Scope struct {
	s         *Session
	gen       uint64
	ctx       context.Context // cancelled when this generation ends
	name      string
	prov      provider.Provider
	member    provider.Member
	hasMember bool
	projects  []provider.Project
}

// ProviderName is the name of the provider this Scope targets.
func (sc Scope) ProviderName() string { return sc.name }

// Member returns the member loaded when the Scope was taken, if any.
func (sc Scope) Member() (provider.Member, bool) { return sc.member, sc.hasMember }

// Projects returns a copy of the projects loaded when the Scope was taken.
func (sc Scope) Projects() []provider.Project {
	return append([]provider.Project(nil), sc.projects...)
}

// Valid reports whether the Session is still on this Scope's provider.
func (sc Scope) Valid() bool { return sc.s.isCurrent(sc.gen) }

// NeedsLogin reports whether the provider has no usable credentials.
func (sc Scope) NeedsLogin() bool { return !sc.prov.IsAuthenticated() }

// Attendance returns the provider's check-in/out capability, or nil if it
// doesn't have one or it isn't enabled in the config.
func (sc Scope) Attendance() provider.Attendance {
	a, ok := sc.prov.(provider.Attendance)
	if !ok || !a.AttendanceEnabled() {
		return nil
	}
	return a
}

// begin derives the context for one call: bounded by timeout, cancelled
// along with parent, and cancelled when the provider is switched away from.
// It returns ErrStale up front if that already happened.
func (sc Scope) begin(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if !sc.Valid() {
		return nil, nil, ErrStale
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	stop := context.AfterFunc(sc.ctx, cancel)
	return ctx, func() { stop(); cancel() }, nil
}

// finish maps the outcome of a call: if the provider was switched away from
// while it ran, its result (or the cancellation error the switch caused) is
// replaced by ErrStale.
func (sc Scope) finish(err error) error {
	if !sc.Valid() {
		return ErrStale
	}
	return err
}

// LoadProfile fetches the authenticated member and their projects and, if
// the Session is still on this provider, stores them. It returns the Scope
// to use from then on, which includes the loaded profile.
func (sc Scope) LoadProfile(ctx context.Context) (Scope, error) {
	ctx, done, err := sc.begin(ctx, RequestTimeout)
	if err != nil {
		return sc, err
	}
	defer done()

	member, err := sc.prov.Me(ctx)
	if err != nil {
		return sc, sc.finish(err)
	}
	projects, err := sc.prov.ListProjects(ctx)
	if err != nil {
		return sc, sc.finish(err)
	}

	s := sc.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != sc.gen {
		return sc, ErrStale
	}
	s.member = member
	s.hasMember = true
	s.projects = append([]provider.Project(nil), projects...)

	sc.member = member
	sc.hasMember = true
	sc.projects = append([]provider.Project(nil), projects...)
	return sc, nil
}

// ListEntries returns the member's entries for the week starting at
// weekStart, which must be local midnight of that week's Monday (see
// StartOfWeek), since providers query by local calendar dates.
func (sc Scope) ListEntries(ctx context.Context, weekStart time.Time) ([]provider.TimeEntry, error) {
	if !sc.hasMember {
		return nil, ErrNoProfile
	}
	ctx, done, err := sc.begin(ctx, RequestTimeout)
	if err != nil {
		return nil, err
	}
	defer done()
	entries, err := sc.prov.ListTimeEntries(ctx, sc.member.ID, weekStart, WeekEnd(weekStart))
	if err = sc.finish(err); err != nil {
		return nil, err
	}
	return entries, nil
}

// ListTags returns the tags usable on projectID.
func (sc Scope) ListTags(ctx context.Context, projectID string) ([]provider.Tag, error) {
	ctx, done, err := sc.begin(ctx, RequestTimeout)
	if err != nil {
		return nil, err
	}
	defer done()
	tags, err := sc.prov.ListTags(ctx, projectID)
	if err = sc.finish(err); err != nil {
		return nil, err
	}
	return tags, nil
}

// BuildEntry validates d against this Scope's projects and member (see the
// package-level BuildEntry).
func (sc Scope) BuildEntry(d EntryDraft, availableTags []provider.Tag) (provider.TimeEntry, error) {
	if !sc.hasMember {
		return provider.TimeEntry{}, ErrNoProfile
	}
	return BuildEntry(d, sc.projects, sc.member.ID, availableTags)
}

// SaveDraft validates d (see BuildEntry), then creates or updates the entry.
func (sc Scope) SaveDraft(ctx context.Context, d EntryDraft, availableTags []provider.Tag) (provider.TimeEntry, error) {
	entry, err := sc.BuildEntry(d, availableTags)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	return sc.SaveEntry(ctx, entry)
}

// SaveEntry creates e (if e.ID is empty) or updates it. A successful create
// remembers its project/tags as the defaults for this provider's next new
// entry.
func (sc Scope) SaveEntry(ctx context.Context, e provider.TimeEntry) (provider.TimeEntry, error) {
	ctx, done, err := sc.begin(ctx, RequestTimeout)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	defer done()

	if e.ID != "" {
		return sc.prov.UpdateTimeEntry(ctx, e)
	}
	saved, err := sc.prov.CreateTimeEntry(ctx, e)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	sc.rememberDefaults(config.LastEntryDefaults{
		ProjectID: e.ProjectID,
		TagIDs:    append([]string(nil), e.TagIDs...),
	})
	return saved, nil
}

// rememberDefaults persists d for this Scope's provider, and also applies it
// in memory if the Session is still on that provider. The entry was really
// created on this provider either way, so its defaults are always saved
// under its name.
func (sc Scope) rememberDefaults(d config.LastEntryDefaults) {
	s := sc.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen == sc.gen {
		s.lastDefaults = d
	}
	_ = s.store.SaveLastEntryDefaults(sc.name, d)
}

// DeleteEntry removes e.
func (sc Scope) DeleteEntry(ctx context.Context, e provider.TimeEntry) error {
	ctx, done, err := sc.begin(ctx, RequestTimeout)
	if err != nil {
		return err
	}
	defer done()
	return sc.prov.DeleteTimeEntry(ctx, e)
}

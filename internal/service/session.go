package service

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// RequestTimeout bounds a single backend call made on the user's behalf.
const RequestTimeout = 20 * time.Second

// LoginTimeout bounds an interactive login (e.g. waiting for the user to
// approve a device-code request in their browser).
const LoginTimeout = 15 * time.Minute

// ErrNoProfile is returned by calls that need the authenticated member
// before LoadProfile has succeeded.
var ErrNoProfile = errors.New("profile not loaded yet")

// ErrStale is returned when the active provider was switched after the
// Scope a call was made with was taken. The call is either not sent at all
// or, for reads already in flight, its result is discarded, so nothing
// meant for one provider is ever applied to another. Front ends should
// treat it as "ignore", not as an error to show.
var ErrStale = errors.New("active provider changed during the request")

// StateStore persists the small bits of per-user state a Session remembers
// across runs. config's file-backed functions are the production
// implementation; tests substitute an in-memory one.
type StateStore interface {
	LoadLastEntryDefaults(providerName string) (config.LastEntryDefaults, error)
	SaveLastEntryDefaults(providerName string, d config.LastEntryDefaults) error
	SaveLastProvider(name string) error
}

type fileStateStore struct{}

func (fileStateStore) LoadLastEntryDefaults(name string) (config.LastEntryDefaults, error) {
	return config.LoadLastEntryDefaults(name)
}

func (fileStateStore) SaveLastEntryDefaults(name string, d config.LastEntryDefaults) error {
	return config.SaveLastEntryDefaults(name, d)
}

func (fileStateStore) SaveLastProvider(name string) error { return config.SaveLastProvider(name) }

// FileStateStore returns the StateStore backed by LazyTrackIt's state files.
func FileStateStore() StateStore { return fileStateStore{} }

// Session is the state shared by every front end for one run of
// LazyTrackIt: the configured providers, which one is active, and what has
// been loaded for it (member, projects, last-used entry defaults). It is
// safe for concurrent use.
//
// Work against the active provider goes through a Scope (see Current): an
// immutable snapshot taken under one lock. Switching providers starts a new
// generation and cancels the previous one's context, so any Scope from
// before the switch stops sending requests and has its results discarded.
type Session struct {
	providers map[string]provider.Provider
	names     []string
	store     StateStore

	mu           sync.RWMutex
	gen          uint64 // bumped on every provider switch
	genCtx       context.Context
	genCancel    context.CancelFunc
	active       string
	prov         provider.Provider
	member       provider.Member
	hasMember    bool
	projects     []provider.Project
	lastDefaults config.LastEntryDefaults
	login        *loginFlight // in-flight login for the current generation
}

// NewSession builds a Session over providers (keyed by name), with names as
// their display order and active as the provider to start with. A nil store
// uses FileStateStore.
func NewSession(providers map[string]provider.Provider, names []string, active string, store StateStore) (*Session, error) {
	prov, ok := providers[active]
	if !ok {
		return nil, fmt.Errorf("active provider %q is not configured", active)
	}
	if store == nil {
		store = FileStateStore()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{
		providers: maps.Clone(providers),
		names:     append([]string(nil), names...),
		store:     store,
		genCtx:    ctx,
		genCancel: cancel,
		active:    active,
		prov:      prov,
	}
	s.lastDefaults, _ = store.LoadLastEntryDefaults(active)
	return s, nil
}

// ProviderNames returns the configured providers' names in display order.
func (s *Session) ProviderNames() []string { return append([]string(nil), s.names...) }

// Current returns a snapshot of the active provider and what's been loaded
// for it, to run calls against.
func (s *Session) Current() Scope {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Scope{
		s:         s,
		gen:       s.gen,
		ctx:       s.genCtx,
		name:      s.active,
		prov:      s.prov,
		member:    s.member,
		hasMember: s.hasMember,
		projects:  append([]provider.Project(nil), s.projects...),
	}
}

// ActiveProviderName returns the name of the active provider.
func (s *Session) ActiveProviderName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

// Member returns the authenticated member, and whether it has loaded yet.
func (s *Session) Member() (provider.Member, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.member, s.hasMember
}

// HasProfile reports whether the member and projects have been loaded.
func (s *Session) HasProfile() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasMember
}

// Projects returns a copy of the active provider's loaded projects.
func (s *Session) Projects() []provider.Project {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]provider.Project(nil), s.projects...)
}

// LastDefaults returns the project/tags most recently used to create an
// entry with the active provider.
func (s *Session) LastDefaults() config.LastEntryDefaults {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.lastDefaults
	d.TagIDs = append([]string(nil), d.TagIDs...)
	return d
}

// Attendance returns the active provider's check-in/out capability, or nil
// if it doesn't have one or it isn't enabled in the config.
func (s *Session) Attendance() provider.Attendance { return s.Current().Attendance() }

// SwitchProvider makes name the active provider, discarding everything
// loaded for the previous one and cancelling its in-flight calls (including
// a pending login), and remembers the choice for next startup. Callers then
// take a fresh Current() and re-run Authenticate and LoadProfile on it.
func (s *Session) SwitchProvider(name string) error {
	prov, ok := s.providers[name]
	if !ok {
		return fmt.Errorf("provider %q is not configured", name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.genCancel()
	s.genCtx, s.genCancel = context.WithCancel(context.Background())
	s.gen++
	s.active = name
	s.prov = prov
	s.member = provider.Member{}
	s.hasMember = false
	s.projects = nil
	s.login = nil
	s.lastDefaults, _ = s.store.LoadLastEntryDefaults(name)
	_ = s.store.SaveLastProvider(name)
	return nil
}

// RememberActiveProvider records the active provider as the one to start
// with next time.
func (s *Session) RememberActiveProvider() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_ = s.store.SaveLastProvider(s.active)
}

// isCurrent reports whether gen is still the active generation.
func (s *Session) isCurrent(gen uint64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gen == gen
}

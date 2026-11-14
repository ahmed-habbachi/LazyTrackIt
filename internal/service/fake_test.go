package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// fakeProvider is an in-memory provider.Provider (and provider.Attendance)
// for exercising Session without a network.
type fakeProvider struct {
	mu sync.Mutex

	name          string
	authenticated bool
	loginErr      error
	prompt        *provider.LoginPrompt
	// loginGate, if set, blocks Login (after showing the prompt) until it's
	// closed or ctx ends.
	loginGate  chan struct{}
	loginCalls atomic.Int32
	member     provider.Member
	projects   []provider.Project
	tags       []provider.Tag
	entries    []provider.TimeEntry
	saveErr    error

	// meGate, if set, blocks Me until it's closed, so tests can switch
	// providers while a profile load is in flight.
	meGate chan struct{}
	// expireOnce makes the next Me fail with ErrReauthRequired.
	expireOnce bool
	// meEntered, if set, is closed as soon as Me is called.
	meEntered chan struct{}

	created, updated, deleted []provider.TimeEntry
	listFrom, listTo          time.Time
	listMemberID              string

	attendanceEnabled bool
	locations         []provider.WorkLocation
	lastLocationID    string
	openSession       *provider.AttendanceSession
	checkInNote       string
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) IsAuthenticated() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.authenticated
}

func (f *fakeProvider) Login(ctx context.Context, onPrompt func(provider.LoginPrompt)) error {
	f.loginCalls.Add(1)
	if f.prompt != nil {
		onPrompt(*f.prompt)
	}
	if f.loginGate != nil {
		select {
		case <-f.loginGate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.loginErr != nil {
		return f.loginErr
	}
	f.mu.Lock()
	f.authenticated = true
	f.mu.Unlock()
	return nil
}

func (f *fakeProvider) Me(ctx context.Context) (provider.Member, error) {
	f.mu.Lock()
	if f.expireOnce {
		// Simulate a saved login the server has ended: the provider drops
		// its credentials and asks for a fresh login.
		f.expireOnce = false
		f.authenticated = false
		f.mu.Unlock()
		return provider.Member{}, fmt.Errorf("wrapped: %w", provider.ErrReauthRequired)
	}
	f.mu.Unlock()
	if f.meEntered != nil {
		close(f.meEntered)
	}
	if f.meGate != nil {
		select {
		case <-f.meGate:
		case <-ctx.Done():
			return provider.Member{}, ctx.Err()
		}
	}
	return f.member, nil
}

func (f *fakeProvider) ListProjects(context.Context) ([]provider.Project, error) {
	return f.projects, nil
}

func (f *fakeProvider) ListTags(context.Context, string) ([]provider.Tag, error) {
	return f.tags, nil
}

func (f *fakeProvider) ListTimeEntries(_ context.Context, memberID string, from, to time.Time) ([]provider.TimeEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listMemberID, f.listFrom, f.listTo = memberID, from, to
	return f.entries, nil
}

func (f *fakeProvider) CreateTimeEntry(_ context.Context, e provider.TimeEntry) (provider.TimeEntry, error) {
	if f.saveErr != nil {
		return provider.TimeEntry{}, f.saveErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	e.ID = "new"
	f.created = append(f.created, e)
	return e, nil
}

func (f *fakeProvider) UpdateTimeEntry(_ context.Context, e provider.TimeEntry) (provider.TimeEntry, error) {
	if f.saveErr != nil {
		return provider.TimeEntry{}, f.saveErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, e)
	return e, nil
}

func (f *fakeProvider) DeleteTimeEntry(_ context.Context, e provider.TimeEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, e)
	return nil
}

func (f *fakeProvider) AttendanceEnabled() bool { return f.attendanceEnabled }

func (f *fakeProvider) ListWorkLocations(context.Context) ([]provider.WorkLocation, error) {
	return f.locations, nil
}

func (f *fakeProvider) LastWorkLocation(context.Context) (string, error) {
	return f.lastLocationID, nil
}

func (f *fakeProvider) AttendanceStatus(context.Context) (*provider.AttendanceSession, error) {
	return f.openSession, nil
}

func (f *fakeProvider) CheckIn(_ context.Context, id, note string) (provider.AttendanceSession, error) {
	f.checkInNote = note
	return provider.AttendanceSession{WorkLocationID: id, Note: note}, nil
}

func (f *fakeProvider) CheckOut(_ context.Context, s provider.AttendanceSession, note string) (provider.AttendanceSession, error) {
	s.Note = note
	s.CheckOut = time.Now()
	return s, nil
}

// memStore is an in-memory StateStore.
type memStore struct {
	mu           sync.Mutex
	defaults     map[string]config.LastEntryDefaults
	lastProvider string
}

func newMemStore() *memStore {
	return &memStore{defaults: map[string]config.LastEntryDefaults{}}
}

func (m *memStore) LoadLastEntryDefaults(name string) (config.LastEntryDefaults, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.defaults[name]
	if !ok {
		return config.LastEntryDefaults{}, errors.New("not found")
	}
	return d, nil
}

func (m *memStore) SaveLastEntryDefaults(name string, d config.LastEntryDefaults) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaults[name] = d
	return nil
}

func (m *memStore) SaveLastProvider(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastProvider = name
	return nil
}

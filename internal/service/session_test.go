package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// waitTimeout bounds every channel wait in these tests, so a regression
// fails fast instead of hanging the test binary.
const waitTimeout = 5 * time.Second

func newFake(name string) *fakeProvider {
	return &fakeProvider{
		name:          name,
		authenticated: true,
		member:        provider.Member{ID: "m-" + name, FullName: "Ada " + name},
		projects:      []provider.Project{{ID: "p1", Name: "Alpha"}, {ID: "p2", Name: "Beta"}},
	}
}

func newTestSession(t *testing.T, provs ...*fakeProvider) (*Session, *memStore) {
	t.Helper()
	m := map[string]provider.Provider{}
	names := []string{}
	for _, p := range provs {
		m[p.name] = p
		names = append(names, p.name)
	}
	store := newMemStore()
	s, err := NewSession(m, names, provs[0].name, store)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return s, store
}

// loaded returns a Scope with the profile loaded.
func loaded(t *testing.T, s *Session) Scope {
	t.Helper()
	sc, err := s.Current().LoadProfile(context.Background())
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	return sc
}

func recv[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(waitTimeout):
		t.Fatal("timed out waiting on channel")
		var zero T
		return zero
	}
}

func wait(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(waitTimeout):
		t.Fatal("timed out waiting on channel")
	}
}

func TestNewSession(t *testing.T) {
	t.Run("rejects unknown active", func(t *testing.T) {
		_, err := NewSession(map[string]provider.Provider{"a": newFake("a")}, []string{"a"}, "nope", newMemStore())
		if err == nil {
			t.Fatal("expected error for unknown active provider")
		}
	})

	t.Run("does not alias caller's map or names", func(t *testing.T) {
		provs := map[string]provider.Provider{"a": newFake("a"), "b": newFake("b")}
		names := []string{"a", "b"}
		s, err := NewSession(provs, names, "a", newMemStore())
		if err != nil {
			t.Fatal(err)
		}
		delete(provs, "b")
		names[0] = "mutated"
		if err := s.SwitchProvider("b"); err != nil {
			t.Errorf("SwitchProvider after caller mutated map: %v", err)
		}
		if s.ProviderNames()[0] != "a" {
			t.Error("ProviderNames aliases caller slice")
		}
	})

	t.Run("loads active provider's defaults", func(t *testing.T) {
		store := newMemStore()
		store.defaults["a"] = config.LastEntryDefaults{ProjectID: "p2", TagIDs: []string{"t"}}
		s, err := NewSession(map[string]provider.Provider{"a": newFake("a")}, []string{"a"}, "a", store)
		if err != nil {
			t.Fatal(err)
		}
		d := s.LastDefaults()
		d.TagIDs[0] = "mutated"
		if s.LastDefaults().ProjectID != "p2" || s.LastDefaults().TagIDs[0] != "t" {
			t.Errorf("LastDefaults = %+v (or exposes internal slice)", s.LastDefaults())
		}
	})
}

func TestAuthenticate(t *testing.T) {
	t.Run("already authenticated skips login", func(t *testing.T) {
		f := newFake("a")
		s, _ := newTestSession(t, f)
		if err := s.Current().Authenticate(context.Background(), nil); err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if f.loginCalls.Load() != 0 {
			t.Error("Login called for an authenticated provider")
		}
	})

	t.Run("forwards prompt and logs in", func(t *testing.T) {
		f := newFake("a")
		f.authenticated = false
		f.prompt = &provider.LoginPrompt{Code: "ABCD"}
		s, _ := newTestSession(t, f)
		sc := s.Current()
		if !sc.NeedsLogin() {
			t.Fatal("NeedsLogin = false, want true")
		}
		var got string
		if err := sc.Authenticate(context.Background(), func(p provider.LoginPrompt) { got = p.Code }); err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if got != "ABCD" {
			t.Errorf("prompt code = %q, want ABCD", got)
		}
		if sc.NeedsLogin() {
			t.Error("NeedsLogin = true after successful login")
		}
	})

	t.Run("returns login failure unchanged and allows retry", func(t *testing.T) {
		f := newFake("a")
		f.authenticated = false
		sentinel := errors.New("denied")
		f.loginErr = sentinel
		s, _ := newTestSession(t, f)
		if err := s.Current().Authenticate(context.Background(), nil); err != sentinel {
			t.Fatalf("err = %v, want %v unchanged", err, sentinel)
		}
		f.loginErr = nil
		if err := s.Current().Authenticate(context.Background(), nil); err != nil {
			t.Fatalf("retry: %v", err)
		}
		if n := f.loginCalls.Load(); n != 2 {
			t.Errorf("login calls = %d, want 2", n)
		}
	})

	t.Run("concurrent callers share one login and all see the prompt", func(t *testing.T) {
		f := newFake("a")
		f.authenticated = false
		f.prompt = &provider.LoginPrompt{Code: "XY"}
		f.loginGate = make(chan struct{})
		s, _ := newTestSession(t, f)

		const callers = 5
		prompts := make(chan string, callers)
		errs := make(chan error, callers)
		for range callers {
			go func() {
				errs <- s.Current().Authenticate(context.Background(), func(p provider.LoginPrompt) { prompts <- p.Code })
			}()
		}
		for range callers {
			if code := recv(t, prompts); code != "XY" {
				t.Errorf("prompt = %q", code)
			}
		}
		close(f.loginGate)
		for range callers {
			if err := recv(t, errs); err != nil {
				t.Errorf("Authenticate: %v", err)
			}
		}
		if n := f.loginCalls.Load(); n != 1 {
			t.Errorf("login calls = %d, want 1", n)
		}
	})

	t.Run("caller giving up does not abort the shared login", func(t *testing.T) {
		f := newFake("a")
		f.authenticated = false
		f.loginGate = make(chan struct{})
		s, _ := newTestSession(t, f)

		ctx, cancel := context.WithCancel(context.Background())
		first := make(chan error, 1)
		go func() { first <- s.Current().Authenticate(ctx, nil) }()
		second := make(chan error, 1)
		go func() { second <- s.Current().Authenticate(context.Background(), nil) }()

		for f.loginCalls.Load() == 0 {
			time.Sleep(time.Millisecond)
		}
		cancel()
		if err := recv(t, first); !errors.Is(err, context.Canceled) {
			t.Errorf("first caller err = %v, want Canceled", err)
		}
		close(f.loginGate)
		if err := recv(t, second); err != nil {
			t.Errorf("second caller err = %v", err)
		}
	})

	t.Run("switching provider cancels login with ErrStale", func(t *testing.T) {
		a, b := newFake("a"), newFake("b")
		a.authenticated = false
		a.loginGate = make(chan struct{}) // never closed
		s, _ := newTestSession(t, a, b)

		done := make(chan error, 1)
		go func() { done <- s.Current().Authenticate(context.Background(), nil) }()
		for a.loginCalls.Load() == 0 {
			time.Sleep(time.Millisecond)
		}
		if err := s.SwitchProvider("b"); err != nil {
			t.Fatal(err)
		}
		if err := recv(t, done); !errors.Is(err, ErrStale) {
			t.Fatalf("err = %v, want ErrStale", err)
		}
	})
}

func TestLoadProfileAndListEntries(t *testing.T) {
	f := newFake("a")
	s, _ := newTestSession(t, f)

	if _, err := s.Current().ListEntries(context.Background(), time.Now()); !errors.Is(err, ErrNoProfile) {
		t.Fatalf("ListEntries before profile: err = %v, want ErrNoProfile", err)
	}

	sc := loaded(t, s)
	member, ok := sc.Member()
	if !ok || member.ID != "m-a" || len(sc.Projects()) != 2 || !s.HasProfile() {
		t.Fatalf("unexpected profile: %+v %+v", member, sc.Projects())
	}

	weekStart := time.Date(2026, 11, 9, 0, 0, 0, 0, time.Local)
	if _, err := sc.ListEntries(context.Background(), weekStart); err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if f.listMemberID != "m-a" || !f.listFrom.Equal(weekStart) || !f.listTo.Equal(weekStart.AddDate(0, 0, 6)) {
		t.Errorf("ListTimeEntries got member=%s from=%v to=%v", f.listMemberID, f.listFrom, f.listTo)
	}
}

func TestProjectsReturnCopies(t *testing.T) {
	s, _ := newTestSession(t, newFake("a"))
	sc := loaded(t, s)
	sc.Projects()[0].Name = "mutated"
	s.Projects()[0].Name = "mutated"
	if sc.Projects()[0].Name != "Alpha" || s.Projects()[0].Name != "Alpha" {
		t.Error("Projects() exposed internal slice")
	}
}

func TestSwitchProviderResetsState(t *testing.T) {
	a, b := newFake("a"), newFake("b")
	s, store := newTestSession(t, a, b)
	store.defaults["b"] = config.LastEntryDefaults{ProjectID: "p2"}
	old := loaded(t, s)

	if err := s.SwitchProvider("b"); err != nil {
		t.Fatalf("SwitchProvider: %v", err)
	}
	if s.ActiveProviderName() != "b" || s.HasProfile() || len(s.Projects()) != 0 {
		t.Errorf("state not reset after switch")
	}
	if old.Valid() || !s.Current().Valid() {
		t.Error("Valid() wrong after switch")
	}
	if s.LastDefaults().ProjectID != "p2" {
		t.Errorf("LastDefaults = %+v, want b's defaults", s.LastDefaults())
	}
	if store.lastProvider != "b" {
		t.Errorf("last provider saved = %q, want b", store.lastProvider)
	}
	if err := s.SwitchProvider("missing"); err == nil {
		t.Error("expected error switching to unknown provider")
	}
}

func TestLoadProfileDiscardsResultAfterSwitch(t *testing.T) {
	a, b := newFake("a"), newFake("b")
	a.meGate = make(chan struct{})
	a.meEntered = make(chan struct{})
	s, _ := newTestSession(t, a, b)

	done := make(chan error, 1)
	go func() {
		_, err := s.Current().LoadProfile(context.Background())
		done <- err
	}()

	wait(t, a.meEntered) // LoadProfile has snapshotted provider a
	if err := s.SwitchProvider("b"); err != nil {
		t.Fatal(err)
	}

	if err := recv(t, done); !errors.Is(err, ErrStale) {
		t.Fatalf("err = %v, want ErrStale", err)
	}
	if s.HasProfile() {
		t.Error("stale profile from provider a was applied to b")
	}
}

// A Scope taken before a switch must never send a write: otherwise an entry
// built with provider A's member/project IDs (or an entry ID from A) would
// be created, updated or deleted on provider B.
func TestStaleScopeRefusesWrites(t *testing.T) {
	a, b := newFake("a"), newFake("b")
	s, store := newTestSession(t, a, b)
	old := loaded(t, s)
	if err := s.SwitchProvider("b"); err != nil {
		t.Fatal(err)
	}

	date := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	draft := EntryDraft{ProjectID: "p1", Date: date, FromMinutes: 60, ToMinutes: 120}
	ctx := context.Background()

	if _, err := old.SaveDraft(ctx, draft, nil); !errors.Is(err, ErrStale) {
		t.Errorf("create: err = %v, want ErrStale", err)
	}
	if _, err := old.SaveEntry(ctx, provider.TimeEntry{ID: "1"}); !errors.Is(err, ErrStale) {
		t.Errorf("update: err = %v, want ErrStale", err)
	}
	if err := old.DeleteEntry(ctx, provider.TimeEntry{ID: "1"}); !errors.Is(err, ErrStale) {
		t.Errorf("delete: err = %v, want ErrStale", err)
	}
	if _, err := old.ListEntries(ctx, date); !errors.Is(err, ErrStale) {
		t.Errorf("list: err = %v, want ErrStale", err)
	}
	if _, err := old.ListTags(ctx, "p1"); !errors.Is(err, ErrStale) {
		t.Errorf("tags: err = %v, want ErrStale", err)
	}
	for _, f := range []*fakeProvider{a, b} {
		if len(f.created)+len(f.updated)+len(f.deleted) != 0 {
			t.Errorf("provider %s received writes: %+v %+v %+v", f.name, f.created, f.updated, f.deleted)
		}
	}
	if len(store.defaults) != 0 {
		t.Errorf("defaults saved: %+v", store.defaults)
	}
}

func TestSaveDraft(t *testing.T) {
	date := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)

	t.Run("create remembers defaults", func(t *testing.T) {
		f := newFake("a")
		s, store := newTestSession(t, f)
		sc := loaded(t, s)
		tags := []provider.Tag{{ID: "t1", Name: "Billable"}}
		d := EntryDraft{ProjectID: "p2", Date: date, FromMinutes: 540, ToMinutes: 600, Tags: []string{"billable"}}
		saved, err := sc.SaveDraft(context.Background(), d, tags)
		if err != nil {
			t.Fatalf("SaveDraft: %v", err)
		}
		if saved.ID != "new" || len(f.created) != 1 || f.created[0].MemberID != "m-a" {
			t.Fatalf("unexpected create: %+v", f.created)
		}
		got := s.LastDefaults()
		if got.ProjectID != "p2" || len(got.TagIDs) != 1 || got.TagIDs[0] != "t1" {
			t.Errorf("LastDefaults = %+v", got)
		}
		if store.defaults["a"].ProjectID != "p2" {
			t.Errorf("defaults not persisted: %+v", store.defaults)
		}
	})

	t.Run("edit updates and keeps defaults", func(t *testing.T) {
		f := newFake("a")
		s, store := newTestSession(t, f)
		sc := loaded(t, s)
		d := EntryDraft{ID: "42", ProjectID: "p1", Date: date, FromMinutes: 60, ToMinutes: 120}
		if _, err := sc.SaveDraft(context.Background(), d, nil); err != nil {
			t.Fatalf("SaveDraft: %v", err)
		}
		if len(f.updated) != 1 || len(f.created) != 0 {
			t.Fatalf("created=%d updated=%d", len(f.created), len(f.updated))
		}
		if _, ok := store.defaults["a"]; ok {
			t.Error("editing should not overwrite last-entry defaults")
		}
	})

	t.Run("requires profile", func(t *testing.T) {
		s, _ := newTestSession(t, newFake("a"))
		if _, err := s.Current().SaveDraft(context.Background(), EntryDraft{}, nil); !errors.Is(err, ErrNoProfile) {
			t.Fatalf("err = %v, want ErrNoProfile", err)
		}
	})

	t.Run("validation error is returned before calling provider", func(t *testing.T) {
		f := newFake("a")
		s, _ := newTestSession(t, f)
		sc := loaded(t, s)
		d := EntryDraft{ProjectID: "p1", Date: date, FromMinutes: 600, ToMinutes: 540}
		if _, err := sc.SaveDraft(context.Background(), d, nil); !errors.Is(err, ErrEndNotAfter) {
			t.Fatalf("err = %v, want ErrEndNotAfter", err)
		}
		if len(f.created) != 0 {
			t.Error("provider was called despite validation failure")
		}
	})

	t.Run("provider error propagates", func(t *testing.T) {
		f := newFake("a")
		f.saveErr = errors.New("boom")
		s, store := newTestSession(t, f)
		sc := loaded(t, s)
		d := EntryDraft{ProjectID: "p1", Date: date, FromMinutes: 60, ToMinutes: 120}
		if _, err := sc.SaveDraft(context.Background(), d, nil); err == nil {
			t.Fatal("expected error")
		}
		if _, ok := store.defaults["a"]; ok {
			t.Error("failed create should not save defaults")
		}
	})
}

func TestDeleteEntry(t *testing.T) {
	f := newFake("a")
	s, _ := newTestSession(t, f)
	if err := s.Current().DeleteEntry(context.Background(), provider.TimeEntry{ID: "9"}); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 1 || f.deleted[0].ID != "9" {
		t.Errorf("deleted = %+v", f.deleted)
	}
}

// TestConcurrentUse hammers a Session from many goroutines, mixing provider
// switches with reads and writes, for the race detector to check, and
// asserts every write landed on the provider whose member it was built for.
func TestConcurrentUse(t *testing.T) {
	a, b := newFake("a"), newFake("b")
	s, _ := newTestSession(t, a, b)
	date := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 50 {
				switch (i + j) % 4 {
				case 0:
					_ = s.SwitchProvider([]string{"a", "b"}[j%2])
				case 1:
					sc, err := s.Current().LoadProfile(context.Background())
					if err == nil {
						_, _ = sc.ListEntries(context.Background(), date)
					}
				case 2:
					sc := s.Current()
					d := EntryDraft{ProjectID: "p1", Date: date, FromMinutes: 60, ToMinutes: 120}
					_, _ = sc.SaveDraft(context.Background(), d, nil)
				default:
					_ = s.Projects()
					_ = s.LastDefaults()
					_, _ = s.Member()
					_ = s.Attendance()
				}
			}
		})
	}
	wg.Wait()

	for _, f := range []*fakeProvider{a, b} {
		for _, e := range f.created {
			if e.MemberID != "m-"+f.name {
				t.Errorf("provider %s received entry for member %s", f.name, e.MemberID)
			}
		}
	}
}

func TestAttendance(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		s, _ := newTestSession(t, newFake("a"))
		sc := s.Current()
		if s.Attendance() != nil || sc.Attendance() != nil {
			t.Error("Attendance() should be nil when disabled")
		}
		if _, err := sc.AttendanceStatus(context.Background()); !errors.Is(err, ErrAttendanceDisabled) {
			t.Errorf("err = %v, want ErrAttendanceDisabled", err)
		}
		if _, err := sc.CheckIn(context.Background(), "x", ""); !errors.Is(err, ErrAttendanceDisabled) {
			t.Errorf("err = %v, want ErrAttendanceDisabled", err)
		}
	})

	t.Run("check-in options preselect last location", func(t *testing.T) {
		f := newFake("a")
		f.attendanceEnabled = true
		f.locations = []provider.WorkLocation{{ID: "office"}, {ID: "home"}}
		f.lastLocationID = "home"
		s, _ := newTestSession(t, f)
		opts, err := s.Current().CheckInOptions(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if opts.SelectedIndex != 1 || len(opts.Locations) != 2 {
			t.Errorf("opts = %+v", opts)
		}
	})

	t.Run("no locations", func(t *testing.T) {
		f := newFake("a")
		f.attendanceEnabled = true
		s, _ := newTestSession(t, f)
		if _, err := s.Current().CheckInOptions(context.Background()); !errors.Is(err, ErrNoWorkLocations) {
			t.Errorf("err = %v, want ErrNoWorkLocations", err)
		}
	})

	t.Run("check in trims note, check out", func(t *testing.T) {
		f := newFake("a")
		f.attendanceEnabled = true
		s, _ := newTestSession(t, f)
		sc := s.Current()
		sess, err := sc.CheckIn(context.Background(), "office", "  hi  ")
		if err != nil {
			t.Fatal(err)
		}
		if f.checkInNote != "hi" || sess.WorkLocationID != "office" {
			t.Errorf("note=%q session=%+v", f.checkInNote, sess)
		}
		out, err := sc.CheckOut(context.Background(), sess, "")
		if err != nil || out.Open() {
			t.Errorf("CheckOut: %+v %v", out, err)
		}
	})

	t.Run("stale scope refuses check-in", func(t *testing.T) {
		a, b := newFake("a"), newFake("b")
		a.attendanceEnabled = true
		s, _ := newTestSession(t, a, b)
		old := s.Current()
		if err := s.SwitchProvider("b"); err != nil {
			t.Fatal(err)
		}
		if _, err := old.CheckIn(context.Background(), "office", ""); !errors.Is(err, ErrStale) {
			t.Errorf("err = %v, want ErrStale", err)
		}
	})
}

func TestConnect(t *testing.T) {
	t.Run("logs in when needed then loads profile", func(t *testing.T) {
		f := newFake("a")
		f.authenticated = false
		s, _ := newTestSession(t, f)
		sc, err := s.Current().Connect(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := sc.Member(); !ok || f.loginCalls.Load() != 1 {
			t.Errorf("member loaded=%v logins=%d", ok, f.loginCalls.Load())
		}
	})

	t.Run("expired saved login triggers one fresh login", func(t *testing.T) {
		f := newFake("a") // looks authenticated...
		f.expireOnce = true
		f.prompt = &provider.LoginPrompt{Code: "NEW"}
		s, _ := newTestSession(t, f)
		var prompted string
		sc, err := s.Current().Connect(context.Background(), func(p provider.LoginPrompt) { prompted = p.Code })
		if err != nil {
			t.Fatalf("Connect: %v", err)
		}
		if prompted != "NEW" || f.loginCalls.Load() != 1 {
			t.Errorf("prompted=%q logins=%d, want one fresh login", prompted, f.loginCalls.Load())
		}
		if _, ok := sc.Member(); !ok || !s.HasProfile() {
			t.Error("profile not loaded after re-login")
		}
	})

	t.Run("failed re-login is returned", func(t *testing.T) {
		f := newFake("a")
		f.expireOnce = true
		f.loginErr = errors.New("denied")
		s, _ := newTestSession(t, f)
		if _, err := s.Current().Connect(context.Background(), nil); err == nil || err.Error() != "denied" {
			t.Errorf("err = %v, want denied", err)
		}
	})
}

type versionedFake struct {
	*fakeProvider
	version string
}

func (v versionedFake) BackendVersion(context.Context) (string, error) { return v.version, nil }

func TestBackendVersion(t *testing.T) {
	s, err := NewSession(map[string]provider.Provider{
		"v": versionedFake{newFake("v"), "4.2.1"},
		"p": newFake("p"),
	}, []string{"p", "v"}, "v", newMemStore())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Current().BackendVersion(context.Background()); err != nil || got != "4.2.1" {
		t.Errorf("versioned: %q, %v", got, err)
	}
	if err := s.SwitchProvider("p"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Current().BackendVersion(context.Background()); err != nil || got != "" {
		t.Errorf("unversioned: %q, %v", got, err)
	}
}

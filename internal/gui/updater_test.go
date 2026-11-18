package gui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/update"
)

// updaterRun drives runUpdater with a fake clock: each wait advances the
// clock by step (ticks times in all), and results feeds the checks in order
// (then "up to date"). It all runs on the test's goroutine.
type updaterRun struct {
	now     time.Time
	checks  []time.Time
	results []update.CheckResult
}

func (r *updaterRun) start(t *testing.T, svc *Service, ticks int, step time.Duration) {
	t.Helper()
	runUpdater(context.Background(), svc, func(context.Context) update.CheckResult {
		r.checks = append(r.checks, r.now)
		if len(r.results) == 0 {
			return update.CheckResult{}
		}
		res := r.results[0]
		r.results = r.results[1:]
		return res
	}, func() bool {
		if ticks == 0 {
			return false
		}
		ticks--
		r.now = r.now.Add(step)
		return true
	}, func() time.Time { return r.now })
}

func TestUpdaterChecksDaily(t *testing.T) {
	r := &updaterRun{now: time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)}
	// Hourly for just under 3 days.
	r.start(t, &Service{host: newFakeHost()}, 71, time.Hour)
	if len(r.checks) != 3 {
		t.Fatalf("checks = %v, want one at start and one a day after", r.checks)
	}
	if gap := r.checks[1].Sub(r.checks[0]); gap != 24*time.Hour {
		t.Errorf("gap between checks = %v, want 24h", gap)
	}
}

func TestUpdaterRetriesSoonAfterFailure(t *testing.T) {
	r := &updaterRun{
		now:     time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC),
		results: []update.CheckResult{{Failed: true}},
	}
	r.start(t, &Service{host: newFakeHost()}, 3, 30*time.Minute)
	if len(r.checks) != 2 || r.checks[1].Sub(r.checks[0]) != time.Hour {
		t.Fatalf("checks = %v, want a retry an hour after the failure", r.checks)
	}
}

func TestUpdaterStopsOnceInstalled(t *testing.T) {
	host := newFakeHost()
	svc := &Service{host: host, restart: func() error { return nil }}
	r := &updaterRun{
		now:     time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC),
		results: []update.CheckResult{{Installed: true, NewVersion: "v1.4.0"}},
	}
	r.start(t, svc, 100, time.Hour)
	if len(r.checks) != 1 {
		t.Fatalf("checked %d times, want to stop after installing", len(r.checks))
	}
	st := svc.State().Update
	if st.Ready != "v1.4.0" || !st.CanRestart || st.Notice == "" {
		t.Errorf("update state = %+v", st)
	}
	if ev := host.events[EventUpdate]; len(ev) != 1 || ev[0].(UpdateStatus).Ready != "v1.4.0" {
		t.Errorf("update events = %v", ev)
	}
}

func TestRestartToUpdate(t *testing.T) {
	svc := &Service{host: newFakeHost()}
	if err := svc.RestartToUpdate(); err == nil {
		t.Error("restart with no update waiting: want an error")
	}
	svc.updateReady = "v1.4.0"
	if err := svc.RestartToUpdate(); err == nil {
		t.Error("restart without a restart hook: want an error")
	}
	called := false
	svc.restart = func() error { called = true; return nil }
	if err := svc.RestartToUpdate(); err != nil || !called {
		t.Errorf("RestartToUpdate = %v, called %v", err, called)
	}
	svc.restart = func() error { return errors.New("boom") }
	if err := svc.RestartToUpdate(); err == nil {
		t.Error("want the restart error passed on")
	}
}

func TestUpdaterOffersPackageDownload(t *testing.T) {
	host := newFakeHost()
	svc := &Service{host: host, restart: func() error { return nil }}
	r := &updaterRun{
		now: time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC),
		results: []update.CheckResult{
			{Available: "v1.4.0", DownloadURL: "https://example.org/lazytrackit-desktop_1.4.0_amd64.deb"},
			{Installed: true, NewVersion: "v1.4.0"},
		},
	}
	r.start(t, svc, 0, time.Hour)
	st := svc.State().Update
	if !st.Download || st.CanRestart || st.Notice == "" {
		t.Errorf("after an available package: %+v", st)
	}
	if err := svc.DownloadUpdate(); err != nil || len(host.opened) != 1 || host.opened[0] != "https://example.org/lazytrackit-desktop_1.4.0_amd64.deb" {
		t.Errorf("DownloadUpdate: err %v, opened %v", err, host.opened)
	}

	// Once the package is installed, the next check offers the restart.
	r.start(t, svc, 0, time.Hour)
	st = svc.State().Update
	if st.Download || !st.CanRestart || st.Ready != "v1.4.0" {
		t.Errorf("after installing the package: %+v", st)
	}
}

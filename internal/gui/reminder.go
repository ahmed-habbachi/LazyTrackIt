package gui

import (
	"context"
	"sync"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
	"github.com/ahmed-habbachi/lazytrackit/internal/service"
)

// Check-in reminders: on weekdays, if you haven't checked in, the app shows
// a desktop notification
//   - at the configured time (check_in_reminder), then every repeat until
//     cutoff, and
//   - shortly after the app starts or the computer wakes from sleep, during
//     working hours (wakeFrom–wakeUntil).
//
// Once the app has seen you checked in on a day, it stays quiet for the
// rest of that day, even after you check out (e.g. for lunch).

// reminderPolicy holds the rules; it's pure so tests can drive it.
type reminderPolicy struct {
	at        time.Duration // time of day of the first fixed reminder
	repeat    time.Duration // gap between reminders
	cutoff    time.Duration // no fixed reminders from this time of day on
	wakeDelay time.Duration // wait after start/wake before checking
	wakeFrom  time.Duration // start/wake reminders only in [wakeFrom, wakeUntil)
	wakeUntil time.Duration
}

func defaultReminderPolicy(at time.Duration) reminderPolicy {
	return reminderPolicy{
		at:        at,
		repeat:    30 * time.Minute,
		cutoff:    12 * time.Hour,
		wakeDelay: 2 * time.Minute,
		wakeFrom:  7 * time.Hour,
		wakeUntil: 17 * time.Hour,
	}
}

// reminderState is what the policy remembers between ticks.
type reminderState struct {
	day          string    // the date the fields below are for
	checkedIn    bool      // seen checked in today: no more reminders
	lastReminder time.Time // last reminder shown (or check made) today
	wakeAt       time.Time // pending start/wake check; zero = none
}

func timeOfDay(t time.Time) time.Duration {
	y, m, d := t.Date()
	return t.Sub(time.Date(y, m, d, 0, 0, 0, 0, t.Location()))
}

func isWeekday(t time.Time) bool {
	wd := t.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

// rollover resets the per-day fields when the date changes. A pending
// start/wake check survives: waking up just after midnight still counts.
func (st *reminderState) rollover(now time.Time) {
	if day := now.Format(time.DateOnly); st.day != day {
		*st = reminderState{day: day, wakeAt: st.wakeAt}
	}
}

// armWake schedules a check after the app starts or the computer wakes.
func (p reminderPolicy) armWake(now time.Time, st *reminderState) {
	st.rollover(now)
	st.wakeAt = now.Add(p.wakeDelay)
}

// due reports whether it's time to check the attendance status (and remind
// if still checked out). It consumes a pending start/wake check.
func (p reminderPolicy) due(now time.Time, st *reminderState) bool {
	st.rollover(now)
	wake := !st.wakeAt.IsZero() && !now.Before(st.wakeAt)
	if wake {
		st.wakeAt = time.Time{}
	}
	if st.checkedIn || !isWeekday(now) {
		return false
	}
	tod := timeOfDay(now)
	if wake && tod >= p.wakeFrom && tod < p.wakeUntil {
		return true
	}
	if tod < p.at || tod >= p.cutoff {
		return false
	}
	return st.lastReminder.IsZero() || now.Sub(st.lastReminder) >= p.repeat
}

// reminderNotifier shows and clears the reminder notification.
type reminderNotifier interface {
	ShowCheckInReminder()
	ClearCheckInReminder()
}

// reminder runs the policy against the live attendance state.
type reminder struct {
	svc    *Service
	policy reminderPolicy
	notify reminderNotifier
	tick   time.Duration

	mu    sync.Mutex
	state reminderState
}

func newReminder(svc *Service, at time.Duration, notify reminderNotifier) *reminder {
	return &reminder{svc: svc, policy: defaultReminderPolicy(at), notify: notify, tick: time.Minute}
}

// observe is called with every attendance update: seeing an open session
// silences reminders for the day and clears a reminder still on screen.
func (r *reminder) observe(open *provider.AttendanceSession) {
	if open == nil {
		return
	}
	r.mu.Lock()
	r.state.rollover(time.Now())
	already := r.state.checkedIn
	r.state.checkedIn = true
	r.mu.Unlock()
	if !already {
		r.notify.ClearCheckInReminder()
	}
}

// run checks once a minute until ctx ends. A wall-clock jump much larger
// than the tick means the computer was asleep, which arms a wake check.
func (r *reminder) run(ctx context.Context) {
	r.mu.Lock()
	r.policy.armWake(time.Now(), &r.state)
	r.mu.Unlock()

	ticker := time.NewTicker(r.tick)
	defer ticker.Stop()
	last := time.Now().Round(0) // wall clock: keeps counting during sleep
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		now := time.Now().Round(0)
		r.mu.Lock()
		if now.Sub(last) > 3*r.tick {
			r.policy.armWake(now, &r.state)
		}
		last = now
		due := r.policy.due(now, &r.state)
		if due {
			// Count the check itself, so a failing status request doesn't
			// retry every minute.
			r.state.lastReminder = now
		}
		r.mu.Unlock()
		if due {
			r.check(ctx)
		}
	}
}

// check fetches the current status (never starting an interactive login)
// and shows the reminder if still checked out.
func (r *reminder) check(ctx context.Context) {
	sc, err := r.svc.scope()
	if err != nil || sc.Attendance() == nil || sc.NeedsLogin() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, service.RequestTimeout)
	defer cancel()
	if _, ok := sc.Member(); !ok {
		if sc, err = sc.LoadProfile(ctx); err != nil {
			return
		}
	}
	open, err := r.svc.fetchAttendance(ctx, sc)
	if err != nil {
		return
	}
	if open != nil {
		r.observe(open)
		return
	}
	r.notify.ShowCheckInReminder()
}

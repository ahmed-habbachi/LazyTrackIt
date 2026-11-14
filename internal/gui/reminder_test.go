package gui

import (
	"testing"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// Friday 2026-11-13 and Saturday 2026-11-14.
func at(day, hh, mm int) time.Time { return time.Date(2026, 11, day, hh, mm, 0, 0, time.Local) }

func TestReminderFixedTimeRepeatsUntilCutoff(t *testing.T) {
	p := defaultReminderPolicy(9*time.Hour + 30*time.Minute)
	var st reminderState
	steps := []struct {
		now  time.Time
		want bool
	}{
		{at(13, 9, 0), false},  // before the reminder time
		{at(13, 9, 30), true},  // reminder time
		{at(13, 9, 45), false}, // within the repeat gap
		{at(13, 10, 0), true},  // 30 min later
		{at(13, 11, 59), true}, // still before noon
		{at(13, 12, 0), false}, // cutoff
		{at(13, 15, 0), false}, // afternoon
	}
	for _, s := range steps {
		got := p.due(s.now, &st)
		if got != s.want {
			t.Errorf("due(%s) = %v, want %v", s.now.Format("15:04"), got, s.want)
		}
		if got {
			st.lastReminder = s.now
		}
	}
}

func TestReminderSkipsWeekends(t *testing.T) {
	p := defaultReminderPolicy(9*time.Hour + 30*time.Minute)
	var st reminderState
	if p.due(at(14, 9, 30), &st) {
		t.Error("fixed reminder fired on a Saturday")
	}
	p.armWake(at(14, 10, 0), &st)
	if p.due(at(14, 10, 5), &st) {
		t.Error("wake reminder fired on a Saturday")
	}
}

func TestReminderWakeCheck(t *testing.T) {
	p := defaultReminderPolicy(9*time.Hour + 30*time.Minute)
	var st reminderState
	st.lastReminder = at(13, 14, 0) // a reminder earlier doesn't matter
	p.armWake(at(13, 14, 10), &st)
	if p.due(at(13, 14, 11), &st) {
		t.Error("wake check fired before its delay")
	}
	if !p.due(at(13, 14, 12), &st) {
		t.Error("wake check didn't fire in working hours")
	}
	if p.due(at(13, 14, 13), &st) {
		t.Error("wake check fired twice")
	}

	// Outside working hours it's dropped, not deferred.
	p.armWake(at(13, 20, 0), &st)
	if p.due(at(13, 20, 2), &st) {
		t.Error("wake check fired in the evening")
	}
	if !st.wakeAt.IsZero() {
		t.Error("evening wake check stayed pending")
	}
}

func TestReminderQuietOnceCheckedIn(t *testing.T) {
	p := defaultReminderPolicy(9*time.Hour + 30*time.Minute)
	st := reminderState{day: at(13, 0, 0).Format(time.DateOnly), checkedIn: true}
	if p.due(at(13, 9, 30), &st) {
		t.Error("reminded after checking in")
	}
	p.armWake(at(13, 13, 0), &st) // back from lunch, checked out meanwhile
	if p.due(at(13, 13, 5), &st) {
		t.Error("wake reminder after checking in earlier today")
	}
	// The next day starts fresh. Monday 2026-11-16:
	if !p.due(time.Date(2026, 11, 16, 9, 30, 0, 0, time.Local), &st) {
		t.Error("checked-in flag carried over to the next day")
	}
}

type fakeNotifier struct{ shown, cleared int }

func (f *fakeNotifier) ShowCheckInReminder()  { f.shown++ }
func (f *fakeNotifier) ClearCheckInReminder() { f.cleared++ }

func TestReminderObserveClearsOnCheckIn(t *testing.T) {
	n := &fakeNotifier{}
	r := newReminder(nil, 9*time.Hour, n)
	r.observe(nil)
	if n.cleared != 0 {
		t.Error("cleared on a checked-out update")
	}
	r.observe(&provider.AttendanceSession{})
	r.observe(&provider.AttendanceSession{})
	if n.cleared != 1 || !r.state.checkedIn {
		t.Errorf("after check-in: cleared=%d checkedIn=%v, want 1 true", n.cleared, r.state.checkedIn)
	}
}

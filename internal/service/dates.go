package service

import (
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// DailyTarget is the number of hours a user is expected to log per weekday.
const DailyTarget = 8 * time.Hour

// StartOfDay returns midnight in t's own location. Unlike t.Truncate(24h),
// which truncates the absolute UTC instant, this respects local civil dates
// in timezones not aligned to UTC midnight.
func StartOfDay(t time.Time) time.Time {
	y, mo, d := t.Date()
	return time.Date(y, mo, d, 0, 0, 0, 0, t.Location())
}

// StartOfWeek returns midnight of the Monday of t's week, in t's location.
func StartOfWeek(t time.Time) time.Time {
	t = StartOfDay(t)
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset)
}

// WeekEnd returns the last day (Sunday) of the week starting at weekStart.
func WeekEnd(weekStart time.Time) time.Time {
	return weekStart.AddDate(0, 0, 6)
}

// MinutesOfDay returns t's time-of-day as minutes after midnight.
func MinutesOfDay(t time.Time) int {
	return t.Hour()*60 + t.Minute()
}

// CalendarDate normalizes t to midnight UTC, regardless of t's own location.
// An entry's date only ever represents a calendar date (year/month/day),
// never a real instant, so it must stay in one fixed location: otherwise
// the date sent to a provider would carry whatever non-UTC offset t happened
// to have (e.g. Local), which can shift the calendar date by a day once a
// provider formats it with that offset and the server normalizes it back to
// UTC.
func CalendarDate(t time.Time) time.Time {
	y, mo, d := t.Date()
	return time.Date(y, mo, d, 0, 0, 0, 0, time.UTC)
}

// DefaultEntryDate picks "today" when the week starting at weekStart
// contains now, and that week's Monday otherwise, so a new entry's default
// date always falls within the week the user is looking at.
func DefaultEntryDate(weekStart, now time.Time) time.Time {
	today := StartOfDay(now)
	if !today.Before(weekStart) && !today.After(WeekEnd(weekStart)) {
		return today
	}
	return weekStart
}

// LatestEntryEndMinutes returns the latest End time-of-day, in minutes,
// among entries that fall on date, so a new entry can default to starting
// right after it instead of landing on an already-booked slot. ok is false
// if date has no entries yet.
func LatestEntryEndMinutes(entries []provider.TimeEntry, date time.Time) (minutes int, ok bool) {
	for _, e := range entries {
		if !StartOfDay(e.End).Equal(date) {
			continue
		}
		if end := MinutesOfDay(e.End); !ok || end > minutes {
			minutes = end
			ok = true
		}
	}
	return minutes, ok
}

// TotalDuration sums the durations of entries.
func TotalDuration(entries []provider.TimeEntry) time.Duration {
	var total time.Duration
	for _, e := range entries {
		total += e.Duration()
	}
	return total
}

// ExpectedWeekDuration returns the cumulated hours a user is expected to
// have logged for weekStart's week by now: DailyTarget for every Mon-Fri up
// to and including today, capped to the full week once today is past it and
// zero once the week hasn't started yet. This lets a week view show how far
// ahead or behind the real logged hours are, instead of only the week's
// final 40h target which isn't meaningful until the week is over.
func ExpectedWeekDuration(weekStart, now time.Time) time.Duration {
	weekEnd := WeekEnd(weekStart)
	cursor := StartOfDay(now)
	if cursor.Before(weekStart) {
		return 0
	}
	if cursor.After(weekEnd) {
		cursor = weekEnd
	}
	var days int
	for d := weekStart; !d.After(cursor); d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			days++
		}
	}
	return time.Duration(days) * DailyTarget
}

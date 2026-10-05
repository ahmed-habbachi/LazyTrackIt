package weekbooking

import "time"

// isoWeekMonday returns the Monday (at midnight, local time) of ISO week
// `week` in ISO year `year`. The backend only ever gives us (year, week,
// weekday) triples, never a timestamp, so this is how entry dates are
// reconstructed independent of whatever `from`/`to` a caller happened to ask
// for.
//
// ISO 8601 defines week 1 of a year as the week containing that year's
// January 4th, so finding week 1's Monday and stepping forward (week-1)*7
// days lands on the Monday of any other week in the same ISO year.
func isoWeekMonday(year, week int) time.Time {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.Local)
	daysSinceMonday := (int(jan4.Weekday()) + 6) % 7 // Sunday=0 -> 6, Monday=1 -> 0, ...
	week1Monday := jan4.AddDate(0, 0, -daysSinceMonday)
	return week1Monday.AddDate(0, 0, (week-1)*7)
}

// isoWeekdayDate returns the date (midnight, local time) of the given ISO
// weekday (1=Mon...7=Sun) within ISO (year, week).
func isoWeekdayDate(year, week, weekday int) time.Time {
	return isoWeekMonday(year, week).AddDate(0, 0, weekday-1)
}

// toISOWeekday converts a time.Time into the spec's 1=Mon...7=Sun weekday
// numbering (time.Weekday uses 0=Sun...6=Sat).
func toISOWeekday(t time.Time) int {
	wd := int(t.Weekday())
	if wd == 0 {
		return 7
	}
	return wd
}

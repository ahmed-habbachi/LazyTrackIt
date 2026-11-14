package config

import (
	"testing"
	"time"
)

func TestCheckInReminderTime(t *testing.T) {
	cases := []struct {
		in      string
		at      time.Duration
		ok, bad bool
	}{
		{"", DefaultCheckInReminder, true, false},
		{"08:45", 8*time.Hour + 45*time.Minute, true, false},
		{" 10:00 ", 10 * time.Hour, true, false},
		{"off", 0, false, false},
		{"OFF", 0, false, false},
		{"false", 0, false, false},
		{"9.30", 0, false, true},
		{"25:00", 0, false, true},
	}
	for _, c := range cases {
		cfg := Config{CheckInReminder: c.in}
		at, ok, err := cfg.CheckInReminderTime()
		if (err != nil) != c.bad || ok != c.ok || at != c.at {
			t.Errorf("%q: got (%v, %v, %v), want (%v, %v, bad=%v)", c.in, at, ok, err, c.at, c.ok, c.bad)
		}
	}
}

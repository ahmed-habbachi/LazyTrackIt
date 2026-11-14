package service

import (
	"reflect"
	"testing"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

func TestStartOfWeek(t *testing.T) {
	cases := []struct {
		in   time.Time
		want time.Time
	}{
		{time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC), time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)},  // Monday
		{time.Date(2026, 11, 15, 23, 0, 0, 0, time.UTC), time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)}, // Sunday
		{time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)},  // Thursday
		{time.Date(2027, 1, 1, 12, 0, 0, 0, time.UTC), time.Date(2026, 12, 28, 0, 0, 0, 0, time.UTC)},  // across year
	}
	for _, c := range cases {
		if got := StartOfWeek(c.in); !got.Equal(c.want) {
			t.Errorf("StartOfWeek(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestExpectedWeekDuration(t *testing.T) {
	weekStart := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		now  time.Time
		want time.Duration
	}{
		{"before week", weekStart.AddDate(0, 0, -1), 0},
		{"monday", weekStart.Add(9 * time.Hour), 8 * time.Hour},
		{"wednesday", weekStart.AddDate(0, 0, 2), 24 * time.Hour},
		{"saturday", weekStart.AddDate(0, 0, 5), 40 * time.Hour},
		{"after week", weekStart.AddDate(0, 0, 20), 40 * time.Hour},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ExpectedWeekDuration(weekStart, c.now); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestTotalDuration(t *testing.T) {
	base := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
	entries := []provider.TimeEntry{
		{Start: base, End: base.Add(time.Hour)},
		{Start: base, End: base.Add(30 * time.Minute)},
	}
	if got := TotalDuration(entries); got != 90*time.Minute {
		t.Errorf("TotalDuration = %v", got)
	}
}

func TestParseList(t *testing.T) {
	cases := map[string][]string{
		"":              nil,
		"   ":           nil,
		"a":             {"a"},
		" a , b ,, c ,": {"a", "b", "c"},
	}
	for in, want := range cases {
		if got := ParseList(in); !reflect.DeepEqual(got, want) {
			t.Errorf("ParseList(%q) = %#v, want %#v", in, got, want)
		}
	}
	if got := JoinList([]string{"a", "b"}); got != "a,b" {
		t.Errorf("JoinList = %q", got)
	}
}

func TestResolveTagIDs(t *testing.T) {
	tags := []provider.Tag{{ID: "t1", Name: "Billable"}, {ID: "t2", Name: "Remote"}}
	got, err := ResolveTagIDs([]string{"BILLABLE", "t2"}, tags)
	if err != nil || !reflect.DeepEqual(got, []string{"t1", "t2"}) {
		t.Errorf("got %v, %v", got, err)
	}
	if got, err := ResolveTagIDs([]string{"raw"}, nil); err != nil || !reflect.DeepEqual(got, []string{"raw"}) {
		t.Errorf("passthrough got %v, %v", got, err)
	}
}

func TestTagIDsToNames(t *testing.T) {
	tags := []provider.Tag{{ID: "t1", Name: "Billable"}}
	in := []string{"t1", "typed"}
	got, changed := TagIDsToNames(in, tags)
	if !changed || !reflect.DeepEqual(got, []string{"Billable", "typed"}) {
		t.Errorf("got %v, %v", got, changed)
	}
	if in[0] != "t1" {
		t.Error("input was mutated")
	}
	if _, changed := TagIDsToNames(in, nil); changed {
		t.Error("changed with no tags")
	}
}

func TestSplitTagNames(t *testing.T) {
	project, system := SplitTagNames([]provider.Tag{{Name: "a"}, {Name: "b", IsSystem: true}})
	if !reflect.DeepEqual(project, []string{"a"}) || !reflect.DeepEqual(system, []string{"b"}) {
		t.Errorf("project=%v system=%v", project, system)
	}
}

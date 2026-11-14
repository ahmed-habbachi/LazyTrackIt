package service

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

var testProjects = []provider.Project{{ID: "p1", Name: "Alpha"}, {ID: "p2", Name: "Beta"}}

func TestBuildEntry(t *testing.T) {
	date := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	tags := []provider.Tag{{ID: "t1", Name: "Billable"}, {ID: "t2", Name: "Remote", IsSystem: true}}

	cases := []struct {
		name     string
		draft    EntryDraft
		projects []provider.Project
		wantErr  error
	}{
		{"no projects", EntryDraft{ProjectID: "p1", FromMinutes: 1, ToMinutes: 2}, nil, ErrNoProjects},
		{"unknown project", EntryDraft{ProjectID: "zz", FromMinutes: 1, ToMinutes: 2}, testProjects, ErrUnknownProj},
		{"end before start", EntryDraft{ProjectID: "p1", FromMinutes: 600, ToMinutes: 540}, testProjects, ErrEndNotAfter},
		{"end equals start", EntryDraft{ProjectID: "p1", FromMinutes: 600, ToMinutes: 600}, testProjects, ErrEndNotAfter},
		{"negative start", EntryDraft{ProjectID: "p1", FromMinutes: -1, ToMinutes: 60}, testProjects, ErrMinuteBounds},
		{"end past midnight", EntryDraft{ProjectID: "p1", FromMinutes: 1380, ToMinutes: 1441}, testProjects, ErrMinuteBounds},
		{"unknown tag", EntryDraft{ProjectID: "p1", FromMinutes: 60, ToMinutes: 120, Tags: []string{"nope"}}, testProjects, ErrUnknownTag},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.draft.Date = date
			_, err := BuildEntry(c.draft, c.projects, "m", tags)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
		})
	}

	t.Run("valid entry ending at midnight", func(t *testing.T) {
		d := EntryDraft{
			ID: "7", ProjectID: "p2", Date: date, FromMinutes: 23 * 60, ToMinutes: 24 * 60,
			Description: "  review  ", Tags: []string{"billable", "t2"},
		}
		got, err := BuildEntry(d, testProjects, "m1", tags)
		if err != nil {
			t.Fatal(err)
		}
		want := provider.TimeEntry{
			ID: "7", ProjectID: "p2", ProjectName: "Beta", MemberID: "m1",
			Start:       date.Add(23 * time.Hour),
			End:         date.Add(24 * time.Hour),
			Description: "review",
			TagIDs:      []string{"t1", "t2"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got  %+v\nwant %+v", got, want)
		}
	})

	t.Run("date is normalized to UTC calendar date", func(t *testing.T) {
		loc := time.FixedZone("UTC+5", 5*3600)
		d := EntryDraft{ProjectID: "p1", Date: time.Date(2026, 11, 12, 2, 0, 0, 0, loc), FromMinutes: 60, ToMinutes: 120}
		got, err := BuildEntry(d, testProjects, "m", nil)
		if err != nil {
			t.Fatal(err)
		}
		if want := time.Date(2026, 11, 12, 1, 0, 0, 0, time.UTC); !got.Start.Equal(want) || got.Start.Location() != time.UTC {
			t.Errorf("Start = %v, want %v", got.Start, want)
		}
	})
}

func TestNewEntryDraft(t *testing.T) {
	weekStart := time.Date(2026, 11, 9, 0, 0, 0, 0, time.Local) // Monday
	now := time.Date(2026, 11, 11, 14, 37, 0, 0, time.Local)    // Wednesday
	defaults := config.LastEntryDefaults{ProjectID: "p2", TagIDs: []string{"t1"}}

	t.Run("no entries today starts at current hour", func(t *testing.T) {
		d := NewEntryDraft(weekStart, now, defaults, nil)
		if d.FromMinutes != 14*60 || d.ToMinutes != 15*60 {
			t.Errorf("from/to = %d/%d", d.FromMinutes, d.ToMinutes)
		}
		if want := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC); !d.Date.Equal(want) {
			t.Errorf("Date = %v, want %v", d.Date, want)
		}
		if d.ProjectID != "p2" || !reflect.DeepEqual(d.Tags, []string{"t1"}) || d.IsEdit() {
			t.Errorf("draft = %+v", d)
		}
	})

	t.Run("starts after latest entry that day", func(t *testing.T) {
		day := time.Date(2026, 11, 11, 0, 0, 0, 0, time.Local)
		entries := []provider.TimeEntry{
			{Start: day.Add(8 * time.Hour), End: day.Add(10*time.Hour + 30*time.Minute)},
			{Start: day.Add(10*time.Hour + 30*time.Minute), End: day.Add(12 * time.Hour)},
			{Start: day.Add(-20 * time.Hour), End: day.Add(-18 * time.Hour)}, // previous day
		}
		d := NewEntryDraft(weekStart, now, defaults, entries)
		if d.FromMinutes != 12*60 || d.ToMinutes != 13*60 {
			t.Errorf("from/to = %d/%d", d.FromMinutes, d.ToMinutes)
		}
	})

	t.Run("end is capped at midnight", func(t *testing.T) {
		day := time.Date(2026, 11, 11, 0, 0, 0, 0, time.Local)
		entries := []provider.TimeEntry{{Start: day.Add(22 * time.Hour), End: day.Add(23*time.Hour + 30*time.Minute)}}
		d := NewEntryDraft(weekStart, now, defaults, entries)
		if d.FromMinutes != 23*60+30 || d.ToMinutes != 24*60 {
			t.Errorf("from/to = %d/%d, want 1410/1440", d.FromMinutes, d.ToMinutes)
		}
		if _, err := BuildEntry(d, testProjects, "m", nil); err != nil {
			t.Errorf("default draft is not valid: %v", err)
		}
	})

	t.Run("other week defaults to its Monday", func(t *testing.T) {
		d := NewEntryDraft(weekStart.AddDate(0, 0, -7), now, defaults, nil)
		if want := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC); !d.Date.Equal(want) {
			t.Errorf("Date = %v, want %v", d.Date, want)
		}
	})

	t.Run("does not alias defaults slice", func(t *testing.T) {
		d := NewEntryDraft(weekStart, now, defaults, nil)
		d.Tags[0] = "x"
		if defaults.TagIDs[0] != "t1" {
			t.Error("draft tags alias defaults")
		}
	})
}

func TestEditEntryDraft(t *testing.T) {
	start := time.Date(2026, 11, 10, 9, 15, 0, 0, time.Local)
	e := provider.TimeEntry{
		ID: "5", ProjectID: "p1", Start: start, End: start.Add(90 * time.Minute),
		Description: "x", TagIDs: []string{"t1"},
	}
	d := EditEntryDraft(e)
	if !d.IsEdit() || d.FromMinutes != 9*60+15 || d.ToMinutes != 10*60+45 || d.ProjectID != "p1" {
		t.Errorf("draft = %+v", d)
	}
	if want := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC); !d.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", d.Date, want)
	}
}

func TestProjectIndex(t *testing.T) {
	if got := ProjectIndex(testProjects, "p2"); got != 1 {
		t.Errorf("ProjectIndex(p2) = %d", got)
	}
	if got := ProjectIndex(testProjects, "missing"); got != 0 {
		t.Errorf("ProjectIndex(missing) = %d", got)
	}
}

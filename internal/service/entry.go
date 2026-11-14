package service

import (
	"errors"
	"strings"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// Validation errors returned by BuildEntry, so a front end can tell them
// apart (e.g. to highlight the offending field).
var (
	ErrNoProjects   = errors.New("no projects available; cannot create a time entry")
	ErrUnknownProj  = errors.New("selected project is not available")
	ErrEndNotAfter  = errors.New("to must be after from")
	ErrMinuteBounds = errors.New("times must fall within the entry's day")
)

// EntryDraft is what a front end's entry form holds before it's turned into
// a provider.TimeEntry: a calendar date plus from/to as minutes after
// midnight, which is how both the TUI and GUI let the user enter times.
type EntryDraft struct {
	// ID is empty for a new entry, or the ID of the entry being edited.
	ID          string
	ProjectID   string
	Date        time.Time
	FromMinutes int
	ToMinutes   int
	Description string
	// Tags are tag names or raw IDs, as typed by the user.
	Tags []string
}

// IsEdit reports whether the draft edits an existing entry.
func (d EntryDraft) IsEdit() bool { return d.ID != "" }

// NewEntryDraft builds the initial draft for a new entry: defaulted to the
// viewed week's current day (see DefaultEntryDate), starting right after the
// latest entry already logged that day (or at the top of the current hour if
// there is none) and lasting one hour (capped at midnight, so the default is
// always a valid entry), against the last-used project/tags.
func NewEntryDraft(weekStart, now time.Time, defaults config.LastEntryDefaults, existing []provider.TimeEntry) EntryDraft {
	date := DefaultEntryDate(weekStart, now)
	from := now.Hour() * 60
	if lastEnd, ok := LatestEntryEndMinutes(existing, date); ok {
		from = lastEnd
	}
	return EntryDraft{
		ProjectID:   defaults.ProjectID,
		Date:        CalendarDate(date),
		FromMinutes: from,
		ToMinutes:   min(from+60, minutesPerDay),
		Tags:        append([]string(nil), defaults.TagIDs...),
	}
}

// EditEntryDraft builds a draft pre-filled from an existing entry.
func EditEntryDraft(e provider.TimeEntry) EntryDraft {
	return EntryDraft{
		ID:          e.ID,
		ProjectID:   e.ProjectID,
		Date:        CalendarDate(StartOfDay(e.Start)),
		FromMinutes: MinutesOfDay(e.Start),
		ToMinutes:   MinutesOfDay(e.End),
		Description: e.Description,
		Tags:        append([]string(nil), e.TagIDs...),
	}
}

// BuildEntry validates d and turns it into the provider.TimeEntry to send.
// availableTags resolves tag names to IDs (see ResolveTagIDs).
func BuildEntry(d EntryDraft, projects []provider.Project, memberID string, availableTags []provider.Tag) (provider.TimeEntry, error) {
	if len(projects) == 0 {
		return provider.TimeEntry{}, ErrNoProjects
	}
	proj, ok := findProject(projects, d.ProjectID)
	if !ok {
		return provider.TimeEntry{}, ErrUnknownProj
	}
	if d.FromMinutes < 0 || d.FromMinutes >= minutesPerDay || d.ToMinutes > minutesPerDay {
		return provider.TimeEntry{}, ErrMinuteBounds
	}
	if d.ToMinutes <= d.FromMinutes {
		return provider.TimeEntry{}, ErrEndNotAfter
	}
	tagIDs, err := ResolveTagIDs(d.Tags, availableTags)
	if err != nil {
		return provider.TimeEntry{}, err
	}

	date := CalendarDate(d.Date)
	return provider.TimeEntry{
		ID:          d.ID,
		ProjectID:   proj.ID,
		ProjectName: proj.Name,
		MemberID:    memberID,
		Start:       date.Add(time.Duration(d.FromMinutes) * time.Minute),
		End:         date.Add(time.Duration(d.ToMinutes) * time.Minute),
		Description: strings.TrimSpace(d.Description),
		TagIDs:      tagIDs,
	}, nil
}

// minutesPerDay bounds an entry's times: it may start at any minute of its
// day and end as late as midnight at the end of it.
const minutesPerDay = 24 * 60

func findProject(projects []provider.Project, id string) (provider.Project, bool) {
	for _, p := range projects {
		if p.ID == id {
			return p, true
		}
	}
	return provider.Project{}, false
}

// ProjectIndex returns the index of projectID within projects, or 0 if it
// isn't there (so a form falls back to the first project).
func ProjectIndex(projects []provider.Project, projectID string) int {
	for i, p := range projects {
		if p.ID == projectID {
			return i
		}
	}
	return 0
}

package gui

import (
	"fmt"
	"regexp"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
	"github.com/ahmed-habbachi/lazytrackit/internal/service"
)

// dateLayout is how calendar dates cross the Go/JS boundary. Plain
// "YYYY-MM-DD" strings avoid any timezone reinterpretation by the browser's
// Date parsing, which would otherwise shift dates near midnight.
const dateLayout = "2006-01-02"

// State is the app-level status the frontend renders its chrome from.
type State struct {
	Version      string       `json:"version"`
	SetupMessage string       `json:"setupMessage"` // non-empty: no usable config yet
	Welcome      string       `json:"welcome"`      // non-empty: first run, a starter config was written
	Providers    []string     `json:"providers"`
	Active       string       `json:"active"`
	NeedsLogin   bool         `json:"needsLogin"`
	HasProfile   bool         `json:"hasProfile"`
	Member       string       `json:"member"`
	Attendance   bool         `json:"attendance"`
	Autostart    bool         `json:"autostart"`
	Update       UpdateStatus `json:"update"`
}

// UpdateStatus is what the window shows about self-update.
type UpdateStatus struct {
	Notice     string `json:"notice"`     // empty: nothing to show
	Ready      string `json:"ready"`      // installed version awaiting a restart
	CanRestart bool   `json:"canRestart"` // "Restart now" can work
	Download   bool   `json:"download"`   // a newer package can be downloaded (DownloadUpdate)
}

// Project is a provider.Project for the frontend.
type Project struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Client string `json:"client"`
	Color  string `json:"color"`
}

// Tag is a provider.Tag for the frontend.
type Tag struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	IsSystem bool   `json:"isSystem"`
}

// Entry is a provider.TimeEntry for the frontend, with its times split into
// a calendar date and minutes after midnight (the same shape EntryDraft uses).
type Entry struct {
	ID          string   `json:"id"`
	ProjectID   string   `json:"projectId"`
	ProjectName string   `json:"projectName"`
	Date        string   `json:"date"`
	From        int      `json:"from"`
	To          int      `json:"to"`
	Minutes     int      `json:"minutes"`
	Description string   `json:"description"`
	TagIDs      []string `json:"tagIds"`
	Locked      bool     `json:"locked"`
	// Editable is false for locked entries and for entries that run past
	// midnight, which the date + from/to form can't represent.
	Editable bool `json:"editable"`
}

// Week is one week of entries plus the totals the header shows.
type Week struct {
	Start           string  `json:"start"`
	End             string  `json:"end"`
	Today           string  `json:"today"`
	Entries         []Entry `json:"entries"`
	LoggedMinutes   int     `json:"loggedMinutes"`
	ExpectedMinutes int     `json:"expectedMinutes"`
}

// Draft is a service.EntryDraft for the frontend. Tags holds tag IDs (the
// GUI picks tags from a list rather than typing names).
type Draft struct {
	ID          string   `json:"id"`
	ProjectID   string   `json:"projectId"`
	Date        string   `json:"date"`
	From        int      `json:"from"`
	To          int      `json:"to"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// WorkLocation is a provider.WorkLocation for the frontend.
type WorkLocation struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// AttendanceSession is a provider.AttendanceSession for the frontend.
type AttendanceSession struct {
	Location string    `json:"location"`
	CheckIn  time.Time `json:"checkIn"`
	Note     string    `json:"note"`
}

// CheckInOptions is a service.CheckInOptions for the frontend.
type CheckInOptions struct {
	Locations []WorkLocation `json:"locations"`
	Selected  int            `json:"selected"`
}

// hexColor matches the only color format passed on to the frontend, where
// colors end up in CSS: anything else from a provider (e.g. a url(...)) is
// dropped rather than trusted.
var hexColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

func safeColor(c string) string {
	if hexColor.MatchString(c) {
		return c
	}
	return ""
}

func toProjects(ps []provider.Project) []Project {
	out := make([]Project, 0, len(ps))
	for _, p := range ps {
		out = append(out, Project{ID: p.ID, Name: p.Name, Client: p.ClientName, Color: safeColor(p.Color)})
	}
	return out
}

func toTags(ts []provider.Tag) []Tag {
	out := make([]Tag, 0, len(ts))
	for _, t := range ts {
		out = append(out, Tag{ID: t.ID, Name: t.Name, Color: safeColor(t.Color), IsSystem: t.IsSystem})
	}
	return out
}

// toEntry splits e into its start's calendar date and from/to minutes. An
// entry ending exactly at the next midnight gets To = 1440; one running
// further into the next day keeps To = From + duration (> 1440) and is
// marked not Editable.
func toEntry(e provider.TimeEntry) Entry {
	from := service.MinutesOfDay(e.Start)
	startDay := service.StartOfDay(e.Start)
	end := e.End.In(e.Start.Location())
	var to int
	switch {
	case service.StartOfDay(end).Equal(startDay):
		to = service.MinutesOfDay(end)
	case end.Equal(startDay.AddDate(0, 0, 1)):
		to = 24 * 60
	default:
		to = from + int(e.Duration().Minutes())
	}
	return Entry{
		ID:          e.ID,
		ProjectID:   e.ProjectID,
		ProjectName: e.ProjectName,
		Date:        e.Start.Format(dateLayout),
		From:        from,
		To:          to,
		Minutes:     int(e.Duration().Minutes()),
		Description: e.Description,
		TagIDs:      append([]string{}, e.TagIDs...),
		Locked:      e.Locked,
		Editable:    !e.Locked && to <= 24*60,
	}
}

func toDraft(d service.EntryDraft) Draft {
	return Draft{
		ID:          d.ID,
		ProjectID:   d.ProjectID,
		Date:        d.Date.Format(dateLayout),
		From:        d.FromMinutes,
		To:          d.ToMinutes,
		Description: d.Description,
		Tags:        append([]string{}, d.Tags...),
	}
}

func (d Draft) toService() (service.EntryDraft, error) {
	date, err := time.Parse(dateLayout, d.Date)
	if err != nil {
		return service.EntryDraft{}, fmt.Errorf("invalid date %q", d.Date)
	}
	return service.EntryDraft{
		ID:          d.ID,
		ProjectID:   d.ProjectID,
		Date:        service.CalendarDate(date),
		FromMinutes: d.From,
		ToMinutes:   d.To,
		Description: d.Description,
		Tags:        append([]string(nil), d.Tags...),
	}, nil
}

func toSession(s *provider.AttendanceSession) *AttendanceSession {
	if s == nil {
		return nil
	}
	return &AttendanceSession{Location: s.WorkLocationName, CheckIn: s.CheckIn, Note: s.Note}
}

func toCheckInOptions(o service.CheckInOptions) CheckInOptions {
	locs := make([]WorkLocation, 0, len(o.Locations))
	for _, l := range o.Locations {
		locs = append(locs, WorkLocation{ID: l.ID, Name: l.Name, Color: safeColor(l.Color)})
	}
	return CheckInOptions{Locations: locs, Selected: o.SelectedIndex}
}

// parseWeekStart turns the frontend's "YYYY-MM-DD" (any day of the wanted
// week, or "" for the current week) into local midnight of that week's
// Monday, which is what service.Scope.ListEntries expects.
func parseWeekStart(s string, now time.Time) (time.Time, error) {
	if s == "" {
		return service.StartOfWeek(now), nil
	}
	day, err := time.ParseInLocation(dateLayout, s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid week %q", s)
	}
	return service.StartOfWeek(day), nil
}

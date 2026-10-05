package weekbooking

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// entryDto mirrors the week-booking API's Entry schema. Unlike TrackIt/Toggl
// there's no start/end timestamp: an entry is just a decimal-hours duration
// booked against an ISO (year, week, weekday).
type entryDto struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	ProjectID int64   `json:"project_id"`
	Year      int     `json:"year"`
	Week      int     `json:"week"`
	Weekday   int     `json:"weekday"`
	Hours     float64 `json:"hours"`
	Note      string  `json:"note"`
}

// entriesResponse is GET /api/v1/entries's envelope: `{entries: [...]}`.
type entriesResponse struct {
	Entries []entryDto `json:"entries"`
}

// weekStatusClosedDto mirrors GET /api/v1/weeks/status, read here only for
// its `closed` flag (see dtoToEntry's Locked field).
type weekStatusClosedDto struct {
	Closed bool `json:"closed"`
}

func dtoToEntry(d entryDto, locked bool) provider.TimeEntry {
	start := isoWeekdayDate(d.Year, d.Week, d.Weekday)
	return provider.TimeEntry{
		ID:          d.ID,
		ProjectID:   idToString(d.ProjectID),
		MemberID:    d.UserID,
		Start:       start,
		End:         start.Add(hoursToDuration(d.Hours)),
		Description: d.Note,
		Locked:      locked,
	}
}

func hoursToDuration(h float64) time.Duration {
	return time.Duration(h * float64(time.Hour))
}

func durationToHours(d time.Duration) float64 {
	return d.Hours()
}

// weekQuery resolves the ISO (year, week) that a date falls in.
func weekQuery(t time.Time) url.Values {
	year, week := t.ISOWeek()
	return url.Values{
		"year": {fmt.Sprintf("%d", year)},
		"week": {fmt.Sprintf("%d", week)},
	}
}

// ListTimeEntries returns the authenticated user's entries for the ISO week
// that `from` falls in (the UI always calls this with a Monday-to-Sunday
// range for a single week, matching the backend's own year/week model).
// memberID is accepted to satisfy provider.Provider but unused: user_id is
// omitted from the request so the backend resolves it to the caller, and v1
// only ever needs "my own" entries.
func (c *Client) ListTimeEntries(ctx context.Context, memberID string, from, to time.Time) ([]provider.TimeEntry, error) {
	q := weekQuery(from)

	var closed weekStatusClosedDto
	if err := c.doJSON(ctx, "GET", "/api/v1/weeks/status?"+q.Encode(), nil, &closed); err != nil {
		return nil, err
	}

	var resp entriesResponse
	if err := c.doJSON(ctx, "GET", "/api/v1/entries?"+q.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	out := make([]provider.TimeEntry, 0, len(resp.Entries))
	for _, d := range resp.Entries {
		out = append(out, dtoToEntry(d, closed.Closed))
	}
	return out, nil
}

// entryCreateRequest mirrors EntryCreateRequest. user_id is left out: we only
// ever book for the caller (self) in v1.
type entryCreateRequest struct {
	ProjectID int64   `json:"project_id"`
	Year      int     `json:"year"`
	Week      int     `json:"week"`
	Weekday   int     `json:"weekday"`
	Hours     float64 `json:"hours"`
	Note      string  `json:"note"`
}

// CreateTimeEntry creates a new entry from e and returns it as stored. The
// entry's ISO (year, week, weekday) is derived from e.Start, and its hours
// from e.End-e.Start.
func (c *Client) CreateTimeEntry(ctx context.Context, e provider.TimeEntry) (provider.TimeEntry, error) {
	projectID, err := idFromString(e.ProjectID)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	year, week := e.Start.ISOWeek()
	req := entryCreateRequest{
		ProjectID: projectID,
		Year:      year,
		Week:      week,
		Weekday:   toISOWeekday(e.Start),
		Hours:     durationToHours(e.End.Sub(e.Start)),
		Note:      e.Description,
	}
	var dto entryDto
	if err := c.doJSON(ctx, "POST", "/api/v1/entries", req, &dto); err != nil {
		return provider.TimeEntry{}, err
	}
	return dtoToEntry(dto, false), nil
}

// entryUpdateRequest mirrors EntryUpdateRequest. All fields are sent on
// every update (rather than relying on the API's pointer/optional
// semantics), since provider.TimeEntry always carries a complete entry.
// year/week are immutable server-side, so moving e.Start to a different ISO
// week here would silently have no effect on those two fields; the UI only
// ever edits an entry within the week it's currently showing, so this
// doesn't come up in practice.
type entryUpdateRequest struct {
	ProjectID int64   `json:"project_id"`
	Weekday   int     `json:"weekday"`
	Hours     float64 `json:"hours"`
	Note      string  `json:"note"`
}

// UpdateTimeEntry updates an existing entry (matched by e.ID) and returns it
// as stored.
func (c *Client) UpdateTimeEntry(ctx context.Context, e provider.TimeEntry) (provider.TimeEntry, error) {
	projectID, err := idFromString(e.ProjectID)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	req := entryUpdateRequest{
		ProjectID: projectID,
		Weekday:   toISOWeekday(e.Start),
		Hours:     durationToHours(e.End.Sub(e.Start)),
		Note:      e.Description,
	}
	path := "/api/v1/entries/" + url.PathEscape(e.ID)
	var dto entryDto
	if err := c.doJSON(ctx, "PATCH", path, req, &dto); err != nil {
		return provider.TimeEntry{}, err
	}
	return dtoToEntry(dto, false), nil
}

// DeleteTimeEntry removes an entry. Deletes are idempotent server-side: a
// repeat delete of an already-removed entry still returns 204.
func (c *Client) DeleteTimeEntry(ctx context.Context, e provider.TimeEntry) error {
	path := "/api/v1/entries/" + url.PathEscape(e.ID)
	return c.doJSON(ctx, "DELETE", path, nil, nil)
}

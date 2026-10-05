package toggl

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// timeEntryDto mirrors one entry from GET /api/v9/me/time_entries?meta=true.
// The meta=true flag adds project_name, which the base response omits.
type timeEntryDto struct {
	ID          int64      `json:"id"`
	ProjectID   *int64     `json:"project_id"`
	ProjectName string     `json:"project_name"`
	UserID      int64      `json:"user_id"`
	Start       time.Time  `json:"start"`
	Stop        *time.Time `json:"stop"`
	Description string     `json:"description"`
	TagIDs      []int64    `json:"tag_ids"`
}

func dtoToEntry(d timeEntryDto) provider.TimeEntry {
	tagIDs := make([]string, 0, len(d.TagIDs))
	for _, id := range d.TagIDs {
		tagIDs = append(tagIDs, idToString(id))
	}
	var projectID string
	if d.ProjectID != nil {
		projectID = idToString(*d.ProjectID)
	}
	// A running entry (no stop yet) has no defined end; v1 is CRUD-only and
	// has no notion of an in-progress timer, so it's reported as zero-length
	// rather than left with a zero End.
	end := d.Start
	if d.Stop != nil {
		end = *d.Stop
	}
	return provider.TimeEntry{
		ID:          idToString(d.ID),
		ProjectID:   projectID,
		ProjectName: d.ProjectName,
		MemberID:    idToString(d.UserID),
		Start:       d.Start,
		End:         end,
		Description: d.Description,
		TagIDs:      tagIDs,
		// Toggl's public API exposes workspace-level entry locking (tied to
		// billing-period reports) but not a per-entry locked flag here.
		Locked: false,
	}
}

// ListTimeEntries returns the authenticated user's entries whose date falls
// within [from, to]. memberID is accepted to satisfy provider.Provider but
// unused: /me/time_entries always scopes to the token's own user, and v1
// only ever needs "my own" entries.
func (c *Client) ListTimeEntries(ctx context.Context, memberID string, from, to time.Time) ([]provider.TimeEntry, error) {
	q := url.Values{
		"start_date": {from.Format("2006-01-02")},
		"end_date":   {to.Format("2006-01-02")},
		"meta":       {"true"},
	}
	path := "/api/v9/me/time_entries?" + q.Encode()

	var dtos []timeEntryDto
	if err := c.doJSON(ctx, "GET", path, nil, &dtos); err != nil {
		return nil, err
	}
	out := make([]provider.TimeEntry, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, dtoToEntry(d))
	}
	return out, nil
}

type createTimeEntryCommand struct {
	WorkspaceID int64   `json:"workspace_id"`
	ProjectID   *int64  `json:"project_id,omitempty"`
	Start       string  `json:"start"`
	Stop        string  `json:"stop"`
	Duration    int64   `json:"duration"`
	Description string  `json:"description"`
	TagIDs      []int64 `json:"tag_ids,omitempty"`
	CreatedWith string  `json:"created_with"`
}

// CreateTimeEntry creates a new time entry from e and returns it as stored.
func (c *Client) CreateTimeEntry(ctx context.Context, e provider.TimeEntry) (provider.TimeEntry, error) {
	wsID, err := c.currentWorkspaceID(ctx)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	projectID, err := idFromString(e.ProjectID)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	tagIDs, err := idsFromStrings(e.TagIDs)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	cmd := createTimeEntryCommand{
		WorkspaceID: wsID,
		ProjectID:   &projectID,
		Start:       e.Start.Format(time.RFC3339),
		Stop:        e.End.Format(time.RFC3339),
		Duration:    int64(e.End.Sub(e.Start).Seconds()),
		Description: e.Description,
		TagIDs:      tagIDs,
		CreatedWith: "lazytrackit",
	}
	path := fmt.Sprintf("/api/v9/workspaces/%d/time_entries", wsID)
	var dto timeEntryDto
	if err := c.doJSON(ctx, "POST", path, cmd, &dto); err != nil {
		return provider.TimeEntry{}, err
	}
	return dtoToEntry(dto), nil
}

type updateTimeEntryCommand struct {
	WorkspaceID int64   `json:"workspace_id"`
	ProjectID   *int64  `json:"project_id,omitempty"`
	Start       string  `json:"start"`
	Stop        string  `json:"stop"`
	Duration    int64   `json:"duration"`
	Description string  `json:"description"`
	TagIDs      []int64 `json:"tag_ids,omitempty"`
}

// UpdateTimeEntry updates an existing entry (matched by e.ID) and returns it as stored.
func (c *Client) UpdateTimeEntry(ctx context.Context, e provider.TimeEntry) (provider.TimeEntry, error) {
	wsID, err := c.currentWorkspaceID(ctx)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	id, err := idFromString(e.ID)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	projectID, err := idFromString(e.ProjectID)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	tagIDs, err := idsFromStrings(e.TagIDs)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	cmd := updateTimeEntryCommand{
		WorkspaceID: wsID,
		ProjectID:   &projectID,
		Start:       e.Start.Format(time.RFC3339),
		Stop:        e.End.Format(time.RFC3339),
		Duration:    int64(e.End.Sub(e.Start).Seconds()),
		Description: e.Description,
		TagIDs:      tagIDs,
	}
	path := fmt.Sprintf("/api/v9/workspaces/%d/time_entries/%d", wsID, id)
	var dto timeEntryDto
	if err := c.doJSON(ctx, "PUT", path, cmd, &dto); err != nil {
		return provider.TimeEntry{}, err
	}
	return dtoToEntry(dto), nil
}

// DeleteTimeEntry removes an entry.
func (c *Client) DeleteTimeEntry(ctx context.Context, e provider.TimeEntry) error {
	wsID, err := c.currentWorkspaceID(ctx)
	if err != nil {
		return err
	}
	id, err := idFromString(e.ID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/api/v9/workspaces/%d/time_entries/%d", wsID, id)
	return c.doJSON(ctx, "DELETE", path, nil, nil)
}

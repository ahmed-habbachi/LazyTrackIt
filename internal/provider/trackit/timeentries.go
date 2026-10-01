package trackit

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

type timeEntryDto struct {
	ID            int32  `json:"id"`
	ProjectID     int32  `json:"projectId"`
	ProjectName   string `json:"projectName"`
	MemberID      int32  `json:"memberId"`
	Date          string `json:"date"`
	Description   string `json:"description"`
	TimeActual    int32  `json:"timeActual"`
	TimeFrom      int32  `json:"timeFrom"`
	TimeTo        int32  `json:"timeTo"`
	IsLocked      bool   `json:"isLocked"`
	TimeEntryTags []struct {
		ID   int32  `json:"id"`
		Name string `json:"name"`
	} `json:"timeEntryTags"`
}

type timeEntriesVm struct {
	TimeEntries []timeEntryDto `json:"timeEntries"`
	Count       int32          `json:"count"`
}

const dateLayout = "2006-01-02T15:04:05Z07:00"

// parseServerDate accepts both a fully-qualified RFC3339 timestamp and the
// timezone-less "2006-01-02T15:04:05" form .NET's System.Text.Json commonly
// emits for DateTime (as opposed to DateTimeOffset) values.
func parseServerDate(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", s, time.Local); err == nil {
		return t
	}
	return time.Time{}
}

// dateAtMidnight truncates t to midnight in its own location.
func dateAtMidnight(t time.Time) time.Time {
	y, mo, d := t.Date()
	return time.Date(y, mo, d, 0, 0, 0, 0, t.Location())
}

// timeAtSeconds builds the instant `seconds` after midnight on day's date.
func timeAtSeconds(day time.Time, seconds int32) time.Time {
	return dateAtMidnight(day).Add(time.Duration(seconds) * time.Second)
}

// secondsSinceMidnight is timeAtSeconds's inverse: TrackIt's timeFrom/timeTo/
// timeActual wire unit is seconds after midnight on the entry's date
// (confirmed against a live account: a 09:00 entry round-trips as 32400, not
// 540 minutes), so this is what CreateTimeEntry/UpdateTimeEntry send.
func secondsSinceMidnight(t time.Time) int32 {
	return int32(t.Sub(dateAtMidnight(t)).Seconds())
}

// idToString/idFromString convert between the domain model's string IDs and
// TrackIt's wire-level int32 IDs.
func idToString(id int32) string { return strconv.Itoa(int(id)) }

func idFromString(s string) (int32, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q: %w", s, err)
	}
	return int32(n), nil
}

func idsFromStrings(ss []string) ([]int32, error) {
	out := make([]int32, 0, len(ss))
	for _, s := range ss {
		id, err := idFromString(s)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func dtoToEntry(d timeEntryDto) provider.TimeEntry {
	date := parseServerDate(d.Date)
	tagIDs := make([]string, 0, len(d.TimeEntryTags))
	for _, t := range d.TimeEntryTags {
		tagIDs = append(tagIDs, idToString(t.ID))
	}
	return provider.TimeEntry{
		ID:          idToString(d.ID),
		ProjectID:   idToString(d.ProjectID),
		ProjectName: d.ProjectName,
		MemberID:    idToString(d.MemberID),
		Start:       timeAtSeconds(date, d.TimeFrom),
		End:         timeAtSeconds(date, d.TimeTo),
		Description: d.Description,
		TagIDs:      tagIDs,
		Locked:      d.IsLocked,
	}
}

// ListTimeEntries returns the authenticated user's entries whose date falls
// within [from, to]. memberID is accepted to satisfy provider.Provider but
// unused here: TrackIt's GET /api/TimeEntries/{id} route takes the backend's
// string userId (not the int32 member id used elsewhere), and we only ever
// need "my own" entries in v1.
func (c *Client) ListTimeEntries(ctx context.Context, memberID string, from, to time.Time) ([]provider.TimeEntry, error) {
	userID, err := c.currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	q := url.Values{
		"startDate": {from.Format(dateLayout)},
		"endDate":   {to.Format(dateLayout)},
	}
	path := fmt.Sprintf("/api/TimeEntries/%s?%s", url.PathEscape(userID), q.Encode())

	var vm timeEntriesVm
	if err := c.doJSON(ctx, "GET", path, nil, &vm); err != nil {
		return nil, err
	}
	out := make([]provider.TimeEntry, 0, len(vm.TimeEntries))
	for _, d := range vm.TimeEntries {
		out = append(out, dtoToEntry(d))
	}
	return out, nil
}

type createTimeEntryCommand struct {
	ProjectID      int32   `json:"projectId"`
	UserID         string  `json:"userId"`
	MemberID       int32   `json:"memberId"`
	TaskID         int32   `json:"taskId"`
	TimeActual     int32   `json:"timeActual"`
	TimeEstimated  int32   `json:"timeEstimated"`
	TimeFrom       int32   `json:"timeFrom"`
	TimeTo         int32   `json:"timeTo"`
	TimeTimerStart int32   `json:"timeTimerStart"`
	IsFromToShow   bool    `json:"isFromToShow"`
	Date           string  `json:"date"`
	Description    string  `json:"description"`
	IsLocked       bool    `json:"isLocked"`
	TagsIDs        []int32 `json:"tagsIds,omitempty"`
}

// CreateTimeEntry creates a new time entry from e and returns it as stored.
func (c *Client) CreateTimeEntry(ctx context.Context, e provider.TimeEntry) (provider.TimeEntry, error) {
	userID, err := c.currentUserID(ctx)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	projectID, err := idFromString(e.ProjectID)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	memberID, err := idFromString(e.MemberID)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	tagIDs, err := idsFromStrings(e.TagIDs)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	cmd := createTimeEntryCommand{
		ProjectID:    projectID,
		UserID:       userID,
		MemberID:     memberID,
		TimeActual:   int32(e.End.Sub(e.Start).Seconds()),
		TimeFrom:     secondsSinceMidnight(e.Start),
		TimeTo:       secondsSinceMidnight(e.End),
		IsFromToShow: true,
		Date:         dateAtMidnight(e.Start).Format(dateLayout),
		Description:  e.Description,
		TagsIDs:      tagIDs,
	}
	var dto timeEntryDto
	if err := c.doJSON(ctx, "POST", "/api/TimeEntries", cmd, &dto); err != nil {
		return provider.TimeEntry{}, err
	}
	return dtoToEntry(dto), nil
}

type updateTimeEntryCommand struct {
	ID             int32   `json:"id"`
	ProjectID      int32   `json:"projectId"`
	UserID         string  `json:"userId"`
	MemberID       int32   `json:"memberId"`
	TaskID         int32   `json:"taskId"`
	TimeActual     int32   `json:"timeActual"`
	TimeEstimated  int32   `json:"timeEstimated"`
	TimeFrom       int32   `json:"timeFrom"`
	TimeTo         int32   `json:"timeTo"`
	TimeTimerStart int32   `json:"timeTimerStart"`
	IsFromToShow   bool    `json:"isFromToShow"`
	Date           string  `json:"date"`
	Description    string  `json:"description"`
	IsLocked       bool    `json:"isLocked"`
	TagsIDs        []int32 `json:"tagsIds,omitempty"`
}

// UpdateTimeEntry updates an existing entry (matched by e.ID) and returns it as stored.
func (c *Client) UpdateTimeEntry(ctx context.Context, e provider.TimeEntry) (provider.TimeEntry, error) {
	userID, err := c.currentUserID(ctx)
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
	memberID, err := idFromString(e.MemberID)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	tagIDs, err := idsFromStrings(e.TagIDs)
	if err != nil {
		return provider.TimeEntry{}, err
	}
	cmd := updateTimeEntryCommand{
		ID:           id,
		ProjectID:    projectID,
		UserID:       userID,
		MemberID:     memberID,
		TimeActual:   int32(e.End.Sub(e.Start).Seconds()),
		TimeFrom:     secondsSinceMidnight(e.Start),
		TimeTo:       secondsSinceMidnight(e.End),
		IsFromToShow: true,
		Date:         dateAtMidnight(e.Start).Format(dateLayout),
		Description:  e.Description,
		TagsIDs:      tagIDs,
	}
	var dto timeEntryDto
	if err := c.doJSON(ctx, "PUT", "/api/TimeEntries", cmd, &dto); err != nil {
		return provider.TimeEntry{}, err
	}
	return dtoToEntry(dto), nil
}

type deleteTimeEntryCommand struct {
	ID        int32  `json:"id"`
	UserID    string `json:"userId"`
	ProjectID int32  `json:"projectId"`
}

// DeleteTimeEntry removes an entry.
func (c *Client) DeleteTimeEntry(ctx context.Context, e provider.TimeEntry) error {
	userID, err := c.currentUserID(ctx)
	if err != nil {
		return err
	}
	id, err := idFromString(e.ID)
	if err != nil {
		return err
	}
	projectID, err := idFromString(e.ProjectID)
	if err != nil {
		return err
	}
	cmd := deleteTimeEntryCommand{ID: id, UserID: userID, ProjectID: projectID}
	return c.doJSON(ctx, "DELETE", "/api/TimeEntries", cmd, nil)
}

package trackit

import (
	"context"
	"fmt"
	"net/url"
	"sort"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// workLocationDto mirrors WorkLocationDto (TrackIt >= 4.0.0).
type workLocationDto struct {
	ID        int32       `json:"id"`
	Name      string      `json:"name"`
	Color     looseString `json:"color"`
	IsActive  bool        `json:"isActive"`
	SortOrder int32       `json:"sortOrder"`
}

type workLocationsListVm struct {
	WorkLocations []workLocationDto `json:"workLocations"`
	Count         int32             `json:"count"`
}

// checkInDto mirrors CheckInDto, returned by check-in, check-out and the
// status query alike.
type checkInDto struct {
	ID                int32       `json:"id"`
	WorkLocationID    int32       `json:"workLocationId"`
	WorkLocationName  string      `json:"workLocationName"`
	WorkLocationColor looseString `json:"workLocationColor"`
	CheckInTime       string      `json:"checkInTime"`
	CheckOutTime      *string     `json:"checkOutTime"`
	Note              string      `json:"note"`
}

type checkInCommand struct {
	UserID         string `json:"userId"`
	WorkLocationID int32  `json:"workLocationId"`
	Note           string `json:"note,omitempty"`
}

// checkOutCommand's Note is a pointer because the server keeps the
// check-in's note only when the field is null (`request.Note ?? old`): an
// empty string would wipe it.
type checkOutCommand struct {
	ID     int32   `json:"id"`
	UserID string  `json:"userId"`
	Note   *string `json:"note,omitempty"`
}

func dtoToSession(d checkInDto) provider.AttendanceSession {
	s := provider.AttendanceSession{
		ID:               idToString(d.ID),
		WorkLocationID:   idToString(d.WorkLocationID),
		WorkLocationName: d.WorkLocationName,
		CheckIn:          parseServerDate(d.CheckInTime),
		Note:             d.Note,
	}
	if d.CheckOutTime != nil {
		s.CheckOut = parseServerDate(*d.CheckOutTime)
	}
	return s
}

// AttendanceEnabled reports whether check-in/out was turned on for this
// provider in the config (`attendance: true`).
func (c *Client) AttendanceEnabled() bool { return c.attendance }

// ListWorkLocations returns the active work locations, in the server's
// configured display order.
func (c *Client) ListWorkLocations(ctx context.Context) ([]provider.WorkLocation, error) {
	var vm workLocationsListVm
	if err := c.doJSON(ctx, "GET", "/api/WorkLocations?activeOnly=true", nil, &vm); err != nil {
		return nil, err
	}
	sort.SliceStable(vm.WorkLocations, func(i, j int) bool {
		return vm.WorkLocations[i].SortOrder < vm.WorkLocations[j].SortOrder
	})
	out := make([]provider.WorkLocation, 0, len(vm.WorkLocations))
	for _, w := range vm.WorkLocations {
		if !w.IsActive {
			continue
		}
		out = append(out, provider.WorkLocation{
			ID:    idToString(w.ID),
			Name:  w.Name,
			Color: string(w.Color),
		})
	}
	return out, nil
}

// LastWorkLocation returns the location of the caller's most recent
// check-in (open or closed), or "" if they've never checked in.
func (c *Client) LastWorkLocation(ctx context.Context) (string, error) {
	userID, err := c.currentUserID(ctx)
	if err != nil {
		return "", err
	}
	var id *int32
	path := "/api/Attendance/me/last-work-location?" + url.Values{"userId": {userID}}.Encode()
	if err := c.doJSON(ctx, "GET", path, nil, &id); err != nil {
		return "", err
	}
	if id == nil {
		return "", nil
	}
	return idToString(*id), nil
}

// AttendanceStatus returns the caller's open session, or nil if they're not
// checked in (the server answers that with an empty/null body).
func (c *Client) AttendanceStatus(ctx context.Context) (*provider.AttendanceSession, error) {
	userID, err := c.currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	var dto *checkInDto
	path := "/api/Attendance/me/status?" + url.Values{"userId": {userID}}.Encode()
	if err := c.doJSON(ctx, "GET", path, nil, &dto); err != nil {
		return nil, err
	}
	if dto == nil || dto.ID == 0 {
		return nil, nil
	}
	s := dtoToSession(*dto)
	return &s, nil
}

// CheckIn opens a new attendance session at workLocationID.
func (c *Client) CheckIn(ctx context.Context, workLocationID, note string) (provider.AttendanceSession, error) {
	userID, err := c.currentUserID(ctx)
	if err != nil {
		return provider.AttendanceSession{}, err
	}
	locID, err := idFromString(workLocationID)
	if err != nil {
		return provider.AttendanceSession{}, err
	}
	cmd := checkInCommand{UserID: userID, WorkLocationID: locID, Note: note}
	var dto checkInDto
	if err := c.doJSON(ctx, "POST", "/api/Attendance/checkin", cmd, &dto); err != nil {
		return provider.AttendanceSession{}, err
	}
	return dtoToSession(dto), nil
}

// CheckOut closes session s. An empty note keeps the check-in's note.
func (c *Client) CheckOut(ctx context.Context, s provider.AttendanceSession, note string) (provider.AttendanceSession, error) {
	userID, err := c.currentUserID(ctx)
	if err != nil {
		return provider.AttendanceSession{}, err
	}
	id, err := idFromString(s.ID)
	if err != nil {
		return provider.AttendanceSession{}, err
	}
	cmd := checkOutCommand{ID: id, UserID: userID}
	if note != "" {
		cmd.Note = &note
	}
	var dto checkInDto
	path := fmt.Sprintf("/api/Attendance/%d/checkout", id)
	if err := c.doJSON(ctx, "POST", path, cmd, &dto); err != nil {
		return provider.AttendanceSession{}, err
	}
	return dtoToSession(dto), nil
}

var _ provider.Attendance = (*Client)(nil)

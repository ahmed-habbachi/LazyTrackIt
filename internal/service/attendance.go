package service

import (
	"context"
	"errors"
	"strings"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// ErrAttendanceDisabled is returned by attendance calls when the active
// provider doesn't support check-in/out or it isn't enabled in the config.
var ErrAttendanceDisabled = errors.New("check-in/out isn't enabled for this provider")

// ErrNoWorkLocations is returned when the server has no active work
// locations to check in at.
var ErrNoWorkLocations = errors.New("no active work locations are configured on the server")

// CheckInOptions is what a check-in form offers: the active work locations
// and which of them to preselect (the one used last).
type CheckInOptions struct {
	Locations     []provider.WorkLocation
	SelectedIndex int
}

func (sc Scope) requireAttendance() (provider.Attendance, error) {
	a := sc.Attendance()
	if a == nil {
		return nil, ErrAttendanceDisabled
	}
	return a, nil
}

// AttendanceStatus returns the member's open session, or nil if they're not
// checked in.
func (sc Scope) AttendanceStatus(ctx context.Context) (*provider.AttendanceSession, error) {
	a, err := sc.requireAttendance()
	if err != nil {
		return nil, err
	}
	ctx, done, err := sc.begin(ctx, RequestTimeout)
	if err != nil {
		return nil, err
	}
	defer done()
	session, err := a.AttendanceStatus(ctx)
	if err = sc.finish(err); err != nil {
		return nil, err
	}
	return session, nil
}

// CheckInOptions fetches the work locations to offer, preselecting the one
// last checked in at. A failure to look up the last location isn't fatal:
// the first location is preselected instead.
func (sc Scope) CheckInOptions(ctx context.Context) (CheckInOptions, error) {
	a, err := sc.requireAttendance()
	if err != nil {
		return CheckInOptions{}, err
	}
	ctx, done, err := sc.begin(ctx, RequestTimeout)
	if err != nil {
		return CheckInOptions{}, err
	}
	defer done()

	locs, err := a.ListWorkLocations(ctx)
	if err = sc.finish(err); err != nil {
		return CheckInOptions{}, err
	}
	if len(locs) == 0 {
		return CheckInOptions{}, ErrNoWorkLocations
	}
	opts := CheckInOptions{Locations: locs}
	lastID, _ := a.LastWorkLocation(ctx)
	for i, l := range locs {
		if l.ID == lastID {
			opts.SelectedIndex = i
		}
	}
	return opts, nil
}

// CheckIn opens a new session at workLocationID.
func (sc Scope) CheckIn(ctx context.Context, workLocationID, note string) (provider.AttendanceSession, error) {
	a, err := sc.requireAttendance()
	if err != nil {
		return provider.AttendanceSession{}, err
	}
	ctx, done, err := sc.begin(ctx, RequestTimeout)
	if err != nil {
		return provider.AttendanceSession{}, err
	}
	defer done()
	return a.CheckIn(ctx, workLocationID, strings.TrimSpace(note))
}

// CheckOut closes session. An empty note keeps the one given at check-in.
func (sc Scope) CheckOut(ctx context.Context, session provider.AttendanceSession, note string) (provider.AttendanceSession, error) {
	a, err := sc.requireAttendance()
	if err != nil {
		return provider.AttendanceSession{}, err
	}
	ctx, done, err := sc.begin(ctx, RequestTimeout)
	if err != nil {
		return provider.AttendanceSession{}, err
	}
	defer done()
	return a.CheckOut(ctx, session, strings.TrimSpace(note))
}

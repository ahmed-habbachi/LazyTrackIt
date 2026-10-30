// Package provider defines the backend-agnostic interface LazyTrackIt's UI
// talks to, so a new time-tracking API can be added by implementing
// Provider without touching the UI layer.
package provider

import (
	"context"
	"time"
)

// Project is a place time can be logged against.
type Project struct {
	ID         string
	Name       string
	ClientName string
	Color      string
	Active     bool
}

// Tag is an optional label that can be attached to a time entry. IsSystem
// marks a tag defined globally (usable on any project) rather than assigned
// to one specific project.
type Tag struct {
	ID       string
	Name     string
	Color    string
	IsSystem bool
}

// Member identifies the authenticated user within the backend.
type Member struct {
	ID       string
	UserID   string
	FullName string
	Username string
	Email    string
}

// DisplayName returns the member's full name, falling back to their email
// and then their username if the name is unavailable.
func (m Member) DisplayName() string {
	switch {
	case m.FullName != "":
		return m.FullName
	case m.Email != "":
		return m.Email
	default:
		return m.Username
	}
}

// TimeEntry is one logged (or being-logged) block of time.
type TimeEntry struct {
	ID          string
	ProjectID   string
	ProjectName string
	MemberID    string
	Start       time.Time
	End         time.Time
	Description string
	TagIDs      []string
	Locked      bool
}

// Duration returns End-Start as a time.Duration.
func (e TimeEntry) Duration() time.Duration {
	return e.End.Sub(e.Start)
}

// LoginPrompt is shown to the user when Login needs them to complete an
// out-of-band step before it can proceed (e.g. approving a device-code
// request in a browser). Its fields are deliberately generic so any auth
// method can populate them, not just OAuth device flow.
type LoginPrompt struct {
	Message string
	URL     string
	Code    string
	Expires time.Duration
}

// Provider is the backend-agnostic interface the UI depends on.
type Provider interface {
	// Name is a short identifier, e.g. "trackit".
	Name() string

	// IsAuthenticated reports whether usable credentials are already
	// available (e.g. a cached token, or a configured API key), without
	// making a network call.
	IsAuthenticated() bool

	// Login obtains credentials and persists them for future use. onPrompt
	// is invoked only if the auth method needs the user to do something
	// out-of-band (e.g. a device-code flow); a provider whose auth requires
	// no interaction (e.g. a pre-configured API key) can validate it and
	// return without ever calling onPrompt.
	Login(ctx context.Context, onPrompt func(LoginPrompt)) error

	// Me returns the authenticated user's member record.
	Me(ctx context.Context) (Member, error)

	// ListProjects returns the projects available to the authenticated user.
	ListProjects(ctx context.Context) ([]Project, error)

	// ListTags returns the tags defined on the given project, so the UI can
	// offer them as suggestions instead of making the user guess tag IDs.
	ListTags(ctx context.Context, projectID string) ([]Tag, error)

	// ListTimeEntries returns entries for memberID within [from, to] (inclusive dates).
	ListTimeEntries(ctx context.Context, memberID string, from, to time.Time) ([]TimeEntry, error)

	// CreateTimeEntry creates a new entry and returns it as stored.
	CreateTimeEntry(ctx context.Context, e TimeEntry) (TimeEntry, error)

	// UpdateTimeEntry updates an existing entry (by e.ID) and returns it as stored.
	UpdateTimeEntry(ctx context.Context, e TimeEntry) (TimeEntry, error)

	// DeleteTimeEntry removes an entry.
	DeleteTimeEntry(ctx context.Context, e TimeEntry) error
}

// WorkLocation is a place a member can check in at (e.g. office, home).
type WorkLocation struct {
	ID    string
	Name  string
	Color string
}

// AttendanceSession is one check-in, open until CheckOut is set.
type AttendanceSession struct {
	ID               string
	WorkLocationID   string
	WorkLocationName string
	CheckIn          time.Time
	CheckOut         time.Time
	Note             string
}

// Open reports whether the session hasn't been checked out of yet.
func (s AttendanceSession) Open() bool { return s.CheckOut.IsZero() }

// Attendance is an optional capability a Provider may implement to support
// checking in and out of a work location. The UI type-asserts for it, and
// only offers check-in/out when AttendanceEnabled also reports true (it's
// opt-in per provider in the config, since not every deployment of a
// backend that could support it actually does).
type Attendance interface {
	// AttendanceEnabled reports whether check-in/out is turned on for this
	// provider.
	AttendanceEnabled() bool

	// ListWorkLocations returns the active locations a member can check in at.
	ListWorkLocations(ctx context.Context) ([]WorkLocation, error)

	// LastWorkLocation returns the ID of the location the member most
	// recently checked in at, or "" if they never have.
	LastWorkLocation(ctx context.Context) (string, error)

	// AttendanceStatus returns the member's open session, or nil if they're
	// not currently checked in.
	AttendanceStatus(ctx context.Context) (*AttendanceSession, error)

	// CheckIn opens a new session at workLocationID.
	CheckIn(ctx context.Context, workLocationID, note string) (AttendanceSession, error)

	// CheckOut closes session s. An empty note keeps the one given at check-in.
	CheckOut(ctx context.Context, s AttendanceSession, note string) (AttendanceSession, error)
}

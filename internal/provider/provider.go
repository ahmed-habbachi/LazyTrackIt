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
	UserID   string // backend's opaque per-user identifier, distinct from ID
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
	Message string        // instructions to show, e.g. "Open this URL and enter the code below"
	URL     string        // if set, a link the UI should try to open automatically
	Code    string        // if set, a short code the user enters at URL
	Expires time.Duration // if nonzero, how long Code/URL stay valid
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

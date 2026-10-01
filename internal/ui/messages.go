package ui

import "github.com/ahmed-habbachi/lazytrackit/internal/provider"

type loginPromptMsg struct{ prompt provider.LoginPrompt }

type loginResultMsg struct{ err error }

type meAndProjectsLoadedMsg struct {
	member   provider.Member
	projects []provider.Project
	err      error
}

type entriesLoadedMsg struct {
	entries []provider.TimeEntry
	err     error
}

type entrySavedMsg struct {
	entry   provider.TimeEntry
	err     error
	wasEdit bool
}

type entryDeletedMsg struct{ err error }

// tagsLoadedMsg carries the tags fetched for one project. projectID lets the
// handler discard a stale response if the user has since switched projects.
type tagsLoadedMsg struct {
	projectID string
	tags      []provider.Tag
	err       error
}

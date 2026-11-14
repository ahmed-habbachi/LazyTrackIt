package ui

import "github.com/ahmed-habbachi/lazytrackit/internal/provider"

type loginPromptMsg struct{ prompt provider.LoginPrompt }

type loginResultMsg struct{ err error }

// meAndProjectsLoadedMsg reports that the session finished loading the
// member and projects (which it keeps; see service.Session.LoadProfile).
type meAndProjectsLoadedMsg struct{ err error }

type entriesLoadedMsg struct {
	entries []provider.TimeEntry
	err     error
}

type entrySavedMsg struct {
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

// updateInstalledMsg reports that a newer LazyTrackIt release was found,
// downloaded, and installed in place of the running executable. It'll be
// used the next time the app starts.
type updateInstalledMsg struct{ version string }

// updateBlockedMsg reports that self-update can never succeed from this
// install location, because dir (the directory holding the running
// executable) isn't writable by the current user.
type updateBlockedMsg struct{ dir string }

// clearStatusMsg asks the handler to clear m.status, but only if seq still
// matches m.statusSeq — i.e. no newer status was set in the meantime, which
// would otherwise get wiped out by this stale timer.
type clearStatusMsg struct{ seq int }

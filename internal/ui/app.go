// Package ui implements LazyTrackIt's Bubble Tea TUI on top of a
// provider.Provider backend.
package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
	"github.com/ahmed-habbachi/lazytrackit/internal/service"
	"github.com/ahmed-habbachi/lazytrackit/internal/update"
	"github.com/ahmed-habbachi/lazytrackit/internal/version"
)

type screen int

const (
	screenLogin screen = iota
	screenLoading
	screenList
	screenForm
	screenConfirmDelete
	screenProviderPicker
	screenAttendance
	screenAddProvider
	screenConfirmRemoveProvider
	screenError
)

// statusDisplayTime is how long an informational status message (e.g.
// "✓ Entry created.") stays on screen before it's cleared automatically.
const statusDisplayTime = 5 * time.Second

// Model is the root Bubble Tea model for LazyTrackIt.
type Model struct {
	// session owns the providers, which one is active, and what's been
	// loaded for it (member, projects, last-used entry defaults). It's
	// shared by pointer, so every copy of Model sees the same state.
	session *service.Session
	// providerCursor is the selection while screenProviderPicker is shown;
	// one past the last provider is the "Add a provider" row.
	providerCursor int
	// providerForm backs screenAddProvider.
	providerForm *providerForm
	// removeTarget is the provider screenConfirmRemoveProvider asks about.
	removeTarget string
	// backendVersion is the active provider's server version, once known.
	backendVersion  string
	events          chan tea.Msg
	checkForUpdates bool

	screen  screen
	spinner spinner.Model
	table   table.Model

	width, height int

	loginPrompt *provider.LoginPrompt

	entries   []provider.TimeEntry
	weekStart time.Time

	form         *entryForm
	deleteTarget *provider.TimeEntry

	// attendanceSession is the open check-in session (nil if checked out);
	// only used when the active provider supports and enables attendance
	// (see Model.attendance).
	attendanceSession *provider.AttendanceSession
	attendanceLoaded  bool
	attendanceErr     error
	attendanceForm    *attendanceForm

	err    error
	status string

	// statusSeq increments every time status is set, so a pending
	// clearStatusMsg can tell whether it's still the one that should clear
	// it or whether a newer status has since replaced it.
	statusSeq int

	// updateNotice, once set, is shown on the list screen: either that a
	// newer release was downloaded and installed in the background (a
	// restart will pick it up), or that self-update can never work from
	// this install location at all (see updateBlockedMsg).
	updateNotice string
}

// New builds the initial Model on top of session. checkForUpdates controls
// whether LazyTrackIt checks GitHub for a newer release on startup.
func New(session *service.Session, checkForUpdates bool) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	t := table.New(
		table.WithColumns(entryColumns(80)),
		table.WithFocused(true),
		table.WithHeight(12),
	)
	ts := table.DefaultStyles()
	ts.Header = ts.Header.
		Bold(true).
		Foreground(colorAccent).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorMuted).
		BorderBottom(true)
	ts.Selected = ts.Selected.
		Bold(true).
		Foreground(lipgloss.Color("0")).
		Background(colorAccent)
	t.SetStyles(ts)

	return Model{
		session:         session,
		events:          make(chan tea.Msg, 4),
		checkForUpdates: checkForUpdates,
		screen:          screenLoading,
		spinner:         s,
		table:           t,
		weekStart:       service.StartOfWeek(time.Now()),
	}
}

func waitForEvent(events chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-events
	}
}

// setStatus sets a status message and returns a command that clears it again
// after statusDisplayTime, unless a newer status has been set in the
// meantime.
func (m *Model) setStatus(s string) tea.Cmd {
	m.status = s
	m.statusSeq++
	seq := m.statusSeq
	return tea.Tick(statusDisplayTime, func(time.Time) tea.Msg {
		return clearStatusMsg{seq: seq}
	})
}

// Init kicks off authentication (if needed), the initial data load, and (if
// enabled) a background check for a newer release.
func (m Model) Init() tea.Cmd {
	go m.bootstrap(m.session.Current())
	if m.checkForUpdates {
		go m.checkForUpdate()
	}
	return tea.Batch(m.spinner.Tick, waitForEvent(m.events))
}

// checkForUpdate runs in its own goroutine. It's entirely best-effort: on
// any failure (offline, rate-limited, no release for this platform, etc.)
// it just stays quiet rather than reporting an error, since this is a
// background convenience, not something the user is waiting on.
func (m Model) checkForUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res := update.CheckAndApply(ctx, version.Version)
	switch {
	case res.Installed:
		m.events <- updateInstalledMsg{version: res.NewVersion}
	case res.Unwritable != "":
		m.events <- updateBlockedMsg{dir: res.Unwritable}
	}
}

// The background loaders below each run in their own goroutine against the
// service.Scope captured (in Update) when they were started, so they always
// target the provider that was on screen at the time. If the user switched
// provider since, the call returns service.ErrStale and its result is
// dropped instead of being sent, since it belongs to a provider no longer
// shown (the switch has already started the new provider's own loads).

// stale reports whether err means a result should be dropped (see above).
func stale(err error) bool { return errors.Is(err, service.ErrStale) }

// bootstrap logs in if necessary (including again, if the saved login has
// expired), then loads the authenticated member and their projects; the
// meAndProjectsLoadedMsg handler goes on to load the current week.
func (m Model) bootstrap(sc service.Scope) {
	prompted := false
	_, err := sc.Connect(context.Background(), func(p provider.LoginPrompt) {
		prompted = true
		if sc.Valid() {
			m.events <- loginPromptMsg{prompt: p}
		}
	})
	if stale(err) {
		return
	}
	if prompted {
		// Leave the login screen (for the error screen, on failure).
		m.events <- loginResultMsg{err: err}
		if err != nil {
			return
		}
	}
	m.events <- meAndProjectsLoadedMsg{err: err}
}

// retryLoad (re)loads whatever hasn't successfully loaded yet: login and
// the member/project list first (also when a later call found the saved
// login expired), then the current week's entries.
// loadBackendVersion fetches the server's version for the header; it's
// informational, so failures just leave it out.
func (m Model) loadBackendVersion(sc service.Scope) {
	v, err := sc.BackendVersion(context.Background())
	if err != nil {
		return
	}
	m.events <- backendVersionMsg{provider: sc.ProviderName(), version: v}
}

func (m Model) retryLoad(sc service.Scope) {
	if _, ok := sc.Member(); !ok || sc.NeedsLogin() {
		m.bootstrap(sc)
		return
	}
	m.loadEntries(sc, m.weekStart)
}

func (m Model) loadEntries(sc service.Scope, weekStart time.Time) {
	entries, err := sc.ListEntries(context.Background(), weekStart)
	if stale(err) {
		return
	}
	m.events <- entriesLoadedMsg{entries: entries, err: err}
}

func (m Model) saveEntry(sc service.Scope, e provider.TimeEntry) {
	_, err := sc.SaveEntry(context.Background(), e)
	if stale(err) {
		return
	}
	m.events <- entrySavedMsg{err: err, wasEdit: e.ID != ""}
}

// loadTags fetches the tags defined on projectID so the form can offer them
// as suggestions instead of making the user guess tag IDs.
func (m Model) loadTags(sc service.Scope, projectID string) {
	tags, err := sc.ListTags(context.Background(), projectID)
	if stale(err) {
		return
	}
	m.events <- tagsLoadedMsg{projectID: projectID, tags: tags, err: err}
}

func (m Model) deleteEntry(sc service.Scope, e provider.TimeEntry) {
	err := sc.DeleteEntry(context.Background(), e)
	if stale(err) {
		return
	}
	m.events <- entryDeletedMsg{err: err}
}

// Update implements tea.Model. It delegates to updateInner and then makes
// sure the background-event channel listener is always re-armed, regardless
// of which branch of updateInner handled this message.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.updateInner(msg)
	nm := next.(Model)
	return nm, tea.Batch(cmd, waitForEvent(nm.events))
}

func (m Model) updateInner(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.table.SetWidth(m.width - 4)
		m.table.SetColumns(entryColumns(m.width - 4))
		if m.height > 10 {
			m.table.SetHeight(m.height - 10)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case loginPromptMsg:
		p := msg.prompt
		m.loginPrompt = &p
		m.screen = screenLogin
		if p.URL != "" {
			tryOpenBrowser(p.URL)
		}
		return m, nil

	case loginResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.screen = screenError
			return m, nil
		}
		m.screen = screenLoading
		return m, nil

	case meAndProjectsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.screen = screenError
			return m, nil
		}
		go m.loadEntries(m.session.Current(), m.weekStart)
		go m.loadAttendanceStatus(m.session.Current())
		go m.loadBackendVersion(m.session.Current())
		return m, nil

	case backendVersionMsg:
		if msg.provider == m.session.ActiveProviderName() {
			m.backendVersion = msg.version
		}
		return m, nil

	case entriesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.screen = screenError
			return m, nil
		}
		m.entries = msg.entries
		m.rebuildTable()
		m.screen = screenList
		return m, nil

	case entrySavedMsg:
		if msg.err != nil {
			m.status = ""
			m.err = msg.err
			m.screen = screenError
			return m, nil
		}
		status := "Entry created."
		if msg.wasEdit {
			status = "Entry updated."
		}
		statusCmd := m.setStatus(status)
		m.form = nil
		m.screen = screenLoading
		go m.loadEntries(m.session.Current(), m.weekStart)
		return m, statusCmd

	case tagsLoadedMsg:
		if m.form != nil && m.form.currentProjectID() == msg.projectID {
			m.form.setAvailableTags(msg.tags, msg.err)
		}
		return m, nil

	case entryDeletedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.screen = screenError
			return m, nil
		}
		cmd := m.setStatus("Deleted.")
		m.deleteTarget = nil
		m.screen = screenLoading
		go m.loadEntries(m.session.Current(), m.weekStart)
		return m, cmd

	case updateInstalledMsg:
		m.updateNotice = fmt.Sprintf("⬆ Update %s installed — restart LazyTrackIt to use it.", msg.version)
		return m, nil

	case updateBlockedMsg:
		m.updateNotice = fmt.Sprintf("⚠ Installed at %s, which needs elevated permissions to write to — auto-update will never work here. Move the lazytrackit binary to ~/.local/bin (or another folder you own) to enable it.", msg.dir)
		return m, nil

	case attendanceStatusMsg, checkInOptionsMsg, attendanceSavedMsg:
		return m.handleAttendanceMsg(msg)

	case clearStatusMsg:
		if msg.seq == m.statusSeq {
			m.status = ""
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.screen {
	case screenLogin, screenLoading:
		if msg.String() == "q" {
			return m, tea.Quit
		}
		return m, nil

	case screenError:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "r":
			m.err = nil
			m.screen = screenLoading
			go m.retryLoad(m.session.Current())
			return m, nil
		case "p":
			// e.g. a newly added provider with a wrong token: switch back.
			m.providerCursor = indexOf(m.session.ProviderNames(), m.session.ActiveProviderName())
			m.screen = screenProviderPicker
			return m, nil
		}
		return m, nil

	case screenList:
		return m.handleListKey(msg)

	case screenForm:
		return m.handleFormKey(msg)

	case screenConfirmDelete:
		switch msg.String() {
		case "y":
			target := m.deleteTarget
			m.screen = screenLoading
			if target != nil {
				go m.deleteEntry(m.session.Current(), *target)
			}
			return m, nil
		case "n", "esc":
			m.deleteTarget = nil
			m.screen = screenList
			return m, nil
		}
		return m, nil

	case screenProviderPicker:
		return m.handleProviderPickerKey(msg)

	case screenAttendance:
		return m.handleAttendanceKey(msg)

	case screenAddProvider:
		return m.handleProviderFormKey(msg)

	case screenConfirmRemoveProvider:
		return m.handleConfirmRemoveProviderKey(msg)
	}

	return m, nil
}

func (m Model) handleProviderPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.providerCursor > 0 {
			m.providerCursor--
		}
		return m, nil
	case "down", "j":
		if m.providerCursor < len(m.session.ProviderNames()) { // incl. the "add" row
			m.providerCursor++
		}
		return m, nil
	case "a", "+":
		return m.openProviderForm()
	case "e":
		if names := m.session.ProviderNames(); m.providerCursor < len(names) {
			return m.openEditProviderForm(names[m.providerCursor])
		}
		return m, nil
	case "d", "x", "delete":
		if names := m.session.ProviderNames(); m.providerCursor < len(names) {
			return m.confirmRemoveProvider(names[m.providerCursor])
		}
		return m, nil
	case "enter":
		names := m.session.ProviderNames()
		if m.providerCursor >= len(names) {
			return m.openProviderForm()
		}
		name := names[m.providerCursor]
		if name == m.session.ActiveProviderName() {
			m.screen = screenList
			return m, nil
		}
		return m.switchProvider(name)
	case "esc", "q":
		m.screen = screenList
		return m, nil
	}
	return m, nil
}

// switchProvider makes name the active provider, resets the state that's
// specific to whichever provider was active before, remembers the choice
// for next startup, and re-runs bootstrap (login if needed, then load
// member/projects/entries) against the new provider.
func (m Model) switchProvider(name string) (tea.Model, tea.Cmd) {
	if err := m.session.SwitchProvider(name); err != nil {
		m.err = err
		m.screen = screenError
		return m, nil
	}
	return m.reloadProvider()
}

// reloadProvider resets what was loaded for the previous provider and
// re-runs bootstrap against the session's (new) active provider.
func (m Model) reloadProvider() (tea.Model, tea.Cmd) {
	m.entries = nil
	m.backendVersion = ""
	m.form = nil
	m.deleteTarget = nil
	m.attendanceSession = nil
	m.attendanceLoaded = false
	m.attendanceErr = nil
	m.attendanceForm = nil
	m.err = nil
	m.status = ""
	m.loginPrompt = nil

	m.screen = screenLoading
	go m.bootstrap(m.session.Current())
	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "n":
		draft := service.NewEntryDraft(m.weekStart, time.Now(), m.session.LastDefaults(), m.entries)
		m.form = newEntryForm(draft, m.session.Projects())
		m.screen = screenForm
		go m.loadTags(m.session.Current(), m.form.currentProjectID())
		return m, nil
	case "e", "enter":
		if e := m.selectedEntry(); e != nil {
			if e.Locked {
				return m, m.setStatus("This entry is locked and cannot be edited.")
			}
			m.form = newEntryForm(service.EditEntryDraft(*e), m.session.Projects())
			m.screen = screenForm
			go m.loadTags(m.session.Current(), m.form.currentProjectID())
		}
		return m, nil
	case "x", "delete":
		if e := m.selectedEntry(); e != nil {
			if e.Locked {
				return m, m.setStatus("This entry is locked and cannot be deleted.")
			}
			cp := *e
			m.deleteTarget = &cp
			m.screen = screenConfirmDelete
		}
		return m, nil
	case "r":
		m.screen = screenLoading
		go m.loadEntries(m.session.Current(), m.weekStart)
		go m.loadAttendanceStatus(m.session.Current())
		return m, nil
	case "c":
		return m.startAttendance()
	case "[", "left":
		m.weekStart = m.weekStart.AddDate(0, 0, -7)
		m.screen = screenLoading
		go m.loadEntries(m.session.Current(), m.weekStart)
		return m, nil
	case "]", "right":
		m.weekStart = m.weekStart.AddDate(0, 0, 7)
		m.screen = screenLoading
		go m.loadEntries(m.session.Current(), m.weekStart)
		return m, nil
	case "t":
		m.weekStart = service.StartOfWeek(time.Now())
		m.screen = screenLoading
		go m.loadEntries(m.session.Current(), m.weekStart)
		return m, nil
	case "p":
		names := m.session.ProviderNames()
		m.providerCursor = indexOf(names, m.session.ActiveProviderName())
		m.screen = screenProviderPicker
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func indexOf(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return 0
}

func (m *Model) selectedEntry() *provider.TimeEntry {
	row := m.table.Cursor()
	if row < 0 || row >= len(m.entries) {
		return nil
	}
	return &m.entries[row]
}

// View implements tea.Model.
func (m Model) View() string {
	switch m.screen {
	case screenLogin:
		return m.viewLogin()
	case screenLoading:
		return fmt.Sprintf("\n  %s Working...\n", m.spinner.View())
	case screenError:
		return m.viewError()
	case screenForm:
		return m.viewForm()
	case screenConfirmDelete:
		return m.viewConfirmDelete()
	case screenProviderPicker:
		return m.viewProviderPicker()
	case screenAttendance:
		return m.viewAttendance()
	case screenAddProvider:
		return m.viewProviderForm()
	case screenConfirmRemoveProvider:
		return m.viewConfirmRemoveProvider()
	default:
		return m.viewList()
	}
}

func (m Model) viewProviderPicker() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Providers") + "\n\n")
	active := m.session.ActiveProviderName()
	names := m.session.ProviderNames()
	for i, name := range names {
		marker := "  "
		style := subtleStyle
		if i == m.providerCursor {
			marker = "▸ "
			style = focusedFieldStyle
		}
		label := name
		if name == active {
			label += " (active)"
		}
		b.WriteString(marker + style.Render(label) + "\n")
	}
	addMarker, addStyle := "  ", subtleStyle
	if m.providerCursor == len(names) {
		addMarker, addStyle = "▸ ", focusedFieldStyle
	}
	b.WriteString("\n" + addMarker + addStyle.Render("+ Add a provider…") + "\n")
	if m.status != "" {
		b.WriteString("\n" + okStyle.Render(m.status) + "\n")
	}
	b.WriteString("\n" + helpStyle.Render("[enter] select  [a] add  [e] edit  [d] remove  [esc] back"))
	return "\n" + boxStyle.Render(b.String()) + "\n"
}

func (m Model) viewError() string {
	body := errorStyle.Render("Error: "+m.err.Error()) + "\n" +
		helpStyle.Render("[r] retry   [p] providers   [q] quit")
	return "\n" + boxStyle.Render(body) + "\n"
}

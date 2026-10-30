// Package ui implements LazyTrackIt's Bubble Tea TUI on top of a
// provider.Provider backend.
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
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
	screenError
)

const requestTimeout = 20 * time.Second

// statusDisplayTime is how long an informational status message (e.g.
// "✓ Entry created.") stays on screen before it's cleared automatically.
const statusDisplayTime = 5 * time.Second

// Model is the root Bubble Tea model for LazyTrackIt.
type Model struct {
	providers map[string]provider.Provider
	// providerNames is sorted, for stable display order.
	providerNames      []string
	activeProviderName string
	// providerCursor is the selection while screenProviderPicker is shown.
	providerCursor  int
	prov            provider.Provider
	events          chan tea.Msg
	checkForUpdates bool

	screen  screen
	spinner spinner.Model
	table   table.Model

	width, height int

	loginPrompt *provider.LoginPrompt

	member    provider.Member
	hasMember bool
	projects  []provider.Project
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

	// lastDefaults remembers the project/tags most recently used to create
	// an entry, so a new entry defaults to them instead of starting blank.
	lastDefaults config.LastEntryDefaults

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

// New builds the initial Model. providers holds every configured provider
// keyed by name, names is their stable display order, active is which of
// them to start with, and checkForUpdates controls whether LazyTrackIt
// checks GitHub for a newer release on startup.
func New(providers map[string]provider.Provider, names []string, active string, checkForUpdates bool) Model {
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

	lastDefaults, _ := config.LoadLastEntryDefaults(active)

	return Model{
		providers:          providers,
		providerNames:      names,
		activeProviderName: active,
		prov:               providers[active],
		events:             make(chan tea.Msg, 4),
		checkForUpdates:    checkForUpdates,
		screen:             screenLoading,
		spinner:            s,
		table:              t,
		weekStart:          startOfWeek(time.Now()),
		lastDefaults:       lastDefaults,
	}
}

// startOfDay returns midnight in t's own location. Unlike t.Truncate(24h),
// which truncates the absolute UTC instant, this respects local civil dates
// in timezones not aligned to UTC midnight.
func startOfDay(t time.Time) time.Time {
	y, mo, d := t.Date()
	return time.Date(y, mo, d, 0, 0, 0, 0, t.Location())
}

func startOfWeek(t time.Time) time.Time {
	t = startOfDay(t)
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset)
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
	go m.bootstrap()
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

// bootstrap runs in its own goroutine: logs in if necessary, then loads the
// authenticated member, their projects, and the current week's entries.
func (m Model) bootstrap() {
	ctx := context.Background()

	if !m.prov.IsAuthenticated() {
		loginCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		err := m.prov.Login(loginCtx, func(p provider.LoginPrompt) {
			m.events <- loginPromptMsg{prompt: p}
		})
		cancel()
		m.events <- loginResultMsg{err: err}
		if err != nil {
			return
		}
	}

	m.retryLoad()
}

// retryLoad (re)loads whatever hasn't successfully loaded yet: the member
// and project list first, then the current week's entries.
func (m Model) retryLoad() {
	if !m.hasMember {
		m.loadMeAndProjects()
		return
	}
	m.loadEntries(m.weekStart)
}

func (m Model) loadMeAndProjects() {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	member, err := m.prov.Me(ctx)
	if err != nil {
		m.events <- meAndProjectsLoadedMsg{err: err}
		return
	}
	projects, err := m.prov.ListProjects(ctx)
	m.events <- meAndProjectsLoadedMsg{member: member, projects: projects, err: err}
}

func (m Model) loadEntries(weekStart time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	entries, err := m.prov.ListTimeEntries(ctx, m.member.ID, weekStart, weekStart.AddDate(0, 0, 6))
	m.events <- entriesLoadedMsg{entries: entries, err: err}
}

func (m Model) saveEntry(e provider.TimeEntry, isEdit bool) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	var err error
	if isEdit {
		_, err = m.prov.UpdateTimeEntry(ctx, e)
	} else {
		_, err = m.prov.CreateTimeEntry(ctx, e)
	}
	m.events <- entrySavedMsg{entry: e, err: err, wasEdit: isEdit}
}

// loadTags fetches the tags defined on projectID so the form can offer them
// as suggestions instead of making the user guess tag IDs.
func (m Model) loadTags(projectID string) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	tags, err := m.prov.ListTags(ctx, projectID)
	m.events <- tagsLoadedMsg{projectID: projectID, tags: tags, err: err}
}

func (m Model) deleteEntry(e provider.TimeEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	err := m.prov.DeleteTimeEntry(ctx, e)
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
		m.member = msg.member
		m.hasMember = true
		m.projects = msg.projects
		go m.loadEntries(m.weekStart)
		go m.loadAttendanceStatus()
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
		var statusCmd tea.Cmd
		if msg.wasEdit {
			statusCmd = m.setStatus("Entry updated.")
		} else {
			statusCmd = m.setStatus("Entry created.")
			m.lastDefaults = config.LastEntryDefaults{
				ProjectID: msg.entry.ProjectID,
				TagIDs:    msg.entry.TagIDs,
			}
			_ = config.SaveLastEntryDefaults(m.prov.Name(), m.lastDefaults)
		}
		m.form = nil
		m.screen = screenLoading
		go m.loadEntries(m.weekStart)
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
		go m.loadEntries(m.weekStart)
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
			go m.retryLoad()
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
				go m.deleteEntry(*target)
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
		if m.providerCursor < len(m.providerNames)-1 {
			m.providerCursor++
		}
		return m, nil
	case "enter":
		name := m.providerNames[m.providerCursor]
		if name == m.activeProviderName {
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
	m.activeProviderName = name
	m.prov = m.providers[name]
	m.hasMember = false
	m.member = provider.Member{}
	m.projects = nil
	m.entries = nil
	m.form = nil
	m.deleteTarget = nil
	m.attendanceSession = nil
	m.attendanceLoaded = false
	m.attendanceErr = nil
	m.attendanceForm = nil
	m.err = nil
	m.status = ""
	m.loginPrompt = nil
	m.lastDefaults, _ = config.LoadLastEntryDefaults(name)
	_ = config.SaveLastProvider(name)

	m.screen = screenLoading
	go m.bootstrap()
	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "n":
		m.form = newEntryForm(nil, m.projects, m.weekStart, m.lastDefaults, m.entries)
		m.screen = screenForm
		go m.loadTags(m.form.currentProjectID())
		return m, nil
	case "e", "enter":
		if e := m.selectedEntry(); e != nil {
			if e.Locked {
				return m, m.setStatus("This entry is locked and cannot be edited.")
			}
			m.form = newEntryForm(e, m.projects, m.weekStart, m.lastDefaults, nil)
			m.screen = screenForm
			go m.loadTags(m.form.currentProjectID())
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
		go m.loadEntries(m.weekStart)
		go m.loadAttendanceStatus()
		return m, nil
	case "c":
		return m.startAttendance()
	case "[", "left":
		m.weekStart = m.weekStart.AddDate(0, 0, -7)
		m.screen = screenLoading
		go m.loadEntries(m.weekStart)
		return m, nil
	case "]", "right":
		m.weekStart = m.weekStart.AddDate(0, 0, 7)
		m.screen = screenLoading
		go m.loadEntries(m.weekStart)
		return m, nil
	case "t":
		m.weekStart = startOfWeek(time.Now())
		m.screen = screenLoading
		go m.loadEntries(m.weekStart)
		return m, nil
	case "p":
		if len(m.providerNames) < 2 {
			return m, m.setStatus("Only one provider configured.")
		}
		m.providerCursor = indexOf(m.providerNames, m.activeProviderName)
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
	default:
		return m.viewList()
	}
}

func (m Model) viewProviderPicker() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Switch provider") + "\n\n")
	for i, name := range m.providerNames {
		marker := "  "
		style := subtleStyle
		if i == m.providerCursor {
			marker = "▸ "
			style = focusedFieldStyle
		}
		label := name
		if name == m.activeProviderName {
			label += " (active)"
		}
		b.WriteString(marker + style.Render(label) + "\n")
	}
	b.WriteString("\n" + helpStyle.Render("[enter] select  [esc] cancel"))
	return "\n" + boxStyle.Render(b.String()) + "\n"
}

func (m Model) viewError() string {
	body := errorStyle.Render("Error: "+m.err.Error()) + "\n" +
		helpStyle.Render("[r] retry   [q] quit")
	return "\n" + boxStyle.Render(body) + "\n"
}

// Package ui implements LazyTrackIt's Bubble Tea TUI on top of a
// provider.Provider backend.
package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

type screen int

const (
	screenLogin screen = iota
	screenLoading
	screenList
	screenForm
	screenConfirmDelete
	screenError
)

const requestTimeout = 20 * time.Second

// Model is the root Bubble Tea model for LazyTrackIt.
type Model struct {
	prov   provider.Provider
	events chan tea.Msg

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

	// lastDefaults remembers the project/tags most recently used to create
	// an entry, so a new entry defaults to them instead of starting blank.
	lastDefaults config.LastEntryDefaults

	err    error
	status string
}

// New builds the initial Model for the given provider.
func New(prov provider.Provider) Model {
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

	// A missing or unreadable state file just means no remembered
	// defaults yet; it's not worth failing startup over.
	lastDefaults, _ := config.LoadLastEntryDefaults(prov.Name())

	return Model{
		prov:         prov,
		events:       make(chan tea.Msg, 4),
		screen:       screenLoading,
		spinner:      s,
		table:        t,
		weekStart:    startOfWeek(time.Now()),
		lastDefaults: lastDefaults,
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
	// time.Weekday: Sunday=0 ... Saturday=6. We want Monday-start weeks.
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset)
}

func waitForEvent(events chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-events
	}
}

// Init kicks off authentication (if needed) and the initial data load.
func (m Model) Init() tea.Cmd {
	go m.bootstrap()
	return tea.Batch(m.spinner.Tick, waitForEvent(m.events))
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
		if msg.wasEdit {
			m.status = "Entry updated."
		} else {
			m.status = "Entry created."
			m.lastDefaults = config.LastEntryDefaults{
				ProjectID: msg.entry.ProjectID,
				TagIDs:    msg.entry.TagIDs,
			}
			// Best-effort: losing the remembered defaults isn't worth
			// surfacing an error over.
			_ = config.SaveLastEntryDefaults(m.prov.Name(), m.lastDefaults)
		}
		m.form = nil
		m.screen = screenLoading
		go m.loadEntries(m.weekStart)
		return m, nil

	case tagsLoadedMsg:
		// Discard a response for a project the form has since moved away
		// from (e.g. the user flipped projects again before it arrived).
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
		m.status = "Deleted."
		m.deleteTarget = nil
		m.screen = screenLoading
		go m.loadEntries(m.weekStart)
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
	}

	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "n":
		m.form = newEntryForm(nil, m.projects, m.weekStart, m.lastDefaults)
		m.screen = screenForm
		go m.loadTags(m.form.currentProjectID())
		return m, nil
	case "e", "enter":
		if e := m.selectedEntry(); e != nil {
			if e.Locked {
				m.status = "This entry is locked and cannot be edited."
				return m, nil
			}
			m.form = newEntryForm(e, m.projects, m.weekStart, m.lastDefaults)
			m.screen = screenForm
			go m.loadTags(m.form.currentProjectID())
		}
		return m, nil
	case "x", "delete":
		if e := m.selectedEntry(); e != nil {
			if e.Locked {
				m.status = "This entry is locked and cannot be deleted."
				return m, nil
			}
			cp := *e
			m.deleteTarget = &cp
			m.screen = screenConfirmDelete
		}
		return m, nil
	case "r":
		m.screen = screenLoading
		go m.loadEntries(m.weekStart)
		return m, nil
	case "[":
		m.weekStart = m.weekStart.AddDate(0, 0, -7)
		m.screen = screenLoading
		go m.loadEntries(m.weekStart)
		return m, nil
	case "]":
		m.weekStart = m.weekStart.AddDate(0, 0, 7)
		m.screen = screenLoading
		go m.loadEntries(m.weekStart)
		return m, nil
	case "t":
		m.weekStart = startOfWeek(time.Now())
		m.screen = screenLoading
		go m.loadEntries(m.weekStart)
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
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
	default:
		return m.viewList()
	}
}

func (m Model) viewError() string {
	body := errorStyle.Render("Error: "+m.err.Error()) + "\n" +
		helpStyle.Render("[r] retry   [q] quit")
	return "\n" + boxStyle.Render(body) + "\n"
}

package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/ahmed-habbachi/lazytrackit/internal/version"
)

func entryColumns(totalWidth int) []table.Column {
	const (
		dateWidth = 11
		fromWidth = 6
		toWidth   = 6
		durWidth  = 9
		projWidth = 18
		numCols   = 6
		cellPad   = 2 // table.DefaultStyles() pads each cell 1 space per side
	)
	fixed := dateWidth + fromWidth + toWidth + durWidth + projWidth + numCols*cellPad
	descWidth := totalWidth - fixed
	if descWidth < 10 {
		descWidth = 10
	}
	return []table.Column{
		{Title: "Date", Width: dateWidth},
		{Title: "From", Width: fromWidth},
		{Title: "To", Width: toWidth},
		{Title: "Dur", Width: durWidth},
		{Title: "Project", Width: projWidth},
		{Title: "Description", Width: descWidth},
	}
}

func formatClock(t time.Time) string {
	return t.Format("15:04")
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%02dm", h, m)
}

func truncate(s string, w int) string {
	if len(s) <= w {
		return s
	}
	if w <= 1 {
		return s[:w]
	}
	return s[:w-1] + "…"
}

func (m *Model) rebuildTable() {
	rows := make([]table.Row, 0, len(m.entries))
	for _, e := range m.entries {
		desc := e.Description
		if e.Locked {
			desc = "🔒 " + desc
		}
		rows = append(rows, table.Row{
			e.Start.Format("Mon 01/02"),
			formatClock(e.Start),
			formatClock(e.End),
			formatDuration(e.Duration()),
			truncate(e.ProjectName, 18),
			desc,
		})
	}
	m.table.SetColumns(entryColumns(m.width - 4))
	m.table.SetRows(rows)
	if m.table.Cursor() >= len(rows) && len(rows) > 0 {
		m.table.SetCursor(len(rows) - 1)
	}
}

func (m Model) totalWeekDuration() time.Duration {
	var total time.Duration
	for _, e := range m.entries {
		total += e.Duration()
	}
	return total
}

// contentWidth returns the usable width inside the main view's outer
// padding, with a sane floor for the first frame or a very narrow terminal.
func (m Model) contentWidth() int {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	return w
}

// viewHeader renders the app name on the left and the logged-in user's
// display name on the right, separated by a divider rule.
func (m Model) viewHeader() string {
	width := m.contentWidth()

	left := appTitleStyle.Render("LazyTrackIt") + " " + subtleStyle.Render(version.Version)
	right := ""
	if name := m.member.DisplayName(); name != "" {
		right = userIconStyle.Render("logged in as ") + userBadgeStyle.Render(name)
	}

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	bar := left + strings.Repeat(" ", gap) + right
	divider := headerDividerStyle.Render(strings.Repeat("─", width))
	return bar + "\n" + divider
}

func (m Model) viewList() string {
	var b strings.Builder

	b.WriteString(m.viewHeader())
	b.WriteString("\n\n")

	rangeLabel := fmt.Sprintf("%s – %s", m.weekStart.Format("Jan 2"), m.weekStart.AddDate(0, 0, 6).Format("Jan 2, 2006"))
	weekPill := pillStyle.Render("📅 " + rangeLabel)
	totalPill := pillAccentStyle.Render(fmt.Sprintf("⏱ %s logged", formatDuration(m.totalWeekDuration())))
	b.WriteString(weekPill + " " + totalPill + "\n\n")

	if len(m.entries) == 0 {
		b.WriteString(subtleStyle.Render("No time entries for this week. Press 'n' to add one.") + "\n")
	} else {
		b.WriteString(m.table.View() + "\n")
	}

	if m.status != "" {
		b.WriteString("\n" + okStyle.Render("✓ "+m.status))
	}

	help := "[n] new  [e] edit  [x/del] delete  [r] refresh  [←/→] week  [t] today  [q] quit"
	if len(m.providerNames) > 1 {
		help = "[n] new  [e] edit  [x/del] delete  [r] refresh  [←/→] week  [t] today  [p] providers  [q] quit"
	}
	b.WriteString("\n" + helpStyle.Render(help))

	if m.updateNotice != "" {
		b.WriteString("\n" + subtleStyle.Render("⬆ "+m.updateNotice))
	}

	return sectionStyle.Render(b.String())
}

package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/ahmed-habbachi/lazytrackit/internal/service"
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
		cellPad   = 2
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

// expectedPill renders the cumulated expected-hours-so-far pill, flagging
// in pillWarnStyle whenever logged hours fall short of it.
func (m Model) expectedPill() string {
	expected := service.ExpectedWeekDuration(m.weekStart, time.Now())
	delta := service.TotalDuration(m.entries) - expected

	label := fmt.Sprintf("⏳ %s expected", formatDuration(expected))
	style := pillAccentStyle
	switch {
	case delta < 0:
		label += fmt.Sprintf(" (-%s)", formatDuration(-delta))
		style = pillWarnStyle
	case delta > 0:
		label += fmt.Sprintf(" (+%s)", formatDuration(delta))
	}
	return style.Render(label)
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

	left := appTitleStyle.Render("LazyTrackIt") + " " + subtleStyle.Render(displayVersion(version.Version))
	if m.backendVersion != "" {
		left += subtleStyle.Render("  ·  "+m.session.ActiveProviderName()+" ") + subtleStyle.Render(displayVersion(m.backendVersion))
	}
	right := ""
	member, _ := m.session.Member()
	if name := member.DisplayName(); name != "" {
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

// displayVersion prefixes a numeric version with "v" ("1.2.3" → "v1.2.3");
// anything else (e.g. "dev") is shown as is.
func displayVersion(v string) string {
	if v != "" && v[0] >= '0' && v[0] <= '9' {
		return "v" + v
	}
	return v
}

func (m Model) viewList() string {
	var b strings.Builder

	b.WriteString(m.viewHeader())
	b.WriteString("\n\n")

	rangeLabel := fmt.Sprintf("%s – %s", m.weekStart.Format("Jan 2"), m.weekStart.AddDate(0, 0, 6).Format("Jan 2, 2006"))
	weekPill := pillStyle.Render("📅 " + rangeLabel)
	totalPill := pillAccentStyle.Render(fmt.Sprintf("⏱ %s logged", formatDuration(service.TotalDuration(m.entries))))
	pills := weekPill + " " + totalPill + " " + m.expectedPill()
	if att := m.attendancePill(); att != "" {
		pills += " " + att
	}
	b.WriteString(pills + "\n\n")

	if len(m.entries) == 0 {
		b.WriteString(subtleStyle.Render("No time entries for this week. Press 'n' to add one.") + "\n")
	} else {
		b.WriteString(m.table.View() + "\n")
	}

	if m.status != "" {
		b.WriteString("\n" + okStyle.Render("✓ "+m.status))
	}

	help := "[n] new  [e] edit  [x/del] delete  [r] refresh  [←/→] week  [t] today"
	if m.attendance() != nil {
		help += "  [c] check in/out"
	}
	help += "  [p] providers"
	help += "  [q] quit"
	b.WriteString("\n" + helpStyle.Render(help))

	if m.updateNotice != "" {
		b.WriteString("\n" + subtleStyle.Render(m.updateNotice))
	}

	return sectionStyle.Render(b.String())
}

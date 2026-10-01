package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent  = lipgloss.Color("86")
	colorAccent2 = lipgloss.Color("212")
	colorMuted   = lipgloss.Color("243")
	colorError   = lipgloss.Color("203")
	colorOK      = lipgloss.Color("114")
	colorPillBg  = lipgloss.Color("238")
	colorPillFg  = lipgloss.Color("255")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			Padding(0, 1)

	subtleStyle = lipgloss.NewStyle().Foreground(colorMuted)

	errorStyle = lipgloss.NewStyle().Foreground(colorError).Bold(true)

	okStyle = lipgloss.NewStyle().Foreground(colorOK)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(colorMuted).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)

	fieldLabelStyle = lipgloss.NewStyle().Width(17).Foreground(colorMuted)

	focusedFieldStyle = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)

	// appTitleStyle renders the app name in the top header bar.
	appTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(colorAccent).
			Padding(0, 2)

	// userBadgeStyle renders the logged-in user's name/email in the header.
	userBadgeStyle = lipgloss.NewStyle().
			Foreground(colorAccent2).
			Bold(true)

	userIconStyle = lipgloss.NewStyle().Foreground(colorMuted)

	headerDividerStyle = lipgloss.NewStyle().Foreground(colorMuted)

	pillStyle = lipgloss.NewStyle().
			Foreground(colorPillFg).
			Background(colorPillBg).
			Bold(true).
			Padding(0, 1)

	pillAccentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(colorOK).
			Bold(true).
			Padding(0, 1)

	sectionStyle = lipgloss.NewStyle().Padding(1, 2)

	// timeSegmentStyle highlights whichever of hour/minute is currently
	// being edited within a timeField.
	timeSegmentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(colorAccent).
				Bold(true)

	formTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(colorAccent2).
			Padding(0, 2)

	groupHeadingStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Bold(true)
)

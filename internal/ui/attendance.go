package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
	"github.com/ahmed-habbachi/lazytrackit/internal/service"
)

// attendanceStatusMsg carries the member's open session (nil if they're not
// checked in).
type attendanceStatusMsg struct {
	session *provider.AttendanceSession
	err     error
}

// checkInOptionsMsg carries what the check-in form needs to offer.
type checkInOptionsMsg struct {
	opts service.CheckInOptions
	err  error
}

// attendanceSavedMsg reports the outcome of a check-in or check-out.
type attendanceSavedMsg struct {
	session   provider.AttendanceSession
	checkedIn bool
	err       error
}

// attendanceForm backs screenAttendance, for both checking in (pick a work
// location, optional note) and checking out (optional note only).
type attendanceForm struct {
	checkOut  bool
	session   provider.AttendanceSession
	locations []provider.WorkLocation
	cursor    int
	note      textinput.Model
	loading   bool
	err       error
}

// attendance returns the active provider's check-in/out capability, or nil
// if it doesn't have one or it isn't enabled in the config.
func (m Model) attendance() provider.Attendance {
	return m.session.Attendance()
}

func (m Model) loadAttendanceStatus(sc service.Scope) {
	if sc.Attendance() == nil {
		return
	}
	s, err := sc.AttendanceStatus(context.Background())
	if stale(err) {
		return
	}
	m.events <- attendanceStatusMsg{session: s, err: err}
}

func (m Model) loadCheckInOptions(sc service.Scope) {
	if sc.Attendance() == nil {
		return
	}
	opts, err := sc.CheckInOptions(context.Background())
	if stale(err) {
		return
	}
	m.events <- checkInOptionsMsg{opts: opts, err: err}
}

func (m Model) saveAttendance(sc service.Scope, f attendanceForm) {
	ctx := context.Background()
	note := f.note.Value()
	if f.checkOut {
		s, err := sc.CheckOut(ctx, f.session, note)
		if !stale(err) {
			m.events <- attendanceSavedMsg{session: s, err: err}
		}
		return
	}
	s, err := sc.CheckIn(ctx, f.locations[f.cursor].ID, note)
	if !stale(err) {
		m.events <- attendanceSavedMsg{session: s, checkedIn: true, err: err}
	}
}

// startAttendance opens the check-out form if the member has an open
// session, or the check-in form (fetching work locations) otherwise.
func (m Model) startAttendance() (tea.Model, tea.Cmd) {
	if m.attendance() == nil {
		return m, m.setStatus("Check-in/out isn't enabled for this provider.")
	}
	if !m.attendanceLoaded {
		go m.loadAttendanceStatus(m.session.Current())
		return m, m.setStatus("Attendance status is still loading, try again in a moment.")
	}

	note := newTextInput("optional note", "", 500, 40)
	note.Focus()
	f := &attendanceForm{note: note}
	if m.attendanceSession != nil {
		f.checkOut = true
		f.session = *m.attendanceSession
	} else {
		f.loading = true
		go m.loadCheckInOptions(m.session.Current())
	}
	m.attendanceForm = f
	m.screen = screenAttendance
	return m, textinput.Blink
}

func (m Model) handleAttendanceMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case attendanceStatusMsg:
		m.attendanceErr = msg.err
		if msg.err == nil {
			m.attendanceSession = msg.session
			m.attendanceLoaded = true
		}
		return m, nil

	case checkInOptionsMsg:
		f := m.attendanceForm
		if f == nil || f.checkOut {
			return m, nil
		}
		f.loading = false
		f.err = msg.err
		f.locations = msg.opts.Locations
		f.cursor = msg.opts.SelectedIndex
		return m, nil

	case attendanceSavedMsg:
		if msg.err != nil {
			if f := m.attendanceForm; f != nil {
				f.loading = false
				f.err = msg.err
			}
			go m.loadAttendanceStatus(m.session.Current())
			return m, nil
		}
		m.attendanceForm = nil
		m.screen = screenList
		m.attendanceErr = nil
		m.attendanceLoaded = true
		if msg.checkedIn {
			s := msg.session
			m.attendanceSession = &s
			return m, m.setStatus(fmt.Sprintf("Checked in at %s.", s.WorkLocationName))
		}
		m.attendanceSession = nil
		return m, m.setStatus(fmt.Sprintf("Checked out after %s.", formatDuration(msg.session.CheckOut.Sub(msg.session.CheckIn))))
	}
	return m, nil
}

func (m Model) handleAttendanceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.attendanceForm
	if f == nil {
		m.screen = screenList
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.attendanceForm = nil
		m.screen = screenList
		return m, nil
	}
	if f.loading {
		return m, nil
	}
	switch msg.String() {
	case "up":
		if !f.checkOut && f.cursor > 0 {
			f.cursor--
		}
		return m, nil
	case "down":
		if !f.checkOut && f.cursor < len(f.locations)-1 {
			f.cursor++
		}
		return m, nil
	case "enter":
		if !f.checkOut && len(f.locations) == 0 {
			return m, nil
		}
		f.loading = true
		f.err = nil
		go m.saveAttendance(m.session.Current(), *f)
		return m, nil
	}
	var cmd tea.Cmd
	f.note, cmd = f.note.Update(msg)
	return m, cmd
}

func (m Model) viewAttendance() string {
	f := m.attendanceForm
	if f == nil {
		return ""
	}
	var b strings.Builder
	if f.checkOut {
		b.WriteString(titleStyle.Render("Check out") + "\n\n")
		b.WriteString(fmt.Sprintf("Checked in at %s since %s (%s).\n\n",
			f.session.WorkLocationName, formatClock(f.session.CheckIn),
			formatDuration(time.Since(f.session.CheckIn))))
	} else {
		b.WriteString(titleStyle.Render("Check in") + "\n\n")
		switch {
		case f.loading && len(f.locations) == 0:
			b.WriteString(m.spinner.View() + " Loading work locations...\n")
		default:
			for i, l := range f.locations {
				marker, style := "  ", subtleStyle
				if i == f.cursor {
					marker, style = "▸ ", focusedFieldStyle
				}
				b.WriteString(marker + style.Render(l.Name) + "\n")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString(fieldLabelStyle.Render("Note") + f.note.View() + "\n")

	if f.loading && (f.checkOut || len(f.locations) > 0) {
		b.WriteString("\n" + m.spinner.View() + " Saving...\n")
	}
	if f.err != nil {
		b.WriteString("\n" + errorStyle.Render("Error: "+f.err.Error()) + "\n")
	}

	help := "[↑/↓] location  [enter] check in  [esc] cancel"
	if f.checkOut {
		help = "[enter] check out  [esc] cancel"
	}
	b.WriteString("\n" + helpStyle.Render(help))
	return "\n" + boxStyle.Render(b.String()) + "\n"
}

// attendancePill renders the list header's check-in status, or "" when
// attendance isn't enabled for the active provider.
func (m Model) attendancePill() string {
	if m.attendance() == nil {
		return ""
	}
	switch {
	case m.attendanceErr != nil && !m.attendanceLoaded:
		return pillWarnStyle.Render("⚠ attendance unavailable")
	case !m.attendanceLoaded:
		return pillStyle.Render("… attendance")
	case m.attendanceSession != nil:
		s := m.attendanceSession
		return pillAccentStyle.Render(fmt.Sprintf("🟢 %s since %s (%s)",
			s.WorkLocationName, formatClock(s.CheckIn), formatDuration(time.Since(s.CheckIn))))
	default:
		return pillStyle.Render("⚪ checked out")
	}
}

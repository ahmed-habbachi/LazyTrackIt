package gui

import (
	"fmt"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// newTray puts LazyTrackIt in the system tray. Left-click toggles the
// window; the menu offers opening it, the check-in state (jumping straight
// to check in/out), launching at login, and quitting.
//
// On GNOME the tray needs the AppIndicator extension; without it the icon
// isn't shown, but the app still works through its window and launcher.
func newTray(app *application.App, window *application.WebviewWindow, svc *Service) *application.SystemTray {
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("LazyTrackIt")

	menu := app.NewMenu()
	menu.Add("Open LazyTrackIt").OnClick(func(*application.Context) { showWindow(window) })
	menu.AddSeparator()

	status := menu.Add("Attendance: loading…").SetEnabled(false).SetHidden(true)
	attendance := menu.Add("Check in…").SetHidden(true).OnClick(func(*application.Context) {
		showWindow(window)
		app.Event.Emit(EventNavigate, "attendance")
	})
	menu.AddSeparator()

	startAtLogin := menu.AddCheckbox("Start at login", false)
	if on, err := app.Autostart.IsEnabled(); err == nil {
		startAtLogin.SetChecked(on)
	}
	startAtLogin.OnClick(func(*application.Context) {
		// Ask the OS for the real state rather than trusting the checkbox,
		// which the window's own toggle may have changed meanwhile.
		on, err := app.Autostart.IsEnabled()
		if err != nil {
			on = !startAtLogin.Checked()
		}
		if err := svc.SetAutostart(!on); err != nil {
			startAtLogin.SetChecked(on)
			app.Dialog.Error().SetTitle("Start at login").SetMessage(err.Error()).Show()
		}
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) { app.Quit() })

	tray.SetMenu(menu)
	tray.OnClick(func() {
		if window.IsVisible() {
			window.Hide()
			return
		}
		showWindow(window)
	})

	svc.mu.Lock()
	svc.onAttendance = func(open *provider.AttendanceSession, enabled bool) {
		label, action, tooltip := attendanceLabels(open, time.Now())
		if !enabled {
			tooltip = "LazyTrackIt"
		}
		application.InvokeAsync(func() {
			status.SetLabel(label).SetHidden(!enabled)
			attendance.SetLabel(action).SetHidden(!enabled)
			tray.SetTooltip(tooltip)
			menu.Update()
		})
	}
	svc.onAutostart = func(enabled bool) {
		application.InvokeAsync(func() {
			startAtLogin.SetChecked(enabled)
			menu.Update()
		})
	}
	svc.mu.Unlock()
	return tray
}

// attendanceLabels renders the tray's status line, its check in/out action
// and its tooltip for the open session (nil when checked out).
func attendanceLabels(open *provider.AttendanceSession, now time.Time) (status, action, tooltip string) {
	if open == nil {
		return "Checked out", "Check in…", "LazyTrackIt — checked out"
	}
	since := open.CheckIn.Format("15:04")
	elapsed := now.Sub(open.CheckIn).Round(time.Minute)
	status = fmt.Sprintf("At %s since %s", open.WorkLocationName, since)
	tooltip = fmt.Sprintf("LazyTrackIt — %s since %s (%dh%02dm)", open.WorkLocationName, since, int(elapsed.Hours()), int(elapsed.Minutes())%60)
	return status, "Check out…", tooltip
}

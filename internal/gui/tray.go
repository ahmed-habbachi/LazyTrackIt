package gui

import (
	"fmt"
	"runtime"
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
	// On Linux the tooltip's title is the tray label, which Wails fills with
	// "Wails" when it's unset; on Windows the tooltip falls back to the
	// label. Setting only the label shows "TrackIt" on both.
	tray.SetLabel(trayLabel)

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
		label, action, detail := attendanceLabels(open, time.Now())
		if !enabled {
			detail = ""
		}
		application.InvokeAsync(func() {
			status.SetLabel(label).SetHidden(!enabled)
			attendance.SetLabel(action).SetHidden(!enabled)
			tray.SetTooltip(trayTooltip(detail, runtime.GOOS))
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

// trayLabel is the tray's name, shown as its tooltip title.
const trayLabel = "TrackIt"

// trayTooltip is the tooltip text for an attendance detail line ("" for
// none). Linux shows the tray label as the tooltip's title above this text,
// so the detail stands alone there; Windows has no title, so it's prefixed
// with the name. An empty tooltip falls back to the label on Windows.
func trayTooltip(detail, goos string) string {
	if detail == "" || goos == "linux" {
		return detail
	}
	return trayLabel + " — " + detail
}

// attendanceLabels renders the tray's status line, its check in/out action
// and the tooltip's detail line for the open session (nil when checked out).
func attendanceLabels(open *provider.AttendanceSession, now time.Time) (status, action, detail string) {
	if open == nil {
		return "Checked out", "Check in…", "Checked out"
	}
	// The provider may report the check-in in UTC; show it in local time,
	// as the window does.
	since := open.CheckIn.Local().Format("15:04")
	elapsed := now.Sub(open.CheckIn).Round(time.Minute)
	status = fmt.Sprintf("At %s since %s", open.WorkLocationName, since)
	detail = fmt.Sprintf("%s since %s (%dh%02dm)", open.WorkLocationName, since, int(elapsed.Hours()), int(elapsed.Minutes())%60)
	return status, "Check out…", detail
}

package gui

import (
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	checkInReminderID  = "lazytrackit-check-in-reminder"
	checkInCategoryID  = "lazytrackit-check-in"
	checkInActionID    = "check-in"
	checkInTitle       = "Time to check in"
	checkInReminderTxt = "You haven't checked in yet today."
)

// platformNotifier delivers the check-in reminder with the OS's notification
// system: D-Bus on Linux (notify_linux.go), the Wails notification service
// elsewhere (notify_wails.go).
type platformNotifier interface {
	// services are Wails services the notifier needs registered at startup.
	services() []application.Service
	// start readies it once the app runs; nil means notifications aren't
	// available, so reminders are skipped.
	start(app *application.App, window *application.WebviewWindow) reminderNotifier
}

// openCheckIn shows the window's check-in dialog, for a click on the
// reminder or its "Check in" button.
func openCheckIn(app *application.App, window *application.WebviewWindow) {
	application.InvokeAsync(func() {
		showWindow(window)
		app.Event.Emit(EventNavigate, "attendance")
	})
}

// writeNotificationIcon puts the app icon in the user cache directory, since
// Linux notification daemons take an image path, not bytes. It returns ""
// if that fails; the notification then just has no icon.
func writeNotificationIcon() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	dir = filepath.Join(dir, "lazytrackit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	path := filepath.Join(dir, "notification-icon.png")
	if err := os.WriteFile(path, appIcon, 0o644); err != nil {
		return ""
	}
	return path
}

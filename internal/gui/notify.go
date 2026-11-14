package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

const (
	checkInReminderID  = "lazytrackit-check-in-reminder"
	checkInCategoryID  = "lazytrackit-check-in"
	checkInActionID    = "check-in"
	checkInReminderTxt = "You haven't checked in yet today."
)

// optionalNotifications runs the Wails notification service without letting
// it take the app down: its startup can fail (e.g. no notification daemon
// on D-Bus, or a locked-down Windows registry), and a failing service
// startup aborts the whole app. Reminders are just skipped then. Wrapping it
// also keeps its methods out of the frontend bindings.
type optionalNotifications struct {
	ns    *notifications.NotificationService
	ready atomic.Bool
}

func (o *optionalNotifications) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
	if err := o.ns.ServiceStartup(ctx, opts); err != nil {
		fmt.Fprintln(os.Stderr, "lazytrackit-gui: notifications unavailable, check-in reminders are off:", err)
		return nil
	}
	o.ready.Store(true)
	return nil
}

func (o *optionalNotifications) ServiceShutdown() error {
	if o.ready.Load() {
		return o.ns.ServiceShutdown()
	}
	return nil
}

// desktopNotifier shows the check-in reminder through the OS notification
// service (D-Bus notifications on Linux, toasts on Windows).
type desktopNotifier struct {
	svc      *notifications.NotificationService
	iconPath string // the app icon on disk, for Linux notification daemons
}

// newDesktopNotifier wires the reminder's "Check in" button and a click on
// the notification itself to open the window's check-in dialog.
func newDesktopNotifier(ns *notifications.NotificationService, app *application.App, window *application.WebviewWindow) *desktopNotifier {
	ns.OnNotificationResponse(func(res notifications.NotificationResult) {
		if res.Error != nil || res.Response.ID != checkInReminderID {
			return
		}
		switch res.Response.ActionIdentifier {
		case checkInActionID, notifications.DefaultActionIdentifier:
			application.InvokeAsync(func() {
				showWindow(window)
				app.Event.Emit(EventNavigate, "attendance")
			})
		}
	})
	return &desktopNotifier{svc: ns, iconPath: writeNotificationIcon()}
}

// register declares the reminder's action button. It must run after the
// notification service has started.
func (n *desktopNotifier) register() {
	_ = n.svc.RegisterNotificationCategory(notifications.NotificationCategory{
		ID:      checkInCategoryID,
		Actions: []notifications.NotificationAction{{ID: checkInActionID, Title: "Check in"}},
	})
}

func (n *desktopNotifier) ShowCheckInReminder() {
	opts := notifications.NotificationOptions{
		ID:         checkInReminderID,
		Title:      "Time to check in",
		Body:       checkInReminderTxt,
		CategoryID: checkInCategoryID,
	}
	if n.iconPath != "" {
		opts.Attachments = []notifications.NotificationAttachment{{Path: n.iconPath}}
	}
	_ = n.svc.SendNotificationWithActions(opts)
}

func (n *desktopNotifier) ClearCheckInReminder() {
	_ = n.svc.RemoveNotification(checkInReminderID)
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

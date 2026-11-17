//go:build !linux

package gui

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// On Windows (and macOS) the reminder uses the Wails notification service:
// toast notifications, with its categories kept in the registry.

func newPlatformNotifier() platformNotifier {
	return &optionalNotifications{ns: notifications.New()}
}

func (o *optionalNotifications) services() []application.Service {
	return []application.Service{application.NewService(o)}
}

func (o *optionalNotifications) start(app *application.App, window *application.WebviewWindow) reminderNotifier {
	if !o.ready.Load() {
		return nil
	}
	dn := newDesktopNotifier(o.ns, app, window)
	dn.register()
	return dn
}

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
			openCheckIn(app, window)
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
		Title:      checkInTitle,
		Body:       checkInReminderTxt,
		CategoryID: checkInCategoryID,
	}
	if n.iconPath != "" {
		// On Windows an attachment without a type is a full-width inline
		// image, which blows the toast up; appLogoOverride shows it as the
		// small logo beside the text instead.
		opts.Attachments = []notifications.NotificationAttachment{{Path: n.iconPath, Type: "appLogoOverride"}}
	}
	_ = n.svc.SendNotificationWithActions(opts)
}

func (n *desktopNotifier) ClearCheckInReminder() {
	_ = n.svc.RemoveNotification(checkInReminderID)
}

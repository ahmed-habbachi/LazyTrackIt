//go:build linux

package gui

import (
	"fmt"
	"os"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// On Linux the reminder talks to the desktop's notification daemon
// (org.freedesktop.Notifications) directly over D-Bus. The Wails
// notification service would do the same, but it also creates a
// ~/.config/<app name> folder for its own bookkeeping on every start.

const (
	notifyDest  = "org.freedesktop.Notifications"
	notifyPath  = dbus.ObjectPath("/org/freedesktop/Notifications")
	notifyIface = "org.freedesktop.Notifications"
)

type dbusNotifier struct {
	conn     *dbus.Conn
	iconPath string
	onClick  func()

	mu sync.Mutex
	id uint32 // the reminder on screen (0 = none), replaced in place
}

func newPlatformNotifier() platformNotifier { return &dbusNotifier{} }

func (n *dbusNotifier) services() []application.Service { return nil }

func (n *dbusNotifier) start(app *application.App, window *application.WebviewWindow) reminderNotifier {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazytrackit-gui: no D-Bus session, check-in reminders are off:", err)
		return nil
	}
	if err := conn.AddMatchSignal(dbus.WithMatchInterface(notifyIface), dbus.WithMatchObjectPath(notifyPath)); err != nil {
		fmt.Fprintln(os.Stderr, "lazytrackit-gui: can't watch notification clicks:", err)
	}
	n.conn = conn
	n.iconPath = writeNotificationIcon()
	n.onClick = func() { openCheckIn(app, window) }

	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)
	go n.watch(signals)
	go func() {
		<-app.Context().Done()
		conn.Close()
	}()
	return n
}

// watch handles clicks on our notification ("default" is the body, which
// most daemons map to a plain click) and forgets it once it's closed.
func (n *dbusNotifier) watch(signals <-chan *dbus.Signal) {
	for sig := range signals {
		if len(sig.Body) < 2 {
			continue
		}
		id, _ := sig.Body[0].(uint32)
		n.mu.Lock()
		ours := id != 0 && id == n.id
		if ours && sig.Name == notifyIface+".NotificationClosed" {
			n.id = 0
		}
		n.mu.Unlock()
		if !ours || sig.Name != notifyIface+".ActionInvoked" {
			continue
		}
		if action, _ := sig.Body[1].(string); action == "default" || action == checkInActionID {
			n.onClick()
		}
	}
}

func (n *dbusNotifier) ShowCheckInReminder() {
	n.mu.Lock()
	replaces := n.id
	n.mu.Unlock()
	actions := []string{"default", "Open LazyTrackIt", checkInActionID, "Check in"}
	hints := map[string]dbus.Variant{"urgency": dbus.MakeVariant(byte(1))}
	var id uint32
	err := n.conn.Object(notifyDest, notifyPath).Call(notifyIface+".Notify", 0,
		"LazyTrackIt", replaces, n.iconPath, checkInTitle, checkInReminderTxt, actions, hints, int32(-1),
	).Store(&id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazytrackit-gui: showing the check-in reminder:", err)
		return
	}
	n.mu.Lock()
	n.id = id
	n.mu.Unlock()
}

func (n *dbusNotifier) ClearCheckInReminder() {
	n.mu.Lock()
	id := n.id
	n.id = 0
	n.mu.Unlock()
	if id != 0 {
		n.conn.Object(notifyDest, notifyPath).Call(notifyIface+".CloseNotification", 0, id)
	}
}

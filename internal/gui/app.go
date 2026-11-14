package gui

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/service"
	"github.com/ahmed-habbachi/lazytrackit/internal/update"
	"github.com/ahmed-habbachi/lazytrackit/internal/version"
)

//go:embed all:frontend
var frontendFS embed.FS

//go:embed icons/tray.png
var trayIcon []byte

//go:embed icons/app.png
var appIcon []byte

// HiddenFlag starts the app in the tray without showing the window. It's
// what the login item launches with, so starting with the system doesn't pop
// a window up in the user's face.
const HiddenFlag = "--hidden"

// appID identifies the app for single-instance locking.
const appID = "io.github.ahmed-habbachi.lazytrackit"

// Options configures Run.
type Options struct {
	// StartHidden keeps the window hidden until opened from the tray.
	StartHidden bool
}

// Run starts the desktop app and blocks until it quits.
func Run(opts Options) error {
	host := &wailsHost{}
	svc := buildService(host)

	assets, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		return fmt.Errorf("load frontend assets: %w", err)
	}

	services := []application.Service{application.NewService(svc)}
	var notifier *optionalNotifications
	if svc.session != nil && svc.remindersOn {
		notifier = &optionalNotifications{ns: notifications.New()}
		services = append(services, application.NewService(notifier))
	}

	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name:        "LazyTrackIt",
		Description: "Time tracking in your tray",
		Icon:        appIcon,
		Services:    services,
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: appID,
			// Launching it again (e.g. from the start menu) brings the
			// running instance's window forward instead of starting a
			// second tray icon.
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if window != nil {
					showWindow(window)
				}
			},
		},
		Windows: application.WindowsOptions{DisableQuitOnLastWindowClosed: true},
		Linux:   application.LinuxOptions{DisableQuitOnLastWindowClosed: true},
	})
	host.app = app

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     "LazyTrackIt",
		Width:     940,
		Height:    640,
		MinWidth:  420,
		MinHeight: 480,
		URL:       "/",
		Hidden:    opts.StartHidden,
	})
	// Closing the window only hides it: the app keeps running in the tray
	// until Quit is chosen from the tray menu.
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		window.Hide()
		e.Cancel()
	})

	newTray(app, window, svc)

	if svc.session != nil {
		go checkForUpdate(svc)
		app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
			if opts.StartHidden {
				// The frontend only boots once the window is shown, so fill
				// in the tray's check-in state from Go (never starting a
				// login).
				go svc.warmUp()
			}
			if notifier != nil && notifier.ready.Load() {
				startReminder(app, window, svc, notifier.ns)
			}
		})
	}
	return app.Run()
}

// startReminder starts the check-in reminder loop (see reminder.go) for the
// life of the app.
func startReminder(app *application.App, window *application.WebviewWindow, svc *Service, ns *notifications.NotificationService) {
	dn := newDesktopNotifier(ns, app, window)
	dn.register()
	r := newReminder(svc, svc.reminderAt, dn)
	svc.mu.Lock()
	svc.reminder = r
	svc.mu.Unlock()
	go r.run(app.Context())
}

// buildService loads the config and providers. Problems that the user has
// to fix (no config yet, an invalid one) don't stop the app: they're shown
// in the window instead, since a tray app has no terminal to print to.
func buildService(host Host) *Service {
	cfg, err := config.Load()
	if err != nil {
		if os.IsNotExist(err) {
			path, werr := config.WriteExample()
			if werr != nil {
				return NewSetupService(host, fmt.Sprintf("No configuration found, and writing a starter one failed: %v", werr))
			}
			return NewSetupService(host, fmt.Sprintf("No configuration found, so a starter config was written to %s. Edit it with your provider details, then restart LazyTrackIt.", path))
		}
		return NewSetupService(host, fmt.Sprintf("Couldn't read the configuration: %v", err))
	}

	providers, names, err := service.BuildProviders(cfg)
	if err != nil {
		return NewSetupService(host, err.Error())
	}
	active, _, err := cfg.ResolveActiveProvider()
	if err != nil {
		return NewSetupService(host, err.Error())
	}
	session, err := service.NewSession(providers, names, active, service.FileStateStore())
	if err != nil {
		return NewSetupService(host, err.Error())
	}
	session.RememberActiveProvider()
	svc := NewService(host, session)
	svc.updatesDisabled = !cfg.UpdatesEnabled()
	at, on, err := cfg.CheckInReminderTime()
	if err != nil {
		return NewSetupService(host, err.Error())
	}
	svc.reminderAt, svc.remindersOn = at, on
	return svc
}

// checkForUpdate installs a newer GUI release in the background, the same
// best-effort way the TUI does (see update.CheckAndApply).
func checkForUpdate(svc *Service) {
	if svc.updatesDisabled {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	res := update.CheckAndApplyProduct(ctx, version.Version, update.GUI)
	switch {
	case res.Installed:
		svc.setUpdateNotice(fmt.Sprintf("Update %s installed — restart LazyTrackIt to use it.", res.NewVersion))
	case res.Unwritable != "":
		svc.setUpdateNotice(fmt.Sprintf("Installed at %s, which you can't write to, so auto-update can't work. Move LazyTrackIt to a folder you own to enable it.", res.Unwritable))
	}
}

func showWindow(w *application.WebviewWindow) {
	w.Show()
	w.Restore()
	w.Focus()
}

// wailsHost implements Host on top of the running Wails app.
type wailsHost struct {
	app *application.App
}

func (h *wailsHost) Emit(name string, data any) {
	if h.app != nil {
		h.app.Event.Emit(name, data)
	}
}

func (h *wailsHost) OpenURL(url string) {
	if h.app != nil {
		_ = h.app.Browser.OpenURL(url)
	}
}

func (h *wailsHost) AutostartEnabled() (bool, error) {
	if h.app == nil {
		return false, nil
	}
	return h.app.Autostart.IsEnabled()
}

func (h *wailsHost) SetAutostart(enabled bool) error {
	if h.app == nil {
		return fmt.Errorf("app not running")
	}
	if !enabled {
		return h.app.Autostart.Disable()
	}
	// The registration ID is left to Wails' default (derived from the app
	// name) so IsEnabled and Disable, which use the default, find it.
	return h.app.Autostart.EnableWithOptions(application.AutostartOptions{
		Arguments: []string{HiddenFlag},
	})
}

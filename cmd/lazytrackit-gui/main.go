// Command lazytrackit-gui is LazyTrackIt's desktop app: a system-tray icon
// and window for the same time tracking the terminal UI offers, built with
// Wails v3. It shares all its logic with the TUI through internal/service.
//
// Pass --hidden to start in the tray without showing the window (this is
// what the "Start at login" registration launches with), and
// --test-notification to see the check-in reminder a few seconds after start.
package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/ahmed-habbachi/lazytrackit/internal/gui"
	"github.com/ahmed-habbachi/lazytrackit/internal/version"
)

func main() {
	args := os.Args[1:]
	if slices.Contains(args, "--version") || slices.Contains(args, "-v") {
		fmt.Println("lazytrackit-gui " + version.Version)
		return
	}

	opts := gui.Options{
		StartHidden:      slices.Contains(args, gui.HiddenFlag),
		TestNotification: slices.Contains(args, gui.TestNotificationFlag),
	}
	if err := gui.Run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "lazytrackit-gui:", err)
		os.Exit(1)
	}
}

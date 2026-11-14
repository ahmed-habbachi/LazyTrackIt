// Command lazytrackit is a terminal UI for tracking time against one or
// more configured backend APIs (currently: Case.TrackIt).
package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/service"
	"github.com/ahmed-habbachi/lazytrackit/internal/ui"
	"github.com/ahmed-habbachi/lazytrackit/internal/version"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("lazytrackit " + version.Version)
		return
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazytrackit:", err)
		pauseOnWindows()
		os.Exit(1)
	}
}

// pauseOnWindows waits for Enter before returning, and only on Windows.
// Double-clicking lazytrackit.exe (rather than running it from an
// already-open terminal) spawns a console that closes the instant the
// process exits, so without this a first-run message or error would flash
// and disappear before anyone could read it.
func pauseOnWindows() {
	if runtime.GOOS != "windows" {
		return
	}
	fmt.Print("\nPress Enter to exit...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		if os.IsNotExist(err) {
			return bootstrapConfig()
		}
		return err
	}

	providers, names, err := service.BuildProviders(cfg)
	if err != nil {
		return err
	}

	active, _, err := cfg.ResolveActiveProvider()
	if err != nil {
		return err
	}
	session, err := service.NewSession(providers, names, active, service.FileStateStore())
	if err != nil {
		return err
	}
	session.RememberActiveProvider()

	p := tea.NewProgram(ui.New(session, cfg.UpdatesEnabled()), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func bootstrapConfig() error {
	path, err := config.WriteExample()
	if err != nil {
		return err
	}
	fmt.Printf("No configuration found, so a starter config was written to:\n\n  %s\n\n", path)
	fmt.Println("Edit it with your Case.TrackIt base URL and Keycloak client_id, then run lazytrackit again.")
	pauseOnWindows()
	return nil
}

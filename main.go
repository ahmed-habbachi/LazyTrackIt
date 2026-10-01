// Command lazytrackit is a terminal UI for tracking time against one or
// more configured backend APIs (currently: Case.TrackIt).
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider/trackit"
	"github.com/ahmed-habbachi/lazytrackit/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazytrackit:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		if os.IsNotExist(err) {
			return bootstrapConfig()
		}
		return err
	}

	name, pcfg, err := cfg.Active()
	if err != nil {
		return err
	}

	var prov provider.Provider
	switch pcfg.Type {
	case "trackit", "":
		prov, err = trackit.New(name, pcfg)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown provider type %q for provider %q", pcfg.Type, name)
	}

	p := tea.NewProgram(ui.New(prov), tea.WithAltScreen())
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
	return nil
}

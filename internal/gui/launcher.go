package gui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/update"
	"github.com/ahmed-habbachi/lazytrackit/internal/version"
)

// The app is installed by copying one file, so on its first run it adds
// itself to the system's app launcher: a .desktop entry on Linux, a Start
// Menu shortcut on Windows (unless the installer already made one). A
// marker in the state directory remembers that it did, so removing the
// entry sticks instead of it coming back on the next start.
const launcherMarker = "launcher-created"

// ensureLauncher adds or refreshes the launcher entry for exe. Dev builds
// (go build / go run) are skipped, so a checkout doesn't end up in the
// app menu. Failures are only logged: the app works without an entry.
func ensureLauncher(exe string) {
	if version.Version == "dev" || version.Version == "" {
		return
	}
	dir, err := config.StateDir()
	if err != nil {
		return
	}
	marker := filepath.Join(dir, launcherMarker)
	_, statErr := os.Stat(marker)
	firstRun := statErr != nil
	packaged := update.PackageManaged()
	if err := installLauncher(exe, firstRun, packaged); err != nil {
		fmt.Fprintln(os.Stderr, "lazytrackit-gui: adding the app launcher entry:", err)
		return
	}
	// A package's entry isn't ours: leave the marker for a copy that
	// does write one (e.g. if the package is swapped for the tar.gz).
	if firstRun && !packaged {
		_ = os.MkdirAll(dir, 0o700)
		_ = os.WriteFile(marker, []byte("The app added its launcher entry; delete this file to have it added again.\n"), 0o600)
	}
}

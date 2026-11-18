//go:build linux

package gui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// launcherKey marks .desktop files this app wrote, so it only ever updates
// its own (and leaves one the user or a package made alone).
const launcherKey = "X-LazyTrackIt-Managed=true"

// installLauncher writes ~/.local/share/applications/lazytrackit.desktop
// and its icon. On later runs it keeps an entry it wrote pointing at the
// current binary (e.g. after moving it), but doesn't bring back one that
// was deleted.
//
// packaged means the system package manager installed the app (see
// update.PackageManaged): the package provides the entry, so none is
// written.
func installLauncher(exe string, firstRun, packaged bool) error {
	data, err := xdgDataHome()
	if err != nil {
		return err
	}
	path := filepath.Join(data, "applications", "lazytrackit.desktop")
	icon := filepath.Join(data, "icons", "hicolor", "256x256", "apps", "lazytrackit.png")
	if packaged {
		// The package installs its own entry; one written by an earlier
		// copy (e.g. from the tar.gz) would hide it, so drop that.
		if old, err := os.ReadFile(path); err == nil && bytes.Contains(old, []byte(launcherKey)) {
			return os.Remove(path)
		}
		return nil
	}
	want := []byte(desktopEntry(exe, icon))

	old, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !firstRun {
			return nil
		}
	case err != nil:
		return err
	case !bytes.Contains(old, []byte(launcherKey)), bytes.Equal(old, want):
		return writeIfChanged(icon, appIcon, 0o644)
	}
	if err := writeIfChanged(icon, appIcon, 0o644); err != nil {
		return err
	}
	return writeIfChanged(path, want, 0o644)
}

func desktopEntry(exe, icon string) string {
	return `[Desktop Entry]
Type=Application
Name=LazyTrackIt
GenericName=Time tracking
Comment=Log your work time and check in or out
Exec=` + desktopQuote(exe) + `
Icon=` + icon + `
Terminal=false
Categories=Office;ProjectManagement;
Keywords=time;tracking;timesheet;trackit;
StartupWMClass=lazytrackit-gui
` + launcherKey + "\n"
}

// desktopQuote quotes an Exec argument per the Desktop Entry spec.
func desktopQuote(s string) string {
	if !strings.ContainsAny(s, " \t\n\"'\\><~|&;$*?#()`%") {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`, `%`, `%%`)
	return `"` + r.Replace(s) + `"`
}

func xdgDataHome() (string, error) {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

func writeIfChanged(path string, data []byte, perm os.FileMode) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

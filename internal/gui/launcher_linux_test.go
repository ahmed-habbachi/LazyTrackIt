//go:build linux

package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxLauncher(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	entry := filepath.Join(data, "applications", "lazytrackit.desktop")
	icon := filepath.Join(data, "icons", "hicolor", "256x256", "apps", "lazytrackit.png")
	read := func() string {
		b, err := os.ReadFile(entry)
		if err != nil {
			return ""
		}
		return string(b)
	}

	// First run: entry and icon are written.
	if err := installLauncher("/home/me/.local/bin/lazytrackit-gui", true, false); err != nil {
		t.Fatal(err)
	}
	got := read()
	for _, want := range []string{"Exec=/home/me/.local/bin/lazytrackit-gui\n", "Icon=" + icon + "\n", "StartupWMClass=lazytrackit-gui\n", launcherKey} {
		if !strings.Contains(got, want) {
			t.Errorf("entry lacks %q:\n%s", want, got)
		}
	}
	if _, err := os.Stat(icon); err != nil {
		t.Errorf("icon not written: %v", err)
	}

	// Moved binary: our entry follows it.
	if err := installLauncher("/opt/My Apps/lazytrackit-gui", false, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(), `Exec="/opt/My Apps/lazytrackit-gui"`) {
		t.Errorf("entry not updated for the moved binary:\n%s", read())
	}

	// Deleted by the user: stays deleted.
	os.Remove(entry)
	if err := installLauncher("/home/me/.local/bin/lazytrackit-gui", false, false); err != nil {
		t.Fatal(err)
	}
	if read() != "" {
		t.Error("a deleted entry came back")
	}

	// Someone else's entry (no marker) is left alone.
	own := "[Desktop Entry]\nName=Mine\nExec=/usr/bin/lazytrackit-gui\n"
	os.WriteFile(entry, []byte(own), 0o644)
	if err := installLauncher("/home/me/.local/bin/lazytrackit-gui", true, false); err != nil {
		t.Fatal(err)
	}
	if read() != own {
		t.Errorf("overwrote an entry it didn't write:\n%s", read())
	}
}

func TestDesktopQuote(t *testing.T) {
	for in, want := range map[string]string{
		"/usr/bin/lt":      "/usr/bin/lt",
		"/a b/lt":          `"/a b/lt"`,
		`/a"b/$x%`:         `"/a\"b/\$x%%"`,
		"/home/me/100%/lt": `"/home/me/100%%/lt"`,
	} {
		if got := desktopQuote(in); got != want {
			t.Errorf("desktopQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestLinuxLauncherPackaged(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	entry := filepath.Join(data, "applications", "lazytrackit.desktop")

	// Installed from the package: nothing is written...
	if err := installLauncher("/usr/bin/lazytrackit-gui", true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(entry); !os.IsNotExist(err) {
		t.Errorf("packaged install wrote an entry (err %v)", err)
	}

	// ...and one an earlier unpackaged copy wrote is removed, so the
	// package's own entry shows.
	if err := installLauncher("/home/me/.local/bin/lazytrackit-gui", true, false); err != nil {
		t.Fatal(err)
	}
	if err := installLauncher("/usr/bin/lazytrackit-gui", false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(entry); !os.IsNotExist(err) {
		t.Errorf("our old entry wasn't removed (err %v)", err)
	}

	// Someone else's entry stays.
	own := "[Desktop Entry]\nName=Mine\n"
	os.WriteFile(entry, []byte(own), 0o644)
	if err := installLauncher("/usr/bin/lazytrackit-gui", false, true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(entry); string(b) != own {
		t.Errorf("someone else's entry was changed: %q", b)
	}
}

package update

import (
	"strings"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.0.2", "v0.0.1", true},
		{"v0.1.0", "v0.0.9", true},
		{"v1.0.0", "v0.9.9", true},
		{"v0.0.1", "v0.0.1", false},
		{"v0.0.1", "v0.0.2", false},
		{"v0.0.2", "dev", false},
		{"not-a-version", "v0.0.1", false},
		{"v0.0.2", "not-a-version", false},
	}
	for _, c := range cases {
		if got := isNewer(c.latest, c.current); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestFindAsset(t *testing.T) {
	assets := []asset{
		{Name: "lazytrackit-gui_v0.0.2_linux_amd64_desktop.tar.gz"},
		{Name: "lazytrackit_v0.0.2_linux_amd64.tar.gz"},
		{Name: "lazytrackit_v0.0.2_linux_amd64.tar.gz.sha256"},
		{Name: "lazytrackit_v0.0.2_windows_amd64.zip"},
	}

	a, ok := findAsset(assets, "lazytrackit_v0.0.2_linux_amd64.tar.gz")
	if !ok || a.Name != "lazytrackit_v0.0.2_linux_amd64.tar.gz" {
		t.Errorf("findAsset TUI linux = %+v, %v", a, ok)
	}

	a, ok = findAsset(assets, "lazytrackit-gui_v0.0.2_linux_amd64_desktop.tar.gz")
	if !ok || a.Name != "lazytrackit-gui_v0.0.2_linux_amd64_desktop.tar.gz" {
		t.Errorf("findAsset GUI linux = %+v, %v", a, ok)
	}

	if _, ok := findAsset(assets, "lazytrackit_v0.0.2_darwin_amd64.tar.gz"); ok {
		t.Error("findAsset should not match a platform with no published asset")
	}
}

// Released TUI versions select the first asset whose name ends in
// "_<os>_<arch>.<ext>"; a GUI archive must never match that, or those
// clients would download it instead of the TUI archive.
func TestGUIArchiveInvisibleToLegacySuffixMatch(t *testing.T) {
	for _, suffix := range []string{"_linux_amd64.tar.gz", "_linux_arm64.tar.gz", "_windows_amd64.zip"} {
		for _, ext := range []string{"tar.gz", "zip"} {
			for _, platform := range []string{"linux_amd64", "linux_arm64", "windows_amd64"} {
				name := "lazytrackit-gui_v1.2.3_" + platform + GUI.Variant + "." + ext
				if strings.HasSuffix(name, suffix) {
					t.Errorf("GUI archive %s matches legacy suffix %s", name, suffix)
				}
			}
		}
	}
	if !strings.Contains(GUI.archiveName("v1"), "_desktop.") {
		t.Errorf("GUI archive name %q lacks its variant", GUI.archiveName("v1"))
	}
}

func TestProductArchiveNamesDiffer(t *testing.T) {
	tui, gui := TUI.archiveName("v1.2.3"), GUI.archiveName("v1.2.3")
	if tui == gui {
		t.Fatalf("TUI and GUI archives collide: %s", tui)
	}
	if !strings.HasPrefix(tui, "lazytrackit_v1.2.3_") || !strings.HasPrefix(gui, "lazytrackit-gui_v1.2.3_") {
		t.Errorf("unexpected archive names %q, %q", tui, gui)
	}
	if TUI.binaryName() == GUI.binaryName() {
		t.Error("TUI and GUI binary names collide")
	}
}

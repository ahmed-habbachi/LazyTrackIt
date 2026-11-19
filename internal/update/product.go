package update

import (
	"fmt"
	"runtime"
)

// Product identifies which LazyTrackIt executable is updating itself. The
// terminal UI and the desktop GUI are published side by side in the same
// GitHub release, so each must pick out its own archive and binary rather
// than whichever asset happens to match the platform first.
type Product struct {
	// Name is both the archive prefix and the executable's base name inside
	// the archive.
	Name string
	// Variant, if set, is appended after the platform in the archive name
	// (Name_<tag>_<os>_<arch><Variant>.<ext>).
	Variant string
	// Deb, if set, is the name of the product's Debian package; an
	// install from it is updated by installing a newer package.
	Deb string
}

var (
	// TUI is the terminal UI, built from the repository root.
	TUI = Product{Name: "lazytrackit"}
	// GUI is the Wails desktop app, built from cmd/lazytrackit-gui. Its
	// "_desktop" variant keeps its archives from ending in
	// "_<os>_<arch>.<ext>": TUI releases from before per-product updates
	// pick the first asset with that suffix, and would otherwise grab the
	// GUI archive and fail to update.
	GUI = Product{Name: "lazytrackit-gui", Variant: "_desktop", Deb: "lazytrackit-desktop"}
)

// binaryName is the executable's file name inside the release archive.
func (p Product) binaryName() string {
	if runtime.GOOS == "windows" {
		return p.Name + ".exe"
	}
	return p.Name
}

// archiveName is the exact release asset name for tag on this platform,
// matching what .github/workflows/release.yml publishes.
func (p Product) archiveName(tag string) string {
	return fmt.Sprintf("%s_%s_%s_%s%s.%s", p.Name, tag, runtime.GOOS, runtime.GOARCH, p.Variant, archiveExt())
}

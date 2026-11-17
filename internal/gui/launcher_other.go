//go:build !linux && !windows

package gui

// installLauncher does nothing elsewhere (a macOS app bundle shows up in
// Launchpad on its own).
func installLauncher(string, bool) error { return nil }

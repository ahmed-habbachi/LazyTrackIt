package ui

import (
	"os/exec"
	"runtime"
)

// tryOpenBrowser best-effort opens url in the user's default browser.
// Failures are silently ignored: the URL and code are always shown on
// screen too, so this is purely a convenience.
func tryOpenBrowser(url string) {
	if url == "" {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

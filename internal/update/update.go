// Package update implements LazyTrackIt's self-update: on startup it checks
// the GitHub releases page for a newer tagged version and, if one exists and
// a build is published for the current OS/arch, downloads it, verifies its
// checksum, and swaps it in for the currently running executable. The new
// binary takes effect the next time LazyTrackIt is started — a running
// session is never interrupted.
package update

import (
	"context"
	"os"
	"runtime"
)

// CheckResult is the outcome of CheckAndApply. At most one of Installed and
// Unwritable is set; the zero value means nothing happened (up to date, or a
// failure that was logged and skipped).
type CheckResult struct {
	// Installed reports that NewVersion was downloaded and installed in
	// place of the running executable; it takes effect on next start.
	Installed  bool
	NewVersion string

	// Unwritable, if set, is the install directory the current user can't
	// write to, meaning self-update can never succeed from there.
	Unwritable string
}

// CheckAndApply checks GitHub for a release newer than currentVersion and, if
// one is found and a matching platform build is published, downloads,
// verifies, and installs it in place of the running executable.
//
// Any failure along the way (network error, rate limiting, no release for
// this platform, checksum mismatch, etc.) returns a zero CheckResult: this is
// a best-effort background convenience, not something worth interrupting the
// user over. The one failure reported back is an install directory that
// isn't writable (CheckResult.Unwritable), since that can never fix itself
// and the user has to move the binary. Every outcome, including a no-op and
// why, is appended to update.log in the config directory (see logf).
func CheckAndApply(ctx context.Context, currentVersion string) CheckResult {
	return CheckAndApplyProduct(ctx, currentVersion, TUI)
}

// CheckAndApplyProduct is CheckAndApply for a specific product, so the GUI
// only ever installs a GUI build over itself (and the TUI a TUI build).
func CheckAndApplyProduct(ctx context.Context, currentVersion string, product Product) CheckResult {
	if currentVersion == "dev" || currentVersion == "" {
		logf("skip: running a dev build (%q), nothing to compare against", currentVersion)
		return CheckResult{}
	}

	dir, writable, err := installDirWritable()
	if err != nil {
		logf("checking install directory failed: %v", err)
		return CheckResult{}
	}
	if !writable {
		logf("install directory %s is not writable by the current user — self-update can never succeed here", dir)
		return CheckResult{Unwritable: dir}
	}

	rel, err := fetchLatestRelease(ctx)
	if err != nil || rel == nil {
		logf("fetching latest release failed: %v", err)
		return CheckResult{}
	}
	if !isNewer(rel.TagName, currentVersion) {
		logf("up to date: latest release %s is not newer than running %s", rel.TagName, currentVersion)
		return CheckResult{}
	}
	logf("found newer release %s (running %s)", rel.TagName, currentVersion)

	archiveName := product.archiveName(rel.TagName)
	archiveAsset, ok := findAsset(rel.Assets, archiveName)
	if !ok {
		logf("no release asset published for this platform (%s)", archiveName)
		return CheckResult{}
	}
	checksumAsset, ok := findAsset(rel.Assets, archiveName+".sha256")
	if !ok {
		logf("no checksum asset found for %s", archiveAsset.Name)
		return CheckResult{}
	}

	archivePath, err := downloadToTemp(ctx, archiveAsset.BrowserDownloadURL, "lazytrackit-update-*"+archiveExtDot())
	if err != nil {
		logf("downloading %s failed: %v", archiveAsset.Name, err)
		return CheckResult{}
	}
	defer os.Remove(archivePath)

	wantSum, err := fetchChecksum(ctx, checksumAsset.BrowserDownloadURL)
	if err != nil {
		logf("fetching checksum failed: %v", err)
		return CheckResult{}
	}
	if err := verifySHA256(archivePath, wantSum); err != nil {
		logf("checksum verification failed: %v", err)
		return CheckResult{}
	}

	binaryPath, err := extractBinary(archivePath, product.binaryName())
	if err != nil {
		logf("extracting binary from archive failed: %v", err)
		return CheckResult{}
	}
	defer os.Remove(binaryPath)

	if err := install(binaryPath); err != nil {
		logf("installing update failed: %v", err)
		return CheckResult{}
	}

	logf("installed %s — will take effect on next restart", rel.TagName)
	return CheckResult{Installed: true, NewVersion: rel.TagName}
}

func archiveExt() string {
	if runtime.GOOS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

func archiveExtDot() string {
	return "." + archiveExt()
}

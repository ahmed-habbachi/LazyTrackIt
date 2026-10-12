// Package update implements LazyTrackIt's self-update: on startup it checks
// the GitHub releases page for a newer tagged version and, if one exists and
// a build is published for the current OS/arch, downloads it, verifies its
// checksum, and swaps it in for the currently running executable. The new
// binary takes effect the next time LazyTrackIt is started — a running
// session is never interrupted.
package update

import (
	"context"
	"fmt"
	"os"
	"runtime"
)

// CheckAndApply checks for a release newer than currentVersion and, if one
// is found and a matching platform build is published, downloads, verifies,
// and installs it in place of the running executable.
//
// It reports ok=true with the new version string only on a fully successful
// install. Any failure along the way (network error, rate limiting, no
// release for this platform, checksum mismatch, no write permission on the
// install directory, etc.) is treated as a no-op: this is a best-effort
// background convenience, not something worth interrupting or alarming the
// user over, so callers should simply skip showing anything when ok is
// false. Every outcome, including a no-op and why, is appended to update.log
// in the config directory (see logf) so a silent failure can still be
// diagnosed after the fact.
func CheckAndApply(ctx context.Context, currentVersion string) (newVersion string, ok bool) {
	// "dev" (the default for local go-run/go-build builds) has nothing
	// meaningful to compare a release tag against.
	if currentVersion == "dev" || currentVersion == "" {
		logf("skip: running a dev build (%q), nothing to compare against", currentVersion)
		return "", false
	}

	rel, err := fetchLatestRelease(ctx)
	if err != nil || rel == nil {
		logf("fetching latest release failed: %v", err)
		return "", false
	}
	if !isNewer(rel.TagName, currentVersion) {
		logf("up to date: latest release %s is not newer than running %s", rel.TagName, currentVersion)
		return "", false
	}
	logf("found newer release %s (running %s)", rel.TagName, currentVersion)

	archiveSuffix := fmt.Sprintf("_%s_%s.%s", runtime.GOOS, runtime.GOARCH, archiveExt())
	archiveAsset, ok := findAsset(rel.Assets, archiveSuffix)
	if !ok {
		logf("no release asset published for this platform (suffix %s)", archiveSuffix)
		return "", false
	}
	checksumAsset, ok := findAsset(rel.Assets, archiveAsset.Name+".sha256")
	if !ok {
		logf("no checksum asset found for %s", archiveAsset.Name)
		return "", false
	}

	archivePath, err := downloadToTemp(ctx, archiveAsset.BrowserDownloadURL, "lazytrackit-update-*"+archiveExtDot())
	if err != nil {
		logf("downloading %s failed: %v", archiveAsset.Name, err)
		return "", false
	}
	defer os.Remove(archivePath)

	wantSum, err := fetchChecksum(ctx, checksumAsset.BrowserDownloadURL)
	if err != nil {
		logf("fetching checksum failed: %v", err)
		return "", false
	}
	if err := verifySHA256(archivePath, wantSum); err != nil {
		logf("checksum verification failed: %v", err)
		return "", false
	}

	binaryPath, err := extractBinary(archivePath)
	if err != nil {
		logf("extracting binary from archive failed: %v", err)
		return "", false
	}
	defer os.Remove(binaryPath)

	if err := install(binaryPath); err != nil {
		logf("installing update failed: %v", err)
		return "", false
	}

	logf("installed %s — will take effect on next restart", rel.TagName)
	return rel.TagName, true
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

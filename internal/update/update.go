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
// false.
func CheckAndApply(ctx context.Context, currentVersion string) (newVersion string, ok bool) {
	// "dev" (the default for local go-run/go-build builds) has nothing
	// meaningful to compare a release tag against.
	if currentVersion == "dev" || currentVersion == "" {
		return "", false
	}

	rel, err := fetchLatestRelease(ctx)
	if err != nil || rel == nil {
		return "", false
	}
	if !isNewer(rel.TagName, currentVersion) {
		return "", false
	}

	archiveSuffix := fmt.Sprintf("_%s_%s.%s", runtime.GOOS, runtime.GOARCH, archiveExt())
	archiveAsset, ok := findAsset(rel.Assets, archiveSuffix)
	if !ok {
		return "", false // no build published for this OS/arch
	}
	checksumAsset, ok := findAsset(rel.Assets, archiveAsset.Name+".sha256")
	if !ok {
		return "", false
	}

	archivePath, err := downloadToTemp(ctx, archiveAsset.BrowserDownloadURL, "lazytrackit-update-*"+archiveExtDot())
	if err != nil {
		return "", false
	}
	defer os.Remove(archivePath)

	wantSum, err := fetchChecksum(ctx, checksumAsset.BrowserDownloadURL)
	if err != nil {
		return "", false
	}
	if err := verifySHA256(archivePath, wantSum); err != nil {
		return "", false
	}

	binaryPath, err := extractBinary(archivePath)
	if err != nil {
		return "", false
	}
	defer os.Remove(binaryPath)

	if err := install(binaryPath); err != nil {
		return "", false
	}

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

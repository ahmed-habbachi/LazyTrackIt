package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// The desktop app is also released as a Debian package (see
// .github/workflows/release.yml), which installs it to /usr/bin. That's
// root-owned, so it can't replace itself there: instead it tells the user
// a newer package is available (CheckResult.Available) and, once they've
// installed it, that a restart will start it (CheckResult.Installed).

// systemBinDir is where the Debian package installs the executable.
const systemBinDir = "/usr/bin"

// PackageManaged reports whether the running executable was installed by
// the system package manager (it lives in /usr/bin), so it's updated by
// installing a new package rather than replacing itself.
func PackageManaged() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	exe, err := currentExecutable()
	return err == nil && filepath.Dir(exe) == systemBinDir
}

// DebVersion turns a release tag into the package's Debian version: no "v",
// and a pre-release suffix sorting before the release ("1.2.0-rc1" →
// "1.2.0~rc1"). It must match how the release workflow names the package.
func DebVersion(tag string) string {
	return strings.ReplaceAll(strings.TrimPrefix(tag, "v"), "-", "~")
}

// debName is the release asset name of p's Debian package for tag.
func (p Product) debName(tag string) string {
	return fmt.Sprintf("%s_%s_%s.deb", p.Deb, DebVersion(tag), runtime.GOARCH)
}

// startedFrom is the executable as it was when this process started, so a
// package upgrade replacing it (a new file at the same path) is noticed.
var startedFrom os.FileInfo

func init() {
	if exe, err := currentExecutable(); err == nil {
		startedFrom, _ = os.Stat(exe)
	}
}

// replacedOnDisk reports whether the executable at the running program's
// path is no longer the file it started from.
func replacedOnDisk() bool {
	if startedFrom == nil {
		return false
	}
	exe, err := currentExecutable()
	if err != nil {
		return false
	}
	now, err := os.Stat(exe)
	return err == nil && !os.SameFile(startedFrom, now)
}

// checkPackage is CheckAndApplyProduct for a package-managed install: it
// never installs anything itself.
func checkPackage(ctx context.Context, currentVersion string, product Product) CheckResult {
	rel, err := fetchLatestRelease(ctx)
	if err != nil || rel == nil {
		logf("fetching latest release failed: %v", err)
		return CheckResult{Failed: true}
	}
	if replacedOnDisk() {
		logf("the package was upgraded (to %s or later) since this instance started", rel.TagName)
		return CheckResult{Installed: true, NewVersion: rel.TagName}
	}
	if !isNewer(rel.TagName, currentVersion) {
		logf("up to date: latest release %s is not newer than running %s", rel.TagName, currentVersion)
		return CheckResult{}
	}
	name := product.debName(rel.TagName)
	a, ok := findAsset(rel.Assets, name)
	if !ok {
		logf("found newer release %s, but no package %s in it", rel.TagName, name)
		return CheckResult{}
	}
	logf("found newer release %s (running %s); installed by the package manager, so it's offered for download", rel.TagName, currentVersion)
	return CheckResult{Available: rel.TagName, DownloadURL: a.BrowserDownloadURL}
}

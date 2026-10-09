package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// binaryName is the file this project's build produces, inside both the
// .tar.gz (Linux) and .zip (Windows) release archives.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "lazytrackit.exe"
	}
	return "lazytrackit"
}

// downloadToTemp streams url's body into a new temp file and returns its
// path. The caller owns the file and should remove it once done.
func downloadToTemp(ctx context.Context, url, pattern string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "lazytrackit-updater")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: unexpected status %s", url, resp.Status)
	}

	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// fetchChecksum downloads a sha256sum-formatted file (one "<hex>  <name>"
// line, as produced by `sha256sum`) and returns just the hex digest.
func fetchChecksum(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "lazytrackit-updater")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: unexpected status %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return "", fmt.Errorf("%s: empty checksum file", url)
	}
	return strings.ToLower(fields[0]), nil
}

// verifySHA256 checks that path's contents hash to wantHex.
func verifySHA256(path, wantHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, wantHex) {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, wantHex)
	}
	return nil
}

// extractBinary pulls binaryName() out of a release archive (.tar.gz on
// Linux, .zip on Windows) and writes it to a new temp file, returning its
// path with the executable bit already set (a no-op on Windows).
func extractBinary(archivePath string) (string, error) {
	if runtime.GOOS == "windows" {
		return extractFromZip(archivePath)
	}
	return extractFromTarGz(archivePath)
}

func extractFromTarGz(archivePath string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	want := binaryName()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return "", fmt.Errorf("%s not found in archive", want)
		}
		if err != nil {
			return "", err
		}
		if filepath.Base(hdr.Name) != want {
			continue
		}
		return writeTemp(tr, 0o755)
	}
}

func extractFromZip(archivePath string) (string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	want := binaryName()
	for _, zf := range zr.File {
		if filepath.Base(zf.Name) != want {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		return writeTemp(rc, 0o755)
	}
	return "", fmt.Errorf("%s not found in archive", want)
}

func writeTemp(r io.Reader, perm os.FileMode) (string, error) {
	out, err := os.CreateTemp("", "lazytrackit-new-*")
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, r); err != nil {
		os.Remove(out.Name())
		return "", err
	}
	if err := out.Chmod(perm); err != nil {
		os.Remove(out.Name())
		return "", err
	}
	return out.Name(), nil
}

// install replaces the currently running executable with newBinaryPath.
// It stages the new file next to the current one (so the final swap is a
// same-filesystem rename, not a cross-device copy) and keeps a ".old"
// backup it restores from if anything goes wrong partway through.
//
// This works even though the current executable is "in use": both Linux and
// Windows allow renaming a running program's file out from under it (the
// loader has already mapped its contents into memory), which is the same
// trick most self-updating tools rely on.
func install(newBinaryPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(exePath)
	staged := filepath.Join(dir, ".lazytrackit-new")
	if err := copyFile(newBinaryPath, staged, 0o755); err != nil {
		return err
	}

	backup := exePath + ".old"
	os.Remove(backup) // best-effort: drop any leftover from a previous update

	if err := os.Rename(exePath, backup); err != nil {
		os.Remove(staged)
		return fmt.Errorf("renaming current executable aside: %w", err)
	}
	if err := os.Rename(staged, exePath); err != nil {
		// Best-effort rollback so a failed update doesn't leave the user
		// without a working binary at all.
		_ = os.Rename(backup, exePath)
		return fmt.Errorf("installing new executable: %w", err)
	}
	_ = os.Remove(backup) // best-effort; fine if it lingers, e.g. still open on some platform
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

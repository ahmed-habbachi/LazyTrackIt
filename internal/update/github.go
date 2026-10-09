package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// repoOwner and repoName identify the GitHub repository that publishes
// LazyTrackIt releases (see .github/workflows/release.yml).
const (
	repoOwner = "ahmed-habbachi"
	repoName  = "LazyTrackIt"
)

const latestReleaseURL = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"

// asset is one file attached to a GitHub release.
type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// release is the subset of GitHub's release API response this package uses.
type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

// fetchLatestRelease asks GitHub for the most recent published (non-draft,
// non-prerelease) release of the LazyTrackIt repository.
func fetchLatestRelease(ctx context.Context) (*release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return nil, err
	}
	// GitHub's API rejects requests with no User-Agent, and this header
	// pins the response shape to the version this code was written against.
	req.Header.Set("User-Agent", "lazytrackit-updater")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: unexpected status %s fetching latest release", resp.Status)
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("github: decoding latest release: %w", err)
	}
	return &rel, nil
}

// findAsset returns the first asset in assets whose name ends with suffix.
func findAsset(assets []asset, suffix string) (asset, bool) {
	for _, a := range assets {
		if strings.HasSuffix(a.Name, suffix) {
			return a, true
		}
	}
	return asset{}, false
}

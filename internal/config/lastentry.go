package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LastEntryDefaults remembers the project and tags most recently used to
// create a time entry, so the next new entry can default to them instead of
// making the user re-pick every time. The user is usually logging time
// against the same project/tags across several entries in a row.
type LastEntryDefaults struct {
	ProjectID string   `json:"project_id"`
	TagIDs    []string `json:"tag_ids"`
}

// lastEntryStateDir returns the directory LazyTrackIt stores per-run UI
// state in, honoring XDG_STATE_HOME when set.
func lastEntryStateDir() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "lazytrackit"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "lazytrackit"), nil
}

func lastEntryPath(providerName string) (string, error) {
	dir, err := lastEntryStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "last-entry-"+providerName+".json"), nil
}

// LoadLastEntryDefaults reads the last-used project/tags for a provider. A
// missing file is not an error; it returns a zero value so callers fall
// back to their own defaults.
func LoadLastEntryDefaults(providerName string) (LastEntryDefaults, error) {
	path, err := lastEntryPath(providerName)
	if err != nil {
		return LastEntryDefaults{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return LastEntryDefaults{}, nil
		}
		return LastEntryDefaults{}, err
	}
	var d LastEntryDefaults
	if err := json.Unmarshal(data, &d); err != nil {
		return LastEntryDefaults{}, err
	}
	return d, nil
}

// SaveLastEntryDefaults writes the last-used project/tags for a provider,
// creating the state directory as needed.
func SaveLastEntryDefaults(providerName string, d LastEntryDefaults) error {
	path, err := lastEntryPath(providerName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// StoreDir returns the directory tokens are cached in, honoring
// XDG_STATE_HOME when set, and otherwise defaulting to the platform's
// normal place for local (non-roaming) app state: %LOCALAPPDATA% on
// Windows (cached tokens are machine-local, unlike hand-edited config), or
// ~/.local/state elsewhere.
func StoreDir() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "lazytrackit", "tokens"), nil
	}
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "lazytrackit", "tokens"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "lazytrackit", "tokens"), nil
}

func tokenPath(providerName string) (string, error) {
	dir, err := StoreDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, providerName+".json"), nil
}

// LoadTokens reads cached tokens for a provider. A missing file is not an
// error; it returns (nil, nil) so callers can fall through to login.
func LoadTokens(providerName string) (*Tokens, error) {
	path, err := tokenPath(providerName)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var t Tokens
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// SaveTokens writes tokens for a provider, creating the directory as needed
// and restricting permissions since this file holds bearer credentials.
func SaveTokens(providerName string, t *Tokens) error {
	path, err := tokenPath(providerName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

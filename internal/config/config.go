// Package config loads and saves LazyTrackIt's YAML configuration file.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProviderConfig describes one configured backend API. Auth is left
// undecoded here: its shape depends on Type (an OIDC issuer/client_id for
// one provider, an API key for another), so each provider package decodes
// it into its own config struct via Auth.Decode.
type ProviderConfig struct {
	Type    string    `yaml:"type"`
	BaseURL string    `yaml:"base_url"`
	Auth    yaml.Node `yaml:"auth"`
}

// Config is the root configuration document.
type Config struct {
	ActiveProvider string                    `yaml:"active_provider"`
	Providers      map[string]ProviderConfig `yaml:"providers"`
}

// Dir returns the directory LazyTrackIt stores its config in, honoring
// XDG_CONFIG_HOME when set.
func Dir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lazytrackit"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lazytrackit"), nil
}

// Path returns the full path to config.yaml.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

const exampleConfig = `# LazyTrackIt configuration
# Pick which configured provider is active.
active_provider: trackit

providers:
  trackit:
    type: trackit
    # Base URL of your Case.TrackIt API (no trailing slash).
    base_url: https://trackit.case-tunisia.com
    auth:
      # Keycloak (or any OIDC) realm issuer URL.
      issuer: https://auth2.case-tunisia.com/realms/case
      # Client registered in Keycloak with the "Device Authorization Grant" enabled.
      client_id: lazytrackit
      # Only needed if the client is confidential.
      client_secret: ""
      scopes:
        - openid
        - profile
        - offline_access
`

// WriteExample writes a commented example config file to Path(), creating
// the parent directory if needed. It refuses to overwrite an existing file.
func WriteExample() (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("config already exists at %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(exampleConfig), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// Load reads and parses the config file at Path().
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}

// Active returns the currently selected provider's configuration.
func (c *Config) Active() (string, ProviderConfig, error) {
	if c.ActiveProvider == "" {
		return "", ProviderConfig{}, fmt.Errorf("active_provider is not set in config")
	}
	p, ok := c.Providers[c.ActiveProvider]
	if !ok {
		return "", ProviderConfig{}, fmt.Errorf("active_provider %q has no matching entry under providers", c.ActiveProvider)
	}
	return c.ActiveProvider, p, nil
}

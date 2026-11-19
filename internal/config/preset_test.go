package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestStarterConfigWithoutPresetIsTheExample(t *testing.T) {
	if Preset() {
		t.Skip("built with a preset")
	}
	if starterConfig() != exampleConfig {
		t.Error("starter config differs from the example without a preset")
	}
}

func TestPresetConfig(t *testing.T) {
	doc := presetConfig(exampleConfig, "https://trackit.example.org", "https://sso.example.org/realms/acme", "my-client", true)
	var cfg Config
	if err := yaml.Unmarshal([]byte(doc), &cfg); err != nil {
		t.Fatal(err)
	}
	p, ok := cfg.Providers["trackit"]
	if !ok {
		t.Fatal("trackit provider missing")
	}
	a := decodeAuth(t, p)
	if p.BaseURL != "https://trackit.example.org" || a.Issuer != "https://sso.example.org/realms/acme" ||
		a.ClientID != "my-client" || !p.Attendance {
		t.Errorf("preset not applied: %+v %+v", p, a)
	}
	if len(cfg.Providers) != 1 {
		t.Errorf("commented examples should stay commented, got providers %v", cfg.Providers)
	}

	// Without a client ID or attendance, those keep the starter's values.
	doc = presetConfig(exampleConfig, "https://t.example.org", "https://a.example.org/realms/x", "", false)
	cfg = Config{}
	if err := yaml.Unmarshal([]byte(doc), &cfg); err != nil {
		t.Fatal(err)
	}
	if p := cfg.Providers["trackit"]; decodeAuth(t, p).ClientID != "lazytrackit" || p.Attendance {
		t.Errorf("defaults changed: %+v", p)
	}
}

type testAuth struct {
	Issuer   string `yaml:"issuer"`
	ClientID string `yaml:"client_id"`
}

func decodeAuth(t *testing.T, p ProviderConfig) testAuth {
	t.Helper()
	var a testAuth
	if err := p.Auth.Decode(&a); err != nil {
		t.Fatal(err)
	}
	return a
}

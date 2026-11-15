package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type testEntry struct {
	Type string            `yaml:"type"`
	Auth map[string]string `yaml:"auth"`
}

var newEntry = testEntry{Type: "toggl", Auth: map[string]string{"api_token": "tok"}}

const newEntryText = `  toggl2:
    type: toggl
    auth:
      api_token: tok`

func TestAddProviderStarterConfigKeepsEverythingElse(t *testing.T) {
	out, err := addProviderYAML([]byte(exampleConfig), "toggl2", newEntry)
	if err != nil {
		t.Fatal(err)
	}
	// Removing the inserted lines must give back the original byte for byte.
	if got := strings.Replace(string(out), newEntryText+"\n", "", 1); got != exampleConfig {
		t.Fatalf("other lines changed:\n%s", out)
	}
	// It goes after the trackit entry's own trailing comment and before the
	// commented-out examples.
	idx := strings.Index(string(out), newEntryText)
	if att := strings.Index(string(out), "# attendance: true"); att > idx {
		t.Error("inserted before the trackit entry's attendance comment")
	}
	if ex := strings.Index(string(out), "# Add as many more providers"); ex < idx {
		t.Error("inserted after the commented-out examples")
	}
	var cfg Config
	if err := yaml.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Providers["toggl2"].Type != "toggl" || cfg.Providers["trackit"].Type != "trackit" {
		t.Errorf("providers after edit = %+v", cfg.Providers)
	}
}

func TestAddProviderFollowsExistingIndent(t *testing.T) {
	doc := "providers:\n    a:\n        type: toggl\nactive_provider: a\n"
	out, err := addProviderYAML([]byte(doc), "b", newEntry)
	if err != nil {
		t.Fatal(err)
	}
	want := "providers:\n    a:\n        type: toggl\n    b:\n      type: toggl\n      auth:\n        api_token: tok\nactive_provider: a\n"
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestAddProviderNoOrEmptyProvidersSection(t *testing.T) {
	for _, doc := range []string{"active_provider: x\n", "active_provider: x\nproviders:\n", "providers:\nactive_provider: x\n"} {
		out, err := addProviderYAML([]byte(doc), "x", newEntry)
		if err != nil {
			t.Fatalf("%q: %v", doc, err)
		}
		var cfg Config
		if err := yaml.Unmarshal(out, &cfg); err != nil {
			t.Fatalf("%q: %v\n%s", doc, err, out)
		}
		if cfg.Providers["x"].Type != "toggl" || cfg.ActiveProvider != "x" {
			t.Errorf("%q: got %+v from\n%s", doc, cfg, out)
		}
	}
}

func TestAddProviderRefusals(t *testing.T) {
	if _, err := addProviderYAML([]byte(exampleConfig), "trackit", newEntry); !errors.Is(err, ErrProviderExists) {
		t.Errorf("duplicate name: err = %v", err)
	}
	if _, err := addProviderYAML([]byte("providers: {a: {type: toggl}}\n"), "b", newEntry); err == nil {
		t.Error("flow-style providers: no error")
	}
}

func TestWriteFileAtomicKeepsMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("a: 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("a: 2\n")); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "a: 2\n" || st.Mode().Perm() != 0o640 {
		t.Errorf("content %q mode %v", data, st.Mode().Perm())
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("leftover temp files: %v", entries)
	}
}

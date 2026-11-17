package config

import (
	"errors"
	"strings"
	"testing"
)

const twoProviders = `# My config
active_provider: work # start here

providers:
  # The main one.
  work:
    type: trackit
    base_url: https://old.example.com
    # a note about work
    auth:
      issuer: https://auth.example.com
      client_id: lt

  toggl:
    type: toggl
    auth:
      api_token: tok

  # Commented-out example, at the entries' level:
  # other:
  #   type: toggl
check_for_updates: false
`

func TestUpdateProviderReplacesOnlyItsLines(t *testing.T) {
	out, err := updateProviderYAML([]byte(twoProviders), "work", testEntry{Type: "toggl", Auth: map[string]string{"api_token": "new"}})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(twoProviders, `  work:
    type: trackit
    base_url: https://old.example.com
    # a note about work
    auth:
      issuer: https://auth.example.com
      client_id: lt
`, `  work:
    type: toggl
    auth:
      api_token: new
`, 1)
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestUpdateProviderLastEntryKeepsTrailingComments(t *testing.T) {
	out, err := updateProviderYAML([]byte(twoProviders), "toggl", testEntry{Type: "toggl", Auth: map[string]string{"api_token": "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "      api_token: x\n\n  # Commented-out example") {
		t.Errorf("trailing comments moved:\n%s", out)
	}
}

func TestRemoveProviderRepointsActive(t *testing.T) {
	out, err := removeProviderYAML([]byte(twoProviders), "work", "toggl")
	if err != nil {
		t.Fatal(err)
	}
	want := `# My config
active_provider: toggl # start here

providers:
  toggl:
    type: toggl
    auth:
      api_token: tok

  # Commented-out example, at the entries' level:
  # other:
  #   type: toggl
check_for_updates: false
`
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestRemoveProviderActiveBelow(t *testing.T) {
	doc := "providers:\n  a:\n    type: toggl\n  b:\n    type: toggl\nactive_provider: a\n"
	out, err := removeProviderYAML([]byte(doc), "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if want := "providers:\n  b:\n    type: toggl\nactive_provider: b\n"; string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestRemoveProviderInactive(t *testing.T) {
	out, err := removeProviderYAML([]byte(twoProviders), "toggl", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "toggl:\n") || !strings.Contains(string(out), "active_provider: work # start here") {
		t.Errorf("got:\n%s", out)
	}
	if strings.Contains(string(out), "\n\n\n") {
		t.Errorf("left a double blank line:\n%s", out)
	}
}

func TestEditMissingProvider(t *testing.T) {
	if _, err := updateProviderYAML([]byte(twoProviders), "nope", newEntry); !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("update: %v", err)
	}
	if _, err := removeProviderYAML([]byte(twoProviders), "nope", ""); !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("remove: %v", err)
	}
	if _, err := removeProviderYAML([]byte(twoProviders), "work", ""); err == nil {
		t.Error("removing the active_provider without a fallback: no error")
	}
}

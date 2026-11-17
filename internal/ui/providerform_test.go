package ui

import (
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ahmed-habbachi/lazytrackit/internal/service"
)

func keys(f *providerForm) []string {
	var out []string
	for _, fld := range f.fields() {
		out = append(out, fld.key)
	}
	return out
}

func TestProviderFormFieldsFollowType(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // no config to copy defaults from
	f := newProviderForm()
	if got := keys(f); !slices.Equal(got, []string{"name", "type", "baseURL", "issuer", "clientID", "clientSecret", "scopes", "attendance"}) {
		t.Errorf("trackit fields = %v", got)
	}
	if got := f.inputs["scopes"].Value(); got != "openid profile offline_access" {
		t.Errorf("default scopes = %q", got)
	}

	m := Model{providerForm: f, screen: screenAddProvider}
	press := func(k tea.KeyMsg) {
		next, _ := m.handleProviderFormKey(k)
		m = next.(Model)
	}
	press(tea.KeyMsg{Type: tea.KeyTab}) // to "type"
	press(tea.KeyMsg{Type: tea.KeyRight})
	if got := keys(m.providerForm); !slices.Equal(got, []string{"name", "type", "baseURL", "apiToken"}) {
		t.Errorf("toggl fields = %v", got)
	}
	if d := m.providerForm.draft(); d.Type != "toggl" {
		t.Errorf("draft type = %q", d.Type)
	}
}

func TestProviderFormTypingAndDraft(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := Model{providerForm: newProviderForm(), screen: screenAddProvider}
	press := func(k tea.KeyMsg) {
		next, _ := m.handleProviderFormKey(k)
		m = next.(Model)
	}
	for _, r := range "test" {
		press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Jump to the attendance toggle (last field) and switch it on.
	press(tea.KeyMsg{Type: tea.KeyShiftTab})
	press(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	d := m.providerForm.draft()
	if d.Name != "test" || !d.Attendance || d.Type != "trackit" {
		t.Errorf("draft = %+v", d)
	}
	press(tea.KeyMsg{Type: tea.KeyEsc})
	if m.providerForm != nil || m.screen != screenProviderPicker {
		t.Errorf("esc: form=%v screen=%v", m.providerForm, m.screen)
	}
}

func TestEditProviderFormPrefillsAndFixesName(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	f := newEditProviderForm(service.ProviderDraft{Name: "tg", Type: "toggl", APIToken: "tok", BaseURL: "https://p.example.com"})
	if got := keys(f); !slices.Equal(got, []string{"type", "baseURL", "apiToken"}) {
		t.Errorf("fields = %v", got)
	}
	if f.focused() != "type" {
		t.Errorf("focus starts on %q", f.focused())
	}
	d := f.draft()
	if d.Name != "tg" || d.Type != "toggl" || d.APIToken != "tok" || d.BaseURL != "https://p.example.com" {
		t.Errorf("draft = %+v", d)
	}
}

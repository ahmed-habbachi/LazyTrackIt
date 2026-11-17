package service

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// captureSaves replaces config writing for the test and records the calls.
func captureSaves(t *testing.T) *[]string {
	t.Helper()
	var saved []string
	orig := saveProviderConfig
	saveProviderConfig = func(name string, value any) error {
		b, err := yaml.Marshal(value)
		if err != nil {
			return err
		}
		saved = append(saved, name+"\n"+string(b))
		return nil
	}
	t.Cleanup(func() { saveProviderConfig = orig })
	return &saved
}

func TestAddProviderTrackit(t *testing.T) {
	saved := captureSaves(t)
	s, _ := newTestSession(t, newFake("work"))
	name, err := s.AddProvider(ProviderDraft{
		Name: " second ", Type: "trackit", BaseURL: "https://trackit.example.com/",
		Issuer: "https://auth.example.com/realms/r", ClientID: "lazytrackit",
		APIToken: "ignored for trackit", Attendance: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "second" {
		t.Errorf("name = %q", name)
	}
	if got := s.ProviderNames(); !slices.Equal(got, []string{"second", "work"}) {
		t.Errorf("names = %v", got)
	}
	if err := s.SwitchProvider("second"); err != nil {
		t.Errorf("switching to the new provider: %v", err)
	}
	want := `second
type: trackit
base_url: https://trackit.example.com
auth:
    issuer: https://auth.example.com/realms/r
    client_id: lazytrackit
    scopes:
        - openid
        - profile
        - offline_access
attendance: true
`
	if len(*saved) != 1 || (*saved)[0] != want {
		t.Errorf("saved:\n%v\nwant:\n%s", *saved, want)
	}
}

func TestAddProviderTokenTypes(t *testing.T) {
	saved := captureSaves(t)
	s, _ := newTestSession(t, newFake("work"))
	if _, err := s.AddProvider(ProviderDraft{Name: "t", Type: "toggl", APIToken: "tok", Issuer: "x", Attendance: true}); err != nil {
		t.Fatalf("toggl without base URL: %v", err)
	}
	if !strings.Contains((*saved)[0], "api_token: tok") || strings.Contains((*saved)[0], "base_url") ||
		strings.Contains((*saved)[0], "issuer") || strings.Contains((*saved)[0], "attendance") {
		t.Errorf("toggl entry:\n%s", (*saved)[0])
	}
	if _, err := s.AddProvider(ProviderDraft{Name: "w", Type: "weekbooking", APIToken: "tok"}); err == nil {
		t.Error("week booking without base URL: no error")
	}
}

func TestAddProviderValidation(t *testing.T) {
	saved := captureSaves(t)
	s, _ := newTestSession(t, newFake("work"))
	ok := ProviderDraft{Name: "n", Type: "weekbooking", BaseURL: "https://x.example.com", APIToken: "tok"}
	cases := map[string]func(d *ProviderDraft){
		"empty name":     func(d *ProviderDraft) { d.Name = " " },
		"bad name":       func(d *ProviderDraft) { d.Name = "my/provider" },
		"duplicate name": func(d *ProviderDraft) { d.Name = "WORK" },
		"unknown type":   func(d *ProviderDraft) { d.Type = "jira" },
		"bad base URL":   func(d *ProviderDraft) { d.BaseURL = "x.example.com" },
		"no token":       func(d *ProviderDraft) { d.APIToken = "" },
		"trackit no issuer": func(d *ProviderDraft) {
			*d = ProviderDraft{Name: "n", Type: "trackit", BaseURL: "https://x.example.com", ClientID: "c"}
		},
		"trackit no client id": func(d *ProviderDraft) {
			*d = ProviderDraft{Name: "n", Type: "trackit", BaseURL: "https://x.example.com", Issuer: "https://a.example.com"}
		},
	}
	for name, mutate := range cases {
		d := ok
		mutate(&d)
		if _, err := s.AddProvider(d); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if len(*saved) != 0 {
		t.Errorf("invalid drafts were saved: %v", *saved)
	}
	if got := s.ProviderNames(); !slices.Equal(got, []string{"work"}) {
		t.Errorf("names = %v", got)
	}
}

func TestAddProviderSaveFailureLeavesSessionAlone(t *testing.T) {
	orig := saveProviderConfig
	saveProviderConfig = func(string, any) error { return errors.New("disk full") }
	t.Cleanup(func() { saveProviderConfig = orig })
	s, _ := newTestSession(t, newFake("work"))
	_, err := s.AddProvider(ProviderDraft{Name: "t", Type: "toggl", APIToken: "tok"})
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("err = %v", err)
	}
	if got := s.ProviderNames(); !slices.Equal(got, []string{"work"}) {
		t.Errorf("names = %v", got)
	}
}

func TestNewProviderDraftCopiesTrackitLogin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg := `providers:
  work:
    type: trackit
    base_url: https://trackit.example.com
    auth:
      issuer: https://auth.example.com/realms/r
      client_id: lazytrackit
      scopes: [openid, offline_access]
`
	if err := os.MkdirAll(filepath.Join(dir, "lazytrackit"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lazytrackit", "config.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	d := NewProviderDraft("trackit")
	if d.Issuer != "https://auth.example.com/realms/r" || d.ClientID != "lazytrackit" ||
		!slices.Equal(d.Scopes, []string{"openid", "offline_access"}) || d.BaseURL != "" {
		t.Errorf("draft = %+v", d)
	}
	if d := NewProviderDraft("toggl"); d.Issuer != "" || len(d.Scopes) != 0 {
		t.Errorf("toggl draft = %+v", d)
	}
}

func TestParseScopes(t *testing.T) {
	if got := ParseScopes(" openid, profile  offline_access,"); !slices.Equal(got, []string{"openid", "profile", "offline_access"}) {
		t.Errorf("got %v", got)
	}
}

// stubEdits replaces the config writers and saved-login cleanup, recording
// what they were asked to do.
type editLog struct {
	updated, removed, loginsDeleted []string
	removeFallback                  string
	old                             ProviderDraft
}

func stubEdits(t *testing.T) *editLog {
	t.Helper()
	l := &editLog{}
	origU, origR, origD, origL := updateProviderConfig, removeProviderConfig, deleteSavedLogin, loadProviderDraft
	updateProviderConfig = func(name string, _ any) error { l.updated = append(l.updated, name); return nil }
	removeProviderConfig = func(name, fallback string) error {
		l.removed = append(l.removed, name)
		l.removeFallback = fallback
		return nil
	}
	deleteSavedLogin = func(name string) error { l.loginsDeleted = append(l.loginsDeleted, name); return nil }
	loadProviderDraft = func(string) (ProviderDraft, error) { return l.old, nil }
	t.Cleanup(func() {
		updateProviderConfig, removeProviderConfig, deleteSavedLogin, loadProviderDraft = origU, origR, origD, origL
	})
	return l
}

func TestUpdateProviderActiveSwitchesOver(t *testing.T) {
	l := stubEdits(t)
	l.old = ProviderDraft{Name: "a", Type: "toggl", APIToken: "old"}
	s, _ := newTestSession(t, newFake("a"), newFake("b"))
	before := s.Current()
	active, err := s.UpdateProvider(ProviderDraft{Name: "a", Type: "toggl", APIToken: "new"})
	if err != nil || !active {
		t.Fatalf("active=%v err=%v", active, err)
	}
	if s.isCurrent(before.gen) {
		t.Error("the active provider's generation wasn't renewed")
	}
	if len(l.updated) != 1 || len(l.loginsDeleted) != 0 {
		t.Errorf("updated=%v loginsDeleted=%v (token change shouldn't drop a login)", l.updated, l.loginsDeleted)
	}
	if active, _ := s.UpdateProvider(ProviderDraft{Name: "b", Type: "toggl", APIToken: "x"}); active {
		t.Error("an inactive provider reported active")
	}
}

func TestUpdateProviderLoginChangeDropsSavedLogin(t *testing.T) {
	l := stubEdits(t)
	l.old = ProviderDraft{Name: "a", Type: "trackit", Issuer: "https://old.example.com", ClientID: "c"}
	s, _ := newTestSession(t, newFake("a"), newFake("b"))
	_, err := s.UpdateProvider(ProviderDraft{Name: "a", Type: "trackit", BaseURL: "https://t.example.com", Issuer: "https://new.example.com", ClientID: "c"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(l.loginsDeleted, []string{"a"}) {
		t.Errorf("loginsDeleted = %v", l.loginsDeleted)
	}
}

func TestUpdateProviderErrors(t *testing.T) {
	l := stubEdits(t)
	s, _ := newTestSession(t, newFake("a"), newFake("b"))
	if _, err := s.UpdateProvider(ProviderDraft{Name: "nope", Type: "toggl", APIToken: "x"}); err == nil {
		t.Error("unknown provider: no error")
	}
	if _, err := s.UpdateProvider(ProviderDraft{Name: "a", Type: "toggl"}); err == nil {
		t.Error("missing token: no error")
	}
	if len(l.updated) != 0 {
		t.Errorf("saved despite errors: %v", l.updated)
	}
}

func TestRemoveProvider(t *testing.T) {
	l := stubEdits(t)
	s, store := newTestSession(t, newFake("a"), newFake("b"), newFake("c"))

	// Inactive: active_provider stays pointed at the active one.
	if next, err := s.RemoveProvider("b"); err != nil || next != "" {
		t.Fatalf("remove b: next=%q err=%v", next, err)
	}
	if l.removeFallback != "a" || !slices.Equal(s.ProviderNames(), []string{"a", "c"}) {
		t.Errorf("fallback=%q names=%v", l.removeFallback, s.ProviderNames())
	}

	// Active: switches to another and remembers it.
	next, err := s.RemoveProvider("a")
	if err != nil || next != "c" {
		t.Fatalf("remove a: next=%q err=%v", next, err)
	}
	if s.ActiveProviderName() != "c" || l.removeFallback != "c" || store.lastProvider != "c" {
		t.Errorf("active=%q fallback=%q remembered=%q", s.ActiveProviderName(), l.removeFallback, store.lastProvider)
	}
	if !slices.Equal(l.loginsDeleted, []string{"b", "a"}) {
		t.Errorf("loginsDeleted = %v", l.loginsDeleted)
	}

	// The last one can't go.
	if _, err := s.RemoveProvider("c"); err == nil {
		t.Error("removed the only provider")
	}
	if _, err := s.RemoveProvider("zzz"); err == nil {
		t.Error("removed an unknown provider")
	}
}

func TestProviderDraftFor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg := `providers:
  work:
    base_url: https://t.example.com
    attendance: true
    auth:
      issuer: https://auth.example.com
      client_id: lt
      client_secret: s
  tg:
    type: toggl
    auth:
      api_token: tok
`
	if err := os.MkdirAll(filepath.Join(dir, "lazytrackit"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lazytrackit", "config.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := ProviderDraftFor("work")
	if err != nil || d.Type != "trackit" || d.Issuer != "https://auth.example.com" || d.ClientSecret != "s" || !d.Attendance {
		t.Errorf("work = %+v, %v", d, err)
	}
	d, err = ProviderDraftFor("tg")
	if err != nil || d.Type != "toggl" || d.APIToken != "tok" {
		t.Errorf("tg = %+v, %v", d, err)
	}
	if _, err := ProviderDraftFor("nope"); err == nil {
		t.Error("unknown provider: no error")
	}
}

package service

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ahmed-habbachi/lazytrackit/internal/auth"
	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// ProviderType describes a backend type a provider can be added for, so
// front ends can build their "add provider" forms from one list.
type ProviderType struct {
	ID    string // the config's "type"
	Label string
	// Fields lists which ProviderDraft fields the type uses, by name:
	// "baseURL", "issuer", "clientID", "clientSecret", "scopes", "apiToken",
	// "attendance".
	Fields []string
	// BaseURLOptional is true when an empty base URL means a default.
	BaseURLOptional bool
}

// ProviderTypes are the types that can be added, in display order.
var ProviderTypes = []ProviderType{
	{ID: "trackit", Label: "TrackIt", Fields: []string{"baseURL", "issuer", "clientID", "clientSecret", "scopes", "attendance"}},
	{ID: "toggl", Label: "Toggl Track", Fields: []string{"baseURL", "apiToken"}, BaseURLOptional: true},
	{ID: "weekbooking", Label: "Week booking (Zeiterfassung)", Fields: []string{"baseURL", "apiToken"}},
}

// LookupProviderType returns the ProviderType with id.
func LookupProviderType(id string) (ProviderType, bool) {
	for _, t := range ProviderTypes {
		if t.ID == id {
			return t, true
		}
	}
	return ProviderType{}, false
}

// Uses reports whether the type uses the named draft field.
func (t ProviderType) Uses(field string) bool { return slices.Contains(t.Fields, field) }

// DefaultScopes are requested by a new TrackIt provider unless changed.
var DefaultScopes = []string{"openid", "profile", "offline_access"}

// ProviderDraft is a provider being added from a form.
type ProviderDraft struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	BaseURL string `json:"baseURL"`
	// TrackIt (OIDC device flow)
	Issuer       string   `json:"issuer"`
	ClientID     string   `json:"clientID"`
	ClientSecret string   `json:"clientSecret"`
	Scopes       []string `json:"scopes"`
	Attendance   bool     `json:"attendance"`
	// Toggl / week booking
	APIToken string `json:"apiToken"`
}

// NewProviderDraft returns a draft of type typ with sensible defaults. For a
// TrackIt provider it copies the login settings (issuer, client ID, scopes)
// of an already-configured TrackIt provider, since a second account or
// environment usually shares them.
func NewProviderDraft(typ string) ProviderDraft {
	d := ProviderDraft{Type: typ}
	if typ != "trackit" {
		return d
	}
	d.Scopes = slices.Clone(DefaultScopes)
	cfg, err := config.Load()
	if err != nil {
		return d
	}
	names := make([]string, 0, len(cfg.Providers))
	for name := range cfg.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := cfg.Providers[name]
		if p.Type != "trackit" && p.Type != "" {
			continue
		}
		var a trackitAuth
		if p.Auth.Decode(&a) == nil && a.Issuer != "" {
			d.Issuer, d.ClientID = a.Issuer, a.ClientID
			if len(a.Scopes) > 0 {
				d.Scopes = a.Scopes
			}
			break
		}
	}
	return d
}

// ParseScopes splits a space- or comma-separated scope list.
func ParseScopes(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' })
}

var providerNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,39}$`)

// normalized trims the draft's text fields and drops the fields its type
// doesn't use.
func (d ProviderDraft) normalized() ProviderDraft {
	t, _ := LookupProviderType(d.Type)
	pick := func(field, v string) string {
		if t.Uses(field) {
			return strings.TrimSpace(v)
		}
		return ""
	}
	n := ProviderDraft{
		Name:         strings.TrimSpace(d.Name),
		Type:         d.Type,
		BaseURL:      strings.TrimRight(pick("baseURL", d.BaseURL), "/"),
		Issuer:       strings.TrimRight(pick("issuer", d.Issuer), "/"),
		ClientID:     pick("clientID", d.ClientID),
		ClientSecret: pick("clientSecret", d.ClientSecret),
		APIToken:     pick("apiToken", d.APIToken),
		Attendance:   t.Uses("attendance") && d.Attendance,
	}
	if t.Uses("scopes") {
		for _, s := range d.Scopes {
			if s = strings.TrimSpace(s); s != "" {
				n.Scopes = append(n.Scopes, s)
			}
		}
		if len(n.Scopes) == 0 {
			n.Scopes = slices.Clone(DefaultScopes)
		}
	}
	return n
}

// Validate checks a (normalized) draft against the existing provider names.
func (d ProviderDraft) Validate(existing []string) error {
	switch {
	case d.Name == "":
		return errors.New("give the provider a name")
	case !providerNameRE.MatchString(d.Name):
		return errors.New("the name can only use letters, digits, '.', '-' and '_' (up to 40 characters)")
	}
	for _, n := range existing {
		if strings.EqualFold(n, d.Name) {
			return fmt.Errorf("there's already a provider called %q", n)
		}
	}
	t, ok := LookupProviderType(d.Type)
	if !ok {
		return fmt.Errorf("pick a provider type")
	}
	if d.BaseURL == "" && !t.BaseURLOptional {
		return errors.New("enter the base URL")
	}
	if d.BaseURL != "" {
		if err := checkURL("base URL", d.BaseURL); err != nil {
			return err
		}
	}
	if t.Uses("issuer") {
		if d.Issuer == "" {
			return errors.New("enter the login issuer URL")
		}
		if err := checkURL("issuer URL", d.Issuer); err != nil {
			return err
		}
	}
	if t.Uses("clientID") && d.ClientID == "" {
		return errors.New("enter the client ID")
	}
	if t.Uses("apiToken") && d.APIToken == "" {
		return errors.New("enter the API token")
	}
	return nil
}

func checkURL(what, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("the %s must be a full web address, like https://example.com", what)
	}
	return nil
}

// The config entries, field order as in the starter config.
type trackitAuth struct {
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret,omitempty"`
	Scopes       []string `yaml:"scopes,omitempty"`
}

type tokenAuth struct {
	APIToken string `yaml:"api_token"`
}

type providerEntry struct {
	Type       string `yaml:"type"`
	BaseURL    string `yaml:"base_url,omitempty"`
	Auth       any    `yaml:"auth"`
	Attendance bool   `yaml:"attendance,omitempty"`
}

// entry is the config.yaml body for a (normalized) draft.
func (d ProviderDraft) entry() providerEntry {
	e := providerEntry{Type: d.Type, BaseURL: d.BaseURL, Attendance: d.Attendance}
	if d.Type == "trackit" {
		e.Auth = trackitAuth{Issuer: d.Issuer, ClientID: d.ClientID, ClientSecret: d.ClientSecret, Scopes: d.Scopes}
	} else {
		e.Auth = tokenAuth{APIToken: d.APIToken}
	}
	return e
}

// providerConfig is the entry as the config package would load it.
func (d ProviderDraft) providerConfig() (config.ProviderConfig, error) {
	var pc config.ProviderConfig
	b, err := yaml.Marshal(d.entry())
	if err != nil {
		return pc, err
	}
	err = yaml.Unmarshal(b, &pc)
	return pc, err
}

// saveProviderConfig writes a new provider to config.yaml; tests replace it.
var saveProviderConfig = config.AddProvider

// AddProvider validates draft, saves it to config.yaml and makes it
// available to switch to, without restarting. It returns the provider's
// name. It doesn't switch to it; callers do that if they want to.
func (s *Session) AddProvider(draft ProviderDraft) (string, error) {
	d := draft.normalized()
	if err := d.Validate(s.ProviderNames()); err != nil {
		return "", err
	}
	pcfg, err := d.providerConfig()
	if err != nil {
		return "", err
	}
	prov, err := buildProvider(d.Name, pcfg)
	if err != nil {
		return "", err
	}
	if err := saveProviderConfig(d.Name, d.entry()); err != nil {
		if errors.Is(err, config.ErrProviderExists) {
			return "", fmt.Errorf("there's already a provider called %q in the config file", d.Name)
		}
		return "", fmt.Errorf("couldn't save the provider: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[d.Name] = prov
	s.names = append(s.names, d.Name)
	sort.Strings(s.names)
	return d.Name, nil
}

// buildProvider is newProvider plus the attendance capability check that
// BuildProviders does.
func buildProvider(name string, pcfg config.ProviderConfig) (provider.Provider, error) {
	prov, err := newProvider(name, pcfg)
	if err != nil {
		return nil, err
	}
	if pcfg.Attendance {
		if _, ok := prov.(provider.Attendance); !ok {
			return nil, fmt.Errorf("provider %q has attendance enabled, but type %q doesn't support check-in/check-out", name, pcfg.Type)
		}
	}
	return prov, nil
}

// ProviderDraftFor loads provider name from config.yaml into a draft, to
// edit it.
func ProviderDraftFor(name string) (ProviderDraft, error) {
	cfg, err := config.Load()
	if err != nil {
		return ProviderDraft{}, err
	}
	p, ok := cfg.Providers[name]
	if !ok {
		return ProviderDraft{}, fmt.Errorf("provider %q isn't in the config file", name)
	}
	d := ProviderDraft{Name: name, Type: p.Type, BaseURL: p.BaseURL, Attendance: p.Attendance}
	if d.Type == "" {
		d.Type = "trackit"
	}
	if d.Type == "trackit" {
		var a trackitAuth
		if err := p.Auth.Decode(&a); err != nil {
			return ProviderDraft{}, fmt.Errorf("reading provider %q: %w", name, err)
		}
		d.Issuer, d.ClientID, d.ClientSecret, d.Scopes = a.Issuer, a.ClientID, a.ClientSecret, a.Scopes
	} else {
		var a tokenAuth
		if err := p.Auth.Decode(&a); err != nil {
			return ProviderDraft{}, fmt.Errorf("reading provider %q: %w", name, err)
		}
		d.APIToken = a.APIToken
	}
	return d, nil
}

// Config writers and the saved-login cleanup; tests replace them.
var (
	updateProviderConfig = config.UpdateProvider
	removeProviderConfig = config.RemoveProvider
	deleteSavedLogin     = auth.DeleteTokens
	loadProviderDraft    = ProviderDraftFor
)

// UpdateProvider saves new settings for the existing provider draft.Name
// (it can't be renamed: its saved login and remembered state are keyed by
// the name). If its login settings changed, the saved login is dropped so
// the next use signs in afresh. It reports whether the provider is the
// active one; if so the session has already switched over to the new
// settings, and the caller should reload (as after SwitchProvider).
func (s *Session) UpdateProvider(draft ProviderDraft) (active bool, err error) {
	d := draft.normalized()
	names := s.ProviderNames()
	if !slices.Contains(names, d.Name) {
		return false, fmt.Errorf("provider %q is not configured", d.Name)
	}
	others := slices.DeleteFunc(slices.Clone(names), func(n string) bool { return n == d.Name })
	if err := d.Validate(others); err != nil {
		return false, err
	}
	pcfg, err := d.providerConfig()
	if err != nil {
		return false, err
	}
	prov, err := buildProvider(d.Name, pcfg)
	if err != nil {
		return false, err
	}
	old, oldErr := loadProviderDraft(d.Name)
	if err := updateProviderConfig(d.Name, d.entry()); err != nil {
		if errors.Is(err, config.ErrProviderNotFound) {
			return false, fmt.Errorf("provider %q isn't in the config file anymore", d.Name)
		}
		return false, fmt.Errorf("couldn't save the provider: %w", err)
	}
	if oldErr != nil || old.Type != d.Type || old.Issuer != d.Issuer || old.ClientID != d.ClientID {
		_ = deleteSavedLogin(d.Name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[d.Name] = prov
	if s.active != d.Name {
		return false, nil
	}
	return true, s.switchLocked(d.Name)
}

// RemoveProvider deletes provider name from config.yaml and the session,
// along with its saved login. The last provider can't be removed. If it
// was the active one, the session switches to another and returns its name
// (the caller then reloads, as after SwitchProvider); otherwise it returns
// "".
func (s *Session) RemoveProvider(name string) (newActive string, err error) {
	names := s.ProviderNames()
	if !slices.Contains(names, name) {
		return "", fmt.Errorf("provider %q is not configured", name)
	}
	if len(names) == 1 {
		return "", errors.New("you can't remove your only provider; add another one first")
	}
	fallback := ""
	for _, n := range names {
		if n != name {
			fallback = n
			break
		}
	}
	active := s.ActiveProviderName()
	if active == name {
		newActive = fallback
	}
	// If active_provider in the file names it, point it at whichever
	// provider is (or is about to be) active.
	target := active
	if newActive != "" {
		target = newActive
	}
	if err := removeProviderConfig(name, target); err != nil {
		if errors.Is(err, config.ErrProviderNotFound) {
			return "", fmt.Errorf("provider %q isn't in the config file anymore", name)
		}
		return "", fmt.Errorf("couldn't remove the provider: %w", err)
	}
	_ = deleteSavedLogin(name)

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.providers, name)
	s.names = slices.DeleteFunc(s.names, func(n string) bool { return n == name })
	if s.active == name {
		if err := s.switchLocked(newActive); err != nil {
			return "", err
		}
	}
	return newActive, nil
}

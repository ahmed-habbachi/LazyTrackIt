package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ahmed-habbachi/lazytrackit/internal/service"
)

// providerForm backs screenAddProvider: adding a provider to config.yaml
// from inside the app, or editing one (editing is its name, which can't
// change). Which fields show depends on the chosen type (see
// service.ProviderTypes).
type providerForm struct {
	editing string
	typeIdx int
	inputs  map[string]*textinput.Model // by service field name, plus "name"
	attend  bool
	focus   int // index into fields()
	err     string
}

// providerField is one row of the form.
type providerField struct {
	key, label string
}

const (
	fieldKeyName   = "name"
	fieldKeyType   = "type"
	fieldKeyAttend = "attendance"
)

var providerFieldLabels = map[string]string{
	fieldKeyName:   "Name",
	fieldKeyType:   "Type",
	"baseURL":      "Base URL",
	"issuer":       "Login issuer",
	"clientID":     "Client ID",
	"clientSecret": "Client secret",
	"scopes":       "Scopes",
	"apiToken":     "API token",
	fieldKeyAttend: "Check-in/out",
}

func newProviderForm() *providerForm {
	mk := func(placeholder string, secret bool) *textinput.Model {
		ti := newTextInput(placeholder, "", 300, 44)
		if secret {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		return &ti
	}
	f := &providerForm{inputs: map[string]*textinput.Model{
		fieldKeyName:   mk("e.g. trackit-test (letters, digits, - _ .)", false),
		"baseURL":      mk("https://…", false),
		"issuer":       mk("https://auth.example.com/realms/yourrealm", false),
		"clientID":     mk("lazytrackit", false),
		"clientSecret": mk("only for confidential clients", true),
		"scopes":       mk("openid profile offline_access", false),
		"apiToken":     mk("paste the token", true),
	}}
	f.applyDefaults()
	f.focusCurrent()
	return f
}

// newEditProviderForm builds the form filled in with an existing provider.
func newEditProviderForm(d service.ProviderDraft) *providerForm {
	f := newProviderForm()
	f.editing = d.Name
	for i, t := range service.ProviderTypes {
		if t.ID == d.Type {
			f.typeIdx = i
		}
	}
	set := func(key, v string) { f.inputs[key].SetValue(v) }
	set(fieldKeyName, d.Name)
	set("baseURL", d.BaseURL)
	set("issuer", d.Issuer)
	set("clientID", d.ClientID)
	set("clientSecret", d.ClientSecret)
	set("scopes", strings.Join(d.Scopes, " "))
	set("apiToken", d.APIToken)
	f.attend = d.Attendance
	f.focus = 0 // the name row is hidden when editing: start on the type
	f.focusCurrent()
	return f
}

func (f *providerForm) providerType() service.ProviderType {
	return service.ProviderTypes[f.typeIdx]
}

// applyDefaults fills the type's defaults into fields that are still empty
// (e.g. a TrackIt login copied from an existing TrackIt provider).
func (f *providerForm) applyDefaults() {
	t := f.providerType()
	d := service.NewProviderDraft(t.ID)
	set := func(key, v string) {
		if in := f.inputs[key]; in.Value() == "" {
			in.SetValue(v)
		}
	}
	set("issuer", d.Issuer)
	set("clientID", d.ClientID)
	set("scopes", strings.Join(d.Scopes, " "))
	base := "https://…"
	if t.BaseURLOptional {
		base = "optional; leave empty for the default"
	}
	f.inputs["baseURL"].Placeholder = base
}

// fields lists the rows shown for the current type.
func (f *providerForm) fields() []providerField {
	keys := []string{fieldKeyName, fieldKeyType}
	if f.editing != "" {
		keys = keys[1:] // the name is shown as the title instead
	}
	keys = append(keys, f.providerType().Fields...)
	out := make([]providerField, len(keys))
	for i, k := range keys {
		out[i] = providerField{key: k, label: providerFieldLabels[k]}
	}
	return out
}

func (f *providerForm) focused() string { return f.fields()[f.focus].key }

func (f *providerForm) focusCurrent() {
	for _, in := range f.inputs {
		in.Blur()
	}
	if in, ok := f.inputs[f.focused()]; ok {
		in.Focus()
	}
}

func (f *providerForm) move(delta int) {
	if f.focus >= len(f.fields()) {
		f.focus = 0
	}
	n := len(f.fields())
	f.focus = (f.focus + delta + n) % n
	f.focusCurrent()
}

// draft collects the form into a service.ProviderDraft.
func (f *providerForm) draft() service.ProviderDraft {
	v := func(k string) string { return f.inputs[k].Value() }
	return service.ProviderDraft{
		Name:         f.name(),
		Type:         f.providerType().ID,
		BaseURL:      v("baseURL"),
		Issuer:       v("issuer"),
		ClientID:     v("clientID"),
		ClientSecret: v("clientSecret"),
		Scopes:       service.ParseScopes(v("scopes")),
		APIToken:     v("apiToken"),
		Attendance:   f.attend,
	}
}

func (f *providerForm) name() string {
	if f.editing != "" {
		return f.editing
	}
	return f.inputs[fieldKeyName].Value()
}

// openEditProviderForm opens the form for provider name.
func (m Model) openEditProviderForm(name string) (tea.Model, tea.Cmd) {
	d, err := service.ProviderDraftFor(name)
	if err != nil {
		return m, m.setStatus("Can't edit it: " + err.Error())
	}
	m.providerForm = newEditProviderForm(d)
	m.screen = screenAddProvider
	return m, textinput.Blink
}

func (m Model) openProviderForm() (tea.Model, tea.Cmd) {
	m.providerForm = newProviderForm()
	m.screen = screenAddProvider
	return m, textinput.Blink
}

func (m Model) handleProviderFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.providerForm
	if f == nil {
		m.screen = screenList
		return m, nil
	}
	key := f.focused()

	switch msg.String() {
	case "esc":
		m.providerForm = nil
		m.screen = screenProviderPicker
		return m, nil
	case "ctrl+s":
		return m.submitProviderForm()
	case "enter":
		if f.focus == len(f.fields())-1 {
			return m.submitProviderForm()
		}
		f.move(1)
		return m, nil
	case "tab", "down":
		f.move(1)
		return m, nil
	case "shift+tab", "up":
		f.move(-1)
		return m, nil
	}

	switch key {
	case fieldKeyType:
		switch msg.String() {
		case "left", "h":
			f.typeIdx = (f.typeIdx - 1 + len(service.ProviderTypes)) % len(service.ProviderTypes)
		case "right", "l", " ":
			f.typeIdx = (f.typeIdx + 1) % len(service.ProviderTypes)
		default:
			return m, nil
		}
		f.applyDefaults()
		f.err = ""
		return m, nil
	case fieldKeyAttend:
		if s := msg.String(); s == " " || s == "left" || s == "right" || s == "h" || s == "l" {
			f.attend = !f.attend
		}
		return m, nil
	}

	if in, ok := f.inputs[key]; ok {
		var cmd tea.Cmd
		*in, cmd = in.Update(msg)
		f.err = ""
		return m, cmd
	}
	return m, nil
}

// submitProviderForm saves the provider and switches to it, which starts
// its login (for TrackIt) and loads its data. When editing, it saves the
// changes, and reloads only if it's the active provider.
func (m Model) submitProviderForm() (tea.Model, tea.Cmd) {
	f := m.providerForm
	if f.editing != "" {
		active, err := m.session.UpdateProvider(f.draft())
		if err != nil {
			f.err = err.Error()
			return m, nil
		}
		m.providerForm = nil
		msg := fmt.Sprintf("Provider %q saved.", f.editing)
		if !active {
			m.screen = screenProviderPicker
			return m, m.setStatus(msg)
		}
		next, cmd := m.reloadProvider()
		nm := next.(Model)
		return nm, tea.Batch(cmd, nm.setStatus(msg))
	}
	name, err := m.session.AddProvider(f.draft())
	if err != nil {
		f.err = err.Error()
		return m, nil
	}
	m.providerForm = nil
	next, cmd := m.switchProvider(name)
	nm := next.(Model)
	return nm, tea.Batch(cmd, nm.setStatus(fmt.Sprintf("Provider %q added to your config and selected.", name)))
}

func (m Model) viewProviderForm() string {
	f := m.providerForm
	if f == nil {
		return ""
	}
	var b strings.Builder
	title := " Add a provider "
	if f.editing != "" {
		title = fmt.Sprintf(" Edit provider %q ", f.editing)
	}
	b.WriteString(formTitleStyle.Render(title) + "\n\n")
	for i, fld := range f.fields() {
		focused := i == f.focus
		var value string
		switch fld.key {
		case fieldKeyType:
			value = fmt.Sprintf("◀ %s ▶", f.providerType().Label)
		case fieldKeyAttend:
			value = "[ ] off"
			if f.attend {
				value = okStyle.Render("[x] on")
			}
			value += subtleStyle.Render("  (TrackIt 4.0.0+)")
		default:
			value = f.inputs[fld.key].View()
		}
		b.WriteString(renderField(fld.label, value, focused))
	}

	if f.providerType().ID == "trackit" && f.editing == "" {
		b.WriteString("\n" + subtleStyle.Render("You'll sign in to it in your browser right after saving.") + "\n")
	}
	if f.err != "" {
		b.WriteString("\n" + errorStyle.Render(f.err) + "\n")
	}

	help := "[tab/↑↓] field  [ctrl+s] save  [esc] cancel"
	if f.editing != "" {
		help = "[tab/↑↓] field  [ctrl+s] save changes  [esc] cancel"
	}
	switch f.focused() {
	case fieldKeyType:
		help = "[←/→] type  " + help
	case fieldKeyAttend:
		help = "[space] toggle  " + help
	}
	b.WriteString("\n" + helpStyle.Render(help))
	return "\n" + boxStyle.Render(b.String()) + "\n"
}

// confirmRemoveProvider asks before removing provider name.
func (m Model) confirmRemoveProvider(name string) (tea.Model, tea.Cmd) {
	if len(m.session.ProviderNames()) < 2 {
		return m, m.setStatus("You can't remove your only provider; add another one first.")
	}
	m.removeTarget = name
	m.screen = screenConfirmRemoveProvider
	return m, nil
}

func (m Model) handleConfirmRemoveProviderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		name := m.removeTarget
		m.removeTarget = ""
		next, err := m.session.RemoveProvider(name)
		if err != nil {
			m.screen = screenProviderPicker
			return m, m.setStatus("Couldn't remove it: " + err.Error())
		}
		msg := fmt.Sprintf("Provider %q removed.", name)
		if next == "" {
			m.screen = screenProviderPicker
			m.providerCursor = min(m.providerCursor, len(m.session.ProviderNames())-1)
			return m, m.setStatus(msg)
		}
		nm, cmd := m.reloadProvider()
		model := nm.(Model)
		return model, tea.Batch(cmd, model.setStatus(fmt.Sprintf("%s Switched to %q.", msg, next)))
	case "n", "esc":
		m.removeTarget = ""
		m.screen = screenProviderPicker
	}
	return m, nil
}

func (m Model) viewConfirmRemoveProvider() string {
	body := errorStyle.Render(fmt.Sprintf("Remove provider %q?", m.removeTarget)) + "\n\n" +
		"It's deleted from your config file, and its saved login is removed.\n"
	if m.removeTarget == m.session.ActiveProviderName() {
		body += subtleStyle.Render("It's the active provider, so LazyTrackIt switches to another one.") + "\n"
	}
	body += "\n" + helpStyle.Render("[y] remove   [n] cancel")
	return "\n" + boxStyle.Render(body) + "\n"
}

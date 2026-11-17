package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrProviderNotFound is returned when editing a provider that isn't in the
// config file.
var ErrProviderNotFound = errors.New("no provider with that name in the config file")

// UpdateProvider replaces the body of provider name in config.yaml with
// value. Only that entry's lines change: comments above it and everything
// else in the file stay as they were (comments inside the entry itself are
// dropped, since they described the old values).
func UpdateProvider(name string, value any) error {
	return editConfigFile(func(doc []byte) ([]byte, error) { return updateProviderYAML(doc, name, value) })
}

// RemoveProvider deletes provider name from config.yaml. If active_provider
// names it, it's changed to fallback, so the config stays valid.
func RemoveProvider(name, fallback string) error {
	return editConfigFile(func(doc []byte) ([]byte, error) { return removeProviderYAML(doc, name, fallback) })
}

// entrySpan is the lines (0-based, end exclusive) of one provider entry.
type entrySpan struct {
	start, end int
	indent     string
}

// parsed is a config document as both text lines and YAML nodes.
type parsed struct {
	cfg    Config
	lines  []string
	top    *yaml.Node // the root mapping
	provK  *yaml.Node // the "providers" key
	provs  *yaml.Node // the providers mapping
	active *yaml.Node // the active_provider value, if any
}

func parseForEdit(doc []byte) (*parsed, error) {
	p := &parsed{lines: strings.Split(string(doc), "\n")}
	if err := yaml.Unmarshal(doc, &p.cfg); err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(doc, &root); err != nil {
		return nil, err
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("the config isn't a YAML mapping")
	}
	p.top = root.Content[0]
	for i := 0; i+1 < len(p.top.Content); i += 2 {
		switch p.top.Content[i].Value {
		case "providers":
			p.provK, p.provs = p.top.Content[i], p.top.Content[i+1]
		case "active_provider":
			p.active = p.top.Content[i+1]
		}
	}
	return p, nil
}

// locate finds the lines of provider name's entry: from its key line to the
// last line indented deeper than the key (content, or comments annotating
// it), not counting blank lines after it.
func (p *parsed) locate(name string) (entrySpan, error) {
	if p.provs == nil || p.provs.Kind != yaml.MappingNode {
		return entrySpan{}, ErrProviderNotFound
	}
	if p.provs.Style&yaml.FlowStyle != 0 {
		return entrySpan{}, errors.New(`"providers" is written in a form that can't be edited automatically (e.g. {...}); edit it by hand`)
	}
	for i := 0; i+1 < len(p.provs.Content); i += 2 {
		key := p.provs.Content[i]
		if key.Value != name {
			continue
		}
		indent := key.Column - 1
		span := entrySpan{start: key.Line - 1, end: key.Line, indent: strings.Repeat(" ", indent)}
		for j := key.Line; j < len(p.lines); j++ {
			l := p.lines[j]
			t := strings.TrimSpace(l)
			if t == "" {
				continue
			}
			if len(l)-len(strings.TrimLeft(l, " \t")) <= indent {
				break // a sibling, its comment, or the next top-level key
			}
			span.end = j + 1
		}
		return span, nil
	}
	return entrySpan{}, ErrProviderNotFound
}

func updateProviderYAML(doc []byte, name string, value any) ([]byte, error) {
	p, err := parseForEdit(doc)
	if err != nil {
		return nil, err
	}
	span, err := p.locate(name)
	if err != nil {
		return nil, err
	}
	body, err := encodeBlock(value)
	if err != nil {
		return nil, err
	}
	out := append([]string{}, p.lines[:span.start]...)
	out = append(out, entryLines(span.indent, name, body)...)
	out = append(out, p.lines[span.end:]...)
	result := []byte(strings.Join(out, "\n"))

	// Everything but this provider must be unchanged.
	var got Config
	if err := yaml.Unmarshal(result, &got); err != nil {
		return nil, fmt.Errorf("the edited config doesn't parse (%v); edit it by hand", err)
	}
	if _, ok := got.Providers[name]; !ok {
		return nil, errors.New("couldn't update the provider in the config; edit it by hand")
	}
	before := p.cfg
	before.Providers = without(before.Providers, name)
	got.Providers = without(got.Providers, name)
	if !reflect.DeepEqual(plain(before), plain(got)) {
		return nil, errors.New("updating the provider would change other settings; edit it by hand")
	}
	return result, nil
}

func removeProviderYAML(doc []byte, name, fallback string) ([]byte, error) {
	p, err := parseForEdit(doc)
	if err != nil {
		return nil, err
	}
	span, err := p.locate(name)
	if err != nil {
		return nil, err
	}
	lines := p.lines
	// Point active_provider elsewhere first (it's above or below the entry;
	// either way its line index is unaffected by an edit after it, so do the
	// later edit first).
	activeLine := -1
	if p.active != nil && p.active.Value == name {
		if fallback == "" || fallback == name {
			return nil, errors.New("active_provider names this provider and there's no other to point it at")
		}
		activeLine = p.active.Line - 1
	}
	setActive := func(ls []string, i int) []string {
		key, _ := yaml.Marshal(fallback)
		line := ls[i][:p.active.Column-1] + strings.TrimSpace(string(key))
		if p.active.LineComment != "" {
			line += " " + p.active.LineComment
		}
		ls[i] = line
		return ls
	}

	out := append([]string{}, lines...)
	if activeLine >= span.end {
		out = setActive(out, activeLine)
	}
	// The entry's own comment lines directly above it go with it.
	start, end := span.start, span.end
	for start > 0 {
		l := out[start-1]
		if !strings.HasPrefix(strings.TrimSpace(l), "#") || !strings.HasPrefix(l, span.indent) ||
			strings.HasPrefix(l, span.indent+" ") {
			break
		}
		start--
	}
	// Don't leave two blank lines back to back, or one right under
	// "providers:".
	blank := func(i int) bool { return i >= 0 && i < len(out) && strings.TrimSpace(out[i]) == "" }
	if blank(end) && (start == 0 || blank(start-1) || (p.provK != nil && start-1 == p.provK.Line-1)) {
		end++
	}
	if activeLine >= start && activeLine < end {
		return nil, errors.New("couldn't remove the provider cleanly; edit the config by hand")
	}
	out = append(out[:start:start], out[end:]...)
	if activeLine >= 0 && activeLine < start {
		out = setActive(out, activeLine)
	}
	result := []byte(strings.Join(out, "\n"))

	var got Config
	if err := yaml.Unmarshal(result, &got); err != nil {
		return nil, fmt.Errorf("the edited config doesn't parse (%v); edit it by hand", err)
	}
	want := p.cfg
	want.Providers = without(want.Providers, name)
	if activeLine >= 0 {
		want.ActiveProvider = fallback
	}
	if _, ok := got.Providers[name]; ok || !reflect.DeepEqual(plain(want), plain(got)) {
		return nil, errors.New("removing the provider would change other settings; edit it by hand")
	}
	return result, nil
}

// without returns m minus key (nil if that leaves it empty).
func without(m map[string]ProviderConfig, key string) map[string]ProviderConfig {
	out := map[string]ProviderConfig{}
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

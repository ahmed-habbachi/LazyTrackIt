package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrProviderExists is returned by AddProvider when the name is taken.
var ErrProviderExists = errors.New("a provider with that name already exists")

// AddProvider appends a provider entry named name to config.yaml, with value
// (see service.ProviderDraft) marshalled as its body. The entry is inserted
// as text after the last existing provider, so every other line of the file
// (comments, blank lines, formatting) stays exactly as it was, and the file
// is replaced atomically with the same permissions.
func AddProvider(name string, value any) error {
	return editConfigFile(func(doc []byte) ([]byte, error) { return addProviderYAML(doc, name, value) })
}

// editConfigFile applies edit to config.yaml and writes the result back
// atomically.
func editConfigFile(edit func([]byte) ([]byte, error)) error {
	path, err := Path()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := edit(data)
	if err != nil {
		return fmt.Errorf("updating %s: %w", path, err)
	}
	return writeFileAtomic(path, out)
}

// addProviderYAML returns doc with the provider added under "providers".
func addProviderYAML(doc []byte, name string, value any) ([]byte, error) {
	var before Config
	if err := yaml.Unmarshal(doc, &before); err != nil {
		return nil, err
	}
	if _, ok := before.Providers[name]; ok {
		return nil, ErrProviderExists
	}

	var root yaml.Node
	if err := yaml.Unmarshal(doc, &root); err != nil {
		return nil, err
	}
	var top *yaml.Node
	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		top = root.Content[0]
		if top.Kind != yaml.MappingNode {
			return nil, errors.New("the config isn't a YAML mapping")
		}
	}

	body, err := encodeBlock(value)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(doc), "\n")
	var out []string
	var keyNode, provNode *yaml.Node
	if top != nil {
		for i := 0; i+1 < len(top.Content); i += 2 {
			if top.Content[i].Value == "providers" {
				keyNode, provNode = top.Content[i], top.Content[i+1]
			}
		}
	}

	switch {
	case provNode == nil:
		// No providers section: add one at the end of the file.
		out = append(trimTrailingBlank(lines), "providers:")
		out = append(out, entryLines("  ", name, body)...)
		out = append(out, "")

	case provNode.Kind == yaml.MappingNode && provNode.Style&yaml.FlowStyle == 0 && len(provNode.Content) > 0:
		indent := strings.Repeat(" ", provNode.Content[0].Column-1)
		at := sectionEnd(lines, keyNode.Line, len(indent))
		out = append(out, lines[:at]...)
		out = append(out, entryLines(indent, name, body)...)
		out = append(out, lines[at:]...)

	case provNode.Kind == yaml.ScalarNode && provNode.Tag == "!!null" && provNode.Line == keyNode.Line:
		// "providers:" with nothing under it.
		at := keyNode.Line // insert right after the key's line
		out = append(out, lines[:at]...)
		out = append(out, entryLines("  ", name, body)...)
		out = append(out, lines[at:]...)

	default:
		return nil, errors.New(`"providers" is written in a form that can't be edited automatically (e.g. {...}); add the provider by hand`)
	}

	result := []byte(strings.Join(out, "\n"))
	if err := checkOnlyAdded(before, result, name); err != nil {
		return nil, err
	}
	return result, nil
}

// sectionEnd returns the index of the line after the last line belonging
// to the block that starts at keyLine (1-based): content lines, plus
// comments indented deeper than the entries (entryIndent), which annotate
// the entry above them. It stops before comments at the entries' own level
// (e.g. commented-out example providers) and before the next top-level key.
func sectionEnd(lines []string, keyLine, entryIndent int) int {
	end := keyLine // index just past the key line
	for i := keyLine; i < len(lines); i++ {
		l := lines[i]
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		indent := len(l) - len(strings.TrimLeft(l, " \t"))
		if strings.HasPrefix(t, "#") {
			if indent > entryIndent {
				end = i + 1
			}
			continue
		}
		if indent == 0 {
			break // next top-level key
		}
		end = i + 1
	}
	return end
}

// encodeBlock marshals value as a YAML block with 2-space indentation.
func encodeBlock(value any) ([]string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimRight(buf.String(), "\n"), "\n"), nil
}

// entryLines renders "name:" at indent with body nested two spaces deeper.
func entryLines(indent, name string, body []string) []string {
	key, err := yaml.Marshal(name)
	if err != nil {
		key = []byte(name)
	}
	out := []string{indent + strings.TrimSpace(string(key)) + ":"}
	for _, l := range body {
		out = append(out, indent+"  "+l)
	}
	return out
}

func trimTrailingBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// checkOnlyAdded parses the edited config and makes sure the only change is
// the new provider, so a surprising file layout can never silently corrupt
// the rest of the config.
func checkOnlyAdded(before Config, after []byte, name string) error {
	var got Config
	if err := yaml.Unmarshal(after, &got); err != nil {
		return fmt.Errorf("the edited config doesn't parse (%v); add the provider by hand", err)
	}
	if _, ok := got.Providers[name]; !ok {
		return errors.New("couldn't place the new provider in the config; add it by hand")
	}
	got.Providers = without(got.Providers, name)
	before.Providers = without(before.Providers, "")
	if !reflect.DeepEqual(plain(before), plain(got)) {
		return errors.New("adding the provider would change other settings; add it by hand")
	}
	return nil
}

// plain re-encodes c so yaml.Node fields (which record line numbers)
// compare by content only.
func plain(c Config) any {
	b, _ := yaml.Marshal(c)
	var v any
	_ = yaml.Unmarshal(b, &v)
	return v
}

// writeFileAtomic replaces path with data via a temporary file in the same
// directory, keeping the original file's permissions (0600 if it's new,
// since the config can hold secrets).
func writeFileAtomic(path string, data []byte) error {
	mode := os.FileMode(0o600)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

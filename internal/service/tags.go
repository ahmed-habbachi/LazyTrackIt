package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// ErrUnknownTag is returned (wrapped) by ResolveTagIDs for a token that
// matches no available tag by ID or name.
var ErrUnknownTag = errors.New("unknown tag")

// JoinList renders values as the comma-separated form shown in text inputs.
func JoinList(values []string) string {
	return strings.Join(values, ",")
}

// ParseList splits a comma-separated input into its trimmed, non-empty parts.
func ParseList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ResolveTagIDs maps each entered token to a tag ID, accepting either a tag
// name (case-insensitive) or a raw ID. If available is empty (tags haven't
// loaded, or the provider has none), tokens are passed through unchanged so
// typing a raw ID still works.
func ResolveTagIDs(tokens []string, available []provider.Tag) ([]string, error) {
	if len(available) == 0 {
		return tokens, nil
	}
	byID := make(map[string]provider.Tag, len(available))
	byName := make(map[string]provider.Tag, len(available))
	for _, t := range available {
		byID[t.ID] = t
		byName[strings.ToLower(t.Name)] = t
	}
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if t, ok := byID[tok]; ok {
			out = append(out, t.ID)
			continue
		}
		if t, ok := byName[strings.ToLower(tok)]; ok {
			out = append(out, t.ID)
			continue
		}
		return nil, fmt.Errorf("%w %q", ErrUnknownTag, tok)
	}
	return out, nil
}

// TagIDsToNames returns a copy of tokens with every token that's exactly one
// of available's IDs replaced by that tag's name, and whether anything
// changed. This is what turns stored tag IDs (what gets sent to the API)
// back into readable names. Only exact ID matches are touched, so a name
// the user already typed, or an ID that doesn't resolve, is left alone.
func TagIDsToNames(tokens []string, available []provider.Tag) ([]string, bool) {
	out := append([]string(nil), tokens...)
	if len(available) == 0 {
		return out, false
	}
	nameByID := make(map[string]string, len(available))
	for _, t := range available {
		nameByID[t.ID] = t.Name
	}
	changed := false
	for i, tok := range out {
		if name, ok := nameByID[tok]; ok {
			out[i] = name
			changed = true
		}
	}
	return out, changed
}

// SplitTagNames groups available's names into project-specific and system
// tags, since they come from two different sources and a name collision
// would otherwise be ambiguous.
func SplitTagNames(available []provider.Tag) (project, system []string) {
	for _, t := range available {
		if t.IsSystem {
			system = append(system, t.Name)
		} else {
			project = append(project, t.Name)
		}
	}
	return project, system
}

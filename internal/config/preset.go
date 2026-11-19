package config

import (
	"fmt"
	"strconv"
	"strings"
)

// The starter config's TrackIt provider can be preset at build time, so an
// organization's own builds start with a working config instead of
// placeholders. The release workflow sets these from repository variables
// (they're deliberately not in the source), e.g.:
//
//	-ldflags "-X github.com/ahmed-habbachi/lazytrackit/internal/config.PresetBaseURL=https://trackit.example.com
//	          -X github.com/ahmed-habbachi/lazytrackit/internal/config.PresetIssuer=https://auth.example.com/realms/example
//	          -X github.com/ahmed-habbachi/lazytrackit/internal/config.PresetClientID=lazytrackit
//	          -X github.com/ahmed-habbachi/lazytrackit/internal/config.PresetAttendance=true"
//
// The preset only applies when both PresetBaseURL and PresetIssuer are set;
// PresetClientID defaults to the starter's "lazytrackit". Nothing secret
// goes here: the TrackIt login is the device flow with a public client.
var (
	PresetBaseURL    string
	PresetIssuer     string
	PresetClientID   string
	PresetAttendance string // "true" turns check-in/check-out on
)

// Preset reports whether this build presets the starter config's TrackIt
// provider (see PresetBaseURL), so a fresh install works without editing.
func Preset() bool {
	return strings.TrimSpace(PresetBaseURL) != "" && strings.TrimSpace(PresetIssuer) != ""
}

// starterConfig is the config WriteExample writes: exampleConfig, with the
// TrackIt provider filled in from the build-time preset if there is one.
func starterConfig() string {
	if !Preset() {
		return exampleConfig
	}
	return presetConfig(exampleConfig, strings.TrimSpace(PresetBaseURL), strings.TrimSpace(PresetIssuer),
		strings.TrimSpace(PresetClientID), strings.EqualFold(strings.TrimSpace(PresetAttendance), "true"))
}

// presetConfig replaces the placeholder lines of the active trackit
// provider in doc (the first occurrence of each, which is the uncommented
// one; the commented examples further down are left alone).
func presetConfig(doc, baseURL, issuer, clientID string, attendance bool) string {
	replace := func(old, new string) { doc = strings.Replace(doc, old, new, 1) }
	replace("    base_url: https://trackit.yourcompany.com\n", fmt.Sprintf("    base_url: %s\n", yamlString(baseURL)))
	replace("      issuer: https://auth.yourcompany.com/realms/yourrealm\n", fmt.Sprintf("      issuer: %s\n", yamlString(issuer)))
	if clientID != "" {
		replace("      client_id: lazytrackit\n", fmt.Sprintf("      client_id: %s\n", yamlString(clientID)))
	}
	if attendance {
		replace("    # attendance: true\n", "    attendance: true\n")
	}
	return doc
}

// yamlString quotes s as a YAML double-quoted scalar, so any value is
// safe to splice into the file.
func yamlString(s string) string {
	return strconv.Quote(s)
}

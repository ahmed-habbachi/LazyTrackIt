package toggl

import (
	"encoding/json"
	"fmt"
	"strings"
)

// APIError represents a non-2xx HTTP response from the Toggl Track API.
// Toggl's error bodies are inconsistent: sometimes a JSON {"error": "..."}
// object, sometimes a bare quoted string, sometimes plain text. This tries
// each in turn, falling back to the raw body.
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Status     string
	Body       string
}

func (e *APIError) Error() string {
	msg := e.extractMessage()
	if msg == "" {
		msg = e.Body
	}
	if msg == "" {
		return fmt.Sprintf("%s %s: %s", e.Method, e.Path, e.Status)
	}
	return fmt.Sprintf("%s %s: %s: %s", e.Method, e.Path, e.Status, msg)
}

func (e *APIError) extractMessage() string {
	var fields struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(e.Body), &fields); err == nil && strings.TrimSpace(fields.Error) != "" {
		return fields.Error
	}
	var plain string
	if err := json.Unmarshal([]byte(e.Body), &plain); err == nil && strings.TrimSpace(plain) != "" {
		return plain
	}
	return ""
}

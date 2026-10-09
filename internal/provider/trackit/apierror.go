package trackit

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// APIError represents a non-2xx HTTP response from the TrackIt API. It tries
// to pull a human-readable message out of whatever error shape the server
// used (ASP.NET's default {"Message":...}, ProblemDetails' {"title":...,
// "detail":...}, or an OAuth-style {"error_description":...}), falling back
// to the raw response body.
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
		Message          string              `json:"Message"`
		MessageLower     string              `json:"message"`
		Title            string              `json:"title"`
		Detail           string              `json:"detail"`
		Error            string              `json:"error"`
		ErrorDescription string              `json:"error_description"`
		Errors           map[string][]string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(e.Body), &fields); err != nil {
		return ""
	}

	// ASP.NET's ValidationProblemDetails puts the generic summary in Title
	// ("One or more validation errors occurred.") and the actually useful,
	// per-field reasons in Errors, so prefer those when present instead of
	// surfacing just the uninformative summary.
	if len(fields.Errors) > 0 {
		keys := make([]string, 0, len(fields.Errors))
		for k := range fields.Errors {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+": "+strings.Join(fields.Errors[k], "; "))
		}
		return strings.Join(parts, " | ")
	}

	for _, candidate := range []string{fields.Message, fields.MessageLower, fields.Detail, fields.Title, fields.ErrorDescription, fields.Error} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return ""
}

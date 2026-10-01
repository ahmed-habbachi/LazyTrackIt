package trackit

import (
	"encoding/json"
	"fmt"
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
		Message          string `json:"Message"`
		MessageLower     string `json:"message"`
		Title            string `json:"title"`
		Detail           string `json:"detail"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal([]byte(e.Body), &fields); err != nil {
		return ""
	}
	for _, candidate := range []string{fields.Message, fields.MessageLower, fields.Detail, fields.Title, fields.ErrorDescription, fields.Error} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return ""
}

package weekbooking

import (
	"encoding/json"
	"fmt"
	"strings"
)

// APIError represents a non-2xx HTTP response from the week-booking API,
// which uses RFC 7807 Problem Details plus a machine-readable `code` field
// (e.g. "week_closed", "duplicate_entry", "reopen_too_old").
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Status     string
	Body       string

	// Code is the ProblemDetails `code` field, if the body parsed as one.
	Code string
}

func (e *APIError) Error() string {
	var fields struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Code   string `json:"code"`
	}
	if err := json.Unmarshal([]byte(e.Body), &fields); err == nil {
		e.Code = fields.Code
		msg := strings.TrimSpace(fields.Detail)
		if msg == "" {
			msg = strings.TrimSpace(fields.Title)
		}
		switch {
		case msg != "" && fields.Code != "":
			return fmt.Sprintf("%s %s: %s: %s (%s)", e.Method, e.Path, e.Status, msg, fields.Code)
		case fields.Code != "":
			return fmt.Sprintf("%s %s: %s: %s", e.Method, e.Path, e.Status, fields.Code)
		case msg != "":
			return fmt.Sprintf("%s %s: %s: %s", e.Method, e.Path, e.Status, msg)
		}
	}
	if e.Body != "" {
		return fmt.Sprintf("%s %s: %s: %s", e.Method, e.Path, e.Status, e.Body)
	}
	return fmt.Sprintf("%s %s: %s", e.Method, e.Path, e.Status)
}

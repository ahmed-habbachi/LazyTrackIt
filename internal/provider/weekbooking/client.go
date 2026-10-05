// Package weekbooking implements provider.Provider against the customer's
// Zeiterfassung ("Week Booking") REST API, authenticating with a static
// per-user API token rather than an OAuth flow. The API also supports a
// browser session-cookie flow (OIDC login + CSRF header), but the bearer
// token is what the spec says is "intended for CLI scripts", so that's what
// this package uses — same shape as the Toggl provider.
package weekbooking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// AuthConfig is weekbooking's shape for config.ProviderConfig.Auth: a single
// long-lived API token, minted via POST /api/v1/me/api-tokens (e.g. from an
// "API tokens" page in the web app — that endpoint isn't itself part of the
// week-booking spec this provider was built from).
type AuthConfig struct {
	APIToken string `yaml:"api_token"`
}

// Client implements provider.Provider for the Zeiterfassung week-booking API.
type Client struct {
	name    string
	baseURL string
	token   string
	http    *http.Client

	mu sync.Mutex
	me *provider.Member
}

// New builds a Client for the given provider name and configuration. It does
// not perform any network I/O until a method is called.
func New(name string, cfg config.ProviderConfig) (*Client, error) {
	var authCfg AuthConfig
	if err := cfg.Auth.Decode(&authCfg); err != nil {
		return nil, fmt.Errorf("decoding auth config for provider %q: %w", name, err)
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("provider %q: base_url is required", name)
	}
	return &Client{
		name:    name,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		token:   authCfg.APIToken,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Client) Name() string { return c.name }

// IsAuthenticated reports whether an API token is configured. Like Toggl's
// token, this never expires and isn't cached separately: the token in
// config.yaml *is* the credential.
func (c *Client) IsAuthenticated() bool { return c.token != "" }

// Login has nothing to negotiate: the API token lives in config.yaml
// already, so this just confirms it actually works by resolving the current
// user. onPrompt is never called.
func (c *Client) Login(ctx context.Context, onPrompt func(provider.LoginPrompt)) error {
	if c.token == "" {
		return fmt.Errorf("provider %q: no api_token configured (mint one via POST /api/v1/me/api-tokens)", c.name)
	}
	_, err := c.Me(ctx)
	return err
}

// currentUserID returns the authenticated user's backend user_id, calling Me
// to resolve it if it hasn't been fetched yet this session.
func (c *Client) currentUserID(ctx context.Context) (string, error) {
	c.mu.Lock()
	cached := c.me
	c.mu.Unlock()
	if cached != nil {
		return cached.UserID, nil
	}
	m, err := c.Me(ctx)
	if err != nil {
		return "", err
	}
	return m.UserID, nil
}

// doJSON performs an HTTP request with a JSON body (if any) and decodes a
// JSON response (if out is non-nil).
func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	if c.token == "" {
		return fmt.Errorf("not logged in: run login first")
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
		return &APIError{
			Method:     method,
			Path:       path,
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(data)),
		}
	}
	if out == nil {
		return nil
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s %s: decoding response: %w", method, path, err)
	}
	return nil
}

// idToString/idFromString convert between the domain model's string project
// IDs and the backend's wire-level integer project IDs.
func idToString(id int64) string { return strconv.FormatInt(id, 10) }

func idFromString(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q: %w", s, err)
	}
	return n, nil
}

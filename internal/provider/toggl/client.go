// Package toggl implements provider.Provider against the Toggl Track API
// (https://engineering.toggl.com/docs/), authenticating with a static
// per-user API token rather than an OAuth flow.
package toggl

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

// defaultBaseURL is Toggl Track's hosted API; self-hosting isn't offered, but
// BaseURL stays configurable (e.g. to point at a proxy) like every provider.
const defaultBaseURL = "https://api.track.toggl.com"

// AuthConfig is Toggl's shape for config.ProviderConfig.Auth: a single
// long-lived API token, found under Profile settings in the Toggl web app.
type AuthConfig struct {
	APIToken string `yaml:"api_token"`
}

// Client implements provider.Provider for the Toggl Track API.
type Client struct {
	name    string
	baseURL string
	token   string
	http    *http.Client

	mu          sync.Mutex
	me          *provider.Member
	workspaceID int64
}

// New builds a Client for the given provider name and configuration. It does
// not perform any network I/O until a method is called.
func New(name string, cfg config.ProviderConfig) (*Client, error) {
	var authCfg AuthConfig
	if err := cfg.Auth.Decode(&authCfg); err != nil {
		return nil, fmt.Errorf("decoding auth config for provider %q: %w", name, err)
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		name:    name,
		baseURL: baseURL,
		token:   authCfg.APIToken,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Client) Name() string { return c.name }

// IsAuthenticated reports whether an API token is configured. Unlike
// TrackIt's OAuth tokens, this never expires and isn't cached separately: the
// token in config.yaml *is* the credential.
func (c *Client) IsAuthenticated() bool { return c.token != "" }

// Login has nothing to negotiate (there's no device flow, no refresh token to
// mint): the API token lives in config.yaml already, so this just confirms it
// actually works by resolving the current user. onPrompt is never called.
func (c *Client) Login(ctx context.Context, onPrompt func(provider.LoginPrompt)) error {
	if c.token == "" {
		return fmt.Errorf("provider %q: no api_token configured (see Profile settings in Toggl Track)", c.name)
	}
	_, err := c.Me(ctx)
	return err
}

// currentWorkspaceID returns the user's default workspace, calling Me to
// resolve it if it hasn't been fetched yet this session. Every Toggl
// time-entry/tag endpoint is scoped to a workspace; v1 only ever operates
// within the caller's default one.
func (c *Client) currentWorkspaceID(ctx context.Context) (int64, error) {
	c.mu.Lock()
	cached := c.workspaceID
	c.mu.Unlock()
	if cached != 0 {
		return cached, nil
	}
	if _, err := c.Me(ctx); err != nil {
		return 0, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.workspaceID, nil
}

// doJSON performs an HTTP request with a JSON body (if any) and decodes a
// JSON response (if out is non-nil). Toggl authenticates via HTTP Basic auth
// with the API token as the username and the literal string "api_token" as
// the password.
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
	req.SetBasicAuth(c.token, "api_token")
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

// idToString/idFromString convert between the domain model's string IDs and
// Toggl's wire-level int64 IDs.
func idToString(id int64) string { return strconv.FormatInt(id, 10) }

func idFromString(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q: %w", s, err)
	}
	return n, nil
}

func idsFromStrings(ss []string) ([]int64, error) {
	out := make([]int64, 0, len(ss))
	for _, s := range ss {
		id, err := idFromString(s)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

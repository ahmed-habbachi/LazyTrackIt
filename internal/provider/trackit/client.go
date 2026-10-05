// Package trackit implements provider.Provider against the Case.TrackIt
// REST API, authenticating via an OIDC device-code flow.
package trackit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/auth"
	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// AuthConfig describes how to obtain a JWT from an OIDC-compliant issuer
// (e.g. a Keycloak realm) using the device authorization grant. It is
// TrackIt's own shape for config.ProviderConfig.Auth; a different provider
// would define its own (e.g. a bare API key).
type AuthConfig struct {
	Issuer       string   `yaml:"issuer"` // e.g. https://auth.yourcompany.com/realms/yourrealm
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret,omitempty"`
	Scopes       []string `yaml:"scopes,omitempty"`
}

// Client implements provider.Provider for the Case.TrackIt API.
type Client struct {
	name    string
	baseURL string
	cfg     AuthConfig
	http    *http.Client

	mu     sync.Mutex
	auth   *auth.Client
	tokens *auth.Tokens
	me     *provider.Member
}

// New builds a Client for the given provider name and configuration. It does
// not perform any network I/O until a method is called.
func New(name string, cfg config.ProviderConfig) (*Client, error) {
	var authCfg AuthConfig
	if err := cfg.Auth.Decode(&authCfg); err != nil {
		return nil, fmt.Errorf("decoding auth config for provider %q: %w", name, err)
	}
	return &Client{
		name:    name,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		cfg:     authCfg,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Client) Name() string { return c.name }

func (c *Client) IsAuthenticated() bool {
	if c.tokens != nil {
		return true
	}
	t, err := auth.LoadTokens(c.name)
	return err == nil && t != nil
}

func (c *Client) ensureAuthClient(ctx context.Context) error {
	if c.auth != nil {
		return nil
	}
	disco, err := auth.Discover(ctx, c.cfg.Issuer)
	if err != nil {
		return err
	}
	c.auth = auth.NewClient(disco, c.cfg.ClientID, c.cfg.ClientSecret, c.cfg.Scopes)
	return nil
}

// Login runs the device-code flow end to end, persisting the resulting tokens.
func (c *Client) Login(ctx context.Context, onPrompt func(provider.LoginPrompt)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureAuthClient(ctx); err != nil {
		return err
	}
	dcr, err := c.auth.StartDeviceAuth(ctx)
	if err != nil {
		return err
	}
	url := dcr.VerificationURI
	if dcr.VerificationURIComplete != "" {
		url = dcr.VerificationURIComplete
	}
	onPrompt(provider.LoginPrompt{
		Message: "Open this URL in your browser (attempting to open it for you) and enter the code below if it isn't pre-filled.",
		URL:     url,
		Code:    dcr.UserCode,
		Expires: time.Duration(dcr.ExpiresIn) * time.Second,
	})
	toks, err := c.auth.PollDeviceToken(ctx, dcr)
	if err != nil {
		return err
	}
	c.tokens = toks
	return auth.SaveTokens(c.name, toks)
}

// accessToken returns a valid access token, refreshing or triggering login as needed.
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tokens == nil {
		t, err := auth.LoadTokens(c.name)
		if err != nil {
			return "", err
		}
		c.tokens = t
	}
	if c.tokens == nil {
		return "", fmt.Errorf("not logged in: run login first")
	}
	if !c.tokens.Expired() {
		return c.tokens.AccessToken, nil
	}
	if c.tokens.RefreshToken == "" {
		return "", fmt.Errorf("access token expired and no refresh token is available: log in again")
	}
	if err := c.ensureAuthClient(ctx); err != nil {
		return "", err
	}
	fresh, err := c.auth.RefreshTokens(ctx, c.tokens.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("refreshing access token: %w", err)
	}
	c.tokens = fresh
	if err := auth.SaveTokens(c.name, fresh); err != nil {
		return "", err
	}
	return c.tokens.AccessToken, nil
}

// currentUserID returns the authenticated user's backend UserID, calling Me
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
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
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
	req.Header.Set("Authorization", "Bearer "+token)
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

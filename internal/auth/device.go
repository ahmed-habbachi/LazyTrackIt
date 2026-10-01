// Package auth implements the OAuth 2.0 Device Authorization Grant
// (RFC 8628) against an OIDC-compliant issuer, plus on-disk token caching.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Discovery holds the subset of an OIDC discovery document we need.
type Discovery struct {
	TokenEndpoint               string `json:"token_endpoint"`
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
}

// Discover fetches {issuer}/.well-known/openid-configuration.
func Discover(ctx context.Context, issuer string) (*Discovery, error) {
	u := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching discovery document: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery document %s returned %s", u, resp.Status)
	}
	var d Discovery
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, fmt.Errorf("decoding discovery document: %w", err)
	}
	if d.TokenEndpoint == "" || d.DeviceAuthorizationEndpoint == "" {
		return nil, fmt.Errorf("discovery document is missing token_endpoint or device_authorization_endpoint")
	}
	return &d, nil
}

// DeviceCodeResponse is RFC 8628 section 3.2's response.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// Tokens is what we persist and hand back to callers.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (t Tokens) Expired() bool {
	return !t.ExpiresAt.IsZero() && time.Now().After(t.ExpiresAt.Add(-30*time.Second))
}

// Client drives the device flow and token refresh for one OIDC client.
type Client struct {
	Discovery    *Discovery
	ClientID     string
	ClientSecret string
	Scopes       []string
	HTTPClient   *http.Client
}

func NewClient(d *Discovery, clientID, clientSecret string, scopes []string) *Client {
	return &Client{Discovery: d, ClientID: clientID, ClientSecret: clientSecret, Scopes: scopes, HTTPClient: http.DefaultClient}
}

func (c *Client) form() url.Values {
	v := url.Values{"client_id": {c.ClientID}}
	if c.ClientSecret != "" {
		v.Set("client_secret", c.ClientSecret)
	}
	if len(c.Scopes) > 0 {
		v.Set("scope", strings.Join(c.Scopes, " "))
	}
	return v
}

// StartDeviceAuth requests a device code from the authorization server.
func (c *Client) StartDeviceAuth(ctx context.Context) (*DeviceCodeResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Discovery.DeviceAuthorizationEndpoint, strings.NewReader(c.form().Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("starting device authorization: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, fmt.Errorf("reading device authorization response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var oauthErr struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(body, &oauthErr) == nil && oauthErr.Error != "" {
			return nil, fmt.Errorf("device authorization endpoint returned %s: %s: %s", resp.Status, oauthErr.Error, oauthErr.ErrorDescription)
		}
		return nil, fmt.Errorf("device authorization endpoint returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var dcr DeviceCodeResponse
	if err := json.Unmarshal(body, &dcr); err != nil {
		return nil, fmt.Errorf("decoding device authorization response: %w", err)
	}
	return &dcr, nil
}

var (
	ErrAuthorizationPending = errors.New("authorization_pending")
	ErrSlowDown             = errors.New("slow_down")
	ErrExpiredToken         = errors.New("expired_token")
	ErrAccessDenied         = errors.New("access_denied")
)

func (c *Client) doTokenRequest(ctx context.Context, form url.Values) (*Tokens, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Discovery.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling token endpoint: %w", err)
	}
	defer resp.Body.Close()

	var raw struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		TokenType        string `json:"token_type"`
		ExpiresIn        int    `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding token response (status %s): %w", resp.Status, err)
	}

	if raw.Error != "" {
		switch raw.Error {
		case "authorization_pending":
			return nil, ErrAuthorizationPending
		case "slow_down":
			return nil, ErrSlowDown
		case "expired_token":
			return nil, ErrExpiredToken
		case "access_denied":
			return nil, ErrAccessDenied
		default:
			return nil, fmt.Errorf("token endpoint error: %s (%s)", raw.Error, raw.ErrorDescription)
		}
	}
	if resp.StatusCode != http.StatusOK || raw.AccessToken == "" {
		return nil, fmt.Errorf("token endpoint returned %s with no access_token", resp.Status)
	}

	return &Tokens{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		TokenType:    raw.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
	}, nil
}

// PollDeviceToken polls the token endpoint until the user approves (or the
// device code expires / is denied), respecting the server-provided interval.
func (c *Client) PollDeviceToken(ctx context.Context, dcr *DeviceCodeResponse) (*Tokens, error) {
	interval := time.Duration(dcr.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dcr.ExpiresIn) * time.Second)

	form := c.form()
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	form.Set("device_code", dcr.DeviceCode)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, ErrExpiredToken
			}
			toks, err := c.doTokenRequest(ctx, form)
			if err == nil {
				return toks, nil
			}
			switch {
			case errors.Is(err, ErrAuthorizationPending):
				continue
			case errors.Is(err, ErrSlowDown):
				ticker.Reset(interval + 5*time.Second)
				continue
			default:
				return nil, err
			}
		}
	}
}

// RefreshTokens exchanges a refresh token for a new token set.
func (c *Client) RefreshTokens(ctx context.Context, refreshToken string) (*Tokens, error) {
	form := c.form()
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	return c.doTokenRequest(ctx, form)
}

// ParseJWTClaims decodes (without verifying) the payload of a JWT, returning
// its claims as a generic map. The token was issued to us directly by the
// authorization server over TLS, so signature verification is unnecessary
// for merely reading claims such as preferred_username.
func ParseJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("not a JWT: expected 3 dot-separated parts, got %d", len(parts))
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding JWT payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, fmt.Errorf("parsing JWT claims: %w", err)
	}
	return claims, nil
}

// StringClaim is a small helper for pulling a string claim out of ParseJWTClaims' result.
func StringClaim(claims map[string]any, key string) string {
	if v, ok := claims[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

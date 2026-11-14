package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRefreshRejectedSessionIsInvalidGrant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Offline user session not found"}`))
	}))
	defer srv.Close()

	c := NewClient(&Discovery{TokenEndpoint: srv.URL}, "client", "", nil)
	_, err := c.RefreshTokens(context.Background(), "dead-refresh-token")
	if !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("err = %v, want ErrInvalidGrant", err)
	}
	// The message stays as informative as before.
	if !strings.Contains(err.Error(), "Offline user session not found") {
		t.Errorf("error lost its description: %v", err)
	}
}

func TestOtherTokenErrorsAreNotInvalidGrant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"bad secret"}`))
	}))
	defer srv.Close()

	c := NewClient(&Discovery{TokenEndpoint: srv.URL}, "client", "", nil)
	if _, err := c.RefreshTokens(context.Background(), "x"); err == nil || errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("err = %v, want a non-invalid_grant error", err)
	}
}

func TestDeleteTokensRemovesAndToleratesMissing(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if err := SaveTokens("p", &Tokens{AccessToken: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteTokens("p"); err != nil {
		t.Fatal(err)
	}
	if tok, err := LoadTokens("p"); err != nil || tok != nil {
		t.Errorf("after delete LoadTokens = %v, %v; want nil, nil", tok, err)
	}
	if err := DeleteTokens("p"); err != nil {
		t.Errorf("deleting again: %v", err)
	}
}

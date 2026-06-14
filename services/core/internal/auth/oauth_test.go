package auth_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/zero-agent/core/internal/auth"
)

func newCloser(s string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(s))
}

func TestOAuthConfigValidate(t *testing.T) {
	cfg := auth.OAuthConfig{}
	if err := cfg.Validate(); err == nil {
		t.Fatal("empty config should fail validation")
	}
	cfg = auth.OAuthConfig{ClientID: "id"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("missing secret should fail")
	}
	cfg = auth.OAuthConfig{ClientID: "id", ClientSecret: "secret"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("missing callback should fail")
	}
	cfg.CallbackURL = "http://127.0.0.1:8910/auth/google/callback"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("complete config should validate, got %v", err)
	}
}

func TestAuthURLContainsRequiredParams(t *testing.T) {
	cfg := auth.OAuthConfig{
		ClientID:     "client.apps.googleusercontent.com",
		ClientSecret: "secret",
		CallbackURL:  "http://127.0.0.1:8910/auth/google/callback",
	}
	pkce, err := auth.NewPkce()
	if err != nil {
		t.Fatalf("pkce: %v", err)
	}
	state, err := auth.RandomState()
	if err != nil {
		t.Fatalf("state: %v", err)
	}

	raw := cfg.AuthURL(state, pkce)
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Host != "accounts.google.com" {
		t.Fatalf("host = %q", parsed.Host)
	}
	q := parsed.Query()
	for _, k := range []string{"client_id", "redirect_uri", "response_type", "scope", "state", "code_challenge", "code_challenge_method"} {
		if q.Get(k) == "" {
			t.Fatalf("missing query param %q", k)
		}
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Fatalf("code_challenge_method = %q", q.Get("code_challenge_method"))
	}
	if !strings.Contains(q.Get("scope"), "openid") || !strings.Contains(q.Get("scope"), "email") || !strings.Contains(q.Get("scope"), "profile") {
		t.Fatalf("scope must include openid email profile, got %q", q.Get("scope"))
	}
	if q.Get("state") != state {
		t.Fatalf("state mismatch")
	}
	if q.Get("code_challenge") != pkce.Challenge {
		t.Fatalf("code_challenge mismatch")
	}
}

func TestPkceVerifierAndChallengeShape(t *testing.T) {
	p, err := auth.NewPkce()
	if err != nil {
		t.Fatalf("pkce: %v", err)
	}
	// Both base64url; verifier 43 chars (32 bytes), challenge 43 chars (sha256 -> 32 bytes).
	if len(p.Verifier) != 43 {
		t.Fatalf("verifier len = %d, want 43", len(p.Verifier))
	}
	if len(p.Challenge) != 43 {
		t.Fatalf("challenge len = %d, want 43", len(p.Challenge))
	}
	if p.Verifier == p.Challenge {
		t.Fatal("verifier and challenge must differ")
	}
}

// fakeRoundTripper lets us inject a static response for code exchange tests
// without spinning up an httptest server.
type fakeRoundTripper struct {
	status int
	body   string
}

func (f *fakeRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: f.status,
		Body:       newCloser(f.body),
		Header:     make(http.Header),
	}, nil
}

func TestExchangeCodeParsesSuccess(t *testing.T) {
	cfg := auth.OAuthConfig{
		ClientID:     "id",
		ClientSecret: "secret",
		CallbackURL:  "http://127.0.0.1:8910/auth/google/callback",
	}
	body, _ := json.Marshal(map[string]any{
		"access_token": "tok-abc",
		"expires_in":   3600,
		"token_type":   "Bearer",
		"scope":        "openid email profile",
	})
	client := &http.Client{Transport: &fakeRoundTripper{status: 200, body: string(body)}}

	tok, err := cfg.ExchangeCode(context.Background(), client, "code-xyz", "verifier-123")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if tok.AccessToken != "tok-abc" {
		t.Fatalf("access_token = %q", tok.AccessToken)
	}
}

func TestExchangeCodeRejectsNon2xx(t *testing.T) {
	cfg := auth.OAuthConfig{
		ClientID:     "id",
		ClientSecret: "secret",
		CallbackURL:  "http://127.0.0.1:8910/auth/google/callback",
	}
	client := &http.Client{Transport: &fakeRoundTripper{status: 400, body: `{"error":"invalid_grant"}`}}

	if _, err := cfg.ExchangeCode(context.Background(), client, "bad-code", "verifier"); err == nil {
		t.Fatal("expected error on non-2xx")
	}
}

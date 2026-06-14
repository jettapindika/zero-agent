package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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

// Google OAuth + OIDC endpoints. Documented at
// https://accounts.google.com/.well-known/openid-configuration. Hardcoded here
// to avoid an extra discovery round-trip on every login.
const (
	googleAuthEndpoint     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint    = "https://oauth2.googleapis.com/token"
	googleUserinfoEndpoint = "https://openidconnect.googleapis.com/v1/userinfo"
	googleScopes           = "openid email profile"
)

// OAuthConfig is the immutable, per-process Google OAuth client config.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
}

// Validate ensures the config is usable. ClientSecret is required because the
// daemon is the trusted backend; we never use the public client flow here.
func (c OAuthConfig) Validate() error {
	if c.ClientID == "" {
		return errors.New("GOOGLE_CLIENT_ID is required")
	}
	if c.ClientSecret == "" {
		return errors.New("GOOGLE_CLIENT_SECRET is required")
	}
	if c.CallbackURL == "" {
		return errors.New("GOOGLE_CALLBACK_URL is required")
	}
	return nil
}

// Pkce holds the per-request PKCE pair. Verifier stays server-side; Challenge
// is sent to Google. The desktop never sees either.
type Pkce struct {
	Verifier  string
	Challenge string
}

// NewPkce mints a fresh verifier + S256 challenge.
func NewPkce() (Pkce, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Pkce{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(raw[:])
	sum := sha256.Sum256([]byte(verifier))
	return Pkce{
		Verifier:  verifier,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
	}, nil
}

// RandomState returns a CSRF token suitable for the OAuth `state` parameter.
func RandomState() (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

// AuthURL builds the redirect-to-Google URL with state + PKCE.
func (c OAuthConfig) AuthURL(state string, pkce Pkce) string {
	q := url.Values{}
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", c.CallbackURL)
	q.Set("response_type", "code")
	q.Set("scope", googleScopes)
	q.Set("state", state)
	q.Set("code_challenge", pkce.Challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("access_type", "online")
	q.Set("include_granted_scopes", "true")
	q.Set("prompt", "select_account")
	return googleAuthEndpoint + "?" + q.Encode()
}

// TokenResponse mirrors the relevant subset of Google's /token reply.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
	IDToken     string `json:"id_token"`
	Scope       string `json:"scope"`
}

// ExchangeCode swaps a one-time auth code for an access token.
func (c OAuthConfig) ExchangeCode(ctx context.Context, client *http.Client, code, verifier string) (*TokenResponse, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	form := url.Values{}
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", c.CallbackURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token exchange failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, errors.New("token exchange returned empty access_token")
	}
	return &tr, nil
}

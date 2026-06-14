package auth_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/zero-agent/core/internal/auth"
)

func TestFetchUserInfoSuccess(t *testing.T) {
	body := `{
		"sub": "1234567890",
		"email": "alice@example.com",
		"email_verified": true,
		"name": "Alice Example",
		"picture": "https://lh3.example/a.png"
	}`
	client := &http.Client{Transport: &staticRT{status: 200, body: body}}

	info, err := auth.FetchUserInfo(context.Background(), client, "tok")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if info.Sub != "1234567890" || info.Email != "alice@example.com" || !info.EmailVerified {
		t.Fatalf("got %+v", info)
	}
}

func TestFetchUserInfoRejectsUnverifiedEmail(t *testing.T) {
	body := `{
		"sub": "x",
		"email": "alice@example.com",
		"email_verified": false
	}`
	client := &http.Client{Transport: &staticRT{status: 200, body: body}}

	if _, err := auth.FetchUserInfo(context.Background(), client, "tok"); err == nil {
		t.Fatal("expected error on unverified email")
	}
}

func TestFetchUserInfoRejectsMissingSub(t *testing.T) {
	body := `{"email":"a@b.c","email_verified":true}`
	client := &http.Client{Transport: &staticRT{status: 200, body: body}}

	if _, err := auth.FetchUserInfo(context.Background(), client, "tok"); err == nil {
		t.Fatal("expected error on missing sub")
	}
}

func TestFetchUserInfoRejectsNon2xx(t *testing.T) {
	client := &http.Client{Transport: &staticRT{status: 401, body: "nope"}}
	if _, err := auth.FetchUserInfo(context.Background(), client, "tok"); err == nil {
		t.Fatal("expected error on non-2xx")
	}
}

type staticRT struct {
	status int
	body   string
}

func (s *staticRT) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: s.status,
		Body:       newCloser2(s.body),
		Header:     make(http.Header),
	}, nil
}

// newCloser2 mirrors newCloser from oauth_test.go to avoid cross-file deps.
func newCloser2(s string) noopCloser {
	return noopCloser{Reader: strings.NewReader(s)}
}

type noopCloser struct {
	*strings.Reader
}

func (noopCloser) Close() error { return nil }

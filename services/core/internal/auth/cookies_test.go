package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// signSessionID and verifySessionID are unexported, so this file lives in the
// same package (no _test suffix on the package name).
func TestSignVerifyRoundtrip(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	id := "abc-123-def"

	signed := signSessionID(id, secret)
	if !strings.HasPrefix(signed, id+".") {
		t.Fatalf("signed = %q does not start with id", signed)
	}

	got, err := verifySessionID(signed, secret)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got != id {
		t.Fatalf("got %q, want %q", got, id)
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	signed := signSessionID("abc-123", secret)

	tampered := "evil-id." + strings.SplitN(signed, ".", 2)[1]
	if _, err := verifySessionID(tampered, secret); err == nil {
		t.Fatal("expected tampered id to fail verification")
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	signed := signSessionID("abc-123", secret)

	tampered := strings.SplitN(signed, ".", 2)[0] + ".AAAA"
	if _, err := verifySessionID(tampered, secret); err == nil {
		t.Fatal("expected tampered signature to fail")
	}
}

func TestSetAndReadCookieRoundtrip(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")

	rw := httptest.NewRecorder()
	SetSessionCookie(rw, "session-99", secret, false)

	resp := rw.Result()
	defer resp.Body.Close()
	cookies := resp.Cookies()
	if len(cookies) != 1 || cookies[0].Name != CookieName {
		t.Fatalf("expected one %q cookie, got %+v", CookieName, cookies)
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(cookies[0])
	got := ReadSessionID(r, secret)
	if got != "session-99" {
		t.Fatalf("ReadSessionID = %q", got)
	}
}

func TestReadCookieReturnsEmptyWhenMissingOrInvalid(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := ReadSessionID(r, secret); got != "" {
		t.Fatalf("missing cookie should return empty, got %q", got)
	}

	r.AddCookie(&http.Cookie{Name: CookieName, Value: "garbage"})
	if got := ReadSessionID(r, secret); got != "" {
		t.Fatalf("invalid cookie should return empty, got %q", got)
	}
}

func TestClearCookieIsTombstone(t *testing.T) {
	rw := httptest.NewRecorder()
	ClearSessionCookie(rw, false)
	cookies := rw.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one tombstone cookie")
	}
	if cookies[0].MaxAge >= 0 {
		t.Fatalf("MaxAge = %d, expected negative tombstone", cookies[0].MaxAge)
	}
}

func TestReadSessionIDAcceptsAuthorizationBearer(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	signed := signSessionID("desktop-session-1", secret)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+signed)

	got := ReadSessionID(r, secret)
	if got != "desktop-session-1" {
		t.Fatalf("got %q, want %q", got, "desktop-session-1")
	}
}

func TestReadSessionIDIgnoresMalformedAuthorization(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")

	cases := []string{
		"",
		"Bearer ",
		"Basic abc",
		"Bearer not-a-signed-value",
		"Bearer foo.bar",
	}
	for _, h := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if h != "" {
			r.Header.Set("Authorization", h)
		}
		if got := ReadSessionID(r, secret); got != "" {
			t.Fatalf("header %q: expected empty, got %q", h, got)
		}
	}
}

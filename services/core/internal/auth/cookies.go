package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
)

// CookieName is the browser cookie name carrying the signed session id.
const CookieName = "zero_auth"

// cookieMaxAge mirrors auth_sessions.expires_at (30 days). Browsers will drop
// the cookie at the same time the server-side row is purged.
const cookieMaxAge = 30 * 24 * time.Hour

// CookieSession TTL is exposed so callers can use the same value when minting
// the server-side session row.
const SessionTTL = cookieMaxAge

var errSignatureMismatch = errors.New("cookie signature mismatch")

// SignSessionID is the exported form callers outside the package use when
// they need the signed cookie value (e.g. to hand it to a Tauri webview that
// can't read cookies the system browser set).
func SignSessionID(id string, secret []byte) string { return signSessionID(id, secret) }

// signSessionID returns "<id>.<base64url(hmac(secret, id))>".
func signSessionID(id string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(id))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return id + "." + sig
}

// verifySessionID returns the bare session id when the signature matches the
// secret. Errors on malformed or tampered values.
func verifySessionID(value string, secret []byte) (string, error) {
	dot := strings.LastIndexByte(value, '.')
	if dot <= 0 || dot == len(value)-1 {
		return "", errors.New("malformed cookie")
	}
	id, sig := value[:dot], value[dot+1:]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(id))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", errSignatureMismatch
	}
	return id, nil
}

// SetSessionCookie writes the signed session cookie. secure=true sets the
// Secure flag; callers should pass true except on plain-http loopback.
func SetSessionCookie(w http.ResponseWriter, sessionID string, secret []byte, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    signSessionID(sessionID, secret),
		Path:     "/",
		MaxAge:   int(cookieMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie writes a tombstone cookie that browsers drop immediately.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ReadSessionID extracts and verifies the session id from the request, or
// returns the empty string when no valid credential is present. Two carriers
// are accepted, in order:
//
//	1. Cookie zero_auth=<signed>            (browsers, system browser flow)
//	2. Authorization: Bearer <signed>       (Tauri webview, where cookies
//	                                         set in the system browser are
//	                                         not visible to fetch())
//
// Both carry the same signed value; same trust either way.
func ReadSessionID(r *http.Request, secret []byte) string {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		if id, err := verifySessionID(c.Value, secret); err == nil {
			return id
		}
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		raw := strings.TrimSpace(authHeader[len("Bearer "):])
		if raw != "" {
			if id, err := verifySessionID(raw, secret); err == nil {
				return id
			}
		}
	}
	return ""
}

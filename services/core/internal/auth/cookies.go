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
// returns the empty string when no valid cookie is present.
func ReadSessionID(r *http.Request, secret []byte) string {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return ""
	}
	id, err := verifySessionID(c.Value, secret)
	if err != nil {
		return ""
	}
	return id
}

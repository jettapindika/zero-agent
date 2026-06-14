package server

import (
	"net/http"
	"strings"

	"github.com/zero-agent/core/internal/auth"
)

// handleAuthDisabled returns a 503 with a JSON body that tells the operator
// how to turn login on. Wired by routes() when auth.Service is nil so the
// /auth/* paths are never silently 404 — the previous behavior made misconfig
// look like a code bug.
func handleAuthDisabled(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"error": "auth disabled",
		"hint":  "set ZERO_AUTH_ENABLED=true with GOOGLE_CLIENT_ID/SECRET + SESSION_SECRET, then rebuild and relaunch zero-server",
		"path":  r.URL.Path,
		"docs":  "see README section 'Optional: enable Google sign-in on the desktop app'",
	})
}

// secureForRequest reports whether the response cookie should set the Secure
// flag. We set it whenever the request host is NOT a loopback address, so the
// cookie still works on plain http://127.0.0.1 during local dev but flips to
// secure-only as soon as the daemon is exposed off-loopback.
func secureForRequest(r *http.Request) bool {
	host := r.Host
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return false
	}
	return true
}

// handleAuthStart begins the OAuth dance: mints state + PKCE, stores the
// verifier server-side, and redirects the browser to Google.
func (s *Server) handleAuthStart(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		http.NotFound(w, r)
		return
	}
	url, _, err := s.auth.BeginFlow()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// handleAuthCallback completes the OAuth dance and sets the session cookie.
// Returns a tiny self-closing HTML page that the system browser displays.
func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	if errMsg := q.Get("error"); errMsg != "" {
		writeError(w, http.StatusBadRequest, "google denied: "+errMsg)
		return
	}
	state := q.Get("state")
	code := q.Get("code")
	if state == "" || code == "" {
		writeError(w, http.StatusBadRequest, "missing state or code")
		return
	}

	sessionID, user, err := s.auth.CompleteFlow(r.Context(), state, code)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	auth.SetSessionCookie(w, sessionID, s.auth.Secret(), secureForRequest(r))

	// Notify any open desktop windows so they can refetch /auth/me without
	// polling.
	s.bus.Publish("auth.signed_in", "", "", map[string]string{
		"userId":      user.ID,
		"email":       user.Email,
		"displayName": user.DisplayName,
		"role":        user.Role,
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html>
<html><head><meta charset="utf-8"><title>Signed in to Zero</title>
<style>body{font-family:system-ui,sans-serif;background:#0f0f10;color:#e7e7e7;display:flex;align-items:center;justify-content:center;height:100vh;margin:0}main{text-align:center}h1{font-size:18px}p{color:#9aa0a3;font-size:14px}</style></head>
<body><main><h1>✓ Signed in to Zero</h1><p>You can close this tab and return to the desktop app.</p></main>
<script>setTimeout(()=>window.close(),1500)</script></body></html>`))
}

// handleAuthMe returns the current user + isDev flag, or 401 when no valid
// session is attached.
func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		http.NotFound(w, r)
		return
	}
	user, sess, err := s.auth.LookupSession(r.Context(), r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":        user,
		"isDev":       s.auth.IsDev(user.Email),
		"sessionId":   sess.ID,
		"expiresAtMs": sess.ExpiresAt,
	})
}

// handleAuthLogout clears the cookie + deletes the server-side row.
func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		http.NotFound(w, r)
		return
	}
	sid, err := s.auth.SignOut(r.Context(), r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	auth.ClearSessionCookie(w, secureForRequest(r))
	if sid != "" {
		s.bus.Publish("auth.signed_out", "", "", map[string]string{"sessionId": sid})
	}
	w.WriteHeader(http.StatusNoContent)
}

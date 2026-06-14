package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/zero-agent/core/internal/storage"
)

// ctxKey is unexported so external packages can't accidentally collide with
// our context values. Routes pull the user via UserFromContext.
type ctxKey int

const (
	ctxUserKey ctxKey = iota
	ctxSessionKey
)

// UserFromContext returns the authenticated user attached by RequireAuth, or
// (nil, false) when the request was not authenticated.
func UserFromContext(ctx context.Context) (*storage.User, bool) {
	u, ok := ctx.Value(ctxUserKey).(*storage.User)
	return u, ok && u != nil
}

// SessionFromContext returns the auth session row attached by RequireAuth.
func SessionFromContext(ctx context.Context) (*storage.AuthSession, bool) {
	s, ok := ctx.Value(ctxSessionKey).(*storage.AuthSession)
	return s, ok && s != nil
}

// IsPublicPath reports whether a path should bypass RequireAuth even when the
// gate is enabled. Health, OAuth handshake, and SSE all stay reachable so the
// login flow can complete and the desktop can subscribe to bus events that
// announce sign-in.
func IsPublicPath(path string) bool {
	switch path {
	case "/health", "/openapi.json":
		return true
	}
	if len(path) >= 6 && path[:6] == "/auth/" {
		return true
	}
	if path == "/events" || (len(path) > 7 && path[:7] == "/events") {
		return true
	}
	return false
}

// RequireAuth returns a middleware that gates the wrapped handler behind a
// valid session cookie. When Service.Enabled() is false the middleware is a
// no-op so existing single-user installs keep working.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.enabled || IsPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		user, sess, err := s.LookupSession(r.Context(), r)
		if err != nil {
			writeJSONErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if user == nil {
			w.Header().Set("WWW-Authenticate", `Cookie realm="zero"`)
			writeJSONErr(w, http.StatusUnauthorized, "authentication required")
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserKey, user)
		ctx = context.WithValue(ctx, ctxSessionKey, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireDev gates a handler behind the dev role. Must be composed inside
// RequireAuth — it expects the user to already be on the context.
func (s *Service) RequireDev(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			writeJSONErr(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if user.Role != "dev" {
			writeJSONErr(w, http.StatusForbidden, "dev role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSONErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

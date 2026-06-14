package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/zero-agent/core/internal/auth"
	"github.com/zero-agent/core/internal/storage"
)

func newServiceWithUser(t *testing.T, role string) (*auth.Service, *storage.User, *storage.AuthSession) {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	user, err := db.UpsertUser(context.Background(), storage.UpsertUserInput{
		GoogleID:    "g-mw",
		Email:       "mw@example.com",
		DisplayName: "MW",
		Role:        role,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	sess, err := db.CreateAuthSession(context.Background(), user.ID, time.Hour)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	svc, err := auth.NewService(auth.Config{
		OAuth: auth.OAuthConfig{
			ClientID:     "id",
			ClientSecret: "secret",
			CallbackURL:  "http://127.0.0.1:8910/auth/google/callback",
		},
		Secret:    []byte("0123456789abcdef0123456789abcdef"),
		DevEmails: []string{"mw@example.com"},
		Enabled:   true,
		DB:        db,
	})
	if err != nil {
		t.Fatalf("service: %v", err)
	}
	return svc, user, sess
}

func TestRequireAuthRejectsMissingCookie(t *testing.T) {
	svc, _, _ := newServiceWithUser(t, "user")

	called := false
	h := svc.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rw := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	h.ServeHTTP(rw, r)

	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rw.Code)
	}
	if called {
		t.Fatal("handler should not run")
	}
}

func TestRequireAuthAcceptsSignedCookie(t *testing.T) {
	svc, user, sess := newServiceWithUser(t, "user")

	called := false
	h := svc.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := auth.UserFromContext(r.Context())
		if !ok || got.ID != user.ID {
			t.Fatalf("user ctx = %+v, ok=%v", got, ok)
		}
		called = true
	}))

	rw := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	auth.SetSessionCookie(rw, sess.ID, svc.Secret(), false)
	for _, c := range rw.Result().Cookies() {
		r.AddCookie(c)
	}
	rw2 := httptest.NewRecorder()
	h.ServeHTTP(rw2, r)

	if rw2.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rw2.Code)
	}
	if !called {
		t.Fatal("handler did not run")
	}
}

func TestRequireAuthSkipsPublicPaths(t *testing.T) {
	svc, _, _ := newServiceWithUser(t, "user")

	for _, path := range []string{"/health", "/auth/google/start", "/auth/me", "/events"} {
		called := false
		h := svc.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))
		rw := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, path, nil)
		h.ServeHTTP(rw, r)
		if rw.Code == http.StatusUnauthorized {
			t.Fatalf("public path %q rejected as 401", path)
		}
		if !called {
			t.Fatalf("public path %q did not run handler", path)
		}
	}
}

func TestRequireDevAllowsDevRole(t *testing.T) {
	svc, _, sess := newServiceWithUser(t, "dev")

	chain := svc.RequireAuth(svc.RequireDev(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	rw := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dev/runtime", nil)
	auth.SetSessionCookie(rw, sess.ID, svc.Secret(), false)
	for _, c := range rw.Result().Cookies() {
		r.AddCookie(c)
	}
	rw2 := httptest.NewRecorder()
	chain.ServeHTTP(rw2, r)

	if rw2.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rw2.Code)
	}
}

func TestRequireDevForbidsNonDevRole(t *testing.T) {
	svc, _, sess := newServiceWithUser(t, "user")

	chain := svc.RequireAuth(svc.RequireDev(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not run")
	})))

	rw := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dev/runtime", nil)
	auth.SetSessionCookie(rw, sess.ID, svc.Secret(), false)
	for _, c := range rw.Result().Cookies() {
		r.AddCookie(c)
	}
	rw2 := httptest.NewRecorder()
	chain.ServeHTTP(rw2, r)

	if rw2.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rw2.Code)
	}
}

func TestServiceDisabledIsNoop(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc, err := auth.NewService(auth.Config{
		Enabled: false,
		DB:      db,
	})
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	called := false
	h := svc.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rw := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	h.ServeHTTP(rw, r)

	if !called || rw.Code == http.StatusUnauthorized {
		t.Fatalf("disabled service should pass through; called=%v code=%d", called, rw.Code)
	}
}

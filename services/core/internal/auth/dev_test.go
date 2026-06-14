package auth_test

import (
	"os"
	"testing"

	"github.com/zero-agent/core/internal/auth"
)

func TestDevEmailsParsesCommaList(t *testing.T) {
	t.Setenv("DEV_EMAILS", "Alice@Example.com, BOB@example.com ,carol@example.com")
	t.Setenv("DEV_EMAIL", "")

	got := auth.DevEmails()
	want := []string{"alice@example.com", "bob@example.com", "carol@example.com"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], v)
		}
	}
}

func TestDevEmailsFallsBackToSingleVar(t *testing.T) {
	os.Unsetenv("DEV_EMAILS")
	t.Setenv("DEV_EMAIL", "Solo@Example.com")
	got := auth.DevEmails()
	if len(got) != 1 || got[0] != "solo@example.com" {
		t.Fatalf("got %v", got)
	}
}

func TestIsDevCaseInsensitive(t *testing.T) {
	allowlist := []string{"dev@example.com"}
	if !auth.IsDev("DEV@example.com", allowlist) {
		t.Fatal("expected case-insensitive match")
	}
	if auth.IsDev("other@example.com", allowlist) {
		t.Fatal("non-dev should not match")
	}
	if auth.IsDev("", allowlist) {
		t.Fatal("empty email should never match")
	}
}

func TestRoleForReturnsDevOrUser(t *testing.T) {
	allowlist := []string{"dev@example.com"}
	if got := auth.RoleFor("dev@example.com", allowlist); got != "dev" {
		t.Fatalf("dev path: %q", got)
	}
	if got := auth.RoleFor("user@example.com", allowlist); got != "user" {
		t.Fatalf("user path: %q", got)
	}
}

// Package auth implements Google OAuth2 + OIDC userinfo for the Zero desktop
// app. The Go daemon holds the client secret; the Tauri window only sees a
// signed cookie. See README "Optional: enable Google sign-in".
package auth

import (
	"os"
	"strings"
)

// DevEmails parses DEV_EMAILS (comma list) with DEV_EMAIL fallback for
// compatibility with the original spec wording. All entries are lowercased.
func DevEmails() []string {
	raw := os.Getenv("DEV_EMAILS")
	if raw == "" {
		raw = os.Getenv("DEV_EMAIL")
	}
	out := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		v := strings.TrimSpace(strings.ToLower(part))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// IsDev reports whether the given email is in the dev allowlist. Comparison is
// case-insensitive.
func IsDev(email string, allowlist []string) bool {
	if email == "" || len(allowlist) == 0 {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(email))
	for _, dev := range allowlist {
		if dev == target {
			return true
		}
	}
	return false
}

// RoleFor returns "dev" if the email is in the allowlist, otherwise "user".
func RoleFor(email string, allowlist []string) string {
	if IsDev(email, allowlist) {
		return "dev"
	}
	return "user"
}

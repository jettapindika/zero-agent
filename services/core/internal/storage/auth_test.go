package storage_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/zero-agent/core/internal/storage"
)

func openTestDB(t *testing.T) *storage.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestUpsertUserInsertsThenUpdatesByGoogleID(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	first, err := db.UpsertUser(ctx, storage.UpsertUserInput{
		GoogleID:    "g-1",
		Email:       "Alice@Example.com",
		DisplayName: "Alice",
		AvatarURL:   "https://lh3.example/a.png",
		Role:        "user",
	})
	if err != nil {
		t.Fatalf("upsert insert: %v", err)
	}
	if first.Email != "alice@example.com" {
		t.Fatalf("email should be lowercased, got %q", first.Email)
	}
	if first.Role != "user" {
		t.Fatalf("role = %q", first.Role)
	}

	// Same googleID with new fields should update, not insert.
	second, err := db.UpsertUser(ctx, storage.UpsertUserInput{
		GoogleID:    "g-1",
		Email:       "alice@new.example",
		DisplayName: "Alice Renamed",
		AvatarURL:   "https://lh3.example/b.png",
		Role:        "dev",
	})
	if err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected stable id; got %q -> %q", first.ID, second.ID)
	}
	if second.DisplayName != "Alice Renamed" || second.Role != "dev" {
		t.Fatalf("update did not refresh fields: %+v", second)
	}
}

func TestUpsertUserDefaultsRoleToUser(t *testing.T) {
	db := openTestDB(t)
	got, err := db.UpsertUser(context.Background(), storage.UpsertUserInput{
		GoogleID: "g-2",
		Email:    "bob@example.com",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got.Role != "user" {
		t.Fatalf("default role = %q", got.Role)
	}
}

func TestGetUserByEmailIsCaseInsensitive(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if _, err := db.UpsertUser(ctx, storage.UpsertUserInput{GoogleID: "g-3", Email: "Carol@Example.com"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := db.GetUserByEmail(ctx, "CAROL@example.com")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.Email != "carol@example.com" {
		t.Fatalf("email = %q", got.Email)
	}
}

func TestAuthSessionLifecycle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	user, err := db.UpsertUser(ctx, storage.UpsertUserInput{GoogleID: "g-4", Email: "d@example.com"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	sess, err := db.CreateAuthSession(ctx, user.ID, time.Hour)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sess.ID == "" || sess.UserID != user.ID {
		t.Fatalf("bad session: %+v", sess)
	}

	got, gotUser, err := db.GetAuthSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != sess.ID || gotUser.ID != user.ID {
		t.Fatalf("mismatch: %+v / %+v", got, gotUser)
	}

	if err := db.DeleteAuthSession(ctx, sess.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, _, err := db.GetAuthSession(ctx, sess.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestPurgeExpiredAuthSessions(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	user, _ := db.UpsertUser(ctx, storage.UpsertUserInput{GoogleID: "g-5", Email: "e@example.com"})
	live, _ := db.CreateAuthSession(ctx, user.ID, time.Hour)

	// Expire by direct SQL update so we don't have to time-skip.
	if _, err := db.Conn().Exec(`UPDATE auth_sessions SET expires_at = 1 WHERE id != ?`, live.ID); err != nil {
		t.Fatalf("update: %v", err)
	}
	// Insert a definitely-stale row too.
	stale, _ := db.CreateAuthSession(ctx, user.ID, time.Millisecond)
	if _, err := db.Conn().Exec(`UPDATE auth_sessions SET expires_at = 1 WHERE id = ?`, stale.ID); err != nil {
		t.Fatalf("update: %v", err)
	}

	n, err := db.PurgeExpiredAuthSessions(ctx)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n == 0 {
		t.Fatalf("expected purge to remove rows")
	}
	if _, _, err := db.GetAuthSession(ctx, live.ID); err != nil {
		t.Fatalf("live session should still exist: %v", err)
	}
}

func TestUpdateUserRole(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	user, _ := db.UpsertUser(ctx, storage.UpsertUserInput{GoogleID: "g-6", Email: "f@example.com"})
	if err := db.UpdateUserRole(ctx, user.ID, "dev"); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := db.GetUserByID(ctx, user.ID)
	if got.Role != "dev" {
		t.Fatalf("role = %q", got.Role)
	}
}

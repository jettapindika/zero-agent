package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zero-agent/core/internal/storage"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("db file not created")
	}

	var tableName string
	err = db.Conn().QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&tableName)
	if err != nil {
		t.Fatalf("sessions table not found: %v", err)
	}
	if tableName != "sessions" {
		t.Fatalf("expected 'sessions', got %q", tableName)
	}
}

func TestWALMode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wal.db")

	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	var mode string
	err = db.Conn().QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("expected wal mode, got %q", mode)
	}
}

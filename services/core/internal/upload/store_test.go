package upload

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreSavesAndRoundtripsBytes(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	body := []byte("hello world")
	path, n, err := s.Save("session-abc", "att123", ".txt", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if n != int64(len(body)) {
		t.Errorf("size = %d, want %d", n, len(body))
	}
	if !strings.HasPrefix(path, filepath.Join(root, "session-abc")) {
		t.Errorf("path %s should be inside session dir", path)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("roundtrip mismatch")
	}
}

func TestStoreRejectsBadIDs(t *testing.T) {
	s := NewStore(t.TempDir())
	cases := []struct {
		sessionID, attID string
	}{
		{"../../escape", "att1"},
		{"good", "../escape"},
		{"good", ""},
		{"", "good"},
		{"with space", "ok"},
	}
	for _, c := range cases {
		if _, _, err := s.Save(c.sessionID, c.attID, ".txt", bytes.NewReader([]byte("x"))); err == nil {
			t.Errorf("Save(%q, %q) should reject bad IDs", c.sessionID, c.attID)
		}
	}
}

func TestStoreRefusesRemoveOutsideRoot(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	other := filepath.Join(t.TempDir(), "evil.txt")
	if err := os.WriteFile(other, []byte("evil"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	err := s.Remove(other)
	if err == nil {
		t.Fatal("Remove outside root should error")
	}
	if _, statErr := os.Stat(other); statErr != nil {
		t.Errorf("file outside root must NOT be deleted: %v", statErr)
	}
}

func TestStoreRemoveSession(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	_, _, err := s.Save("sess-1", "att1", ".txt", bytes.NewReader([]byte("x")))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := s.RemoveSession("sess-1"); err != nil {
		t.Fatalf("remove session: %v", err)
	}
	_, statErr := os.Stat(filepath.Join(root, "sess-1"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("session dir should be gone, got stat err = %v", statErr)
	}
}

func TestSanitizeExt(t *testing.T) {
	cases := map[string]string{
		".txt":         ".txt",
		"txt":          ".txt",
		".PDF":         ".pdf",
		".tar.gz":      "",
		".../escape":   "",
		"":             "",
		strings.Repeat(".x", 30): "",
	}
	for in, want := range cases {
		if got := sanitizeExt(in); got != want {
			t.Errorf("sanitizeExt(%q) = %q, want %q", in, got, want)
		}
	}
}

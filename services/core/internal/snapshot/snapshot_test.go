package snapshot_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/zero-agent/core/internal/snapshot"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@zero.dev")
	run(t, dir, "git", "config", "user.name", "Zero Test")
	os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0o644)
	run(t, dir, "git", "add", "-A")
	run(t, dir, "git", "commit", "-m", "initial")
	return dir
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %s", name, args, out)
	}
}

func TestTrackCreatesSnapshot(t *testing.T) {
	dir := setupGitRepo(t)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0o644)

	hash, err := snapshot.Track(dir, "sess1")
	if err != nil {
		t.Fatalf("track: %v", err)
	}
	if hash == "" {
		t.Fatal("expected hash")
	}
	if len(hash) < 7 {
		t.Fatalf("hash too short: %s", hash)
	}
}

func TestRevertRestoresState(t *testing.T) {
	dir := setupGitRepo(t)
	filePath := filepath.Join(dir, "file.txt")
	os.WriteFile(filePath, []byte("before"), 0o644)
	hash1, _ := snapshot.Track(dir, "sess1")

	os.WriteFile(filePath, []byte("after"), 0o644)
	snapshot.Track(dir, "sess1")

	if err := snapshot.Revert(dir, hash1); err != nil {
		t.Fatalf("revert: %v", err)
	}
	data, _ := os.ReadFile(filePath)
	if string(data) != "before" {
		t.Fatalf("expected 'before', got %q", string(data))
	}
}

func TestDiffShowsChanges(t *testing.T) {
	dir := setupGitRepo(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("v1"), 0o644)
	hash1, _ := snapshot.Track(dir, "sess1")

	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("v2"), 0o644)
	hash2, _ := snapshot.Track(dir, "sess1")

	diff, err := snapshot.Diff(dir, hash1, hash2)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
}

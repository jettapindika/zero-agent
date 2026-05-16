package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestZeroPathsUseHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got := zeroDir(); got != filepath.Join(home, ".zero") {
		t.Fatalf("zeroDir() = %q", got)
	}
	if got := pidPath(); got != filepath.Join(home, ".zero", "zero.pid") {
		t.Fatalf("pidPath() = %q", got)
	}
	if got := logPath(); got != filepath.Join(home, ".zero", "zero.log") {
		t.Fatalf("logPath() = %q", got)
	}
}

func TestEnsureZeroDirCreatesPrivateDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := ensureZeroDir(); err != nil {
		t.Fatalf("ensureZeroDir() error = %v", err)
	}
	info, err := os.Stat(zeroDir())
	if err != nil {
		t.Fatalf("stat zero dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("zero dir is not directory")
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("zero dir perm = %o, want 700", perm)
	}
}

func TestIsTTYUnavailableError(t *testing.T) {
	if !isTTYUnavailableError(assertErr("could not open a new TTY: open /dev/tty: device not configured")) {
		t.Fatalf("expected TTY unavailable error to be detected")
	}
	if isTTYUnavailableError(assertErr("some other error")) {
		t.Fatalf("unexpected TTY unavailable detection")
	}
}

func TestDefaultModelUsesResponsive9RouterModel(t *testing.T) {
	if got := defaultModel(); got != "cx/gpt-5.5" {
		t.Fatalf("defaultModel() = %q, want cx/gpt-5.5", got)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

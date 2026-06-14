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

func TestDefaultModelFallback(t *testing.T) {
	t.Setenv("ZERO_DEFAULT_MODEL", "")
	if got := defaultModel(); got != "gpt-4o-mini" {
		t.Fatalf("defaultModel() = %q, want gpt-4o-mini", got)
	}
}

func TestDefaultModelEnvOverride(t *testing.T) {
	t.Setenv("ZERO_DEFAULT_MODEL", "openai/gpt-4o")
	if got := defaultModel(); got != "openai/gpt-4o" {
		t.Fatalf("defaultModel() with env override = %q, want openai/gpt-4o", got)
	}
}

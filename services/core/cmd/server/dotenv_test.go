package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLineHandlesAssignmentsAndComments(t *testing.T) {
	cases := []struct {
		in       string
		wantKey  string
		wantVal  string
		wantOk   bool
	}{
		{"KEY=value", "KEY", "value", true},
		{"  KEY=value  ", "KEY", "value", true},
		{"KEY=\"value with space\"", "KEY", "value with space", true},
		{"KEY='single'", "KEY", "single", true},
		{"# comment", "", "", false},
		{"", "", "", false},
		{"NO_EQUALS_SIGN", "", "", false},
		{"=leading", "", "", false},
		{"K=", "K", "", true},
	}
	for _, tc := range cases {
		k, v, ok := parseLine(tc.in)
		if ok != tc.wantOk {
			t.Fatalf("%q: ok=%v want %v", tc.in, ok, tc.wantOk)
		}
		if k != tc.wantKey || v != tc.wantVal {
			t.Fatalf("%q: got (%q, %q) want (%q, %q)", tc.in, k, v, tc.wantKey, tc.wantVal)
		}
	}
}

func TestApplyEnvFileDoesNotClobberExistingEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	const body = "ZERO_TEST_DOTENV_NEW=new\nZERO_TEST_DOTENV_PRESET=fromfile\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("ZERO_TEST_DOTENV_PRESET", "preset-wins")
	os.Unsetenv("ZERO_TEST_DOTENV_NEW")

	applyEnvFile(path)

	if got := os.Getenv("ZERO_TEST_DOTENV_NEW"); got != "new" {
		t.Fatalf("missing key not loaded: %q", got)
	}
	if got := os.Getenv("ZERO_TEST_DOTENV_PRESET"); got != "preset-wins" {
		t.Fatalf("dotenv clobbered shell export: %q", got)
	}
}

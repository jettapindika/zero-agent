package main

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// loadDotEnv reads simple `KEY=VALUE` files from a few well-known locations
// and exposes any keys that are not already set in the process env. We avoid
// pulling in github.com/joho/godotenv just for this; the format we accept is
// intentionally narrow:
//
//   - one assignment per line
//   - blank lines and lines starting with '#' are ignored
//   - values are taken verbatim; surrounding "double" or 'single' quotes are
//     stripped so users can paste DSN strings with special characters
//   - no shell interpolation, no export prefix
//
// Search order, first match wins per key:
//   - $ZERO_DOTENV (explicit override)
//   - ./.env                      (repo root when launched from there)
//   - $HOME/.config/zero/.env     (user-global)
//   - $HOME/.zero/.env            (legacy, kept for back-compat)
func loadDotEnv() {
	for _, path := range candidatePaths() {
		applyEnvFile(path)
	}
}

func candidatePaths() []string {
	out := make([]string, 0, 4)
	if v := os.Getenv("ZERO_DOTENV"); v != "" {
		out = append(out, v)
	}
	if cwd, err := os.Getwd(); err == nil {
		out = append(out, filepath.Join(cwd, ".env"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, ".config", "zero", ".env"))
		out = append(out, filepath.Join(home, ".zero", ".env"))
	}
	return out
}

func applyEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			// Quietly skip permission errors etc; auth env still drives behavior.
			return
		}
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		k, v, ok := parseLine(scanner.Text())
		if !ok {
			continue
		}
		// Don't clobber an explicit shell export.
		if _, present := os.LookupEnv(k); present {
			continue
		}
		_ = os.Setenv(k, v)
	}
}

func parseLine(raw string) (string, string, bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	idx := strings.IndexByte(line, '=')
	if idx <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	val := strings.TrimSpace(line[idx+1:])
	val = unquote(val)
	return key, val, true
}

func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

package dotenv


import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func Load() {
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

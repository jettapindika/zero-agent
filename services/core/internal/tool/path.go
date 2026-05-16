package tool

import (
	"fmt"
	"path/filepath"
	"strings"
)

func scopedPath(projectPath, requested string) (string, error) {
	if projectPath == "" {
		return "", fmt.Errorf("project path required")
	}
	if requested == "" {
		requested = "."
	}
	root, err := filepath.Abs(projectPath)
	if err != nil {
		return "", err
	}
	joined := requested
	if !filepath.IsAbs(joined) {
		joined = filepath.Join(root, requested)
	}
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes project: %s", requested)
	}
	return abs, nil
}

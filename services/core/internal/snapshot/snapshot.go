package snapshot

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func Track(projectPath, sessionID string) (string, error) {
	if err := gitExec(projectPath, "add", "-A"); err != nil {
		return "", fmt.Errorf("git add: %w", err)
	}
	msg := fmt.Sprintf("zero-snapshot: session %s", sessionID)
	if err := gitExec(projectPath, "commit", "--allow-empty", "-m", msg); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}
	hash, err := gitOutput(projectPath, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(hash), nil
}

func Revert(projectPath, hash string) error {
	if err := gitExec(projectPath, "reset", "--hard", hash); err != nil {
		return fmt.Errorf("git reset: %w", err)
	}
	return nil
}

func Diff(projectPath, fromHash, toHash string) (string, error) {
	output, err := gitOutput(projectPath, "diff", fromHash, toHash)
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return output, nil
}

func gitExec(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s", err, stderr.String())
	}
	return nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

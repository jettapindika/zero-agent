package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func pendingProjectPath() string {
	return filepath.Join(zeroDir(), "pending-project.txt")
}

func writePendingProject(absPath string) error {
	if err := ensureZeroDir(); err != nil {
		return err
	}
	tmp := pendingProjectPath() + ".tmp"
	if err := os.WriteFile(tmp, []byte(absPath+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, pendingProjectPath())
}

func resolveProjectDir(arg string) (string, error) {
	abs, err := filepath.Abs(arg)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", arg, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat %q: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", abs)
	}
	return abs, nil
}

func looksLikePath(arg string) bool {
	if arg == "" {
		return false
	}
	if arg == "." || arg == ".." {
		return true
	}
	if filepath.IsAbs(arg) {
		return true
	}
	if len(arg) >= 2 && (arg[:2] == "./" || arg[:2] == "..") {
		return true
	}
	if _, err := os.Stat(arg); err == nil {
		info, err2 := os.Stat(arg)
		return err2 == nil && info.IsDir()
	}
	return false
}

func launchDesktop() error {
	switch runtime.GOOS {
	case "darwin":
		if path := findMacApp(); path != "" {
			return exec.Command("open", path).Start()
		}
		return exec.Command("open", "-a", "Zero").Start()
	case "linux":
		for _, name := range []string{"zero-desktop", "Zero"} {
			if p, err := exec.LookPath(name); err == nil {
				return exec.Command(p).Start()
			}
		}
		return fmt.Errorf("Zero desktop binary not found on PATH (looked for zero-desktop, Zero)")
	case "windows":
		for _, name := range []string{"Zero.exe", "zero-desktop.exe"} {
			if p, err := exec.LookPath(name); err == nil {
				return exec.Command(p).Start()
			}
		}
		return fmt.Errorf("Zero desktop binary not found on PATH (looked for Zero.exe, zero-desktop.exe)")
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func findMacApp() string {
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), "Applications", "Zero.app"),
		"/Applications/Zero.app",
	}
	if devBundle := devMacBundle(); devBundle != "" {
		candidates = append(candidates, devBundle)
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}

func devMacBundle() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	repoRoot := findRepoRoot(filepath.Dir(exe))
	if repoRoot == "" {
		return ""
	}
	bundle := filepath.Join(repoRoot, "apps", "desktop", "src-tauri", "target", "release", "bundle", "macos", "Zero.app")
	if info, err := os.Stat(bundle); err == nil && info.IsDir() {
		return bundle
	}
	debugBundle := filepath.Join(repoRoot, "apps", "desktop", "src-tauri", "target", "debug", "bundle", "macos", "Zero.app")
	if info, err := os.Stat(debugBundle); err == nil && info.IsDir() {
		return debugBundle
	}
	return ""
}

func findRepoRoot(start string) string {
	dir := start
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestLoadEmptyProjectReturnsNoSkills(t *testing.T) {
	dir := t.TempDir()
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

func TestLoadFindsSkillFromConventionalLocation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "skills", "tdd", "SKILL.md"), `---
name: tdd
description: Always test-drive new code
---
# TDD
Write the failing test first.
`)
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(got))
	}
	if got[0].Name != "tdd" {
		t.Fatalf("name = %q", got[0].Name)
	}
	if got[0].Description != "Always test-drive new code" {
		t.Fatalf("description = %q", got[0].Description)
	}
	if !strings.Contains(got[0].Body, "failing test") {
		t.Fatalf("body missing expected text: %q", got[0].Body)
	}
}

func TestLoadSkipsNodeModulesAndGit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "node_modules", "pkg", "SKILL.md"), "noise")
	writeFile(t, filepath.Join(dir, ".git", "SKILL.md"), "noise")
	writeFile(t, filepath.Join(dir, "skills", "real", "SKILL.md"), "real")
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d (%+v)", len(got), got)
	}
}

func TestFormatPromptSectionEmpty(t *testing.T) {
	if got := FormatPromptSection(nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestFormatPromptSectionRenders(t *testing.T) {
	skills := []Skill{{Name: "tdd", Description: "Test-drive code", Body: "Write a failing test first."}}
	got := FormatPromptSection(skills)
	for _, want := range []string{"## Installed Skills", "### tdd", "Test-drive code", "failing test"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestFallbacksFillNameAndDescription(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "skills", "no-front-matter", "SKILL.md"), `# Heading

This is the first descriptive line.
`)
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if got[0].Name != "no-front-matter" {
		t.Fatalf("name = %q", got[0].Name)
	}
	if got[0].Description != "This is the first descriptive line." {
		t.Fatalf("description = %q", got[0].Description)
	}
}

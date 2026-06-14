package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPromptContextFillsRequiredSections(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	pc := buildPromptContext(dir, now)
	got := pc.String()

	for _, want := range []string{
		"## Zero Runtime Context",
		"Current date: 2026-01-15",
		"Working directory: " + dir,
		"Platform: ",
		"## Tool Aliases",
		"read_file(path) -> use read",
		"run_command(cmd) -> use bash",
		"## Output Formatting",
		"==important==",
		"[color=red]text[/color]",
		"[color=green]text[/color]",
		"## Local AGENTS.md",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt context missing %q in:\n%s", want, got)
		}
	}
}

func TestPromptContextDoesNotLeakRawTemplatePlaceholders(t *testing.T) {
	dir := t.TempDir()
	pc := buildPromptContext(dir, time.Now())
	rendered := pc.String()

	for _, leak := range []string{"{{", "}}", "{project_path}", "{user_prompt}"} {
		if strings.Contains(rendered, leak) {
			t.Fatalf("prompt context leaked placeholder %q", leak)
		}
	}
}

func TestPromptContextIncludesAgentsMDWhenPresent(t *testing.T) {
	dir := t.TempDir()
	body := "# Project Rules\n\n- Always run tests before committing.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	pc := buildPromptContext(dir, time.Now())
	rendered := pc.String()
	if !strings.Contains(rendered, "Always run tests before committing") {
		t.Fatalf("expected AGENTS.md content in prompt, got:\n%s", rendered)
	}
}

func TestPromptContextHandlesMissingAgentsMD(t *testing.T) {
	dir := t.TempDir()
	pc := buildPromptContext(dir, time.Now())
	rendered := pc.String()
	if !strings.Contains(rendered, "(none found)") {
		t.Fatalf("expected '(none found)' marker for missing AGENTS.md")
	}
}

func TestPromptContextLargeAgentsMDIsBoundedSafely(t *testing.T) {
	dir := t.TempDir()
	// Make a file larger than the 32KB cap.
	huge := strings.Repeat("x", maxAgentsMDBytes+10)
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(huge), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	pc := buildPromptContext(dir, time.Now())
	rendered := pc.String()
	if !strings.Contains(rendered, "too large to inject safely") {
		t.Fatalf("expected size guard message, got:\n%s", rendered[:min(400, len(rendered))])
	}
}

func TestPromptContextDetectsGoStack(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	pc := buildPromptContext(dir, time.Now())
	if !strings.Contains(pc.TechStack, "Go module") {
		t.Fatalf("expected Go module detection, got %q", pc.TechStack)
	}
}


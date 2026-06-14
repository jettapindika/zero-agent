package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/zero-agent/core/internal/skills"
)

const maxAgentsMDBytes = 32 * 1024

type PromptContext struct {
	CurrentDate string
	CWD         string
	OS          string
	Shell       string
	TechStack   string
	AgentsMD    string
	Skills      []skills.Skill
}

func buildPromptContext(projectPath string, now time.Time) PromptContext {
	loaded, _ := skills.Load(projectPath) // best-effort; ignore walk errors
	return PromptContext{
		CurrentDate: now.Format("2006-01-02"),
		CWD:         projectPath,
		OS:          runtime.GOOS,
		Shell:       detectedShell(),
		TechStack:   detectTechStack(projectPath),
		AgentsMD:    readProjectAgentsMD(projectPath),
		Skills:      loaded,
	}
}

func (pc PromptContext) String() string {
	agents := strings.TrimSpace(pc.AgentsMD)
	if agents == "" {
		agents = "(none found)"
	}
	skillsSection := skills.FormatPromptSection(pc.Skills)
	if skillsSection != "" {
		skillsSection = "\n\n" + skillsSection
	}
	return fmt.Sprintf(`## Zero Runtime Context
- Current date: %s
- Working directory: %s
- Platform: %s / %s
- Project context: %s

## Tool Aliases
- read_file(path) -> use read
- write_file(path, content) -> use write
- run_command(cmd) -> use bash
- list_files(path) -> use ls
- search_codebase(query) -> use grep, glob, or walk

## Output Formatting
- ==important== or [highlight]important[/highlight] -> yellow highlight; one phrase per section, not whole paragraphs.
- [color=red]text[/color] for failures, deletions, errors, breaking changes.
- [color=green]text[/color] for successes, additions, verified results.
- [color=yellow]text[/color] for warnings and risks.
- [color=blue]text[/color] for paths, identifiers, informational notes.
- [color=purple]text[/color] for configuration/env-specific values.
- [color=gray]text[/color] for secondary context.
- Do not nest color tags. Do not put color tags inside fenced code blocks.

## Local AGENTS.md
%s%s`, pc.CurrentDate, pc.CWD, pc.OS, pc.Shell, pc.TechStack, agents, skillsSection)
}

func detectedShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return filepath.Base(shell)
	}
	if shell := os.Getenv("ComSpec"); shell != "" {
		return filepath.Base(shell)
	}
	return "unknown"
}

func detectTechStack(projectPath string) string {
	checks := []struct {
		path  string
		label string
	}{
		{"go.work", "Go workspace"},
		{"go.mod", "Go module"},
		{"package.json", "Node/JavaScript"},
		{"pnpm-workspace.yaml", "pnpm workspace"},
		{"apps/desktop/src-tauri/tauri.conf.json", "Tauri desktop"},
		{"apps/desktop/package.json", "Vite React desktop"},
		{"services/core/pkg/server/server.go", "Go chi HTTP server"},
		{"services/core/internal/storage", "SQLite storage"},
		{"apps/cli", "Go CLI/TUI"},
	}

	labels := []string{}
	for _, check := range checks {
		if _, err := os.Stat(filepath.Join(projectPath, check.path)); err == nil {
			labels = append(labels, check.label)
		}
	}
	if len(labels) == 0 {
		return "unknown"
	}
	return strings.Join(labels, ", ")
}

func readProjectAgentsMD(projectPath string) string {
	path := filepath.Join(projectPath, "AGENTS.md")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	if info.Size() > maxAgentsMDBytes {
		return fmt.Sprintf("AGENTS.md exists but is too large to inject safely (%d bytes)", info.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

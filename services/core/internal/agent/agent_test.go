package agent_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/zero-agent/core/internal/agent"
)

func TestRegistryStoresAndRetrievesAgents(t *testing.T) {
	reg := agent.NewRegistry()
	agents := agent.DefaultAgents("cx/gpt-5.5")
	for _, a := range agents {
		reg.Register(a)
	}

	build := reg.Get("build")
	if build == nil || build.Name != "build" {
		t.Fatalf("expected build agent, got %v", build)
	}
	if build.ReadOnly {
		t.Fatalf("build should not be read-only")
	}

	plan := reg.Get("plan")
	if plan == nil || !plan.ReadOnly {
		t.Fatalf("plan should be read-only, got %v", plan)
	}

	explore := reg.Get("explore")
	if explore == nil || !explore.ReadOnly {
		t.Fatalf("explore should be read-only, got %v", explore)
	}

	if len(reg.List()) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(reg.List()))
	}
}

func TestDefaultBuildAgentUsesTerminalLocalFirstSystemPrompt(t *testing.T) {
	build := agent.DefaultAgents("cx/gpt-5.5")[0]

	for _, want := range []string{
		"You are Zero, an expert local-first software engineer embedded in a CLI and desktop coding tool.",
		"Dangerous tools: bash, write, edit, fetch; the runtime gates these.",
		`Do NOT ask the user "Proceed?"`,
		"Project: {project_path}",
		"Session goal: {user_prompt}",
		"[color=red]text[/color]",
		"[color=green]text[/color]",
		"Numbered steps",
		"Action lines",
		"Bullet items",
		"Reasoning prose",
		"Phase headers",
		"Background task log",
		"→ Read",
		"[task]",
	} {
		if !strings.Contains(build.SystemPrompt, want) {
			t.Fatalf("build prompt missing %q", want)
		}
	}
}

func TestDefaultAgentsCanWalkFolders(t *testing.T) {
	for _, a := range agent.DefaultAgents("cx/gpt-5.5") {
		if !slices.Contains(a.AllowedTools, "walk") {
			t.Fatalf("agent %s missing walk tool: %#v", a.Name, a.AllowedTools)
		}
	}
}

func TestDefaultAgentsUsesProvidedModel(t *testing.T) {
	agents := agent.DefaultAgents("kr/claude-sonnet-4.5")
	for _, a := range agents {
		if a.Model != "kr/claude-sonnet-4.5" {
			t.Fatalf("agent %s model = %q", a.Name, a.Model)
		}
	}
}

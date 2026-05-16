package agent_test

import (
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

func TestDefaultAgentsUsesProvidedModel(t *testing.T) {
	agents := agent.DefaultAgents("kr/claude-sonnet-4.5")
	for _, a := range agents {
		if a.Model != "kr/claude-sonnet-4.5" {
			t.Fatalf("agent %s model = %q", a.Name, a.Model)
		}
	}
}

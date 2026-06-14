package agent_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/zero-agent/core/internal/agent"
	"github.com/zero-agent/core/internal/bus"
	"github.com/zero-agent/core/internal/permission"
	"github.com/zero-agent/core/internal/tool"
)

func TestToolExecutorRunsSafeTool(t *testing.T) {
	tools := tool.DefaultRegistry()
	eventBus := bus.New()
	perms := permission.NewManager(eventBus)
	executor := agent.NewToolExecutor(tools, perms, eventBus)

	_, events := eventBus.Subscribe("", "s1", 10)

	tmpDir := t.TempDir()
	args, _ := json.Marshal(map[string]string{"path": "."})
	result, err := executor.Execute(context.Background(), tmpDir, "p1", "s1", "m1", "ls", args)
	if err != nil {
		t.Fatalf("execute ls: %v", err)
	}
	if result.IsError {
		t.Fatalf("ls returned error: %s", result.Output)
	}

	seenStarted := false
	seenCompleted := false
	for len(events) > 0 {
		event := <-events
		if event.Type == "tool.started" {
			seenStarted = true
		}
		if event.Type == "tool.completed" {
			seenCompleted = true
		}
	}
	if !seenStarted || !seenCompleted {
		t.Fatalf("missing events: started=%v completed=%v", seenStarted, seenCompleted)
	}
}

func TestExecutorToolReasoningInjectionDocumentsPermissionChain(t *testing.T) {
	for _, want := range []string{
		"STEP 1 — GOAL",
		"STEP 4 — PERMISSION: Does this require user approval? (bash/write/edit/fetch = YES)",
		"RESULT: [What was returned]",
		"ANOMALY: [What's different from expectations]",
	} {
		if !strings.Contains(agent.ExecutorToolReasoningInjection, want) {
			t.Fatalf("executor injection missing %q", want)
		}
	}
}

func TestToolExecutorReturnsErrorForUnknownTool(t *testing.T) {
	tools := tool.DefaultRegistry()
	eventBus := bus.New()
	perms := permission.NewManager(eventBus)
	executor := agent.NewToolExecutor(tools, perms, eventBus)

	_, err := executor.Execute(context.Background(), "/tmp", "p1", "s1", "m1", "nonexistent", nil)
	if err == nil {
		t.Fatalf("expected error for unknown tool")
	}
}

func TestToolExecutorDeniesDangerousToolWhenPermissionDenied(t *testing.T) {
	tools := tool.DefaultRegistry()
	eventBus := bus.New()
	perms := permission.NewManager(eventBus)
	executor := agent.NewToolExecutor(tools, perms, eventBus)

	go func() {
		deadline := time.After(2 * time.Second)
		for {
			select {
			case <-deadline:
				return
			default:
			}
			pending := perms.ListPending("s1")
			if len(pending) > 0 {
				_ = perms.Resolve(pending[0].ID, permission.DecisionDeny)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	args, _ := json.Marshal(map[string]string{"command": "pwd"})
	result, err := executor.Execute(context.Background(), t.TempDir(), "p1", "s1", "m1", "bash", args)
	if err == nil {
		t.Fatal("expected permission denied error")
	}
	if !result.IsError || !strings.Contains(result.Output, "permission denied") {
		t.Fatalf("unexpected denial result: %#v", result)
	}
}

func TestToolExecutorExposesWalkSchema(t *testing.T) {
	tools := tool.DefaultRegistry()
	eventBus := bus.New()
	perms := permission.NewManager(eventBus)
	executor := agent.NewToolExecutor(tools, perms, eventBus)

	for _, schema := range executor.ToolSchemas() {
		fn, ok := schema["function"].(map[string]any)
		if !ok {
			continue
		}
		if fn["name"] == "walk" {
			return
		}
	}
	t.Fatalf("walk schema not exposed")
}

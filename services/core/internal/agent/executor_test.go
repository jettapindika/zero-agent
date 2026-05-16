package agent_test

import (
	"context"
	"encoding/json"
	"testing"

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

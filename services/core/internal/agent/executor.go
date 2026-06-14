package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zero-agent/core/internal/bus"
	"github.com/zero-agent/core/internal/permission"
	"github.com/zero-agent/core/internal/tool"
)

// ToolExecutor bridges the tool registry, permission manager, and event bus.
type ToolExecutor struct {
	tools       *tool.Registry
	permissions *permission.Manager
	bus         *bus.Bus
}

const ExecutorToolReasoningInjection = `Before calling any tool, follow this reasoning chain:

STEP 1 — GOAL: What specific outcome does this tool call produce?
STEP 2 — SCOPE: Which files/directories/commands are involved?
STEP 3 — RISK: Is this reversible? Could it break existing functionality?
STEP 4 — PERMISSION: Does this require user approval? (bash/write/edit/fetch = YES)
STEP 5 — ALTERNATIVES: Is there a safer read-only tool that can verify first?

Only after this reasoning, emit the tool call.

After the tool result:
RESULT: [What was returned]
IMPACT: [What this tells us / what changed]
NEXT: [Immediate next action OR task complete]

If the result is unexpected:
ANOMALY: [What's different from expectations]
HYPOTHESIS: [Why this might have happened]
RECOVERY: [Proposed next action]`

func NewToolExecutor(tools *tool.Registry, perms *permission.Manager, eventBus *bus.Bus) *ToolExecutor {
	return &ToolExecutor{tools: tools, permissions: perms, bus: eventBus}
}

// Execute runs a tool by name, checking permissions for dangerous tools.
func (te *ToolExecutor) Execute(ctx context.Context, projectPath, projectID, sessionID, messageID, toolName string, args json.RawMessage) (tool.Result, error) {
	t := te.tools.Get(toolName)
	if t == nil {
		return tool.Result{IsError: true, Output: "unknown tool: " + toolName}, fmt.Errorf("unknown tool: %s", toolName)
	}

	te.bus.Publish("tool.started", projectID, sessionID, map[string]string{"name": toolName, "args": string(args)})

	if t.NeedsPermission() {
		if te.permissions == nil {
			return tool.Result{IsError: true, Output: "permission manager is not configured"}, fmt.Errorf("permission manager is not configured")
		}
		argMap := map[string]any{}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &argMap); err != nil {
				return tool.Result{IsError: true, Output: "invalid tool args: " + err.Error()}, err
			}
		}
		decision, err := te.permissions.RequestPermission(ctx, sessionID, toolName, argMap)
		if err != nil {
			te.bus.Publish("tool.failed", projectID, sessionID, map[string]string{"name": toolName, "error": err.Error()})
			return tool.Result{IsError: true, Output: "permission request failed: " + err.Error()}, err
		}
		if decision == permission.DecisionDeny {
			err := fmt.Errorf("permission denied for tool: %s", toolName)
			te.bus.Publish("tool.failed", projectID, sessionID, map[string]string{"name": toolName, "error": err.Error()})
			return tool.Result{IsError: true, Output: err.Error()}, err
		}
	}

	tc := tool.Context{ProjectPath: projectPath, SessionID: sessionID, MessageID: messageID}
	result, err := t.Execute(ctx, args, tc)

	if err != nil {
		te.bus.Publish("tool.failed", projectID, sessionID, map[string]string{"name": toolName, "error": err.Error()})
	} else {
		te.bus.Publish("tool.completed", projectID, sessionID, map[string]string{"name": toolName, "result": result.Output})
	}

	return result, err
}

// ToolSchemas returns OpenAI-compatible function definitions for all registered tools.
func (te *ToolExecutor) ToolSchemas() []map[string]any {
	var schemas []map[string]any
	for _, name := range te.toolNames() {
		t := te.tools.Get(name)
		if t == nil {
			continue
		}
		schemas = append(schemas, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name(),
				"description": t.Description(),
				"parameters":  json.RawMessage(t.Schema()),
			},
		})
	}
	return schemas
}

func (te *ToolExecutor) toolNames() []string {
	names := []string{"read", "ls", "glob", "grep", "walk", "bash", "write", "edit", "fetch"}
	return names
}

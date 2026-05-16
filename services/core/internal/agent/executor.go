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
		te.bus.Publish("permission.required", projectID, sessionID, map[string]string{
			"tool":    toolName,
			"args":    string(args),
			"summary": fmt.Sprintf("%s %s", toolName, string(args)),
		})
		// For now, auto-approve in non-interactive mode.
		// Full interactive approval will come via TUI permission flow.
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
	names := []string{"read", "ls", "glob", "grep", "bash", "write", "edit", "fetch"}
	return names
}

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/zero-agent/core/internal/bus"
	"github.com/zero-agent/core/internal/provider"
	"github.com/zero-agent/core/internal/storage"
	"github.com/zero-agent/core/internal/tool"
)

// MaxAgentSteps caps the number of provider/tool round-trips per Run. This prevents
// infinite tool-call loops if the model keeps calling tools forever.
const MaxAgentSteps = 12

type Runner struct {
	db       *storage.DB
	bus      *bus.Bus
	provider provider.Provider
	executor *ToolExecutor
}

const complexTaskPlanningPromptTemplate = `The user has given you a complex task. Break it down before starting:

TASK ANALYSIS:
- Restate the goal in one sentence.
- Identify all files/modules likely involved.
- List subtasks in dependency order (what must happen before what).
- Estimate tool calls needed per subtask.
- Flag any subtask that requires dangerous tools (bash/write/edit).

OUTPUT FORMAT:
Plan:
  [1]  — tools:  — risk: low/med/high
  [2] ...
  [N] ...

Starting with [1]. Awaiting confirmation before dangerous steps.

After each subtask completes, reprint the plan with ✓ on done items and update the current step.

Original task: %s`

// NewRunner builds a Runner without tool execution. Used by tests and contexts
// that have not wired a permission manager.
func NewRunner(db *storage.DB, eventBus *bus.Bus, p provider.Provider) *Runner {
	return &Runner{db: db, bus: eventBus, provider: p}
}

// NewRunnerWithExecutor builds a Runner that can execute tool calls emitted by
// the model.
func NewRunnerWithExecutor(db *storage.DB, eventBus *bus.Bus, p provider.Provider, executor *ToolExecutor) *Runner {
	return &Runner{db: db, bus: eventBus, provider: p, executor: executor}
}

// SetExecutor attaches a ToolExecutor to an existing runner.
func (r *Runner) SetExecutor(executor *ToolExecutor) {
	r.executor = executor
}

func (r *Runner) Run(ctx context.Context, sessionID string) error {
	session, err := r.db.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	project, err := r.db.GetProject(ctx, session.ProjectID)
	if err != nil {
		return err
	}
	messages, err := r.db.ListMessages(ctx, sessionID)
	if err != nil {
		return err
	}

	providerMessages := make([]provider.Message, 0, len(messages)+1)
	userRequest := firstUserRequest(ctx, r.db, messages)
	if userRequest != "" && shouldAutoTitleSession(session.Title) {
		updated, err := r.db.UpdateSession(ctx, session.ID, titleFromPrompt(userRequest))
		if err != nil {
			return err
		}
		session = updated
		r.bus.Publish("session.updated", session.ProjectID, session.ID, session)
	}
	if userRequest != "" {
		providerMessages = append(providerMessages, provider.Message{Role: "system", Content: systemPromptForSession(session, project.Path, userRequest, time.Now())})
	}
	for _, message := range messages {
		parts, err := r.db.ListParts(ctx, message.ID)
		if err != nil {
			return err
		}
		var content strings.Builder
		for _, part := range parts {
			if part.Text != nil {
				content.WriteString(*part.Text)
			}
		}
		providerMessages = append(providerMessages, provider.Message{Role: message.Role, Content: content.String()})
	}

	var attachmentInfos []attachmentInfo
	for _, message := range messages {
		if message.Role != "system" {
			continue
		}
		parts, err := r.db.ListParts(ctx, message.ID)
		if err != nil {
			continue
		}
		for _, part := range parts {
			if part.Text == nil || !isAttachmentSystemMessage(*part.Text) {
				continue
			}
			ids := extractAttachmentIDs(*part.Text)
			if len(ids) == 0 {
				continue
			}
			results, infos, err := autoReadAttachments(ctx, r.db, ids)
			if err != nil {
				continue
			}
			attachmentInfos = append(attachmentInfos, infos...)

			for i, result := range results {
				toolCallID := fmt.Sprintf("auto-read-%s", ids[i])
				providerMessages = append(providerMessages, provider.Message{
					Role:       "tool",
					Content:    result.Output,
					ToolCallID: toolCallID,
				})
			}
		}
	}

	if len(attachmentInfos) > 0 {
		ackMsg, err := r.db.CreateMessage(ctx, sessionID, "assistant")
		if err == nil {
			var ackText strings.Builder
			for i, info := range attachmentInfos {
				if i > 0 {
					ackText.WriteString("\n")
				}
				ackText.WriteString(buildAckMessage(info))
			}
			text := ackText.String()
			_, err := r.db.CreatePart(ctx, storage.CreatePartInput{
				MessageID: ackMsg.ID,
				Type:      "text",
				OrderNum:  0,
				Text:      &text,
			})
			if err == nil {
				r.bus.Publish("message.created", session.ProjectID, session.ID, map[string]any{
					"message": ackMsg,
					"parts":   []storage.Part{{Text: &text}},
				})
			}
		}
	}

	tools := r.providerTools()

	for step := 0; step < MaxAgentSteps; step++ {
		r.bus.Publish("tool.thinking", session.ProjectID, session.ID, map[string]any{
			"step":  step + 1,
			"total": MaxAgentSteps,
		})
		req := provider.ChatRequest{Model: session.Model, Messages: providerMessages, Tools: tools}
		stream, err := r.provider.StreamChat(ctx, req)
		if err != nil {
			return err
		}

		assistantMessage, err := r.db.CreateMessage(ctx, session.ID, "assistant")
		if err != nil {
			return err
		}
		r.bus.Publish("message.created", session.ProjectID, session.ID, map[string]any{"message": assistantMessage, "parts": []storage.Part{}})

		var (
			text      strings.Builder
			toolCalls []provider.ToolCall
		)
		for event := range stream {
			if event.Err != nil {
				return event.Err
			}
			if event.Delta != "" {
				text.WriteString(event.Delta)
				r.bus.Publish("part.delta", session.ProjectID, session.ID, map[string]string{"messageId": assistantMessage.ID, "delta": event.Delta})
			}
			if len(event.ToolCalls) > 0 {
				toolCalls = append(toolCalls, event.ToolCalls...)
			}
		}

		finalText := text.String()
		var orderNum int64
		if finalText != "" {
			part, err := r.db.CreatePart(ctx, storage.CreatePartInput{MessageID: assistantMessage.ID, Type: "text", OrderNum: orderNum, Text: &finalText})
			if err != nil {
				return err
			}
			orderNum++
			r.bus.Publish("part.created", session.ProjectID, session.ID, part)
		}

		if len(toolCalls) == 0 {
			r.bus.Publish("session.status", session.ProjectID, session.ID, map[string]string{"status": "idle"})
			return nil
		}

		// Persist the assistant tool_call parts before executing so the UI can
		// render the call before its result arrives.
		assistantToolCalls := make([]provider.ToolCall, 0, len(toolCalls))
		for _, call := range toolCalls {
			args := string(call.Arguments)
			part, err := r.db.CreatePart(ctx, storage.CreatePartInput{
				MessageID:    assistantMessage.ID,
				Type:         "tool_call",
				OrderNum:     orderNum,
				ToolName:     ptr(call.Name),
				ToolCallID:   ptr(call.ID),
				ToolArgsJSON: ptr(args),
			})
			if err != nil {
				return err
			}
			orderNum++
			r.bus.Publish("part.created", session.ProjectID, session.ID, part)
			assistantToolCalls = append(assistantToolCalls, call)
		}

		// Append assistant message (with tool_calls) to provider history.
		providerMessages = append(providerMessages, provider.Message{
			Role:      "assistant",
			Content:   finalText,
			ToolCalls: assistantToolCalls,
		})

		if r.executor == nil {
			// No executor wired — record an error part for each unresolved call.
			for _, call := range toolCalls {
				errMsg := fmt.Sprintf("tool %s requested but no executor is configured", call.Name)
				_, err := r.db.CreatePart(ctx, storage.CreatePartInput{
					MessageID:      assistantMessage.ID,
					Type:           "tool_result",
					OrderNum:       orderNum,
					ToolName:       ptr(call.Name),
					ToolCallID:     ptr(call.ID),
					ToolResultJSON: ptr(errMsg),
					IsError:        true,
				})
				if err != nil {
					return err
				}
				orderNum++
			}
			r.bus.Publish("session.status", session.ProjectID, session.ID, map[string]string{"status": "idle"})
			return nil
		}

		// Execute each tool call, persist a tool_result part, and append a
		// tool-role message to provider history for the next loop step.
		for _, call := range toolCalls {
			result, execErr := r.executor.Execute(
				ctx,
				project.Path,
				project.ID,
				session.ID,
				assistantMessage.ID,
				call.Name,
				call.Arguments,
			)
			resultJSON := result.Output
			if encoded, err := json.Marshal(result); err == nil {
				resultJSON = string(encoded)
			}
			isErr := result.IsError || execErr != nil
			part, err := r.db.CreatePart(ctx, storage.CreatePartInput{
				MessageID:      assistantMessage.ID,
				Type:           "tool_result",
				OrderNum:       orderNum,
				ToolName:       ptr(call.Name),
				ToolCallID:     ptr(call.ID),
				ToolResultJSON: ptr(resultJSON),
				IsError:        isErr,
			})
			if err != nil {
				return err
			}
			orderNum++
			r.bus.Publish("part.created", session.ProjectID, session.ID, part)

			providerMessages = append(providerMessages, provider.Message{
				Role:       "tool",
				Content:    summarizeToolResult(result, execErr),
				ToolCallID: call.ID,
				Name:       call.Name,
			})
		}
		// loop continues; next step lets the model react to tool results.
	}

	// Hit the step ceiling — record a soft error and return.
	stepCap := fmt.Sprintf("Agent stopped after %d tool-call rounds.", MaxAgentSteps)
	stepCapPart, err := r.db.CreatePart(ctx, storage.CreatePartInput{
		MessageID: lastAssistantMessageID(ctx, r.db, sessionID),
		Type:      "text",
		OrderNum:  9999,
		Text:      &stepCap,
		IsError:   true,
	})
	if err == nil && stepCapPart != nil {
		r.bus.Publish("part.created", session.ProjectID, session.ID, stepCapPart)
	}
	r.bus.Publish("session.status", session.ProjectID, session.ID, map[string]string{"status": "idle"})
	return nil
}

func (r *Runner) providerTools() []provider.Tool {
	if r.executor == nil {
		return nil
	}
	schemas := r.executor.ToolSchemas()
	tools := make([]provider.Tool, 0, len(schemas))
	for _, schema := range schemas {
		fn, ok := schema["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := fn["name"].(string)
		desc, _ := fn["description"].(string)
		params, _ := fn["parameters"].(json.RawMessage)
		tools = append(tools, provider.Tool{Name: name, Description: desc, Parameters: params})
	}
	return tools
}

func summarizeToolResult(result tool.Result, err error) string {
	if err != nil {
		return fmt.Sprintf("tool error: %s", err.Error())
	}
	if result.Output == "" {
		if result.IsError {
			return "tool reported error with no output"
		}
		return "tool produced no output"
	}
	return result.Output
}

func ptr[T any](v T) *T { return &v }

func lastAssistantMessageID(ctx context.Context, db *storage.DB, sessionID string) string {
	messages, err := db.ListMessages(ctx, sessionID)
	if err != nil {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			return messages[i].ID
		}
	}
	return ""
}

func systemPromptForSession(session *storage.Session, projectPath string, userRequest string, now time.Time) string {
	agentPrompt := terminalLocalFirstSystemPrompt
	for _, a := range DefaultAgents(session.Model) {
		if a.Name == session.Agent {
			agentPrompt = a.SystemPrompt
			break
		}
	}
	replacements := map[string]string{
		"{project_path}": projectPath,
		"{agent_mode}":   session.Agent,
		"{model}":        session.Model,
		"{session_id}":   session.ID,
		"{user_prompt}":  userRequest,
	}
	for old, replacement := range replacements {
		agentPrompt = strings.ReplaceAll(agentPrompt, old, replacement)
	}
	return strings.Join([]string{
		agentPrompt,
		buildPromptContext(projectPath, now).String(),
		ExecutorToolReasoningInjection,
		fmt.Sprintf(complexTaskPlanningPromptTemplate, userRequest),
	}, "\n\n")
}

func firstUserRequest(ctx context.Context, db *storage.DB, messages []storage.Message) string {
	for _, message := range messages {
		if message.Role != "user" {
			continue
		}
		parts, err := db.ListParts(ctx, message.ID)
		if err != nil {
			return ""
		}
		var content strings.Builder
		for _, part := range parts {
			if part.Text != nil {
				content.WriteString(*part.Text)
			}
		}
		return strings.TrimSpace(content.String())
	}
	return ""
}

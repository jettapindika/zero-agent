package provider

import (
	"context"
	"encoding/json"
)

type Provider interface {
	Name() string
	ListModels(ctx context.Context) ([]ModelInfo, error)
	StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error)
	GenerateText(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

type ModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Message is an OpenAI-compatible chat message. ToolCalls are populated when the
// assistant emits tool_calls; ToolCallID is set on tool-result messages.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"toolCalls,omitempty"`
	ToolCallID string     `json:"toolCallId,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall represents a model-issued function call.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Tool describes a function the model may call.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ChatRequest struct {
	Model      string    `json:"model"`
	Messages   []Message `json:"messages"`
	Tools      []Tool    `json:"tools,omitempty"`
	ToolChoice string    `json:"toolChoice,omitempty"`
}

// ChatEvent is one streamed chunk. Either Delta is non-empty (text), ToolCalls is
// non-empty (final aggregated tool calls for this turn), or Err is set. When
// Done is true it marks the end of the assistant turn.
type ChatEvent struct {
	Delta     string
	ToolCalls []ToolCall
	Done      bool
	Err       error
}

type ChatResponse struct {
	Text      string
	ToolCalls []ToolCall
}

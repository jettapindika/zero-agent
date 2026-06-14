package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type OpenAIConfig struct {
	BaseURL string
	APIKey  string
}

type OpenAI struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewOpenAI(cfg OpenAIConfig) *OpenAI {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAI{baseURL: baseURL, apiKey: cfg.APIKey, client: http.DefaultClient}
}

func (p *OpenAI) Name() string {
	return "openai"
}

func (p *OpenAI) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Hit the user-configured /models endpoint live so the desktop's Model
	// Picker reflects whatever the upstream provider actually serves.
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	p.applyHeaders(httpReq)
	res, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("openai list models failed: %s", res.Status)
	}
	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]ModelInfo, 0, len(payload.Data))
	for _, m := range payload.Data {
		if m.ID == "" {
			continue
		}
		name := m.ID
		if _, after, ok := strings.Cut(m.ID, "/"); ok {
			name = after
		}
		out = append(out, ModelInfo{ID: m.ID, Name: name})
	}
	return out, nil
}

// buildRequestBody assembles an OpenAI-compatible chat completion request body.
// Exposed (lowercase) for tests via the same package.
func buildRequestBody(req ChatRequest, stream bool) ([]byte, error) {
	payload := map[string]any{
		"model":    req.Model,
		"messages": toOpenAIMessages(req.Messages),
		"stream":   stream,
	}
	if len(req.Tools) > 0 {
		payload["tools"] = toOpenAITools(req.Tools)
		if req.ToolChoice != "" {
			payload["tool_choice"] = req.ToolChoice
		} else {
			payload["tool_choice"] = "auto"
		}
	}
	return json.Marshal(payload)
}

func toOpenAIMessages(messages []Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		entry := map[string]any{"role": m.Role, "content": m.Content}
		if m.Name != "" {
			entry["name"] = m.Name
		}
		if m.ToolCallID != "" {
			entry["tool_call_id"] = m.ToolCallID
		}
		if len(m.ToolCalls) > 0 {
			calls := make([]map[string]any, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				args := string(tc.Arguments)
				if args == "" {
					args = "{}"
				}
				calls = append(calls, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": args,
					},
				})
			}
			entry["tool_calls"] = calls
		}
		out = append(out, entry)
	}
	return out
}

func toOpenAITools(tools []Tool) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		params := json.RawMessage(t.Parameters)
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  params,
			},
		})
	}
	return out
}

func (p *OpenAI) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	body, err := buildRequestBody(req, true)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.applyHeaders(httpReq)

	res, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, fmt.Errorf("openai stream failed: %s", res.Status)
	}

	ch := make(chan ChatEvent, 16)
	go func() {
		defer close(ch)
		defer res.Body.Close()
		acc := newToolCallAccumulator()
		scanner := bufio.NewScanner(res.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				if calls := acc.finalize(); len(calls) > 0 {
					ch <- ChatEvent{ToolCalls: calls}
				}
				ch <- ChatEvent{Done: true}
				return
			}
			delta, err := parseStreamDelta([]byte(data), acc)
			if err != nil {
				ch <- ChatEvent{Err: err}
				return
			}
			if delta != "" {
				ch <- ChatEvent{Delta: delta}
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- ChatEvent{Err: err}
			return
		}
		if calls := acc.finalize(); len(calls) > 0 {
			ch <- ChatEvent{ToolCalls: calls}
		}
		ch <- ChatEvent{Done: true}
	}()
	return ch, nil
}

func (p *OpenAI) GenerateText(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body, err := buildRequestBody(req, false)
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, err
	}
	p.applyHeaders(httpReq)

	res, err := p.client.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ChatResponse{}, fmt.Errorf("openai generate failed: %s", res.Status)
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return ChatResponse{}, err
	}
	if len(payload.Choices) == 0 {
		return ChatResponse{}, nil
	}
	choice := payload.Choices[0].Message
	resp := ChatResponse{Text: choice.Content}
	for _, tc := range choice.ToolCalls {
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}
	return resp, nil
}

func (p *OpenAI) applyHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
}

// toolCallAccumulator collects partial tool_call deltas across SSE chunks and
// produces a final []ToolCall once the stream finishes. OpenAI streams send
// each tool call as fragments keyed by index, with arguments arriving as a
// concatenated JSON string.
type toolCallAccumulator struct {
	byIndex map[int]*ToolCall
	order   []int
	args    map[int]*strings.Builder
}

func newToolCallAccumulator() *toolCallAccumulator {
	return &toolCallAccumulator{
		byIndex: map[int]*ToolCall{},
		args:    map[int]*strings.Builder{},
	}
}

func (a *toolCallAccumulator) ingest(idx int, id, name, argsFragment string) {
	tc, ok := a.byIndex[idx]
	if !ok {
		tc = &ToolCall{}
		a.byIndex[idx] = tc
		a.args[idx] = &strings.Builder{}
		a.order = append(a.order, idx)
	}
	if id != "" {
		tc.ID = id
	}
	if name != "" {
		tc.Name = name
	}
	if argsFragment != "" {
		a.args[idx].WriteString(argsFragment)
	}
}

func (a *toolCallAccumulator) finalize() []ToolCall {
	if len(a.byIndex) == 0 {
		return nil
	}
	calls := make([]ToolCall, 0, len(a.order))
	for _, idx := range a.order {
		tc := a.byIndex[idx]
		argString := a.args[idx].String()
		if argString == "" {
			argString = "{}"
		}
		// Validate args parse as JSON; if not, wrap as raw string.
		raw := json.RawMessage(argString)
		var probe any
		if err := json.Unmarshal(raw, &probe); err != nil {
			raw = json.RawMessage(`{}`)
		}
		calls = append(calls, ToolCall{ID: tc.ID, Name: tc.Name, Arguments: raw})
	}
	return calls
}

func parseStreamDelta(data []byte, acc *toolCallAccumulator) (string, error) {
	var payload struct {
		Choices []struct {
			Delta struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Index    int    `json:"index"`
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	if len(payload.Choices) == 0 {
		return "", nil
	}
	delta := payload.Choices[0].Delta
	if acc != nil {
		for _, tc := range delta.ToolCalls {
			acc.ingest(tc.Index, tc.ID, tc.Function.Name, tc.Function.Arguments)
		}
	}
	return delta.Content, nil
}

func trimProvider(model string) string {
	_, after, ok := strings.Cut(model, "/")
	if ok {
		return after
	}
	return model
}

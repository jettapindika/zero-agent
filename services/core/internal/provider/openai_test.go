package provider_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zero-agent/core/internal/provider"
)

func TestOpenAIProviderStreamsTextDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertRequestedModel(t, r, "kr/claude-sonnet-4.5")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := provider.NewOpenAI(provider.OpenAIConfig{BaseURL: server.URL + "/v1", APIKey: "test"})
	stream, err := p.StreamChat(context.Background(), provider.ChatRequest{
		Model:    "kr/claude-sonnet-4.5",
		Messages: []provider.Message{{Role: "user", Content: "Say hello"}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	var got string
	for event := range stream {
		if event.Err != nil {
			t.Fatalf("event error: %v", event.Err)
		}
		got += event.Delta
	}

	if got != "Hello" {
		t.Fatalf("expected Hello, got %q", got)
	}
}

func TestOpenAIProviderGenerateText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestedModel(t, r, "cx/gpt-5.5")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"done"}}]}`)
	}))
	defer server.Close()

	p := provider.NewOpenAI(provider.OpenAIConfig{BaseURL: server.URL + "/v1", APIKey: "test"})
	res, err := p.GenerateText(context.Background(), provider.ChatRequest{
		Model:    "cx/gpt-5.5",
		Messages: []provider.Message{{Role: "user", Content: "Work"}},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if res.Text != "done" {
		t.Fatalf("expected done, got %q", res.Text)
	}
}

func TestOpenAIProviderSendsToolsInRequestBody(t *testing.T) {
	var captured []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := provider.NewOpenAI(provider.OpenAIConfig{BaseURL: server.URL + "/v1", APIKey: "test"})
	stream, err := p.StreamChat(context.Background(), provider.ChatRequest{
		Model:    "cx/gpt-5.5",
		Messages: []provider.Message{{Role: "user", Content: "list files"}},
		Tools: []provider.Tool{{
			Name:        "ls",
			Description: "List directory entries",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	for range stream {
	}

	var payload struct {
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Parameters  json.RawMessage `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
		ToolChoice string `json:"tool_choice"`
	}
	if err := json.Unmarshal(captured, &payload); err != nil {
		t.Fatalf("decode: %v\nbody=%s", err, string(captured))
	}
	if len(payload.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d (body=%s)", len(payload.Tools), string(captured))
	}
	if payload.Tools[0].Function.Name != "ls" {
		t.Fatalf("tool name = %q", payload.Tools[0].Function.Name)
	}
	if payload.Tools[0].Type != "function" {
		t.Fatalf("tool type = %q", payload.Tools[0].Type)
	}
	if payload.ToolChoice != "auto" {
		t.Fatalf("expected default tool_choice=auto, got %q", payload.ToolChoice)
	}
}

func TestOpenAIProviderParsesStreamedToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Standard OpenAI streaming for tool calls: id+name come first, args
		// arrive as fragments under the same index.
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_abc\",\"function\":{\"name\":\"ls\",\"arguments\":\"\"}}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"path\\\":\"}}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\".\\\"}\"}}]}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := provider.NewOpenAI(provider.OpenAIConfig{BaseURL: server.URL + "/v1", APIKey: "test"})
	stream, err := p.StreamChat(context.Background(), provider.ChatRequest{
		Model:    "cx/gpt-5.5",
		Messages: []provider.Message{{Role: "user", Content: "ls"}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	var aggregatedCalls []provider.ToolCall
	sawDone := false
	for ev := range stream {
		if ev.Err != nil {
			t.Fatalf("event err: %v", ev.Err)
		}
		if len(ev.ToolCalls) > 0 {
			aggregatedCalls = append(aggregatedCalls, ev.ToolCalls...)
		}
		if ev.Done {
			sawDone = true
		}
	}
	if !sawDone {
		t.Fatal("expected Done event")
	}
	if len(aggregatedCalls) != 1 {
		t.Fatalf("expected 1 aggregated tool call, got %d", len(aggregatedCalls))
	}
	call := aggregatedCalls[0]
	if call.ID != "call_abc" || call.Name != "ls" {
		t.Fatalf("call meta = %+v", call)
	}
	var args map[string]string
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		t.Fatalf("args not valid JSON: %v (raw=%s)", err, string(call.Arguments))
	}
	if args["path"] != "." {
		t.Fatalf("args = %+v", args)
	}
}

func assertRequestedModel(t *testing.T, r *http.Request, want string) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload.Model != want {
		t.Fatalf("model = %q, want %q", payload.Model, want)
	}
}

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

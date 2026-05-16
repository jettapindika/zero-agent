package provider_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zero-agent/core/internal/provider"
)

func TestAnthropicProviderStreamsTextDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Fatal("missing anthropic-version header")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hel\"}}\n\n")
		fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"lo\"}}\n\n")
		fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	}))
	defer server.Close()

	p := provider.NewAnthropic(provider.AnthropicConfig{BaseURL: server.URL, APIKey: "test-key"})
	stream, err := p.StreamChat(context.Background(), provider.ChatRequest{
		Model:    "anthropic/claude-sonnet-4-5",
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

func TestAnthropicProviderGenerateText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"content":[{"type":"text","text":"done"}]}`)
	}))
	defer server.Close()

	p := provider.NewAnthropic(provider.AnthropicConfig{BaseURL: server.URL, APIKey: "test-key"})
	res, err := p.GenerateText(context.Background(), provider.ChatRequest{
		Model:    "anthropic/claude-sonnet-4-5",
		Messages: []provider.Message{{Role: "user", Content: "Work"}},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if res.Text != "done" {
		t.Fatalf("expected done, got %q", res.Text)
	}
}

func TestRegistryForModel(t *testing.T) {
	reg := provider.NewRegistry()
	openai := provider.NewOpenAI(provider.OpenAIConfig{APIKey: "k"})
	anthropic := provider.NewAnthropic(provider.AnthropicConfig{APIKey: "k"})
	reg.Register(openai)
	reg.Register(anthropic)

	p, err := reg.ForModel("anthropic/claude-sonnet-4-5")
	if err != nil {
		t.Fatalf("for model: %v", err)
	}
	if p.Name() != "anthropic" {
		t.Fatalf("expected anthropic, got %s", p.Name())
	}

	p, err = reg.ForModel("openai/gpt-4o")
	if err != nil {
		t.Fatalf("for model: %v", err)
	}
	if p.Name() != "openai" {
		t.Fatalf("expected openai, got %s", p.Name())
	}

	_, err = reg.ForModel("unknown/model")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

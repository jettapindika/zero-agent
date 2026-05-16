package agent_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/zero-agent/core/internal/agent"
	"github.com/zero-agent/core/internal/bus"
	"github.com/zero-agent/core/internal/provider"
	"github.com/zero-agent/core/internal/storage"
)

type fakeProvider struct{}

func (fakeProvider) Name() string                                                 { return "fake" }
func (fakeProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) { return nil, nil }
func (fakeProvider) GenerateText(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{Text: "assistant response"}, nil
}
func (fakeProvider) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.ChatEvent, error) {
	ch := make(chan provider.ChatEvent, 2)
	ch <- provider.ChatEvent{Delta: "assistant "}
	ch <- provider.ChatEvent{Delta: "response"}
	close(ch)
	return ch, nil
}

func TestRunnerPersistsAssistantMessageAndPublishesDeltas(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	project, err := db.GetOrCreateProject(context.Background(), "/tmp/project", "project")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	session, err := db.CreateSession(context.Background(), storage.CreateSessionInput{ProjectID: project.ID, Title: "test", Model: "fake/model", Agent: "build"})
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	_, err = db.CreateMessage(context.Background(), session.ID, "user")
	if err != nil {
		t.Fatalf("message: %v", err)
	}

	eventBus := bus.New()
	_, events := eventBus.Subscribe(project.ID, session.ID, 10)
	runner := agent.NewRunner(db, eventBus, fakeProvider{})

	if err := runner.Run(context.Background(), session.ID); err != nil {
		t.Fatalf("run: %v", err)
	}

	messages, err := db.ListMessages(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[1].Role != "assistant" {
		t.Fatalf("expected assistant message, got %s", messages[1].Role)
	}

	parts, err := db.ListParts(context.Background(), messages[1].ID)
	if err != nil {
		t.Fatalf("list parts: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 assistant part, got %d", len(parts))
	}
	if parts[0].Text == nil || *parts[0].Text != "assistant response" {
		t.Fatalf("unexpected text: %#v", parts[0].Text)
	}

	seenDelta := false
	var deltas []string
	for len(events) > 0 {
		event := <-events
		if event.Type == "part.delta" {
			seenDelta = true
			if payload, ok := event.Payload.(map[string]string); ok {
				deltas = append(deltas, payload["delta"])
				if payload["messageId"] == "" {
					t.Fatal("part.delta missing messageId")
				}
			}
		}
	}
	if !seenDelta {
		t.Fatal("expected part.delta event")
	}
	if len(deltas) != 2 || deltas[0] != "assistant " || deltas[1] != "response" {
		t.Fatalf("deltas = %#v", deltas)
	}
}

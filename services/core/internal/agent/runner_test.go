package agent_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/zero-agent/core/internal/agent"
	"github.com/zero-agent/core/internal/bus"
	"github.com/zero-agent/core/internal/permission"
	"github.com/zero-agent/core/internal/provider"
	"github.com/zero-agent/core/internal/storage"
	"github.com/zero-agent/core/internal/tool"
)

type fakeProvider struct{}

func (fakeProvider) Name() string                                                 { return "fake" }
func (fakeProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) { return nil, nil }
func (fakeProvider) GenerateText(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{Text: "assistant response"}, nil
}

type capturingProvider struct {
	request provider.ChatRequest
}

func (p *capturingProvider) Name() string { return "capture" }
func (p *capturingProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	return nil, nil
}
func (p *capturingProvider) GenerateText(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	p.request = req
	return provider.ChatResponse{Text: "assistant response"}, nil
}
func (p *capturingProvider) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.ChatEvent, error) {
	p.request = req
	ch := make(chan provider.ChatEvent, 1)
	ch <- provider.ChatEvent{Delta: "assistant response"}
	close(ch)
	return ch, nil
}

func TestRunnerPrependsComplexTaskPlanningPrompt(t *testing.T) {
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
	message, err := db.CreateMessage(context.Background(), session.ID, "user")
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	userPrompt := "modify prompts"
	_, err = db.CreatePart(context.Background(), storage.CreatePartInput{MessageID: message.ID, Type: "text", OrderNum: 0, Text: &userPrompt})
	if err != nil {
		t.Fatalf("part: %v", err)
	}

	p := &capturingProvider{}
	runner := agent.NewRunner(db, bus.New(), p)
	if err := runner.Run(context.Background(), session.ID); err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(p.request.Messages) == 0 {
		t.Fatal("expected provider messages")
	}
	first := p.request.Messages[0]
	if first.Role != "system" {
		t.Fatalf("first message role = %q, want system", first.Role)
	}
	for _, want := range []string{
		"You are Zero, an expert local-first software engineer embedded in a CLI and desktop coding tool.",
		"Project: /tmp/project",
		"## Zero Runtime Context",
		"Working directory: /tmp/project",
		"Tool Aliases",
		"Before calling any tool, follow this reasoning chain:",
		"The user has given you a complex task. Break it down before starting:",
		"TASK ANALYSIS:",
		"Original task: modify prompts",
	} {
		if !strings.Contains(first.Content, want) {
			t.Fatalf("runner prompt missing %q in %q", want, first.Content)
		}
	}
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

func TestRunnerAutoTitlesGenericSessionFromFirstPrompt(t *testing.T) {
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
	session, err := db.CreateSession(context.Background(), storage.CreateSessionInput{ProjectID: project.ID, Title: "Desktop session", Model: "fake/model", Agent: "build"})
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	message, err := db.CreateMessage(context.Background(), session.ID, "user")
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	text := "fix the auth bug in login flow"
	_, err = db.CreatePart(context.Background(), storage.CreatePartInput{MessageID: message.ID, Type: "text", OrderNum: 0, Text: &text})
	if err != nil {
		t.Fatalf("part: %v", err)
	}

	runner := agent.NewRunner(db, bus.New(), fakeProvider{})
	if err := runner.Run(context.Background(), session.ID); err != nil {
		t.Fatalf("run: %v", err)
	}

	updated, err := db.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updated.Title != "Fix The Auth Bug in Login Flow" {
		t.Fatalf("title = %q", updated.Title)
	}
}

func TestRunnerPreservesManualSessionTitle(t *testing.T) {
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
	session, err := db.CreateSession(context.Background(), storage.CreateSessionInput{ProjectID: project.ID, Title: "OAuth Refactor", Model: "fake/model", Agent: "build"})
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	message, err := db.CreateMessage(context.Background(), session.ID, "user")
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	text := "fix typo"
	_, err = db.CreatePart(context.Background(), storage.CreatePartInput{MessageID: message.ID, Type: "text", OrderNum: 0, Text: &text})
	if err != nil {
		t.Fatalf("part: %v", err)
	}

	runner := agent.NewRunner(db, bus.New(), fakeProvider{})
	if err := runner.Run(context.Background(), session.ID); err != nil {
		t.Fatalf("run: %v", err)
	}

	updated, err := db.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updated.Title != "OAuth Refactor" {
		t.Fatalf("title = %q", updated.Title)
	}
}

// scriptedProvider returns a queue of pre-built ChatEvent batches, one batch per
// StreamChat call. This lets us simulate multi-step tool-call conversations
// without needing a real network provider.
type scriptedProvider struct {
	mu       sync.Mutex
	batches  [][]provider.ChatEvent
	requests []provider.ChatRequest
}

func (p *scriptedProvider) Name() string                                                 { return "scripted" }
func (p *scriptedProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) { return nil, nil }
func (p *scriptedProvider) GenerateText(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}
func (p *scriptedProvider) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.ChatEvent, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	if len(p.batches) == 0 {
		ch := make(chan provider.ChatEvent)
		close(ch)
		return ch, nil
	}
	batch := p.batches[0]
	p.batches = p.batches[1:]
	ch := make(chan provider.ChatEvent, len(batch)+1)
	for _, ev := range batch {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func TestRunnerExecutesToolCallsAndAppendsResults(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	projectDir := t.TempDir()
	project, err := db.GetOrCreateProject(context.Background(), projectDir, "project")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	session, err := db.CreateSession(context.Background(), storage.CreateSessionInput{ProjectID: project.ID, Title: "ToolLoop", Model: "fake/model", Agent: "build"})
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	userMsg, err := db.CreateMessage(context.Background(), session.ID, "user")
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	userText := "list files"
	if _, err := db.CreatePart(context.Background(), storage.CreatePartInput{MessageID: userMsg.ID, Type: "text", OrderNum: 0, Text: &userText}); err != nil {
		t.Fatalf("part: %v", err)
	}

	args, _ := json.Marshal(map[string]string{"path": "."})
	scripted := &scriptedProvider{
		batches: [][]provider.ChatEvent{
			{
				{Delta: "I will list the directory.\n"},
				{ToolCalls: []provider.ToolCall{{ID: "call_1", Name: "ls", Arguments: args}}},
				{Done: true},
			},
			{
				{Delta: "Listing complete."},
				{Done: true},
			},
		},
	}

	eventBus := bus.New()
	perms := permission.NewManager(eventBus)
	executor := agent.NewToolExecutor(tool.DefaultRegistry(), perms, eventBus)
	runner := agent.NewRunnerWithExecutor(db, eventBus, scripted, executor)

	if err := runner.Run(context.Background(), session.ID); err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(scripted.requests) != 2 {
		t.Fatalf("expected 2 provider calls (initial + after-tool), got %d", len(scripted.requests))
	}

	// Second request must include the tool result message.
	second := scripted.requests[1]
	sawToolMessage := false
	for _, m := range second.Messages {
		if m.Role == "tool" && m.ToolCallID == "call_1" {
			sawToolMessage = true
			break
		}
	}
	if !sawToolMessage {
		t.Fatalf("second request missing tool-role message, got: %+v", second.Messages)
	}

	// Provider should have been told about tools the first time.
	if len(scripted.requests[0].Tools) == 0 {
		t.Fatalf("expected provider to receive tool schemas")
	}

	// Verify parts were persisted: tool_call + tool_result for the first assistant
	// message, plus a final text part on the second assistant message.
	messages, err := db.ListMessages(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) < 3 {
		t.Fatalf("expected user + 2 assistant messages, got %d", len(messages))
	}

	firstAssistantParts, err := db.ListParts(context.Background(), messages[1].ID)
	if err != nil {
		t.Fatalf("list parts: %v", err)
	}
	hasToolCall := false
	hasToolResult := false
	for _, p := range firstAssistantParts {
		if p.Type == "tool_call" && p.ToolName != nil && *p.ToolName == "ls" {
			hasToolCall = true
		}
		if p.Type == "tool_result" && p.ToolName != nil && *p.ToolName == "ls" {
			hasToolResult = true
		}
	}
	if !hasToolCall || !hasToolResult {
		t.Fatalf("missing tool parts: call=%v result=%v parts=%+v", hasToolCall, hasToolResult, firstAssistantParts)
	}
}

func TestRunnerStopsAtMaxAgentSteps(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	projectDir := t.TempDir()
	project, err := db.GetOrCreateProject(context.Background(), projectDir, "project")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	session, err := db.CreateSession(context.Background(), storage.CreateSessionInput{ProjectID: project.ID, Title: "InfiniteLoop", Model: "fake/model", Agent: "build"})
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	userMsg, err := db.CreateMessage(context.Background(), session.ID, "user")
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	userText := "loop forever"
	if _, err := db.CreatePart(context.Background(), storage.CreatePartInput{MessageID: userMsg.ID, Type: "text", OrderNum: 0, Text: &userText}); err != nil {
		t.Fatalf("part: %v", err)
	}

	args, _ := json.Marshal(map[string]string{"path": "."})
	// Build MaxAgentSteps batches that all emit a tool call so the loop never
	// terminates naturally.
	batches := make([][]provider.ChatEvent, 0, agent.MaxAgentSteps+2)
	for i := 0; i < agent.MaxAgentSteps+2; i++ {
		batches = append(batches, []provider.ChatEvent{
			{ToolCalls: []provider.ToolCall{{ID: "call_loop", Name: "ls", Arguments: args}}},
			{Done: true},
		})
	}
	scripted := &scriptedProvider{batches: batches}

	eventBus := bus.New()
	perms := permission.NewManager(eventBus)
	executor := agent.NewToolExecutor(tool.DefaultRegistry(), perms, eventBus)
	runner := agent.NewRunnerWithExecutor(db, eventBus, scripted, executor)

	if err := runner.Run(context.Background(), session.ID); err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(scripted.requests) > agent.MaxAgentSteps {
		t.Fatalf("expected at most %d provider calls, got %d", agent.MaxAgentSteps, len(scripted.requests))
	}
}

func TestRunnerEmitsToolThinkingEventEachStep(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	projectDir := t.TempDir()
	project, err := db.GetOrCreateProject(context.Background(), projectDir, "project")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	session, err := db.CreateSession(context.Background(), storage.CreateSessionInput{ProjectID: project.ID, Title: "Thinking", Model: "fake/model", Agent: "build"})
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	userMsg, err := db.CreateMessage(context.Background(), session.ID, "user")
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	userText := "hi"
	if _, err := db.CreatePart(context.Background(), storage.CreatePartInput{MessageID: userMsg.ID, Type: "text", OrderNum: 0, Text: &userText}); err != nil {
		t.Fatalf("part: %v", err)
	}

	eventBus := bus.New()
	_, events := eventBus.Subscribe(project.ID, session.ID, 16)

	scripted := &scriptedProvider{
		batches: [][]provider.ChatEvent{
			{
				{Delta: "ok"},
				{Done: true},
			},
		},
	}
	runner := agent.NewRunner(db, eventBus, scripted)

	if err := runner.Run(context.Background(), session.ID); err != nil {
		t.Fatalf("run: %v", err)
	}

	sawThinking := false
	for len(events) > 0 {
		event := <-events
		if event.Type == "tool.thinking" {
			payload, ok := event.Payload.(map[string]any)
			if !ok {
				t.Fatalf("tool.thinking payload type = %T", event.Payload)
			}
			if payload["step"] == nil || payload["total"] == nil {
				t.Fatalf("tool.thinking payload missing fields: %+v", payload)
			}
			sawThinking = true
		}
	}
	if !sawThinking {
		t.Fatal("expected tool.thinking event before tool round")
	}
}

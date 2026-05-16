package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type fakeRunner struct {
	newSessionID string
	prompts      []string
}

type blockingRunner struct{}

func (blockingRunner) SendPrompt(ctx context.Context, sessionID, prompt string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func (blockingRunner) NewSession(ctx context.Context) (string, error) {
	return "new-session", nil
}

func (f *fakeRunner) SendPrompt(ctx context.Context, sessionID, prompt string) (string, error) {
	f.prompts = append(f.prompts, prompt)
	return "answer: " + prompt, nil
}

func (f *fakeRunner) NewSession(ctx context.Context) (string, error) {
	return f.newSessionID, nil
}

func TestInitialModelRendersOpenCodeLikeShell(t *testing.T) {
	m := NewModel(Config{SessionID: "session-123456", Model: "kr/claude", Runner: &fakeRunner{}})
	view := m.View()
	for _, want := range []string{"Zero", "Sessions", "Chat", "ctrl+p palette", "session-"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q in:\n%s", want, view)
		}
	}
}

func TestDefaultModelUsesResponsive9RouterModel(t *testing.T) {
	m := NewModel(Config{SessionID: "session-1", Runner: &fakeRunner{}})
	if m.model != "cx/gpt-5.5" {
		t.Fatalf("model = %q, want cx/gpt-5.5", m.model)
	}
}

func TestCtrlNCreatesNewSession(t *testing.T) {
	runner := &fakeRunner{newSessionID: "new-session-999"}
	m := NewModel(Config{SessionID: "old-session", Runner: runner})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = next.(Model)

	if m.sessionID != "new-session-999" {
		t.Fatalf("sessionID = %q", m.sessionID)
	}
	if got := m.messages[len(m.messages)-1].Text; !strings.Contains(got, "new-session") {
		t.Fatalf("last message = %q", got)
	}
}

func TestEnterSendsPrompt(t *testing.T) {
	runner := &fakeRunner{}
	m := NewModel(Config{SessionID: "session-1", Runner: runner})
	m.input.SetValue("hello")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)

	if len(runner.prompts) != 1 || runner.prompts[0] != "hello" {
		t.Fatalf("prompts = %#v", runner.prompts)
	}
	view := m.View()
	if !strings.Contains(view, "YOU") || !strings.Contains(view, "answer: hello") {
		t.Fatalf("view missing chat messages:\n%s", view)
	}
}

func TestTypingRunesUpdatesPromptInput(t *testing.T) {
	m := NewModel(Config{SessionID: "session-1", Runner: &fakeRunner{}})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m = next.(Model)

	if got := m.input.Value(); got != "hi" {
		t.Fatalf("input value = %q, want hi", got)
	}
}

func TestEnterStartsAsyncSendAndShowsRunning(t *testing.T) {
	runner := &fakeRunner{}
	m := NewModel(Config{SessionID: "session-1", Runner: runner})
	m.input.SetValue("hello")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if cmd == nil {
		t.Fatalf("expected async send command")
	}
	if !m.busy {
		t.Fatalf("model should be busy immediately after enter")
	}
	if len(runner.prompts) != 0 {
		t.Fatalf("runner should not be called synchronously, got %#v", runner.prompts)
	}

	msg := cmd()
	next, _ = m.Update(msg)
	m = next.(Model)
	if m.busy {
		t.Fatalf("model should stop running after send result")
	}
	if !strings.Contains(m.View(), "answer: hello") {
		t.Fatalf("view missing async answer:\n%s", m.View())
	}
}

func TestRunningViewShowsAgentTypingIndicator(t *testing.T) {
	runner := &fakeRunner{}
	m := NewModel(Config{SessionID: "session-1", Runner: runner})
	m.input.SetValue("hello")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	view := m.View()
	for _, want := range []string{"Agent thinking", "ZERO", "typing", "▌"} {
		if !strings.Contains(view, want) {
			t.Fatalf("running view missing %q:\n%s", want, view)
		}
	}
}

func TestReadableChatLabels(t *testing.T) {
	m := NewModel(Config{SessionID: "session-1", Runner: &fakeRunner{}})
	m.messages = []Message{{Role: "user", Text: "hi"}, {Role: "assistant", Text: "hello"}, {Role: "error", Text: "boom"}}
	m.syncViewport()
	view := m.View()
	for _, want := range []string{"YOU", "ZERO", "ERROR", "hi", "hello", "boom"} {
		if !strings.Contains(view, want) {
			t.Fatalf("readable view missing %q:\n%s", want, view)
		}
	}
}

func TestCtrlJSendsPromptBecauseSomeTerminalsMapEnterToCtrlJ(t *testing.T) {
	runner := &fakeRunner{}
	m := NewModel(Config{SessionID: "session-1", Runner: runner})
	m.input.SetValue("hello")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = next.(Model)

	if cmd == nil {
		t.Fatalf("expected ctrl+j to send command")
	}
	if !m.busy {
		t.Fatalf("expected running state after ctrl+j")
	}
}

func TestSendPromptTimesOutAndClearsRunningState(t *testing.T) {
	m := NewModel(Config{SessionID: "session-1", Runner: blockingRunner{}, SendTimeout: time.Millisecond})
	m.input.SetValue("hello")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatalf("expected send command")
	}

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		next, _ = m.Update(msg)
		m = next.(Model)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("send command did not time out")
	}

	if m.busy {
		t.Fatalf("expected running state to clear after timeout")
	}
	if !strings.Contains(m.View(), "timed out") {
		t.Fatalf("expected timeout message in view:\n%s", m.View())
	}
}

func TestCtrlCQuits(t *testing.T) {
	m := NewModel(Config{SessionID: "session-1", Runner: &fakeRunner{}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatalf("expected quit command")
	}
}

func TestAssistantDeltaStreamsIncrementally(t *testing.T) {
	m := NewModel(Config{SessionID: "session-1", Runner: &fakeRunner{}})
	m.busy = true

	next, _ := m.Update(AssistantDeltaMsg{Delta: "Hel"})
	m = next.(Model)
	next, _ = m.Update(AssistantDeltaMsg{Delta: "lo"})
	m = next.(Model)

	last := m.messages[len(m.messages)-1]
	if last.Role != "streaming" || last.Text != "Hello" {
		t.Fatalf("streaming message = %#v", last)
	}
	if !strings.Contains(m.View(), "ZERO ●") {
		t.Fatalf("view missing streaming indicator:\n%s", m.View())
	}
}

func TestRunDoneMsgFinalizesStreaming(t *testing.T) {
	m := NewModel(Config{SessionID: "session-1", Runner: &fakeRunner{}})
	m.busy = true
	m.messages = append(m.messages, Message{Role: "streaming", Text: "Hello"})

	next, _ := m.Update(RunDoneMsg{})
	m = next.(Model)

	if m.busy {
		t.Fatalf("expected busy=false after RunDoneMsg")
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != "assistant" || last.Text != "Hello" {
		t.Fatalf("finalized message = %#v", last)
	}
}

func TestRunErrMsgShowsError(t *testing.T) {
	m := NewModel(Config{SessionID: "session-1", Runner: &fakeRunner{}})
	m.busy = true

	next, _ := m.Update(RunErrMsg{Err: fmt.Errorf("provider failed")})
	m = next.(Model)

	if m.busy {
		t.Fatalf("expected busy=false after RunErrMsg")
	}
	if !strings.Contains(m.View(), "ERROR") || !strings.Contains(m.View(), "provider failed") {
		t.Fatalf("view missing error:\n%s", m.View())
	}
}

func TestToolStartedAndCompletedShowsCards(t *testing.T) {
	m := NewModel(Config{SessionID: "session-1", Runner: &fakeRunner{}})
	m.busy = true

	next, _ := m.Update(ToolStartedMsg{Name: "read", Args: `{"path":"main.go"}`})
	m = next.(Model)
	view := m.View()
	if !strings.Contains(view, "TOOL read") || !strings.Contains(view, "running") {
		t.Fatalf("view missing tool started card:\n%s", view)
	}

	next, _ = m.Update(ToolCompletedMsg{Name: "read", Result: "file content here"})
	m = next.(Model)
	view = m.View()
	if !strings.Contains(view, "TOOL read") || !strings.Contains(view, "done") {
		t.Fatalf("view missing tool completed card:\n%s", view)
	}
	if !strings.Contains(view, "file content here") {
		t.Fatalf("view missing tool result:\n%s", view)
	}
}

func TestPermissionRequiredShowsPrompt(t *testing.T) {
	m := NewModel(Config{SessionID: "session-1", Runner: &fakeRunner{}})
	m.busy = true

	next, _ := m.Update(PermissionRequiredMsg{ID: "p1", Tool: "bash", Summary: "go test ./..."})
	m = next.(Model)
	view := m.View()

	for _, want := range []string{"PERMISSION REQUIRED", "bash", "go test ./...", "[a] allow", "[d] deny"} {
		if !strings.Contains(view, want) {
			t.Fatalf("permission view missing %q:\n%s", want, view)
		}
	}
	if !m.permissionPending {
		t.Fatalf("expected permissionPending=true")
	}
}

func TestPermissionAllowClearsPrompt(t *testing.T) {
	m := NewModel(Config{SessionID: "session-1", Runner: &fakeRunner{}})
	m.busy = true
	m.permissionPending = true
	m.pendingPermission = &PendingPermission{ID: "p1", Tool: "bash", Summary: "go test ./..."}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = next.(Model)

	if m.permissionPending {
		t.Fatalf("expected permissionPending=false after allow")
	}
}

func TestPermissionDenyClearsPrompt(t *testing.T) {
	m := NewModel(Config{SessionID: "session-1", Runner: &fakeRunner{}})
	m.busy = true
	m.permissionPending = true
	m.pendingPermission = &PendingPermission{ID: "p1", Tool: "bash", Summary: "go test ./..."}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = next.(Model)

	if m.permissionPending {
		t.Fatalf("expected permissionPending=false after deny")
	}
}

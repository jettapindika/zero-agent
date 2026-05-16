package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type Runner interface {
	SendPrompt(ctx context.Context, sessionID, prompt string) (string, error)
	NewSession(ctx context.Context) (string, error)
}

type Config struct {
	SessionID   string
	Model       string
	Runner      Runner
	SendTimeout time.Duration
}

type Message struct {
	Role string
	Text string
	Tool *ToolCall
}

type ToolCall struct {
	Name   string
	Args   string
	Result string
	Status string // "running" or "done"
}

type sendResultMsg struct {
	Answer string
	Err    error
}

// Exported event messages for dispatching from CLI runner
type AssistantDeltaMsg struct{ Delta string }
type ToolStartedMsg struct{ Name, Args string }
type ToolCompletedMsg struct{ Name, Result string }
type PermissionRequiredMsg struct{ ID, Tool, Summary string }
type RunDoneMsg struct{}
type RunErrMsg struct{ Err error }

type PendingPermission struct {
	ID      string
	Tool    string
	Summary string
}

type Model struct {
	sessionID         string
	model             string
	runner            Runner
	messages          []Message
	input             textarea.Model
	viewport          viewport.Model
	spinner           spinner.Model
	width             int
	height            int
	busy              bool
	busySince         time.Time
	sendTimeout       time.Duration
	permissionPending bool
	pendingPermission *PendingPermission
}

func NewModel(cfg Config) Model {
	input := textarea.New()
	input.Placeholder = "Ask Zero…"
	input.ShowLineNumbers = false
	input.Focus()
	input.SetHeight(3)
	input.CharLimit = 0

	vp := viewport.New(80, 20)
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	model := cfg.Model
	if model == "" {
		model = "cx/gpt-5.5"
	}
	sendTimeout := cfg.SendTimeout
	if sendTimeout == 0 {
		sendTimeout = 90 * time.Second
	}
	m := Model{
		sessionID:   cfg.SessionID,
		model:       model,
		runner:      cfg.Runner,
		input:       input,
		viewport:    vp,
		spinner:     sp,
		width:       100,
		height:      30,
		sendTimeout: sendTimeout,
		messages:    []Message{{Role: "system", Text: "Welcome to Zero. ctrl+p palette · ctrl+n new · ctrl+c quit"}},
	}
	m.syncViewport()
	return m
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case AssistantDeltaMsg:
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "streaming" {
			m.messages[len(m.messages)-1].Text += msg.Delta
		} else {
			m.messages = append(m.messages, Message{Role: "streaming", Text: msg.Delta})
		}
		m.syncViewport()
		return m, nil
	case RunDoneMsg:
		m.busy = false
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "streaming" {
			m.messages[len(m.messages)-1].Role = "assistant"
		}
		m.syncViewport()
		return m, nil
	case RunErrMsg:
		m.busy = false
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "streaming" {
			m.messages[len(m.messages)-1].Role = "assistant"
		}
		m.messages = append(m.messages, Message{Role: "error", Text: msg.Err.Error()})
		m.syncViewport()
		return m, nil
	case ToolStartedMsg:
		m.messages = append(m.messages, Message{Role: "tool", Tool: &ToolCall{Name: msg.Name, Args: msg.Args, Status: "running"}})
		m.syncViewport()
		return m, nil
	case ToolCompletedMsg:
		for i := len(m.messages) - 1; i >= 0; i-- {
			if m.messages[i].Role == "tool" && m.messages[i].Tool != nil && m.messages[i].Tool.Name == msg.Name && m.messages[i].Tool.Status == "running" {
				m.messages[i].Tool.Status = "done"
				m.messages[i].Tool.Result = msg.Result
				break
			}
		}
		m.syncViewport()
		return m, nil
	case PermissionRequiredMsg:
		m.permissionPending = true
		m.pendingPermission = &PendingPermission{ID: msg.ID, Tool: msg.Tool, Summary: msg.Summary}
		m.syncViewport()
		return m, nil
	case editorResultMsg:
		if msg.Content != "" {
			m.input.SetValue(msg.Content)
		}
		return m, nil
	case sendResultMsg:
		m.busy = false
		if msg.Err != nil {
			m.messages = append(m.messages, Message{Role: "error", Text: msg.Err.Error()})
		} else {
			m.messages = append(m.messages, Message{Role: "assistant", Text: msg.Answer})
		}
		m.syncViewport()
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.syncViewport()
		return m, nil
	case tea.KeyMsg:
		if m.permissionPending {
			switch msg.String() {
			case "a":
				m.permissionPending = false
				m.pendingPermission = nil
				m.messages = append(m.messages, Message{Role: "system", Text: "Permission allowed"})
				m.syncViewport()
				return m, nil
			case "d", "esc":
				m.permissionPending = false
				m.pendingPermission = nil
				m.messages = append(m.messages, Message{Role: "system", Text: "Permission denied"})
				m.syncViewport()
				return m, nil
			}
			return m, nil
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyCtrlN:
			if m.runner == nil {
				return m, nil
			}
			sessionID, err := m.runner.NewSession(context.Background())
			if err != nil {
				m.messages = append(m.messages, Message{Role: "error", Text: err.Error()})
			} else {
				m.sessionID = sessionID
				m.messages = append(m.messages, Message{Role: "system", Text: "New session: " + sessionID})
			}
			m.syncViewport()
			return m, nil
		case tea.KeyCtrlJ:
			return m.submitPrompt()
		case tea.KeyEnter:
			return m.submitPrompt()
		case tea.KeyCtrlE:
			return m, openEditorCmd()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) submitPrompt() (tea.Model, tea.Cmd) {
	if m.busy {
		return m, nil
	}
	prompt := strings.TrimSpace(m.input.Value())
	if prompt == "" {
		return m, nil
	}
	m.input.Reset()

	// Handle slash commands
	if strings.HasPrefix(prompt, "/") {
		return m.handleSlashCommand(prompt)
	}

	m.messages = append(m.messages, Message{Role: "user", Text: prompt})
	if m.runner != nil {
		m.busy = true
		m.busySince = time.Now()
		sessionID := m.sessionID
		runner := m.runner
		timeout := m.sendTimeout
		m.syncViewport()
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			answer, err := runner.SendPrompt(ctx, sessionID, prompt)
			if err == context.DeadlineExceeded || ctx.Err() == context.DeadlineExceeded {
				err = fmt.Errorf("request timed out after %s", timeout)
			}
			return sendResultMsg{Answer: answer, Err: err}
		}
	}
	m.syncViewport()
	return m, nil
}

func (m Model) handleSlashCommand(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	command := parts[0]

	switch command {
	case "/clear":
		m.messages = []Message{{Role: "system", Text: "Chat cleared."}}
		m.syncViewport()
		return m, nil
	case "/new":
		if m.runner == nil {
			return m, nil
		}
		sessionID, err := m.runner.NewSession(context.Background())
		if err != nil {
			m.messages = append(m.messages, Message{Role: "error", Text: err.Error()})
		} else {
			m.sessionID = sessionID
			m.messages = []Message{{Role: "system", Text: "New session: " + sessionID}}
		}
		m.syncViewport()
		return m, nil
	case "/model":
		if len(parts) > 1 {
			m.model = parts[1]
			m.messages = append(m.messages, Message{Role: "system", Text: "Model set to " + m.model})
		} else {
			m.messages = append(m.messages, Message{Role: "system", Text: "Current model: " + m.model})
		}
		m.syncViewport()
		return m, nil
	case "/agent":
		if len(parts) > 1 {
			m.messages = append(m.messages, Message{Role: "system", Text: "Agent set to " + parts[1]})
		} else {
			m.messages = append(m.messages, Message{Role: "system", Text: "Available: build, plan, explore"})
		}
		m.syncViewport()
		return m, nil
	case "/compact":
		m.messages = append(m.messages, Message{Role: "system", Text: "Compaction requested (not yet wired to backend)"})
		m.syncViewport()
		return m, nil
	case "/help":
		help := "Commands: /new /clear /model [name] /agent [name] /compact /help /quit"
		m.messages = append(m.messages, Message{Role: "system", Text: help})
		m.syncViewport()
		return m, nil
	case "/quit", "/exit":
		return m, tea.Quit
	default:
		m.messages = append(m.messages, Message{Role: "error", Text: "Unknown command: " + command + ". Type /help for list."})
		m.syncViewport()
		return m, nil
	}
}

func (m *Model) resize() {
	chatWidth := max(40, m.width-4)
	if m.width >= 100 {
		chatWidth = m.width - 28
	}
	m.viewport.Width = chatWidth
	m.viewport.Height = max(8, m.height-10)
	m.input.SetWidth(max(20, m.width-4))
}

func (m *Model) syncViewport() {
	m.viewport.SetContent(m.renderChat())
	m.viewport.GotoBottom()
}

func (m Model) renderChat() string {
	text := renderMessages(m.messages)
	if m.busy {
		if text != "" {
			text += "\n"
		}
		text += "ZERO  ● typing…\n▌\n"
	}
	return text
}

func renderMessages(messages []Message) string {
	var out strings.Builder
	for _, msg := range messages {
		role := msg.Role
		switch msg.Role {
		case "user":
			role = "YOU"
		case "assistant":
			role = "ZERO"
		case "streaming":
			role = "ZERO ●"
		case "system":
			role = "SYSTEM"
		case "error":
			role = "ERROR"
		case "tool":
			if msg.Tool != nil {
				fmt.Fprintf(&out, "TOOL %s  ● %s\n", msg.Tool.Name, msg.Tool.Status)
				if msg.Tool.Args != "" {
					fmt.Fprintf(&out, "args: %s\n", msg.Tool.Args)
				}
				if msg.Tool.Result != "" {
					fmt.Fprintf(&out, "%s\n", msg.Tool.Result)
				}
				out.WriteString("\n")
			}
			continue
		}
		fmt.Fprintf(&out, "%s\n%s\n\n", role, strings.TrimSpace(msg.Text))
	}
	return out.String()
}

func (m Model) runningFor() int {
	if !m.busy || m.busySince.IsZero() {
		return 0
	}
	return int(time.Since(m.busySince).Seconds())
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Editor support

type editorResultMsg struct{ Content string }

func openEditorCmd() tea.Cmd {
	return func() tea.Msg {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		tmpFile, err := os.CreateTemp("", "zero-prompt-*.md")
		if err != nil {
			return editorResultMsg{}
		}
		tmpFile.Close()
		cmd := exec.Command(editor, tmpFile.Name())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			os.Remove(tmpFile.Name())
			return editorResultMsg{}
		}
		data, err := os.ReadFile(tmpFile.Name())
		os.Remove(tmpFile.Name())
		if err != nil {
			return editorResultMsg{}
		}
		return editorResultMsg{Content: strings.TrimSpace(string(data))}
	}
}

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
	"github.com/charmbracelet/lipgloss"
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
	agent             string
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
		agent:       "build",
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
	case "/clear", "/reset":
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
			m.agent = parts[1]
			m.messages = append(m.messages, Message{Role: "system", Text: "Agent set to " + m.agent})
		} else {
			m.messages = append(m.messages, Message{Role: "system", Text: "Current agent: " + m.agent + "\nAvailable: build, plan, explore"})
		}
		m.syncViewport()
		return m, nil
	case "/plan":
		m.agent = "plan"
		m.messages = append(m.messages, Message{Role: "system", Text: "Plan mode enabled. Agent set to plan."})
		m.syncViewport()
		return m, nil
	case "/ask", "/explore":
		m.agent = "explore"
		m.messages = append(m.messages, Message{Role: "system", Text: "Ask mode enabled. Agent set to explore."})
		m.syncViewport()
		return m, nil
	case "/code", "/build":
		m.agent = "build"
		m.messages = append(m.messages, Message{Role: "system", Text: "Code mode enabled. Agent set to build."})
		m.syncViewport()
		return m, nil
	case "/compact", "/summarize":
		m.messages = append(m.messages, Message{Role: "system", Text: "Compaction requested (not yet wired to backend)"})
		m.syncViewport()
		return m, nil
	case "/status", "/info":
		status := fmt.Sprintf("Status\nSession: %s\nModel: %s\nAgent: %s\nMessages: %d", m.sessionID, m.model, m.agent, len(m.messages))
		m.messages = append(m.messages, Message{Role: "system", Text: status})
		m.syncViewport()
		return m, nil
	case "/history":
		m.messages = append(m.messages, Message{Role: "system", Text: fmt.Sprintf("History\nMessages: %d\nSession: %s", len(m.messages), m.sessionID)})
		m.syncViewport()
		return m, nil
	case "/models":
		m.messages = append(m.messages, Message{Role: "system", Text: "Models\nCurrent: " + m.model + "\nUse /model <provider/model> to switch."})
		m.syncViewport()
		return m, nil
	case "/shortcuts", "/keys":
		shortcuts := "Shortcuts\nenter/ctrl+j send\nctrl+n new session\nctrl+e editor\nesc abort permission prompt\nctrl+c quit"
		m.messages = append(m.messages, Message{Role: "system", Text: shortcuts})
		m.syncViewport()
		return m, nil
	case "/editor", "/edit":
		return m, openEditorCmd()
	case "/help":
		help := "Commands\nSession: /new /clear /reset /history /status\nModes: /agent [name] /plan /ask /code\nModel: /model [name] /models\nContext: /compact /summarize\nInput: /editor /edit /shortcuts\nApp: /help /quit /exit /q"
		m.messages = append(m.messages, Message{Role: "system", Text: help})
		m.syncViewport()
		return m, nil
	case "/quit", "/exit", "/q":
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
	text := renderMessages(m.messages, max(20, m.viewport.Width-4))
	if m.busy {
		if text != "" {
			text += "\n"
		}
		text += "ZERO  ● typing…\n▌\n"
	}
	return text
}

func renderMessages(messages []Message, width ...int) string {
	contentWidth := 76
	if len(width) > 0 && width[0] > 0 {
		contentWidth = width[0]
	}
	var out strings.Builder
	for _, msg := range messages {
		out.WriteString(renderMessageCard(msg, contentWidth))
	}
	return out.String()
}

func renderMessageCard(msg Message, width int) string {
	if msg.Role == "tool" && msg.Tool != nil {
		return renderToolCard(*msg.Tool, width)
	}
	label, style := messageLabel(msg.Role)
	body := formatMessageText(msg.Text, max(20, width-4))
	if body == "" {
		return ""
	}
	return fmt.Sprintf("%s\n%s\n\n", style.Render(label), renderMessageBox(body, max(20, width)))
}

func renderMessageBox(body string, width int) string {
	innerWidth := max(16, width-4)
	border := strings.Repeat("-", innerWidth+2)
	var out strings.Builder
	fmt.Fprintf(&out, "+%s+\n", border)
	for _, line := range strings.Split(body, "\n") {
		fmt.Fprintf(&out, "| %-*s |\n", innerWidth, line)
	}
	fmt.Fprintf(&out, "+%s+", border)
	return out.String()
}

func messageLabel(role string) (string, lipgloss.Style) {
	switch role {
	case "user":
		return "You", userStyle
	case "assistant":
		return "Zero", zeroStyle
	case "streaming":
		return "Zero ●", zeroStyle
	case "system":
		return "System", systemStyle
	case "error":
		return "Error", errorStyle
	default:
		return titleRole(role), mutedStyle
	}
}

func titleRole(role string) string {
	if role == "" {
		return "Message"
	}
	return strings.ToUpper(role[:1]) + role[1:]
}

func renderToolCard(tool ToolCall, width int) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%s\n", toolStyle.Render(fmt.Sprintf("Tool %s  ● %s", tool.Name, tool.Status)))
	if strings.TrimSpace(tool.Args) != "" {
		fmt.Fprintf(&out, "  args: %s\n", wrapLine(strings.TrimSpace(tool.Args), max(20, width-8)))
	}
	if strings.TrimSpace(tool.Result) != "" {
		fmt.Fprintf(&out, "  result:\n%s\n", indentLines(trimToolResult(tool.Result, 6), "    "))
	}
	out.WriteString("\n")
	return out.String()
}

func formatMessageText(text string, width int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	var out []string
	inFence := false
	blank := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimRight(line, " \t")
		if strings.HasPrefix(strings.TrimSpace(trimmed), "```") {
			inFence = !inFence
			out = append(out, trimmed)
			blank = false
			continue
		}
		if strings.TrimSpace(trimmed) == "" {
			if !blank {
				out = append(out, "")
			}
			blank = true
			continue
		}
		blank = false
		if inFence || strings.HasPrefix(trimmed, "    ") || strings.HasPrefix(strings.TrimSpace(trimmed), "|") {
			out = append(out, trimmed)
			continue
		}
		out = append(out, strings.Split(wrapLine(strings.TrimSpace(trimmed), width), "\n")...)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func wrapLine(line string, width int) string {
	if width <= 0 || len(line) <= width {
		return line
	}
	words := strings.Fields(line)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) > width {
			lines = append(lines, current)
			current = word
			continue
		}
		current += " " + word
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

func indentLines(text, prefix string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

func trimToolResult(text string, maxLines int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	return strings.Join(append(lines[:maxLines], fmt.Sprintf("… %d more lines", len(lines)-maxLines)), "\n")
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

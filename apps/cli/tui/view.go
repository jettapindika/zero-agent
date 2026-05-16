package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DCFFF")).Bold(true)
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#565F89"))
	panelStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#414868")).Padding(0, 1)
	chatStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#7AA2F7")).Padding(0, 1)
)

func (m Model) View() string {
	header := accentStyle.Render("Zero") + mutedStyle.Render(fmt.Sprintf(" │ %s │ %s │ %s", m.model, shortID(m.sessionID), statusText(m)))
	chat := chatStyle.Width(max(40, m.viewport.Width)).Height(max(8, m.viewport.Height)).Render("Chat\n" + m.viewport.View())

	body := chat
	if m.width >= 100 {
		sidebar := panelStyle.Width(22).Height(max(8, m.viewport.Height)).Render("Sessions\n> " + shortID(m.sessionID) + "\n\nTeam\nQueue")
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, chat)
	}

	var inputSection string
	if m.permissionPending && m.pendingPermission != nil {
		p := m.pendingPermission
		inputSection = panelStyle.Width(max(40, m.width-4)).Render(
			fmt.Sprintf("PERMISSION REQUIRED\n\nTool: %s\nAction: %s\n\n[a] allow  [d] deny  [esc] deny", p.Tool, p.Summary),
		)
	} else {
		inputSection = panelStyle.Width(max(40, m.width-4)).Render(m.input.View())
	}

	footer := mutedStyle.Render("enter/ctrl+j send · ctrl+n new · esc abort · ctrl+c quit")
	return strings.TrimRight(lipgloss.JoinVertical(lipgloss.Left, header, body, inputSection, footer), "\n")
}

func statusText(m Model) string {
	if m.busy {
		return fmt.Sprintf("● Agent thinking… %ds", m.runningFor())
	}
	return "○ idle"
}

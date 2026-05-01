package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"pm-agent/internal/app"
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#545454")).
			Render
)

func View(m app.Model) string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("PM AI Agent"))
	s.WriteString("\n\n")
	s.WriteString(m.Viewport.View())
	s.WriteString("\n\n")
	s.WriteString(m.TextInput.View())
	s.WriteString("\n\n")
	if m.Loading {
		s.WriteString(InfoStyle("Agent is thinking..."))
	} else {
		s.WriteString(InfoStyle("Press Enter to send • Esc to quit"))
	}

	return s.String()
}

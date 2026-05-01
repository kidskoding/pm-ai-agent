package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"pm-agent/internal/app"
	"pm-agent/pkg/models"
)

var (
	// Colors
	purple    = lipgloss.Color("#7D56F4")
	darkGray  = lipgloss.Color("#242424")
	lightGray = lipgloss.Color("#D9D9D9")
	accent    = lipgloss.Color("#00F5FF")
	errorRed  = lipgloss.Color("#FF4C4C")
	green     = lipgloss.Color("#04B575")
	yellow    = lipgloss.Color("#ECBE13")

	// Layout Styles
	MainContainer = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(purple).
			Padding(0, 2).
			MarginBottom(1)

	SidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(darkGray).
			Padding(0, 1).
			MarginRight(2).
			Width(25)

	// Message Styles
	UserLabel = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)

	AgentLabel = lipgloss.NewStyle().
			Foreground(purple).
			Bold(true)

	SystemLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#545454")).
			Italic(true)

	MessageText = lipgloss.NewStyle().
			Foreground(lightGray)

	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(darkGray).
			Padding(0, 1)

	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#545454"))

	// Backlog Table Styles
	TableHeader = lipgloss.NewStyle().
			Background(darkGray).
			Foreground(lightGray).
			Bold(true).
			Padding(0, 1)

	TableRow = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(darkGray)

	HighPriority   = lipgloss.NewStyle().Foreground(errorRed).Bold(true)
	MediumPriority = lipgloss.NewStyle().Foreground(yellow).Bold(true)
	LowPriority    = lipgloss.NewStyle().Foreground(green).Bold(true)
)

func View(m app.Model) string {
	// Sidebar content (25) + padding (2) + right border (1) - header padding (4) = 24... simplified: viewport offset complement
	totalWidth := m.Viewport.Width + 26
	var content string

	if m.Mode == app.ChatView {
		content = renderChat(m)
	} else if m.Mode == app.BacklogView {
		content = renderBacklog(m)
	} else {
		content = renderWorkflows(m)
	}

	// Sidebar (Stats & Context)
	sidebarContent := lipgloss.JoinVertical(lipgloss.Left,
		UserLabel.Render("PROJECT STATUS"),
		MessageText.Render("󰙅 Mode: Autonomous"),
		MessageText.Render(fmt.Sprintf("󱇳 Backlog: %d items", len(m.Backlog))),
		"",
		UserLabel.Render("SHORTCUTS"),
		MessageText.Render("󰘳 Tab: Switch View"),
		MessageText.Render("󰘳 Enter: Send"),
		MessageText.Render("󰘳 Esc: Exit"),
	)
	sidebar := SidebarStyle.Render(sidebarContent)

	// Input Area
	inputArea := InputStyle.Width(totalWidth).Render(m.TextInput.View())

	// Status Line
	var status string
	if m.Loading {
		status = lipgloss.JoinHorizontal(lipgloss.Center, m.Spinner.View(), lipgloss.NewStyle().Foreground(accent).Render(" Agent is analyzing context..."))
	} else {
		status = lipgloss.NewStyle().Width(totalWidth + 4).Align(lipgloss.Right).Render(InfoStyle.Render("IDLE • READY FOR COMMANDS"))
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
	
	mainView := MainContainer.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			HeaderStyle.Width(totalWidth).Render("PM AI AGENT v0.2.0 • STRATEGIC TERMINAL"),
			body,
			"\n",
			inputArea,
			status,
		),
	)

	return mainView
}

func renderChat(m app.Model) string {
	var styledMessages []string
	for _, msg := range m.Messages {
		if strings.HasPrefix(msg, "You: ") {
			content := strings.TrimPrefix(msg, "You: ")
			styledMessages = append(styledMessages, fmt.Sprintf("%s\n%s", UserLabel.Render("󰭹 YOU"), MessageText.Render(content)))
		} else if strings.HasPrefix(msg, "Agent: ") {
			content := strings.TrimPrefix(msg, "Agent: ")
			styledMessages = append(styledMessages, fmt.Sprintf("%s\n%s", AgentLabel.Render("󰚩 PM AGENT"), MessageText.Render(content)))
		} else if strings.HasPrefix(msg, "System: ") {
			content := strings.TrimPrefix(msg, "System: ")
			styledMessages = append(styledMessages, SystemLabel.Render("󱓞 "+content))
		} else if strings.HasPrefix(msg, "Success! ") {
			styledMessages = append(styledMessages, lipgloss.NewStyle().
				Foreground(green).
				Render("󰄬 "+msg))
		} else {
			styledMessages = append(styledMessages, MessageText.Render(msg))
		}
	}
	m.Viewport.SetContent(strings.Join(styledMessages, "\n\n"))
	return m.Viewport.View()
}

func renderBacklog(m app.Model) string {
	var s strings.Builder
	
	s.WriteString(UserLabel.Render("󱇳 PROJECT BACKLOG") + "\n\n")

	// Table Header
	header := TableHeader.Width(m.Viewport.Width).Render(
		fmt.Sprintf("%-30s | %-10s | %s", "TITLE", "PRIORITY", "USER STORY"),
	)
	s.WriteString(header + "\n")

	if len(m.Backlog) == 0 {
		s.WriteString("\n  No tasks in backlog. Ask the agent to create one!")
	}

	for _, task := range m.Backlog {
		pStyle := LowPriority
		switch task.Priority {
		case "HIGH":
			pStyle = HighPriority
		case "MEDIUM":
			pStyle = MediumPriority
		}

		title := task.Title
		if len(title) > 28 {
			title = title[:25] + "..."
		}

		story := task.UserStory
		if len(story) > m.Viewport.Width-45 {
			story = story[:m.Viewport.Width-48] + "..."
		}

		row := TableRow.Width(m.Viewport.Width).Render(
			fmt.Sprintf("%-30s | %-10s | %s", 
				title, 
				pStyle.Render(string(task.Priority)), 
				MessageText.Render(story)),
		)
		s.WriteString(row + "\n")
	}

	return s.String()
}

func renderWorkflows(m app.Model) string {
	var sb strings.Builder
	sb.WriteString(UserLabel.Render("󰙅 AUTONOMOUS WORKFLOWS") + "\n\n")

	names := []string{"grooming", "standup", "stakeholder"}
	labels := map[string]string{
		"grooming":    "Backlog Grooming  (on commit)",
		"standup":     "Daily Standup     (09:00)",
		"stakeholder": "Stakeholder Update (Mon 09:00)",
	}

	for _, name := range names {
		run, ok := m.WorkflowRuns[name]
		var statusStr string
		if !ok {
			statusStr = InfoStyle.Render("idle")
		} else {
			switch run.Status {
			case models.WorkflowRunning:
				statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECBE13")).Render("running")
			case models.WorkflowDone:
				statusStr = LowPriority.Render("done  " + run.LastRun.Format("15:04"))
			case models.WorkflowFailed:
				statusStr = HighPriority.Render("failed: " + run.Err)
			default:
				statusStr = InfoStyle.Render("idle")
			}
		}
		sb.WriteString(MessageText.Render(fmt.Sprintf("%-34s %s", labels[name], statusStr)) + "\n\n")
	}

	return sb.String()
}

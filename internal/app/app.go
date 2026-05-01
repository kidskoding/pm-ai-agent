package app

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"pm-agent/internal/agent"
	"pm-agent/pkg/models"
)

type ViewMode int

const (
	ChatView     ViewMode = iota
	BacklogView
	WorkflowView
)

type AgentResponseMsg string
type ErrMsg error
type BacklogMsg []models.AddTaskArgs
type WorkflowEventMsg models.WorkflowRun

type Model struct {
	Viewport     viewport.Model
	Messages     []string
	TextInput    textinput.Model
	Spinner      spinner.Model
	Err          error
	Loading      bool
	Agent        *agent.PMAgent
	Mode         ViewMode
	Backlog      []models.AddTaskArgs
	WorkflowRuns map[string]models.WorkflowRun
}

func InitialModel(a *agent.PMAgent) Model {
	ti := textinput.New()
	ti.Placeholder = "Type your idea here..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 40

	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to PM AI Agent TUI!\nType an idea to get started.")

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	return Model{
		TextInput:    ti,
		Viewport:     vp,
		Messages:     []string{},
		Spinner:      s,
		Agent:        a,
		WorkflowRuns: make(map[string]models.WorkflowRun),
	}
}

func (m Model) UpdateAgent(prompt string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		resp, err := m.Agent.GenerateResponse(ctx, prompt)
		if err != nil {
			return ErrMsg(err)
		}
		return AgentResponseMsg(resp)
	}
}

func FetchBacklog() tea.Cmd {
	return func() tea.Msg {
		return nil
	}
}

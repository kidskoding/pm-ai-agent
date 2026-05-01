package app

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"pm-agent/internal/agent"
)

type AgentResponseMsg string
type ErrMsg error

type Model struct {
	Viewport  viewport.Model
	Messages  []string
	TextInput textinput.Model
	Err       error
	Loading   bool
	Agent     *agent.PMAgent
}

func InitialModel(agent *agent.PMAgent) Model {
	ti := textinput.New()
	ti.Placeholder = "Type your idea here..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 40

	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to PM AI Agent TUI!\nType an idea to get started.")

	return Model{
		TextInput: ti,
		Viewport:  vp,
		Messages:  []string{},
		Agent:     agent,
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

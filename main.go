package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"pm-agent/internal/agent"
	"pm-agent/internal/app"
	"pm-agent/internal/ui"
)

type model struct {
	appModel app.Model
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.appModel.TextInput, tiCmd = m.appModel.TextInput.Update(msg)
	m.appModel.Viewport, vpCmd = m.appModel.Viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.appModel.Loading {
				return m, nil
			}
			input := m.appModel.TextInput.Value()
			if input == "" {
				return m, nil
			}

			m.appModel.Messages = append(m.appModel.Messages, fmt.Sprintf("You: %s", input))
			m.appModel.Loading = true
			m.appModel.TextInput.Reset()
			m.appModel.Viewport.SetContent(strings.Join(m.appModel.Messages, "\n\n"))
			m.appModel.Viewport.GotoBottom()

			return m, m.appModel.UpdateAgent(input)
		}

	case app.AgentResponseMsg:
		m.appModel.Loading = false
		m.appModel.Messages = append(m.appModel.Messages, fmt.Sprintf("Agent: %s", string(msg)))
		m.appModel.Viewport.SetContent(strings.Join(m.appModel.Messages, "\n\n"))
		m.appModel.Viewport.GotoBottom()
		return m, nil

	case app.ErrMsg:
		m.appModel.Err = msg
		m.appModel.Loading = false
		m.appModel.Messages = append(m.appModel.Messages, fmt.Sprintf("Error: %v", msg))
		m.appModel.Viewport.SetContent(strings.Join(m.appModel.Messages, "\n\n"))
		return m, nil

	case tea.WindowSizeMsg:
		m.appModel.Viewport.Width = msg.Width
		m.appModel.Viewport.Height = msg.Height - 6
		if m.appModel.Viewport.Height < 0 {
			m.appModel.Viewport.Height = 0
		}
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) View() string {
	return ui.View(m.appModel)
}

func main() {
	_ = godotenv.Load()

	ctx := context.Background()
	pmAgent, err := agent.NewPMAgent(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	initialAppModel := app.InitialModel(pmAgent)
	
	p := tea.NewProgram(model{appModel: initialAppModel}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

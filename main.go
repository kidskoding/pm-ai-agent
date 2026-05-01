package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"pm-agent/internal/agent"
	"pm-agent/internal/app"
	"pm-agent/internal/daemon"
	"pm-agent/internal/ui"
	"pm-agent/internal/workflows"
	"pm-agent/pkg/backlog"
	"pm-agent/pkg/ghclient"
	"pm-agent/pkg/models"
	"pm-agent/pkg/scanner"
)

type model struct {
	appModel  app.Model
	daemonCh  <-chan models.WorkflowRun
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.appModel.Spinner.Tick,
		fetchBacklogCmd(),
		listenDaemon(m.daemonCh),
	)
}

func fetchBacklogCmd() tea.Cmd {
	return func() tea.Msg {
		tasks, _ := backlog.LoadBacklog()
		return app.BacklogMsg(tasks)
	}
}

func listenDaemon(ch <-chan models.WorkflowRun) tea.Cmd {
	return func() tea.Msg {
		return app.WorkflowEventMsg(<-ch)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		spCmd tea.Cmd
	)

	m.appModel.TextInput, tiCmd = m.appModel.TextInput.Update(msg)
	m.appModel.Viewport, vpCmd = m.appModel.Viewport.Update(msg)
	m.appModel.Spinner, spCmd = m.appModel.Spinner.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyTab:
			switch m.appModel.Mode {
			case app.ChatView:
				m.appModel.Mode = app.BacklogView
			case app.BacklogView:
				m.appModel.Mode = app.WorkflowView
			case app.WorkflowView:
				m.appModel.Mode = app.ChatView
			}
			return m, nil
		case tea.KeyEnter:
			if m.appModel.Mode != app.ChatView || m.appModel.Loading {
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

	case spinner.TickMsg:
		return m, spCmd

	case app.AgentResponseMsg:
		m.appModel.Loading = false
		m.appModel.Messages = append(m.appModel.Messages, fmt.Sprintf("Agent: %s", string(msg)))
		m.appModel.Viewport.SetContent(strings.Join(m.appModel.Messages, "\n\n"))
		m.appModel.Viewport.GotoBottom()
		return m, fetchBacklogCmd()

	case app.BacklogMsg:
		m.appModel.Backlog = msg
		return m, nil

	case app.WorkflowEventMsg:
		run := models.WorkflowRun(msg)
		m.appModel.WorkflowRuns[run.Name] = run
		// re-arm the listener
		return m, listenDaemon(m.daemonCh)

	case app.ErrMsg:
		m.appModel.Err = msg
		m.appModel.Loading = false
		m.appModel.Messages = append(m.appModel.Messages, fmt.Sprintf("Error: %v", msg))
		m.appModel.Viewport.SetContent(strings.Join(m.appModel.Messages, "\n\n"))
		return m, nil

	case tea.WindowSizeMsg:
		// Sidebar content (25) + padding (2) + right border (1) + margin (2) + ContainerPadding (4) + ContainerBorder (2) = 36
		m.appModel.Viewport.Width = msg.Width - 36
		m.appModel.Viewport.Height = msg.Height - 12
		if m.appModel.Viewport.Width < 0 {
			m.appModel.Viewport.Width = 0
		}
	}

	return m, tea.Batch(tiCmd, vpCmd, spCmd)
}

func (m model) View() string {
	return ui.View(m.appModel)
}

func main() {
	_ = godotenv.Load()

	ctx := context.Background()

	projectCtx, err := scanner.ScanProject(".")
	if err != nil {
		log.Printf("Warning: could not scan project: %v", err)
		projectCtx = &scanner.ProjectContext{Structure: "Unknown structure"}
	}

	github := ghclient.New()

	pmAgent, err := agent.NewPMAgent(ctx, projectCtx.Structure, github)
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	d := daemon.New(
		workflows.NewGroomingWorkflow(pmAgent, github),
		workflows.NewStandupWorkflow(pmAgent, github),
		workflows.NewStakeholderWorkflow(pmAgent, github),
	)

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go d.Run(cancelCtx)

	initialAppModel := app.InitialModel(pmAgent)
	initialAppModel.Messages = append(initialAppModel.Messages,
		fmt.Sprintf("System: Indexed %d files. Ready to manage your project.", projectCtx.FileCount))
	initialAppModel.Viewport.SetContent(strings.Join(initialAppModel.Messages, "\n\n"))

	p := tea.NewProgram(
		model{appModel: initialAppModel, daemonCh: d.Events},
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

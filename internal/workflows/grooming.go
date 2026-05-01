package workflows

import (
	"context"
	"fmt"
	"strings"

	"pm-agent/internal/agent"
	"pm-agent/pkg/backlog"
	"pm-agent/pkg/ghclient"
	"pm-agent/pkg/git"
)

type GroomingWorkflow struct {
	Agent  *agent.PMAgent
	GitHub *ghclient.Client
}

func NewGroomingWorkflow(a *agent.PMAgent, gh *ghclient.Client) *GroomingWorkflow {
	return &GroomingWorkflow{Agent: a, GitHub: gh}
}

func (w *GroomingWorkflow) Run(ctx context.Context) error {
	todos, err := git.ScanTodos(".")
	if err != nil {
		return fmt.Errorf("scan todos: %w", err)
	}

	commits, err := git.ReadLog(10)
	if err != nil {
		return fmt.Errorf("read git log: %w", err)
	}

	existing, _ := backlog.LoadBacklog()
	existingTitles := make([]string, len(existing))
	for i, t := range existing {
		existingTitles[i] = t.Title
	}

	prompt := buildGroomingPrompt(todos, commits, existingTitles)
	_, err = w.Agent.GenerateResponse(ctx, prompt)
	return err
}

func buildGroomingPrompt(todos []git.TodoItem, commits []git.Commit, existingTitles []string) string {
	var sb strings.Builder
	sb.WriteString("You are performing autonomous backlog grooming.\n\n")

	if len(todos) > 0 {
		sb.WriteString("TODOs found in source code:\n")
		for _, t := range todos {
			sb.WriteString(fmt.Sprintf("- %s:%d: %s\n", t.File, t.Line, t.Text))
		}
		sb.WriteString("\n")
	}

	if len(commits) > 0 {
		sb.WriteString("Recent commits:\n")
		for _, c := range commits {
			sb.WriteString(fmt.Sprintf("- [%s] %s (%s): %s\n", c.Hash[:7], c.Author, c.Date, c.Message))
		}
		sb.WriteString("\n")
	}

	if len(existingTitles) > 0 {
		sb.WriteString("Existing backlog (do not duplicate these):\n")
		for _, t := range existingTitles {
			sb.WriteString(fmt.Sprintf("- %s\n", t))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Based on the above, use the create_task tool to add tasks for any important missing functionality or technical debt. Be selective — only create tasks that provide real product value. If nothing meaningful is missing, do nothing.")
	return sb.String()
}

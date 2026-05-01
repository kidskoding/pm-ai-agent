package workflows

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pm-agent/internal/agent"
	"pm-agent/pkg/backlog"
	"pm-agent/pkg/ghclient"
	"pm-agent/pkg/git"
	"pm-agent/pkg/models"
)

type StakeholderWorkflow struct {
	Agent  *agent.PMAgent
	GitHub *ghclient.Client
}

func NewStakeholderWorkflow(a *agent.PMAgent, gh *ghclient.Client) *StakeholderWorkflow {
	return &StakeholderWorkflow{Agent: a, GitHub: gh}
}

func (w *StakeholderWorkflow) Run(ctx context.Context) error {
	commits, err := git.ReadLog(100)
	if err != nil {
		return fmt.Errorf("read git log: %w", err)
	}

	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	var recent []git.Commit
	for _, c := range commits {
		d, err := time.Parse("2006-01-02", c.Date)
		if err != nil {
			continue
		}
		if !d.Before(cutoff.Truncate(24 * time.Hour)) {
			recent = append(recent, c)
		}
	}

	tasks, _ := backlog.LoadBacklog()

	prompt := buildStakeholderPrompt(recent, tasks)
	update, err := w.Agent.GenerateResponse(ctx, prompt)
	if err != nil {
		return err
	}

	date := time.Now().Format("2006-01-02")
	path := filepath.Join("docs", "updates", date+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(update), 0644); err != nil {
		return err
	}

	title := fmt.Sprintf("Stakeholder Update — %s", date)
	return w.GitHub.CreateIssue(ctx, title, update, []string{"stakeholder-update"})
}

func buildStakeholderPrompt(commits []git.Commit, tasks []models.AddTaskArgs) string {
	var sb strings.Builder
	sb.WriteString("Write a weekly stakeholder update for a non-technical audience.\n\n")

	if len(commits) > 0 {
		sb.WriteString("What the team shipped this week (from git):\n")
		for _, c := range commits {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", c.Author, c.Message))
		}
		sb.WriteString("\n")
	}

	var pending, completed []models.AddTaskArgs
	for _, t := range tasks {
		if t.Status == models.TaskCompleted {
			completed = append(completed, t)
		} else {
			pending = append(pending, t)
		}
	}

	if len(completed) > 0 {
		sb.WriteString("Completed tasks:\n")
		for _, t := range completed {
			sb.WriteString(fmt.Sprintf("- %s\n", t.Title))
		}
		sb.WriteString("\n")
	}
	if len(pending) > 0 {
		sb.WriteString("Pending tasks (backlog):\n")
		for _, t := range pending {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", t.Priority, t.Title))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Write the update in this format:\n")
	sb.WriteString("## Update — <date>\n\n")
	sb.WriteString("**Shipped:** <2-3 sentences on what was completed, in plain English>\n\n")
	sb.WriteString("**In progress:** <what the team is working on now>\n\n")
	sb.WriteString("**At risk:** <anything that looks delayed or blocked, or 'Nothing flagged'>\n\n")
	sb.WriteString("Avoid technical jargon. Write for a founder or investor.")
	return sb.String()
}

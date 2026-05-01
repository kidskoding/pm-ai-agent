package workflows

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pm-agent/internal/agent"
	"pm-agent/pkg/ghclient"
	"pm-agent/pkg/git"
)

type StandupWorkflow struct {
	Agent  *agent.PMAgent
	GitHub *ghclient.Client
}

func NewStandupWorkflow(a *agent.PMAgent, gh *ghclient.Client) *StandupWorkflow {
	return &StandupWorkflow{Agent: a, GitHub: gh}
}

func (w *StandupWorkflow) Run(ctx context.Context) error {
	commits, err := git.ReadLog(50)
	if err != nil {
		return fmt.Errorf("read git log: %w", err)
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	var recent []git.Commit
	for _, c := range commits {
		d, err := time.Parse("2006-01-02", c.Date)
		if err != nil {
			continue
		}
		if d.After(cutoff) || d.Equal(cutoff.Truncate(24*time.Hour)) {
			recent = append(recent, c)
		}
	}

	prompt := buildStandupPrompt(recent)
	summary, err := w.Agent.GenerateResponse(ctx, prompt)
	if err != nil {
		return err
	}

	date := time.Now().Format("2006-01-02")
	path := filepath.Join("docs", "standups", date+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(summary), 0644); err != nil {
		return err
	}

	return w.GitHub.CommentOnLabeledIssue(ctx, "standup-log",
		"Daily Standup Log", summary)
}

func buildStandupPrompt(commits []git.Commit) string {
	var sb strings.Builder
	sb.WriteString("Generate a concise daily standup summary for the engineering team.\n\n")

	if len(commits) == 0 {
		sb.WriteString("No commits were made in the last 24 hours.\n")
	} else {
		sb.WriteString("Commits from the last 24 hours:\n")
		for _, c := range commits {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", c.Author, c.Date, c.Message))
		}
	}

	sb.WriteString("\nWrite the summary in this format:\n")
	sb.WriteString("## Standup — <date>\n\n")
	sb.WriteString("For each engineer who committed, write:\n")
	sb.WriteString("**<Name>**: <what they worked on, 1-2 sentences>. No blockers noted. (or describe blockers if commit messages suggest them)\n\n")
	sb.WriteString("Keep it short and factual. Do not add commentary or filler.")
	return sb.String()
}

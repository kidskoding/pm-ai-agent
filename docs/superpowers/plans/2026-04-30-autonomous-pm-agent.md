# Autonomous PM Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a background daemon to pm-ai-agent that autonomously grooms the backlog on every commit, generates daily standup summaries, and posts weekly stakeholder updates — all without user prompting.

**Architecture:** A daemon goroutine starts alongside the TUI, polls git for new commits (triggering backlog grooming), and runs a time-based scheduler (standup at 9am daily, stakeholder every Monday). Workflows gather raw data from git/disk, call the existing PMAgent for generation, and write artifacts locally; if GITHUB_TOKEN is set, they also push to GitHub Issues.

**Tech Stack:** Go, Bubble Tea (existing), LangChainGo/Gemini (existing), google/go-github/v66 (new), standard library only for scheduling/git polling.

---

## File Map

### New files
| File | Responsibility |
|---|---|
| `pkg/git/git.go` | `ReadLog`, `LatestHash`, `ScanTodos` |
| `pkg/git/git_test.go` | Tests for git log reader and TODO scanner |
| `pkg/scheduler/scheduler.go` | `Job`, `Scheduler`, `shouldFire` — time-based job runner |
| `pkg/scheduler/scheduler_test.go` | Tests for schedule firing logic |
| `pkg/ghclient/ghclient.go` | Optional GitHub API client; no-ops when token absent |
| `internal/workflows/grooming.go` | `GroomingWorkflow` — commit-triggered backlog grooming |
| `internal/workflows/standup.go` | `StandupWorkflow` — daily standup summary |
| `internal/workflows/stakeholder.go` | `StakeholderWorkflow` — weekly stakeholder update |
| `internal/daemon/daemon.go` | `Daemon` — owns scheduler + git poller, sends events to TUI |

### Modified files
| File | Change |
|---|---|
| `pkg/models/models.go` | Add `TaskStatus`, `WorkflowRunStatus`, `WorkflowRun` |
| `internal/agent/agent.go` | Add `GitHub` field; add `scan_todos`, `read_git_log`, `post_github_issue` tools |
| `internal/app/app.go` | Add `WorkflowRuns`, `WorkflowView` mode, `WorkflowEventMsg` |
| `internal/ui/ui.go` | Add `WorkflowView` rendering |
| `main.go` | Init daemon + GitHub client; wire event channel to TUI |

---

## Task 1: Extend models

**Files:**
- Modify: `pkg/models/models.go`

- [ ] **Step 1: Replace contents of `pkg/models/models.go`**

```go
package models

import "time"

type Priority string

const (
	Low    Priority = "LOW"
	Medium Priority = "MEDIUM"
	High   Priority = "HIGH"
)

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskCompleted TaskStatus = "completed"
)

type AddTaskArgs struct {
	Title     string     `json:"title"`
	UserStory string     `json:"user_story"`
	Priority  Priority   `json:"priority"`
	Status    TaskStatus `json:"status,omitempty"`
}

type WorkflowRunStatus string

const (
	WorkflowIdle    WorkflowRunStatus = "idle"
	WorkflowRunning WorkflowRunStatus = "running"
	WorkflowDone    WorkflowRunStatus = "done"
	WorkflowFailed  WorkflowRunStatus = "failed"
)

type WorkflowRun struct {
	Name    string
	Status  WorkflowRunStatus
	LastRun time.Time
	Err     string
}
```

- [ ] **Step 2: Verify the package compiles**

```bash
go build ./pkg/models/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add pkg/models/models.go
git commit -m "feat(models): add TaskStatus and WorkflowRun types"
```

---

## Task 2: pkg/git — log reader and TODO scanner

**Files:**
- Create: `pkg/git/git.go`
- Create: `pkg/git/git_test.go`

- [ ] **Step 1: Write the failing tests**

Create `pkg/git/git_test.go`:

```go
package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLog(t *testing.T) {
	commits, err := ReadLog(5)
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("expected at least one commit")
	}
	c := commits[0]
	if c.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if c.Author == "" {
		t.Error("expected non-empty author")
	}
	if c.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestLatestHash(t *testing.T) {
	hash, err := LatestHash()
	if err != nil {
		t.Fatalf("LatestHash: %v", err)
	}
	if len(hash) != 40 {
		t.Errorf("expected 40-char hash, got %d chars: %q", len(hash), hash)
	}
}

func TestScanTodos(t *testing.T) {
	dir := t.TempDir()
	content := `package foo

// TODO: fix this later
func bar() {
	// FIXME: this is broken
	x := 1
	_ = x
}
`
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := ScanTodos(dir)
	if err != nil {
		t.Fatalf("ScanTodos: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(items), items)
	}
	if items[0].Line != 3 {
		t.Errorf("expected line 3, got %d", items[0].Line)
	}
	if items[1].Line != 5 {
		t.Errorf("expected line 5, got %d", items[1].Line)
	}
}

func TestScanTodos_IgnoresGitDir(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "// TODO: should be ignored\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := ScanTodos(dir)
	if err != nil {
		t.Fatalf("ScanTodos: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items from .git dir, got %d", len(items))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./pkg/git/... -v
```

Expected: `FAIL` — package does not exist yet.

- [ ] **Step 3: Create `pkg/git/git.go`**

```go
package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Commit struct {
	Hash    string
	Author  string
	Date    string
	Message string
}

// ReadLog returns the last n commits from the current git repo.
func ReadLog(n int) ([]Commit, error) {
	out, err := exec.Command("git", "log",
		fmt.Sprintf("-n%d", n),
		"--format=%H|%an|%ad|%s",
		"--date=short",
	).Output()
	if err != nil {
		return nil, err
	}

	var commits []Commit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}
		commits = append(commits, Commit{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    parts[2],
			Message: parts[3],
		})
	}
	return commits, nil
}

// LatestHash returns the SHA of HEAD.
func LatestHash() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type TodoItem struct {
	File string
	Line int
	Text string
}

var scannedExtensions = map[string]bool{
	".go": true, ".ts": true, ".js": true, ".py": true,
	".rs": true, ".java": true, ".cpp": true, ".c": true,
}

// ScanTodos walks root and returns all TODO/FIXME comments in source files.
func ScanTodos(root string) ([]TodoItem, error) {
	var items []TodoItem
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !scannedExtensions[filepath.Ext(path)] {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			upper := strings.ToUpper(line)
			if strings.Contains(upper, "TODO") || strings.Contains(upper, "FIXME") {
				rel, _ := filepath.Rel(root, path)
				items = append(items, TodoItem{
					File: rel,
					Line: lineNum,
					Text: strings.TrimSpace(line),
				})
			}
		}
		return nil
	})
	return items, err
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./pkg/git/... -v
```

Expected: all tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add pkg/git/
git commit -m "feat(git): add ReadLog, LatestHash, ScanTodos"
```

---

## Task 3: pkg/scheduler — time-based job scheduler

**Files:**
- Create: `pkg/scheduler/scheduler.go`
- Create: `pkg/scheduler/scheduler_test.go`

- [ ] **Step 1: Write failing tests**

Create `pkg/scheduler/scheduler_test.go`:

```go
package scheduler

import (
	"testing"
	"time"
)

func TestShouldFire_Daily(t *testing.T) {
	job := Job{Name: "test", Hour: 9, Minute: 0}

	at9 := time.Date(2026, 1, 1, 9, 0, 30, 0, time.UTC)
	if !shouldFire(job, at9, time.Time{}) {
		t.Error("expected to fire at 09:00")
	}

	at901 := time.Date(2026, 1, 1, 9, 1, 0, 0, time.UTC)
	if shouldFire(job, at901, time.Time{}) {
		t.Error("should not fire at 09:01")
	}

	// must not fire twice in same minute
	if shouldFire(job, at9, at9.Add(-10*time.Second)) {
		t.Error("should not fire twice in same minute")
	}
}

func TestShouldFire_Weekly(t *testing.T) {
	monday := time.Monday
	job := Job{Name: "weekly", Hour: 9, Minute: 0, Weekday: &monday}

	// 2026-04-27 is a Monday
	mon := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	if !shouldFire(job, mon, time.Time{}) {
		t.Error("expected to fire on Monday at 09:00")
	}

	// 2026-04-28 is a Tuesday
	tue := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	if shouldFire(job, tue, time.Time{}) {
		t.Error("should not fire on Tuesday")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./pkg/scheduler/... -v
```

Expected: `FAIL` — package does not exist.

- [ ] **Step 3: Create `pkg/scheduler/scheduler.go`**

```go
package scheduler

import (
	"context"
	"time"
)

type Job struct {
	Name    string
	Hour    int
	Minute  int
	Weekday *time.Weekday // nil = every day
	Fn      func(context.Context)
}

type Scheduler struct {
	jobs []Job
}

func New() *Scheduler {
	return &Scheduler{}
}

func (s *Scheduler) Add(j Job) {
	s.jobs = append(s.jobs, j)
}

// Run checks every 30 seconds whether any job should fire.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	fired := map[string]time.Time{}

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			for _, job := range s.jobs {
				if shouldFire(job, t, fired[job.Name]) {
					fired[job.Name] = t
					go job.Fn(ctx)
				}
			}
		}
	}
}

func shouldFire(job Job, now time.Time, lastFired time.Time) bool {
	if now.Hour() != job.Hour || now.Minute() != job.Minute {
		return false
	}
	if job.Weekday != nil && now.Weekday() != *job.Weekday {
		return false
	}
	if !lastFired.IsZero() && now.Sub(lastFired) < time.Minute {
		return false
	}
	return true
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./pkg/scheduler/... -v
```

Expected: all tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add pkg/scheduler/
git commit -m "feat(scheduler): add time-based job scheduler"
```

---

## Task 4: Add go-github dependency

**Files:**
- Modify: `go.mod`, `go.sum` (via go get)

- [ ] **Step 1: Add go-github**

```bash
go get github.com/google/go-github/v66@latest
```

Expected: output ending in `go: added github.com/google/go-github/v66 ...`

- [ ] **Step 2: Tidy**

```bash
go mod tidy
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add go-github/v66 dependency"
```

---

## Task 5: pkg/ghclient — optional GitHub API client

**Files:**
- Create: `pkg/ghclient/ghclient.go`

No unit tests — GitHub API requires real credentials. Tested end-to-end in Task 12.

- [ ] **Step 1: Create `pkg/ghclient/ghclient.go`**

```go
package ghclient

import (
	"context"
	"os"
	"strings"

	gh "github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

type Client struct {
	client *gh.Client
	owner  string
	repo   string
}

// New returns a Client if GITHUB_TOKEN and GITHUB_REPO are both set; otherwise nil.
// Callers must nil-check before use.
func New() *Client {
	token := os.Getenv("GITHUB_TOKEN")
	repoStr := os.Getenv("GITHUB_REPO") // "owner/repo"
	if token == "" || repoStr == "" {
		return nil
	}
	parts := strings.SplitN(repoStr, "/", 2)
	if len(parts) != 2 {
		return nil
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		client: gh.NewClient(tc),
		owner:  parts[0],
		repo:   parts[1],
	}
}

// CreateIssue opens a new GitHub Issue. No-ops if c is nil.
func (c *Client) CreateIssue(ctx context.Context, title, body string, labels []string) error {
	if c == nil {
		return nil
	}
	_, _, err := c.client.Issues.Create(ctx, c.owner, c.repo, &gh.IssueRequest{
		Title:  gh.Ptr(title),
		Body:   gh.Ptr(body),
		Labels: &labels,
	})
	return err
}

// CommentOnLabeledIssue finds the first open issue with label and appends body as a comment.
// Creates the issue with title if none found. No-ops if c is nil.
func (c *Client) CommentOnLabeledIssue(ctx context.Context, label, title, body string) error {
	if c == nil {
		return nil
	}
	issues, _, err := c.client.Issues.ListByRepo(ctx, c.owner, c.repo, &gh.IssueListByRepoOptions{
		Labels: []string{label},
		State:  "open",
	})
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		_, _, err = c.client.Issues.CreateComment(ctx, c.owner, c.repo,
			*issues[0].Number,
			&gh.IssueComment{Body: gh.Ptr(body)},
		)
		return err
	}
	return c.CreateIssue(ctx, title, body, []string{label})
}
```

- [ ] **Step 2: Build to verify it compiles**

```bash
go build ./pkg/ghclient/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add pkg/ghclient/
git commit -m "feat(ghclient): add optional GitHub API client"
```

---

## Task 6: internal/agent — add scan_todos, read_git_log, post_github_issue tools

**Files:**
- Modify: `internal/agent/agent.go`

- [ ] **Step 1: Add `GitHub` field to `PMAgent` and update `NewPMAgent` signature**

Replace the struct and constructor in `internal/agent/agent.go`:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
	"pm-agent/pkg/ghclient"
	"pm-agent/pkg/git"
)

type PMAgent struct {
	LLM     *googleai.GoogleAI
	History []llms.MessageContent
	Tools   []llms.Tool
	GitHub  *ghclient.Client
}

func NewPMAgent(ctx context.Context, projectContext string, github *ghclient.Client) (*PMAgent, error) {
```

- [ ] **Step 2: Store the GitHub client and add three new tool definitions**

Inside `NewPMAgent`, after building the existing `tools` slice (after `generate_prd`), append:

```go
	tools = append(tools,
		llms.Tool{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        "scan_todos",
				Description: "Scan the project source files for TODO and FIXME comments",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		llms.Tool{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        "read_git_log",
				Description: "Read the last N git commits with author, date, and message",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"n": map[string]any{"type": "integer", "description": "Number of commits to fetch (max 50)"},
					},
					"required": []string{"n"},
				},
			},
		},
		llms.Tool{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        "post_github_issue",
				Description: "Create a GitHub Issue (requires GITHUB_TOKEN and GITHUB_REPO env vars)",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title":  map[string]any{"type": "string"},
						"body":   map[string]any{"type": "string"},
						"labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
					"required": []string{"title", "body"},
				},
			},
		},
	)

	return &PMAgent{
		LLM:     llm,
		History: history,
		Tools:   tools,
		GitHub:  github,
	}, nil
```

- [ ] **Step 3: Add cases to `executeTool`**

Append to the `switch` in `executeTool`:

```go
	case "scan_todos":
		items, err := git.ScanTodos(".")
		if err != nil {
			return fmt.Sprintf("Error scanning TODOs: %v", err), nil
		}
		if len(items) == 0 {
			return "No TODOs or FIXMEs found.", nil
		}
		var sb strings.Builder
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("%s:%d: %s\n", item.File, item.Line, item.Text))
		}
		return sb.String(), nil

	case "read_git_log":
		var args struct {
			N int `json:"n"`
		}
		json.Unmarshal([]byte(tc.FunctionCall.Arguments), &args)
		if args.N <= 0 || args.N > 50 {
			args.N = 10
		}
		commits, err := git.ReadLog(args.N)
		if err != nil {
			return fmt.Sprintf("Error reading git log: %v", err), nil
		}
		var sb strings.Builder
		for _, c := range commits {
			sb.WriteString(fmt.Sprintf("[%s] %s (%s): %s\n", c.Hash[:7], c.Author, c.Date, c.Message))
		}
		return sb.String(), nil

	case "post_github_issue":
		var args struct {
			Title  string   `json:"title"`
			Body   string   `json:"body"`
			Labels []string `json:"labels"`
		}
		json.Unmarshal([]byte(tc.FunctionCall.Arguments), &args)
		if err := a.GitHub.CreateIssue(context.Background(), args.Title, args.Body, args.Labels); err != nil {
			return fmt.Sprintf("Error creating GitHub issue: %v", err), nil
		}
		return fmt.Sprintf("Success! Created GitHub issue: %s", args.Title), nil
```

- [ ] **Step 4: Build to verify it compiles**

```bash
go build ./internal/agent/...
```

Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/agent.go
git commit -m "feat(agent): add scan_todos, read_git_log, post_github_issue tools"
```

---

## Task 7: internal/workflows/grooming — commit-triggered backlog grooming

**Files:**
- Create: `internal/workflows/grooming.go`

- [ ] **Step 1: Create `internal/workflows/grooming.go`**

```go
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
```

- [ ] **Step 2: Build to verify it compiles**

```bash
go build ./internal/workflows/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/workflows/grooming.go
git commit -m "feat(workflows): add backlog grooming workflow"
```

---

## Task 8: internal/workflows/standup — daily standup summary

**Files:**
- Create: `internal/workflows/standup.go`

- [ ] **Step 1: Create `internal/workflows/standup.go`**

```go
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
```

- [ ] **Step 2: Build to verify it compiles**

```bash
go build ./internal/workflows/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/workflows/standup.go
git commit -m "feat(workflows): add daily standup workflow"
```

---

## Task 9: internal/workflows/stakeholder — weekly stakeholder update

**Files:**
- Create: `internal/workflows/stakeholder.go`

- [ ] **Step 1: Create `internal/workflows/stakeholder.go`**

```go
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
```

- [ ] **Step 2: Build to verify it compiles**

```bash
go build ./internal/workflows/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/workflows/stakeholder.go
git commit -m "feat(workflows): add weekly stakeholder update workflow"
```

---

## Task 10: internal/daemon — background orchestrator

**Files:**
- Create: `internal/daemon/daemon.go`

- [ ] **Step 1: Create `internal/daemon/daemon.go`**

```go
package daemon

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"pm-agent/internal/workflows"
	"pm-agent/pkg/git"
	"pm-agent/pkg/models"
	"pm-agent/pkg/scheduler"
)

type Daemon struct {
	grooming    *workflows.GroomingWorkflow
	standup     *workflows.StandupWorkflow
	stakeholder *workflows.StakeholderWorkflow
	sched       *scheduler.Scheduler
	Events      chan models.WorkflowRun
	lastHash    string
}

func New(
	g *workflows.GroomingWorkflow,
	st *workflows.StandupWorkflow,
	sh *workflows.StakeholderWorkflow,
) *Daemon {
	d := &Daemon{
		grooming:    g,
		standup:     st,
		stakeholder: sh,
		sched:       scheduler.New(),
		Events:      make(chan models.WorkflowRun, 20),
	}

	stHour, stMin := parseTime(os.Getenv("STANDUP_TIME"), 9, 0)
	d.sched.Add(scheduler.Job{
		Name:   "standup",
		Hour:   stHour,
		Minute: stMin,
		Fn:     d.runStandup,
	})

	monday := time.Monday
	d.sched.Add(scheduler.Job{
		Name:    "stakeholder",
		Hour:    9,
		Minute:  0,
		Weekday: &monday,
		Fn:      d.runStakeholder,
	})

	return d
}

// Run starts the scheduler and the git polling loop. Blocks until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) {
	go d.sched.Run(ctx)

	hash, _ := git.LatestHash()
	d.lastHash = hash

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.checkNewCommit(ctx)
		}
	}
}

func (d *Daemon) checkNewCommit(ctx context.Context) {
	hash, err := git.LatestHash()
	if err != nil || hash == d.lastHash {
		return
	}
	d.lastHash = hash
	go d.runGrooming(ctx)
}

func (d *Daemon) runGrooming(ctx context.Context) {
	d.emit("grooming", models.WorkflowRunning, "")
	if err := d.grooming.Run(ctx); err != nil {
		log.Printf("grooming workflow error: %v", err)
		d.emit("grooming", models.WorkflowFailed, err.Error())
		return
	}
	d.emit("grooming", models.WorkflowDone, "")
}

func (d *Daemon) runStandup(ctx context.Context) {
	d.emit("standup", models.WorkflowRunning, "")
	if err := d.standup.Run(ctx); err != nil {
		log.Printf("standup workflow error: %v", err)
		d.emit("standup", models.WorkflowFailed, err.Error())
		return
	}
	d.emit("standup", models.WorkflowDone, "")
}

func (d *Daemon) runStakeholder(ctx context.Context) {
	d.emit("stakeholder", models.WorkflowRunning, "")
	if err := d.stakeholder.Run(ctx); err != nil {
		log.Printf("stakeholder workflow error: %v", err)
		d.emit("stakeholder", models.WorkflowFailed, err.Error())
		return
	}
	d.emit("stakeholder", models.WorkflowDone, "")
}

func (d *Daemon) emit(name string, status models.WorkflowRunStatus, errMsg string) {
	select {
	case d.Events <- models.WorkflowRun{
		Name:    name,
		Status:  status,
		LastRun: time.Now(),
		Err:     errMsg,
	}:
	default:
		// channel full — drop event rather than block
	}
}

func parseTime(s string, defaultHour, defaultMinute int) (int, int) {
	if s == "" {
		return defaultHour, defaultMinute
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return defaultHour, defaultMinute
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return defaultHour, defaultMinute
	}
	return h, m
}
```

- [ ] **Step 2: Build to verify it compiles**

```bash
go build ./internal/daemon/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/daemon/daemon.go
git commit -m "feat(daemon): add background workflow orchestrator"
```

---

## Task 11: internal/app + internal/ui — WorkflowView

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/ui/ui.go`

- [ ] **Step 1: Update `internal/app/app.go`**

Replace the full file:

```go
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
```

- [ ] **Step 2: Add `WorkflowView` rendering to `internal/ui/ui.go`**

Add the following function at the bottom of the file:

```go
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
```

Also update the `View` function's `content` assignment to handle `WorkflowView`:

```go
	if m.Mode == app.ChatView {
		content = renderChat(m)
	} else if m.Mode == app.BacklogView {
		content = renderBacklog(m)
	} else {
		content = renderWorkflows(m)
	}
```

Add the missing import at the top of `ui.go`:

```go
import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"pm-agent/internal/app"
	"pm-agent/pkg/models"
)
```

- [ ] **Step 3: Build to verify it compiles**

```bash
go build ./internal/...
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add internal/app/app.go internal/ui/ui.go
git commit -m "feat(ui): add WorkflowView and WorkflowRuns state"
```

---

## Task 12: main.go — wire daemon to TUI

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Replace `main.go` with the wired version**

```go
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
```

- [ ] **Step 2: Build the full binary**

```bash
go build ./...
```

Expected: no output (success).

- [ ] **Step 3: Run and verify the TUI starts**

```bash
go run main.go
```

Expected: TUI opens. Tab now cycles Chat → Backlog → Workflows. Workflows panel shows all three workflows as "idle".

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add main.go
git commit -m "feat: wire autonomous daemon to TUI — grooming, standup, stakeholder live"
```

---

## Self-Review Checklist

- [x] **Spec coverage**: backlog grooming (Tasks 7, 10, 12), standup (Tasks 8, 10, 12), stakeholder (Tasks 9, 10, 12), GitHub optional (Tasks 5, 7-9), WorkflowView (Task 11), daemon (Task 10), models (Task 1) — all covered
- [x] **No placeholders**: all code complete
- [x] **Type consistency**: `models.WorkflowRun` used throughout; `models.WorkflowRunStatus` constants referenced in daemon and ui; `AddTaskArgs.Status` used in stakeholder workflow
- [x] **`NewPMAgent` signature change**: updated in Task 6, consumed in Task 12 `main.go`
- [x] **Variable shadowing fix**: `sh` parameter name in `daemon.New` conflicted with `sh, sm := parseTime(...)` — fixed to `stHour, stMin` in the plan above.

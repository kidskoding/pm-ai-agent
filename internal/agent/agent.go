package agent

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
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}

	llm, err := googleai.New(ctx, googleai.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}

	preamble := fmt.Sprintf(`You are an Elite Autonomous Product Manager. 
You are currently managing a Go project with the following structure:
%s

Your capabilities:
1. CREATE_TASK: Add items to the JSON backlog.
2. DELETE_TASK: Remove items from the backlog by their exact title.
3. READ_FILE: Inspect actual source code to give precise technical PM advice.
4. GENERATE_PRD: Write deep, professional Product Requirement Documents in Markdown.
5. SCAN_TODOS: Look for TODO/FIXME in the source code.
6. READ_GIT_LOG: See what the engineers have been shipping.
7. POST_GITHUB_ISSUE: Escalate tasks to GitHub Issues.

Strategic Mandate:
- Don't just answer questions; proactively manage the project.
- If you notice a file is missing or a feature is underspecified, READ the relevant code and propose a TASK.
- Use tools to maintain a 'Single Source of Truth' in the 'backlog/' and 'docs/prd/' directories.`, projectContext)

	history := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, preamble),
	}

	tools := []llms.Tool{
		{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        "create_task",
				Description: "Create a new task in the project backlog",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title":      map[string]any{"type": "string"},
						"user_story": map[string]any{"type": "string"},
						"priority":   map[string]any{"type": "string", "enum": []string{"LOW", "MEDIUM", "HIGH"}},
					},
					"required": []string{"title", "user_story", "priority"},
				},
			},
		},
		{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        "delete_task",
				Description: "Remove a task from the backlog by its title",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title": map[string]any{"type": "string", "description": "Exact title of the task to delete"},
					},
					"required": []string{"title"},
				},
			},
		},
		{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        "read_file",
				Description: "Read the content of a file in the project to understand the code",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string", "description": "Relative path to the file"},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        "generate_prd",
				Description: "Generate a comprehensive PRD in Markdown",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"filename": map[string]any{"type": "string", "description": "Name of the file (e.g. auth_service.md)"},
						"content":  map[string]any{"type": "string", "description": "Full Markdown content of the PRD"},
					},
					"required": []string{"filename", "content"},
				},
			},
		},
		{
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
		{
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
		{
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
	}

	return &PMAgent{
		LLM:     llm,
		History: history,
		Tools:   tools,
		GitHub:  github,
	}, nil
}

func (a *PMAgent) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	a.History = append(a.History, llms.TextParts(llms.ChatMessageTypeGeneric, prompt))

	resp, err := a.LLM.GenerateContent(ctx, a.History,
		llms.WithModel("gemini-2.0-flash"),
		llms.WithTools(a.Tools),
	)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from agent")
	}

	choice := resp.Choices[0]

	if len(choice.ToolCalls) > 0 {
		var toolResults []string
		for _, tc := range choice.ToolCalls {
			result, err := a.executeTool(tc)
			if err != nil {
				return "", err
			}
			toolResults = append(toolResults, result)
		}
		
		// Combine tool results and return
		finalMsg := strings.Join(toolResults, "\n")
		a.History = append(a.History, llms.TextParts(llms.ChatMessageTypeAI, finalMsg))
		return finalMsg, nil
	}

	responseText := choice.Content
	a.History = append(a.History, llms.TextParts(llms.ChatMessageTypeAI, responseText))

	return responseText, nil
}

func (a *PMAgent) executeTool(tc llms.ToolCall) (string, error) {
	switch tc.FunctionCall.Name {
	case "create_task":
		var args struct {
			Title     string `json:"title"`
			UserStory string `json:"user_story"`
			Priority  string `json:"priority"`
		}
		json.Unmarshal([]byte(tc.FunctionCall.Arguments), &args)
		path := filepath.Join("backlog", fmt.Sprintf("task_%d.json", time.Now().UnixNano()))
		content, _ := json.MarshalIndent(args, "", "  ")
		os.WriteFile(path, content, 0644)
		return fmt.Sprintf("Success! Created task: %s", args.Title), nil

	case "delete_task":
		var args struct{ Title string `json:"title"` }
		json.Unmarshal([]byte(tc.FunctionCall.Arguments), &args)
		// Simple search and delete
		files, _ := os.ReadDir("backlog")
		for _, f := range files {
			data, _ := os.ReadFile(filepath.Join("backlog", f.Name()))
			if strings.Contains(string(data), args.Title) {
				os.Remove(filepath.Join("backlog", f.Name()))
				return fmt.Sprintf("Success! Deleted task: %s", args.Title), nil
			}
		}
		return fmt.Sprintf("Task '%s' not found.", args.Title), nil

	case "read_file":
		var args struct{ Path string `json:"path"` }
		json.Unmarshal([]byte(tc.FunctionCall.Arguments), &args)
		data, err := os.ReadFile(args.Path)
		if err != nil {
			return fmt.Sprintf("Error reading file: %v", err), nil
		}
		return fmt.Sprintf("File content of %s:\n\n%s", args.Path, string(data)), nil

	case "generate_prd":
		var args struct {
			Filename string `json:"filename"`
			Content  string `json:"content"`
		}
		json.Unmarshal([]byte(tc.FunctionCall.Arguments), &args)
		path := filepath.Join("docs", "prd", args.Filename)
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(args.Content), 0644)
		return fmt.Sprintf("Success! Generated PRD: %s", path), nil

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
	}

	return "Unknown tool", nil
}

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
		Title:  gh.String(title),
		Body:   gh.String(body),
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
			&gh.IssueComment{Body: gh.String(body)},
		)
		return err
	}
	return c.CreateIssue(ctx, title, body, []string{label})
}

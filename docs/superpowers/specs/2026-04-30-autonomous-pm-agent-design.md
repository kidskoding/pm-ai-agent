# Autonomous PM Agent — Design Spec

**Date:** 2026-04-30
**Status:** Approved

## Problem

Small engineering teams (2–5 engineers, no dedicated PM) have no one grooming their backlog, writing standup summaries, or keeping stakeholders informed. Existing tools (Linear AI, Notion AI, Jira AI) are reactive chatbots — you prompt them, they generate text. None read your actual code.

## Pitch

*"The first PM agent that reads your code, not just your tickets."*

## Target User

Small engineering teams who can't afford a dedicated PM.

## Approach

Hybrid: local code-reading brain + GitHub as output surface. Single binary — TUI for display, background daemon goroutine for autonomous work. GitHub is optional and gracefully degrades without a token.

---

## Architecture

Two goroutines in one binary:

```
┌─────────────────────────── single binary ───────────────────────────┐
│                                                                       │
│   ┌─────────────┐        ┌──────────────────────────────────────┐   │
│   │     TUI     │◄──────►│           Daemon (goroutine)          │   │
│   │  (display)  │ events │  ┌──────────┐  ┌───────────────────┐ │   │
│   └─────────────┘        │  │Git Watch │  │Scheduler          │ │   │
│                           │  │(fsnotify)│  │standup: daily 9am │ │   │
│                           │  │on commit │  │stakeholder: weekly│ │   │
│                           │  └────┬─────┘  └────────┬──────────┘ │   │
│                           │       └────────┬──────────┘           │   │
│                           │          ┌─────▼──────┐               │   │
│                           │          │  Workflow  │               │   │
│                           │          │   Engine   │               │   │
│                           │          └─────┬──────┘               │   │
│                           └────────────────┼─────────────────────┘   │
│                                            │                          │
│                           ┌────────────────┼──────────────┐          │
│                           │                │              │          │
│                      Local JSON       Local Docs     GitHub Issues   │
│                      backlog/         docs/          (optional)      │
└───────────────────────────────────────────────────────────────────────┘
```

The TUI gains a "Workflows" panel (Tab through views) showing last run time and status for each workflow.

---

## The Three Workflows

### 1. Backlog Grooming
**Trigger:** every git commit (detected via fsnotify on `.git/refs`)

Steps:
1. Scan all source files for `TODO` / `FIXME` comments
2. Read last 10 git commits for context on what changed
3. Read existing `backlog/*.json` to avoid duplicates
4. Agent generates new tasks with title, user story, priority
5. Writes to `backlog/task_<nanoseconds>.json`
6. If `GITHUB_TOKEN` set: creates corresponding GitHub Issues

### 2. Daily Standup Summary
**Trigger:** scheduler at 9am daily

Steps:
1. Read git log for last 24h across all local branches
2. Group commits by author name
3. Agent writes human-readable summary per engineer ("worked on X, merged Y, no blockers noted")
4. Saves to `docs/standups/YYYY-MM-DD.md`
5. If `GITHUB_TOKEN` set: posts as a comment on a GitHub Issue labeled `standup-log` (creates it if it doesn't exist)

### 3. Weekly Stakeholder Update
**Trigger:** scheduler every Monday at 9am

Steps:
1. Read git log for last 7 days
2. Read backlog: tasks grouped by `Status` field (`pending` / `completed`)
3. Read any PRDs in `docs/prd/`
4. Agent writes non-technical digest ("Team shipped X, currently working on Y, Z is at risk")
5. Saves to `docs/updates/YYYY-MM-DD.md`
6. If `GITHUB_TOKEN` set: creates a new GitHub Issue tagged `stakeholder-update`

---

## Package Structure

### New packages
```
internal/daemon/        goroutine entrypoint; owns scheduler + git watcher; sends WorkflowEvent to TUI via channel
internal/workflows/     one file per workflow
  grooming.go
  standup.go
  stakeholder.go
pkg/git/                reads git log, diffs, scans TODOs from source files
pkg/github/             GitHub API client (go-github); no-ops gracefully if GITHUB_TOKEN unset
pkg/scheduler/          lightweight cron: runs callbacks at daily/weekly intervals
```

### Modified packages
```
internal/agent/         add new tools: scan_todos, read_git_log, post_github_issue
internal/app/           add WorkflowStatus to Model (last run time + result per workflow)
internal/ui/            add WorkflowView (third Tab view)
pkg/models/             add WorkflowRun, WorkflowStatus types; add Status field to Task (pending/completed)
```

---

## New Agent Tools

| Tool | Description |
|---|---|
| `scan_todos` | Scan source files for TODO/FIXME, return list with file+line |
| `read_git_log` | Read last N commits with author, message, files changed |
| `post_github_issue` | Create a GitHub Issue with title, body, labels |

---

## Environment Variables

| Variable | Required | Purpose |
|---|---|---|
| `GEMINI_API_KEY` | Yes | LLM |
| `GITHUB_TOKEN` | No | GitHub sync (degrades gracefully) |
| `GITHUB_REPO` | No | `owner/repo` format, required if token set |
| `STANDUP_TIME` | No | Override daily standup time (default `09:00`) |

---

## Out of Scope (MVP)

- Sprint planning (requires business priority context the agent can't know)
- Acceptance criteria generation
- Linear / Jira integration
- Multi-repo support
- Web dashboard

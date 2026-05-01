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

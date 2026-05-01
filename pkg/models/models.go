package models

type Priority string

const (
	Low    Priority = "LOW"
	Medium Priority = "MEDIUM"
	High   Priority = "HIGH"
)

type AddTaskArgs struct {
	Title     string   `json:"title"`
	UserStory string   `json:"user_story"`
	Priority  Priority `json:"priority"`
}

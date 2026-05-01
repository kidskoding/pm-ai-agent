package backlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"pm-agent/pkg/models"
)

// LoadBacklog reads all JSON files in the backlog directory and returns a slice of tasks
func LoadBacklog() ([]models.AddTaskArgs, error) {
	dir := "backlog"
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.AddTaskArgs{}, nil
		}
		return nil, err
	}

	var tasks []models.AddTaskArgs
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(dir, file.Name()))
			if err != nil {
				continue
			}

			var task models.AddTaskArgs
			if err := json.Unmarshal(data, &task); err == nil {
				tasks = append(tasks, task)
			}
		}
	}

	return tasks, nil
}

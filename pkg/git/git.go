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

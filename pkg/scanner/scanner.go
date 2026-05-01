package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectContext holds the summary of the codebase
type ProjectContext struct {
	Structure string
	FileCount int
}

// ScanProject crawls the current directory and returns a structural summary
func ScanProject(root string) (*ProjectContext, error) {
	var sb strings.Builder
	fileCount := 0

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ignore common noisy directories
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "target" || name == "node_modules" || name == ".gemini" {
				return filepath.SkipDir
			}
		}

		// Only include files we care about (go files, md, etc)
		relPath, _ := filepath.Rel(root, path)
		if relPath == "." {
			return nil
		}

		if info.IsDir() {
			sb.WriteString(fmt.Sprintf("Directory: %s\n", relPath))
		} else {
			fileCount++
			sb.WriteString(fmt.Sprintf("File: %s\n", relPath))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &ProjectContext{
		Structure: sb.String(),
		FileCount: fileCount,
	}, nil
}

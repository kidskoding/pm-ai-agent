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

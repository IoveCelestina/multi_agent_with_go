package readfile

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteReadsFileWithOffsetAndLimit(t *testing.T) {
	root := tempDir(t)
	path := filepath.Join(root, "note.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tool, err := New([]string{root}, 64)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	args := mustJSON(t, map[string]any{
		"path":   path,
		"offset": 6,
		"limit":  5,
	})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("Execute() returned tool error: %s", result.Content)
	}
	if result.Content != "world" {
		t.Fatalf("content = %q, want world", result.Content)
	}
}

func TestExecuteRejectsPathOutsideRoot(t *testing.T) {
	base := tempDir(t)
	root := filepath.Join(base, "repo")
	sibling := filepath.Join(base, "repo2")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatalf("Mkdir(root) error = %v", err)
	}
	if err := os.Mkdir(sibling, 0o700); err != nil {
		t.Fatalf("Mkdir(sibling) error = %v", err)
	}

	outsidePath := filepath.Join(sibling, "secret.txt")
	if err := os.WriteFile(outsidePath, []byte("secret"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tool, err := New([]string{root}, 64)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"path": outsidePath}))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.IsError {
		t.Fatalf("Execute() IsError = false, content = %q", result.Content)
	}
	if !strings.Contains(result.Content, "outside allowed roots") {
		t.Fatalf("error content = %q, want outside allowed roots", result.Content)
	}
}

func TestExecuteRejectsMissingPathWithoutFallback(t *testing.T) {
	root := tempDir(t)
	tool, err := New([]string{root}, 64)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	missing := filepath.Join(root, "missing.txt")
	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"path": missing}))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.IsError {
		t.Fatalf("Execute() IsError = false, content = %q", result.Content)
	}
	if !strings.Contains(result.Content, "not accessible") {
		t.Fatalf("error content = %q, want not accessible", result.Content)
	}
}

func TestExecuteRejectsLimitAboveMaxBytes(t *testing.T) {
	root := tempDir(t)
	path := filepath.Join(root, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tool, err := New([]string{root}, 4)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"path":  path,
		"limit": 5,
	}))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.IsError {
		t.Fatalf("Execute() IsError = false, content = %q", result.Content)
	}
	if !strings.Contains(result.Content, "limit exceeds max_bytes") {
		t.Fatalf("error content = %q, want limit exceeds max_bytes", result.Content)
	}
}

func TestNewFailsWhenRootCannotEvalSymlinks(t *testing.T) {
	missingRoot := filepath.Join(tempDir(t), "missing")
	if _, err := New([]string{missingRoot}, 64); err == nil {
		t.Fatal("New() error = nil")
	}
}

func tempDir(t *testing.T) string {
	t.Helper()

	parent := filepath.Join(".", ".testtmp")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	dir, err := os.MkdirTemp(parent, "readfile-")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	t.Cleanup(func() {
		if err := os.RemoveAll(abs); err != nil {
			t.Fatalf("RemoveAll() error = %v", err)
		}
	})

	return abs
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()

	b, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	return b
}

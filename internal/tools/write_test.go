package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeToolArgs(t *testing.T, path, content string) json.RawMessage {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"path": path, "content": content})
	return b
}

func TestWriteTool_NewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.txt")
	tool := NewWriteTool(true) // unsafe=true: skip confirmation

	got, err := tool.Execute(context.Background(), writeToolArgs(t, path, "hello luna\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "wrote") {
		t.Errorf("expected 'wrote' in result, got %q", got)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello luna\n" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestWriteTool_Overwrite_Unsafe(t *testing.T) {
	path := writeTemp(t, "old content\n")
	tool := NewWriteTool(true)

	got, err := tool.Execute(context.Background(), writeToolArgs(t, path, "new content\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "overwrote") {
		t.Errorf("expected 'overwrote' in result, got %q", got)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "new content\n" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestWriteTool_CreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", "file.txt")
	tool := NewWriteTool(true)

	_, err := tool.Execute(context.Background(), writeToolArgs(t, path, "deep\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created at %s: %v", path, err)
	}
}

func TestWriteTool_EmptyPath(t *testing.T) {
	tool := NewWriteTool(true)
	b, _ := json.Marshal(map[string]string{"path": "", "content": "x"})
	_, err := tool.Execute(context.Background(), b)
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestWriteTool_EmptyContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.txt")
	tool := NewWriteTool(true)

	_, err := tool.Execute(context.Background(), writeToolArgs(t, path, ""))
	if err != nil {
		t.Fatalf("unexpected error writing empty content: %v", err)
	}

	data, _ := os.ReadFile(path)
	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}

func TestWriteTool_InvalidJSON(t *testing.T) {
	tool := NewWriteTool(true)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestWriteTool_ReportsByteCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "count.txt")
	content := "hello"
	tool := NewWriteTool(true)

	got, err := tool.Execute(context.Background(), writeToolArgs(t, path, content))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "5") {
		t.Errorf("expected byte count '5' in result, got %q", got)
	}
}

func TestLineCount(t *testing.T) {
	tests := []struct {
		content string
		want    int
	}{
		{"a\nb\nc\n", 4},   // trailing newline adds +1
		{"single", 1},
		{"a\nb", 2},
		{"", 1},
	}
	for _, tc := range tests {
		path := writeTemp(t, tc.content)
		got := lineCount(path)
		if got != tc.want {
			t.Errorf("lineCount(%q) = %d, want %d", tc.content, got, tc.want)
		}
	}
}

func TestLineCount_NonExistent(t *testing.T) {
	got := lineCount("/does/not/exist.txt")
	if got != 0 {
		t.Errorf("expected 0 for non-existent file, got %d", got)
	}
}

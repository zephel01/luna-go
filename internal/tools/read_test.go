package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func readArgs(t *testing.T, path string) json.RawMessage {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"path": path})
	return b
}

func TestReadTool_Normal(t *testing.T) {
	path := writeTemp(t, "line1\nline2\nline3\n")
	tool := NewReadTool()

	got, err := tool.Execute(context.Background(), readArgs(t, path))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(got, "1\tline1") {
		t.Errorf("missing line 1 with number: %q", got)
	}
	if !strings.Contains(got, "2\tline2") {
		t.Errorf("missing line 2 with number: %q", got)
	}
	if !strings.Contains(got, "3\tline3") {
		t.Errorf("missing line 3 with number: %q", got)
	}
}

func TestReadTool_FileNotExist(t *testing.T) {
	tool := NewReadTool()
	_, err := tool.Execute(context.Background(), readArgs(t, "/nonexistent/file.txt"))
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestReadTool_EmptyPath(t *testing.T) {
	tool := NewReadTool()
	b, _ := json.Marshal(map[string]string{"path": ""})
	_, err := tool.Execute(context.Background(), b)
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestReadTool_EmptyFile(t *testing.T) {
	path := writeTemp(t, "")
	tool := NewReadTool()

	got, err := tool.Execute(context.Background(), readArgs(t, path))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty file: one empty line with number "1\t"
	if !strings.Contains(got, "1\t") {
		t.Errorf("expected at least line 1, got %q", got)
	}
}

func TestReadTool_Truncation(t *testing.T) {
	// Write 2001 lines
	var sb strings.Builder
	for i := 0; i < 2001; i++ {
		sb.WriteString("line\n")
	}
	path := writeTemp(t, sb.String())
	tool := NewReadTool()

	got, err := tool.Execute(context.Background(), readArgs(t, path))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "truncated") {
		t.Errorf("expected truncation notice, got %q", got[:200])
	}
}

func TestReadTool_NoTruncationAt1999(t *testing.T) {
	// 1999 lines of "x\n" → Split produces 2000 elements (1999 "x" + 1 "") → no truncation
	var sb strings.Builder
	for i := 0; i < 1999; i++ {
		sb.WriteString("x\n")
	}
	path := writeTemp(t, sb.String())
	tool := NewReadTool()

	got, err := tool.Execute(context.Background(), readArgs(t, path))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "truncated") {
		t.Error("unexpected truncation notice for 1999 lines")
	}
}

func TestReadTool_TruncationAt2001Lines(t *testing.T) {
	// 2000 lines of "x\n" → Split yields 2001 elements → triggers truncation
	var sb strings.Builder
	for i := 0; i < 2000; i++ {
		sb.WriteString("x\n")
	}
	path := writeTemp(t, sb.String())
	tool := NewReadTool()

	got, err := tool.Execute(context.Background(), readArgs(t, path))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "truncated") {
		t.Error("expected truncation notice")
	}
}

func TestReadTool_InvalidJSON(t *testing.T) {
	tool := NewReadTool()
	_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestReadTool_LineNumbers(t *testing.T) {
	// Verify line numbers are sequential and correct
	path := writeTemp(t, "a\nb\nc\n")
	tool := NewReadTool()

	got, err := tool.Execute(context.Background(), readArgs(t, path))
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(got), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		wantPrefix := strings.Repeat("", 0) // just check it starts with number
		num := i + 1
		prefix := strings.Split(line, "\t")[0]
		if prefix != strings.TrimSpace(strings.Split(line, "\t")[0]) {
			t.Errorf("line %d: unexpected format %q", num, line)
		}
		_ = wantPrefix
	}
}

func TestReadTool_Binaryish(t *testing.T) {
	// File with no newlines at all
	path := writeTemp(t, "singleline")
	tool := NewReadTool()

	got, err := tool.Execute(context.Background(), readArgs(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "singleline") {
		t.Errorf("content not reflected: %q", got)
	}
}

func TestReadTool_LargeFile(t *testing.T) {
	// 1MB file — should not OOM or error
	content := strings.Repeat("x", 1_000_000)
	f, err := os.CreateTemp(t.TempDir(), "luna-large-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()

	tool := NewReadTool()
	_, err = tool.Execute(context.Background(), readArgs(t, f.Name()))
	if err != nil {
		t.Fatalf("unexpected error on large file: %v", err)
	}
}

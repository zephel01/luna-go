package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "luna-edit-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func editArgs(t *testing.T, path, old, new_ string, replaceAll bool) json.RawMessage {
	t.Helper()
	b, _ := json.Marshal(map[string]any{
		"path":        path,
		"old_string":  old,
		"new_string":  new_,
		"replace_all": replaceAll,
	})
	return b
}

func TestEditTool_Normal(t *testing.T) {
	path := writeTemp(t, "hello world\n")
	tool := NewEditTool()

	got, err := tool.Execute(context.Background(), editArgs(t, path, "world", "luna", false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty result")
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello luna\n" {
		t.Errorf("file content = %q, want %q", string(data), "hello luna\n")
	}
}

func TestEditTool_NotFound(t *testing.T) {
	path := writeTemp(t, "hello world\n")
	tool := NewEditTool()

	_, err := tool.Execute(context.Background(), editArgs(t, path, "missing", "x", false))
	if err == nil {
		t.Fatal("expected error when old_string not found")
	}
}

func TestEditTool_Duplicate_NoReplaceAll(t *testing.T) {
	path := writeTemp(t, "foo foo foo\n")
	tool := NewEditTool()

	_, err := tool.Execute(context.Background(), editArgs(t, path, "foo", "bar", false))
	if err == nil {
		t.Fatal("expected error for duplicate old_string without replace_all")
	}
}

func TestEditTool_ReplaceAll(t *testing.T) {
	path := writeTemp(t, "foo foo foo\n")
	tool := NewEditTool()

	_, err := tool.Execute(context.Background(), editArgs(t, path, "foo", "bar", true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "bar bar bar\n" {
		t.Errorf("file content = %q, want %q", string(data), "bar bar bar\n")
	}
}

func TestEditTool_FileNotExist(t *testing.T) {
	tool := NewEditTool()
	_, err := tool.Execute(context.Background(), editArgs(t, "/nonexistent/path.txt", "x", "y", false))
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestEditTool_EmptyPath(t *testing.T) {
	tool := NewEditTool()
	b, _ := json.Marshal(map[string]any{"path": "", "old_string": "x", "new_string": "y"})
	_, err := tool.Execute(context.Background(), b)
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestEditTool_EmptyOldString(t *testing.T) {
	path := writeTemp(t, "hello\n")
	tool := NewEditTool()
	b, _ := json.Marshal(map[string]any{"path": path, "old_string": "", "new_string": "y"})
	_, err := tool.Execute(context.Background(), b)
	if err == nil {
		t.Fatal("expected error for empty old_string")
	}
}

func TestEditTool_MultilineReplacement(t *testing.T) {
	content := "func foo() {\n\treturn 1\n}\n"
	path := writeTemp(t, content)
	tool := NewEditTool()

	_, err := tool.Execute(context.Background(), editArgs(t, path, "return 1", "return 42", false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	want := "func foo() {\n\treturn 42\n}\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}

func TestEditTool_InvalidJSON(t *testing.T) {
	tool := NewEditTool()
	_, err := tool.Execute(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestEditTool_PreservesPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "perm.txt")
	if err := os.WriteFile(path, []byte("hello world\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := NewEditTool()
	_, err := tool.Execute(context.Background(), editArgs(t, path, "world", "luna", false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// File should still be readable after edit
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file unreadable after edit: %v", err)
	}
	if string(data) != "hello luna\n" {
		t.Errorf("got %q", string(data))
	}
}

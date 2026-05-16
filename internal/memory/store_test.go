package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestStore creates a Store backed by a temp directory (not ~/.luna-go).
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return &Store{
		Project: "test-project",
		dir:     filepath.Join(dir, "memory", "test-project"),
	}
}

func TestAppendBuffer_Short_Skipped(t *testing.T) {
	s := newTestStore(t)
	// < 80 chars → should be skipped
	err := s.AppendBuffer("short", "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count, _ := s.BufferCount()
	if count != 0 {
		t.Errorf("expected 0 entries, got %d", count)
	}
}

func TestAppendBuffer_Long_Written(t *testing.T) {
	s := newTestStore(t)
	longText := strings.Repeat("x", 100)
	err := s.AppendBuffer(longText, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count, _ := s.BufferCount()
	if count != 1 {
		t.Errorf("expected 1 entry, got %d", count)
	}
}

func TestAppendBuffer_Multiple(t *testing.T) {
	s := newTestStore(t)
	text := strings.Repeat("a", 100)
	for i := 0; i < 3; i++ {
		if err := s.AppendBuffer(text, "sess-1"); err != nil {
			t.Fatal(err)
		}
	}
	count, _ := s.BufferCount()
	if count != 3 {
		t.Errorf("expected 3 entries, got %d", count)
	}
}

func TestReadBuffer_Empty(t *testing.T) {
	s := newTestStore(t)
	entries, err := s.ReadBuffer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil for empty buffer, got %v", entries)
	}
}

func TestReadBuffer_EntryFields(t *testing.T) {
	s := newTestStore(t)
	text := strings.Repeat("hello world ", 10)
	if err := s.AppendBuffer(text, "sess-42"); err != nil {
		t.Fatal(err)
	}

	entries, err := s.ReadBuffer()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Project != "test-project" {
		t.Errorf("project = %q, want %q", e.Project, "test-project")
	}
	if e.Session != "sess-42" {
		t.Errorf("session = %q, want %q", e.Session, "sess-42")
	}
	if e.Timestamp == "" {
		t.Error("timestamp should not be empty")
	}
	if !strings.Contains(e.Text, "hello world") {
		t.Errorf("text = %q", e.Text)
	}
}

func TestClearBuffer(t *testing.T) {
	s := newTestStore(t)
	text := strings.Repeat("y", 100)
	s.AppendBuffer(text, "s1")
	s.AppendBuffer(text, "s1")

	if err := s.ClearBuffer(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count, _ := s.BufferCount()
	if count != 0 {
		t.Errorf("expected 0 after clear, got %d", count)
	}
}

func TestClearBuffer_NoFile(t *testing.T) {
	s := newTestStore(t)
	// Clearing when no buffer exists should not error
	if err := s.ClearBuffer(); err != nil {
		t.Fatalf("unexpected error on empty clear: %v", err)
	}
}

func TestReadContext_NotExist(t *testing.T) {
	s := newTestStore(t)
	ctx, err := s.ReadContext()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx != "" {
		t.Errorf("expected empty context, got %q", ctx)
	}
}

func TestWriteAndReadContext(t *testing.T) {
	s := newTestStore(t)
	content := "# Memory\nThis is a test context.\n"

	if err := s.WriteContext(content); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.ReadContext()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestWriteContext_CreatesDir(t *testing.T) {
	s := newTestStore(t)
	// dir does not exist yet
	if err := s.WriteContext("hello\n"); err != nil {
		t.Fatalf("WriteContext should create dirs: %v", err)
	}
	if _, err := os.Stat(s.ContextPath()); err != nil {
		t.Errorf("context.md not created: %v", err)
	}
}

func TestStatus_EmptyStore(t *testing.T) {
	s := newTestStore(t)
	status := s.Status()
	if !strings.Contains(status, "test-project") {
		t.Errorf("status missing project name: %q", status)
	}
	if !strings.Contains(status, "0 entries") {
		t.Errorf("status missing entry count: %q", status)
	}
}

func TestStatus_WithData(t *testing.T) {
	s := newTestStore(t)
	text := strings.Repeat("z", 100)
	s.AppendBuffer(text, "s")
	// "line1\nline2" has 1 newline → strings.Count+1 = 2 lines
	s.WriteContext("line1\nline2")

	status := s.Status()
	if !strings.Contains(status, "1 entries") {
		t.Errorf("status should show 1 entry: %q", status)
	}
	if !strings.Contains(status, "2 lines") {
		t.Errorf("status should show 2 lines: %q", status)
	}
}

func TestBufferCount_NoFile(t *testing.T) {
	s := newTestStore(t)
	count, err := s.BufferCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestContextPath(t *testing.T) {
	s := newTestStore(t)
	if !strings.HasSuffix(s.ContextPath(), "context.md") {
		t.Errorf("ContextPath = %q, want suffix context.md", s.ContextPath())
	}
}

func TestSkillsDir(t *testing.T) {
	s := newTestStore(t)
	if !strings.HasSuffix(s.SkillsDir(), "skills") {
		t.Errorf("SkillsDir = %q, want suffix skills", s.SkillsDir())
	}
}

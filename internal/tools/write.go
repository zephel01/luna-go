package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteTool writes content to a file, creating directories as needed.
// When unsafe=false, overwriting an existing file requires confirmation.
type WriteTool struct {
	unsafe  bool
	confirm func(prompt string) bool // nil = built-in bufio fallback
}

func NewWriteTool(unsafe bool) *WriteTool { return &WriteTool{unsafe: unsafe} }

// SetConfirm overrides the built-in stdin confirmation with a custom function.
func (t *WriteTool) SetConfirm(fn func(prompt string) bool) { t.confirm = fn }

func (t *WriteTool) Name() string { return "write" }

func (t *WriteTool) Description() string {
	return "Write content to a file. Creates parent directories if needed. " +
		"When overwriting an existing file, shows a summary and asks for confirmation " +
		"(skip with --unsafe). Prefer the edit tool for targeted partial changes."
}

func (t *WriteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute path to the file to write",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Content to write to the file",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	existing, statErr := os.Stat(args.Path)
	isOverwrite := statErr == nil && !existing.IsDir()

	if !t.unsafe && isOverwrite {
		oldLines := lineCount(args.Path)
		newLines := strings.Count(args.Content, "\n") + 1

		// Print info banner (may contain newlines — keep separate from prompt).
		fmt.Fprintf(os.Stderr, "\n⚠  write: %s\n", args.Path)
		fmt.Fprintf(os.Stderr, "   existing: %d bytes (%d lines)\n", existing.Size(), oldLines)
		fmt.Fprintf(os.Stderr, "   new:      %d bytes (%d lines)\n", len(args.Content), newLines)

		const confirmPrompt = "Overwrite? [y/a/N] "
		var ok bool
		if t.confirm != nil {
			ok = t.confirm(confirmPrompt)
		} else {
			fmt.Fprint(os.Stderr, confirmPrompt)
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				ans := strings.TrimSpace(strings.ToLower(scanner.Text()))
				ok = ans == "y" || ans == "yes"
			}
		}
		if !ok {
			return "cancelled by user", nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(args.Path), 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(args.Path, []byte(args.Content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", args.Path, err)
	}

	action := "wrote"
	if isOverwrite {
		action = "overwrote"
	}
	return fmt.Sprintf("OK: %s %d bytes to %s", action, len(args.Content), args.Path), nil
}

// lineCount returns the number of lines in a file (best-effort, 0 on error).
func lineCount(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "\n") + 1
}

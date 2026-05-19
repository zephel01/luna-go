package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const maxReadLines = 2000

// ReadTool reads a file and returns its contents with line numbers.
type ReadTool struct{}

// NewReadTool returns a new ReadTool ready for use.
func NewReadTool() *ReadTool { return &ReadTool{} }

func (t *ReadTool) Name() string { return "read" }

func (t *ReadTool) Description() string {
	return "Read a file from the filesystem. Returns contents with line numbers. Limited to 2000 lines."
}

func (t *ReadTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to read",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	data, err := os.ReadFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", args.Path, err)
	}

	lines := strings.Split(string(data), "\n")
	truncated := false
	if len(lines) > maxReadLines {
		lines = lines[:maxReadLines]
		truncated = true
	}

	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, "%d\t%s\n", i+1, line)
	}
	if truncated {
		sb.WriteString(fmt.Sprintf("... (truncated at %d lines)\n", maxReadLines))
	}
	return sb.String(), nil
}

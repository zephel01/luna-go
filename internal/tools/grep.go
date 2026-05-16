package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const maxGrepLines = 200

// GrepTool searches files for a pattern using the system grep.
type GrepTool struct{}

func NewGrepTool() *GrepTool { return &GrepTool{} }

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) Description() string {
	return "Search for a regex pattern in files. Returns matching lines with file path and line number. Up to 200 results."
}

func (t *GrepTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression pattern to search for",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "File or directory path to search in",
			},
			"recursive": map[string]any{
				"type":        "boolean",
				"description": "Search directories recursively (default: true)",
			},
		},
		"required": []string{"pattern", "path"},
	}
}

func (t *GrepTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Pattern   string `json:"pattern"`
		Path      string `json:"path"`
		Recursive *bool  `json:"recursive"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if args.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	recursive := true
	if args.Recursive != nil {
		recursive = *args.Recursive
	}

	cmdArgs := []string{"-n", "--color=never"}
	if recursive {
		cmdArgs = append(cmdArgs, "-r")
	}
	cmdArgs = append(cmdArgs, args.Pattern, args.Path)

	out, err := exec.Command("grep", cmdArgs...).Output()
	if err != nil {
		// exit code 1 means no matches — not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "no matches found", nil
		}
		return "", fmt.Errorf("grep: %w", err)
	}

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	truncated := false
	if len(lines) > maxGrepLines {
		lines = lines[:maxGrepLines]
		truncated = true
	}

	result := strings.Join(lines, "\n")
	if truncated {
		result += fmt.Sprintf("\n... (showing first %d matches)", maxGrepLines)
	}
	return result, nil
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// EditTool applies a targeted old_string → new_string replacement to a file.
// This is safer than write for partial edits — only the matched text changes.
type EditTool struct{}

func NewEditTool() *EditTool { return &EditTool{} }

func (t *EditTool) Name() string { return "edit" }

func (t *EditTool) Description() string {
	return "Edit a file by replacing old_string with new_string. " +
		"The file must already exist (read it first). " +
		"old_string must appear exactly once unless replace_all is true. " +
		"Prefer this over write for targeted changes."
}

func (t *EditTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute path to the file to edit",
			},
			"old_string": map[string]any{
				"type":        "string",
				"description": "Exact string to find in the file (must be unique unless replace_all is true)",
			},
			"new_string": map[string]any{
				"type":        "string",
				"description": "String to replace old_string with",
			},
			"replace_all": map[string]any{
				"type":        "boolean",
				"description": "If true, replace every occurrence of old_string (default: false)",
			},
		},
		"required": []string{"path", "old_string", "new_string"},
	}
}

func (t *EditTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Path       string `json:"path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if args.OldString == "" {
		return "", fmt.Errorf("old_string is required")
	}

	data, err := os.ReadFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", args.Path, err)
	}
	content := string(data)

	count := strings.Count(content, args.OldString)
	if count == 0 {
		return "", fmt.Errorf("old_string not found in %s", args.Path)
	}
	if count > 1 && !args.ReplaceAll {
		return "", fmt.Errorf(
			"old_string found %d times in %s — use replace_all:true to replace all, or make old_string more specific",
			count, args.Path,
		)
	}

	var updated string
	if args.ReplaceAll {
		updated = strings.ReplaceAll(content, args.OldString, args.NewString)
	} else {
		updated = strings.Replace(content, args.OldString, args.NewString, 1)
	}

	if err := os.WriteFile(args.Path, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", args.Path, err)
	}

	occurrences := count
	if !args.ReplaceAll {
		occurrences = 1
	}
	return fmt.Sprintf("OK: replaced %d occurrence(s) in %s", occurrences, args.Path), nil
}

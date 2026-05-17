package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// editEntry represents a single old→new replacement within a multi-edit call.
type editEntry struct {
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

// EditTool applies one or more targeted old_string → new_string replacements to a file.
// Two calling conventions are supported:
//
//  1. Single edit (legacy, backward-compatible):
//     {"path": "...", "old_string": "...", "new_string": "...", "replace_all": false}
//
//  2. Multi-edit (new):
//     {"path": "...", "edits": [{"old_string": "...", "new_string": "..."}, ...]}
//
// When "edits" is present it takes priority over the top-level old/new fields.
// Each edit in the array is applied sequentially to the in-memory content before
// writing the file once — so edits must not depend on file state after a previous edit.
type EditTool struct{}

func NewEditTool() *EditTool { return &EditTool{} }

func (t *EditTool) Name() string { return "edit" }

func (t *EditTool) Description() string {
	return "Edit a file by replacing text. " +
		"Use 'edits' array for multiple replacements in one call (applied sequentially). " +
		"Or use old_string/new_string for a single replacement. " +
		"The file must already exist (read it first). " +
		"Each old_string must appear exactly once unless replace_all is true. " +
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
			"edits": map[string]any{
				"type":        "array",
				"description": "Array of replacements to apply sequentially. Takes priority over old_string/new_string.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"old_string": map[string]any{
							"type":        "string",
							"description": "Exact string to find (must be unique unless replace_all is true)",
						},
						"new_string": map[string]any{
							"type":        "string",
							"description": "String to replace old_string with",
						},
						"replace_all": map[string]any{
							"type":        "boolean",
							"description": "Replace every occurrence (default: false)",
						},
					},
					"required": []string{"old_string", "new_string"},
				},
			},
			"old_string": map[string]any{
				"type":        "string",
				"description": "Exact string to find (single-edit form; ignored when 'edits' is set)",
			},
			"new_string": map[string]any{
				"type":        "string",
				"description": "String to replace old_string with (single-edit form)",
			},
			"replace_all": map[string]any{
				"type":        "boolean",
				"description": "If true, replace every occurrence of old_string (single-edit form, default: false)",
			},
		},
		"required": []string{"path"},
	}
}

func (t *EditTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Path       string      `json:"path"`
		Edits      []editEntry `json:"edits"`
		OldString  string      `json:"old_string"`
		NewString  string      `json:"new_string"`
		ReplaceAll bool        `json:"replace_all"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Normalise: if no edits array, promote legacy fields.
	edits := args.Edits
	if len(edits) == 0 {
		if args.OldString == "" {
			return "", fmt.Errorf("old_string is required (or provide 'edits' array)")
		}
		edits = []editEntry{{
			OldString:  args.OldString,
			NewString:  args.NewString,
			ReplaceAll: args.ReplaceAll,
		}}
	}

	data, err := os.ReadFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", args.Path, err)
	}
	content := string(data)

	type result struct {
		idx   int
		count int
	}
	results := make([]result, 0, len(edits))

	for i, e := range edits {
		if e.OldString == "" {
			return "", fmt.Errorf("edits[%d].old_string is empty", i)
		}
		count := strings.Count(content, e.OldString)
		if count == 0 {
			return "", fmt.Errorf("edits[%d]: old_string not found in %s", i, args.Path)
		}
		if count > 1 && !e.ReplaceAll {
			return "", fmt.Errorf(
				"edits[%d]: old_string found %d times in %s — use replace_all:true or make it more specific",
				i, count, args.Path,
			)
		}
		applied := count
		if e.ReplaceAll {
			content = strings.ReplaceAll(content, e.OldString, e.NewString)
		} else {
			content = strings.Replace(content, e.OldString, e.NewString, 1)
			applied = 1
		}
		results = append(results, result{i, applied})
	}

	if err := os.WriteFile(args.Path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", args.Path, err)
	}

	if len(results) == 1 {
		return fmt.Sprintf("OK: replaced %d occurrence(s) in %s", results[0].count, args.Path), nil
	}
	total := 0
	for _, r := range results {
		total += r.count
	}
	return fmt.Sprintf("OK: applied %d edits (%d replacement(s)) in %s", len(results), total, args.Path), nil
}

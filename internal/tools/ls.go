package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LsTool lists the contents of a directory.
type LsTool struct{}

// NewLsTool returns a new LsTool ready for use.
func NewLsTool() *LsTool { return &LsTool{} }

func (t *LsTool) Name() string { return "ls" }

func (t *LsTool) Description() string {
	return "List the contents of a directory. " +
		"Returns each entry with its type (file/dir), size, and name. " +
		"Defaults to the current working directory if no path is given."
}

func (t *LsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Directory path to list. Defaults to current working directory.",
			},
		},
	}
}

// Execute lists the contents of a directory, sorted with directories first.
// Each entry shows an icon (📁/📄), name, and human-readable file size.
// Defaults to the current working directory when no path is provided.
func (t *LsTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	// Tolerate empty input JSON.
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("invalid args: %w", err)
		}
	}

	dir := args.Path
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getwd: %w", err)
		}
		dir = cwd
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("abs: %w", err)
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return "", fmt.Errorf("readdir %s: %w", absDir, err)
	}

	if len(entries) == 0 {
		return fmt.Sprintf("%s (empty)", absDir), nil
	}

	// Sort: directories first, then files, each group alphabetically.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})

	var sb strings.Builder
	sb.WriteString(absDir + "/\n")
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if e.IsDir() {
			sb.WriteString(fmt.Sprintf("  📁 %s/\n", e.Name()))
		} else {
			sb.WriteString(fmt.Sprintf("  📄 %-40s %s\n", e.Name(), formatBytes(info.Size())))
		}
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

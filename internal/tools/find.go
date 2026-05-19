package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultIgnoreDirs contains directories that are almost always irrelevant
// for coding tasks and can be very large.
var defaultIgnoreDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".svn":         true,
	".hg":          true,
	"__pycache__":  true,
	".mypy_cache":  true,
	".pytest_cache": true,
	"dist":         true,
	"build":        true,
	".build":       true,
}

// FindTool searches for files matching a glob pattern within a directory.
type FindTool struct{}

// NewFindTool returns a new FindTool ready for use.
func NewFindTool() *FindTool { return &FindTool{} }

func (t *FindTool) Name() string { return "find" }

func (t *FindTool) Description() string {
	return "Find files matching a glob pattern (e.g. '*.go', '**/*_test.go'). " +
		"Searches recursively. Common directories like .git, node_modules, vendor are skipped automatically. " +
		"Returns a list of matching paths, one per line."
}

func (t *FindTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Glob pattern to match filenames (e.g. '*.go', '*_test.go', '*.md')",
			},
			"dir": map[string]any{
				"type":        "string",
				"description": "Directory to search in. Defaults to current working directory.",
			},
		},
		"required": []string{"pattern"},
	}
}

// Execute walks dir recursively and returns paths matching the glob pattern,
// one per line. Common noise directories (.git, node_modules, vendor, etc.)
// are skipped automatically. Results are capped at 500 entries.
func (t *FindTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Pattern string `json:"pattern"`
		Dir     string `json:"dir"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if args.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	root := args.Dir
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getwd: %w", err)
		}
		root = cwd
	}

	// Resolve to absolute path.
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("abs: %w", err)
	}

	// Strip any leading **/ from the pattern for base matching.
	// WalkDir matches against the filename; handle both "*.go" and "**/*.go".
	basePattern := args.Pattern
	if after, ok := strings.CutPrefix(basePattern, "**/"); ok {
		basePattern = after
	}

	const maxResults = 500
	var matches []string

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if defaultIgnoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		matched, merr := filepath.Match(basePattern, d.Name())
		if merr != nil {
			return merr
		}
		if matched {
			// Return relative path when inside cwd, absolute otherwise.
			rel, rerr := filepath.Rel(absRoot, path)
			if rerr == nil {
				matches = append(matches, rel)
			} else {
				matches = append(matches, path)
			}
			if len(matches) >= maxResults {
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Sprintf("no files matching %q found in %s", args.Pattern, absRoot), nil
	}

	result := strings.Join(matches, "\n")
	if len(matches) >= maxResults {
		result += fmt.Sprintf("\n… (truncated at %d results)", maxResults)
	}
	return result, nil
}

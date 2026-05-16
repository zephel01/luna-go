package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"os/exec"
)

const (
	maxOutputBytes = 10 * 1024 // 10 KB
	bashTimeout    = 30 * time.Second
)

// BashTool executes a shell command and returns its combined output.
// When unsafe=false (default), the user is prompted to confirm each command.
type BashTool struct {
	unsafe  bool
	confirm func(prompt string) bool // nil = built-in bufio fallback
}

func NewBashTool(unsafe bool) *BashTool { return &BashTool{unsafe: unsafe} }

// SetConfirm overrides the built-in stdin confirmation with a custom function.
// Use this to integrate with a readline library (e.g. liner) that holds the
// terminal in raw mode, where bufio.Scanner would not work correctly.
func (t *BashTool) SetConfirm(fn func(prompt string) bool) { t.confirm = fn }

func (t *BashTool) Name() string { return "bash" }

func (t *BashTool) Description() string {
	return "Execute a shell command. Returns stdout and stderr combined. 30s timeout, 10KB output limit."
}

func (t *BashTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to execute",
			},
		},
		"required": []string{"command"},
	}
}

func (t *BashTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if args.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	if !t.unsafe {
		prompt := fmt.Sprintf("\n⚠  bash: %s\nRun? [y/N] ", args.Command)
		var ok bool
		if t.confirm != nil {
			ok = t.confirm(prompt)
		} else {
			fmt.Fprint(os.Stderr, prompt)
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

	ctx, cancel := context.WithTimeout(context.Background(), bashTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", args.Command)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run() // ignore exit error; output is returned regardless

	result := out.String()
	if len(result) > maxOutputBytes {
		// keep the tail — most recent output is usually what matters
		result = "... (truncated)\n" + result[len(result)-maxOutputBytes:]
	}
	return result, nil
}

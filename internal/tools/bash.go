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
	unsafe bool
}

func NewBashTool(unsafe bool) *BashTool { return &BashTool{unsafe: unsafe} }

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
		fmt.Fprintf(os.Stderr, "\n⚠  bash: %s\nRun? [y/N] ", args.Command)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return "cancelled", nil
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
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

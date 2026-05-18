package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	maxOutputBytes = 10 * 1024 // 10 KB
	bashTimeout    = 30 * time.Second
)

// BashTool executes shell commands inside a persistent bash process.
// Environment variables and the current directory are preserved across calls
// within the same session.
// When unsafe=false (default), the user is prompted to confirm each command.
// When sandbox options are set (via SetSandbox), commands run inside a Docker
// container instead of the host shell for stronger isolation.
type BashTool struct {
	unsafe  bool
	sandbox SandboxOptions
	confirm func(prompt string) bool // nil = built-in bufio fallback

	shellOnce sync.Once
	shell     *persistentShell
	shellErr  error
}

func NewBashTool(unsafe bool) *BashTool { return &BashTool{unsafe: unsafe} }

// SetSandbox configures Docker-based isolation for bash commands.
// Must be called before the first Execute call (before the shell is started).
func (t *BashTool) SetSandbox(opts SandboxOptions) { t.sandbox = opts }

// SetConfirm overrides the built-in stdin confirmation with a custom function.
func (t *BashTool) SetConfirm(fn func(prompt string) bool) { t.confirm = fn }

// Close shuts down the underlying persistent shell. Call on session exit.
func (t *BashTool) Close() {
	t.shellOnce.Do(func() {}) // ensure once is marked done
	if t.shell != nil {
		t.shell.close()
	}
}

func (t *BashTool) getShell() (*persistentShell, error) {
	t.shellOnce.Do(func() {
		t.shell, t.shellErr = newPersistentShell(t.sandbox)
	})
	// If the shell died between calls, try restarting with the same config.
	if t.shellErr == nil && !t.shell.alive() {
		t.shell, t.shellErr = newPersistentShell(t.sandbox)
	}
	return t.shell, t.shellErr
}

func (t *BashTool) Name() string { return "bash" }

func (t *BashTool) Description() string {
	base := "Execute a shell command in a persistent bash session. " +
		"Environment variables and working directory (cd) are preserved across calls. " +
		"30s timeout per command, 10KB output limit."
	if t.sandbox.Enabled {
		return base + " Running inside Docker sandbox (" + t.sandbox.Image + ")."
	}
	return base
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
		fmt.Fprintf(os.Stderr, "\n⚠  bash: %s\n", args.Command)
		const confirmPrompt = "Run? [y/a/N] "
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

	sh, err := t.getShell()
	if err != nil {
		return "", fmt.Errorf("shell: %w", err)
	}

	output, exitCode, err := sh.exec(bashTimeout, args.Command)
	if err != nil {
		return "", err
	}
	if exitCode != 0 {
		output = fmt.Sprintf("(exit %d)\n%s", exitCode, output)
	}
	return output, nil
}

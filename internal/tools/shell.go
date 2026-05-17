package tools

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// persistentShell maintains a single bash process for the lifetime of a session.
// Commands are sent via stdin; output is read until a unique sentinel line appears.
// If the shell process dies unexpectedly it is restarted on the next Execute call.
type persistentShell struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

func newPersistentShell() (*persistentShell, error) {
	s := &persistentShell{}
	if err := s.start(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *persistentShell) start() error {
	cmd := exec.Command("bash", "--norc", "--noprofile")
	cmd.Env = cmd.Environ() // inherit environment

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	// Merge stderr into stdout inside the shell (we'll redirect per-command).
	cmd.Stderr = nil // handled per-command via shell redirection

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start bash: %w", err)
	}
	s.cmd = cmd
	s.stdin = stdin
	s.stdout = bufio.NewReaderSize(stdout, 64*1024)
	return nil
}

// alive returns false if the shell process has already exited.
func (s *persistentShell) alive() bool {
	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}
	// ProcessState is only set after Wait; if it's set the process is done.
	return s.cmd.ProcessState == nil
}

// exec runs a shell command, returning combined stdout+stderr output, exit code, and error.
// It uses a nanosecond-unique sentinel to detect command completion.
func (s *persistentShell) exec(timeout time.Duration, command string) (string, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Restart the shell if it died.
	if !s.alive() {
		if err := s.start(); err != nil {
			return "", -1, fmt.Errorf("shell restart: %w", err)
		}
	}

	sentinel := fmt.Sprintf("___LUNA_%d___", time.Now().UnixNano())

	// Send: run command with stderr merged, then print sentinel with exit code.
	script := fmt.Sprintf("{ %s; } 2>&1; printf '\\n%s_%%d___\\n' $?\n", command, sentinel)
	if _, err := fmt.Fprint(s.stdin, script); err != nil {
		return "", -1, fmt.Errorf("write to shell: %w", err)
	}

	// Read output until sentinel line.
	var sb strings.Builder
	exitCode := 0
	deadline := time.Now().Add(timeout)

	type readResult struct {
		line string
		err  error
	}
	lineCh := make(chan readResult, 1)

	readLine := func() {
		line, err := s.stdout.ReadString('\n')
		lineCh <- readResult{line, err}
	}

	for {
		go readLine()
		select {
		case res := <-lineCh:
			if res.err != nil && res.err != io.EOF {
				return sb.String(), -1, fmt.Errorf("read shell output: %w", res.err)
			}
			line := res.line
			// Check for sentinel (with exit code suffix).
			trimmed := strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(trimmed, sentinel+"_") && strings.HasSuffix(trimmed, "___") {
				// Parse exit code from sentinel_N___
				inner := strings.TrimPrefix(trimmed, sentinel+"_")
				inner = strings.TrimSuffix(inner, "___")
				fmt.Sscanf(inner, "%d", &exitCode)
				goto done
			}
			sb.WriteString(line)
			// Output cap.
			if sb.Len() > maxOutputBytes {
				// Drain remaining output (consume until sentinel).
				for {
					go readLine()
					r := <-lineCh
					if r.err != nil {
						goto done
					}
					t2 := strings.TrimRight(r.line, "\r\n")
					if strings.HasPrefix(t2, sentinel+"_") && strings.HasSuffix(t2, "___") {
						inner := strings.TrimPrefix(t2, sentinel+"_")
						inner = strings.TrimSuffix(inner, "___")
						fmt.Sscanf(inner, "%d", &exitCode)
						goto done
					}
				}
			}
		case <-time.After(time.Until(deadline)):
			// Timeout: kill the shell so the next call gets a fresh one.
			_ = s.cmd.Process.Kill()
			_ = s.cmd.Wait()
			s.cmd = nil
			return sb.String(), -1, fmt.Errorf("command timed out after %s", timeout)
		}
	}
done:
	result := sb.String()
	if len(result) > maxOutputBytes {
		result = "... (truncated)\n" + result[len(result)-maxOutputBytes:]
	}
	return result, exitCode, nil
}

// close shuts down the shell process gracefully.
func (s *persistentShell) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stdin != nil {
		_, _ = fmt.Fprintln(s.stdin, "exit 0")
		_ = s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Wait()
	}
}

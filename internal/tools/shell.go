package tools

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// SandboxOptions controls Docker-based isolation for the persistent shell.
// When Enabled is true bash commands run inside a Docker container.
// If Docker is unavailable a warning is printed and execution falls back to
// the host shell so luna remains usable on machines without Docker.
type SandboxOptions struct {
	Enabled bool
	Image   string // e.g. "ubuntu:22.04"
	Network string // "none" | "bridge" | "host"
	Memory  string // Docker memory limit, e.g. "256m"
	CPUs    string // fractional CPU quota, e.g. "0.5"
}

// persistentShell maintains a single bash process for the lifetime of a session.
// Commands are sent via stdin; output is read until a unique sentinel line appears.
// If the shell process dies unexpectedly it is restarted on the next Execute call.
// When sandbox.Enabled is true the shell runs inside a Docker container.
type persistentShell struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	sandbox SandboxOptions // retained for restarts after process death
}

func newPersistentShell(sandbox SandboxOptions) (*persistentShell, error) {
	s := &persistentShell{sandbox: sandbox}
	if err := s.start(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *persistentShell) start() error {
	var cmd *exec.Cmd

	if s.sandbox.Enabled {
		// Resolve defaults for any unset fields.
		image := s.sandbox.Image
		if image == "" {
			image = "ubuntu:22.04"
		}
		network := s.sandbox.Network
		if network == "" {
			network = "none"
		}
		memory := s.sandbox.Memory
		if memory == "" {
			memory = "256m"
		}
		cpus := s.sandbox.CPUs
		if cpus == "" {
			cpus = "0.5"
		}

		// Fall back gracefully when Docker is not installed.
		if _, err := exec.LookPath("docker"); err != nil {
			fmt.Fprintln(os.Stderr, "⚠  docker not found — sandbox disabled, running on host")
			s.sandbox.Enabled = false
			cmd = exec.Command("bash", "--norc", "--noprofile")
			cmd.Env = cmd.Environ()
		} else {
			workdir, err := os.Getwd()
			if err != nil {
				workdir = "."
			}
			args := []string{
				"run", "--rm", "-i",
				"--network", network,
				"--memory", memory,
				"--cpus", cpus,
				"-v", workdir + ":/workspace",
				"-w", "/workspace",
				image,
				"bash", "--norc", "--noprofile",
			}
			cmd = exec.Command("docker", args...)
			fmt.Fprintf(os.Stderr, "🐳 starting container (%s)...\n", image)
		}
	} else {
		cmd = exec.Command("bash", "--norc", "--noprofile")
		cmd.Env = cmd.Environ() // inherit environment
	}

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

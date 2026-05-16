package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/peterh/liner"
	"github.com/zephel01/luna-go/internal/llm"
	"github.com/zephel01/luna-go/internal/tools"
)

var (
	// thinkTagRe strips <think>...</think> blocks (Qwen3 / DeepSeek-R1 reasoning leak).
	thinkTagRe = regexp.MustCompile(`(?s)<think>.*?</think>`)
	// stopMarkerRe strips common tokenizer stop markers that leak into output.
	stopMarkerRe = regexp.MustCompile(`<\|(?:eot_id|im_end|end_of_text|turn|python_tag|channel[^|]*)\|>`)
)

// cleanContent removes reasoning noise from a model response before processing.
func cleanContent(s string) string {
	s = thinkTagRe.ReplaceAllString(s, "")
	s = stopMarkerRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

const systemPromptTpl = `You are Luna, a minimal coding assistant.
You can read files, write files, edit files, run shell commands, and search code.
Rules:
- Always read a file before editing it.
- Always use absolute paths (e.g. /home/user/project/main.go, not main.go).
- When you have received a tool result, answer the question directly. Do not call the same tool again.
Current working directory: %s`

// Options configures an Agent.
type Options struct {
	MaxIter      int
	Stream       bool
	ExtraContext  string // content injected into system prompt (from .luna-context.md)
	SkillsBlock  string // XML block listing available skills (from skills.Load)
	SessionLog   string // path to write session JSONL (empty = disabled)
	// LoopTimeout is the wall-clock limit per Run/loop call.
	// 0 → default (30 min). Negative → no limit.
	LoopTimeout  time.Duration
	// OnSlashCmd is called when the user types a /command in REPL.
	// Return true to continue the REPL, false to exit.
	// If nil, unknown slash commands print a hint.
	OnSlashCmd func(a *Agent, cmd string) bool
	// CompressClient is the LLM client used to summarise old context turns.
	// If nil, context compression is disabled regardless of CompressThreshold.
	CompressClient llm.Client
	// CompressThreshold is the estimated token count that triggers compression.
	// 0 (default) = disabled. Set to a positive value (e.g. 24000) to enable.
	CompressThreshold int
}

// Agent drives the ReAct loop: think → tool → observe → repeat.
type Agent struct {
	client      llm.Client
	registry    *tools.Registry
	history     []llm.Message
	maxIter     int
	stream      bool
	sessionLog  string
	loopTimeout      time.Duration        // wall-clock limit per Run call (0 = default 30 min)
	onSlashCmd       func(a *Agent, cmd string) bool
	completer        func(string) []string // tab-completion candidates for REPL
	rl               *liner.State          // active liner instance (non-nil only during REPL)
	goal             string                // session-level goal injected into every LLM request
	compressClient   llm.Client            // nil = compression disabled
	compressThreshold int                  // estimated token threshold; 0 = disabled
}

// SetGoal sets the session-level goal.
func (a *Agent) SetGoal(goal string) { a.goal = goal }

// Goal returns the current session-level goal.
func (a *Agent) Goal() string { return a.goal }

// ClearGoal removes the session-level goal.
func (a *Agent) ClearGoal() { a.goal = "" }

// SetCompleter registers a tab-completion function for the REPL.
// fn receives the current line prefix and returns matching candidates.
func (a *Agent) SetCompleter(fn func(string) []string) {
	a.completer = fn
}

// SetSlashHandler replaces the slash-command handler (used to wire closures after agent creation).
func (a *Agent) SetSlashHandler(fn func(a *Agent, cmd string) bool) {
	a.onSlashCmd = fn
}

// ReadLine reads a line using the active liner instance (REPL mode) or plain stdin.
// Slash-command handlers should use this instead of bufio.Scanner so they work
// correctly while liner holds the terminal in raw mode.
func (a *Agent) ReadLine(prompt string) (string, error) {
	if a.rl != nil {
		return a.rl.Prompt(prompt)
	}
	// Fallback for one-shot / non-REPL callers.
	fmt.Fprint(os.Stderr, prompt)
	var line string
	_, err := fmt.Scanln(&line)
	return strings.TrimSpace(line), err
}

// SetClient swaps the LLM client (e.g. after a /models switch).
// The current conversation history is preserved.
func (a *Agent) SetClient(client llm.Client) {
	a.client = client
}

// History returns a copy of the current conversation history.
// Used by the caller to capture assistant responses for memory buffering.
func (a *Agent) History() []llm.Message {
	out := make([]llm.Message, len(a.history))
	copy(out, a.history)
	return out
}

// New creates an Agent with a system message already in history.
func New(client llm.Client, registry *tools.Registry, opts Options) *Agent {
	maxIter := opts.MaxIter
	if maxIter <= 0 {
		maxIter = 20
	}
	cwd, _ := os.Getwd()
	sys := fmt.Sprintf(systemPromptTpl, cwd)
	if opts.ExtraContext != "" {
		sys += "\n\n## Project Context\n" + opts.ExtraContext
	}
	if opts.SkillsBlock != "" {
		sys += "\n\n## Available Skills\n" +
			"When a task matches a skill, use the read tool to load the full SKILL.md at its path, then follow the instructions.\n" +
			opts.SkillsBlock
	}

	loopTimeout := opts.LoopTimeout
	if loopTimeout == 0 {
		loopTimeout = 30 * time.Minute // default wall-clock limit
	}

	// Compression: only active when both client and a positive threshold are set.
	compressThreshold := 0
	if opts.CompressClient != nil && opts.CompressThreshold > 0 {
		compressThreshold = opts.CompressThreshold
	}

	return &Agent{
		client:            client,
		registry:          registry,
		history:           []llm.Message{{Role: "system", Content: sys}},
		maxIter:           maxIter,
		stream:            opts.Stream,
		sessionLog:        opts.SessionLog,
		loopTimeout:       loopTimeout,
		onSlashCmd:        opts.OnSlashCmd,
		compressClient:    opts.CompressClient,
		compressThreshold: compressThreshold,
	}
}

// saveHistory writes the conversation history as JSONL to sessionLog.
func (a *Agent) saveHistory() {
	if a.sessionLog == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(a.sessionLog), 0o755); err != nil {
		return
	}
	f, err := os.Create(a.sessionLog)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, m := range a.history {
		_ = enc.Encode(m)
	}
}

// SaveSession flushes the current history to the session log file.
// Call this after one-shot mode completes.
func (a *Agent) SaveSession() {
	a.saveHistory()
}

// Run appends a user message to history and executes the ReAct loop.
func (a *Agent) Run(ctx context.Context, userInput string) error {
	a.history = append(a.history, llm.Message{
		Role:    "user",
		Content: userInput,
	})
	return a.loop(ctx)
}

// loop is the core ReAct cycle.
func (a *Agent) loop(ctx context.Context) error {
	// Apply wall-clock timeout. Negative loopTimeout means no limit.
	if a.loopTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.loopTimeout)
		defer cancel()
	}

	for i := 0; i < a.maxIter; i++ {
		// Check wall-clock timeout before each LLM call.
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("loop timeout (%s elapsed): %w", a.loopTimeout, err)
		}

		// Compress old context turns if history is getting large.
		a.maybeCompress(ctx)

		messages := a.history
		// Inject goal as a leading system message so the model sees it every turn.
		if a.goal != "" {
			goalMsg := llm.Message{
				Role: "system",
				Content: "## 現在のゴール\n" + a.goal + "\n\n" +
					"このゴールを達成するまで自律的に作業を続けること。" +
					"ゴールを達成したら「ゴール達成:」で始まるメッセージで完了を宣言すること。",
			}
			messages = append([]llm.Message{goalMsg}, messages...)
		}
		req := llm.Request{
			Messages: messages,
			Tools:    a.registry.Definitions(),
		}

		var (
			resp *llm.Response
			err  error
		)
		if a.stream {
			resp, err = a.client.Stream(ctx, req, os.Stdout)
		} else {
			resp, err = a.client.Complete(ctx, req)
		}
		if err != nil {
			return fmt.Errorf("llm: %w", err)
		}

		// Strip reasoning noise (<think> tags, stop markers) before any parsing.
		resp.Content = cleanContent(resp.Content)

		// If the model didn't use the tool_calls API, it may have written
		// the call as JSON text in the content field.
		// In that case we execute the tools but inject results as a plain user
		// message instead of using the assistant→tool protocol, which avoids
		// confusing the model into calling the same tool a second time.
		if len(resp.ToolCalls) == 0 {
			if calls := parseContentToolCalls(resp.Content); len(calls) > 0 {
				var resultParts []string
				for _, tc := range calls {
					fmt.Fprintf(os.Stderr, "\n⚙  %s(%s)\n",
						tc.Function.Name, summarize(tc.Function.Arguments, 80))
					result := a.executeTool(ctx, tc)
					resultParts = append(resultParts,
						fmt.Sprintf("Tool '%s' was called and returned:\n%s\n\nDo NOT call '%s' again. Use the above result to answer the question directly.",
							tc.Function.Name, result, tc.Function.Name))
				}
				a.history = append(a.history, llm.Message{
					Role:    "user",
					Content: strings.Join(resultParts, "\n\n"),
				})
				continue // back to LLM with injected results
			}
		}

		// No tool calls → final answer. Save to history, print, and exit loop
		// (or auto-continue when a goal is active and not yet achieved).
		if len(resp.ToolCalls) == 0 {
			if resp.Content != "" {
				a.history = append(a.history, llm.Message{
					Role:    "assistant",
					Content: resp.Content,
				})
				if !a.stream {
					fmt.Print(resp.Content)
				}
			}
			fmt.Println()
			// Goal mode: keep iterating until model declares "ゴール達成".
			if a.goal != "" && !strings.Contains(resp.Content, "ゴール達成") {
				continue
			}
			return nil
		}

		// Proper tool_calls response: record the assistant turn then execute.
		a.history = append(a.history, llm.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			fmt.Fprintf(os.Stderr, "\n⚙  %s(%s)\n",
				tc.Function.Name, summarize(tc.Function.Arguments, 80))

			result := a.executeTool(ctx, tc)

			a.history = append(a.history, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			})
		}
	}
	return fmt.Errorf("max iterations (%d) reached without a final answer", a.maxIter)
}

// executeTool dispatches a single tool call and returns its string result.
func (a *Agent) executeTool(ctx context.Context, tc llm.ToolCall) string {
	tool, ok := a.registry.Get(tc.Function.Name)
	if !ok {
		return fmt.Sprintf("error: unknown tool %q", tc.Function.Name)
	}
	result, err := tool.Execute(ctx, json.RawMessage(tc.Function.Arguments))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return result
}

// REPL runs an interactive read-eval-print loop with tab completion and history.
func (a *Agent) REPL(ctx context.Context) {
	defer a.saveHistory()

	rl := liner.NewLiner()
	a.rl = rl
	defer func() {
		a.rl = nil
		rl.Close()
	}()
	rl.SetCtrlCAborts(true) // Ctrl-C returns ErrPromptAborted instead of killing process

	if a.completer != nil {
		rl.SetCompleter(a.completer)
	}

	fmt.Fprintln(os.Stderr, "Luna v0.4  —  type your request, Tab to complete, Ctrl-C or 'exit' to quit")
	if a.goal != "" {
		fmt.Fprintf(os.Stderr, "🎯 goal: %s\n", a.goal)
	}

	for {
		fmt.Fprintln(os.Stderr)
		input, err := rl.Prompt("> ")
		if err == liner.ErrPromptAborted || err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "input error: %v\n", err)
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Multi-line mode: [[ ... ]] collects lines until ]] is entered.
		if input == "[[" {
			var lines []string
			for {
				line, lerr := rl.Prompt("... ")
				if lerr == liner.ErrPromptAborted || lerr == io.EOF {
					break
				}
				if lerr != nil {
					break
				}
				if strings.TrimSpace(line) == "]]" {
					break
				}
				lines = append(lines, line)
			}
			input = strings.Join(lines, "\n")
			if input == "" {
				continue
			}
		}

		rl.AppendHistory(input) // ↑↓ key history
		if input == "exit" || input == "quit" {
			break
		}
		// Slash commands (/models, /help, custom)
		if strings.HasPrefix(input, "/") {
			if input == "/help" {
				fmt.Fprintln(os.Stderr, "Commands:")
				fmt.Fprintln(os.Stderr, "  /models          — list and switch Ollama models (with RAM recommendation)")
				fmt.Fprintln(os.Stderr, "  /dream           — consolidate session buffer into context.md via Ollama")
				fmt.Fprintln(os.Stderr, "  /memory [show|status|clear] — inspect or clear memory")
				fmt.Fprintln(os.Stderr, "  /skills          — list available skills")
				fmt.Fprintln(os.Stderr, "  /skill:<name>    — load and execute a skill (Agent Skills standard)")
				fmt.Fprintln(os.Stderr, "  /unsafe          — toggle auto-approve for bash/write (unsafe mode)")
				fmt.Fprintln(os.Stderr, "  /goal <text>     — set session goal (injected into every LLM turn)")
				fmt.Fprintln(os.Stderr, "  /goal            — show current goal")
				fmt.Fprintln(os.Stderr, "  /goal clear      — clear goal")
				fmt.Fprintln(os.Stderr, "  /help            — show this message")
				fmt.Fprintln(os.Stderr, "  [[               — start multi-line input (end with ]])")
				fmt.Fprintln(os.Stderr, "  exit             — quit REPL")
				continue
			}
			if a.onSlashCmd != nil {
				if !a.onSlashCmd(a, input) {
					break
				}
			} else {
				fmt.Fprintf(os.Stderr, "unknown command %q — type /help\n", input)
			}
			continue
		}
		if err := a.Run(ctx, input); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
}

// extractFirstJSON scans s and returns the substring containing the first
// complete JSON object or array, ignoring any trailing text (e.g. code blocks).
func extractFirstJSON(s string) string {
	depth := 0
	inStr := false
	escape := false
	start := -1
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escape {
			escape = false
			continue
		}
		if inStr {
			if ch == '\\' {
				escape = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{', '[':
			if depth == 0 {
				start = i
			}
			depth++
		case '}', ']':
			depth--
			if depth == 0 && start != -1 {
				candidate := s[start : i+1]
				var v any
				if json.Unmarshal([]byte(candidate), &v) == nil {
					return candidate
				}
				start = -1
			}
		}
	}
	return s
}

// parseContentToolCalls tries to extract tool calls from a text content response.
// Some models (e.g. older Ollama builds) output tool calls as JSON text instead
// of using the structured tool_calls API field.
//
// Supported formats:
//
//	{"name": "...", "arguments": {...}}
//	[{"name": "...", "arguments": {...}}, ...]
//	```json\n{"name": "...", ...}\n```
func parseContentToolCalls(content string) []llm.ToolCall {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	// Strip markdown code fences if present.
	if strings.HasPrefix(content, "```") {
		start := strings.Index(content, "\n")
		end := strings.LastIndex(content, "```")
		if start != -1 && end > start {
			content = strings.TrimSpace(content[start+1 : end])
		}
	}

	// Extract only the first complete JSON object/array from content.
	// Models sometimes append a code block or explanation after the JSON.
	content = extractFirstJSON(content)

	// normaliseArgs converts a RawMessage (object or JSON string) to a JSON string.
	normaliseArgs := func(raw json.RawMessage) string {
		if len(raw) == 0 {
			return "{}"
		}
		if raw[0] == '"' {
			// Already a JSON string — unwrap.
			var s string
			if json.Unmarshal(raw, &s) == nil {
				return s
			}
		}
		if json.Valid(raw) {
			return string(raw)
		}
		return "{}"
	}

	makeCall := func(name string, args json.RawMessage) llm.ToolCall {
		return llm.ToolCall{
			ID:   fmt.Sprintf("call_text_%s", name),
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      name,
				Arguments: normaliseArgs(args),
			},
		}
	}

	// Format A: {"name": "...", "arguments": {...}}
	type fmtA struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	// Format B: {"function": {"name": "...", "arguments": {...}}}
	type fmtB struct {
		Function fmtA `json:"function"`
	}

	parseOne := func(data []byte) (llm.ToolCall, bool) {
		var a fmtA
		if err := json.Unmarshal(data, &a); err == nil && a.Name != "" {
			return makeCall(a.Name, a.Arguments), true
		}
		var b fmtB
		if err := json.Unmarshal(data, &b); err == nil && b.Function.Name != "" {
			return makeCall(b.Function.Name, b.Function.Arguments), true
		}
		return llm.ToolCall{}, false
	}

	// Try single object first.
	if tc, ok := parseOne([]byte(content)); ok {
		return []llm.ToolCall{tc}
	}

	// Try array of objects.
	var raws []json.RawMessage
	if err := json.Unmarshal([]byte(content), &raws); err == nil {
		var calls []llm.ToolCall
		for _, raw := range raws {
			if tc, ok := parseOne(raw); ok {
				calls = append(calls, tc)
			}
		}
		if len(calls) > 0 {
			return calls
		}
	}

	return nil
}

// summarize shortens s for display (collapses newlines, truncates).
func summarize(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/zephel01/luna-go/internal/agent"
	"github.com/zephel01/luna-go/internal/config"
	"github.com/zephel01/luna-go/internal/llm"
	"github.com/zephel01/luna-go/internal/memory"
	"github.com/zephel01/luna-go/internal/skills"
	"github.com/zephel01/luna-go/internal/tools"
)

const version = "0.3.0"

// Execute is the main entrypoint called from main.go.
func Execute() {
	model     := flag.String("model", "", "LLM model (default: qwen2.5-coder:7b)")
	provider  := flag.String("provider", "", "Provider: ollama | openai (default: ollama)")
	baseURL   := flag.String("base-url", "", "API base URL (default: http://localhost:11434)")
	apiKey    := flag.String("api-key", "", "API key (default: $OPENAI_API_KEY)")
	maxIter   := flag.Int("max-iter", 0, "Max tool iterations (default: 20)")
	unsafe    := flag.Bool("unsafe", false, "Skip confirmation for bash and write")
	stream    := flag.Bool("stream", false, "Enable streaming output (experimental)")
	noContext := flag.Bool("no-context", false, "Skip loading .luna-context.md")
	ver       := flag.Bool("version", false, "Print version and exit")

	flag.BoolVar(ver, "v", false, "Print version and exit (shorthand)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: luna [flags] [prompt]\n\n")
		fmt.Fprintf(os.Stderr, "  prompt    Run one-shot and exit. Omit for interactive REPL.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment:\n")
		fmt.Fprintf(os.Stderr, "  LUNA_MODEL      Override --model\n")
		fmt.Fprintf(os.Stderr, "  LUNA_PROVIDER   Override --provider\n")
		fmt.Fprintf(os.Stderr, "  LUNA_BASE_URL   Override --base-url\n")
		fmt.Fprintf(os.Stderr, "  OPENAI_API_KEY  Used when provider=openai\n")
	}
	flag.Parse()

	if *ver {
		fmt.Println("luna-go v" + version)
		os.Exit(0)
	}

	// Subcommand dispatch: luna dream / luna memory / luna config
	if args := flag.Args(); len(args) > 0 {
		switch args[0] {
		case "dream":
			cfg, err := config.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "config error: %v\n", err)
				os.Exit(1)
			}
			runDream(cfg, args[1:])
			return
		case "memory":
			runMemory(args[1:])
			return
		case "config":
			runConfig(args[1:])
			return
		}
	}

	// Load base config from ~/.luna-go/config.yaml
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	// CLI flags take priority over config file.
	if *model != ""    { cfg.Model = *model }
	if *provider != "" { cfg.Provider = *provider }
	if *baseURL != ""  { cfg.BaseURL = *baseURL }
	if *apiKey != ""   { cfg.APIKey = *apiKey }
	if *maxIter != 0   { cfg.MaxIter = *maxIter }
	if *unsafe         { cfg.Unsafe = true }
	if *stream         { cfg.Stream = true }

	// Environment variables fill gaps not already set by flags.
	if v := os.Getenv("LUNA_MODEL"); v != "" && *model == ""       { cfg.Model = v }
	if v := os.Getenv("LUNA_PROVIDER"); v != "" && *provider == "" { cfg.Provider = v }
	if v := os.Getenv("LUNA_BASE_URL"); v != "" && *baseURL == ""  { cfg.BaseURL = v }
	if v := os.Getenv("OPENAI_API_KEY"); v != "" && cfg.APIKey == "" { cfg.APIKey = v }

	// Memory store for this project (used for buffer capture + context inject).
	store := newStore()

	// Load project context: .luna-context.md → memory/context.md → ~/.luna-go/context.md
	extraContext := ""
	if !*noContext {
		extraContext = loadContextFile(store)
	}

	// Discover Agent Skills (compatible with Claude Code / Cowork / Pi).
	loadedSkills := skills.Load()
	if len(loadedSkills) > 0 {
		fmt.Fprintf(os.Stderr, "🔧 loaded %d skill(s): ", len(loadedSkills))
		names := make([]string, len(loadedSkills))
		for i, s := range loadedSkills {
			names[i] = s.Name
		}
		fmt.Fprintln(os.Stderr, strings.Join(names, ", "))
	}

	// Build LLM client (single OpenAI-compatible implementation for both Ollama and OpenAI).
	var client llm.Client = llm.NewOpenAIClient(cfg.BaseURL, cfg.APIKey, cfg.Model, llm.ClientOptions{
		NumCtx:     cfg.NumCtx,
		NumPredict: cfg.NumPredict,
	})

	// Build tool registry. Keep references to tools that need confirm injection.
	bashTool  := tools.NewBashTool(cfg.Unsafe)
	writeTool := tools.NewWriteTool(cfg.Unsafe)
	reg := tools.NewRegistry()
	reg.Register(tools.NewReadTool())
	reg.Register(writeTool)
	reg.Register(tools.NewEditTool())
	reg.Register(bashTool)
	reg.Register(tools.NewGrepTool())

	// Resolve loop timeout from config.
	// LoopTimeoutMin: 0 = default (30 min), negative = no limit.
	var loopTimeout time.Duration
	switch {
	case cfg.LoopTimeoutMin < 0:
		loopTimeout = -1 // signals "no limit" to agent.New
	case cfg.LoopTimeoutMin > 0:
		loopTimeout = time.Duration(cfg.LoopTimeoutMin) * time.Minute
	// 0 → leave as 0, agent.New applies 30 min default
	}

	// Build agent.
	a := agent.New(client, reg, agent.Options{
		MaxIter:      cfg.MaxIter,
		Stream:       cfg.Stream,
		ExtraContext:  extraContext,
		SkillsBlock:  skills.SystemPromptBlock(loadedSkills),
		SessionLog:   sessionLogPath(),
		LoopTimeout:  loopTimeout,
		OnSlashCmd:   makeSlashHandler(cfg, loadedSkills, func() {}), // toggleUnsafe wired below
	})

	// Wire liner-aware confirmation into tools that prompt the user.
	// autoApprove is shared via closure with the slash handler (/unsafe toggle).
	autoApprove := cfg.Unsafe // inherit --unsafe flag at startup
	confirmFn := func(prompt string) bool {
		if autoApprove {
			fmt.Fprintf(os.Stderr, "%sy\n", prompt) // echo auto-y
			return true
		}
		// prompt already contains [y/a/N] — pass it directly.
		line, err := a.ReadLine(prompt)
		if err != nil {
			return false
		}
		ans := strings.TrimSpace(strings.ToLower(line))
		if ans == "a" || ans == "all" {
			autoApprove = true
			fmt.Fprintln(os.Stderr, "⚠  unsafe mode ON — 以降の bash/write は自動承認")
			return true
		}
		return ans == "y" || ans == "yes"
	}
	bashTool.SetConfirm(confirmFn)
	writeTool.SetConfirm(confirmFn)

	// Expose /unsafe toggle to the slash handler via makeSlashHandler.
	toggleUnsafe := func() {
		autoApprove = !autoApprove
		if autoApprove {
			fmt.Fprintln(os.Stderr, "⚠  unsafe mode ON — 以降の bash/write は自動承認")
		} else {
			fmt.Fprintln(os.Stderr, "✓  unsafe mode OFF — 確認を再有効化")
		}
	}

	// Determine session ID for buffer capture.
	sessionID := time.Now().Format("20060102-150405")

	// One-shot mode when a prompt is provided as arguments.
	if args := flag.Args(); len(args) > 0 {
		prompt := strings.Join(args, " ")
		if err := a.Run(context.Background(), prompt); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		a.SaveSession()
		captureToBuffer(store, a.History(), sessionID)
		return
	}

	// Register tab-completion candidates for the REPL.
	a.SetCompleter(makeCompleter(loadedSkills))

	// Re-wire slash handler now that toggleUnsafe is available.
	a.SetSlashHandler(makeSlashHandler(cfg, loadedSkills, toggleUnsafe))

	// Interactive REPL mode.
	a.REPL(context.Background())
	captureToBuffer(store, a.History(), sessionID)
}

// loadContextFile reads and combines context from multiple sources (priority order):
//  1. cwd/.luna-context.md        (user-maintained, project-specific)
//  2. memory store context.md     (auto-generated by `luna dream`)
//  3. ~/.luna-go/context.md       (global fallback)
//
// Sources 1 and 2 are combined if both exist.
func loadContextFile(store *memory.Store) string {
	var parts []string

	// 1. User-maintained project context
	if data, err := os.ReadFile(".luna-context.md"); err == nil {
		fmt.Fprintln(os.Stderr, "📎 loaded .luna-context.md")
		parts = append(parts, string(data))
	}

	// 2. Auto-generated dream context
	if store != nil {
		if ctx, err := store.ReadContext(); err == nil && ctx != "" {
			fmt.Fprintf(os.Stderr, "🧠 loaded memory/context.md (%s)\n", store.Project)
			parts = append(parts, ctx)
		}
	}

	if len(parts) > 0 {
		return strings.Join(parts, "\n\n---\n\n")
	}

	// 3. Global fallback
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	globalPath := filepath.Join(home, ".luna-go", "context.md")
	if data, err := os.ReadFile(globalPath); err == nil {
		fmt.Fprintln(os.Stderr, "📎 loaded ~/.luna-go/context.md")
		return string(data)
	}
	return ""
}

// sessionLogPath returns ~/.luna-go/sessions/<timestamp>.jsonl
func sessionLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	ts := time.Now().Format("2006-01-02_15-04-05")
	return filepath.Join(home, ".luna-go", "sessions", ts+".jsonl")
}

// --- Slash command handler ---

// makeCompleter returns a tab-completion function for the REPL.
// It covers all built-in slash commands and dynamically adds /skill:<name> entries.
func makeCompleter(loadedSkills []skills.Skill) func(string) []string {
	base := []string{
		"/models",
		"/dream",
		"/memory", "/memory show", "/memory status", "/memory clear",
		"/skills",
		"/unsafe",
		"/goal", "/goal clear",
		"/help",
		"exit",
	}
	return func(line string) []string {
		// Build candidate list: base + /skill:<name> per loaded skill
		candidates := make([]string, len(base))
		copy(candidates, base)
		for _, s := range loadedSkills {
			candidates = append(candidates, "/skill:"+s.Name)
		}
		// Filter by prefix
		var matches []string
		for _, c := range candidates {
			if strings.HasPrefix(c, line) {
				matches = append(matches, c)
			}
		}
		return matches
	}
}

// makeSlashHandler returns a handler for REPL slash commands.
func makeSlashHandler(cfg *config.Config, loadedSkills []skills.Skill, toggleUnsafe func()) func(a *agent.Agent, cmd string) bool {
	return func(a *agent.Agent, cmd string) bool {
		switch {
		case cmd == "/unsafe":
			toggleUnsafe()
		case cmd == "/goal" || strings.HasPrefix(cmd, "/goal "):
			arg := strings.TrimSpace(strings.TrimPrefix(cmd, "/goal"))
			switch arg {
			case "":
				if g := a.Goal(); g != "" {
					fmt.Fprintf(os.Stderr, "🎯 goal: %s\n", g)
				} else {
					fmt.Fprintln(os.Stderr, "goal is not set — use /goal <text> to set one")
				}
			case "clear":
				a.ClearGoal()
				fmt.Fprintln(os.Stderr, "goal cleared")
			default:
				a.SetGoal(arg)
				fmt.Fprintf(os.Stderr, "🎯 goal set: %s\n", arg)
			}
		case cmd == "/models":
			handleModels(a, cfg)
		case cmd == "/dream":
			slashDream(a, cfg, cmd)
		case cmd == "/memory" || strings.HasPrefix(cmd, "/memory "):
			sub := strings.TrimPrefix(strings.TrimPrefix(cmd, "/memory"), " ")
			slashMemory(a, sub)
		case cmd == "/skills":
			handleSkillsList(loadedSkills)
		case strings.HasPrefix(cmd, "/skill:"):
			name := strings.TrimPrefix(cmd, "/skill:")
			// Support "/skill:name extra args"
			parts := strings.SplitN(name, " ", 2)
			skillName := strings.TrimSpace(parts[0])
			args := ""
			if len(parts) > 1 {
				args = strings.TrimSpace(parts[1])
			}
			handleSkillRun(a, loadedSkills, skillName, args)
		default:
			fmt.Fprintf(os.Stderr, "unknown command %q — type /help\n", cmd)
		}
		return true // always continue REPL
	}
}

// handleSkillsList prints all available skills.
func handleSkillsList(loadedSkills []skills.Skill) {
	if len(loadedSkills) == 0 {
		fmt.Fprintln(os.Stderr, "no skills found — place SKILL.md files in .agents/skills/ or ~/.claude/skills/")
		return
	}
	fmt.Fprintln(os.Stderr, "\nAvailable skills:")
	for _, s := range loadedSkills {
		fmt.Fprintf(os.Stderr, "  /skill:%-24s %s\n", s.Name, s.Description)
	}
}

// handleSkillRun loads the full SKILL.md and sends it to the agent as a user message.
func handleSkillRun(a *agent.Agent, loadedSkills []skills.Skill, name, args string) {
	s, ok := skills.Find(loadedSkills, name)
	if !ok {
		fmt.Fprintf(os.Stderr, "skill %q not found — use /skills to list available skills\n", name)
		return
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading skill: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "📦 loading skill: %s\n", s.Name)
	prompt := "Load and follow the instructions in this skill:\n\n" + string(data)
	if args != "" {
		prompt += "\n\nUser: " + args
	}
	if err := a.Run(context.Background(), prompt); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
}

// handleModels lists Ollama models and lets the user switch interactively.
func handleModels(a *agent.Agent, cfg *config.Config) {
	models, err := listOllamaModels(cfg.BaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing models: %v\n", err)
		return
	}
	if len(models) == 0 {
		fmt.Fprintln(os.Stderr, "no models found (is Ollama running?)")
		return
	}

	ram := getSystemRAM()
	if ram > 0 {
		fmt.Fprintf(os.Stderr, "\nAvailable Ollama models:  (RAM: %s)\n", formatSize(ram))
	} else {
		fmt.Fprintln(os.Stderr, "\nAvailable Ollama models:")
	}
	for i, m := range models {
		tag := modelTag(m.Size, ram)
		fmt.Fprintf(os.Stderr, "  %d) %-34s %-8s  %s\n", i+1, m.Name, formatSize(m.Size), tag)
	}
	// Use a.ReadLine so liner's raw-mode terminal is handled correctly.
	choice, err := a.ReadLine(fmt.Sprintf("Select [1-%d] (Enter to cancel): ", len(models)))
	if err != nil {
		return
	}
	choice = strings.TrimSpace(choice)
	if choice == "" {
		return
	}
	n, err := strconv.Atoi(choice)
	if err != nil || n < 1 || n > len(models) {
		fmt.Fprintln(os.Stderr, "invalid selection")
		return
	}

	selected := models[n-1].Name
	cfg.Model = selected
	newClient := llm.NewOpenAIClient(cfg.BaseURL, cfg.APIKey, cfg.Model, llm.ClientOptions{
		NumCtx:     cfg.NumCtx,
		NumPredict: cfg.NumPredict,
	})
	a.SetClient(newClient)
	fmt.Fprintf(os.Stderr, "✔ switched to %s\n\n", selected)
}

// ollamaModel holds a single model entry from the Ollama /api/tags response.
type ollamaModel struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// listOllamaModels fetches the model list from the Ollama REST API.
func listOllamaModels(baseURL string) ([]ollamaModel, error) {
	// baseURL may end with /v1 — strip it to get the Ollama root.
	base := strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1")

	resp, err := http.Get(base + "/api/tags") //nolint:noctx
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
		Models []ollamaModel `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Models, nil
}

// formatSize converts bytes to a human-readable string.
func formatSize(bytes int64) string {
	gb := float64(bytes) / 1e9
	if gb >= 1 {
		return fmt.Sprintf("%.1f GB", gb)
	}
	return fmt.Sprintf("%.0f MB", float64(bytes)/1e6)
}

// getSystemRAM returns total system RAM in bytes (0 if unknown).
// Supports macOS (sysctl) and Linux (/proc/meminfo).
func getSystemRAM() int64 {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0
		}
		n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			return 0
		}
		return n
	case "linux":
		data, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) < 2 {
					return 0
				}
				kb, err := strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					return 0
				}
				return kb * 1024
			}
		}
	}
	return 0
}

// modelTag returns a recommendation label based on model size vs system RAM.
// Apple Silicon uses unified memory, so model size vs total RAM is the right metric.
func modelTag(modelSize, ramBytes int64) string {
	if ramBytes == 0 {
		return ""
	}
	ratio := float64(modelSize) / float64(ramBytes)
	switch {
	case ratio < 0.6:
		return "✅ 推奨"
	case ratio < 0.9:
		return "⚠️  重い可能性あり"
	default:
		return "❌ RAM 不足"
	}
}

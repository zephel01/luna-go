package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zephel01/luna-go/internal/agent"
	"github.com/zephel01/luna-go/internal/llm"
	"github.com/zephel01/luna-go/internal/memory"
)

// projectName returns the current directory basename as the project identifier.
func projectName() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "default"
	}
	return filepath.Base(cwd)
}

// newStore creates a memory.Store for the current project (best-effort, nil on error).
func newStore() *memory.Store {
	s, err := memory.New(projectName())
	if err != nil {
		return nil
	}
	return s
}

// captureToBuffer appends all assistant responses from history to the project buffer.
func captureToBuffer(store *memory.Store, history []llm.Message, sessionID string) {
	if store == nil {
		return
	}
	for _, m := range history {
		if m.Role == "assistant" && m.Content != "" {
			_ = store.AppendBuffer(m.Content, sessionID)
		}
	}
}

// --- luna dream subcommand ---

// runDream implements `luna dream [--dry-run] [--model <model>]`.
func runDream(cfg interface{ GetBaseURL() string; GetModel() string }, args []string) {
	fs := flag.NewFlagSet("dream", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "Print result without saving")
	model  := fs.String("model", "", "Ollama model to use (default: current model)")
	_ = fs.Parse(args)

	store := newStore()
	if store == nil {
		fmt.Fprintln(os.Stderr, "error: could not create memory store")
		os.Exit(1)
	}

	m := *model
	if m == "" {
		m = cfg.GetModel()
	}
	if err := memory.Dream(store, cfg.GetBaseURL(), m, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "dream error: %v\n", err)
		os.Exit(1)
	}
}

// --- luna memory subcommand ---

// runMemory implements `luna memory <show|status|clear|add <text>>`.
func runMemory(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: luna memory <show|status|clear|add <text>>")
		os.Exit(1)
	}

	store := newStore()
	if store == nil {
		fmt.Fprintln(os.Stderr, "error: could not create memory store")
		os.Exit(1)
	}

	switch args[0] {
	case "show":
		ctx, err := store.ReadContext()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if ctx == "" {
			fmt.Println("(context.md is empty — run `luna dream` first)")
		} else {
			fmt.Println(ctx)
		}

	case "status":
		fmt.Println(store.Status())

	case "clear":
		if err := store.ClearBuffer(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✔ buffer cleared")

	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: luna memory add <text>")
			os.Exit(1)
		}
		text := strings.Join(args[1:], " ")
		if err := store.AppendBuffer(text, "manual"); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✔ added to buffer")

	default:
		fmt.Fprintf(os.Stderr, "unknown memory command %q\n", args[0])
		fmt.Fprintln(os.Stderr, "usage: luna memory <show|status|clear|add <text>>")
		os.Exit(1)
	}
}

// --- REPL slash command handlers for /dream and /memory ---

func slashDream(a *agent.Agent, cfg interface{ GetBaseURL() string; GetModel() string }, _ string) {
	store := newStore()
	if store == nil {
		fmt.Fprintln(os.Stderr, "error: could not create memory store")
		return
	}
	if err := memory.Dream(store, cfg.GetBaseURL(), cfg.GetModel(), false); err != nil {
		fmt.Fprintf(os.Stderr, "dream error: %v\n", err)
	}
}

func slashMemory(a *agent.Agent, sub string) {
	store := newStore()
	if store == nil {
		fmt.Fprintln(os.Stderr, "error: could not create memory store")
		return
	}
	switch strings.TrimSpace(sub) {
	case "show":
		ctx, _ := store.ReadContext()
		if ctx == "" {
			fmt.Fprintln(os.Stderr, "(context.md is empty — run /dream first)")
		} else {
			fmt.Fprintln(os.Stderr, ctx)
		}
	case "status", "":
		fmt.Fprintln(os.Stderr, store.Status())
	case "clear":
		_ = store.ClearBuffer()
		fmt.Fprintln(os.Stderr, "✔ buffer cleared")
	default:
		fmt.Fprintf(os.Stderr, "unknown: /memory %s\n", sub)
		fmt.Fprintln(os.Stderr, "usage: /memory [show|status|clear]")
	}
}

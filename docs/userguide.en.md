# Luna User Guide

> A minimal AI coding agent built for Ollama

---

## Table of Contents

1. [Installing Ollama](#1-installing-ollama)
2. [Installing luna-go](#2-installing-luna-go)
3. [Configuration](#3-configuration)
4. [Basic Usage](#4-basic-usage)
5. [REPL Interactive Mode](#5-repl-interactive-mode)
6. [Project Context](#6-project-context)
7. [Session Memory (Dreaming-lite)](#7-session-memory-dreaming-lite)
8. [Agent Skills](#8-agent-skills)
9. [Autonomous Mode (/goal)](#9-autonomous-mode-goal)
10. [Using Cloud APIs](#10-using-cloud-apis)
11. [Combining with CodeRouter](#11-combining-with-coderouter)
12. [Troubleshooting](#12-troubleshooting)

---

## 1. Installing Ollama

Luna uses [Ollama](https://ollama.com) to run local LLMs by default.

### macOS

```bash
brew install ollama
```

Or download the installer from [ollama.com/download](https://ollama.com/download).

### Linux

```bash
curl -fsSL https://ollama.com/install.sh | sh
```

### Start Ollama

The macOS installer app starts automatically from the menu bar. To start manually:

```bash
ollama serve
```

Verify it's running:

```bash
curl http://localhost:11434/api/tags
# {"models":[...]} means it's working
```

### Pull a Model

Pull the model you want Luna to use. Choose based on your available RAM.

| Model | Size | RAM | Notes |
|-------|------|-----|-------|
| `qwen2.5-coder:7b` | 4.7 GB | 8 GB+ | Lightweight, stable. Fallback parsing supported |
| `qwen3.5:9b` | 5.8 GB | 16 GB+ | Recommended balance of speed and quality |
| `qwen3.5:35b-a3b` | 22 GB | 32 GB+ | Best quality |

```bash
ollama pull qwen2.5-coder:7b
```

> **Tip:** In the Luna REPL, type `/models` to see all installed models with RAM-based recommendations, and switch between them on the fly.

---

## 2. Installing luna-go

### Download Binary (Recommended)

```bash
# macOS (Apple Silicon)
curl -L https://github.com/zephel01/luna-go/releases/latest/download/luna-darwin-arm64 \
  -o luna && chmod +x luna && sudo mv luna /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/zephel01/luna-go/releases/latest/download/luna-darwin-amd64 \
  -o luna && chmod +x luna && sudo mv luna /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/zephel01/luna-go/releases/latest/download/luna-linux-amd64 \
  -o luna && chmod +x luna && sudo mv luna /usr/local/bin/
```

### Build from Source

Requires Go 1.22 or later.

```bash
git clone https://github.com/zephel01/luna-go
cd luna-go
go build -o luna .
sudo mv luna /usr/local/bin/
```

### Verify Installation

```bash
luna --version
# luna-go v0.4.5
```

---

## 3. Configuration

### Config File (Optional)

Create `~/.luna-go/config.yaml` to avoid passing flags every time.

```yaml
# ~/.luna-go/config.yaml
model: qwen2.5-coder:7b
provider: ollama
base_url: http://localhost:11434

# Optional settings
max_iter: 20            # Max tool iterations per request
unsafe: false           # true = skip bash/write confirmation
stream: false           # true = enable streaming output
num_ctx: 0              # Context length (0 = auto: 32768 for Ollama)
loop_timeout_min: 30    # Wall-clock timeout in minutes (0 = default 30, -1 = no limit)
```

Generate a template automatically:

```bash
luna config init
```

### Environment Variables

Environment variables override the config file; CLI flags override environment variables.

| Variable | Description |
|----------|-------------|
| `LUNA_MODEL` | Model name |
| `LUNA_PROVIDER` | `ollama` or `openai` |
| `LUNA_BASE_URL` | API endpoint URL |
| `OPENAI_API_KEY` | API key for OpenAI-compatible APIs |

---

## 4. Basic Usage

### One-Shot Mode

Pass a prompt as an argument to run once and exit. Useful for scripts and CI.

```bash
# Read and explain a file
luna "read main.go and explain what Parse does"

# Fix a bug
luna "read internal/llm/client.go and fix the error handling in the Stream method"

# Generate tests
luna "read internal/tools/bash.go and write unit tests for Execute"

# Refactor
luna "read cmd/root.go and extract the model selection logic into a separate function"
```

### Flags

```bash
luna [flags] [prompt]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--model` | Model to use | `qwen2.5-coder:7b` |
| `--provider` | `ollama` \| `openai` | `ollama` |
| `--base-url` | API endpoint | `http://localhost:11434` |
| `--api-key` | API key | `$OPENAI_API_KEY` |
| `--max-iter` | Max tool iterations | `20` |
| `--unsafe` | Skip bash / write confirmation | `false` |
| `--stream` | Streaming output (experimental) | `false` |
| `--no-context` | Skip loading `.luna-context.md` | `false` |
| `-v` / `--version` | Print version and exit | — |

### Subcommands

```bash
luna dream               # Distill session memory into context.md
luna dream --dry-run     # Preview output without saving

luna memory show         # Show current context.md
luna memory status       # Show buffer entry count and size
luna memory clear        # Delete buffer and context.md

luna config show         # Show current effective config
luna config init         # Generate ~/.luna-go/config.yaml template
```

---

## 5. REPL Interactive Mode

Run without arguments to enter the interactive REPL.

```bash
luna
# 🔧 loaded 2 skill(s): git-commit, code-review
# 📎 loaded .luna-context.md
# Luna v0.4  —  type your request, Tab to complete, Ctrl-C or 'exit' to quit
#
# >
```

### Tab Completion

Type `/` and press Tab to complete available commands.

```
> /mo[TAB]    →    /models
> /sk[TAB]    →    /skills
> /skill:[TAB]  →  /skill:git-commit  /skill:code-review
```

### Arrow Key History

Use ↑ / ↓ to navigate input history within the current session.

### REPL Commands

| Command | Description |
|---------|-------------|
| `/models` | List Ollama models with RAM recommendations; enter a number to switch |
| `/dream` | Distill session buffer into `context.md` via Ollama |
| `/memory [show\|status\|clear]` | Inspect or manage session memory |
| `/skills` | List available Agent Skills |
| `/skill:<name>` | Load and run a skill |
| `/unsafe` | Toggle bash / write auto-approval for the current session |
| `/goal <text>` | Set a session goal; Luna works autonomously until achieved |
| `/goal` | Show current goal |
| `/goal clear` | Clear the goal |
| `/help` | Show all commands |
| `[[` | Start multi-line input mode (end with `]]`) |
| `exit` / `quit` | Quit REPL |

### Multi-Line Input (`[[` / `]]`)

For pasting code or writing long prompts across multiple lines, use `[[` to start multi-line mode and `]]` to submit.

```
> [[
... def fib(n):
...     if n < 2: return n
...     return fib(n-1) + fib(n-2)
... ]]
Please port this Python function to Go.
```

### `/unsafe` — Toggle Confirmation Skip

bash and write tools prompt for confirmation before executing. To skip confirmations for the rest of the session, use `/unsafe`:

```
> /unsafe
⚠  unsafe mode ON — all bash/write will auto-approve

> /unsafe
✓  unsafe mode OFF — confirmation re-enabled
```

You can also type `a` at any confirmation prompt to enable auto-approve from that point on:

```
⚠  bash: go test ./...
Run? [y/a/N] a
⚠  unsafe mode ON — all bash/write will auto-approve
```

### `/models` Example

```
> /models

Available Ollama models:  (RAM: 16.0 GB)
  1) qwen2.5-coder:7b             4.7 GB   ✅ Recommended
  2) qwen3.5:9b                   5.8 GB   ✅ Recommended
  3) qwen3.5:35b-a3b              22 GB    ❌ Insufficient RAM

Select [1-3] (Enter to cancel): 2
✔ switched to qwen3.5:9b
```

---

## 6. Project Context

Place a `.luna-context.md` file in your project root and Luna loads it automatically at startup, injecting its contents into the system prompt. No need to re-explain your project every session.

```markdown
# .luna-context.md

## Project Overview
Go 1.24 CLI tool. Packages: cmd/, internal/llm/, internal/tools/, internal/agent/

## Development Rules
- Tests: `go test ./...`
- Format: `gofmt -w .`
- Match existing patterns when adding new files
- Keep external dependencies minimal

## Common Commands
- Build: `go build -o luna .`
- CI locally: `go vet ./... && go test ./...`
```

**Load priority:**

1. `cwd/.luna-context.md` (project-specific)
2. `~/.luna-go/memory/<project>/context.md` (generated by `luna dream`)
3. `~/.luna-go/memory/<project>/experience.md` (generated by `luna dream` — implicit knowledge)
4. `~/.luna-go/context.md` (global fallback)

Sources 1–3 are combined when they all exist. The global fallback is only used when none of 1–3 are present.

---

## 7. Session Memory (Dreaming-lite)

Luna automatically saves session conversations to a buffer. Run `luna dream` to let a local Ollama model distill the buffer into three types of files. These are injected automatically on the next startup.

### What Dream Produces

| File | Contents | How it's used |
|------|----------|---------------|
| `context.md` | Project-specific decisions, structure, and rules | Injected into system prompt |
| `experience.md` | Implicit knowledge, intuitions, "secret sauce" | Injected into system prompt |
| `skills/<name>/SKILL.md` | Reusable procedures (`scope: project` or `global`) | Auto-discovered as skills |

### Directory Layout

```
~/.luna-go/
  config.yaml
  skills/                       ← global-scope skills written by luna dream
    <skill-name>/
      SKILL.md
  memory/
    <project-name>/
      context.md      ← project-specific knowledge (auto-injected at startup)
      experience.md   ← implicit knowledge / secret sauce (auto-injected at startup)
      buffer.jsonl    ← session buffer (consumed by dream)
      skills/         ← project-scope skills written by luna dream
        <skill-name>/
          SKILL.md
  sessions/
    2026-05-17_10-30-00.jsonl   ← full session log
```

### Typical Workflow

```bash
# During a session
luna
> refactor the Stream method to use context cancellation
> add a timeout to the HTTP client in llm/client.go
> exit

# After work
luna dream
# 🌙 dreaming over 3 buffer entries with qwen2.5-coder:7b ...
#    📝 context.md updated
#    ✨ experience.md updated
#    🔧 skill "go-http-timeout" saved (project)
# ✅ dream complete (3 entries processed, buffer cleared)

# Next day
luna
# 🧠 loaded memory/context.md (luna-go)   ← resumes with yesterday's context
# ✨ loaded memory/experience.md (luna-go) ← implicit knowledge injected
# 🔧 loaded 1 skill(s): go-http-timeout   ← extracted skill auto-discovered
```

### Skill Scope

Skills generated by Dream have a `scope`:

- `scope: project` (default) — saved to `~/.luna-go/memory/<project>/skills/`. Only active for this project.
- `scope: global` — saved to `~/.luna-go/skills/`. Available across all projects.

### Managing Memory

```bash
luna memory show    # Display current context.md
luna memory status  # Show buffer count, context/experience line counts, and skill count
luna memory clear   # Reset session buffer
```

---

## 8. Agent Skills

Luna auto-discovers `SKILL.md` files and makes them available as `/skill:<name>` commands. Skills written for Claude Code or Cowork work here without modification.

### Skill File Locations

Discovered in this order (project-local takes priority):

```
<project>/.agents/skills/<name>/SKILL.md
~/.luna-go/memory/<project>/skills/<name>/SKILL.md  ← generated by luna dream (project scope)
~/.agents/skills/<name>/SKILL.md
~/.claude/skills/<name>/SKILL.md                    ← shared with Claude Code / Cowork
~/.pi/agent/skills/<name>/SKILL.md
~/.luna-go/skills/<name>/SKILL.md                   ← generated by luna dream (global scope)
```

### SKILL.md Format

```markdown
---
name: git-commit
description: Review staged changes and generate a Conventional Commits message
---

## Steps

1. Run `bash({"command": "git diff --cached"})` to inspect staged changes.
2. Generate a Conventional Commits message based on the diff.
3. Once confirmed, run `bash({"command": "git commit -m '...'"})`.
```

### Using Skills

```bash
# In the REPL
> /skills              # list available skills
> /skill:git-commit    # load and run a skill

# Or use Tab completion
> /skill:[TAB]
```

### Example Skills

The repository includes ready-to-use skills in [`examples/skills/`](../examples/skills/):

| Skill | Description |
|-------|-------------|
| `git-commit` | Staged diff → Conventional Commits message → commit |
| `go-test-fix` | Run `go test`, fix failures, repeat until all pass |
| `go-lint` | Run `go vet` + `golangci-lint`, fix all findings |
| `code-review` | Review a file or git diff and output a structured report |
| `pr-description` | Generate a GitHub PR description from the current branch |

```bash
# Install all example skills
cp -r examples/skills/* ~/.agents/skills/
```

---

## 9. Autonomous Mode (/goal)

Set a session-level goal with `/goal` and Luna keeps working autonomously — looping through tool calls — until it declares the goal achieved.

### Usage

```
> /goal make all tests pass in internal/tools/

🎯 goal set: make all tests pass in internal/tools/
```

With the goal set, Luna repeats:
1. Call tools (read / bash / write / edit)
2. Observe the results
3. Plan the next step
4. Continue until it outputs a message starting with "ゴール達成:"

```
> /goal run go test and fix all errors until tests pass

⚙  bash({"command":"go test ./..."})
... (failure output)
⚙  read({"path":"/path/to/failing_test.go"})
...
⚙  edit({"path":"...", "old_string":"...", "new_string":"..."})
...
⚙  bash({"command":"go test ./..."})
... (all pass)

ゴール達成: All tests pass. Fixed boundary condition in TestReadTool...
```

### Managing Goals

```
> /goal                    # show current goal
> /goal clear              # clear goal (return to normal mode)
```

### Tips

- **Combine with `/unsafe`** — without it, each bash/write call will pause and wait for confirmation, interrupting the loop.
- **Wall-clock timeout** — defaults to 30 minutes. Adjust with `loop_timeout_min` in config:

  ```yaml
  loop_timeout_min: 60   # extend to 60 minutes
  loop_timeout_min: -1   # no limit
  ```

- **Max iterations** — the loop also stops after `max_iter` (default: 20) tool calls. For complex tasks, set `--max-iter 50`.

### Skills + /goal (Recommended Combo)

Skills provide step-by-step instructions that reduce model guesswork, making autonomous loops more reliable — especially with smaller models.

```
> /unsafe
> /goal run golangci-lint and fix all findings
```

Or load a skill directly:

```
> /skill:go-test-fix
```

---

## 10. Using Cloud APIs

Luna works with any OpenAI-compatible API.

### OpenAI

```bash
export OPENAI_API_KEY=sk-...
luna --provider openai --model gpt-4o "refactor this function"
```

### Local OpenAI-Compatible Server (LM Studio, etc.)

```bash
luna --provider openai --base-url http://localhost:1234/v1 --model local-model "..."
```

### Fix in config.yaml

```yaml
provider: openai
model: gpt-4o-mini
base_url: https://api.openai.com/v1
```

---

## 11. Combining with CodeRouter

[CodeRouter](https://github.com/zephel01/CodeRouter) is a routing layer that sits between luna-go and your LLM backends. Since luna-go speaks OpenAI-compatible API, you only need to change `--base-url`.

```
luna-go
  │
  ▼
CodeRouter (localhost:8088)
  │
  ├─→ Ollama (local / free / fastest)
  ├─→ OpenRouter free tier (cloud / free)
  └─→ OpenAI / Anthropic (paid / opt-in only)
```

### When to Use It

| Situation | Use CodeRouter? |
|-----------|----------------|
| Ollama occasionally returns malformed tool calls | **Yes** — CodeRouter repairs broken JSON before forwarding |
| Ollama goes down and you want automatic fallback to cloud | **Yes** — 3-layer fallback runs automatically |
| Long unattended runs (hours) where model quality degrades | **Yes** — 6 guard systems stabilize long sessions |
| Multiple models to route between | **Yes** — switch via profiles |
| Ollama works fine as-is | Not needed |

### Setup

**1. Install and start CodeRouter**

```bash
uvx --from coderouter-cli coderouter serve --port 8088
```

**2. Point luna-go at CodeRouter**

```bash
luna --base-url http://localhost:8088/v1 --model qwen2.5-coder:7b "fix the bug"
```

Or set in `config.yaml`:

```yaml
base_url: http://localhost:8088/v1
model: qwen2.5-coder:7b
```

### CodeRouter Config Example

```yaml
# ~/.coderouter/providers.yaml
default_profile: local-first

profiles:
  - name: local-first
    providers: [ollama-local, openrouter-free]

providers:
  - name: ollama-local
    kind: openai_compat
    base_url: http://localhost:11434/v1
    model: qwen2.5-coder:7b

  - name: openrouter-free
    kind: openai_compat
    base_url: https://openrouter.ai/api/v1
    model: qwen/qwen2.5-coder:7b:free
    api_key_env: OPENROUTER_API_KEY
```

### luna-go Alone vs. With CodeRouter

| Feature | luna-go alone | luna-go + CodeRouter |
|---------|:-------------:|:--------------------:|
| Ollama direct connection | ✅ | ✅ (via CodeRouter) |
| Tool call repair | ✅ built-in fallback parser | ✅ both sides |
| Automatic fallback | ❌ | ✅ local → free cloud → paid |
| Long-run stability guards | ❌ | ✅ 6 systems |
| `/dashboard` monitoring | ❌ | ✅ |
| Setup overhead | none | CodeRouter must be running |

> See the [CodeRouter documentation](https://github.com/zephel01/CodeRouter) for full details.

---

## 12. Troubleshooting

### Cannot connect to Ollama

```
error: llm: Post "http://localhost:11434/v1/chat/completions": dial tcp ...
```

```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# Start it if not
ollama serve
```

On macOS, check the Ollama icon in the menu bar.

---

### Model not found

```
error: model "qwen3.5:9b" not found
```

```bash
ollama pull qwen3.5:9b
```

Or run `/models` in the REPL to see what's installed.

---

### Tool loop doesn't stop / hits max iterations

```
error: max iterations (20) reached without a final answer
```

- The model may be calling the same tool repeatedly — try a smarter model (`/models`)
- The task may be too complex — break it into smaller steps
- Extend the limit: `--max-iter 50`

---

### Loop timeout during /goal

```
error: loop timeout (30m elapsed): context deadline exceeded
```

The 30-minute wall-clock limit was reached. Increase it in config:

```yaml
loop_timeout_min: 60   # 60 minutes
```

Or set to `-1` to disable entirely.

---

### bash / write asks for confirmation on every command

```
⚠  bash: go test ./...
Run? [y/a/N]
```

This is the default safety behavior. Options to suppress it:

- **Auto-approve the rest of the session**: type `a` at the prompt
- **Toggle in REPL**: run `/unsafe`
- **At startup**: use `--unsafe` flag
- **Permanently**: add `unsafe: true` to `config.yaml`

```bash
luna --unsafe "run go test ./... and fix failing tests"
```

---

### Context seems short / information gets dropped mid-session

Increase `num_ctx` in your config:

```yaml
num_ctx: 32768
```

More context means more RAM usage and slower responses. Luna already sets `num_ctx: 32768` automatically for Ollama endpoints — this is only needed if you want a different value.

---

### Streaming output looks garbled

`--stream` is experimental. If you see issues, run without it (non-streaming is the default).

---

### `.luna-context.md` not being loaded

```bash
# Make sure --no-context flag is not set
luna --no-context   # this flag suppresses context loading

# Verify the file exists in the right place
ls .luna-context.md

# Check startup logs for "loaded .luna-context.md"
luna   # printed to stderr on startup
```

---

### govulncheck warnings in CI

```
llm.OpenAIClient.post calls http.Client.Do, which eventually calls tls.Conn.Write
```

This is not a bug in luna-go itself — it's a CVE reported against Go's standard library (`crypto/tls`, `net/http`). Upgrading the Go version resolves it. If CI is green overall, the release is safe to ship.

---

## Appendix: File and Directory Layout

```
~/.luna-go/
  config.yaml                   # Global config
  context.md                    # Global fallback context
  skills/                       # Global-scope skills (generated by luna dream)
    <skill-name>/
      SKILL.md
  memory/
    <project>/
      context.md                # Project-specific knowledge (generated by luna dream)
      experience.md             # Implicit knowledge / secret sauce (generated by luna dream)
      buffer.jsonl              # Session buffer
      skills/                   # Project-scope skills (generated by luna dream)
        <skill-name>/
          SKILL.md
  sessions/
    YYYY-MM-DD_HH-MM-SS.jsonl  # Full session logs

~/.claude/skills/               # Skills shared with Claude Code / Cowork
~/.agents/skills/               # Agent Skills standard location

<project>/
  .luna-context.md              # Project-specific context (highest priority)
  .agents/skills/               # Project-local skills
```

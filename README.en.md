<div align="center">

# 🌙 luna-go

**A minimal AI coding agent built for Ollama**

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Ollama](https://img.shields.io/badge/Ollama-first-blueviolet)](https://ollama.com)
[![CI](https://github.com/zephel01/luna-go/actions/workflows/ci.yml/badge.svg)](https://github.com/zephel01/luna-go/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/zephel01/luna-go/branch/main/graph/badge.svg)](https://codecov.io/gh/zephel01/luna-go)

English | [日本語](README.md)

</div>

---

```
$ luna "read main.go and refactor the Parse function"

⚙  read({"path": "/project/main.go"})
⚙  edit({"path": "/project/main.go", "old_string": "...", "new_string": "..."})
⚙  bash({"command": "go test ./..."})

All tests pass. Refactored Parse to use a switch statement.
```

---

## Why luna-go?

Most AI coding tools require a cloud subscription, an npm install, or a Python environment. Luna doesn't.

|  | **luna-go** | Claude Code | Aider | Pi |
|--|:-----------:|:-----------:|:-----:|:--:|
| Distribution | **single binary** | npm | pip | TypeScript |
| Default backend | **Ollama (local)** | Anthropic API | Any | Ollama |
| Offline capable | ✅ | ❌ | ✅ | ✅ |
| Tools | **5** | 30+ | git + edit | 4 |
| Session memory | ✅ Dreaming-lite | ✅ | ❌ | ❌ |
| REPL readline | ✅ liner | ✅ | ✅ | ❌ |
| Autonomous loop | ✅ `/goal` | ✅ | ❌ | ❌ |
| Agent Skills | ✅ | ✅ | ❌ | ❌ |

> A smart enough model doesn't need 30 tools. Five is enough.

---

## Install

### Download binary

```bash
# macOS (Apple Silicon)
curl -L https://github.com/zephel01/luna-go/releases/latest/download/luna-darwin-arm64 \
  -o luna && chmod +x luna && sudo mv luna /usr/local/bin/
```

<details>
<summary>Other platforms</summary>

```bash
# macOS (Intel)
curl -L https://github.com/zephel01/luna-go/releases/latest/download/luna-darwin-amd64 \
  -o luna && chmod +x luna && sudo mv luna /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/zephel01/luna-go/releases/latest/download/luna-linux-amd64 \
  -o luna && chmod +x luna && sudo mv luna /usr/local/bin/
```

</details>

### Build from source

```bash
git clone https://github.com/zephel01/luna-go
cd luna-go && make build   # → ./luna
```

> **Note:** Running `go build` without `-o` produces a `luna-go` binary (from the module name).
> Always use `make build` or `go build -o luna .`.

Requires Go 1.22+. Two external dependencies: `gopkg.in/yaml.v3` + `github.com/peterh/liner`.

---

## Quick Start

**1. Install Ollama and pull a model**

```bash
ollama pull qwen3.5:9b
```

**2. Create a config file (optional)**

```yaml
# ~/.luna-go/config.yaml
model: qwen3.5:9b
provider: ollama
base_url: http://localhost:11434
```

**3. Run**

```bash
# One-shot
luna "grep all TODO comments and summarize them"

# Interactive REPL
luna
```

---

## Recommended Models

| Model | Size | Notes |
|-------|------|-------|
| `qwen3.5:35b-a3b` | 22 GB | **Best quality** — large RAM environments |
| `qwen3.5:9b` | 5.8 GB | **Recommended** — reliable tool calling, great balance |
| `qwen3.6:35b-a3b-coding-nvfp4` | 21.9 GB | Coding-specialized quantized variant |
| `qwen2.5-coder:7b` | 4.7 GB | Minimal footprint (fallback parsing supported) |

> Use `/models` in the REPL to see all installed models with RAM recommendations.

---

## Usage

```bash
luna [flags] [prompt]
luna dream [--dry-run] [--model <model>]
luna memory <show|status|clear|add <text>>
luna config <show|init>
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--model` | LLM model | `qwen2.5-coder:7b` |
| `--provider` | `ollama` \| `openai` | `ollama` |
| `--base-url` | API endpoint | `http://localhost:11434` |
| `--api-key` | API key | `$OPENAI_API_KEY` |
| `--max-iter` | Max tool iterations | `20` |
| `--unsafe` | Skip bash / write confirmation | `false` |
| `--stream` | Enable streaming output | `false` |
| `--no-context` | Skip loading `.luna-context.md` | `false` |

### REPL Commands

| Command | Description |
|---------|-------------|
| `/models` | List Ollama models with RAM recommendations; select to switch |
| `/dream` | Summarize session buffer into `context.md` via Ollama |
| `/memory [show\|status\|clear]` | Inspect or reset session memory |
| `/skills` | List available Agent Skills |
| `/skill:<name>` | Load and execute a skill |
| `/unsafe` | Toggle auto-approve for bash / write mid-session |
| `/goal <text>` | Set a session goal — Luna works autonomously until achieved |
| `/goal` | Show current goal |
| `/goal clear` | Clear goal |
| `/help` | Show all commands |
| `[[` | Start multi-line input (end with `]]`) |
| `exit` | Quit REPL |

**Using a cloud API:**

```bash
luna --provider openai --model gpt-4o "add error handling to all HTTP handlers"
```

---

## Project Context

Place a `.luna-context.md` file in your project root and Luna injects it into the system prompt at startup — no need to re-explain your project every session.

```markdown
# .luna-context.md
Go 1.24 project. Packages: cmd/, internal/llm/, internal/tools/, internal/agent/
Tests: go test ./...   Build: go build -o luna .
Keep dependencies minimal.
```

---

## Dreaming-lite (Session Memory)

Luna automatically captures session conversations to a buffer. Run `luna dream` to let a local Ollama model distill the buffer into **three types of files**, all injected on next startup.

```
# After a session
$ luna dream
🌙 dreaming over 5 buffer entries with qwen3.5:9b ...
   📝 context.md updated
   ✨ experience.md updated
   🔧 skill "go-http-timeout" saved (project)
✅ dream complete (5 entries processed, buffer cleared)

# Next day
$ luna
🧠 loaded memory/context.md (luna-go)    ← project-specific knowledge
✨ loaded memory/experience.md (luna-go) ← implicit knowledge / secret sauce
🔧 loaded 1 skill(s): go-http-timeout   ← extracted skill auto-discovered
>
```

| File | Contents |
|------|----------|
| `context.md` | Project-specific decisions and rules |
| `experience.md` | Implicit knowledge, intuitions, "secret sauce" |
| `skills/<name>/SKILL.md` | Reusable procedures (`scope: project` or `global`) |

---

## Autonomous Mode (`/goal`)

Set a session-level goal and Luna keeps working until it declares the goal achieved — looping through tool calls without returning to the REPL.

```
> /unsafe
> /goal make all tests pass in internal/tools/

⚙  bash({"command":"go test ./internal/tools/..."})
--- FAIL: TestReadTool ...

⚙  read({"path":"internal/tools/read_test.go"})
⚙  edit({"path":"...", ...})
⚙  bash({"command":"go test ./internal/tools/..."})
ok  github.com/zephel01/luna-go/internal/tools

ゴール達成: All tests pass. Fixed boundary condition in TestReadTool...

>
```

A 30-minute wall-clock timeout applies by default (configurable via `loop_timeout_min`).

---

## Docker Sandbox

Add `--sandbox` to run all bash commands inside a Docker container instead of on your host. AI-generated code cannot reach your host filesystem or network by default — making it safe to experiment with unfamiliar or untrusted code.

```bash
luna --sandbox "install deps and run tests"
luna --sandbox --sandbox-image luna-sandbox:latest "write and run a Python script"
```

Luna prints the active security configuration at startup:

```
🐳 sandbox mode: ubuntu:22.04  network=none  memory=256m  cpus=0.5
```

### Security boundaries

| Restriction | Default | Effect |
|-------------|---------|--------|
| Network | `none` | Blocks all outbound traffic from the container |
| Memory | `256m` | Prevents the container from exhausting host RAM |
| CPU | `0.5` | Prevents CPU starvation on the host |
| File writes | `/workspace` mount only | Only your working directory is writable |

Your working directory is bind-mounted at `/workspace`. File changes are reflected on the host through this mount, but nothing else on your host filesystem is accessible.

> If Docker is not installed, Luna prints a warning and falls back to running commands on the host.

---

## Agent Skills

Luna auto-discovers `SKILL.md` files and makes them available as `/skill:<name>` commands. Skills created for Claude Code or Cowork work here too.

```
<project>/.agents/skills/<name>/SKILL.md          ← project-local (highest priority)
~/.luna-go/memory/<project>/skills/<name>/SKILL.md ← generated by luna dream (project scope)
~/.agents/skills/<name>/SKILL.md                   ← shared across agents
~/.claude/skills/<name>/SKILL.md                   ← Claude Code / Cowork compatible
~/.luna-go/skills/<name>/SKILL.md                  ← generated by luna dream (global scope)
```

Example skills are included in [`examples/skills/`](examples/skills/):

| Skill | Description |
|-------|-------------|
| `git-commit` | Generate a Conventional Commits message from staged diff and commit |
| `go-test-fix` | Run `go test` and fix failures until all tests pass |
| `go-lint` | Run `go vet` + `golangci-lint` and fix all findings |
| `code-review` | Review a file or git diff and output a structured report |
| `pr-description` | Generate a GitHub PR description from the current branch diff |

```bash
# Install all example skills
cp -r examples/skills/* ~/.agents/skills/
```

---

## Tools

| Tool | What it does |
|------|-------------|
| `read` | Read a file with line numbers (max 2000 lines) |
| `write` | Write a file (prompts for confirmation on overwrite) |
| `edit` | Apply a targeted `old_string → new_string` patch |
| `bash` | Run a shell command (30s timeout, 10KB output cap) |
| `grep` | Search files with a regex pattern |

---

## How It Works

Luna runs a [ReAct](https://arxiv.org/abs/2210.03629) loop.

```
User input
    │
    ▼
LLM (think)
    │
    ├── Tool call? → execute → add result to history → back to LLM
    │
    └── No tool call → print answer → done (or continue if /goal active)
```

Luna handles the quirks of local LLMs automatically:
- **`<think>` tag stripping** — removes reasoning leaks from Qwen3 / DeepSeek-R1 models
- **Fallback JSON parser** — handles models that emit tool calls as text instead of structured `tool_calls`
- **Auto `num_ctx`** — sets `num_ctx: 32768` for Ollama endpoints to prevent silent context truncation

---

## Roadmap

- [x] v0.1 — Core agent loop, 5 tools, Ollama + OpenAI support
- [x] v0.2 — `edit` tool, write confirmation, `.luna-context.md` inject, `/models` with RAM hints
- [x] v0.3 — Dreaming-lite memory (`luna dream` / `luna memory` / `/dream`)
- [x] v0.4 — liner REPL (tab completion, history, multi-line `[[`), Agent Skills, `/unsafe`, `/goal` autonomous loop
- [x] v0.4.5 — Wall-clock loop timeout (default 30 min, `loop_timeout_min` config)
- [x] v0.9.x — Dream 3-way distillation (context / experience / skills), project/global scope auto-write
- [ ] v1.0 — Prebuilt binaries via GitHub Actions, `ollama launch luna`, stable API

---

## Contributing

Issues and PRs welcome. Luna's scope is intentionally narrow — new features should clear a high bar. Open an issue first if you're unsure whether something fits.

---

<div align="center">

MIT License © 2025 zephel01

</div>

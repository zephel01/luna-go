<div align="center">

# 🌙 luna-go

**A minimal AI coding agent built for Ollama**

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Ollama](https://img.shields.io/badge/Ollama-first-blueviolet)](https://ollama.com)

English | [日本語](README.md)

</div>

---

```
$ luna "read main.go and add unit tests for the Parse function"

⚙  read({"path": "/project/main.go"})
⚙  read({"path": "/project/parse.go"})
⚙  write({"path": "/project/parse_test.go", "content": "..."})
⚙  bash({"command": "go test ./..."})

All tests pass.
```

---

## Why luna-go?

Most AI coding tools require a cloud subscription, an npm install, or a Python environment. Luna doesn't.

|  | **luna-go** | Claude Code | Aider | Codex CLI |
|--|:-----------:|:-----------:|:-----:|:---------:|
| Distribution | **single binary** | npm | pip | binary |
| Default backend | **Ollama (local)** | Anthropic API | Any | OpenAI |
| Offline capable | ✅ | ❌ | ✅ | ❌ |
| Tools | **4** | 30+ | git + edit | 10+ |

> A smart enough model doesn't need 30 tools. Four is enough for 80% of real coding tasks.

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
> Always use `make build` or `go build -o luna .` instead.

Requires Go 1.22+. No other dependencies.

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
| `qwen3.5:9b` | 6.6 GB | **Recommended** — reliable tool calling, great balance |
| `qwen3:32b` | 20 GB | Best quality for complex tasks |
| `qwen2.5-coder:14b` | 9.0 GB | Strong for code-specific work |
| `qwen2.5-coder:7b` | 4.7 GB | Minimum viable, works with fallback parsing |

---

## Usage

```bash
luna [flags] [prompt]

# Omitting prompt launches REPL mode
```

| Flag | Description | Default |
|------|-------------|---------|
| `--model` | LLM model | `qwen3.5:9b` |
| `--provider` | `ollama` \| `openai` | `ollama` |
| `--base-url` | API endpoint | `http://localhost:11434` |
| `--api-key` | API key | `$OPENAI_API_KEY` |
| `--max-iter` | Max tool iterations | `20` |
| `--unsafe` | Skip bash confirmation | `false` |
| `--stream` | Enable streaming output | `false` |

**Using a cloud API:**

```bash
luna --provider openai --model gpt-4o "add error handling to all HTTP handlers"
```

---

## What Luna Can (and Can't) Do

**✅ Works well**
- Refactor functions, rename variables across files
- Fix bugs from error messages
- Add tests to existing modules
- Generate README or inline docs from code
- Edit config files (package.json, Dockerfile, tsconfig)
- Parse logs and summarize findings
- Implement new endpoints or utility functions

**⚠️ Works, but takes more iterations**
- Large-scale refactors spanning many files
- Tasks requiring deep framework-specific knowledge

**❌ Out of scope (by design)**
- Interactive debugging (breakpoints, step execution)
- Long-term memory across sessions
- IDE integration
- Parallel task execution

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
    └── No tool call → print answer → done
```

| Tool | What it does |
|------|-------------|
| `read` | Read a file with line numbers (max 2000 lines) |
| `write` | Write or overwrite a file |
| `bash` | Run a shell command (30s timeout, 10KB output cap) |
| `grep` | Search files with a regex pattern |

---

## Roadmap

- [x] v0.1 — Core agent loop, 4 tools, Ollama + OpenAI support
- [ ] v0.2 — Diff preview before write, context window management
- [ ] v0.3 — `brew install`, prebuilt binaries via GitHub Actions
- [ ] v1.0 — Stable API, plugin system

---

## Contributing

Issues and PRs welcome. Luna's scope is intentionally narrow — new tools and features should clear a high bar. If you're unsure whether something fits, open an issue first.

---

<div align="center">

MIT License © 2025 zephel01

</div>

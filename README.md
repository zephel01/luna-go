<div align="center">

# 🌙 luna-go

**Ollama ファーストのミニマルな AI コーディングエージェント**

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Ollama](https://img.shields.io/badge/Ollama-first-blueviolet)](https://ollama.com)

[English](README.en.md) | 日本語

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

## なぜ luna-go か

ほとんどの AI コーディングツールは、クラウド契約・npm・Python 環境のどれかを必要とします。Luna は違います。

|  | **luna-go** | Claude Code | Aider | Pi |
|--|:-----------:|:-----------:|:-----:|:--:|
| 配布形式 | **単一バイナリ** | npm | pip | TypeScript |
| デフォルト | **Ollama (ローカル)** | Anthropic API | Any | Ollama |
| オフライン | ✅ | ❌ | ✅ | ✅ |
| ツール数 | **5** | 30+ | git + edit | 4 |
| セッション記憶 | ✅ Dreaming-lite | ✅ | ❌ | ❌ |
| REPL readline | ✅ liner | ✅ | ✅ | ❌ |
| 自律ループ | ✅ `/goal` | ✅ | ❌ | ❌ |
| Agent Skills | ✅ | ✅ | ❌ | ❌ |

> モデルが賢ければ、ツールは 5 つで十分。

---

## インストール

### バイナリをダウンロード

```bash
# macOS (Apple Silicon)
curl -L https://github.com/zephel01/luna-go/releases/latest/download/luna-darwin-arm64 \
  -o luna && chmod +x luna && sudo mv luna /usr/local/bin/
```

<details>
<summary>その他のプラットフォーム</summary>

```bash
# macOS (Intel)
curl -L https://github.com/zephel01/luna-go/releases/latest/download/luna-darwin-amd64 \
  -o luna && chmod +x luna && sudo mv luna /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/zephel01/luna-go/releases/latest/download/luna-linux-amd64 \
  -o luna && chmod +x luna && sudo mv luna /usr/local/bin/
```

</details>

### ソースからビルド

```bash
git clone https://github.com/zephel01/luna-go
cd luna-go && make build   # → ./luna
```

> **Note:** `go build`（`-o` なし）はモジュール名から `luna-go` バイナリを生成します。
> 必ず `make build` または `go build -o luna .` を使用してください。

Go 1.22+ のみ。外部依存 2 本（`gopkg.in/yaml.v3` + `github.com/peterh/liner`）。

---

## クイックスタート

**1. Ollama をインストールしてモデルを取得**

```bash
ollama pull qwen3.5:9b
```

**2. 設定ファイルを作成（オプション）**

```yaml
# ~/.luna-go/config.yaml
model: qwen3.5:9b
provider: ollama
base_url: http://localhost:11434
```

**3. 実行**

```bash
# ワンショット
luna "grep all TODO comments and summarize them"

# インタラクティブ REPL
luna
```

---

## 推奨モデル

| モデル | サイズ | 特徴 |
|--------|--------|------|
| `qwen3.5:35b-a3b` | 22 GB | **精度重視** — 大容量 RAM 環境向け |
| `qwen3.5:9b` | 5.8 GB | **バランス推奨** — 安定した tool calling |
| `qwen3.6:35b-a3b-coding-nvfp4` | 21.9 GB | コーディング特化量子化版 |
| `qwen2.5-coder:7b` | 4.7 GB | 最小構成（フォールバック対応） |

> `/models` コマンドで搭載 RAM に基づいた推奨が自動表示されます。

---

## 使い方

```bash
luna [flags] [prompt]
luna dream [--dry-run] [--model <model>]
luna memory <show|status|clear|add <text>>
luna config <show|init>
```

### フラグ

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--model` | 使用モデル | `qwen2.5-coder:7b` |
| `--provider` | `ollama` \| `openai` | `ollama` |
| `--base-url` | API エンドポイント | `http://localhost:11434` |
| `--api-key` | API キー | `$OPENAI_API_KEY` |
| `--max-iter` | 最大ツール呼び出し回数 | `20` |
| `--unsafe` | bash / write の確認をスキップ | `false` |
| `--stream` | ストリーミング出力 | `false` |
| `--no-context` | `.luna-context.md` の読み込みをスキップ | `false` |

### REPL コマンド

| コマンド | 説明 |
|----------|------|
| `/models` | Ollama モデル一覧と RAM 推奨を表示、切り替え |
| `/dream` | セッションバッファを Ollama で整理して context.md に保存 |
| `/memory [show\|status\|clear]` | 記憶の確認・管理 |
| `/skills` | 利用可能なスキルの一覧を表示 |
| `/skill:<name>` | スキルをロードして実行 |
| `/unsafe` | bash / write の確認をセッション中だけ ON/OFF |
| `/goal <text>` | ゴールを設定して達成まで自律実行 |
| `/help` | コマンド一覧 |
| `[[` | 多行入力モード（`]]` で確定） |
| `exit` | 終了 |

**クラウド API を使う場合：**

```bash
luna --provider openai --model gpt-4o "add error handling to all HTTP handlers"
```

---

## プロジェクトコンテキスト

Luna はプロジェクトディレクトリに `.luna-context.md` があれば起動時に自動で読み込みます。

```markdown
# .luna-context.md
このプロジェクトは Go 1.22、設計方針はミニマル・依存ゼロ
テストは go test ./... で実行
```

起動時に内容がシステムプロンプトへ注入されるため、毎回説明し直す必要がありません。

---

## Dreaming-lite（セッション記憶）

Claude Dreaming にインスパイアされた、ローカル Ollama で動く記憶機能です。

```
# 1日の作業後
$ luna dream
🌙 dreaming over 5 buffer entries with qwen3.5:9b ...
✅ context.md updated (5 entries processed, buffer cleared)

# 翌日の起動時
$ luna
📎 loaded .luna-context.md
🧠 loaded memory/context.md (luna-go)   ← 昨日の内容を把握した状態でスタート
>
```

セッション中の会話は `~/.luna-go/memory/<project>/buffer.jsonl` に自動保存されます。`luna dream` を実行すると Ollama が内容を要約し、`context.md` に蒸留します。

```
~/.luna-go/memory/<project>/
  context.md     ← 蒸留された記憶（自動 inject）
  buffer.jsonl   ← セッションバッファ（dream で消化）
```

---

## ツール一覧

| ツール | 機能 |
|--------|------|
| `read` | ファイル読み込み（行番号付き、最大 2000 行） |
| `write` | ファイル書き込み（上書き時は確認あり） |
| `edit` | `old_string → new_string` の差分適用（部分編集向け） |
| `bash` | シェルコマンド実行（30 秒タイムアウト、10 KB 上限） |
| `grep` | 正規表現でファイル検索 |

---

## 仕組み

Luna は [ReAct](https://arxiv.org/abs/2210.03629) ループで動作します。

```
ユーザー入力
     │
     ▼
  LLM（思考）
     │
     ├── ツール呼び出し → 実行 → 結果を履歴に追加 → LLM に戻る
     │
     └── ツール呼び出しなし → 回答を出力 → 終了
```

---

## できること / できないこと

**✅ 得意なこと**
- 関数のリファクタ・変数名変更
- エラーメッセージを渡してバグ修正
- 既存モジュールへのテスト追加
- コードから README・コメント生成
- 設定ファイル編集（package.json / Dockerfile / tsconfig）
- ログ解析と要約
- 新しい API エンドポイントの実装

**⚠️ やや苦手（モデル次第）**
- 10 ファイル以上にまたがる大規模リファクタ
- フレームワーク固有の深い知識が必要なタスク

**❌ 対象外（設計上）**
- ブレークポイント・ステップ実行
- IDE 統合
- 並列タスク実行
- ベクトルメモリ・RAG

---

## ロードマップ

- [x] v0.1 — コアエージェントループ、4 ツール、Ollama + OpenAI 対応
- [x] v0.2 — `edit` ツール、write 前確認、`.luna-context.md` inject、セッションログ、`/models` + RAM 推奨
- [x] v0.3 — Dreaming-lite（`luna dream` / `luna memory` / `/dream`）
- [x] v0.4 — liner REPL（タブ補完・↑↓ヒストリー・多行入力）、Agent Skills、`/unsafe` トグル、`/goal` 自律ループ
- [x] v0.4.5 — ウォールクロックループタイムアウト（デフォルト 30 分、`loop_timeout_min` 設定）
- [x] v0.5 — 自動コンテキスト圧縮（`compress_threshold` + `compress_model`）
- [ ] v1.0 — バイナリ自動配布、`ollama launch luna`、安定 API

---

## コントリビュート

Issue・PR 歓迎です。Luna のスコープは意図的に狭く保っています。新機能の追加には高いハードルを設けているので、不明な場合は先に Issue を立ててください。

---

<div align="center">

MIT License © 2025 zephel01

</div>

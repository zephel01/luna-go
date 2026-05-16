# Luna ユーザーガイド

> Ollama ファーストのミニマル AI コーディングエージェント

---

## 目次

1. [Ollama のインストール](#1-ollama-のインストール)
2. [luna-go のインストール](#2-luna-go-のインストール)
3. [初期設定](#3-初期設定)
4. [基本的な使い方](#4-基本的な使い方)
5. [REPL インタラクティブモード](#5-repl-インタラクティブモード)
6. [プロジェクトコンテキスト](#6-プロジェクトコンテキスト)
7. [記憶機能（Dreaming-lite）](#7-記憶機能dreaming-lite)
8. [スキル（Agent Skills）](#8-スキルagent-skills)
9. [自律実行モード（/goal）](#9-自律実行モードgoal)
10. [クラウド API を使う](#10-クラウド-api-を使う)
11. [CodeRouter と組み合わせる](#11-coderouter-と組み合わせる)
12. [トラブルシューティング](#12-トラブルシューティング)

---

## 1. Ollama のインストール

Luna はデフォルトで [Ollama](https://ollama.com) を使ってローカル LLM を実行します。

### macOS

```bash
brew install ollama
```

または [ollama.com/download](https://ollama.com/download) からインストーラをダウンロード。

### Linux

```bash
curl -fsSL https://ollama.com/install.sh | sh
```

### Ollama を起動する

macOS のインストーラ版はメニューバーから自動起動します。手動で起動するには:

```bash
ollama serve
```

起動確認:

```bash
curl http://localhost:11434/api/tags
# {"models":[...]} が返れば OK
```

### モデルを取得する

Luna に使用するモデルを pull します。RAM に合わせて選んでください。

| モデル | サイズ | 推奨 RAM | 特徴 |
|--------|--------|----------|------|
| `qwen2.5-coder:7b` | 4.7 GB | 8 GB+ | 軽量・安定。tool calling フォールバック対応 |
| `qwen3.5:9b` | 5.8 GB | 16 GB+ | バランス重視のおすすめ |
| `qwen3.5:35b-a3b` | 22 GB | 32 GB+ | 精度重視 |

```bash
ollama pull qwen2.5-coder:7b
```

> **ヒント:** Luna の REPL 内で `/models` と入力すると、搭載 RAM に基づいた推奨タグ付きでモデル一覧が表示され、その場で切り替えることもできます。

---

## 2. luna-go のインストール

### バイナリをダウンロード（推奨）

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

### ソースからビルド

Go 1.22 以上が必要です。

```bash
git clone https://github.com/zephel01/luna-go
cd luna-go
go build -o luna .
sudo mv luna /usr/local/bin/
```

### インストール確認

```bash
luna --version
# luna-go v0.4.4
```

---

## 3. 初期設定

### 設定ファイル（オプション）

`~/.luna-go/config.yaml` を作成することで、毎回フラグを指定する手間を省けます。

```yaml
# ~/.luna-go/config.yaml
model: qwen2.5-coder:7b
provider: ollama
base_url: http://localhost:11434

# オプション設定
max_iter: 20      # ツール呼び出しの最大回数
unsafe: false     # true にすると bash/write の確認をスキップ
stream: false     # true にするとストリーミング出力を有効化
num_ctx: 0        # コンテキスト長（0 = モデルデフォルト）
```

`luna config init` で雛形を自動生成することもできます:

```bash
luna config init
```

### 環境変数

設定ファイルよりもフラグ、フラグよりも環境変数が優先されます。

| 変数 | 説明 |
|------|------|
| `LUNA_MODEL` | 使用モデル名 |
| `LUNA_PROVIDER` | `ollama` または `openai` |
| `LUNA_BASE_URL` | API エンドポイント URL |
| `OPENAI_API_KEY` | OpenAI 互換 API のキー |

---

## 4. 基本的な使い方

### ワンショットモード

引数にプロンプトを渡すと 1 回実行して終了します。スクリプトや CI からの利用に向いています。

```bash
# ファイルの内容を読んで説明させる
luna "read main.go and explain what Parse does"

# バグ修正
luna "read internal/llm/client.go and fix the error handling in the Stream method"

# テスト生成
luna "read internal/tools/bash.go and write unit tests for Execute"

# リファクタ
luna "read cmd/root.go and extract the model selection logic into a separate function"
```

### フラグ一覧

```bash
luna [flags] [prompt]
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--model` | 使用モデル | `qwen2.5-coder:7b` |
| `--provider` | `ollama` \| `openai` | `ollama` |
| `--base-url` | API エンドポイント | `http://localhost:11434` |
| `--api-key` | API キー | `$OPENAI_API_KEY` |
| `--max-iter` | 最大ツール呼び出し回数 | `20` |
| `--unsafe` | bash / write の確認をスキップ | `false` |
| `--stream` | ストリーミング出力（実験的） | `false` |
| `--no-context` | `.luna-context.md` の読み込みをスキップ | `false` |
| `-v` / `--version` | バージョンを表示して終了 | — |

### サブコマンド

```bash
luna dream               # セッション記憶を整理・蒸留
luna dream --dry-run     # 整理結果を表示するだけ（保存しない）

luna memory show         # 現在の記憶を表示
luna memory status       # バッファエントリ数・サイズを表示
luna memory clear        # バッファと context.md を削除

luna config show         # 現在の設定を表示
luna config init         # ~/.luna-go/config.yaml の雛形を生成
```

---

## 5. REPL インタラクティブモード

引数なしで起動すると REPL（対話モード）に入ります。

```bash
luna
# 🔧 loaded 2 skill(s): git-commit, code-review
# 📎 loaded .luna-context.md
# Luna v0.4  —  type your request, Tab to complete, Ctrl-C or 'exit' to quit
#
# >
```

### タブ補完

`/` を入力して Tab を押すと、利用可能なコマンドが補完されます。

```
> /mo[TAB]    →    /models
> /sk[TAB]    →    /skills
> /skill:[TAB]  →  /skill:git-commit  /skill:code-review
```

### 矢印キーヒストリー

↑ / ↓ キーで同一セッション内の入力履歴を呼び出せます。

### REPL コマンド

| コマンド | 説明 |
|----------|------|
| `/models` | Ollama モデル一覧と RAM 推奨を表示。番号を入力すると即切り替え |
| `/dream` | セッションバッファを Ollama で整理して `context.md` に保存 |
| `/memory [show\|status\|clear]` | 記憶を管理（表示 / 状態確認 / 削除） |
| `/skills` | 利用可能なスキルの一覧を表示 |
| `/skill:<name>` | 指定したスキルをロードして実行 |
| `/unsafe` | bash / write の確認をセッション中だけ ON/OFF 切り替え |
| `/goal <text>` | セッションゴールを設定（全 LLM ターンに注入・達成まで自律実行） |
| `/goal` | 現在のゴールを確認 |
| `/goal clear` | ゴールを削除 |
| `/help` | コマンド一覧を表示 |
| `[[` | 多行入力モード開始（`]]` で確定） |
| `exit` / `quit` | REPL を終了 |

### 多行入力（`[[` / `]]`）

コードのコピペや複数行プロンプトを入力するには `[[` で多行モードを開始し、`]]` で確定します。

```
> [[
... def fib(n):
...     if n < 2: return n
...     return fib(n-1) + fib(n-2)
... ]]
# 上記の関数をレビューして Go に移植してください
```

> **注意:** `[[` を使わずに改行を含む文字列を直接ペーストすると liner が途中で切断する場合があります。

---

### `/unsafe` — 確認スキップのトグル

bash / write ツールは安全のため実行前に確認を求めます。セッション中だけまとめて承認したい場合は `/unsafe` で切り替えます。

```
> /unsafe
⚠  unsafe mode ON — 以降の bash/write は自動承認

> /unsafe
✓  unsafe mode OFF — 確認を再有効化
```

確認プロンプト（`[y/a/N]`）で `a` を入力すると、その場で unsafe mode に切り替えて以降を全て承認することもできます。

```
⚠  bash: go test ./...
Run? [y/a/N] a
⚠  unsafe mode ON — 以降の bash/write は自動承認
```

---

### `/models` の使い方

```
> /models

Available Ollama models:  (RAM: 16.0 GB)
  1) qwen2.5-coder:7b             4.7 GB   ✅ 推奨
  2) qwen3.5:9b                   5.8 GB   ✅ 推奨
  3) qwen3.5:35b-a3b              22 GB    ❌ RAM 不足

Select [1-3] (Enter to cancel): 2
✔ switched to qwen3.5:9b
```

---

## 6. プロジェクトコンテキスト

プロジェクトのルートに `.luna-context.md` を置くと、Luna は起動時に自動で読み込み、システムプロンプトに注入します。毎回プロジェクトの説明をしなくて済みます。

```markdown
# .luna-context.md

## プロジェクト概要
Go 1.24 製の CLI ツール。パッケージ構成: cmd/, internal/llm/, internal/tools/, internal/agent/

## 開発ルール
- テストは `go test ./...`
- フォーマットは `gofmt -w .`
- 新しいファイルを作るときは必ず既存のパターンに合わせる
- 外部依存は最小限に保つ

## よく使うコマンド
- ビルド: `go build -o luna .`
- CI ローカル実行: `go vet ./... && go test ./...`
```

**読み込み優先順位:**

1. `cwd/.luna-context.md`（プロジェクト固有）
2. `~/.luna-go/memory/<project>/context.md`（`luna dream` が生成）
3. `~/.luna-go/context.md`（グローバルフォールバック）

1 と 2 は両方存在する場合に結合されます。

---

## 7. 記憶機能（Dreaming-lite）

Luna はセッション中の会話を自動的にバッファに保存します。`luna dream` を実行すると、ローカル Ollama がバッファを要約して `context.md` に蒸留します。翌日の起動時にこの記憶が自動で注入されます。

### ファイル構成

```
~/.luna-go/
  config.yaml
  memory/
    <project-name>/
      context.md      ← 蒸留済みの記憶（起動時に自動 inject）
      buffer.jsonl    ← セッションバッファ（dream で消化される）
  sessions/
    2026-05-17_10-30-00.jsonl   ← セッションログ（全会話の記録）
```

### 典型的なワークフロー

```bash
# 作業中
luna
> refactor the Stream method to use context cancellation
> add a timeout to the HTTP client in llm/client.go
> exit

# 作業終了後
luna dream
# 🌙 dreaming over 3 buffer entries with qwen2.5-coder:7b ...
# ✅ context.md updated (3 entries processed, buffer cleared)

# 翌日
luna
# 🧠 loaded memory/context.md (luna-go)   ← 昨日の作業内容を把握した状態でスタート
```

### 記憶の確認・管理

```bash
luna memory show    # context.md の内容を表示
luna memory status  # バッファのエントリ数とサイズを表示
luna memory clear   # バッファと context.md を削除してリセット
```

---

## 8. スキル（Agent Skills）

Luna は Agent Skills standard に対応しており、`SKILL.md` ファイルで定義されたスキルを自動検出します。Claude Code / Cowork で使っているスキルがそのまま動作します。

### スキルファイルの場所

検出される順序（プロジェクトローカル優先）:

```
<project>/.agents/skills/<name>/SKILL.md
~/.agents/skills/<name>/SKILL.md
~/.claude/skills/<name>/SKILL.md     ← Claude Code / Cowork と共有可能
~/.pi/agent/skills/<name>/SKILL.md
~/.luna-go/skills/<name>/SKILL.md
```

### SKILL.md の形式

```markdown
---
name: git-commit
description: ステージ済みの変更を確認してコミットメッセージを生成する
---

## 手順

1. `bash({"command": "git diff --cached"})` でステージ済みの差分を確認する
2. 変更内容に基づいて Conventional Commits 形式のメッセージを提案する
3. ユーザーが確認したら `bash({"command": "git commit -m '...'"})` を実行する
```

### スキルの使い方

```bash
# REPL 内で
> /skills              # 利用可能なスキルの一覧
> /skill:git-commit    # スキルをロードして実行

# または TAB 補完で
> /skill:[TAB]
```

---

## 9. 自律実行モード（/goal）

`/goal` でセッションレベルのゴールを設定すると、Luna はゴールを達成するまで自律的にツール呼び出しと応答を繰り返します。

### 使い方

```
> /goal テストが全て通るように internal/tools/ のユニットテストを書く

🎯 goal set: テストが全て通るように internal/tools/ のユニットテストを書く
```

ゴールが設定されると、Luna は以下を繰り返します:

1. ツール呼び出し（read / bash / write / edit）
2. 結果の観察
3. 次のステップを判断
4. 「ゴール達成: ...」と宣言するまで継続

```
> /goal ゴールが達成されるまで go test を実行してエラーを全て修正する

⚙  bash({"command":"go test ./..."})
... (失敗ログ)
⚙  read({"path":"/path/to/failing_test.go"})
...
⚙  edit({"path":"...", "old_string":"...", "new_string":"..."})
...
⚙  bash({"command":"go test ./..."})
... (全テスト PASS)

ゴール達成: 全テストが通るようになりました。修正内容: ...
```

### ゴールの管理

```
> /goal                       # 現在のゴールを確認
🎯 goal: テストが全て通るように...

> /goal clear                  # ゴールを削除（通常の対話モードに戻る）
goal cleared
```

### 注意事項

- ゴール達成には**`/unsafe` か `--unsafe` フラグ**を推奨します（確認プロンプトでループが止まるため）
- 最大ツール呼び出し回数（デフォルト 20）を超えるとタイムアウトします。複雑なタスクは `--max-iter 50` などで調整してください
- ゴールを達成したらモデルは「ゴール達成:」で始まるメッセージを返し、REPL に戻ります

---

## 10. クラウド API を使う

Luna は OpenAI 互換 API であればどれでも使えます。

### OpenAI

```bash
export OPENAI_API_KEY=sk-...
luna --provider openai --model gpt-4o "refactor this function"
```

### ローカルの OpenAI 互換サーバー（LM Studio など）

```bash
luna --provider openai --base-url http://localhost:1234/v1 --model local-model "..."
```

### config.yaml で固定する

```yaml
provider: openai
model: gpt-4o-mini
base_url: https://api.openai.com/v1
```

---

## 11. CodeRouter と組み合わせる

[CodeRouter](https://github.com/zephel01/CodeRouter) は、ローカル LLM とクラウド API の間に置くルーター層です。luna-go は OpenAI 互換 API をそのまま使っているため、`--base-url` を変えるだけで接続できます。

```
luna-go
  │
  ▼
CodeRouter (localhost:8088)
  │
  ├─→ Ollama (ローカル / 無料・最速)
  ├─→ OpenRouter 無料枠 (クラウド / 無料)
  └─→ OpenAI / Anthropic (有料・オプトイン時のみ)
```

### どんなときに使うか

| 状況 | CodeRouter は？ |
|------|----------------|
| Ollama のレスポンスがたまにおかしい（tool call が壊れる） | **有効** — 壊れた JSON を修復してから luna に渡す |
| Ollama が落ちたとき自動でクラウドに切り替えたい | **有効** — 3 層フォールバックが自動で動く |
| 長時間（数時間）回すとモデルが劣化・止まる | **有効** — 6 系統ガードで無人運転を安定化 |
| 複数のモデルを使い分けたい | **有効** — プロファイルで切り替え |
| Ollama 直結で問題なく動いている | 不要 |

### セットアップ

**1. CodeRouter をインストール・起動**

```bash
# インストール（Python 3.12+ 必須）
uvx --from coderouter-cli coderouter serve --port 8088

# 設定ファイルを配置（初回のみ）
mkdir -p ~/.coderouter
curl -fsSL https://raw.githubusercontent.com/zephel01/CodeRouter/main/examples/providers.yaml \
  > ~/.coderouter/providers.yaml
```

**2. luna-go を CodeRouter に向ける**

```bash
# ワンショット
luna --base-url http://localhost:8088/v1 --model qwen2.5-coder:7b "fix the bug"

# または config.yaml で固定
```

```yaml
# ~/.luna-go/config.yaml
base_url: http://localhost:8088/v1
model: qwen2.5-coder:7b   # CodeRouter 側の providers.yaml で定義したモデルと合わせる
```

以降は `luna` をそのまま使うだけ。CodeRouter が透過的にルーティングします。

### CodeRouter 側の設定例

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

  - name: openrouter-free          # Ollama が落ちたときのフォールバック
    kind: openai_compat
    base_url: https://openrouter.ai/api/v1
    model: qwen/qwen2.5-coder:7b:free
    api_key_env: OPENROUTER_API_KEY
```

### luna-go 単体 vs CodeRouter 経由の違い

| 機能 | luna-go 単体 | luna-go + CodeRouter |
|------|-------------|----------------------|
| Ollama 直接接続 | ✅ | ✅（CodeRouter 経由） |
| tool call の修復 | ✅ フォールバックパーサ内蔵 | ✅ + CodeRouter 側でも修復 |
| 自動フォールバック | ❌ | ✅ ローカル → 無料クラウド → 有料 |
| 長時間運用ガード | ❌ | ✅ Context Budget / Drift Detection など 6 系統 |
| `/dashboard` でリアルタイム監視 | ❌ | ✅ |
| セットアップの手間 | なし | CodeRouter の起動が別途必要 |

### 動作確認

```bash
# CodeRouter の診断コマンド
coderouter doctor

# luna から疎通確認
luna --base-url http://localhost:8088/v1 --model qwen2.5-coder:7b "hello"
```

> 詳細は [CodeRouter ドキュメント](https://github.com/zephel01/CodeRouter) を参照してください。

---

## 12. トラブルシューティング

### Ollama に接続できない

```
error: llm: Post "http://localhost:11434/v1/chat/completions": dial tcp ...
```

**確認事項:**

```bash
# Ollama が起動しているか確認
curl http://localhost:11434/api/tags

# 起動していなければ
ollama serve
```

macOS の場合はメニューバーの Ollama アイコンを確認してください。

---

### モデルが見つからない

```
error: model "qwen3.5:9b" not found
```

**対処:**

```bash
ollama pull qwen3.5:9b
```

または REPL 内で `/models` を実行してインストール済みモデルを確認してください。

---

### ツール呼び出しが止まらない / ループする

`max_iter` の上限（デフォルト 20）に達した場合のエラー:

```
error: max iterations (20) reached without a final answer
```

**原因と対処:**

- モデルが同じツールを繰り返し呼んでいる場合は、より賢いモデルに切り替えてください（`/models`）
- タスクが複雑すぎる場合は、小さいステップに分割してください
- `--max-iter 30` で上限を引き上げることも可能です

---

### bash / write の確認プロンプトが毎回出る

```
⚠  bash: go test ./...
Run? [y/a/N]
```

これは安全のためのデフォルト動作です。以下の方法で抑制できます:

- **その場で全承認**: プロンプトに `a` を入力 → セッション中は以降全て自動承認
- **REPL 内でトグル**: `/unsafe` を実行 → ON/OFF が切り替わる
- **起動時から有効**: `--unsafe` フラグを使う
- **設定ファイルで固定**: `config.yaml` に `unsafe: true`

```bash
luna --unsafe "run go test ./... and fix failing tests"
```

---

### コンテキストが短い / 途中で切れる

長いファイルや会話で情報が欠落する場合は、`num_ctx` を増やします:

```yaml
# ~/.luna-go/config.yaml
num_ctx: 32768
```

ただし増やすほど RAM 消費と応答時間が増加します。Ollama のモデルページで推奨値を確認してください。

---

### ストリーミングで文字化け・表示がおかしい

`--stream` フラグは実験的機能です。問題が発生した場合はフラグなしで実行してください（デフォルトは非ストリーミング）。

---

### `.luna-context.md` が読み込まれない

```bash
# 読み込みをスキップするフラグが付いていないか確認
luna --no-context  # このフラグがあると読み込まれない

# ファイルが正しい場所にあるか確認
ls .luna-context.md

# 起動ログに "loaded .luna-context.md" が出るか確認
luna  # 標準エラー出力に表示される
```

---

### govulncheck の警告が CI で出る

```
llm.OpenAIClient.post calls http.Client.Do, which eventually calls tls.Conn.Write
```

これは luna-go のコード自体のバグではなく、使用している Go の標準ライブラリ（`crypto/tls`, `net/http`）に報告された CVE です。Go バージョンを上げることで解消します。CI が緑であればリリースに問題はありません。

---

## 付録: ファイル・ディレクトリ構成

```
~/.luna-go/
  config.yaml               # グローバル設定
  context.md                # グローバルフォールバックコンテキスト
  memory/
    <project>/
      context.md            # プロジェクト別・luna dream が生成
      buffer.jsonl          # セッションバッファ
  sessions/
    YYYY-MM-DD_HH-MM-SS.jsonl  # セッションログ（全会話記録）
  skills/                   # luna-go 固有スキルの置き場

~/.claude/skills/           # Claude Code / Cowork と共有できるスキル
~/.agents/skills/           # Agent Skills standard 準拠の場所

<project>/
  .luna-context.md          # プロジェクト固有のコンテキスト（最優先）
  .agents/skills/           # プロジェクトローカルスキル
```

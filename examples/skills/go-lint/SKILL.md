---
name: go-lint
description: go vet と golangci-lint を実行し、指摘された問題を修正する
---

## 手順

1. まず `go vet` を実行する。
   ```
   bash({"command": "go vet ./... 2>&1"})
   ```

2. `golangci-lint` が使えるか確認する。
   ```
   bash({"command": "which golangci-lint 2>/dev/null || echo 'not found'"})
   ```
   - 見つからなければ `go vet` の結果のみで対応する。

3. `golangci-lint` がある場合は実行する。
   ```
   bash({"command": "golangci-lint run ./... 2>&1"})
   ```

4. 指摘された問題を分類する。
   - **自動修正可能**: `gofmt`, `goimports`, `whitespace` 系
   - **要確認**: `errcheck`（エラー無視）, `gocritic`, `revive` などの提案
   - **スキップ推奨**: `wsl`（空白強制）など好みの問題は `.golangci.yml` で除外

5. 各問題を `read` で該当ファイルを確認してから `edit` で修正する。

6. 修正後に `go vet ./...` と `go build ./...` を再実行して壊れていないか確認する。

7. 修正内容のサマリーを報告する。

## よくある指摘と対応

| 指摘 | 原因 | 対応 |
|------|------|------|
| `errcheck` | エラーを `_` で無視している | エラーハンドリングを追加、または `//nolint:errcheck` でコメント |
| `unused` | 未使用の変数・関数 | 削除する |
| `shadow` | 外側の変数を内側でシャドウ | 変数名を変える |
| `nilnil` | エラーなしで nil を返している | 専用のエラー型を作る |
| `gofmt` | フォーマットが乱れている | `bash({"command": "gofmt -w ."})` |

## 注意

- `//nolint:<linter>` のコメントは本当に必要な場合のみ使う。理由をコメントに書く。
- 既存の `//nolint` を無断で削除しない。追加した人の意図がある可能性がある。

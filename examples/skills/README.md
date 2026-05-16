# luna-go スキル サンプル集

luna-go の [Agent Skills](../../docs/userguide.md#8-スキルagent-skills) として使えるサンプルです。
[Agent Skills standard](https://github.com/zephel01/luna-go/blob/main/docs/userguide.md) に準拠しており、Claude Code / Cowork とも互換があります。

## インストール方法

使いたいスキルのディレクトリごと `~/.agents/skills/` または `~/.claude/skills/` にコピーします。

```bash
# 全部インストール
cp -r examples/skills/* ~/.agents/skills/

# 個別にインストール
cp -r examples/skills/git-commit ~/.agents/skills/
```

luna を起動すると自動で認識されます。

```
🔧 loaded 5 skill(s): git-commit, go-test-fix, go-lint, code-review, pr-description
```

## スキル一覧

| スキル | 説明 | 主な用途 |
|--------|------|---------|
| [`git-commit`](./git-commit/SKILL.md) | ステージ済みの変更から Conventional Commits メッセージを生成してコミット | 毎回のコミット |
| [`go-test-fix`](./go-test-fix/SKILL.md) | `go test ./...` を実行し、失敗したテストを修正。全通過まで繰り返す | テスト修正ループ |
| [`go-lint`](./go-lint/SKILL.md) | `go vet` + `golangci-lint` を実行して lint 指摘を修正 | コード品質 |
| [`code-review`](./code-review/SKILL.md) | ファイルまたは git diff をレビューし、問題・提案・良い点をレポート | PR レビュー補助 |
| [`pr-description`](./pr-description/SKILL.md) | 現在のブランチの変更から GitHub PR 本文を生成 | PR 作成 |

## 使い方

```
# REPL で
> /skills                    # 一覧表示
> /skill:git-commit          # 実行
> /skill:go-test-fix         # 実行

# /goal と組み合わせる（推奨）
> /unsafe
> /goal go test ./... が全て通るように修正する
```

## 自分のスキルを作る

```markdown
---
name: your-skill-name
description: スキルの説明（/skills で表示される）
---

## 手順

1. ...
2. ...
```

`~/.agents/skills/your-skill-name/SKILL.md` に置くと次回起動時から使えます。

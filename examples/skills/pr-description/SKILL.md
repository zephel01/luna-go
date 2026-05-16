---
name: pr-description
description: 現在のブランチの変更から GitHub PR の本文（日本語）を生成する
---

## 手順

1. ブランチ名とベースブランチを確認する。
   ```
   bash({"command": "git branch --show-current"})
   bash({"command": "git log main...HEAD --oneline"})
   ```

2. 変更の概要を取得する。
   ```
   bash({"command": "git diff main...HEAD --stat"})
   bash({"command": "git diff main...HEAD"})
   ```

3. 差分が大きい場合（300 行以上）は重要なファイルを `read` で読んで内容を把握する。

4. 以下のテンプレートで PR 本文を生成する。

---

## 出力テンプレート

```markdown
## 変更内容

<変更の目的と背景を 2〜3 文で説明>

## 主な変更点

- <変更点 1>
- <変更点 2>
- <変更点 3>

## 技術的な詳細

<実装上の判断・設計の意図・注意点があれば記載。なければ省略>

## テスト

- [ ] `go test ./...` が通ることを確認
- [ ] <追加した場合: テストケース名>
- [ ] 手動での動作確認: <確認した内容>

## スクリーンショット / 実行例

<CLI ツールの場合: 実行例を貼る。なければ省略>
```

---

## 注意

- Breaking change がある場合は冒頭に ⚠️ を付けて明記する。
- 関連 Issue がある場合は `Closes #<number>` を末尾に追加するようユーザーに促す。
- PR タイトルも合わせて提案する（Conventional Commits 形式）。

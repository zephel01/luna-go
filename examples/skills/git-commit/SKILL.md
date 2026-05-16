---
name: git-commit
description: ステージ済みの変更を確認して Conventional Commits 形式のコミットメッセージを生成し、コミットする
---

## 手順

1. `bash({"command": "git diff --cached --stat"})` でステージ済みファイルの一覧を確認する。
   - 何もなければ「ステージされた変更がありません。`git add` してください」と伝えて終了。

2. `bash({"command": "git diff --cached"})` で差分の詳細を確認する。

3. 差分の内容から **Conventional Commits** 形式のコミットメッセージを生成する。

   形式:
   ```
   <type>(<scope>): <subject>

   <body>  ← 変更理由や背景が必要な場合のみ
   ```

   type の選択基準:
   - `feat` — 新機能
   - `fix` — バグ修正
   - `refactor` — 動作を変えないリファクタ
   - `docs` — ドキュメントのみの変更
   - `test` — テストの追加・修正
   - `chore` — ビルド設定・依存関係・CI など
   - `perf` — パフォーマンス改善
   - `style` — フォーマットのみ（空白・セミコロン等）

   ルール:
   - subject は命令形・現在形・50 文字以内・末尾にピリオドなし
   - scope はファイル名・パッケージ名・機能名など（省略可）
   - 破壊的変更がある場合は `BREAKING CHANGE:` を body に記載

4. 生成したメッセージをユーザーに提示し、確認を求める。
   - 修正が必要であれば修正案を出す。
   - 承認されたら `bash({"command": "git commit -m '<message>'"})` を実行する。

## 使用例

```
> /skill:git-commit

ステージ済みの変更:
 M internal/agent/agent.go
 M internal/tools/bash.go

提案するコミットメッセージ:
feat(agent): add /goal autonomous loop and [y/a/N] confirmation

- loop() continues iterating when goal is set and "ゴール達成" not found
- bash/write confirm prompts now support 'a' to enable autoApprove

このメッセージでコミットしますか？ [y/N]
```

# Webhook Pipeline

GitHub webhook → Hermes → gh CLI → PR 作成のパイプライン。

## イベントルーティング

```
issues (opened)       → Hermes 実装
issue_comment (@gemma) → Gemma チェック (Ollama)
issue_comment (その他) → 内容に応じて対応
check_suite (success) → 自動マージ判定
```

## 疎通確認

- 2026-07-03: Issue #55 で webhook → Hermes → gh CLI → PR 作成の疎通を確認
- 2026-07-05: Issue #73 で webhook 復元後の疎通を確認（action=closed → Hermes 実装フロー）
- Hermes は `gh` CLI で GitHub 操作（MCP 不使用）
- 認証: `~/.hermes/.env` の `GH_TOKEN` を使用

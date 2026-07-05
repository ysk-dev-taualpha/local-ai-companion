# Agent Tool Calling 設計

## 概要

Go Runtime に Agent Tool Calling 基盤を追加し、LLM（Gemma 4 / Ollama）が tool call を提案し、
Go Runtime が tool の実行・権限制御・監査ログを担当する。

## アーキテクチャ

```
┌─────────────────────────────────────────────────┐
│                  AgentLoop                       │
│  ┌──────────┐    ┌──────────────┐               │
│  │ Ollama   │───▶│ ToolExecutor │               │
│  │ Client   │◀───│              │               │
│  └──────────┘    │ ┌──────────┐ │               │
│                  │ │ToolPolicy│ │               │
│                  │ └──────────┘ │               │
│                  │ ┌──────────┐ │               │
│                  │ │ AuditLog │ │               │
│                  │ └──────────┘ │               │
│                  └──────┬───────┘               │
│                         │                       │
│                  ┌──────▼───────┐               │
│                  │ ToolRegistry │               │
│                  │ ┌──────────┐ │               │
│                  │ │web_search│ │               │
│                  │ │web_fetch │ │               │
│                  │ │audio_ctrl│ │               │
│                  │ │set_state │ │               │
│                  │ └──────────┘ │               │
│                  └──────────────┘               │
└─────────────────────────────────────────────────┘
```

## パッケージ構成

```
internal/agent/
├── tool.go         # Tool interface, ToolRegistry
├── policy.go       # ToolPolicy (allowlist)
├── executor.go     # ToolExecutor
├── audit.go        # AuditLog
├── ollama.go       # Ollama client (/api/chat with tools)
├── loop.go         # AgentLoop (orchestration)
└── tools/
    ├── web_search.go    # DuckDuckGo Instant Answer API
    ├── web_fetch.go     # URL content fetch
    ├── audio_control.go # Audio playback control
    └── set_state.go     # State management
```

## データフロー

```
User Message
     │
     ▼
AgentLoop.Run(ctx, userMessage)
     │
     ▼
OllamaClient.Chat(messages, tools)
     │
     ├── tool_calls なし → 最終応答を返す
     │
     └── tool_calls あり
           │
           ▼
     ToolExecutor.Execute(toolCall)
           │
           ├── ToolPolicy.Check() → DENIED → error
           ├── ToolRegistry.Get()  → not found → error
           └── Tool.Execute(ctx, args) → result/error
                    │
                    ▼
            tool result → role:"tool" で messages に追加
                    │
                    ▼
            OllamaClient.Chat() 再呼び出し
                    │
                    ▼
            最終応答 or 次の tool_calls（maxIter までループ）
```

## 安全方針

| 項目 | 実装 |
|------|------|
| allowlist | `ToolPolicy` が allowlist にない tool を拒否（`PolicyDenied`） |
| malformed arguments | JSON パース失敗時は `PolicyMalformed`、構造化エラーを LLM に返す |
| secret 保護 | tool result / log に secret を含めない（tool 実装の責務） |
| 危険 tool 除外 | `write_file`, `run_command`, `git push` 等は初期実装に含めない |
| context propagation | `ctx` を tool Execute に伝播、timeout/cancel で停止 |

## 初期 tool

### web_search

- **provider**: DuckDuckGo Instant Answer API（API キー不要）
- **インターフェース**: `WebSearchProvider`（将来 SearxNG / Brave / Tavily に差し替え可）
- **パラメータ**: `query` (必須), `max_results` (1-10, デフォルト 5)
- **戻り値**: `{"results": [...], "query": "..."}`

### web_fetch

- **説明**: URL のコンテンツを取得（HTML → テキスト抽出）
- **制限**: http/https のみ、1MB 応答制限、10000 文字に切り詰め
- **パラメータ**: `url` (必須)
- **戻り値**: `{"url": "...", "content": "...", "status_code": N}`

### audio_control

- **説明**: Unity クライアントへの音声制御指示
- **パラメータ**: `action` (必須, enum: speak/stop/pause/resume)
- **戻り値**: `{"action": "...", "message": "..."}`

### set_state

- **説明**: コンパニオンの状態設定
- **パラメータ**: `state` (必須, 文字列)
- **戻り値**: `{"previous": "...", "current": "..."}`

## 設定

```json
{
  "agent": {
    "enabled": false,
    "ollama_url": "http://192.168.12.107:11434",
    "ollama_model": "g4v100",
    "max_iter": 5,
    "system_prompt": "",
    "allowed_tools": [],
    "audit_size": 1000
  }
}
```

- `allowed_tools` が空の場合は全 tool 許可（allowlist 無効）
- `audit_size` は監査ログの最大エントリ数（0 = 無制限）

## 監査ログ

`AuditLog` は各 tool 実行の以下を記録:

- `timestamp` — 実行時刻
- `request_id` — リクエスト識別子
- `tool_call_id` — Ollama の tool call ID
- `tool_name` — tool 名
- `policy` — ALLOWED / DENIED / MALFORMED
- `duration` — 実行時間
- `error` — エラー内容（あれば）

## Unity WebSocket 互換性

- 既存の WebSocket contract（`type: text`, `type: ai_response`, `type: audio`）は変更なし
- Agent loop は独立した機能として追加され、既存の会話フローには影響しない
- 将来の統合時に `type: agent_response` を追加予定

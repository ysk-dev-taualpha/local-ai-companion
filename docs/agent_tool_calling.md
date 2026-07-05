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
RuntimeContext を system message に注入
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
| runtime context | current date / time / timezone / locale を LLM 呼び出しごとに注入 |

## 初期 tool

### web_search

- **provider**: Ollama 公式 Web Search API（`agent.web_search_url` + `OLLAMA_API_KEY`）
- **fallback**: API key 未設定時は DuckDuckGo HTML 検索
- **インターフェース**: `WebSearchProvider`（将来 SearxNG / Brave / Tavily に差し替え可）
- **パラメータ**: `query` (必須), `max_results` (デフォルト 3)
- **戻り値**: 検索結果の番号付きテキスト

### web_fetch

- **説明**: URL のコンテンツを取得（HTML → テキスト抽出）
- **制限**: http/https のみ、1MB 応答制限、10000 文字に切り詰め
- **パラメータ**: `url` (必須)
- **戻り値**: `{"url": "...", "content": "...", "status_code": N}`

### audio_control

- **説明**: Unity クライアントへの音声制御指示
- **パラメータ**: `action` (必須, enum: stop/clear_queue)
- **戻り値**: `{"action": "...", "message": "..."}`

### set_state

- **説明**: コンパニオンの状態設定
- **パラメータ**: `state` (必須, enum: IDLE/LISTENING/THINKING/SPEAKING)
- **戻り値**: `{"previous": "...", "current": "..."}`

## 設定

```json
{
  "ollama": {
    "enabled": false,
    "base_url": "http://192.168.12.107:11434",
    "model": "g4v100",
    "timeout_ms": 60000
  },
  "agent": {
    "enabled": false,
    "max_tool_loops": 5,
    "system_prompt": "あなたは local-ai-companion です。日本語で応答し、必要に応じてツールを使用してください。",
    "allowed_tools": ["web_search", "web_fetch", "audio_control", "set_state"],
    "timezone": "Asia/Tokyo",
    "locale": "ja-JP",
    "web_search_url": "https://ollama.com",
    "web_search_api_key_env": "OLLAMA_API_KEY"
  }
}
```

- `timezone` / `locale` は RuntimeContext として LLM 呼び出しごとに system message へ注入する。
- `web_search_api_key_env` は環境変数名であり、API key 値そのものを config に書かない。
- `allowed_tools` に列挙された tool のみ実行する。

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

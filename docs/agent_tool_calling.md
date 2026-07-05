# Agent Tool Calling Design

## Purpose

Go Runtime を agent host として拡張し、Gemma 4 / Ollama が選択した tool call を安全に実行できる基盤を追加する。

この設計は、Unity companion の会話・音声・状態制御を保ちつつ、将来的に web 検索、記憶検索、リポジトリ参照、GitHub 連携へ拡張するための土台とする。

## Core Idea

LLM は tool を直接実行しない。LLM は必要な tool call を提案し、Go Runtime が権限、状態、ログ、実行回数を管理したうえで tool を実行する。

Go Runtime は tool calling 以前の前提情報として、現在日時、timezone、locale などの runtime context を LLM 呼び出し時に注入する。現在日時のように外部検索なしで Runtime が確定できる情報は tool に過剰依存せず、system message 側で毎回提供する。

```text
Unity
  -> user text / voice event
Go Runtime
  -> inject runtime context
  -> Ollama / Gemma 4 chat request with tools
      <- assistant message with tool_calls
  -> ToolPolicy check
  -> ToolExecutor run
  -> tool results appended to conversation
  -> Ollama / Gemma 4 follow-up request
      <- final assistant response
Go Runtime
  -> ai_response / state_change / audio / audio_control
Unity
```

## Component Responsibilities

### AgentLoop

Ollama `/api/chat` との往復を管理する。

Responsibilities:

- user message と conversation history を組み立てる
- runtime context を system message に注入する
- ToolRegistry から tool schema を取得して Ollama に渡す
- `tool_calls` を検出する
- tool result を `role: tool` として会話履歴に追加する
- 最大 loop 回数を超えた場合は安全に停止する
- 最終応答を既存の `ai_response` として返す

### RuntimeContext

LLM 呼び出しごとに注入する実行時コンテキストを生成する。

Responsibilities:

- current date / time / timezone を Runtime のシステム時刻から生成する
- locale や user location など、回答方針に影響する安定情報を保持する
- 相対日付、現在年、曜日に関する質問では Runtime の日時を authoritative とする指示を含める
- secret や不要な環境情報を含めない

Example system context:

```text
Current date: 2026-07-05
Current time: 16:50:00
Timezone: Asia/Tokyo
Locale: ja-JP

When answering questions about today, yesterday, tomorrow, current year,
weekdays, schedules, or recentness, use this runtime date as authoritative.
Use web_search only when external current information is required.
```

### ToolRegistry

Runtime が提供する tool の一覧と JSON schema を保持する。

Responsibilities:

- tool 名の一意性を保証する
- Ollama に渡す tool schema を生成する
- tool 名から executor を解決する
- provider 差し替え時も外部 contract を維持する

### ToolPolicy

tool 実行の許可、制限、確認要否を判断する。

Responsibilities:

- tool ごとの許可/禁止を判断する
- request 単位の最大 tool call 回数を制限する
- 同一 tool の連続呼び出しを制限する
- 将来の危険操作に approval を要求できる境界を提供する
- audit log に必要な判断理由を残す

### ToolExecutor

実際の tool 処理を実行する。

Responsibilities:

- tool arguments を検証する
- timeout / context cancellation を尊重する
- 実行結果を構造化して返す
- secret や不要な個人情報を結果に含めない
- 失敗時も LLM に渡せるエラー結果を返す

## Initial Tools

最初の実装では危険操作を含まない tool に限定する。

| tool | purpose | side effect |
|------|---------|-------------|
| `web_search(query)` | 最新情報を検索する | 外部 API 呼び出し |
| `web_fetch(url)` | 指定 URL の本文を取得する | 外部 API 呼び出し |
| `audio_control(action)` | Unity の音声再生を停止/キュー消去する | Unity playback 状態変更 |
| `set_state(state)` | Runtime/Unity の会話状態を変更する | state machine 変更 |

`web_search` と `web_fetch` の初期 provider は Ollama 公式 Web Search API とする。Runtime 側では provider interface を切り、後から SearxNG、Brave Search、Tavily などへ差し替え可能にする。

## Tool Message Contracts

### web_search

Request:

```json
{
  "query": "Gemma 4 tool calling",
  "max_results": 5
}
```

Result:

```json
{
  "results": [
    {
      "title": "Result title",
      "url": "https://example.com",
      "content": "短い要約またはスニペット"
    }
  ]
}
```

### web_fetch

Request:

```json
{
  "url": "https://example.com"
}
```

Result:

```json
{
  "title": "Page title",
  "content": "取得した本文",
  "links": ["https://example.com/next"]
}
```

### audio_control

Request:

```json
{
  "action": "stop"
}
```

Allowed actions:

- `stop`
- `clear_queue`

Runtime は既存 WebSocket contract の `type: "audio_control"` を Unity に送信する。

### set_state

Request:

```json
{
  "state": "THINKING"
}
```

Allowed states:

- `IDLE`
- `LISTENING`
- `THINKING`
- `SPEAKING`

Runtime は既存 StateMachine の有効遷移だけを許可する。

## Safety Policy

初期実装では以下を必須とする。

- current date / time / timezone は LLM 呼び出しごとに runtime context として注入する
- tool loop の最大回数を設定する
- request timeout と context cancellation を全 tool に伝播する
- tool call と result の audit log を残す
- secret を log / tool result / LLM prompt に含めない
- allowlist に存在しない tool は実行しない
- malformed arguments は実行せず、構造化エラーとして LLM に返す
- `write_file`, `run_command`, `git push` などの危険 tool は初期実装に含めない

将来、ファイル書き込みやコマンド実行を追加する場合は、workspace sandbox、approval、diff review、destructive operation guard を別設計として追加する。

## Phased Expansion

### Phase 1: Safe Companion Tools

- `web_search`
- `web_fetch`
- `audio_control`
- `set_state`
- audit log
- loop limit

### Phase 2: Read-Only Development Tools

- `repo_search`
- `read_file`
- `github_get_issue`
- `github_comment_issue`

### Phase 3: Controlled Code-Agent Tools

- `write_file`
- `run_tests`
- `create_branch`
- `create_pull_request`

Phase 3 は approval と sandbox が実装されるまで開始しない。

## Open Questions

- Ollama 公式 Web Search API の API key をどの設定ファイルに置くか
- Gemma 4 の推奨モデルを `26b` とするか `31b` とするか
- Python AI Service に残す処理と Go Runtime に移す処理の境界
- tool result を conversation history に永続化するか、一時 context のみにするか

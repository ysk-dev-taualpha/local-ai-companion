# Architecture

## Responsibility Split

### Unity

Unityは表示、入力、キャラクター制御を担当する。

Primary responsibilities:

- キャラクター表示
- 表情制御
- モーション再生
- 字幕表示
- 口パク
- ユーザー入力UI
- Go Runtimeとの通信

UnityはAI処理を直接持たない。Go Runtimeから受け取ったイベントを表示・再生する。

Unity 側の詳細設計は [unity_design.md](unity_design.md) を参照する。

### Go Runtime

Go Runtimeは常駐プロセス、通信、並行処理、キャンセル制御を担当する。

Primary responsibilities:

- Unityとの通信
- Python AI Serviceとの通信
- 外部API通信の集約
- APIキー管理
- request_id管理
- timeout / cancel制御
- goroutineによる並行処理
- イベント配送
- ログ
- 設定管理
- プロセス監視

Go Runtimeはアシスタント全体の神経系として振る舞う。LLMの知能そのものよりも、イベント制御、安全管理、サービス間接続を担当する。

### Python AI Service

Python AI ServiceはLLM、STT、TTS、RAG、ML系処理を担当する。

Primary responsibilities:

- プロンプト生成
- LLM応答生成
- JSON応答整形
- JSON検証
- フォールバック応答
- 会話履歴処理
- STT
- TTS
- RAG
- 埋め込み処理
- ローカルLLM連携

初期段階ではPythonが直接LLMを呼び出してよい。最終的には外部APIキー管理と外部API通信をGo Runtimeへ寄せる。

## Initial Data Flow

```text
User
  ↓ text
Python Conversation Core
  ↓ response JSON
CLI / logs
```

## Target Data Flow

```text
User
  ↓ voice / text
Unity
  ↓ event
Go Runtime
  ↓ request
Python AI Service
  ↓ LLM request, when needed
Go LLM Gateway
  ↓
External API / Local LLM
  ↓
Go Runtime
  ↓ event
Unity Character
```

## Communication Policy

Early stages should use simple HTTP + JSON where possible.

Use WebSocket when bidirectional event streaming is needed, especially between Unity and Go Runtime.

Use request_id for all cross-service requests.

## Cancellation Policy

Go Runtime owns conversation-level cancellation.

When a user interrupts, Unity disconnects, a timeout occurs, or a newer request supersedes the current one, Go Runtime cancels the active context and propagates cancellation to dependent operations.

Operations affected by cancellation:

- LLM generation
- TTS generation
- TTS playback
- response streaming
- character speech events

## v0.4: TTS Data Flow

v0.4 では TTS 出力が追加され、Python AI Service が生成した音声データが Go Runtime 経由で Unity に配送される。

```text
User
  ↓ text (WebSocket)
Unity
  ↓ {"type":"text", ...}
Go Runtime
  ↓ HTTP POST /v1/conversation
Python AI Service
  ↓ assistant response + audio data
Go Runtime
  ↓ WebSocket: ai_response + audio
Unity
  ↓ 字幕表示 + 音声再生
User
```

### 音声配送フロー

1. Python AI Service が `assistant.text` から TTS 音声を生成
2. Go Runtime が WebSocket で `ai_response` → `audio` の順に送信
3. Unity が `ai_response` のテキストを字幕表示し、続く `audio` を再生
4. 再生完了後、`state_change: IDLE` で待機状態に戻る

### TTS 責務分担

| 責務 | 担当 |
|------|------|
| TTS 音声生成 | Python AI Service |
| 音声データ配送 | Go Runtime |
| 音声再生・口パク同期 | Unity |

TTS の詳細な API 契約は [api_contracts.md](api_contracts.md) の TTS セクションを参照。

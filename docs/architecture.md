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

## v0.4: TTS Integration

v0.4 では TTS（Text-to-Speech）連携を導入。Python AI Service が生成した応答テキストを音声に変換し、WebSocket 経由で Unity クライアントに配信する。

### v0.4 Data Flow

```text
User
  ↓ voice / text
Unity
  ↓ WebSocket: type:"text"
Go Runtime
  ↓ HTTP request
Python AI Service
  ↓ LLM response JSON
Go Runtime
  ↓ VOICEVOX TTS (audio_query → synthesis)
  ↓ WebSocket: type:"ai_response" (audio フィールドに base64 WAV)
Unity Character
  ↓ text display → audio playback
```

### Key additions in v0.4

- WebSocket `ai_response` に `audio` フィールド（base64 WAV）を追加（`docs/api_contracts.md` 参照）
- Go Runtime が VOICEVOX を直接呼び出し（`internal/tts/` パッケージ）
- `config.json` の `tts` セクションで有効化・話者設定
- TTS 失敗時はログ出力のみで AI 応答テキストは通常通り返す

## v0.5: Voice Input

v0.5 では音声入力（Voice Input）を導入。ユーザーの音声を Unity でキャプチャし、Go Runtime → Python AI Service 経由で VAD（Voice Activity Detection）と STT（Speech-to-Text）を順次処理し、認識テキストを既存の会話フローに統合する。

### v0.5 Data Flow

```text
Unity (WinPC)
  ↓ WebSocket binary: audio_chunk (100ms PCM)
Go Runtime (X1C6)
  ↓ HTTP: POST /vad/chunk (PCM streaming)
Python AI Service (X1C6)
  ↓ Silero VAD → speech_start / speech_end 検出
  ↓ speech_end 時: WAV → HTTP POST /v1/transcribe
WinPC faster-whisper (CUDA)
  ↓ text
Python AI Service
  ↓ 既存会話フロー (LLM → TTS)
Go Runtime
  ↓ WebSocket: ai_response (audio + text)
Unity
```

### Component Responsibilities

- **Unity (WinPC)**: マイクキャプチャ、100ms PCM チャンクの WebSocket バイナリフレーム送信、認識テキスト表示、キャンセル UI（押しボタン式）
- **Go Runtime (X1C6)**: WebSocket バイナリフレームの受信・中継、VAD イベント (`speech_start`, `speech_end`) の内部配信、既存会話フローとの統合、タイムアウト制御
- **Python AI Service (X1C6)**: Silero VAD による発話区間検出（PCM ストリーミング入力）、`speech_end` 時の WAV バッファ回収と WinPC への STT リクエスト送信、認識テキストの既存会話ロジックへの注入
- **WinPC faster-whisper**: STT エンジン（faster-whisper `small` モデル、CUDA アクセラレーション）、`POST /v1/transcribe` で WAV 受信 → テキスト返却

### GPU Memory Budget (RTX 2080 SUPER 8GB)

| プロセス | 使用量 |
|---------|--------|
| Ollama g4v100 | ~6.0 GB |
| faster-whisper small | ~1.5 GB |
| **合計** | **~7.5 GB** |

Ollama と faster-whisper は同一 GPU 上で常駐する。8GB VRAM 上限に対して 7.5GB と余裕が少ないため、Ollama のコンテキスト長やバッチサイズの調整が必要な場合がある。faster-whisper は STT 処理時のみ推論を実行し、待機時はモデルを GPU に保持し続ける。

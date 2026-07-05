# API Contracts

## Go Runtime エンドポイント一覧

| メソッド | パス | 説明 |
|----------|------|------|
| POST | `/v1/conversation` | 会話リクエストを Python サービスに転送 |
| GET | `/healthz` | ヘルスチェック |
| GET | `/ws` | WebSocket 接続（Upgrade） |

## Assistant Response JSON

ConversationCore が返す assistant response のスキーマ。

```json
{
  "text": "返答本文",
  "emotion": "neutral",
  "motion": "idle",
  "speak_style": "normal",
  "interruptible": true
}
```

## Fields

### text

アシスタントの応答テキスト。

制約:

- 文字列であること
- 空文字列・空白のみは不可（strip 後に 1 文字以上）
- 最大 500 文字（`MAX_TEXT_LENGTH`）。TTS での発話長を考慮した制限

### emotion

許可値（8 種）:

```text
neutral
happy
sad
thinking
surprised
angry
sleepy
confident
```

実装箇所: `src/local_ai_companion/schema.py` → `ALLOWED_EMOTIONS`

### motion

許可値（7 種）:

```text
idle
nod
shake_head
wave
look_away
think
point
```

実装箇所: `src/local_ai_companion/schema.py` → `ALLOWED_MOTIONS`

### speak_style

許可値（6 種）:

```text
normal
soft
fast
slow
serious
playful
```

実装箇所: `src/local_ai_companion/schema.py` → `ALLOWED_SPEAK_STYLES`

### interruptible

発話再生中に割り込み可能かどうか。

制約:

- boolean であること（`True` / `False`）

## Validation

`validate_assistant_response(value)` が以下を実行する:

1. 入力が dict であることを確認
2. 全フィールドの存在確認
3. `text`: 非空文字列かつ `MAX_TEXT_LENGTH` 以下
4. `emotion`: `ALLOWED_EMOTIONS` に含まれる
5. `motion`: `ALLOWED_MOTIONS` に含まれる
6. `speak_style`: `ALLOWED_SPEAK_STYLES` に含まれる
7. `interruptible`: bool 型

違反時は `ResponseValidationError(ValueError)` を送出する。

## Fallback Response

LLM の応答が validation を通過できなかった場合、`fallback_response()` が以下を返す:

```json
{
  "text": "すみません、応答を整えるところで失敗しました。もう一度お願いします。",
  "emotion": "neutral",
  "motion": "idle",
  "speak_style": "soft",
  "interruptible": true
}
```

fallback 自体も `validate_assistant_response()` を通過することをテストで保証する。

## Conversation Request

ConversationCore.send() の入力:

```text
user_text: str          # ユーザーのテキスト入力
conversation_id: str    # 会話セッション識別子（デフォルト "default"）
request_id: str | None  # リクエスト識別子。未指定時は自動生成 (uuid4)
```

## Conversation Response

ConversationCore.send() の戻り値:

```json
{
  "request_id": "uuid",
  "conversation_id": "default",
  "assistant": {
    "text": "受け取りました: ...",
    "emotion": "neutral",
    "motion": "nod",
    "speak_style": "normal",
    "interruptible": true
  }
}
```

## Error Handling

- JSON パース失敗時 → `fallback_response()` を返す
- バリデーション失敗時 → `fallback_response()` を返す
- 内部履歴には `valid: false` と `error` メッセージを記録する

エラーレスポンスに API キーや raw secrets、不要な個人情報を含めないこと。

## Go Runtime: /v1/conversation

**メソッド:** `POST`

**リクエストボディ:**

```json
{
  "message": "ユーザーの入力テキスト",
  "conversation_id": "default",
  "request_id": "32byte-hex-string"
}
```

| フィールド | 型 | 必須 | 説明 |
|------------|-----|------|------|
| `message` | string | **必須** | ユーザーの入力テキスト |
| `conversation_id` | string | 任意 | 会話セッション識別子。未指定時は Python 側のデフォルト |
| `request_id` | string | 任意 | リクエスト識別子。未指定時は 32byte ランダム hex 文字列を自動生成 |

**成功レスポンス (200):**

```json
{
  "request_id": "a1b2c3d4e5f6...",
  "conversation_id": "default",
  "assistant": {
    "text": "こんにちは、何かお手伝いしましょうか？",
    "emotion": "happy",
    "motion": "wave",
    "speak_style": "normal",
    "interruptible": true
  }
}
```

**エラーレスポンス:**

| ステータス | コード | 説明 |
|------------|--------|------|
| 400 | `invalid_request` | JSON パース失敗、または `message` が空 |
| 405 | `method_not_allowed` | POST 以外のメソッド |
| 502 | `python_service_error` | Python サービスのエラー |
| 504 | `timeout` | リクエストタイムアウト（`RequestTimeoutMs` 設定値） |

エラーレスポンスのフォーマット:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "message is required"
  }
}
```

実装箇所: `internal/api/handler.go` → `HandleConversation`

## Go Runtime: /healthz

**メソッド:** `GET`

**レスポンス (200):**

```json
{
  "status": "ok"
}
```

常に 200 を返す。実装箇所: `internal/api/handler.go` → `HandleHealth`

## Go Runtime: Python Service 起動設定

Go Runtime は `python_service.command` が空の場合、`python_service.base_url` で指定された外部 Python AI Service に接続する。
`python_service.command` が指定された場合は Runtime 起動時に子プロセスとして Python AI Service を起動し、`base_url` に HTTP 到達できるまで待ってから Runtime を ready とする。

```json
{
  "python_service": {
    "base_url": "http://127.0.0.1:8090",
    "command": "PYTHONPATH=./src python3 -m local_ai_companion --serve",
    "ready_timeout_ms": 10000,
    "shutdown_timeout_ms": 5000
  }
}
```

| フィールド | 型 | 必須 | 説明 |
|------------|----|:--:|------|
| `base_url` | string | 任意 | Python AI Service の base URL。既定値は `http://127.0.0.1:8090` |
| `command` | string | 任意 | 子プロセスとして起動する shell command。空文字の場合は起動管理しない |
| `ready_timeout_ms` | number | 任意 | 起動後に `base_url` へ到達可能になるまで待つ最大時間。既定値は `10000` |
| `shutdown_timeout_ms` | number | 任意 | Runtime shutdown 時に子プロセス停止を待つ最大時間。既定値は `5000` |

実装箇所:

- `internal/config/config.go` → `PythonServiceConfig`
- `internal/pythonservice/service.go` → `Service`
- `cmd/local-ai-runtime/main.go`

## Go Runtime: /ws — WebSocket

**プロトコル:** WebSocket（`ws://` または `wss://`）。HTTP GET を WebSocket に Upgrade して利用する。

**接続管理:** `WebSocketHub` が `sync.RWMutex` で goroutine-safe に全接続を管理。切断時は自動的にクリーンアップされる。

### WebSocketHub API

| メソッド | シグネチャ | 説明 |
|----------|-----------|------|
| `NewWebSocketHub(pythonClient, stateMachine, requestTimeoutMs)` | `*WebSocketHub` | Hub を生成 |
| `HandleWS(w, r)` | `http.HandlerFunc` | `/ws` エンドポイントのハンドラ |
| `Broadcast(msg)` | `error` | 全接続クライアントにメッセージを一斉送信 |
| `ConnectionCount()` | `int` | 現在の接続数を返す |

### メッセージフォーマット (WSMessage)

```json
{
  "type": "text",
  "payload": "送信内容",
  "request_id": "req-001"
}
```

| フィールド | 型 | 必須 | 説明 |
|------------|-----|------|------|
| `type` | string | **必須** | メッセージ種別。`text` は AI Service 連携、それ以外は ack エコー |
| `payload` | string | **必須** | メッセージ本文 |
| `request_id` | string | 任意 | リクエスト識別子。`json:"request_id,omitempty"` |

**リクエスト例:**

```json
{
  "type": "text",
  "payload": "Hello, Unity!",
  "request_id": "req-001"
}
```

**`type: "text"` のレスポンス:**

`type: "text"` は Python AI Service に転送され、状態遷移通知と AI 応答を WebSocket で返す。

状態遷移通知:

```json
{
  "type": "state_change",
  "state": "LISTENING"
}
```

AI 応答:

```json
{
  "type": "ai_response",
  "request_id": "req-001",
  "conversation_id": "default",
  "assistant": {
    "text": "こんにちは、何かお手伝いしましょうか？",
    "emotion": "happy",
    "motion": "wave",
    "speak_style": "normal",
    "interruptible": true
  }
}
```

エラー時:

```json
{
  "type": "error",
  "request_id": "req-001",
  "error": "python service error"
}
```

状態遷移は通常 `LISTENING` → `THINKING` → `SPEAKING` → `IDLE` の順で通知される。

**非 `text` メッセージのレスポンス（ack）:**

`type: "text"` 以外のメッセージは、後方互換のため `type` に `_ack` を付加してエコーバックする:

```json
{
  "type": "ping_ack",
  "payload": "Hello, Unity!",
  "request_id": "req-001"
}
```

**Broadcast 例:**

```json
{
  "type": "broadcast",
  "payload": "hello all"
}
```

Broadcast は全接続クライアントに同じメッセージを送信する。個別の送信エラーは無視され、他の接続への送信は継続される。

### `audio` メッセージタイプ（v0.4 TTS 連携）

`ai_response` に続いて音声データを送信する。

```json
{
  "type": "audio",
  "request_id": "req-001",
  "format": "wav",
  "sample_rate": 24000,
  "channels": 1,
  "data": "base64_encoded_audio_data",
  "is_last": true
}
```

| フィールド | 型 | 必須 | 説明 |
|------------|-----|------|------|
| `type` | string | **必須** | `"audio"` 固定 |
| `request_id` | string | 任意 | 対応する `ai_response` の `request_id` |
| `format` | string | **必須** | 音声フォーマット。`"wav"` または `"mp3"` |
| `sample_rate` | number | **必須** | サンプルレート（Hz） |
| `channels` | number | **必須** | チャンネル数。モノラルは `1` |
| `data` | string | **必須** | Base64 エンコードされた音声データ |
| `is_last` | boolean | **必須** | 最終チャンクかどうか。分割送信時に使用 |

**注意事項:**

- 不正な JSON を受信した場合、そのメッセージは無視され接続は維持される
- `request_id` が空の場合、レスポンスの `request_id` は空文字列になる（`omitempty` によりキー自体が省略される場合もある）
- 全オリジンを許可（`CheckOrigin: true`）
- 同時接続は goroutine-safe に管理される
- 同一 WebSocket 接続への書き込みは接続ごとの mutex で直列化される
- `StateMachine` の `Current` / `Transition` / `Reset` は goroutine-safe
- WebSocket の会話フロー全体は `WebSocketHub` 側でも直列化される
- `ai_response` の直後に `audio` メッセージが続く。`audio` がない場合は音声なし応答とみなす

実装箇所: `internal/api/websocket.go` → `HandleWS` / `WebSocketHub`

## TTS (Text-to-Speech) Integration

v0.4 で導入された TTS 連携の API 仕様。

### WebSocket `audio` メッセージ

`ai_response` に続いて、音声データを `audio` タイプでクライアントに送信する。

```json
{
  "type": "audio",
  "request_id": "req-001",
  "conversation_id": "default",
  "payload": "<base64-encoded WAV audio>",
  "format": "wav",
  "sample_rate": 24000
}
```

| フィールド | 型 | 必須 | 説明 |
|------------|-----|:--:|------|
| `type` | string | **必須** | `"audio"` 固定 |
| `request_id` | string | 任意 | 対応する `ai_response` の request_id |
| `conversation_id` | string | 任意 | 会話セッション識別子 |
| `payload` | string | **必須** | base64 エンコードされた音声データ |
| `format` | string | **必須** | 音声フォーマット。`"wav"` または `"mp3"` |
| `sample_rate` | number | 任意 | サンプルレート（Hz）。既定値 24000 |

### TTS フロー

```text
1. クライアントが type: "text" を WebSocket で送信
2. Go Runtime → Python AI Service が応答 JSON を生成
3. Go Runtime が type: "ai_response" を WebSocket で送信
4. Go Runtime が Python AI Service に TTS 生成をリクエスト
5. Go Runtime が type: "audio" を WebSocket で送信（base64 エンコード音声）
6. クライアント（Unity）が audio データを受信し、再生
```

ai_response と audio の連続送信は同一 WebSocket 接続上で行われ、request_id で紐付けられる。

### Unity 受信仕様

- `ai_response` 受信後、字幕・表情・モーションを先に表示する
- `audio` メッセージ受信時に音声データをデコード・再生する
- 音声再生は `interruptible` フラグに従い、`true` の場合はユーザー入力で割り込み可能
- 再生完了後、IDLE 状態に遷移する

### TTS キャンセル制御

- TTS 生成中に新しい `type: "text"` を受信した場合、前の生成をキャンセルする
- TTS 再生中に割り込みが発生した場合、`interruptible: true` であれば再生を停止し新しいリクエストの処理を開始する
- キャンセル制御は Go Runtime が `context.Context` で管理する

## StateMachine API

会話状態を管理するステートマシン。許可された遷移のみを実行し、遷移時にコールバックを発火する。`Current` / `Transition` / `Reset` は goroutine-safe。

実装箇所: `internal/state/state.go`

### 状態一覧

| 定数 | 値 | 日本語ラベル | 説明 |
|------|-----|-------------|------|
| `IDLE` | 0 | 待機中 | 初期状態。会話待ち |
| `LISTENING` | 1 | 受信中 | ユーザー音声受信中 |
| `THINKING` | 2 | 思考中 | 応答生成中 |
| `SPEAKING` | 3 | 発話中 | 音声出力中 |

`State.String()` で日本語ラベルを取得可能。未定義値は `不明(N)` を返す。

### 状態遷移図

```
         ┌──────────────────┐
         │                  │
         ▼                  │
     ┌──────┐  cancel   ┌──┴───┐
     │ IDLE │◄──────────│LISTEN│
     └──┬───┘           └──┬───┘
        │                  │
        │ start            │ recognized
        │                  │
        ▼                  ▼
     ┌──────┐           ┌──────┐
     │LISTEN│           │THINK │
     └──────┘           └──┬───┘
                           │
                 cancel    │ generated
                    │      │
                    ▼      ▼
                 ┌──────┐┌──────┐
                 │ IDLE ││SPEAK │
                 └──────┘└──┬───┘
                            │
                            │ done
                            │
                            ▼
                         ┌──────┐
                         │ IDLE │
                         └──────┘
```

### 有効な遷移

| from | to | 説明 |
|------|-----|------|
| `IDLE` | `LISTENING` | ユーザー発話の聞き取り開始 |
| `LISTENING` | `THINKING` | 音声認識完了、応答生成開始 |
| `LISTENING` | `IDLE` | キャンセル（聞き取り中断） |
| `THINKING` | `SPEAKING` | 応答生成完了、発話開始 |
| `THINKING` | `IDLE` | キャンセル（生成中断） |
| `SPEAKING` | `IDLE` | 発話完了 |

同一状態への遷移（例: `IDLE` → `IDLE`）はエラーにならず、コールバックも発火しない（no-op）。

### 無効な遷移

以下の遷移はエラー（`invalid transition: X → Y`）になる:

- `IDLE` → `THINKING`, `SPEAKING`
- `LISTENING` → `SPEAKING`
- `THINKING` → `LISTENING`
- `SPEAKING` → `LISTENING`, `THINKING`

### API

```go
// 型
type State int  // iota: IDLE=0, LISTENING=1, THINKING=2, SPEAKING=3
type StateChangeCallback func(from, to State)
type StateMachine struct { /* unexported fields */ }

// コンストラクタ
func New(callback StateChangeCallback) *StateMachine

// メソッド
func (sm *StateMachine) Current() State
func (sm *StateMachine) Transition(to State) error
func (sm *StateMachine) Reset()
```

#### `New(callback) *StateMachine`

初期状態 `IDLE` で StateMachine を生成する。`callback` が非 nil の場合、有効な遷移時に `callback(from, to)` が呼ばれる。

#### `Current() State`

現在の状態を返す。複数 goroutine から同時に呼び出しても安全。

#### `Transition(to State) error`

`to` への遷移を試みる。状態の検証と更新は内部 mutex で保護される。

- 同一状態 → no-op（nil を返す、コールバック発火なし）
- 無効な遷移 → `fmt.Errorf("invalid transition: %s → %s", from, to)` を返す
- 有効な遷移 → 状態を更新し、`onChange` コールバックがあれば発火

コールバックは状態更新後、内部 mutex を解放してから呼び出される。コールバック内で `Current()` などの StateMachine メソッドを呼び出してもデッドロックしない。

#### `Reset()`

現在の状態にかかわらず `IDLE` に強制リセットする。実際に状態が変わった場合のみコールバックを発火する（`IDLE` からのリセットは no-op）。状態の読み取りと更新は内部 mutex で保護され、コールバックは mutex 解放後に呼び出される。

### コールバックの挙動

| 操作 | コールバック発火 | 備考 |
|------|:--:|------|
| 有効な遷移 (`IDLE`→`LISTENING`) | ✅ | `from=IDLE, to=LISTENING` |
| 同一状態遷移 (`IDLE`→`IDLE`) | ❌ | no-op |
| 無効な遷移 (`IDLE`→`THINKING`) | ❌ | エラーを返す |
| `Reset()` (非 IDLE から) | ✅ | `from=現在状態, to=IDLE` |
| `Reset()` (IDLE から) | ❌ | no-op |

実装箇所: `internal/state/state.go`

## TTS（v0.4）

v0.4 で導入される TTS（Text-to-Speech）の API 契約。

### 概要

Python AI Service が `assistant.text` から音声データを生成し、Go Runtime が WebSocket 経由で Unity クライアントに配送する。TTS 生成自体は Python AI Service の責務であり、Unity は音声再生のみを担当する。

### データフロー

```text
Python AI Service
  ↓ assistant response + audio data
Go Runtime
  ↓ WebSocket: ai_response → audio
Unity
  ↓ 音声再生
```

### WebSocket シーケンス

1. Unity が `{"type":"text", ...}` を送信
2. Runtime が `{"type":"state_change","state":"LISTENING"}` を返す
3. Runtime が `{"type":"state_change","state":"THINKING"}` を返す
4. Runtime が `{"type":"ai_response", ...}` を返す
5. Runtime が `{"type":"state_change","state":"SPEAKING"}` を返す
6. Runtime が `{"type":"audio", ...}` を返す（音声データ）
7. Runtime が `{"type":"state_change","state":"IDLE"}` を返す

`audio` メッセージは `ai_response` の直後、`SPEAKING` 状態遷移の後に送信される。`ai_response` に続いて `audio` が送信されない場合、音声なし応答（テキストのみ）とみなす。

### 音声フォーマット

| 設定 | 値 |
|------|-----|
| フォーマット | WAV（PCM 16bit）または MP3 |
| サンプルレート | 24000 Hz（VOICEVOX デフォルト） |
| チャンネル | 1（モノラル） |
| エンコード | Base64 |

### Unity 側の受信仕様

Unity は `ai_response` 受信後、続く `audio` メッセージを受信して音声再生を開始する。

- `ai_response` の `assistant.text` を字幕表示
- 続く `audio` メッセージの Base64 データをデコードし、`AudioSource` で再生
- `interruptible: true` の場合、ユーザー入力で再生を中断可能
- `speak_style` に応じて再生速度・ピッチを調整可能（将来の拡張）

実装箇所（将来）: `unity/v0.4-tts-output/Assets/Scripts/AudioPresenter.cs`

### キャンセル

TTS 生成中・再生中にキャンセルが発生した場合:

- Go Runtime が TTS 生成をキャンセルする
- Unity は再生中の音声を停止し、キューをクリアする
- キャンセル後は `state_change: IDLE` が送信される

## Voice Input（v0.5）

v0.5 で導入される音声入力（Voice Input）の API 契約。
Unity → Go Runtime への音声ストリーミング、VAD による発話区間検出、faster-whisper による音声認識のパイプラインを定義する。

### 概要

音声入力パイプラインは以下のコンポーネントで構成される:

| コンポーネント | 場所 | 役割 |
|--------------|------|------|
| Unity クライアント | WinPC (:8090/ws) | マイク入力 → PCM ストリーミング |
| Go Runtime | X1C6 (:8090) | WebSocket ハブ、VAD リレー、STT リレー |
| Python VAD Service | X1C6 (:8092) | Silero VAD による発話区間検出 |
| faster-whisper STT | WinPC (:8093) | 音声認識（Whisper small, CUDA） |

### データフロー

```text
Unity (マイク入力, 100ms PCM chunks)
  ↓ WebSocket バイナリフレーム: audio_chunk
Go Runtime
  ↓ HTTP POST /vad/chunk (application/octet-stream)
Python VAD Service → 発話区間検出 (Silero VAD)
  ↓ HTTP JSON: {event: "speech_start" | "idle"}
Go Runtime
  ↓ WebSocket JSON: vad_event
Unity

[speech_end 検出後]

Go Runtime
  ↓ HTTP POST /v1/transcribe (multipart/form-data: WAV)
faster-whisper STT (WinPC, CUDA)
  ↓ HTTP JSON: {text: "認識結果", ...}
Go Runtime
  ↓ WebSocket JSON: speech_recognized
Unity
```

### WebSocket バイナリフレーム: audio_chunk（Unity → Go）

Unity はマイク入力を 100ms チャンク単位で PCM int16 にエンコードし、WebSocket バイナリフレームとして Go Runtime にストリーミング送信する。

| フィールド | 型 | バイト数 | 説明 |
|------------|-----|:--:|------|
| `request_id` | 16 bytes (hex) | 16 | セッション中の音声入力識別子 |
| `seq` | uint32 | 4 | シーケンス番号（チャンクの順序） |
| `sample_rate` | uint16 | 2 | サンプルレート（Hz）。例: 16000 |
| `samples` | []int16 | 可変 | PCM 16bit リトルエンディアン音声サンプル |

バイナリフレーム構造:

```text
[request_id: 16 bytes][seq: uint32 LE][sample_rate: uint16 LE][PCM samples: int16 LE × N]
```

| 制約 | 値 |
|------|-----|
| チャンク長 | 100ms |
| サンプルレート | 16000 Hz（推奨）、24000 Hz も対応 |
| チャンネル | 1（モノラル） |
| 最大シーケンス番号 | 65535（オーバーフロー時は 0 にラップ） |

Go Runtime は受信した PCM チャンクをリクエスト単位でバッファリングし、VAD Service に転送する。

### WebSocket JSON: vad_event（Go → Unity）

VAD Service からの発話区間検出結果を Unity に通知する。

```json
{
  "type": "vad_event",
  "request_id": "a1b2c3d4e5f6a1b2",
  "event": "speech_start"
}
```

| フィールド | 型 | 必須 | 説明 |
|------------|-----|:--:|------|
| `type` | string | **必須** | `"vad_event"` 固定 |
| `request_id` | string | **必須** | 対応する audio_chunk の request_id |
| `event` | string | **必須** | `"speech_start"` または `"speech_end"` |

| `event` 値 | 意味 | Unity 側のアクション |
|-----------|------|---------------------|
| `"speech_start"` | 発話開始を検出 | 録音中 UI を表示 |
| `"speech_end"` | 発話終了を検出 | 認識結果待ち UI に遷移 |

**遷移保証:** `speech_start` は `speech_end` より先に送信される。`speech_start` がない状態で `speech_end` が送信されることはない。
`speech_start` 後に続く `speech_end` がない場合（VAD が発話終了を検出できなかった）、Go Runtime はタイムアウト（既定 5 秒）後に `speech_end` を自動送信する。

### WebSocket JSON: speech_recognized（Go → Unity）

faster-whisper による音声認識結果を Unity に通知する。

```json
{
  "type": "speech_recognized",
  "request_id": "a1b2c3d4e5f6a1b2",
  "text": "こんにちは、元気ですか？",
  "cancelable": true
}
```

| フィールド | 型 | 必須 | 説明 |
|------------|-----|:--:|------|
| `type` | string | **必須** | `"speech_recognized"` 固定 |
| `request_id` | string | **必須** | 対応する audio_chunk の request_id |
| `text` | string | **必須** | 認識されたテキスト。空文字列は認識失敗 |
| `cancelable` | boolean | **必須** | 認識結果をキャンセル可能かどうか |

`cancelable: true` の場合、Unity は認識結果を表示しつつキャンセルボタンを有効にする。
キャンセル時は `cancel_speech` メッセージを送信する。

`text` が空文字列の場合:
- 背景ノイズのみで音声が検出されなかった
- Unity は「聞き取れませんでした」の UI を表示する

### WebSocket JSON: cancel_speech（Unity → Go）

ユーザーが認識結果をキャンセルしたことを Go Runtime に通知する。

```json
{
  "type": "cancel_speech",
  "request_id": "a1b2c3d4e5f6a1b2"
}
```

| フィールド | 型 | 必須 | 説明 |
|------------|-----|:--:|------|
| `type` | string | **必須** | `"cancel_speech"` 固定 |
| `request_id` | string | **必須** | キャンセル対象の request_id |

Go Runtime は `cancel_speech` 受信時に以下を実行する:

1. 該当 request_id の VAD バッファを破棄
2. STT リクエストが送信済みの場合はキャンセル（コンテキストキャンセル）
3. 状態を `IDLE` に遷移

### Voice Input シーケンス図

```text
Unity                          Go Runtime                  VAD Service       STT Service
  |                                |                            |                |
  |--- audio_chunk (binary) ------>|                            |                |
  |--- audio_chunk (binary) ------>|--- POST /vad/chunk ------->|                |
  |                                |<--- {event:"speech_start"} |                |
  |<-- vad_event(speech_start) ---|                            |                |
  |--- audio_chunk (binary) ------>|--- POST /vad/chunk ------->|                |
  |                                |<--- {event:"idle"} -------|                |
  |                                |                            |                |
  |  ... (継続的にチャンク送信) ... |                            |                |
  |                                |                            |                |
  |--- audio_chunk (binary) ------>|--- POST /vad/chunk ------->|                |
  |                                |<--- {event:"speech_end"} --|                |
  |<-- vad_event(speech_end) -----|                            |                |
  |                                |                            |                |
  |                                |--- POST /v1/transcribe --------------------->|
  |                                |<--- {text:"認識結果"} ------------------------|
  |                                |                            |                |
  |<-- speech_recognized ---------|                            |                |
```

キャンセル時のシーケンス:

```text
Unity                          Go Runtime
  |                                |
  |<-- speech_recognized ----------|
  |                                |
  |--- cancel_speech ------------->|
  |                                |--- (VAD バッファ破棄、IDLE に遷移)
  |<-- state_change(IDLE) ---------|
```

### HTTP API: Go → Python VAD chunk relay

**メソッド:** `POST`

**エンドポイント:** `http://127.0.0.1:8092/vad/chunk`

**Content-Type:** `application/octet-stream`

**リクエストボディ:** PCM int16 リトルエンディアン、16000Hz モノラル、100ms チャンク（3200 bytes = 1600 samples × 2 bytes）

**成功レスポンス (200):**

```json
{
  "event": "speech_start"
}
```

```json
{
  "event": "idle"
}
```

| フィールド | 型 | 必須 | 説明 |
|------------|-----|:--:|------|
| `event` | string | **必須** | `"speech_start"`, `"speech_end"`, `"idle"` のいずれか |

| `event` 値 | 意味 |
|-----------|------|
| `"speech_start"` | 発話開始を検出 |
| `"speech_end"` | 発話終了を検出 |
| `"idle"` | 発話なし（無音継続中） |

**エラーレスポンス:**

| ステータス | コード | 説明 |
|------------|--------|------|
| 400 | `invalid_chunk` | PCM データのサイズが不正 |
| 500 | `vad_error` | VAD モデルの推論エラー |

エラーレスポンスフォーマット:

```json
{
  "error": {
    "code": "invalid_chunk",
    "message": "chunk must be 3200 bytes for 100ms at 16000Hz mono int16"
  }
}
```

実装箇所（将来）:
- Go: `internal/api/vad_relay.go`
- Python: `local_ai_companion/vad_service.py`

### HTTP API: Python → WinPC faster-whisper STT

**メソッド:** `POST`

**エンドポイント:** `http://192.168.12.107:8093/v1/transcribe`

**Content-Type:** `multipart/form-data`

**リクエストパラメータ:**

| フィールド | 型 | 必須 | 説明 |
|------------|-----|:--:|------|
| `file` | file | **必須** | WAV 音声ファイル（PCM 16bit, 16000Hz, モノラル） |
| `language` | string | 任意 | 言語コード。`"ja"` で日本語指定。未指定時は自動検出 |

**成功レスポンス (200):**

```json
{
  "text": "こんにちは、元気ですか？",
  "duration": 3.2,
  "error": null
}
```

| フィールド | 型 | 説明 |
|------------|-----|------|
| `text` | string | 認識されたテキスト。空文字列は認識失敗 |
| `duration` | float | 音声の長さ（秒） |
| `error` | string \| null | エラー詳細。成功時は `null` |

**エラーレスポンス:**

| ステータス | コード | 説明 |
|------------|--------|------|
| 400 | `invalid_audio` | 音声ファイルが不正または空 |
| 413 | `audio_too_large` | 音声ファイルがサイズ上限（30 秒相当）を超過 |
| 500 | `transcription_error` | Whisper モデルの推論エラー |
| 503 | `model_not_loaded` | Whisper モデルがロードされていない |

エラーレスポンスフォーマット:

```json
{
  "text": "",
  "duration": 0,
  "error": "model not loaded: faster-whisper small model is not available"
}
```

**タイムアウト:** Go Runtime は STT リクエストに 10 秒のタイムアウトを設定する。タイムアウト時はキャンセルされ、`speech_recognized` の `text` は空文字列となる。

実装箇所（将来）:
- Go: `internal/api/stt_relay.go`
- Python: `local_ai_companion/stt_service.py`（WinPC 上で稼働）

### 状態遷移（Voice Input）

音声入力パイプライン中の状態遷移:

```text
IDLE
  ↓ audio_chunk 受信開始
LISTENING (VAD が speech_start を検出)
  ↓ audio_chunk 継続受信
LISTENING
  ↓ VAD が speech_end を検出
THINKING (STT リクエスト送信)
  ↓ STT 応答受信
SPEAKING (認識結果を Unity に表示)
  ↓ 認識結果確定
IDLE
```

キャンセル時は常に `IDLE` に遷移する。

### 注意事項

- audio_chunk の request_id は Unity 側で UUID v4 から生成した 16 バイトのバイナリ（hex エンコードせず raw bytes）を使用する
- VAD は speech_start 検出後、無音が 500ms 継続したら speech_end を発火する
- speech_start から speech_end までの最大発話長は 30 秒（この制限を超えた場合、強制的に speech_end が発火される）
- STT リクエストがタイムアウトまたは失敗した場合、`speech_recognized.text` は空文字列で返され、Unity はエラー UI を表示する
- バイナリフレームと JSON フレームは同一 WebSocket 接続上で混在する。Go Runtime は `websocket.MessageType` でバイナリ/テキストを判別する

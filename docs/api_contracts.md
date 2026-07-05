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

**注意事項:**

- 不正な JSON を受信した場合、そのメッセージは無視され接続は維持される
- `request_id` が空の場合、レスポンスの `request_id` は空文字列になる（`omitempty` によりキー自体が省略される場合もある）
- 全オリジンを許可（`CheckOrigin: true`）
- 同時接続は goroutine-safe に管理される
- 同一 WebSocket 接続への書き込みは接続ごとの mutex で直列化される
- `StateMachine` の `Current` / `Transition` / `Reset` は goroutine-safe
- WebSocket の会話フロー全体は `WebSocketHub` 側でも直列化される

実装箇所: `internal/api/websocket.go` → `HandleWS` / `WebSocketHub`

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

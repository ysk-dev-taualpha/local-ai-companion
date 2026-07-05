# Unity Design

## Purpose

Unity は AI Companion の表示、入力、キャラクター制御を担当するクライアントである。
AI 推論、会話履歴、外部 API 呼び出し、キャンセル制御は Unity では持たない。

基本方針は [architecture.md](architecture.md) の Responsibility Split に従う。

## Scope

### In Scope

- ユーザー入力 UI
- 応答テキスト表示
- キャラクター表示
- 表情、モーション、口パク、字幕の再生
- Runtime との通信
- Runtime から受け取ったイベントの表示反映
- 接続状態、送信状態、エラー状態の UI 表示

### Out of Scope

- LLM 応答生成
- プロンプト生成
- 会話履歴の永続管理
- API キー管理
- 外部 API 直接呼び出し
- Python AI Service への直接依存

## Current Implementation

v0.3 では `unity/v0.3-text-connection/` が Unity プロジェクトとして動作する。

```text
unity/v0.3-text-connection/
  Assets/
    Scenes/
      SampleScene.unity
    Scripts/
      UIManager.cs
      WebSocketClient.cs
  Packages/
  ProjectSettings/
```

`UIManager` は Play 時に UI を自動生成する。
手動で Canvas、InputField、Button、ScrollView を配置しなくても動作する。

`WebSocketClient` は Runtime の `/ws` に接続し、テキストメッセージを送受信する。
WebSocket 接続に失敗した場合は HTTP API へフォールバックできる。

## Runtime Connection

Default endpoints:

```text
WebSocket:     ws://192.168.12.112:8090/ws
HTTP fallback: http://192.168.12.112:8090/v1/conversation
```

Unity は原則として WebSocket を使う。
HTTP fallback は開発時の切り分けと、WebSocket が利用できない構成での一時的な互換経路である。

## Data Flow

### Text Request

```text
User
  ↓ InputField
UIManager
  ↓ MessageJson
WebSocketClient
  ↓ WebSocket /ws
Runtime
```

WebSocket 送信形式:

```json
{
  "type": "text",
  "payload": "こんにちは",
  "request_id": "uuid"
}
```

HTTP fallback 送信形式:

```json
{
  "message": "こんにちは",
  "request_id": "uuid"
}
```

### AI Response

```text
Runtime
  ↓ ai_response
WebSocketClient
  ↓ OnMessageReceived
UIManager
  ↓ AppendResponse
Chat log UI
```

受信形式:

```json
{
  "type": "ai_response",
  "request_id": "uuid",
  "conversation_id": "default",
  "assistant": {
    "text": "受け取りました: こんにちは",
    "emotion": "neutral",
    "motion": "nod",
    "speak_style": "normal",
    "interruptible": true
  }
}
```

v0.3 では `assistant.text` をチャット履歴へ表示する。
`emotion`、`motion`、`speak_style`、`interruptible` は将来のキャラクター制御に使う。

### Audio Response（v0.4）

v0.4 では `ai_response` に続いて `audio` メッセージが送信される。

```text
Runtime
  ↓ ai_response
  ↓ audio
WebSocketClient
  ↓ OnMessageReceived
UIManager / AudioPresenter
  ↓ AppendResponse + PlayAudio
Chat log UI + Audio playback
```

受信形式:

```json
{
  "type": "audio",
  "request_id": "uuid",
  "format": "wav",
  "sample_rate": 24000,
  "channels": 1,
  "data": "base64_encoded_audio_data",
  "is_last": true
}
```

Unity の `AudioPresenter` が Base64 データをデコードし、`AudioSource` で再生する。
`interruptible: true` の場合、ユーザー入力で再生を中断できる。

## Component Design

### UIManager

Responsibilities:

- UI の自動生成
- 入力欄と送信ボタンの制御
- 送信中の UI 無効化
- ユーザー入力のチャット履歴表示
- Runtime 応答のチャット履歴表示
- 接続状態の表示
- WebSocket と HTTP fallback の切り替え
- Unity メインスレッド上での UI 更新

UIManager は AI 応答の意味解釈を最小限に留める。
表示に必要な `assistant.text` の抽出のみ行い、会話制御や状態遷移の正当性判断は Runtime 側に任せる。

### WebSocketClient

Responsibilities:

- WebSocket 接続
- JSON テキスト送信
- JSON テキスト受信
- フレーム断片の結合
- 切断時の自動再接続
- 接続、切断、受信、エラーイベントの通知

WebSocketClient は Unity UI を直接操作しない。
UI 更新は `UIManager` がイベント購読を通じて行う。

### UnityMainThreadDispatcher

Responsibilities:

- WebSocket コールバックからの UI 更新を Unity メインスレッドへ寄せる
- すでにメインスレッド上にいる場合は即時実行する

Unity の UI API はメインスレッド上で操作する。
通信処理の実行スレッドに依存しないため、UI 更新は dispatcher を経由する。

## UI Design

v0.3 の UI は検証用の最小チャット UI とする。

```text
TitleText
StatusText
ScrollView / ResponseText
InputField
SendButton
```

UI は専用 Canvas を `Screen Space - Overlay` で生成する。
既存 Canvas やシーン内オブジェクトの描画順に隠れないよう、専用 Canvas の sorting order を高くする。

Font は Unity 2022 の組み込みフォント変更に対応するため `LegacyRuntime.ttf` を使用する。

## State Handling

Unity 側で扱う状態は UI 表示用に限定する。

```text
接続中
接続済み
切断
送信中
HTTP 接続
HTTP エラー
Runtime state_change
```

Runtime から `state_change` を受け取った場合は StatusText に表示する。
ただし Unity は状態遷移の妥当性を判断しない。

## Error Handling

### WebSocket Error

- エラーを Runtime 接続失敗として扱う
- HTTP fallback URL が設定されている場合は fallback mode に切り替える
- 再接続は `WebSocketClient` が 3 秒後に試行する

### HTTP Error

- response code、error、URL を画面と Unity Console に表示する
- 入力欄と送信ボタンを再有効化する

### Unknown Message

未対応の JSON は捨てずにチャットログへ表示する。
これにより Runtime 側のプロトコル変更を Unity 上で観測できる。

## Threading Policy

- Unity UI の更新は Unity メインスレッドで行う
- WebSocket 受信処理は UI を直接触らない
- `UnityMainThreadDispatcher.Enqueue` を通して UI 更新する
- `Enqueue` 呼び出し時点でメインスレッドなら即時実行してよい

## Extension Plan

### Character Presenter

将来的に `assistant` の表示制御を `CharacterPresenter` に分離する。

```text
assistant.text       -> SubtitlePresenter
assistant.emotion    -> EmotionController
assistant.motion     -> MotionController
assistant.speak_style -> VoiceStyleController / TTS request
assistant.interruptible -> Interrupt UI / playback policy
```

### Event Router

Runtime から受け取るイベントが増えた段階で、`UIManager` から message routing を分離する。

```text
WebSocketClient
  ↓ raw JSON
RuntimeEventRouter
  ↓ typed event
UIManager / CharacterPresenter / AudioPresenter
```

### Audio and Lip Sync

v0.4 では TTS 音声再生が追加される。Unity は `audio` メッセージの受信と再生、口パクの同期を担当する。
TTS 生成そのものは Python AI Service、配送は Go Runtime の責務とする。
詳細は [api_contracts.md](api_contracts.md) の TTS セクションを参照。

## Design Rules

- Unity は AI 処理を持たない
- Unity は Runtime から受け取ったイベントを表示、再生する
- Runtime との境界は JSON に限定する
- `request_id` を送信ごとに発行し、ログと応答の対応を追えるようにする
- UI 更新はメインスレッドで行う
- 手動シーン構築を必須にしない
- 生成物の `Library/`、`Temp/`、`Logs/`、`UserSettings/` はコミットしない

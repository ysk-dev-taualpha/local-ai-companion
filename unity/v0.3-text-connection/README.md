# Unity v0.3/v0.4 Text and Audio Client

AI Companion の Unity クライアントです。Go Runtime（X1C6, `192.168.12.112:8090`）と WebSocket で通信し、テキスト応答と v0.4 の音声メッセージを扱います。WebSocket に接続できない場合は HTTP API へフォールバックします。

## 動作環境

- Unity: 2022 LTS
- PC: WinPC (`192.168.12.107`, RTX2080S)
- 依存: 外部パッケージ不要
- WebSocket: `ws://192.168.12.112:8090/ws`
- HTTP fallback: `http://192.168.12.112:8090/v1/conversation`

## 構成

```text
unity/v0.3-text-connection/
├── README.md
├── Packages/
├── ProjectSettings/
└── Assets/
    ├── Scenes/
    └── Scripts/
        ├── WebSocketClient.cs
        └── UIManager.cs
```

`UIManager` は Play 時に UI と `AudioSource` を自動生成します。既存のシーンに手動で Canvas や EventSystem を置く必要はありません。

## WebSocket

### 送信

```json
{
  "type": "text",
  "payload": "こんにちは",
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 受信

状態遷移:

```json
{"type": "state_change", "state": "SPEAKING"}
```

AI応答:

```json
{
  "type": "ai_response",
  "request_id": "550e8400-...",
  "conversation_id": "default",
  "assistant": {
    "text": "こんにちは",
    "emotion": "happy",
    "motion": "wave",
    "speak_style": "normal",
    "interruptible": true
  }
}
```

音声:

```json
{
  "type": "audio",
  "request_id": "550e8400-...",
  "data": "<base64-wav>"
}
```

`data`, `audio`, `audio_base64` のいずれかに Base64 エンコードされた WAV を入れると再生します。PCM 8/16/24/32bit と IEEE float 32bit WAV をサポートします。

エラー:

```json
{"type": "error", "request_id": "...", "error": "python service error"}
```

## 音声再生

- `audio` メッセージを Base64 decode して `AudioClip` に変換します。
- `AudioSource` は `UIManager` が自動で追加します。
- 複数の `audio` メッセージはキューに入り、順番に再生されます。
- `state_change: SPEAKING` で再生キューを開始します。
- `state_change: IDLE` で現在の再生とキューを停止します。

## 確認手順

1. Runtime が `192.168.12.112:8090` で起動していることを確認します。
2. Unity Hub で `unity/v0.3-text-connection/` を開きます。
3. Play します。
4. ステータスが接続済みになることを確認します。
5. テキストを送信し、`You:` と `AI:` が表示されることを確認します。
6. v0.4 TTS 実装が有効な場合、`audio` メッセージ受信後に音声が再生されることを確認します。

## 備考

- 切断時は 3 秒後に自動再接続します。
- 応答ログは最大 200 行です。
- Unity 2022 の組み込みフォント変更に合わせて `LegacyRuntime.ttf` を使用します。

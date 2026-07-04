# Unity v0.3 — テキスト接続クライアント

AI Companion の Unity 側テキストチャットクライアントです。
Runtime（X1C6, 192.168.12.112:8090）と WebSocket で通信します。
WebSocket に接続できない場合は HTTP API へフォールバックします。

## 動作環境

- **Unity**: 2022 LTS 推奨（.NET Standard 2.1）
- **PC**: WinPC (192.168.12.107, RTX2080S)
- **依存**: 外部パッケージ不要（`System.Net.WebSockets` 標準利用）

## プロジェクト構成

```
unity/v0.3-text-connection/
├── README.md
├── Packages/
├── ProjectSettings/
└── Assets/
    ├── Scenes/
    └── Scripts/
        ├── WebSocketClient.cs   # WebSocket クライアント（自動再接続付き）
        └── UIManager.cs         # UI 自動生成 + UI 制御 + メインディスパッチャ
```

## セットアップ手順

### 1. Unity プロジェクトを開く

Unity Hub で `unity/v0.3-text-connection/` をプロジェクトとして開いてください。
`Packages/`、`ProjectSettings/`、シーンファイルは同梱済みです。

### 2. シーン構築

`UIManager` はシーン起動時に自動生成されます。
Canvas、TitleText、StatusText、ScrollView、InputField、SendButton、EventSystem は手動配置不要です。

既存 Canvas の描画順に影響されないよう、専用 Canvas を前面に生成します。

### 3. 接続先

デフォルト接続先:

- WebSocket: `ws://192.168.12.112:8090/ws`
- HTTP fallback: `http://192.168.12.112:8090/v1/conversation`

別環境で動かす場合は `UIManager` の `_wsUrl` と `_httpFallbackUrl` を変更してください。

### 4. UnityMainThreadDispatcher

`UIManager.cs` に `UnityMainThreadDispatcher` が同梱されています。
シーン起動時に自動生成されるため、手動配置は不要です。

## WebSocket プロトコル

### 送信

```json
{
  "type": "text",
  "payload": "こんにちは",
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 受信

**状態遷移通知:**
```json
{"type": "state_change", "state": "LISTENING"}
```

**AI 応答:**
```json
{
  "type": "ai_response",
  "request_id": "550e8400-...",
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

**エラー:**
```json
{"type": "error", "request_id": "...", "error": "python service error"}
```

## 動作確認

1. Runtime が X1C6:8090 で起動していることを確認
2. Unity で Play ボタンを押す
3. ステータスが「接続済み」になることを確認
4. テキストを入力して送信 → チャット履歴に `You:` と `AI:` が表示されることを確認

## 備考

- 切断時は 3 秒後に自動再接続します
- 送信中はボタンと入力欄が無効化されます
- 応答ログは最大 200 行まで保持されます
- Unity 2022 の組み込みフォント変更に対応するため `LegacyRuntime.ttf` を使用します

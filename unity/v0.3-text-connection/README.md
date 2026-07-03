# Unity v0.3 — テキスト接続クライアント

AI Companion の Unity 側テキストチャットクライアントです。
Go Runtime（X1C6, 192.168.12.112:8080）と WebSocket で通信します。

## 動作環境

- **Unity**: 2022 LTS 推奨（.NET Standard 2.1）
- **PC**: WinPC (192.168.12.107, RTX2080S)
- **依存**: 外部パッケージ不要（`System.Net.WebSockets` 標準利用）

## プロジェクト構成

```
unity/v0.3-text-connection/
├── README.md
└── Assets/
    └── Scripts/
        ├── WebSocketClient.cs   # WebSocket クライアント（自動再接続付き）
        └── UIManager.cs         # UI 制御 + メインディスパッチャ
```

## セットアップ手順

### 1. Unity プロジェクト作成

Unity Hub で新規 2D プロジェクトを作成し、
`Assets/Scripts/` に上記2ファイルをコピーしてください。

### 2. シーン構築

以下の GameObjects をシーンに配置します：

| GameObject | Component | 設定 |
|---|---|---|
| Canvas | Canvas | Render Mode: Screen Space - Overlay |
| ├ TitleText | Text | Text: "AI Companion v0.3", FontSize: 28 |
| ├ StatusText | Text | Text: "接続中...", FontSize: 14, Color: gray |
| ├ ScrollView | ScrollRect + Mask | 応答表示エリア |
| │ └ ResponseText | Text | UIManager の `_responseText` にアタッチ |
| ├ InputField | InputField | Placeholder: "メッセージを入力..." |
| └ SendButton | Button | Text: "送信" |
| EventSystem | EventSystem + StandaloneInputModule | 自動生成 |

### 3. UIManager のアタッチ

Canvas に `UIManager` をアタッチし、Inspector で以下の参照を設定：

- `_inputField` → InputField
- `_sendButton` → SendButton
- `_responseText` → ScrollView 内の ResponseText
- `_scrollRect` → ScrollView
- `_statusText` → StatusText（任意）
- `_titleText` → TitleText（任意）

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

1. Go Runtime が X1C6:8080 で起動していることを確認
2. Unity で Play ボタンを押す
3. ステータスが「接続済み」になることを確認
4. テキストを入力して送信 → AI の応答が表示されることを確認

## 備考

- 切断時は 3 秒後に自動再接続します
- 送信中はボタンと入力欄が無効化されます
- 応答ログは最大 200 行まで保持されます

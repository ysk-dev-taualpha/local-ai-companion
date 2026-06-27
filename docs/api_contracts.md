# API Contracts

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

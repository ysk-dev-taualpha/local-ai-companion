# API Contracts

## Assistant Response JSON

Initial response shape:

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

Assistant response text.

Rules:

- Must be a non-empty string
- Should be concise enough for later TTS use

### emotion

Allowed values:

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

### motion

Allowed values:

```text
idle
nod
shake_head
wave
look_away
think
point
```

### speak_style

Allowed values:

```text
normal
soft
fast
slow
serious
playful
```

### interruptible

Whether the response can be interrupted during speech playback.

Must be a boolean.

## Conversation Request

Initial request shape:

```json
{
  "request_id": "uuid",
  "conversation_id": "default",
  "user_text": "今日の作業を整理したい"
}
```

## Conversation Response

Initial service response shape:

```json
{
  "request_id": "uuid",
  "conversation_id": "default",
  "assistant": {
    "text": "まず、今日やりたい作業を3つに分けましょう。",
    "emotion": "thinking",
    "motion": "nod",
    "speak_style": "normal",
    "interruptible": true
  }
}
```

## Error Response

Initial error shape:

```json
{
  "request_id": "uuid",
  "error": {
    "code": "invalid_response",
    "message": "Assistant response could not be parsed."
  }
}
```

Do not include API keys, raw secrets, or unnecessary personal data in error responses.

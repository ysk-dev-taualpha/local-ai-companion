Return only one valid JSON object with this exact shape:

```json
{
  "text": "short natural response text",
  "emotion": "neutral",
  "motion": "idle",
  "speak_style": "normal",
  "interruptible": true
}
```

Allowed values:

- emotion: neutral, happy, sad, thinking, surprised, angry, sleepy, confident
- motion: idle, nod, shake_head, wave, look_away, think, point
- speak_style: normal, soft, fast, slow, serious, playful

Rules:

- Do not wrap the JSON in Markdown.
- Do not add commentary before or after the JSON.
- Keep `text` concise.
- `interruptible` must be a boolean, not a string.

応答は次のJSONスキーマに従ってください:

```json
{
  "text": "返答本文（500文字以内）",
  "emotion": "neutral|happy|sad|thinking|surprised|angry|sleepy|confident",
  "motion": "idle|nod|shake_head|wave|look_away|think|point",
  "speak_style": "normal|soft|fast|slow|serious|playful",
  "interruptible": true|false
}
```

textは音声合成に適した自然な長さに収めてください。
余計な説明文は含めず、JSONオブジェクトのみを返してください。

# Decision Log

This file records architectural decisions that should be visible to the human developer, Codex, and Hermes.

## 2026-06-27: Stage-based Development

Decision:

Build one stage to a usable level before moving to the next stage.

Reason:

The project depends heavily on interaction quality. Thin implementations across STT, LLM, TTS, character display, memory, and autonomy would make the whole system hard to evaluate.

## 2026-06-27: Initial Target Is Conversation Core

Decision:

The first deliverable is a text-input conversation core that returns stable JSON.

Reason:

Audio, character control, memory, and autonomous behavior can all depend on this contract later.

## 2026-06-27: Responsibility Split

Decision:

- Unity owns display, input, character control, subtitles, lip sync, and animation.
- Go owns the long-running runtime, communication, concurrency, cancellation, external API gateway behavior, logging, and process supervision.
- Python owns LLM, TTS, RAG, prompt logic, VAD, and ML-oriented processing.

Reason:

Each technology is used where it is strongest. Go is a good fit for goroutine-based event orchestration and cancellation. Python is a good fit for AI and ML libraries. Unity is a good fit for character presentation.

## 2026-06-27: LLM API Communication Placement

Decision:

Python may call LLM APIs directly during early experimentation. The target architecture should move external API keys and external API communication into Go Runtime.

Reason:

Python gives faster prompt and LLM experimentation early. Go is better suited for long-term API key management, timeout, retry, cancellation, streaming, rate limiting, and centralized logging.

## 2026-06-27: Early Communication Protocol

Decision:

Use HTTP + JSON for early service boundaries. Introduce WebSocket where bidirectional streaming becomes necessary.

Reason:

HTTP + JSON is easy to debug and sufficient for early request-response flows. WebSocket is better reserved for Unity-Go event streams and streaming responses.

## 2026-06-28: LLM Provider Uses OpenAI-Compatible API

Decision:

The Python conversation core uses an OpenAI-compatible chat completions interface for both hosted and local LLMs. Local LLMs should be exposed through an OpenAI-compatible HTTP server instead of being embedded directly into the application process.

Reason:

This keeps the provider boundary simple and avoids separate local-model integration paths during v0.1. Go can later own the same external API boundary when the runtime layer becomes the main communication pipe.

## 2026-06-28: Logging Defaults Avoid User Text and Raw Responses

Decision:

Conversation JSONL logging is enabled only when configured, and debug-sensitive fields are opt-in. The default log output excludes `user_text` and `raw_response`; these can be enabled explicitly through `logging.include_user_text` and `logging.include_raw_response`.

Reason:

Reliable personal-information detection is difficult and should not be treated as a v0.1 safety mechanism. Safe defaults should reduce accidental capture of user content while keeping operational fields such as request ID, provider, latency, validity, and fallback errors available.

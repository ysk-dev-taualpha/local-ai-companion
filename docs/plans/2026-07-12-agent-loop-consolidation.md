# Agent Loop 統合計画

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** 3系統に分裂したLLM呼び出しパスを `agent.Loop` 1本に統合し、ツール呼び出し（web_search, web_fetch, audio_control, set_state）が正しく動作するようにする。

**Architecture:** WebSocket → `agentLoop.Run()` → Ollama `/api/chat`（ネイティブ形式）→ tool.ExecutorService → 結果を履歴に追加 → 再帰呼び出し。`websocket.go` の独自 `callOllama` / `webSearch` / ハードコードtools は全削除。

**Tech Stack:** Go, Ollama `/api/chat` (native), modernc.org/sqlite (memory.Store)

---

## 背景

現在、会話処理は以下の3系統に分裂している：

| 系統 | 場所 | 実際に使われてる？ |
|------|------|-----------------|
| `agent.Loop` | `internal/agent/loop.go` | ❌ 作っただけで未使用 |
| `callOllama` | `internal/api/websocket.go:282` | ✅ 実際の会話で使われてる |
| Python `ConversationCore` | `src/local_ai_companion/conversation.py` | ✅ `/v1/conversation` で使用 |

`websocket.go` の `callOllama` は `agentLoop` の存在確認だけして、実際には独自の簡易実装で LLM を呼んでいる。この独自実装には以下のバグがある：

1. `content == ""` で早期 return → ツール呼び出しが握り潰される
2. ツール引数（JSON文字列）を `json.Unmarshal` せずに検索クエリとして渡す
3. assistant の `tool_calls` を履歴に保存せず破棄
4. ツール結果に `tool_name` を付けない
5. Ollama に OpenAI互換 `/v1/chat/completions` エンドポイントを使っている

## 修正対象ファイル

| ファイル | 変更内容 |
|----------|---------|
| `internal/agent/ollama.go` | ツール定義を `{"type":"function","function":{...}}` ラッパー形式に修正。ツール結果を `tool_name` ベースに |
| `internal/agent/loop.go` | memory.Store 統合、履歴読み込み/保存対応 |
| `internal/api/websocket.go` | `callOllama`, `webSearch`, `ollamaRequest`, ハードコードtools 削除。`agentLoop.Run()` 呼び出しに差し替え |
| `internal/api/voice.go` | `HandleVoiceTextAgent` も `agentLoop.Run()` に差し替え |
| `internal/memory/store.go` | ターン数ベース管理に修正（任意：既存データ互換性のため後回し可） |
| `cmd/local-ai-runtime/main.go` | agentLoop に memoryStore 参照を渡す |
| `internal/agent/loop_test.go` | 新規：ツール呼び出しの単体テスト |
| `internal/api/websocket_test.go` | 新規：統合後の結合テスト |

## 完了条件

- [ ] WebSocket テキストメッセージが `agentLoop.Run()` 経由で処理される
- [ ] `web_search` ツールが正しいクエリで検索を実行する
- [ ] ツール結果を踏まえた2回目のLLM呼び出しが動作する
- [ ] 会話履歴が memory.Store に正しく保存される
- [ ] websocket.go から独自LLM実装（callOllama, webSearch, ollamaRequest, tools JSON）が削除されている
- [ ] 既存のテストが全てパスする

---

## Issue 分割

依存順に6つの Issue に分割：

### Issue 1: agent.Loop の Ollama ネイティブAPI形式を修正
**依存:** なし
**内容:** `ollama.go` のツール定義をラッパー形式に、ツール結果形式を `tool_name` ベースに修正

### Issue 2: agent.Loop を memory.Store と統合
**依存:** Issue 1
**内容:** `loop.go` が履歴を memory.Store から読み込み、保存できるようにする

### Issue 3: WebSocket を agentLoop.Run() に差し替え
**依存:** Issue 2
**内容:** `handleTextMessageAgent` と `HandleVoiceTextAgent` が `agentLoop.Run()` を呼ぶように。独自実装を削除

### Issue 4: ツール引数のJSONパースとツール結果の修正
**依存:** Issue 3
**内容:** tool.ExecutorService が受け取る引数を正しくパース。ツール結果に `tool_name` を付与

### Issue 5: 履歴をターン数ベース管理に修正
**依存:** Issue 3
**内容:** memory.Store の maxTurns を「完了会話ターン数」ベースに。ツール呼び出し中間メッセージを履歴上限から除外

### Issue 6: テスト追加
**依存:** Issue 5
**内容:** ツール呼び出しの単体テスト + WebSocket 結合テスト

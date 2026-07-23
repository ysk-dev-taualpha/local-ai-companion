# 作業日報

## 2026-07-14 〜 07-15: Agent Loop 統合 + Codex レビュー対応

### 完了
- [x] Agent Loop 統合: 3系統のLLM呼び出しを1本化 (PR #156)
  - websocket.go から callOllama/webSearch/ollamaRequest 他 172行削除
  - Ollama ネイティブAPI形式に統一 (type/function ラッパー)
  - tool_calls 先判定、tool_name 付与
  - memory.Store 統合、ConversationTurn でターン数ベース管理
  - モック Ollama によるツール呼び出しテスト 4件追加
  - CI go-version 1.21→1.25 修正
- [x] Codex レビュー #1-3 修正 (PR #157)
  - セッションID永続化（query param で引き継ぎ）
  - TTS状態管理（早すぎる IDLE 遷移を削除）
  - ツールテスト改善（JSON object arguments, tool_name/tool_calls 検証）
- [x] cron スクリプト修正（approve=Codex, auto-merge=Hermes トークン分離）
- [x] ワークフロー整理（main 直 push 禁止、branch→PR→review→merge）

### 進行中
- [ ] Codex レビュー #4: 排他制御をセッション単位に

### 未着手
- [ ] Codex レビュー #5: 履歴保存トランザクション化
- [ ] Codex レビュー #6: web_fetch SSRF 対策
- [ ] Codex レビュー #7: 細部（MaxToolLoops, ツール一覧ソート）

### v0.5 残作業
- [ ] TBD: docs: v0.5 STT 担当を Go に変更
- [ ] TBD: fix: STT multipart フィールド名を `file` に統一
- [ ] TBD: fix: STT レスポンスに duration フィールド追加
- [ ] TBD: refactor: Python STT クライアント削除
- [ ] TBD: docs: API 契約の不整合を修正
- [ ] #114: speech_end → STT → LLM → TTS フロー統合
- [ ] #115: Unity 認識テキスト表示 + 送信前キャンセル

### 環境メモ
- ThinkPad X1C6: Go 1.26.4, Hermes CLI
- WinPC: Ollama g4v100, ComfyUI, faster-whisper :8093
- VOICEVOX: 127.0.0.1:50021, speaker=3 (ずんだもん)
- cron: codex-auto-work (10分間隔, approve+auto-merge)

# Local AI Companion

ローカル環境で動作する常駐AIアシスタントを、段階的に開発するプロジェクトです。

## Start Here

- Original plan: [local_ai_assistant_development_plan.md](local_ai_assistant_development_plan.md)
- Roadmap: [docs/roadmap.md](docs/roadmap.md)
- Architecture: [docs/architecture.md](docs/architecture.md)
- WBS: [docs/wbs.md](docs/wbs.md)
- API contracts: [docs/api_contracts.md](docs/api_contracts.md)
- Decision log: [docs/decisions.md](docs/decisions.md)
- Collaboration guide: [docs/collaboration.md](docs/collaboration.md)
- Git workflow: [docs/git_workflow.md](docs/git_workflow.md)
- Project management: [docs/project_management.md](docs/project_management.md)

## Current Target

v0.1: Python Conversation Core

テキスト入力に対して、安定した assistant response JSON を返す会話コアを作ります。

```json
{
  "text": "今日は何から始めますか？",
  "emotion": "neutral",
  "motion": "idle",
  "speak_style": "normal",
  "interruptible": true
}
```

## セットアップ

Python 3.x のみが必要です。追加の依存パッケージはありません。

```bash
# リポジトリのクローン
git clone https://github.com/ysk-dev-taualpha/local-ai-companion.git
cd local-ai-companion
```

## 設定ファイル

`config.example.json` をコピーしてカスタマイズします。

```bash
cp config.example.json config.json
```

設定項目の意味:

```jsonc
{
  "conversation": {
    "default_conversation_id": "default", // 会話セッションのデフォルトID
    "max_history_turns": 12              // LLMに渡す過去ターンの上限
  },
  "llm": {
    "provider": "mock"                    // 使用するLLMプロバイダ（現在は "mock" のみ）
  }
}
```

設定ファイルを指定しない場合は、上記のデフォルト値が使われます。

## 実行方法

### シングルメッセージ

```bash
# Linux / macOS
PYTHONPATH=./src python3 -m local_ai_companion --message "こんにちは"

# --config で設定ファイルを指定可能
PYTHONPATH=./src python3 -m local_ai_companion --config config.json --message "こんにちは"

# Windows (PowerShell)
$env:PYTHONPATH=(Resolve-Path .\src)
python -m local_ai_companion --message "こんにちは"
```

### 対話モード（REPL）

```bash
# Linux / macOS
PYTHONPATH=./src python3 -m local_ai_companion

# `/exit` または `/quit` で終了、Ctrl+D でも終了できます
```

出力例:

```json
{
  "request_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "conversation_id": "default",
  "assistant": {
    "text": "受け取りました: こんにちは",
    "emotion": "neutral",
    "motion": "nod",
    "speak_style": "normal",
    "interruptible": true
  }
}
```

## テスト実行

```bash
# Linux / macOS
PYTHONPATH=./src python3 -m unittest discover -s tests -v

# Windows (PowerShell)
$env:PYTHONPATH=(Resolve-Path .\src)
python -m unittest discover -s tests -v
```

## Participants

- Human developer: product direction and final acceptance
- Codex: implementation and review agent
- Hermes: implementation and review agent

All participants should follow [docs/collaboration.md](docs/collaboration.md).

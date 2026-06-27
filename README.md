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

## Run v0.1 CLI

From the project root:

```powershell
$env:PYTHONPATH=(Resolve-Path .\src)
python -m local_ai_companion --message "今日の作業を整理したい"
```

Run tests:

```powershell
$env:PYTHONPATH=(Resolve-Path .\src)
python -m unittest discover -s tests
```

## Participants

- Human developer: product direction and final acceptance
- Codex: implementation and review agent
- Hermes: implementation and review agent

All participants should follow [docs/collaboration.md](docs/collaboration.md).

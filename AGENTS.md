# AGENTS.md — local-ai-companion エージェント共通ルール

## プロジェクト概要

local-ai-companion は自律的な AI 開発アシスタントプロジェクトです。
Hermes、Codex、Gemma の3エージェントが GitHub 上で連携し、
人間の介在を最小化した自動開発パイプラインを実現します。

- リポジトリ: `ysk-dev-taualpha/local-ai-companion`
- 作業ディレクトリ: `/home/yplic/workspace/local-ai-companion`

## アーキテクチャ

```
Issues ──→ webhook ──→ Hermes (実装)
PRs    ──→ cron    ──→ Codex  (レビュー・設計・コーディング)
@gemma ──→ webhook ──→ Gemma  (機械的チェック)
CI     ──→ webhook ──→ auto-merge
```

- Hermes: GitHub webhook で issues/issue_comment/check_suite イベントに反応
- Codex: cron 10分おき。`codex exec resume --last` でコンテキスト維持
- Gemma: `@gemma` コメントで発火。Ollama (g4v100) で軽量チェック

## 技術スタック

- **言語**: Python (AI Service), Go (Runtime)
- **テスト**: `python -m unittest`, `go test`
- **DB**: modernc.org/sqlite (no CGO), in-memory for tests
- **CI/CD**: GitHub Actions
- **エージェント**: Hermes (CLI), Codex (CLI), Gemma (Ollama)

## ブランチ命名

```
codex/issue-N-description    # Codexの作業
hermes/issue-N-description   # Hermesの作業
gemma/issue-N-description    # Gemmaの作業
feature/xxx                  # 人間主導の作業のみ
docs/xxx                     # ドキュメントのみ
fix/xxx                      # バグ修正
```

## コミットメッセージ

```
area: short imperative summary
```

例:
```
docs: add AGENTS.md
core: validate assistant response schema
runtime: add request timeout handling
```

## PR のルール

- タイトル・本文は日本語
- 必ず `Closes #N` または `Refs #N` でIssueを参照
- `ysk0518` に review request を送る
- 自分のPRを自分でapproveしない
- mergeは原則人間が判断。自動マージ条件（approve + test pass + resolved）を満たせば委譲可

## 相互レビュー

- Codex実装 → Hermes/Human がレビュー
- Hermes実装 → Codex/Human がレビュー
- Gemma実装 → Hermes/Codex がレビュー
- レビュー優先順位: バグ > 契約不一致 > テスト不足 > エラーハンドリング > 並行性 > セキュリティ

## 各エージェントの役割

### Hermes
- 実装、レビュー、設計提案が可能
- webhook で Issue 作成を検知 → 自動実装
- 設計判断が必要な場合はコメントで提案し、勝手にアーキテクチャ変更しない
- 詳細スキル: `hermes-github-implementation-workflow`

### Codex
- 設計、レビュー、コーディング全般
- cron で定期的に GitHub をチェック
- `codex exec resume --last` でコンテキスト維持
- コード変更は `codex/` ブランチで PR 作成

### Gemma
- 機械的チェックのみ: typo, import漏れ, 明白なバグ
- ドキュメントの網羅性チェック
- アーキテクチャ判断・パフォーマンス分析・複雑ロジックレビューは禁止
- `@gemma` または `gemma check` コメントでのみ発火
- モデル: Gemma 4 26B (ollama, g4v100, ctx 65536)

## 禁止事項（全エージェント共通）

- main ブランチへの直接 push
- 他人のブランチ履歴の無断書き換え
- タスク外の広範囲リファクタリング
- シークレット・APIキー・.env のコミット
- 生成ログ・ビルド成果物のコミット
- 未検証のコードの PR 化

## 参照ドキュメント

- `docs/collaboration.md` — エージェント間連携の詳細
- `docs/project_management.md` — Issue管理とワークフロー
- `docs/git_workflow.md` — ブランチ戦略とマージポリシー
- `docs/architecture.md` — システム設計
- `docs/decisions.md` — アーキテクチャ決定記録
- `docs/wbs.md` — タスク分解
- `docs/roadmap.md` — マイルストーン

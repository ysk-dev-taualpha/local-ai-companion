# Project Management

## Purpose

GitHub Issues を、このプロジェクトの Backlog として使います。

開発フローは GitHub Webhook によるイベント駆動で自動化されています。Issue 作成 → 実装 → PR → レビュー → マージ のサイクルがエージェントによって自律的に進行します。

## Backlog Structure

進行中の作業は GitHub Issues を正とします。

- 親Issue: マイルストーンまたはエピック
- 子Issue: 実装、ドキュメント、検証などの具体的な作業
- Pull Request: 1つのIssue、または強く関連する小さなIssue群に対する変更提案
- WBS文書: 計画の参照元。日々の作業台帳はIssue側に寄せる

例:

```text
#3 v0.1: Python会話コアを完成させる
  #4 v0.1: 応答JSONスキーマを厳密化する
  #5 v0.1: LLMプロバイダ差し替え境界を整える
  #6 v0.1: プロンプト管理の初期構成を作る
```

## Event-Driven Workflow

GitHub Webhook で以下のイベントが発火し、各エージェントが自律的に動作します。

| イベント | 動作 |
|---|---|
| Issue 作成 | 実装開始（ブランチ作成 → コード変更 → PR作成） |
| PR open / sync | Codex によるコードレビュー |
| PR review 投稿 | レビュー内容に応じて修正・返信 |
| `@codex` コメント | Codex が設計・コーディングを実行 |
| `@gemma` コメント | Gemma が機械的チェックを実行 |
| CI 完了 | 条件を満たせば自動マージ |

人間の介在が必要な場合は、PR 上で `@ysk0518` によるレビューやマージ判断を待ちます。

## Issue Types

Issue type は次の基準で使います。

- Feature: ユーザーに見える機能、またはマイルストーン級の成果物
- Task: 実装、ドキュメント、検証、環境整備などの具体的作業
- Bug: 不具合、退行、期待と異なる挙動

迷ったら Task にします。

## Issue Fields

軽量な計画管理として、リポジトリの Issue field を使います。

### Priority

- Urgent: 現在の作業を止める、または重大な破損につながる
- High: 現在のマイルストーン達成に必要
- Medium: 現在のマイルストーンに有用だが、直ちにブロックしない
- Low: 近い開発に影響せず後回しにできる

### Effort

- High: 複数ファイル、設計相談、または大きめの検証が必要
- Medium: テストやドキュメントを含む、まとまった実装
- Low: 小さな修正、狭いドキュメント更新

### Dates

Start date と Target date は、日付管理が本当に必要な作業にだけ使います。

欄を埋めるためだけの日付は入れません。

## Task Lifecycle

通常の開発では次の状態で考えます。

1. Backlog: Issueはあるが、まだ着手可能とは限らない
2. Ready: スコープと完了条件が明確
3. In Progress: ブランチがあり、作業中
4. Review: Pull Requestがopen
5. Done: Pull Requestがmergeされ、Issueがclose

GitHub Projects のボードを作るまでは、状態は Issue コメントや紐付いた Pull Request で表現します。

## Branch and Pull Request Linkage

意味のある作業ごとにブランチを作ります。

推奨ブランチ名:

```text
codex/issue-4-json-schema
hermes/issue-5-provider-interface
gemma/issue-8-typo-fix
fix/issue-7-json-fallback
```

Codex、Hermes、Gemma が主体の作業では、必ず `codex/`、`hermes/`、`gemma/` の prefix を使います。

`feature/` は、人間主導の作業、または担当 agent が未確定の共有作業だけに使います。

Pull Request 本文には Issue 番号を入れます。

Issue を完全に完了する場合:

```text
Closes #4
Fixes #7
```

一部だけ関係する場合:

```text
Refs #4
```

## Review Assignment

標準のレビュー割当は次の通りです。

- Codex 実装: Hermes または人間の開発者がレビュー
- Hermes 実装: Codex または人間の開発者がレビュー
- Gemma 実装: Hermes または Codex がレビュー
- 人間の実装: 必要に応じて Codex または Hermes がレビュー

PR 作成者は自分の PR を approve しません。

最終的な受け入れと merge 判断は、明示的に委譲されない限り人間の開発者が持ちます。
自動マージは、以下の条件をすべて満たした場合に委譲されます:
- 1件以上の approve review
- CI テスト通過
- レビューコメントがすべて解決済み

## Issue Template for Agents

エージェント向け Issue には、できるだけ次の情報を入れます。

```md
## 実装範囲

- 変更してよいファイル:
- 変更してはいけないファイル:
- 追加してよいテスト:
- 完了条件:

## 参照する設計

- docs/architecture.md
- docs/api_contracts.md
- docs/decisions.md
```

曖昧な表現を避けます。

- 避ける: いい感じに整える
- 避ける: 必要なら設計も見直す
- 避ける: 関連しそうなところも直す
- 推奨: `src/local_ai_companion/schema.py` の検証条件だけを変更する
- 推奨: `tests/test_schema.py` に不正値のテストを追加する
- 推奨: `docs/api_contracts.md` の該当フィールド説明だけを更新する

設計判断が必要な場合は、Issue コメントまたは別 Issue で提案します。

## Manual Working Rule

Webhook が動いていない場合や、人間が手動で作業する場合:

作業開始前:

1. 現在のマイルストーン親 Issue を確認する。
2. Ready 状態の Issue を1つ選ぶ。
3. 完了条件を確認する。
4. 最新の `main` からブランチを作る。

PR 作成前:

1. 最小限必要な検証を実行する。
2. 影響するドキュメントを更新する。
3. PR 本文から Issue をリンクする。
4. PR タイトルと説明は日本語を基本にする。
5. PR 作成時に `ysk0518` へ review request を送る。

merge 後:

1. 完了した Issue を close する。
2. 親 Issue に未完了の子 Issue が残っているか確認する。
3. 不要になった作業ブランチを削除する。

## GitHub Projects

Issue 数が増えたら GitHub Projects を追加します。

推奨カラム:

- Backlog
- Ready
- In Progress
- Review
- Done

現時点のパイロット段階では、Issue と親子 Issue の関係だけで十分です。ボード表示が必要になった時点で Projects を追加します。

# Project Management

## Purpose

GitHub Issuesを、このプロジェクトのBacklogとして使います。

パイロット段階では重い外部ツールを増やさず、人間の開発者、Codex、Hermesの作業状況が見える状態を優先します。

## Backlog Structure

進行中の作業はGitHub Issuesを正とします。

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

## Issue Types

Issue typeは次の基準で使います。

- Feature: ユーザーに見える機能、またはマイルストーン級の成果物
- Task: 実装、ドキュメント、検証、環境整備などの具体的作業
- Bug: 不具合、退行、期待と異なる挙動

迷ったらTaskにします。

## Issue Fields

軽量な計画管理として、リポジトリのIssue fieldを使います。

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

Start dateとTarget dateは、日付管理が本当に必要な作業にだけ使います。

欄を埋めるためだけの日付は入れません。

## Task Lifecycle

通常の開発では次の状態で考えます。

1. Backlog: Issueはあるが、まだ着手可能とは限らない
2. Ready: スコープと完了条件が明確
3. In Progress: ブランチがあり、作業中
4. Review: Pull Requestがopen
5. Done: Pull Requestがmergeされ、Issueがclose

GitHub Projectsのボードを作るまでは、状態はIssueコメントや紐付いたPull Requestで表現します。

## Branch and Pull Request Linkage

意味のある作業ごとにブランチを作ります。

推奨ブランチ名:

```text
codex/issue-4-json-schema
hermes/issue-5-provider-interface
hermes/issue-10-tests-and-readme
fix/issue-7-json-fallback
```

CodexまたはHermesが主体の作業では、必ず`codex/`または`hermes/`のprefixを使います。

`feature/`は、人間主導の作業、または担当agentが未確定の共有作業だけに使います。

既にopen済みのPRでbranch名がずれている場合は、無理にrenameせず、PRタイトル・本文・コメントのいずれかに作業主体を明記します。次回以降のbranchで正しいprefixに戻します。

Pull Request本文にはIssue番号を入れます。

Issueを完全に完了する場合:

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

- Codex実装: Hermesまたは人間の開発者がレビュー
- Hermes実装: Codexまたは人間の開発者がレビュー
- 人間の実装: 必要に応じてCodexまたはHermesがレビュー

PR作成者は自分のPRをapproveしません。

最終的な受け入れとmerge判断は、明示的に委譲されない限り人間の開発者が持ちます。

## Hermes Task Assignment

Hermesに作業を渡すIssueは、実装専任で動ける粒度まで事前に絞ります。

Hermes向けIssueには、できるだけ次の情報を入れます。

```md
## Hermesへの指示

このIssueでは実装のみ行う。

設計判断、公開API変更、アーキテクチャ変更、依存ライブラリ追加、データ形式変更、ディレクトリ構成変更、広範囲のリファクタが必要に見えた場合は、実装を止めてIssueまたはPull Requestで確認すること。

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

Hermes向けIssueでは、曖昧な表現を避けます。

- 避ける: いい感じに整える
- 避ける: 必要なら設計も見直す
- 避ける: 関連しそうなところも直す
- 推奨: `src/local_ai_companion/schema.py` の検証条件だけを変更する
- 推奨: `tests/test_schema.py` に不正値のテストを追加する
- 推奨: `docs/api_contracts.md` の該当フィールド説明だけを更新する

HermesがIssue外の改善点を見つけた場合は、実装に含めず、コメントまたは別Issue候補として残します。

## Daily Working Rule

作業開始前:

1. 現在のマイルストーン親Issueを確認する。
2. Ready状態のIssueを1つ選ぶ。
3. 完了条件を確認する。
4. 最新の`main`からブランチを作る。

PR作成前:

1. 最小限必要な検証を実行する。
2. 影響するドキュメントを更新する。
3. PR本文からIssueをリンクする。
4. PRタイトルと説明は日本語を基本にする。
5. PR作成時に`ysk0518`へreview requestを送る。
6. reviewer指定に失敗した場合は、PRコメントと最終報告で明示する。

merge後:

1. 完了したIssueをcloseする。
2. 親Issueに未完了の子Issueが残っているか確認する。
3. 不要になった作業ブランチを削除する。

## GitHub Projects

Issue数が増えたらGitHub Projectsを追加します。

推奨カラム:

- Backlog
- Ready
- In Progress
- Review
- Done

現時点のパイロット段階では、Issueと親子Issueの関係だけで十分です。ボード表示が必要になった時点でProjectsを追加します。

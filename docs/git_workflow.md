# Git Workflow

## Current Policy

Codex、Hermes、Gemma が GitHub 上で自律連携します。作業はブランチ名とコミットメッセージで区別します。

開発フローは GitHub Webhook によるイベント駆動で自動化されています。

## Branch Strategy

### main

`main` is the stable integration branch.

Rules:

- Keep `main` buildable and explainable.
- Do not do direct feature work on `main`.
- Merge only reviewed or explicitly accepted work.
- Documentation-only changes may be merged with lightweight review.

### Agent Branches

Codex、Hermes、Gemma はそれぞれ別の短命ブランチで作業します。

```text
codex/issue-4-response-schema
codex/issue-6-prompt-management
hermes/issue-5-provider-interface
hermes/issue-10-tests-and-readme
gemma/issue-8-typo-fix
gemma/issue-12-doc-check
```

These branches should target one WBS item or one small coherent change.

Agent-owned work should not use `feature/` branches.

### Shared Feature Branches

Use `feature/` only when the work is not owned by a single agent or is driven directly by the human developer.

```text
feature/v0.1-cli
feature/v0.2-go-runtime
```

### Fix Branches

Use `fix/` for defects found after a feature branch is merged.

```text
fix/v0.1-invalid-json-fallback
fix/v0.2-timeout-leak
```

### Docs Branches

Use `docs/` for documentation-only work.

```text
docs/branch-strategy
docs/api-contracts
```

## Merge Policy

Before merging into `main`:

- Working tree must be clean.
- Relevant tests or checks must pass.
- Public contracts must be documented in `docs/api_contracts.md`.
- Architecture-impacting changes must be reflected in `docs/architecture.md` or `docs/decisions.md`.
- WBS progress should be updated when a task is completed.
- A GitHub pull request should be opened from the task branch into `main`.
- At least one reviewer should approve the pull request before merge.

### Auto-Merge

Webhook 駆動で CI 成功時に自動マージが行われます。条件:

- 1件以上の approve review
- テスト通過
- レビューコメントがすべて解決済み
- 明示的なマージ委譲があること

人間の最終判断が必要な場合は自動マージされません。

Preferred merge style:

```text
Squash merge for small task branches.
Regular merge only when preserving multiple commits is useful.
```

Recommended repository settings:

- Set `main` as the default branch.
- Protect `main`.
- Require pull request before merge.
- Require at least one approval.
- Require conversation resolution before merge.
- Prefer squash merge for task branches.
- Disable direct pushes to `main` for agent-operated workflows.

Recommended merge buttons:

- Enable squash merge.
- Keep regular merge available only if preserving branch history is useful.
- Disable rebase merge unless there is a clear reason to use it.

## Pull Request Flow

1. Create a task branch from the latest `main`.
2. Commit focused changes.
3. Push the task branch to GitHub.
4. Open a pull request into `main`.
5. Fill in the pull request template.
6. Request review from the expected reviewer when creating or updating the pull request.
7. Address review comments on the same branch.
8. Merge after approval and verification.
9. Delete the task branch after merge.

For Codex, Hermes, and Gemma:

- Do not merge your own implementation unless auto-merge conditions are met.
- When opening a pull request, request review from `ysk0518` by default.
- If another agent is the expected reviewer, request that reviewer explicitly.
- If reviewing another agent's work, use a review stance: prioritize bugs, contract mismatches, missing tests, and operational risks.
- If a PR changes architecture or API contracts, verify the relevant docs changed in the same PR.

## Pull Request Language

Pull request titles, descriptions, review comments, and merge notes should be written in Japanese by default.

English terms may be used when they are standard technical names or when Japanese translation would make the meaning less clear.

## Review Ownership

Default review pairing:

- Codex implementation should be reviewable by Hermes or the human developer.
- Hermes implementation should be reviewable by Codex or the human developer.
- Gemma implementation should be reviewable by Hermes or Codex.
- Human-directed emergency changes can bypass agent review, but should be documented if they affect architecture or contracts.

No agent should silently rewrite another participant's branch history.

## Conflict Policy

When a conflict occurs:

- Preserve user-authored work unless explicitly told otherwise.
- Prefer resolving at the smallest affected file scope.
- If both branches changed a contract, update the contract doc as part of the conflict resolution.
- If both branches changed behavior differently, record the final decision in `docs/decisions.md`.

## Branch Naming

Use small branches scoped to one task.

Examples:

```text
codex/issue-4-response-schema
hermes/issue-5-provider-interface
gemma/issue-8-typo-fix
feature/v0.1-cli
docs/git-workflow
fix/v0.1-json-fallback
```

When checking branch lists, `codex/`、`hermes/`、`gemma/` prefixes should make ownership obvious without opening the pull request.

## Commit Messages

Use:

```text
area: short imperative summary
```

Examples:

```text
docs: add git workflow
core: validate assistant response schema
runtime: add request timeout handling
```

## Review Flow

For meaningful changes:

1. Create a branch.
2. Make a focused change.
3. Run the smallest useful verification.
4. Update docs if behavior, contracts, or architecture changed.
5. Ask another participant to review.

Review should prioritize:

- behavioral bugs
- API contract mismatches
- missing tests
- error handling gaps
- concurrency and cancellation risks
- secret handling risks

## GitHub Token Policy

Fine-grained personal access tokens are used for each agent, scoped to this repository only.

### Recommended Permissions

**Codex:**
```text
Contents: Read and write
Pull requests: Read and write
Issues: Read and write
Metadata: Read
```

**Hermes:**
```text
Contents: Read and write
Pull requests: Read and write
Issues: Read and write
Metadata: Read
```

**Gemma:**
```text
Contents: Read
Pull requests: Read
Issues: Read
Metadata: Read
```

Do not grant broad account-wide permissions unless there is a clear need.

Do not grant Actions, Secrets, or Administration permissions during the initial phase.

Do not allow agent accounts to push directly to `main`.

The human developer keeps final merge authority unless explicitly delegated via auto-merge conditions.

## Files That Should Not Be Committed

- `.env` or other secret files
- API keys
- generated logs
- local runtime data
- Unity Library and Temp folders
- Python virtual environments
- large local model files

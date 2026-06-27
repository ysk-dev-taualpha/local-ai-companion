# Git Workflow

## Current Policy

Use the human developer's Git identity for local commits.

Codex and Hermes do not need separate GitHub accounts during the early phase. Distinguish work by branch names, commit messages, and review notes.

## Branch Strategy

### main

`main` is the stable integration branch.

Rules:

- Keep `main` buildable and explainable.
- Do not do direct feature work on `main`.
- Merge only reviewed or explicitly accepted work.
- Documentation-only changes may be merged with lightweight review.

### Agent Branches

Codex and Hermes should work on separate short-lived branches.

Use agent-prefixed branches when the main actor is clear:

```text
codex/v0.1-python-scaffold
codex/v0.1-response-schema
hermes/v0.1-provider-interface
hermes/v0.1-logging
```

These branches should target one WBS item or one small coherent change.

### Shared Feature Branches

Use `feature/` when the work is not owned by a single agent or is driven directly by the human developer.

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
- At least one reviewer should approve the pull request before merge, unless the human developer explicitly accepts a fast path.

Preferred merge style:

```text
Squash merge for small task branches.
Regular merge only when preserving multiple commits is useful.
```

Local-only early phase:

If no remote repository exists yet, a branch can be merged locally after the human developer accepts it.

GitHub phase:

Once a remote repository exists, prefer GitHub pull requests for all non-trivial changes.

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

For Codex and Hermes:

- Do not merge your own implementation unless the human developer explicitly asks.
- When opening a pull request, request review from `ysk0518` by default.
- If another agent is the expected reviewer, request that reviewer explicitly.
- If review request fails, leave a pull request comment that names the intended reviewer and mention it in the final report.
- If reviewing another agent's work, use a review stance: prioritize bugs, contract mismatches, missing tests, and operational risks.
- If a PR changes architecture or API contracts, verify the relevant docs changed in the same PR.

## Pull Request Language

Pull request titles, descriptions, review comments, and merge notes should be written in Japanese by default.

English terms may be used when they are standard technical names or when Japanese translation would make the meaning less clear.

When English wording is useful for learning or future reference, add it as a short supplement after the Japanese text instead of replacing the Japanese explanation.

## Review Ownership

Default review pairing:

- Codex implementation should be reviewable by Hermes or the human developer.
- Hermes implementation should be reviewable by Codex or the human developer.
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
codex/v0.1-response-schema
hermes/v0.1-provider-interface
feature/v0.1-cli
docs/git-workflow
fix/v0.1-json-fallback
```

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

When useful, mention the agent in the commit body:

```text
Implemented with Codex.
Reviewed by Hermes.
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

When GitHub or MCP integration is introduced, use a fine-grained personal access token scoped to this repository only.

Recommended Codex permissions:

```text
Contents: Read and write
Pull requests: Read and write
Issues: Read and write
Metadata: Read
```

Recommended Hermes permissions:

```text
Contents: Read and write
Pull requests: Read and write
Issues: Read
Metadata: Read
```

Hermes should be able to create branches, push implementation work, and open pull requests.

Hermes should not be able to change issue scope, priority, labels, milestones, or completion conditions.

Do not grant broad account-wide permissions unless there is a clear need.

Do not grant Actions, Secrets, or Administration permissions during the initial phase.

Do not allow agent accounts to push directly to `main`.

The human developer keeps final merge authority unless explicitly delegated.

## Files That Should Not Be Committed

- `.env` or other secret files
- API keys
- generated logs
- local runtime data
- Unity Library and Temp folders
- Python virtual environments
- large local model files

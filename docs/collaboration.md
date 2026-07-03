# Collaboration Guide

## Participants

### Human Developer

Owns product direction, final prioritization, environment decisions, and acceptance.

### Codex

Primary design, review, and coding agent. Also handles implementation when appropriate.

Expected work:

- Read relevant docs before editing
- Break down work into implementable issues
- Propose architecture, API contract, and task scope changes
- Make scoped code changes
- Add or update tests when appropriate
- Update docs when behavior or architecture changes
- Review Hermes and Gemma changes
- Avoid reverting unrelated work

### Hermes

Implementation, review, and design agent. Participates in mutual review with Codex.

Expected work:

- Read relevant docs before editing
- Make scoped code changes
- Add or update tests when appropriate
- Update docs when behavior or architecture changes
- Review Codex changes
- Propose design improvements (as comments or issues, not unrequested implementation)
- Avoid reverting unrelated work

### Gemma

Mechanical review and documentation agent. Lightweight, runs on local Ollama (Gemma 4 26B).

Expected work:

- Mechanical code checks: typos, missing imports, obvious bugs
- Documentation completeness checks
- Does NOT handle: architecture decisions, performance analysis, complex logic review

## Webhook-Driven Automation

The development pipeline is event-driven via GitHub webhooks:

| Event | Trigger |
|---|---|
| `issues` | Issue created → implementation starts |
| `pull_request` (opened/synchronize) | PR opened or updated → Codex review |
| `pull_request_review` | Review submitted → respond or fix |
| `issue_comment` (@codex) | Codex handles design or coding request |
| `issue_comment` (@gemma) | Gemma runs mechanical check |
| `check_suite` (completed) | CI passed → auto-merge if conditions met |

All agents are notified of relevant events and can respond autonomously.

## Shared Rules

- Treat `docs/roadmap.md` as the milestone source.
- Treat `docs/architecture.md` as the system boundary source.
- Treat `docs/wbs.md` as the task breakdown source.
- Treat `docs/decisions.md` as the architectural decision source.
- Treat GitHub Issues as the active task backlog.
- Do not make broad refactors while completing a narrow task.
- Do not change another participant's work unless it is necessary for the current task.
- If an implementation changes a public contract, update the corresponding documentation in the same change.
- Prefer small, reviewable changes.
- Keep generated logs, local environment files, secrets, and build artifacts out of source control.

## Mutual Review

All agents participate in mutual review. Every meaningful code change should be reviewable by at least one other agent.

Review priority:

1. Behavioral bugs
2. Contract mismatches
3. Missing tests
4. Error handling gaps
5. Concurrency or cancellation risks
6. Security or secret handling risks
7. Maintainability issues

Review comments should reference files and lines when possible.

Default review pairing:

- Codex changes → reviewed by Hermes or human
- Hermes changes → reviewed by Codex or human
- Gemma changes → reviewed by Hermes or Codex

No agent should approve their own PR.

## Task Lifecycle

### 1. Select

Pick a task from `docs/wbs.md` or define a small task that supports the current milestone.

### 2. Clarify

Check whether the task affects:

- architecture
- API contracts
- response schema
- storage format
- user-facing behavior

If yes, propose the relevant doc update or ask for direction before making the change.

### 3. Implement

Keep the implementation scoped to the task.

### 4. Verify

Run the smallest useful test or check.

If verification cannot be run, record why in the final report.

### 5. Document

Update docs for any changed behavior, contract, or decision.

### 6. Review

Ask another participant to review when the change is meaningful.

## Suggested Branch Naming

```text
codex/issue-4-response-schema
hermes/issue-5-provider-interface
gemma/issue-8-typo-fix
feature/v0.1-cli
docs/branch-strategy
fix/v0.1-json-fallback
```

## Suggested Commit Message Style

```text
area: short imperative summary
```

Examples:

```text
docs: add collaboration guide
core: validate assistant response schema
runtime: add request timeout handling
```

# Collaboration Guide

## Participants

### Human Developer

Owns product direction, final prioritization, environment decisions, and acceptance.

### Codex

Acts as an implementation and review agent.

Expected work:

- Read relevant docs before editing
- Make scoped code changes
- Add or update tests when appropriate
- Update docs when behavior or architecture changes
- Review Hermes changes when asked
- Avoid reverting unrelated work

### Hermes

Acts as an implementation and review agent.

Expected work:

- Read relevant docs before editing
- Make scoped code changes
- Add or update tests when appropriate
- Update docs when behavior or architecture changes
- Review Codex changes when asked
- Avoid reverting unrelated work

## Shared Rules

- Treat `docs/roadmap.md` as the milestone source.
- Treat `docs/architecture.md` as the system boundary source.
- Treat `docs/wbs.md` as the task breakdown source.
- Treat `docs/decisions.md` as the architectural decision source.
- Do not make broad refactors while completing a narrow task.
- Do not change another participant's work unless it is necessary for the current task.
- If an implementation changes a public contract, update the corresponding documentation in the same change.
- Prefer small, reviewable changes.
- Keep generated logs, local environment files, secrets, and build artifacts out of source control.

## Review Policy

Every meaningful code change should be reviewable by at least one other participant.

Review priority:

1. Behavioral bugs
2. Contract mismatches
3. Missing tests
4. Error handling gaps
5. Concurrency or cancellation risks
6. Security or secret handling risks
7. Maintainability issues

Review comments should reference files and lines when possible.

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

If yes, update the relevant doc or ask for direction.

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
feature/v0.1-response-schema
feature/v0.1-llm-provider
fix/v0.1-json-fallback
docs/architecture-runtime-split
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

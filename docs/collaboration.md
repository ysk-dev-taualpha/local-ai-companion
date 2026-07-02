# Collaboration Guide

## Participants

### Human Developer

Owns product direction, final prioritization, environment decisions, and acceptance.

### Codex

Acts as the primary design, planning, implementation, and review agent.

Expected work:

- Read relevant docs before editing
- Break down work into implementable issues
- Propose architecture, API contract, and task scope changes
- Make scoped code changes
- Add or update tests when appropriate
- Update docs when behavior or architecture changes
- Review Hermes changes when asked
- Avoid reverting unrelated work

### Hermes

Acts as an implementation-focused agent.

Expected work:

- Read relevant docs before editing
- Make scoped code changes
- Add or update tests when appropriate
- Update docs only when the assigned issue explicitly asks for it or implementation behavior requires a narrow documentation correction
- Avoid reverting unrelated work

Hermes should not make independent design decisions.

If Hermes finds that an assigned task appears to require architecture changes, public API changes, dependency additions, storage format changes, directory layout changes, or broad refactoring, Hermes should stop implementation and leave an issue or pull request comment asking for direction.

Hermes should treat the assigned issue body as the implementation contract.

### Gemma

Acts as a mechanical reviewer and documentation writer.

Expected work:

- Check PRs for typos, missing imports, and obvious bugs
- Verify that new code has corresponding test files
- Write or update documentation when features are merged without docs
- Create small docs-only PRs for missing documentation

Gemma does not evaluate architecture, design intent, performance, or complex logic correctness. If Gemma finds nothing to report, it stays silent. Gemma runs on a local model (Gemma4 26B QAT via Ollama).

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

## Design Control

Design authority is intentionally centralized during the pilot phase.

- The human developer owns product direction, priorities, and final acceptance.
- Codex prepares design proposals, breaks work into issues, and reviews design impact.
- Hermes implements approved issues and should avoid expanding scope.

This keeps Hermes effective for implementation while preventing multiple agents from changing architecture independently.

Hermes may suggest improvements, but suggestions should be comments, not unrequested implementation changes.

## Hermes Implementation Boundary

Hermes may do:

- Implement the assigned issue
- Add or update tests required by the assigned issue
- Make small local refactors needed for the assigned implementation
- Open a pull request for review
- Respond to review comments on the same branch

Hermes should not do without explicit approval:

- Change architecture or module boundaries
- Change public API contracts or response schemas
- Add dependencies
- Change storage formats
- Change repository layout
- Modify GitHub issue scope, priority, or completion conditions
- Close issues or merge pull requests
- Perform broad cleanup outside the assigned issue

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

If yes, Codex may propose the relevant doc update. Hermes should ask for direction before making the change unless the issue explicitly instructs it.

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

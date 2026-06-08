---
name: pi-workers
description: Use when delegating work through the local pi_workers MCP tools. Covers Codex parent-orchestrated Pi scout, reviewer, worker, researcher, planner, and context-builder agents using spawn, wait, send, follow-up, close, and list lifecycle tools.
---

# Pi Workers

This skill is for the parent Codex orchestrator only. Do not inject it into spawned workers. The parent owns delegation, synthesis, deciding what to apply, and final user reporting.

Use `pi_workers_*` MCP tools when independent work can run in a fresh Pi worker without filling the parent context: codebase recon, adversarial review, external research, planning, handoff context, or implementation.

MCP v1 is intentionally small: there is no chain, parallel, doctor, raw status, or raw interrupt surface. Compose workflows with multiple `pi_workers_spawn_agent` calls plus `pi_workers_wait_agent`, `pi_workers_send_message`/`pi_workers_followup_task`, `pi_workers_close_agent`, and `pi_workers_list_agents`.

Hard defaults: every spawn is async, fresh-context, and file-only. Reports are Markdown files under `.pi-agents/<agent_type>-<task_name>-<suffix>.md`. Workers do not inherit the Codex conversation.

Safety defaults: reviewers are read-only. Use one `worker` for a given worktree write scope.

## Lifecycle

1. Decide whether to delegate.
   - Delegate bounded, independent work.
   - Keep immediate blocking work local unless waiting is worth the context savings.
   - Do not spawn a worker just to do the next tiny step.

2. Spawn with `pi_workers_spawn_agent`.
   - Required inputs: `task_name`, `agent_type`, `message`.
   - Use lowercase task names with underscores, such as `auth_map`, `correctness_review`, or `test_review`.
   - Spawn parallel work with multiple `pi_workers_spawn_agent` calls.
   - Do not reuse an active `task_name`; it is the local handle for later control.

3. Work locally while workers run.
   - Do non-overlapping parent work: inspect diffs, run local checks, prepare synthesis, or continue implementation.
   - Do not manually poll or narrate repeated "still running" updates.

4. Wait only when useful.
   - Use `pi_workers_wait_agent` when you need the next mailbox update from any live worker.
   - `wait_agent` is not targeted. With multiple workers, it may return whichever worker updates next.
   - Timeout is not completion or failure. Default is 10 minutes; wait silently inside the tool call instead of narrating repeated "still running" checks. Use `timeout_ms` only when a shorter or longer blocking wait is intentional.
   - Read generated `.pi-agents/...` Markdown reports only when needed to integrate findings.

5. Control active workers.
   - Use `pi_workers_send_message` or `pi_workers_followup_task` to resume an active run with more input. They are equivalent resume-style tools; choose the name that best describes your intent.
   - Use `pi_workers_close_agent` when abandoning a worker.
   - Use `pi_workers_list_agents` to see live workers spawned by this MCP server process. Use `path_prefix` to filter by task-name prefix.

## Agent Roles

- `scout`: read-only codebase recon. Ask for architecture, relevant files, call paths, risks, and open questions.
- `reviewer`: read-only adversarial review. Ask for concrete findings with severity and file/line references. Tell reviewers: "Do not edit project/source files; write findings only to the configured output artifact."
- `worker`: implementation. Use one writer for a given worktree scope. Give accepted scope, constraints, validation, and stop rules.
- `researcher`: external evidence. Ask for primary sources, links, confidence, and local implications. Do not ask for edits.
- `planner`: implementation plan. Ask for steps, files, validation, risks, and decisions needing parent/user approval.
- `context-builder`: handoff context. Ask for compact summaries, relevant files, constraints, validation plan, and a worker-ready meta-prompt.
- `oracle`: advisory decision review. MCP workers are fresh-context, so include the relevant decisions, assumptions, and reasoning in the message; keep it read-only unless explicitly scoped otherwise.
- `delegate`: lightweight generic delegation when no specialized role fits.
- `debug`: focused debugging analysis or reproduction guidance.
- `pi-subagents`: subagent-orchestration expertise. Use sparingly; the parent remains responsible for orchestration.

## Prompt Contract

Every `message` should include:

- Goal and exact scope.
- Files, diffs, issue links, commands, or docs to inspect.
- Hard constraints, especially read-only/no-edit requirements.
- Expected output shape: findings, file/line refs, changed files, validation, risks, or handoff prompt.
- Stop rules: when enough evidence is gathered, when to record uncertainty, and what not to expand into.

Because there is no live intercom surface in MCP v1, tell workers to record unresolved decisions, blockers, and needed approvals in their output artifact rather than making product/API/scope decisions silently.

## Common Patterns

### Parallel Review

Use after implementation or before merge. Spawn two or three `reviewer` workers with distinct angles:

- `correctness_review`: correctness, regressions, integration risks.
- `test_review`: missing tests, validation gaps, CLI/API behavior.
- `maintainability_review`: simplicity, naming, docs, operational risks.

All reviewer prompts must be read-only. The parent reads outputs, decides which findings matter, and applies fixes locally or with one `worker`.

### Recon Before Planning

For unfamiliar code, spawn `scout` first. If the task also needs external facts, spawn `researcher` in parallel. Then synthesize locally before planning or implementing.

### Implementation Handoff

Use `worker` only after scope is clear. Give one worker the accepted target, files, constraints, and validation. Do not launch several writer workers into the same dirty worktree scope.

### Review Loop

For "implement then review" work, keep the loop in the parent:

1. `worker` implements accepted scope.
2. fresh `reviewer` workers inspect the actual diff read-only.
3. parent accepts or rejects findings.
4. parent fixes locally or launches one follow-up `worker`.

Stop when reviewers find no blocking or worth-doing-now issues, the remaining feedback is optional, a decision needs user approval, or the loop hits 3 review rounds. Do not loop for optional polish.

---
name: pi-workers
description: Use when delegating work through the local pi_workers MCP tools. Covers Codex parent-orchestrated Pi scout, reviewer, worker, researcher, planner, and context-builder agents using spawn, wait, send, follow-up, close, and list lifecycle tools.
---

# Pi Workers

This skill is for the parent Codex orchestrator only. Do not inject it into spawned workers. The parent owns delegation, synthesis, deciding what to apply, and final user reporting.

Use `pi_workers_*` MCP tools when independent work can run in a fresh Pi worker without filling the parent context: codebase recon, adversarial review, external research, planning, handoff context, or implementation.

## Lifecycle

1. Decide whether to delegate.
   - Delegate bounded, independent work.
   - Keep immediate blocking work local unless waiting is worth the context savings.
   - Do not spawn a worker just to do the next tiny step.

2. Spawn with `pi_workers_spawn_agent`.
   - Required inputs: `task_name`, `agent_type`, `message`.
   - Use lowercase task names with underscores, such as `auth_map`, `correctness_review`, or `test_review`.
   - Spawn parallel work with multiple `pi_workers_spawn_agent` calls.
   - Workers use fresh async context and file-only `.pi-agents/...` outputs by default.

3. Work locally while workers run.
   - Do non-overlapping parent work: inspect diffs, run local checks, prepare synthesis, or continue implementation.
   - Do not manually poll or narrate repeated “still running” updates.

4. Wait only when useful.
   - Use `pi_workers_wait_agent` when you need the next completion/update.
   - If no update arrives before timeout, continue useful local work or wait again later.
   - Read `.pi-agents/...` reports only when needed to integrate findings.

5. Control active workers.
   - Use `pi_workers_send_message` for clarification or extra constraints.
   - Use `pi_workers_followup_task` for a follow-up on the same run.
   - Use `pi_workers_close_agent` when abandoning a worker.
   - Use `pi_workers_list_agents` to see live workers spawned by this MCP server process.

## Agent Roles

- `scout`: read-only codebase recon. Ask for architecture, relevant files, call paths, risks, and open questions.
- `reviewer`: read-only adversarial review. Ask for concrete findings with severity and file/line references. Tell reviewers: "Do not edit project/source files; write findings only to the configured output artifact."
- `worker`: implementation. Use one writer for a given worktree scope. Give accepted scope, constraints, validation, and stop rules.
- `researcher`: external evidence. Ask for primary sources, links, confidence, and local implications. Do not ask for edits.
- `planner`: implementation plan. Ask for steps, files, validation, risks, and decisions needing parent/user approval.
- `context-builder`: handoff context. Ask for compact summaries, relevant files, constraints, validation plan, and a worker-ready meta-prompt.

## Prompt Contract

Every `message` should include:

- Goal and exact scope.
- Files, diffs, issue links, commands, or docs to inspect.
- Hard constraints, especially read-only/no-edit requirements.
- Expected output shape: findings, file/line refs, changed files, validation, risks, or handoff prompt.
- Stop rules: when enough evidence is gathered, when to escalate uncertainty, and what not to expand into.

Fresh workers do not inherit the Codex conversation. Put the necessary context directly in the `message`.

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

Stop when reviewers find no blocking or worth-doing-now issues, the remaining feedback is optional, or a decision needs user approval.

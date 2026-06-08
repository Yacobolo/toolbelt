---
name: pi-workers
description: Use when delegating work through the local pi_workers MCP tools, especially spawning Pi scout, reviewer, worker, researcher, planner, or context-builder agents while Codex remains the parent orchestrator.
---

# Pi Workers

Use `pi_workers_*` MCP tools for bounded work that can run outside the parent context: codebase recon, review, research, implementation handoff, or parallel checks. The parent Codex thread remains responsible for deciding what to delegate, integrating results, and reporting to the user.

## Operating Rules

- Delegate only concrete independent tasks. Keep immediate blocking work local unless waiting is worth the context savings.
- Use `scout` for read-only codebase mapping, `reviewer` for adversarial review, `worker` for implementation, `researcher` for external evidence, `planner` for plans, and `context-builder` for handoff context.
- `task_name` is required and is the local handle. Use stable lowercase names with underscores, such as `auth_map`, `correctness_review`, or `test_review`.
- Spawn parallel work with multiple `pi_workers_spawn_agent` calls, not one combined prompt.
- Spawned workers use fresh async context and file-only outputs. They do not see the Codex conversation unless you put the necessary scope, files, constraints, and success criteria in `message`.
- Prefer review-only prompts unless the user explicitly wants a writer. For implementation, use one writer for a given worktree scope.
- Call `pi_workers_wait_agent` only when you need the next update or result. Do not manually poll or narrate repeated “still running” updates.
- After completion, read generated `.pi-agents/...` reports only when needed to integrate findings. Keep large artifacts out of the parent context unless they matter.
- Use `pi_workers_send_message` or `pi_workers_followup_task` to clarify or refine an active worker. Use `pi_workers_close_agent` when abandoning a live worker.

## Prompt Shape

For each spawned agent, include:

- Goal and exact scope.
- Relevant files, diffs, issue links, or commands to inspect.
- Hard constraints, such as “Do not edit files” for review-only work.
- Expected output shape: concise findings, file/line references, changed files, validation run, or remaining risks.

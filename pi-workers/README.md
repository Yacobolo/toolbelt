# pi-workers

Local STDIO MCP server for Pi's registered `subagent` extension tool.

`pi-workers` is a developer tool for Codex-style delegation. It creates an
in-memory Pi SDK session with `tools: ["subagent"]`, launches Pi workers with
fresh async file-only defaults, and exposes a small model-visible lifecycle over
MCP. It is local to each worktree and writes generated worker reports under
that worktree's `.pi-agents/` directory.

There is no public CLI surface in v1. The only package bin is
`pi-workers-mcp`.

## Requirements

Install Pi's subagent extension in your user Pi environment:

```bash
pi install npm:pi-subagents
```

Install and build this package:

```bash
npm --prefix pi-workers install
npm --prefix pi-workers run build
```

For local development, link the `pi-workers-mcp` bin onto your `PATH`:

```bash
npm --prefix pi-workers run link:global
```

Then verify the linked bin:

```bash
pi-workers-mcp
```

The command is a STDIO MCP server, so it normally waits for an MCP client and
prints nothing when started by hand. Stop it with `Ctrl-C`.

## Codex MCP Config

Register one STDIO MCP server per active worktree:

```toml
[mcp_servers.pi_workers]
command = "node"
args = ["pi-workers/dist/mcp.js"]
cwd = "/absolute/path/to/worktree"
tool_timeout_sec = 1800
```

If `pi-workers-mcp` is on `PATH`, you can register the package bin instead:

```toml
[mcp_servers.pi_workers]
command = "pi-workers-mcp"
cwd = "/absolute/path/to/worktree"
tool_timeout_sec = 1800
```

`tool_timeout_sec` applies to blocking MCP tool calls, especially
`pi_workers_wait_agent`.

## MCP Tools

The server exposes a compact Codex-shaped lifecycle:

```text
pi_workers_list_agents
pi_workers_spawn_agent
pi_workers_wait_agent
pi_workers_send_message
pi_workers_followup_task
pi_workers_close_agent
```

`pi_workers_spawn_agent` accepts:

```json
{
  "task_name": "correctness",
  "agent_type": "reviewer",
  "message": "Review the current diff. Do not edit files."
}
```

`task_name` must use lowercase letters, digits, and underscores. `agent_type` is
one of the known Pi agents: `context-builder`, `debug`, `delegate`, `oracle`,
`pi-subagents`, `planner`, `researcher`, `reviewer`, `scout`, or `worker`.

Spawns always apply these defaults:

```json
{
  "context": "fresh",
  "async": true,
  "output": ".pi-agents/<agent>-<task-name>-<suffix>.md",
  "outputMode": "file-only",
  "progress": false
}
```

Spawn multiple workers with multiple `pi_workers_spawn_agent` calls. The MCP
server keeps an in-memory live-agent registry for workers spawned through that
server process. `task_name` is the local handle, so an active task name cannot
be reused until that worker reaches a terminal state or is closed.

`pi_workers_wait_agent` accepts optional `timeout_ms` only. It waits for the
next live-agent mailbox update and returns one result. The default is `30000`,
minimum `10000`, maximum `3600000`. If a status transition already happened,
the queued update is returned immediately.

`pi_workers_send_message` and `pi_workers_followup_task` send a Pi resume
message to a task name or raw run id. `pi_workers_close_agent` interrupts a task
name or raw run id.

## Development

```bash
npm --prefix pi-workers run check
npm --prefix pi-workers test
```

The default tests use fake subagent tools. The optional smoke test exercises the
real Pi SDK and extension discovery:

```bash
npm --prefix pi-workers run smoke
```

Optional contract tests compare parent-visible client output against direct Pi
tool execution:

```bash
PI_WORKERS_CONTRACT=1 npm --prefix pi-workers test
PI_WORKERS_CONTRACT_RUN=1 npm --prefix pi-workers test
```

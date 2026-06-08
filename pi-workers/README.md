# pi-workers

Standalone developer CLI for Pi's registered `subagent` extension tool.
Codex or another harness shells out to this package; this package creates an
in-memory Pi SDK session with `tools: ["subagent"]` and calls the tool directly.
It also ships a local STDIO MCP server so Codex can use the same lifecycle as
real model-visible tools.

V1 is intentionally opinionated: launch commands are small JSON builders for
our Codex best practice: fresh context, async execution, required labels,
file-only artifacts, and compact attach output by default. Pi owns async runs,
status, result files, logs, sessions, outputs, and persistence.

## Requirements

Install Pi's subagent extension in your user Pi environment:

```bash
pi install npm:pi-subagents
```

## Usage

```bash
npm --prefix pi-workers run build
node pi-workers/dist/cli.js --help
```

Commands:

```bash
pi-workers doctor
pi-workers list
pi-workers get reviewer --agent
pi-workers get review-loop --chain
pi-workers create --config agent.json
pi-workers update reviewer --agent --config '{"model":"anthropic/claude-sonnet-4"}'
pi-workers delete old-chain --chain
pi-workers run reviewer "Review the current diff. Do not edit files." --label diff-review
pi-workers run reviewer "Review the current diff. Do not edit files." --label diff-review --background
pi-workers parallel correctness=reviewer:"Review correctness" files=scout:"Map relevant files"
pi-workers parallel correctness=reviewer:"Review correctness" tests=reviewer:"Review tests and UX"
pi-workers chain --step auth-map=scout:"Map auth" --group '[{"label":"correctness","agent":"reviewer","task":"Review correctness"}]'
pi-workers status piw-example
pi-workers wait piw-example
pi-workers interrupt piw-example
pi-workers resume piw-example "Check the failing test too"
```

The helper launch commands intentionally add Codex-side defaults. For example:

```bash
pi-workers run scout "Map the repo" --label repo-map
```

is equivalent to this native subagent payload, followed by compact attach:

```json
{
  "agent": "scout",
  "task": "Map the repo",
  "label": "repo-map",
  "context": "fresh",
  "async": true,
  "output": ".pi-agents/scout-repo-map-k4p9x2ad.md",
  "outputMode": "file-only",
  "progress": false
}
```

The helper launch commands always set `context: "fresh"` because this CLI does
not run inside a persisted Pi parent conversation and cannot fork the Codex
context window. They also always launch async and write file-only outputs under
`.pi-agents/` by default. Use `--background` when you want to return immediately
instead of attaching, `--label name` for single-run artifact identity,
`label=agent:task` for parallel and inline chain steps, `--output path.md` for
an exact single-run output path, or `--output-dir dir` for a different
generated-output directory. Labels are required for helper launches and become
part of generated artifact names and attach receipts. Use `--reads`, `--skill`,
`--acceptance-json`, `--control-json`, and `--output-schema` to pass the
corresponding supported Pi params.

For Codex-style delegation, use `--background`, keep working locally, then call
`pi-workers wait <id>` only when the result is needed. `wait` blocks inside one
CLI process and prints only the final native Pi status by default, similar to a
`wait_agent` call. Add `--progress` only when you intentionally want compact
state changes in the shell output.

Attach mode watches Pi's `status.json`, then performs one final native `status`
call and prints a small receipt with output paths and trace paths:

```text
[attach] launched 268e9162 parallel
[attach] 2 agents running · 0/2 done
[attach] complete · 2/2 done

[attach] complete 268e9162
Outputs:
  correctness: .pi-agents/reviewer-correctness-k4p9x2ad.md
  tests: .pi-agents/reviewer-tests-k4p9x2ad.md

Trace:
  Log:    /var/.../subagent-log-268e9162.md
```

The full Pi status/result/log files stay in Pi-owned storage until explicitly
opened.

Machine-readable output is available with `--json` or `-j`; it wraps only the
direct tool result. In attach mode, `--json` emits newline-delimited launch,
status, and final events.

Every command has help:

```bash
pi-workers help run
pi-workers chain --help
```

## MCP server

Build the package, then register one STDIO MCP server per worktree:

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

The MCP server is local-first: it runs in the configured `cwd`, writes generated
file-only outputs under that worktree's `.pi-agents/`, and exposes a small
Codex-shaped lifecycle:

```text
pi_workers_list_agents
pi_workers_spawn_agent
pi_workers_wait_agent
pi_workers_send_message
pi_workers_followup_task
pi_workers_close_agent
```

`pi_workers_spawn_agent` accepts `task_name`, `message`, and an `agent_type`
enum for the known Pi agents. `task_name` follows Codex naming: lowercase
letters, digits, and underscores. The tool always launches async with generated
file-only output under `.pi-agents/`. Spawn multiple workers with multiple
`pi_workers_spawn_agent` calls.

```json
{ "task_name": "correctness", "agent_type": "reviewer", "message": "Review the diff." }
```

`pi_workers_list_agents` lists live agents spawned through the current MCP
server process, with optional `path_prefix` filtering. `pi_workers_wait_agent`
matches Codex v2's simple wait shape: it accepts optional `timeout_ms` only and
waits for the next live-agent mailbox update. The MCP server starts a small
background monitor for each spawned Pi worker; status transitions are queued in
an in-memory mailbox, so a later wait returns immediately when an update is
already available. The timeout matches Codex `wait_agent`: default `30000`,
minimum `10000`, maximum `3600000`.

## Development

```bash
npm --prefix pi-workers install
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

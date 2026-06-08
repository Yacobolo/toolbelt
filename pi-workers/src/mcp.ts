#!/usr/bin/env node

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import { AgentRegistry } from "./agent-registry.js";
import { PiWorkersError, createSubagentClient } from "./client.js";
import {
  createOutputNamer,
  requireLabel,
} from "./launch-defaults.js";
import { resultRunId } from "./result.js";
import type {
  CommandResult,
  PiSubagentClient,
  SpawnOptions,
} from "./index.js";
import type { LiveAgent, WaitResult } from "./agent-registry.js";

const DEFAULT_WAIT_TIMEOUT_MS = 30_000;
const MIN_WAIT_TIMEOUT_MS = 10_000;
const MAX_WAIT_TIMEOUT_MS = 3_600_000;
const TASK_NAME_PATTERN = /^[a-z0-9_]+$/;
const AGENT_TYPES = [
  "context-builder",
  "debug",
  "delegate",
  "oracle",
  "pi-subagents",
  "planner",
  "researcher",
  "reviewer",
  "scout",
  "worker",
] as const;

export interface PiWorkersMcpOptions {
  cwd?: string;
  clientFactory?: (cwd?: string) => PiWorkersMcpClient;
  monitorIntervalMs?: number;
}

export type PiWorkersMcpClient = Pick<
  PiSubagentClient,
  "spawn" | "status" | "resume" | "interrupt" | "close"
>;

export function createPiWorkersMcpServer(options: PiWorkersMcpOptions = {}): McpServer {
  const cwd = options.cwd ?? process.cwd();
  const clientFactory = options.clientFactory ?? ((clientCwd?: string) => createSubagentClient(clientCwd ? { cwd: clientCwd } : {}));
  const client = clientFactory(cwd);
  const monitorIntervalMs = options.monitorIntervalMs ?? 1_000;
  const server = new McpServer({
    name: "pi-workers",
    version: "0.1.0",
  });
  const registry = new AgentRegistry(client, monitorIntervalMs);

  server.registerTool("pi_workers_list_agents", {
    title: "Pi Workers List Agents",
    description: "List live Pi worker agents spawned through this MCP server.",
    inputSchema: {
      path_prefix: z.string().min(1).optional(),
    },
  }, async (args) => safeToolCall(() => listLiveAgents(registry.list(args.path_prefix))));

  server.registerTool("pi_workers_spawn_agent", {
    title: "Pi Workers Spawn Agent",
    description: "Spawn one fresh-context async Pi worker with generated file-only output.",
    inputSchema: {
      task_name: z.string().min(1).regex(TASK_NAME_PATTERN, "task_name must use lowercase letters, digits, and underscores"),
      message: z.string().min(1),
      agent_type: z.enum(AGENT_TYPES),
    },
  }, async (args) => {
    const input = spawnAgentOptions(args);
    return safeToolCall(async () => {
      registry.assertCanRegister(args.task_name);
      const result = await client.spawn(input);
      registry.remember(args.task_name, args.agent_type, args.message, result);
      return result;
    }, { output: input.output });
  });

  server.registerTool("pi_workers_wait_agent", {
    title: "Pi Workers Wait Agent",
    description: "Wait for a live Pi worker mailbox update. Defaults to 30000 ms, matching Codex wait_agent.",
    inputSchema: {
      timeout_ms: z.number().int().min(MIN_WAIT_TIMEOUT_MS).max(MAX_WAIT_TIMEOUT_MS).optional(),
    },
  }, async (args) => safeToolCall(async () => waitResult(registry, await registry.waitForUpdate(waitTimeoutMs(args.timeout_ms)))));

  server.registerTool("pi_workers_send_message", {
    title: "Pi Workers Send Message",
    description: "Send a message to an async Pi subagent run.",
    inputSchema: {
      target: z.string().min(1),
      message: z.string().min(1),
    },
  }, async (args) => safeToolCall(() => resumeAgent(registry, client, args.target, args.message)));

  server.registerTool("pi_workers_followup_task", {
    title: "Pi Workers Follow-Up Task",
    description: "Send a follow-up message to an async Pi subagent run.",
    inputSchema: {
      target: z.string().min(1),
      message: z.string().min(1),
    },
  }, async (args) => safeToolCall(() => resumeAgent(registry, client, args.target, args.message)));

  server.registerTool("pi_workers_close_agent", {
    title: "Pi Workers Close Agent",
    description: "Close an async Pi subagent run by interrupting it.",
    inputSchema: {
      target: z.string().min(1),
    },
  }, async (args) => safeToolCall(() => closeAgent(registry, client, args.target)));

  const originalClose = server.close.bind(server);
  server.close = async () => {
    registry.stop();
    await client.close();
    await originalClose();
  };

  return server;
}

export async function main(): Promise<void> {
  const server = createPiWorkersMcpServer();
  await server.connect(new StdioServerTransport());
}

function spawnAgentOptions(args: {
  task_name: string;
  message: string;
  agent_type: string;
}): SpawnOptions {
  const label = requireLabel(args.task_name, "pi_workers_spawn_agent requires task_name");
  return {
    agent: args.agent_type,
    task: args.message,
    label,
    async: true,
    output: createOutputNamer().pathFor(args.agent_type, label),
    outputMode: "file-only",
    progress: false,
  };
}

function waitTimeoutMs(value: number | undefined): number {
  return value ?? DEFAULT_WAIT_TIMEOUT_MS;
}

function waitResult(registry: AgentRegistry, result: WaitResult): CommandResult {
  return commandResult(result.message, {
    message: result.message,
    timed_out: result.timedOut,
    agents: registry.all(),
  }, {
    state: result.event?.agent_status ?? (result.terminal ? "complete" : "running"),
    terminal: result.terminal,
  });
}

async function resumeAgent(
  registry: AgentRegistry,
  client: PiWorkersMcpClient,
  target: string,
  message: string,
): Promise<CommandResult> {
  const agent = registry.resolve(target);
  const result = await client.resume({
    id: agent?.agent_id ?? target,
    message,
  });
  registry.updateLastMessage(agent, message);
  return result;
}

async function closeAgent(
  registry: AgentRegistry,
  client: PiWorkersMcpClient,
  target: string,
): Promise<CommandResult> {
  const agent = registry.resolve(target);
  const result = await client.interrupt({ id: agent?.agent_id ?? target });
  registry.markInterrupted(agent);
  return result;
}

async function listLiveAgents(agents: LiveAgent[]): Promise<CommandResult> {
  const text = agents.length === 0
    ? "Live agents: none"
    : ["Live agents:", ...agents.map(formatLiveAgent)].join("\n");
  return commandResult(text, { agents });
}

function formatLiveAgent(agent: LiveAgent): string {
  const id = agent.agent_id ? ` (${agent.agent_id})` : "";
  const message = agent.last_task_message ? ` — ${agent.last_task_message}` : "";
  return `- ${agent.agent_name}: ${agent.agent_status}${id}${message}`;
}

function commandResult(
  text: string,
  details: Record<string, unknown>,
  overrides: Partial<Pick<CommandResult, "state" | "terminal">> = {},
): CommandResult {
  return {
    callId: "pi-workers-mcp",
    params: {},
    text,
    isError: false,
    ...(overrides.state ? { state: overrides.state } : {}),
    terminal: overrides.terminal ?? false,
    details,
  };
}

async function safeToolCall(
  callback: () => Promise<CommandResult>,
  extra: Record<string, unknown> = {},
): Promise<CallToolResult> {
  try {
    return toMcpResult(await callback(), extra);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return {
      isError: true,
      content: [{ type: "text", text: message }],
      structuredContent: { isError: true, message },
    };
  }
}

function toMcpResult(result: CommandResult, extra: Record<string, unknown>): CallToolResult {
  const structuredContent = {
    callId: result.callId,
    params: result.params,
    text: result.text,
    isError: result.isError,
    state: result.state,
    terminal: result.terminal,
    runId: resultRunId(result),
    agents: resultAgents(result.details),
    timed_out: resultTimedOut(result.details),
    ...extra,
  };
  return {
    isError: result.isError,
    content: [{ type: "text", text: result.text }],
    structuredContent,
  };
}

function resultAgents(details: unknown): LiveAgent[] | undefined {
  if (!details || typeof details !== "object" || Array.isArray(details)) return undefined;
  const value = (details as { agents?: unknown }).agents;
  if (!Array.isArray(value)) return undefined;
  return value.filter((item): item is LiveAgent => Boolean(item) && typeof item === "object" && "agent_name" in item);
}

function resultTimedOut(details: unknown): boolean | undefined {
  if (!details || typeof details !== "object" || Array.isArray(details)) return undefined;
  const value = (details as { timed_out?: unknown }).timed_out;
  return typeof value === "boolean" ? value : undefined;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((error) => {
    const message = error instanceof PiWorkersError || error instanceof Error ? error.message : String(error);
    process.stderr.write(`${message}\n`);
    process.exit(1);
  });
}

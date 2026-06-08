#!/usr/bin/env node

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import { PiWorkersError, createSubagentClient } from "./client.js";
import {
  createOutputNamer,
  requireLabel,
} from "./launch-defaults.js";
import type {
  CommandResult,
  PiSubagentClient,
  SpawnOptions,
} from "./index.js";

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

type AgentStatus = "pending_init" | "running" | "interrupted" | "shutdown" | "not_found" | "complete" | "completed" | "failed" | "paused" | "cancelled" | "canceled" | string;

interface LiveAgent {
  agent_name: string;
  agent_id?: string | undefined;
  agent_type: string;
  agent_status: AgentStatus;
  last_task_message: string | null;
}

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
  const liveAgents = new Map<string, LiveAgent>();
  const mailbox = createMailbox();
  const monitorState = { stopped: false };

  server.registerTool("pi_workers_list_agents", {
    title: "Pi Workers List Agents",
    description: "List live Pi worker agents spawned through this MCP server.",
    inputSchema: {
      path_prefix: z.string().min(1).optional(),
    },
  }, async (args) => safeToolCall(() => listLiveAgents(liveAgents, args.path_prefix)));

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
      const result = await client.spawn(input);
      rememberLiveAgent(liveAgents, args.task_name, args.agent_type, args.message, result);
      startAgentMonitor({
        client,
        liveAgents,
        mailbox,
        monitorState,
        taskName: args.task_name,
        intervalMs: monitorIntervalMs,
      });
      return result;
    }, { output: input.output });
  });

  server.registerTool("pi_workers_wait_agent", {
    title: "Pi Workers Wait Agent",
    description: "Wait for a live Pi worker mailbox update. Defaults to 30000 ms, matching Codex wait_agent.",
    inputSchema: {
      timeout_ms: z.number().int().min(MIN_WAIT_TIMEOUT_MS).max(MAX_WAIT_TIMEOUT_MS).optional(),
    },
  }, async (args) => safeToolCall(() => waitForMailboxUpdate({
    liveAgents,
    mailbox,
    timeoutMs: waitTimeoutMs(args.timeout_ms),
  })));

  server.registerTool("pi_workers_send_message", {
    title: "Pi Workers Send Message",
    description: "Send a message to an async Pi subagent run.",
    inputSchema: {
      target: z.string().min(1),
      message: z.string().min(1),
    },
  }, async (args) => safeToolCall(async () => {
    const agent = resolveLiveAgent(liveAgents, args.target);
    const result = await client.resume({
      id: agent?.agent_id ?? args.target,
      message: args.message,
    });
    if (agent) agent.last_task_message = args.message;
    return result;
  }));

  server.registerTool("pi_workers_followup_task", {
    title: "Pi Workers Follow-Up Task",
    description: "Send a follow-up message to an async Pi subagent run.",
    inputSchema: {
      target: z.string().min(1),
      message: z.string().min(1),
    },
  }, async (args) => safeToolCall(async () => {
    const agent = resolveLiveAgent(liveAgents, args.target);
    const result = await client.resume({
      id: agent?.agent_id ?? args.target,
      message: args.message,
    });
    if (agent) agent.last_task_message = args.message;
    return result;
  }));

  server.registerTool("pi_workers_close_agent", {
    title: "Pi Workers Close Agent",
    description: "Close an async Pi subagent run by interrupting it.",
    inputSchema: {
      target: z.string().min(1),
    },
  }, async (args) => safeToolCall(async () => {
    const agent = resolveLiveAgent(liveAgents, args.target);
    const result = await client.interrupt({ id: agent?.agent_id ?? args.target });
    if (agent) {
      agent.agent_status = "interrupted";
      enqueueMailbox(mailbox, {
        agent_name: agent.agent_name,
        agent_status: "interrupted",
        message: `${agent.agent_name}: interrupted`,
        terminal: true,
      });
    }
    return result;
  }));

  const originalClose = server.close.bind(server);
  server.close = async () => {
    monitorState.stopped = true;
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
  return cleanParams({
    agent: args.agent_type,
    task: args.message,
    label,
    async: true,
    output: createOutputNamer().pathFor(args.agent_type, label),
    outputMode: "file-only",
    progress: false,
  }) as unknown as SpawnOptions;
}

interface WaitForMailboxOptions {
  liveAgents: Map<string, LiveAgent>;
  mailbox: Mailbox;
  timeoutMs?: number | undefined;
}

async function waitForMailboxUpdate(options: WaitForMailboxOptions): Promise<CommandResult> {
  const queued = options.mailbox.events.shift();
  if (queued) return mailboxEventResult(queued, options.liveAgents);

  const active = [...options.liveAgents.values()].some((agent) => !isTerminalState(agent.agent_status));
  if (!active) return waitSummary("No live agents.", false, true, options.liveAgents);

  const event = await waitForMailboxEvent(options.mailbox, options.timeoutMs ?? DEFAULT_WAIT_TIMEOUT_MS);
  return event
    ? mailboxEventResult(event, options.liveAgents)
    : waitSummary("No agent updates before timeout.", false, true, options.liveAgents);
}

function waitTimeoutMs(value: number | undefined): number {
  return value ?? DEFAULT_WAIT_TIMEOUT_MS;
}

function rememberLiveAgent(
  liveAgents: Map<string, LiveAgent>,
  taskName: string,
  agentType: string,
  message: string,
  result: CommandResult,
): void {
  liveAgents.set(taskName, {
    agent_name: taskName,
    agent_id: runId(result),
    agent_type: agentType,
    agent_status: result.state ?? "running",
    last_task_message: message,
  });
}

interface MailboxEvent {
  agent_name: string;
  agent_status: AgentStatus;
  message: string;
  terminal: boolean;
}

interface Mailbox {
  events: MailboxEvent[];
  waiters: Array<(event: MailboxEvent | undefined) => void>;
}

interface MonitorOptions {
  client: PiWorkersMcpClient;
  liveAgents: Map<string, LiveAgent>;
  mailbox: Mailbox;
  monitorState: { stopped: boolean };
  taskName: string;
  intervalMs: number;
}

function createMailbox(): Mailbox {
  return { events: [], waiters: [] };
}

function startAgentMonitor(options: MonitorOptions): void {
  void monitorAgent(options);
}

async function monitorAgent(options: MonitorOptions): Promise<void> {
  while (!options.monitorState.stopped) {
    await sleep(options.intervalMs);
    if (options.monitorState.stopped) return;

    const agent = options.liveAgents.get(options.taskName);
    if (!agent || !agent.agent_id || isTerminalState(agent.agent_status)) return;

    let result: CommandResult;
    try {
      result = await options.client.status({ id: agent.agent_id });
    } catch (error) {
      enqueueMailbox(options.mailbox, {
        agent_name: agent.agent_name,
        agent_status: agent.agent_status,
        message: `${agent.agent_name}: status check failed: ${error instanceof Error ? error.message : String(error)}`,
        terminal: false,
      });
      continue;
    }

    const nextStatus = result.state ?? lineValue(result.text, "State") ?? agent.agent_status;
    const terminal = result.terminal || isTerminalState(nextStatus);
    if (nextStatus === agent.agent_status && !terminal) continue;

    agent.agent_status = nextStatus;
    enqueueMailbox(options.mailbox, {
      agent_name: agent.agent_name,
      agent_status: nextStatus,
      message: `${agent.agent_name}: ${nextStatus}`,
      terminal,
    });
    if (terminal) return;
  }
}

function enqueueMailbox(mailbox: Mailbox, event: MailboxEvent): void {
  const waiter = mailbox.waiters.shift();
  if (waiter) {
    waiter(event);
    return;
  }
  mailbox.events.push(event);
}

function waitForMailboxEvent(mailbox: Mailbox, timeoutMs: number): Promise<MailboxEvent | undefined> {
  return new Promise((resolve) => {
    const timeout = setTimeout(() => finish(undefined), timeoutMs);

    function finish(event: MailboxEvent | undefined): void {
      clearTimeout(timeout);
      const index = mailbox.waiters.indexOf(finish);
      if (index >= 0) mailbox.waiters.splice(index, 1);
      resolve(event);
    }

    mailbox.waiters.push(finish);
  });
}

function mailboxEventResult(event: MailboxEvent, liveAgents: Map<string, LiveAgent>): CommandResult {
  return commandResult(event.message, {
    message: event.message,
    timed_out: false,
    agents: [...liveAgents.values()],
  }, { state: event.terminal ? "complete" : "running", terminal: event.terminal });
}

async function listLiveAgents(liveAgents: Map<string, LiveAgent>, pathPrefix: string | undefined): Promise<CommandResult> {
  const agents = [...liveAgents.values()]
    .filter((agent) => pathPrefix === undefined || agent.agent_name.startsWith(pathPrefix));
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

function waitSummary(message: string, terminal: boolean, timedOut: boolean, liveAgents: Map<string, LiveAgent>): CommandResult {
  return commandResult(message, {
    message,
    timed_out: timedOut,
    agents: [...liveAgents.values()],
  }, { state: terminal ? "complete" : "running", terminal });
}

function commandResult(
  text: string,
  details: Record<string, unknown>,
  overrides: Partial<Pick<CommandResult, "state" | "terminal">> = {},
): CommandResult {
  return cleanParams({
    callId: "pi-workers-mcp",
    params: {},
    text,
    isError: false,
    state: overrides.state,
    terminal: overrides.terminal ?? false,
    details,
    raw: { content: [{ type: "text", text }], details },
  }) as unknown as CommandResult;
}

function resolveLiveAgent(liveAgents: Map<string, LiveAgent>, target: string): LiveAgent | undefined {
  return liveAgents.get(target) ?? [...liveAgents.values()].find((agent) => agent.agent_id === target);
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
    runId: runId(result),
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

function runId(result: CommandResult): string | undefined {
  return lineValue(result.text, "Run") ?? detailsRunId(result.details);
}

function detailsRunId(details: unknown): string | undefined {
  if (!details || typeof details !== "object" || Array.isArray(details)) return undefined;
  const record = details as Record<string, unknown>;
  const value = record.runId ?? record.asyncId;
  return typeof value === "string" && value.trim() ? value : undefined;
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

function isTerminalState(value: unknown): boolean {
  return typeof value === "string" && ["complete", "completed", "failed", "paused", "cancelled", "canceled", "interrupted"].includes(value.toLowerCase());
}

function remainingTimeout(started: number, timeoutMs: number | undefined): number | undefined {
  if (timeoutMs === undefined) return undefined;
  return timeoutMs - (Date.now() - started);
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function lineValue(text: string, label: string): string | undefined {
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = text.match(new RegExp(`^${escaped}:\\s*(.+)$`, "m"));
  return match?.[1]?.trim() || undefined;
}

function cleanParams(input: Record<string, unknown>): Record<string, unknown> {
  const output: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(input)) {
    if (value !== undefined) output[key] = value;
  }
  return output;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((error) => {
    const message = error instanceof PiWorkersError || error instanceof Error ? error.message : String(error);
    process.stderr.write(`${message}\n`);
    process.exit(1);
  });
}

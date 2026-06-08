import { lineValue, resultRunId } from "./result.js";
import type { CommandResult, StatusOptions } from "./types.js";

export type AgentStatus =
  | "pending_init"
  | "running"
  | "interrupted"
  | "shutdown"
  | "not_found"
  | "complete"
  | "completed"
  | "failed"
  | "paused"
  | "cancelled"
  | "canceled"
  | string;

export interface LiveAgent {
  agent_name: string;
  agent_id?: string | undefined;
  agent_type: string;
  agent_status: AgentStatus;
  last_task_message: string | null;
}

export interface MailboxEvent {
  agent_name: string;
  agent_status: AgentStatus;
  message: string;
  terminal: boolean;
}

export interface WaitResult {
  message: string;
  timedOut: boolean;
  terminal: boolean;
  event?: MailboxEvent;
}

interface AgentMonitorClient {
  status(input: StatusOptions): Promise<CommandResult>;
}

const terminalStates = new Set([
  "complete",
  "completed",
  "failed",
  "paused",
  "cancelled",
  "canceled",
  "interrupted",
]);

export class AgentRegistry {
  private readonly agents = new Map<string, LiveAgent>();
  private readonly mailbox = new Mailbox();
  private stopped = false;

  constructor(
    private readonly client: AgentMonitorClient,
    private readonly monitorIntervalMs: number,
  ) {}

  stop(): void {
    this.stopped = true;
    this.mailbox.resolveAll();
  }

  assertCanRegister(taskName: string): void {
    const existing = this.agents.get(taskName);
    if (existing && !isTerminalState(existing.agent_status)) {
      throw new Error(`Live agent task_name already exists: ${taskName}`);
    }
  }

  remember(taskName: string, agentType: string, message: string, result: CommandResult): void {
    this.assertCanRegister(taskName);
    this.agents.set(taskName, {
      agent_name: taskName,
      agent_id: resultRunId(result),
      agent_type: agentType,
      agent_status: result.state ?? "running",
      last_task_message: message,
    });
    this.startMonitor(taskName);
  }

  list(pathPrefix: string | undefined): LiveAgent[] {
    return [...this.agents.values()]
      .filter((agent) => pathPrefix === undefined || agent.agent_name.startsWith(pathPrefix));
  }

  all(): LiveAgent[] {
    return [...this.agents.values()];
  }

  resolve(target: string): LiveAgent | undefined {
    return this.agents.get(target) ?? [...this.agents.values()].find((agent) => agent.agent_id === target);
  }

  updateLastMessage(agent: LiveAgent | undefined, message: string): void {
    if (agent) agent.last_task_message = message;
  }

  markInterrupted(agent: LiveAgent | undefined): void {
    if (!agent) return;
    agent.agent_status = "interrupted";
    this.mailbox.enqueue({
      agent_name: agent.agent_name,
      agent_status: "interrupted",
      message: `${agent.agent_name}: interrupted`,
      terminal: true,
    });
  }

  async waitForUpdate(timeoutMs: number): Promise<WaitResult> {
    const queued = this.mailbox.shift();
    if (queued) return eventWaitResult(queued);

    const active = this.all().some((agent) => !isTerminalState(agent.agent_status));
    if (!active) {
      return { message: "No live agents.", timedOut: true, terminal: false };
    }

    const event = await this.mailbox.wait(timeoutMs);
    return event
      ? eventWaitResult(event)
      : { message: "No agent updates before timeout.", timedOut: true, terminal: false };
  }

  private startMonitor(taskName: string): void {
    void this.monitor(taskName);
  }

  private async monitor(taskName: string): Promise<void> {
    while (!this.stopped) {
      await sleep(this.monitorIntervalMs);
      if (this.stopped) return;

      const agent = this.agents.get(taskName);
      if (!agent || !agent.agent_id || isTerminalState(agent.agent_status)) return;

      let result: CommandResult;
      try {
        result = await this.client.status({ id: agent.agent_id });
      } catch (error) {
        this.mailbox.enqueue({
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
      this.mailbox.enqueue({
        agent_name: agent.agent_name,
        agent_status: nextStatus,
        message: `${agent.agent_name}: ${nextStatus}`,
        terminal,
      });
      if (terminal) return;
    }
  }
}

export function isTerminalState(value: unknown): boolean {
  return typeof value === "string" && terminalStates.has(value.toLowerCase());
}

function eventWaitResult(event: MailboxEvent): WaitResult {
  return {
    message: event.message,
    timedOut: false,
    terminal: event.terminal,
    event,
  };
}

type Waiter = (event: MailboxEvent | undefined) => void;

class Mailbox {
  private readonly events: MailboxEvent[] = [];
  private readonly waiters: Waiter[] = [];

  shift(): MailboxEvent | undefined {
    return this.events.shift();
  }

  enqueue(event: MailboxEvent): void {
    const waiter = this.waiters.shift();
    if (waiter) {
      waiter(event);
      return;
    }
    this.events.push(event);
  }

  wait(timeoutMs: number): Promise<MailboxEvent | undefined> {
    return new Promise((resolve) => {
      let waiter: Waiter;
      const timeout = setTimeout(() => {
        this.removeWaiter(waiter);
        resolve(undefined);
      }, timeoutMs);
      timeout.unref?.();

      waiter = (event) => {
        clearTimeout(timeout);
        this.removeWaiter(waiter);
        resolve(event);
      };
      this.waiters.push(waiter);
    });
  }

  resolveAll(): void {
    for (const waiter of this.waiters.splice(0)) {
      waiter(undefined);
    }
  }

  private removeWaiter(waiter: Waiter): void {
    const index = this.waiters.indexOf(waiter);
    if (index >= 0) this.waiters.splice(index, 1);
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => {
    const timeout = setTimeout(resolve, ms);
    timeout.unref?.();
  });
}

import { randomUUID } from "node:crypto";
import { createAgentSession, SessionManager } from "@earendil-works/pi-coding-agent";
import { normalizeResult } from "./result.js";
import type {
  CommandResult,
  CreateClientOptions,
  ExecuteOptions,
  InterruptOptions,
  PiWorkersSession,
  ResumeOptions,
  SpawnOptions,
  StatusOptions,
  SubagentParams,
} from "./types.js";

const missingSubagentMessage = [
  "Pi subagent tool is not available.",
  "Install the Pi extension with: pi install npm:pi-subagents",
].join(" ");

export class PiWorkersError extends Error {}

export class PiSubagentClient {
  private readonly cwd: string;
  private readonly sessionFactory: (cwd: string) => Promise<PiWorkersSession>;
  private readonly idFactory: () => string;
  private session: PiWorkersSession | undefined;

  constructor(options: CreateClientOptions = {}) {
    this.cwd = options.cwd ?? process.cwd();
    this.sessionFactory = options.sessionFactory ?? createDefaultSession;
    this.idFactory = options.idFactory ?? defaultID;
  }

  async close(): Promise<void> {
    this.session?.dispose();
    this.session = undefined;
  }

  async execute(params: SubagentParams, options: ExecuteOptions = {}): Promise<CommandResult> {
    const callId = this.idFactory();
    const session = await this.getSession();
    const tool = session.getToolDefinition("subagent");
    if (!tool) throw new PiWorkersError(missingSubagentMessage);
    const context = session.extensionRunner.createContext();
    const raw = await tool.execute(callId, params as never, options.signal, options.onUpdate, context);
    return normalizeResult(callId, params, raw);
  }

  async spawn(input: SpawnOptions, options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute(cleanParams({
      agent: input.agent,
      task: input.task,
      label: input.label,
      async: input.async,
      context: "fresh",
      output: input.output,
      outputMode: input.outputMode,
      progress: input.progress,
    }), options);
  }

  async status(input: StatusOptions = {}, options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute(cleanParams({ action: "status", id: input.id }), options);
  }

  async interrupt(input: InterruptOptions = {}, options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute(cleanParams({ action: "interrupt", id: input.id }), options);
  }

  async resume(input: ResumeOptions, options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute(cleanParams({
      action: "resume",
      id: input.id,
      message: input.message,
      index: input.index,
    }), options);
  }

  private async getSession(): Promise<PiWorkersSession> {
    this.session ??= await this.sessionFactory(this.cwd);
    return this.session;
  }
}

export function createSubagentClient(options: CreateClientOptions = {}): PiSubagentClient {
  return new PiSubagentClient(options);
}

async function createDefaultSession(cwd: string): Promise<PiWorkersSession> {
  const { session } = await createAgentSession({
    cwd,
    tools: ["subagent"],
    sessionManager: SessionManager.inMemory(),
  });
  return session;
}

function cleanParams(input: Record<string, unknown>): SubagentParams {
  const output: SubagentParams = {};
  for (const [key, value] of Object.entries(input)) {
    if (value !== undefined) output[key] = value;
  }
  return output;
}

function defaultID(): string {
  return `piw-${Date.now().toString(36)}-${randomUUID().slice(0, 8)}`;
}

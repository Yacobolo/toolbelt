import { randomUUID } from "node:crypto";
import { createAgentSession, SessionManager } from "@earendil-works/pi-coding-agent";
import { normalizeResult } from "./result.js";
import type {
  ChainOptions,
  CommandResult,
  CreateClientOptions,
  CreateOptions,
  DeleteOptions,
  ExecuteOptions,
  GetOptions,
  InterruptOptions,
  ParallelOptions,
  PiWorkersSession,
  ResumeOptions,
  SpawnOptions,
  StatusOptions,
  SubagentParams,
  SubagentTask,
  UpdateOptions,
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

  async doctor(options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute({ action: "doctor" }, options);
  }

  async list(options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute({ action: "list" }, options);
  }

  async get(input: GetOptions, options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute(cleanParams({ action: "get", agent: input.agent, chainName: input.chainName }), options);
  }

  async create(input: CreateOptions, options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute({ action: "create", config: input.config }, options);
  }

  async update(input: UpdateOptions, options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute(cleanParams({
      action: "update",
      agent: input.agent,
      chainName: input.chainName,
      config: input.config,
    }), options);
  }

  async delete(input: DeleteOptions, options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute(cleanParams({ action: "delete", agent: input.agent, chainName: input.chainName }), options);
  }

  async spawn(input: SpawnOptions, options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute(cleanParams({
      agent: input.agent,
      task: input.task,
      label: input.label,
      async: asyncOption(input),
      context: "fresh",
      cwd: input.cwd,
      count: input.count,
      output: input.output,
      outputMode: input.outputMode,
      model: input.model,
      progress: input.progress,
      reads: input.reads,
      skill: input.skill,
      acceptance: input.acceptance,
      outputSchema: input.outputSchema,
    }), options);
  }

  async parallel(input: ParallelOptions, options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute(cleanParams({
      tasks: input.tasks.map(taskParams),
      async: asyncOption(input),
      context: "fresh",
      concurrency: input.concurrency,
      cwd: input.cwd,
      worktree: input.worktree,
      acceptance: input.acceptance,
      control: input.control,
    }), options);
  }

  async chain(input: ChainOptions, options: ExecuteOptions = {}): Promise<CommandResult> {
    return this.execute(cleanParams({
      chain: input.chain,
      task: input.task,
      async: asyncOption(input),
      context: "fresh",
      cwd: input.cwd,
      chainDir: input.chainDir,
      concurrency: input.concurrency,
      acceptance: input.acceptance,
      control: input.control,
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

function taskParams(task: SubagentTask): SubagentParams {
  return cleanParams({
    agent: task.agent,
    task: task.task,
    label: task.label,
    cwd: task.cwd,
    count: task.count,
    output: task.output,
    outputMode: task.outputMode,
    model: task.model,
    progress: task.progress,
    reads: task.reads,
    skill: task.skill,
    acceptance: task.acceptance,
    outputSchema: task.outputSchema,
  });
}

function cleanParams(input: Record<string, unknown>): SubagentParams {
  const output: SubagentParams = {};
  for (const [key, value] of Object.entries(input)) {
    if (value !== undefined) output[key] = value;
  }
  return output;
}

function asyncOption(input: { async?: boolean; foreground?: boolean }): boolean | undefined {
  if (input.foreground) return false;
  return input.async;
}

function defaultID(): string {
  return `piw-${Date.now().toString(36)}-${randomUUID().slice(0, 8)}`;
}

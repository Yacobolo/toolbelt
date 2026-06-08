import type { AgentToolResult, AgentToolUpdateCallback, ExtensionContext, ToolDefinition } from "@earendil-works/pi-coding-agent";

export type OutputMode = "inline" | "file-only";

export interface SubagentTask {
  agent: string;
  task: string;
  label?: string;
  cwd?: string;
  count?: number;
  output?: string | false;
  outputMode?: OutputMode;
  model?: string;
  progress?: boolean;
  reads?: string[] | false;
  skill?: string[] | string | boolean;
  acceptance?: unknown;
  outputSchema?: unknown;
}

export interface SpawnOptions extends SubagentTask {
  async?: boolean;
  foreground?: boolean;
}

export interface ParallelOptions {
  tasks: SubagentTask[];
  async?: boolean;
  foreground?: boolean;
  concurrency?: number;
  cwd?: string;
  worktree?: boolean;
  acceptance?: unknown;
  control?: unknown;
}

export type ChainStep = Record<string, unknown>;

export interface ChainOptions {
  chain: ChainStep[];
  task?: string;
  async?: boolean;
  foreground?: boolean;
  cwd?: string;
  chainDir?: string;
  concurrency?: number;
  acceptance?: unknown;
  control?: unknown;
}

export interface StatusOptions {
  id?: string;
}

export interface InterruptOptions {
  id?: string;
}

export interface ResumeOptions {
  id: string;
  message: string;
  index?: number;
}

export interface WaitOptions {
  id: string;
  timeoutMs?: number;
  progress?: boolean;
}

export type ManagementTarget =
  | { agent: string; chainName?: never }
  | { agent?: never; chainName: string };

export type GetOptions = ManagementTarget;

export interface CreateOptions {
  config: unknown;
}

export type UpdateOptions = ManagementTarget & { config: unknown };

export type DeleteOptions = ManagementTarget;

export type SubagentParams = Record<string, unknown>;
export type SubagentToolResult = AgentToolResult<unknown>;
export type SubagentUpdateCallback = AgentToolUpdateCallback<unknown>;

export type SubagentTool = Pick<ToolDefinition, "execute" | "name">;

export interface PiWorkersSession {
  getToolDefinition(name: string): SubagentTool | undefined;
  extensionRunner: {
    createContext(): ExtensionContext;
  };
  dispose(): void;
}

export interface CommandResult {
  callId: string;
  params: SubagentParams;
  text: string;
  isError: boolean;
  state?: string;
  terminal: boolean;
  details?: unknown;
  raw: SubagentToolResult;
}

export interface CreateClientOptions {
  cwd?: string;
  sessionFactory?: (cwd: string) => Promise<PiWorkersSession>;
  idFactory?: () => string;
}

export interface ExecuteOptions {
  signal?: AbortSignal;
  onUpdate?: SubagentUpdateCallback;
}

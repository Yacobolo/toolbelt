import type { AgentToolResult, AgentToolUpdateCallback, ExtensionContext, ToolDefinition } from "@earendil-works/pi-coding-agent";

export type OutputMode = "inline" | "file-only";

export interface SpawnOptions {
  agent: string;
  task: string;
  label?: string;
  output?: string;
  outputMode?: OutputMode;
  progress?: boolean;
  async?: boolean;
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
  raw?: SubagentToolResult;
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

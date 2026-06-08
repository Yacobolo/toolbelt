import * as crypto from "node:crypto";
import * as path from "node:path";
import type { ChainOptions, SubagentTask } from "./types.js";

export interface OutputNamer {
  pathFor(agent: string, label: string | undefined): string;
}

export function createOutputNamer(outputDir = ".pi-agents"): OutputNamer {
  if (!outputDir.trim()) throw new Error("outputDir must not be empty");
  const suffix = outputSuffix();
  const seen = new Map<string, number>();
  return {
    pathFor(agent: string, label: string | undefined): string {
      const agentSlug = slug(agent);
      const labelSlug = label ? slug(label) : undefined;
      const base = labelSlug
        ? labelSlug.startsWith(`${agentSlug}-`) ? labelSlug : slug(`${agentSlug}-${labelSlug}`)
        : agentSlug || "subagent-output";
      const count = (seen.get(base) ?? 0) + 1;
      seen.set(base, count);
      const numbered = count === 1 ? base : `${base}-${count}`;
      return path.join(outputDir, `${numbered}-${suffix}.md`);
    },
  };
}

export function normalizeLabel(value: string | undefined, label = "label"): string | undefined {
  if (value === undefined) return undefined;
  const parsed = slug(value);
  if (!parsed) throw new Error(`${label} must contain at least one letter or number`);
  return parsed;
}

export function requireLabel(value: string | undefined, message: string): string {
  const label = normalizeLabel(value, "label");
  if (!label) throw new Error(message);
  return label;
}

export function applyTaskOutputDefaults(task: SubagentTask, namer: OutputNamer): void {
  const label = requireLabel(task.label, `${task.agent} task requires label`);
  task.label = label;
  task.output = typeof task.output === "string" && task.output.trim() ? task.output : namer.pathFor(task.agent, label);
  task.outputMode = "file-only";
  task.progress = false;
}

export function applyChainOutputDefaults(chain: ChainOptions["chain"], namer: OutputNamer): void {
  for (const step of chain) {
    const parallel = step.parallel;
    if (Array.isArray(parallel)) {
      for (const item of parallel) {
        if (isChainTask(item)) applyTaskOutputDefaults(item, namer);
      }
      continue;
    }
    if (isChainTask(step)) applyTaskOutputDefaults(step, namer);
    if (isDynamicParallelTemplate(parallel)) applyTaskOutputDefaults(parallel, namer);
  }
}

export function isChainTask(value: unknown): value is SubagentTask {
  return Boolean(value)
    && typeof value === "object"
    && !Array.isArray(value)
    && typeof (value as { agent?: unknown }).agent === "string"
    && typeof (value as { task?: unknown }).task === "string";
}

function isDynamicParallelTemplate(value: unknown): value is SubagentTask {
  return isChainTask(value);
}

function outputSuffix(): string {
  const fromEnv = process.env.PI_WORKERS_OUTPUT_SUFFIX;
  if (fromEnv && /^[a-zA-Z0-9_-]+$/.test(fromEnv)) return fromEnv.slice(0, 24);
  return crypto.randomUUID().replace(/-/g, "").slice(0, 8);
}

function slug(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 72)
    .replace(/-+$/g, "");
}

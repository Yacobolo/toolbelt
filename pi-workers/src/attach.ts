import * as fs from "node:fs";
import * as path from "node:path";
import { PiWorkersError } from "./client.js";
import type { CommandResult } from "./types.js";

interface AttachOptions {
  json: boolean;
  write(text: string): void;
  status(runId: string): Promise<CommandResult>;
}

interface AttachTarget {
  runId: string;
  asyncDir: string;
}

const terminalStates = new Set(["complete", "completed", "failed", "paused", "cancelled", "canceled", "interrupted"]);

export async function attachToAsyncRun(launch: CommandResult, options: AttachOptions): Promise<CommandResult> {
  const target = attachTarget(launch);
  if (!target) {
    throw new PiWorkersError("Attach requires an async Pi subagent result with details.runId and details.asyncDir.");
  }

  const statusPath = path.join(target.asyncDir, "status.json");
  await waitForFile(statusPath);

  let lastSummary = "";
  while (true) {
    const status = readStatus(statusPath);
    if (status) {
      const summary = statusSummary(status);
      if (summary !== lastSummary) {
        lastSummary = summary;
        writeAttachStatus(status, summary, target, options);
      }
      if (isTerminal(status.state)) break;
    }
    await waitForStatusChange(statusPath);
  }

  const final = await options.status(target.runId);
  writeAttachFinal(final, options, launch);
  return final;
}

function attachTarget(result: CommandResult): AttachTarget | undefined {
  const details = objectValue(result.details);
  const runId = stringValue(details?.runId) ?? stringValue(details?.asyncId);
  const asyncDir = stringValue(details?.asyncDir);
  return runId && asyncDir ? { runId, asyncDir } : undefined;
}

function writeAttachStatus(status: Record<string, unknown>, summary: string, target: AttachTarget, options: AttachOptions): void {
  const state = stringValue(status.state) ?? "unknown";
  const terminal = isTerminal(state);
  if (options.json) {
    options.write(`${JSON.stringify({
      type: "attach.status",
      runId: target.runId,
      state,
      terminal,
      summary,
      ts: new Date().toISOString(),
    })}\n`);
    return;
  }
  options.write(`[attach] ${summary}\n`);
}

function writeAttachFinal(result: CommandResult, options: AttachOptions, launch: CommandResult): void {
  if (options.json) {
    options.write(`${JSON.stringify({
      type: "attach.final",
      callId: result.callId,
      params: result.params,
      text: result.text,
      isError: result.isError,
      state: result.state,
      terminal: result.terminal,
      details: result.details,
      ts: new Date().toISOString(),
    })}\n`);
    return;
  }
  const summary = finalSummary(result, launch);
  options.write(`\n[attach] ${summary.state ?? "complete"}${summary.runId ? ` ${shortId(summary.runId)}` : ""}\n`);
  if (summary.outputs.length > 0) {
    options.write("Outputs:\n");
    for (const output of summary.outputs) {
      options.write(`  ${output.label ? `${output.label}: ` : ""}${toDisplayPath(output.path)}\n`);
    }
  }
  if (summary.resultPath || summary.logPath) {
    options.write(`${summary.outputs.length > 0 ? "\n" : ""}Trace:\n`);
    if (summary.resultPath) options.write(`  Result: ${toTracePath(summary.resultPath)}\n`);
    if (summary.logPath) options.write(`  Log:    ${toTracePath(summary.logPath)}\n`);
  }
}

function statusSummary(status: Record<string, unknown>): string {
  const state = stringValue(status.state) ?? "unknown";
  const steps = Array.isArray(status.steps) ? status.steps : [];
  const done = steps.filter((step) => {
    const value = stringValue(objectValue(step)?.status);
    return value === "complete" || value === "completed";
  }).length;
  const running = steps.filter((step) => stringValue(objectValue(step)?.status) === "running").length;
  const singleAgent = steps.length === 1 ? stringValue(objectValue(steps[0])?.agent) : undefined;
  if (steps.length === 0) return state;
  if (singleAgent) return `${state} ${singleAgent} · ${done}/${steps.length} done`;
  if (running > 0) return `${running === 1 ? "1 agent" : `${running} agents`} running · ${done}/${steps.length} done`;
  return `${state} · ${done}/${steps.length} done`;
}

interface FinalSummary {
  runId: string | undefined;
  state: string | undefined;
  resultPath: string | undefined;
  logPath: string | undefined;
  outputs: OutputRef[];
}

interface OutputRef {
  path: string;
  label?: string;
}

function finalSummary(result: CommandResult, launch: CommandResult): FinalSummary {
  const resultPath = lineValue(result.text, "Result");
  const resultOutputs = resultPath ? outputPathsFromResult(resultPath) : [];
  const expectedOutputs = outputPathsFromParams(launch.params);
  return {
    runId: lineValue(result.text, "Run") ?? asyncRunId(result),
    state: lineValue(result.text, "State") ?? result.state,
    resultPath,
    logPath: lineValue(result.text, "Log"),
    outputs: uniqueOutputs([...resultOutputs, ...expectedOutputs]),
  };
}

function outputPathsFromResult(resultPath: string): OutputRef[] {
  const outputs = new Map<string, OutputRef>();
  try {
    const parsed = JSON.parse(fs.readFileSync(resultPath, "utf-8"));
    collectOutputPaths(parsed, outputs);
  } catch {
    return [];
  }
  return [...outputs.values()];
}

function collectOutputPaths(value: unknown, outputs: Map<string, OutputRef>): void {
  if (typeof value === "string") {
    const re = /Output saved to:\s*(.+?)(?:\s+\(|$)/g;
    for (const match of value.matchAll(re)) {
      const output = match[1]?.trim();
      if (output) outputs.set(output, { path: output });
    }
    return;
  }
  if (Array.isArray(value)) {
    for (const item of value) collectOutputPaths(item, outputs);
    return;
  }
  const object = objectValue(value);
  if (!object) return;
  for (const item of Object.values(object)) collectOutputPaths(item, outputs);
}

function outputPathsFromParams(params: unknown): OutputRef[] {
  const outputs = new Map<string, OutputRef>();
  collectParamOutputs(params, outputs);
  return [...outputs.values()];
}

function collectParamOutputs(value: unknown, outputs: Map<string, OutputRef>): void {
  if (Array.isArray(value)) {
    for (const item of value) collectParamOutputs(item, outputs);
    return;
  }
  const object = objectValue(value);
  if (!object) return;
  const output = stringValue(object.output);
  const label = stringValue(object.label);
  if (output) outputs.set(output, label ? { path: output, label } : { path: output });
  collectParamOutputs(object.tasks, outputs);
  collectParamOutputs(object.chain, outputs);
  collectParamOutputs(object.parallel, outputs);
}

function lineValue(text: string, label: string): string | undefined {
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = text.match(new RegExp(`^${escaped}:\\s*(.+)$`, "m"));
  return match?.[1]?.trim() || undefined;
}

function asyncRunId(result: CommandResult): string | undefined {
  const details = objectValue(result.details);
  return stringValue(details?.runId) ?? stringValue(details?.asyncId);
}

function shortId(id: string): string {
  return id.length > 8 ? id.slice(0, 8) : id;
}

function toDisplayPath(file: string): string {
  if (!path.isAbsolute(file)) return file;
  const relative = path.relative(process.cwd(), file);
  return relative && !relative.startsWith("..") && !path.isAbsolute(relative) ? relative : file;
}

function toTracePath(file: string): string {
  return file.replace(/^\/var\/folders\/[^/]+\/[^/]+\/T\//, "/var/.../");
}

function uniqueOutputs(values: OutputRef[]): OutputRef[] {
  const outputs = new Map<string, OutputRef>();
  for (const value of values) {
    const existing = outputs.get(value.path);
    outputs.set(value.path, existing?.label ? existing : value);
  }
  return [...outputs.values()];
}

function waitForFile(file: string): Promise<void> {
  if (fs.existsSync(file)) return Promise.resolve();
  return waitForStatusChange(file);
}

function waitForStatusChange(file: string): Promise<void> {
  return new Promise((resolve) => {
    const dir = path.dirname(file);
    const basename = path.basename(file);
    let close: fs.FSWatcher | undefined;
    let fallback: NodeJS.Timeout | undefined;
    const finish = () => {
      if (fallback) clearTimeout(fallback);
      close?.close();
      resolve();
    };
    close = fs.existsSync(dir)
      ? fs.watch(dir, (_eventType, changed) => {
        if (!changed || changed.toString() === basename) finish();
      })
      : undefined;
    fallback = setTimeout(finish, 10_000);
  });
}

function readStatus(file: string): Record<string, unknown> | undefined {
  try {
    return JSON.parse(fs.readFileSync(file, "utf-8")) as Record<string, unknown>;
  } catch {
    return undefined;
  }
}

function isTerminal(value: unknown): boolean {
  const state = stringValue(value);
  return state ? terminalStates.has(state.toLowerCase()) : false;
}

function objectValue(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : undefined;
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}

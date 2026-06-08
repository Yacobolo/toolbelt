import * as fs from "node:fs";
import * as path from "node:path";
import { PiWorkersError } from "./client.js";
import type { CommandResult, WaitOptions } from "./types.js";

interface WaitForRunOptions extends WaitOptions {
  json: boolean;
  write(text: string): void;
  status(id: string): Promise<CommandResult>;
  intervalMs?: number;
}

const terminalStates = new Set(["complete", "completed", "failed", "paused", "cancelled", "canceled", "interrupted"]);

export async function waitForRun(options: WaitForRunOptions): Promise<CommandResult> {
  const started = Date.now();
  let lastSummary = "";

  while (true) {
    const result = await options.status(options.id);
    if (result.terminal || isTerminal(result.state)) return result;

    if (options.progress) {
      const summary = statusTextSummary(result.text) ?? result.state ?? "running";
      if (summary !== lastSummary) {
        lastSummary = summary;
        writeProgress(options, result, summary);
      }
    }

    const remaining = remainingTimeout(started, options.timeoutMs);
    if (remaining !== undefined && remaining <= 0) {
      throw new PiWorkersError(`Timed out waiting for Pi subagent run '${options.id}'.`);
    }

    const statusPath = statusPathFromText(result.text);
    await waitForNextCheck(statusPath, remaining, options.intervalMs);
  }
}

function writeProgress(options: WaitForRunOptions, result: CommandResult, summary: string): void {
  if (options.json) {
    options.write(`${JSON.stringify({
      type: "wait.progress",
      id: options.id,
      state: result.state,
      terminal: result.terminal,
      summary,
      ts: new Date().toISOString(),
    })}\n`);
    return;
  }
  options.write(`[wait] ${summary}\n`);
}

function statusTextSummary(text: string): string | undefined {
  const state = lineValue(text, "State");
  const progress = lineValue(text, "Progress");
  if (state && progress) return `${state.toLowerCase()} · ${progress}`;
  return state ?? progress;
}

function statusPathFromText(text: string): string | undefined {
  const dir = lineValue(text, "Dir");
  if (!dir) return undefined;
  return path.join(dir, "status.json");
}

function lineValue(text: string, label: string): string | undefined {
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = text.match(new RegExp(`^${escaped}:\\s*(.+)$`, "m"));
  return match?.[1]?.trim() || undefined;
}

function remainingTimeout(started: number, timeoutMs: number | undefined): number | undefined {
  if (timeoutMs === undefined) return undefined;
  return timeoutMs - (Date.now() - started);
}

async function waitForNextCheck(statusPath: string | undefined, remaining: number | undefined, intervalMs = 5_000): Promise<void> {
  const timeout = Math.max(1, Math.min(remaining ?? intervalMs, intervalMs));
  if (statusPath && fs.existsSync(statusPath)) {
    await waitForStatusChange(statusPath, timeout);
    return;
  }
  await sleep(Math.min(timeout, 5_000));
}

function waitForStatusChange(file: string, fallbackMs: number): Promise<void> {
  return new Promise((resolve) => {
    const dir = path.dirname(file);
    const basename = path.basename(file);
    let close: fs.FSWatcher | undefined;
    const fallback = setTimeout(finish, fallbackMs);

    function finish(): void {
      clearTimeout(fallback);
      close?.close();
      resolve();
    }

    close = fs.watch(dir, (_eventType, changed) => {
      if (!changed || changed.toString() === basename) finish();
    });
  });
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isTerminal(value: unknown): boolean {
  return typeof value === "string" && terminalStates.has(value.toLowerCase());
}

import type { CommandResult, SubagentParams, SubagentToolResult } from "./types.js";

const terminalStates = new Set(["complete", "completed", "failed", "paused", "cancelled", "canceled", "interrupted"]);

export function normalizeResult(callId: string, params: SubagentParams, raw: SubagentToolResult): CommandResult {
  const text = extractText(raw);
  const state = extractState(text);
  const isError = (raw as { isError?: boolean }).isError === true;
  return {
    callId,
    params,
    text,
    isError,
    ...(state ? { state } : {}),
    terminal: isError || (state ? terminalStates.has(state.toLowerCase()) : false),
    ...(raw.details !== undefined ? { details: raw.details } : {}),
    raw,
  };
}

export function extractText(raw: SubagentToolResult): string {
  const content = raw.content;
  if (!Array.isArray(content)) return "";
  return content.map((part) => {
    if (!part || typeof part !== "object") return "";
    if ("text" in part && typeof part.text === "string") return part.text;
    return JSON.stringify(part);
  }).filter(Boolean).join("\n");
}

export function extractState(text: string): string | undefined {
  const match = text.match(/^State:\s*([A-Za-z_-]+)/m);
  return match?.[1];
}

export function summarizeProgress(raw: SubagentToolResult): string[] {
  const details = raw.details;
  if (!details || typeof details !== "object" || !("progress" in details) || !Array.isArray(details.progress)) {
    return [];
  }
  const lines: string[] = [];
  for (const item of details.progress) {
    if (!item || typeof item !== "object") continue;
    const record = item as Record<string, unknown>;
    const agent = stringValue(record.agent) ?? "subagent";
    const status = stringValue(record.status) ?? "running";
    const currentTool = stringValue(record.currentTool);
    const currentPath = stringValue(record.currentPath);
    const toolText = currentTool ? ` tool=${currentTool}${currentPath ? ` path=${currentPath}` : ""}` : "";
    lines.push(`${agent} ${status}${toolText}`);
  }
  return lines;
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}

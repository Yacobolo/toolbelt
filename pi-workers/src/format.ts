import type { CommandResult } from "./types.js";

export function formatHuman(result: CommandResult): string {
  return result.text.trimEnd();
}

export function formatJSON(result: CommandResult): string {
  return JSON.stringify({
    callId: result.callId,
    params: result.params,
    text: result.text,
    isError: result.isError,
    state: result.state,
    terminal: result.terminal,
    details: result.details,
  }, null, 2);
}

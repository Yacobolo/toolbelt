#!/usr/bin/env node

import { attachToAsyncRun } from "./attach.js";
import { createProgram, type CommandInvocation } from "./cli-args.js";
import { PiWorkersError, createSubagentClient } from "./client.js";
import { formatHuman, formatJSON } from "./format.js";
import { summarizeProgress } from "./result.js";
import type { CommandResult } from "./types.js";
import { waitForRun } from "./wait.js";

export async function main(argv = process.argv.slice(2)): Promise<number> {
  const program = createProgram(runInvocation);
  if (argv.length === 0) {
    program.outputHelp();
    return 0;
  }
  await program.parseAsync(argv, { from: "user" });
  return typeof process.exitCode === "number" ? process.exitCode : 0;
}

async function runInvocation(invocation: CommandInvocation): Promise<number> {
  let client: ReturnType<typeof createSubagentClient> | undefined;
  try {
    if (invocation.command === "help") {
      process.stdout.write("Run 'pi-workers --help' or 'pi-workers help <command>'.\n");
      return 0;
    }
    if (invocation.command === "version") {
      process.stdout.write("pi-workers 0.1.0\n");
      return 0;
    }

    client = createSubagentClient(clientOptions(invocation.cwd));
    const onUpdate = invocation.json ? undefined : (update: unknown) => {
      for (const line of summarizeProgress(update as never)) {
        process.stderr.write(`${line}\n`);
      }
    };
    const progressOptions = onUpdate === undefined ? {} : { onUpdate };

    if (invocation.command === "wait") {
      const final = await waitForRun({
        ...invocation.options,
        json: invocation.json,
        write: (text) => process.stdout.write(text),
        status: (id) => client!.status({ id }),
      });
      process.stdout.write(`${invocation.json ? formatJSON(final) : formatHuman(final)}\n`);
      return final.isError ? 1 : 0;
    }

    const result = await runMirrorCommand(client, invocation, progressOptions);
    if (shouldAttach(invocation) && !result.isError) {
      writeLaunch(result, invocation.json);
      const final = await attachToAsyncRun(result, {
        json: invocation.json,
        write: (text) => process.stdout.write(text),
        status: (id) => client!.status({ id }),
      });
      return final.isError ? 1 : 0;
    }
    process.stdout.write(`${invocation.json ? formatJSON(result) : formatHuman(result)}\n`);
    return result.isError ? 1 : 0;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    process.stderr.write(`${message}\nRun 'pi-workers --help' for usage.\n`);
    if (error instanceof PiWorkersError) return 2;
    return 1;
  } finally {
    await client?.close();
  }
}

async function runMirrorCommand(
  client: ReturnType<typeof createSubagentClient>,
  invocation: CommandInvocation,
  progressOptions: { onUpdate?: (update: unknown) => void },
): Promise<CommandResult> {
  switch (invocation.command) {
    case "doctor":
      return client.doctor();
    case "list":
      return client.list();
    case "get":
      return client.get(invocation.options);
    case "create":
      return client.create(invocation.options);
    case "update":
      return client.update(invocation.options);
    case "delete":
      return client.delete(invocation.options);
    case "run":
      return client.spawn(invocation.options, progressOptions);
    case "parallel":
      return client.parallel(invocation.options, progressOptions);
    case "chain":
      return client.chain(invocation.options, progressOptions);
    case "status":
      return client.status(invocation.id === undefined ? {} : { id: invocation.id });
    case "interrupt":
      return client.interrupt(invocation.id === undefined ? {} : { id: invocation.id });
    case "resume":
      return client.resume(invocation.options);
    case "wait":
    case "help":
    case "version":
      throw new Error(`Command ${invocation.command} is handled before dispatch.`);
    default:
      return unreachable(invocation);
  }
}

function unreachable(value: never): never {
  throw new Error(`Unhandled command: ${JSON.stringify(value)}`);
}

function clientOptions(cwd: string | undefined): { cwd?: string } {
  return cwd === undefined ? {} : { cwd };
}

function shouldAttach(invocation: CommandInvocation): boolean {
  return (
    (invocation.command === "run" || invocation.command === "parallel" || invocation.command === "chain")
    && invocation.attach === true
  );
}

function writeLaunch(result: CommandResult, json: boolean): void {
  const runId = asyncRunId(result);
  if (json) {
    process.stdout.write(`${JSON.stringify({
      type: "attach.launch",
      callId: result.callId,
      params: result.params,
      isError: result.isError,
      state: result.state,
      terminal: result.terminal,
      runId,
      details: result.details,
      ts: new Date().toISOString(),
    })}\n`);
    return;
  }
  const label = launchLabel(result);
  process.stdout.write(`[attach] launched${runId ? ` ${shortId(runId)}` : ""}${label ? ` ${label}` : ""}\n`);
}

function asyncRunId(result: CommandResult): string | undefined {
  const details = result.details;
  if (!details || typeof details !== "object" || Array.isArray(details)) return undefined;
  const record = details as Record<string, unknown>;
  const runId = record.runId ?? record.asyncId;
  return typeof runId === "string" && runId.trim() ? runId : undefined;
}

function launchLabel(result: CommandResult): string | undefined {
  const details = objectValue(result.details);
  const params = objectValue(result.params);
  const mode = stringValue(details?.mode);
  if (mode && mode !== "single") return mode;
  const agent = stringValue(params?.agent);
  if (agent) return agent;
  if (Array.isArray(params?.tasks)) return "parallel";
  if (Array.isArray(params?.chain)) return "chain";
  return mode;
}

function shortId(id: string): string {
  return id.length > 8 ? id.slice(0, 8) : id;
}

function objectValue(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : undefined;
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const exitCode = await main();
  process.exit(exitCode);
}

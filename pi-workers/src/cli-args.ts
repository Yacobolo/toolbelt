import { Command, InvalidArgumentError } from "commander";
import * as fs from "node:fs";
import {
  applyChainOutputDefaults,
  applyTaskOutputDefaults,
  createOutputNamer,
  normalizeLabel,
  requireLabel,
  type OutputNamer,
} from "./launch-defaults.js";
import type {
  ChainOptions,
  CreateOptions,
  DeleteOptions,
  GetOptions,
  ParallelOptions,
  ResumeOptions,
  SpawnOptions,
  SubagentTask,
  UpdateOptions,
  WaitOptions,
} from "./types.js";

export type CommandInvocation =
  | { command: "doctor"; json: boolean; cwd?: string }
  | { command: "list"; json: boolean; cwd?: string }
  | { command: "get"; json: boolean; cwd?: string; options: GetOptions }
  | { command: "create"; json: boolean; cwd?: string; options: CreateOptions }
  | { command: "update"; json: boolean; cwd?: string; options: UpdateOptions }
  | { command: "delete"; json: boolean; cwd?: string; options: DeleteOptions }
  | { command: "run"; json: boolean; cwd?: string; attach: boolean; options: SpawnOptions }
  | { command: "parallel"; json: boolean; cwd?: string; attach: boolean; options: ParallelOptions }
  | { command: "chain"; json: boolean; cwd?: string; attach: boolean; options: ChainOptions }
  | { command: "status"; json: boolean; cwd?: string; id?: string }
  | { command: "wait"; json: boolean; cwd?: string; options: WaitOptions }
  | { command: "interrupt"; json: boolean; cwd?: string; id?: string }
  | { command: "resume"; json: boolean; cwd?: string; options: ResumeOptions }
  | { command: "help"; json: boolean; cwd?: string; topic?: string }
  | { command: "version"; json: boolean; cwd?: string };

interface GlobalOptions {
  cwd?: string;
  json?: boolean;
}

export function parseArgs(argv: string[]): CommandInvocation {
  let invocation: CommandInvocation | undefined;
  const program = buildProgram((next) => {
    invocation = next;
  });
  overrideExit(program);
  silenceOutput(program);
  program.parse(argv, { from: "user" });
  return invocation ?? withGlobals({ command: "help" }, program);
}

export function createProgram(run: (invocation: CommandInvocation) => Promise<number> | number): Command {
  const program = buildProgram(async (invocation) => {
    try {
      process.exitCode = await run(invocation);
    } catch (error) {
      process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
      process.exitCode = 1;
    }
  });
  program.showHelpAfterError();
  return program;
}

function buildProgram(setInvocation: (invocation: CommandInvocation) => void | Promise<void>): Command {
  const program = new Command();
  program
    .name("pi-workers")
    .description("Codex-friendly CLI for opinionated Pi subagent orchestration")
    .version("0.1.0")
    .option("-C, --cwd <dir>", "run from another working directory")
    .option("-j, --json", "print machine-readable JSON")
    .addHelpCommand("help [command]", "show command-specific help")
    .showSuggestionAfterError();

  program.command("doctor")
    .description("mirror subagent({ action: \"doctor\" })")
    .action(() => setInvocation(withGlobals({ command: "doctor" }, program)));

  program.command("list")
    .aliases(["ls", "agents"])
    .description("mirror subagent({ action: \"list\" })")
    .action(() => setInvocation(withGlobals({ command: "list" }, program)));

  program.command("get")
    .description("mirror subagent({ action: \"get\", agent|chainName })")
    .argument("<name>", "agent or chain name")
    .option("--agent", "target an agent")
    .option("--chain", "target a chain")
    .action((name: string, options: TargetOptions) => {
      return setInvocation(withGlobals({ command: "get", options: targetOptions(name, options) }, program));
    });

  program.command("create")
    .description("mirror subagent({ action: \"create\", config })")
    .requiredOption("--config <json-or-file>", "agent or chain config JSON, or path to JSON file")
    .action((options: ConfigCommandOptions) => {
      return setInvocation(withGlobals({ command: "create", options: { config: parseJSONOrFile(options.config, "--config") } }, program));
    });

  program.command("update")
    .description("mirror subagent({ action: \"update\", agent|chainName, config })")
    .argument("<name>", "agent or chain name")
    .option("--agent", "target an agent")
    .option("--chain", "target a chain")
    .requiredOption("--config <json-or-file>", "agent or chain config JSON, or path to JSON file")
    .action((name: string, options: TargetOptions & ConfigCommandOptions) => {
      return setInvocation(withGlobals({
        command: "update",
        options: { ...targetOptions(name, options), config: parseJSONOrFile(options.config, "--config") },
      }, program));
    });

  program.command("delete")
    .alias("rm")
    .description("mirror subagent({ action: \"delete\", agent|chainName })")
    .argument("<name>", "agent or chain name")
    .option("--agent", "target an agent")
    .option("--chain", "target a chain")
    .action((name: string, options: TargetOptions) => {
      return setInvocation(withGlobals({ command: "delete", options: targetOptions(name, options) }, program));
    });

  program.command("run")
    .description("launch one worker with Codex defaults: fresh context, async, file-only output, attached")
    .argument("[agent]", "worker role, such as reviewer, worker, scout")
    .argument("[task...]", "task text")
    .option("-a, --agent <name>", "worker role, such as reviewer, worker, scout")
    .option("-t, --task <text>", "task text")
    .option("-b, --background", "launch async and return immediately instead of attaching")
    .option("-m, --model <model>", "set model")
    .option("--label <label>", "artifact label for generated output and attach receipts")
    .option("--output <path>", "exact output file path")
    .option("--output-dir <dir>", "directory for generated output files", ".pi-agents")
    .option("--reads <files>", "comma-separated files for reads")
    .option("--no-reads", "set reads: false")
    .option("--skill <name>", "set skill; repeatable", collect, [])
    .option("--acceptance-json <json>", "set acceptance from raw JSON")
    .option("--output-schema <json-or-file>", "set outputSchema from JSON or file")
    .addHelpText("after", `
Examples:
  $ pi-workers run scout "Map the auth flow" --label auth-map
  $ pi-workers run reviewer "Review the current diff. Do not edit files." --label diff-review --background`)
    .action((agentArg: string | undefined, taskArg: string[], options: RunCommandOptions) => {
      return setInvocation(withGlobals({ command: "run", attach: options.background !== true, options: runOptions(agentArg, taskArg, options) }, program));
    });

  program.command("parallel")
    .description("launch parallel workers with Codex defaults: fresh context, async, file-only outputs, attached")
    .argument("[tasks...]", "worker tasks as label=agent:task")
    .option("--task <label=agent:task>", "worker task as label=agent:task", collect, [])
    .option("-b, --background", "launch async and return immediately instead of attaching")
    .option("--output-dir <dir>", "directory for generated output files", ".pi-agents")
    .option("--concurrency <n>", "set concurrency", parsePositiveInteger)
    .option("--worktree", "set worktree: true")
    .option("--reads <files>", "set reads on every task")
    .option("--no-reads", "set reads: false on every task")
    .option("--skill <name>", "set skill on every task; repeatable", collect, [])
    .option("--acceptance-json <json>", "set top-level acceptance from raw JSON")
    .option("--control-json <json>", "set control from raw JSON")
    .addHelpText("after", `
Examples:
  $ pi-workers parallel correctness=reviewer:"Review correctness" tests=reviewer:"Review tests"
  $ pi-workers parallel --task tests=reviewer:"Review tests" --task challenge=oracle:"Challenge plan" --background`)
    .action((taskArgs: string[], options: ParallelCommandOptions) => {
      return setInvocation(withGlobals({ command: "parallel", attach: options.background !== true, options: parallelOptions(taskArgs, options) }, program));
    });

  const chainItems: ChainInlineItem[] = [];
  program.command("chain")
    .description("launch a chain with Codex defaults: fresh context, async, file-only outputs, attached")
    .option("-f, --file <path>", "JSON file containing a chain array or { chain: [...] }")
    .option("--step <label=agent:task>", "append one sequential chain step", (value) => {
      chainItems.push({ kind: "step", value });
      return value;
    })
    .option("--group <json>", "append one parallel group as a JSON array", (value) => {
      chainItems.push({ kind: "group", value });
      return value;
    })
    .option("-b, --background", "launch async and return immediately instead of attaching")
    .option("--output-dir <dir>", "directory for generated output files", ".pi-agents")
    .option("--task <text>", "set top-level task")
    .option("--chain-dir <dir>", "set chainDir")
    .option("--concurrency <n>", "set concurrency", parsePositiveInteger)
    .option("--acceptance-json <json>", "set acceptance from raw JSON")
    .option("--control-json <json>", "set control from raw JSON")
    .addHelpText("after", `
Examples:
  $ pi-workers chain --file workflow.chain.json
  $ pi-workers chain --step auth-map=scout:"Map auth" --group '[{"label":"correctness","agent":"reviewer","task":"Review correctness"}]'`)
    .action((options: ChainCommandOptions) => {
      return setInvocation(withGlobals({ command: "chain", attach: options.background !== true, options: chainOptions(options, chainItems) }, program));
    });

  program.command("status")
    .alias("ps")
    .description("mirror subagent({ action: \"status\", id? })")
    .argument("[id]", "run id or prefix")
    .option("-i, --id <id>", "run id or prefix")
    .action((idArg: string | undefined, options: IDOptions) => {
      return setInvocation(withGlobals(optionalIDInvocation("status", options.id ?? idArg), program));
    });

  program.command("wait")
    .description("wait for an async run to finish, then print one final native status result")
    .argument("[id]", "run id or prefix")
    .option("-i, --id <id>", "run id or prefix")
    .option("--timeout <duration>", "maximum wait, such as 30s, 10m, or 1h", parseDurationMs)
    .option("--progress", "print compact progress changes while waiting")
    .addHelpText("after", `
Examples:
  $ pi-workers wait 268e9162
  $ pi-workers wait 268e9162 --timeout 10m --progress`)
    .action((idArg: string | undefined, options: WaitCommandOptions) => {
      const id = requireString(options.id ?? idArg, "wait requires an id");
      return setInvocation(withGlobals({ command: "wait", options: waitOptions(id, options) }, program));
    });

  program.command("interrupt")
    .alias("stop")
    .description("mirror subagent({ action: \"interrupt\", id? })")
    .argument("[id]", "run id or prefix")
    .option("-i, --id <id>", "run id or prefix")
    .action((idArg: string | undefined, options: IDOptions) => {
      return setInvocation(withGlobals(optionalIDInvocation("interrupt", options.id ?? idArg), program));
    });

  program.command("resume")
    .description("mirror subagent({ action: \"resume\", id, message, index? })")
    .argument("[id]", "run id or prefix")
    .argument("[message...]", "follow-up message")
    .option("-i, --id <id>", "run id or prefix")
    .option("--message <text>", "follow-up message")
    .option("--index <n>", "child index for multi-child runs", parseNonNegativeInteger)
    .action((idArg: string | undefined, messageArg: string[], options: ResumeCommandOptions) => {
      const id = requireString(options.id ?? idArg, "resume requires an id");
      const message = requireString(options.message ?? messageArg.join(" "), "resume requires a message");
      return setInvocation(withGlobals({ command: "resume", options: resumeOptions(id, message, options.index) }, program));
    });

  return program;
}

function overrideExit(command: Command): void {
  command.exitOverride();
  for (const child of command.commands) overrideExit(child);
}

function silenceOutput(command: Command): void {
  command.configureOutput({
    writeOut: () => {},
    writeErr: () => {},
  });
  for (const child of command.commands) silenceOutput(child);
}

interface TargetOptions {
  agent?: boolean;
  chain?: boolean;
}

interface ConfigCommandOptions {
  config: string;
}

interface LaunchCommandOptions {
  background?: boolean;
}

interface RunCommandOptions extends LaunchCommandOptions {
  agent?: string;
  task?: string;
  model?: string;
  label?: string;
  output?: string;
  outputDir?: string;
  reads?: string | false;
  skill?: string[];
  acceptanceJson?: string;
  outputSchema?: string;
}

interface ParallelCommandOptions extends LaunchCommandOptions {
  task?: string[];
  outputDir?: string;
  concurrency?: number;
  worktree?: boolean;
  reads?: string | false;
  skill?: string[];
  acceptanceJson?: string;
  controlJson?: string;
}

interface ChainCommandOptions extends LaunchCommandOptions {
  file?: string;
  outputDir?: string;
  task?: string;
  chainDir?: string;
  concurrency?: number;
  acceptanceJson?: string;
  controlJson?: string;
}

interface IDOptions {
  id?: string;
}

interface ResumeCommandOptions extends IDOptions {
  message?: string;
  index?: number;
}

interface WaitCommandOptions extends IDOptions {
  timeout?: number;
  progress?: boolean;
}

interface ChainInlineItem {
  kind: "step" | "group";
  value: string;
}

function targetOptions(name: string, options: TargetOptions): GetOptions | DeleteOptions {
  if (options.agent === true && options.chain === true) throw new InvalidArgumentError("choose only one of --agent or --chain");
  if (options.chain === true) return { chainName: name };
  if (options.agent === true) return { agent: name };
  throw new InvalidArgumentError("choose --agent or --chain");
}

function runOptions(agentArg: string | undefined, taskArg: string[], options: RunCommandOptions): SpawnOptions {
  const agent = requireString(options.agent ?? agentArg, "run requires an agent");
  const task = requireString(options.task ?? taskArg.join(" "), "run requires a task");
  const label = requireLabel(options.label, "run requires --label");
  const namer = createOutputNamer(requireOutputDir(options.outputDir));
  const output: SpawnOptions = {
    agent,
    task,
    async: true,
    label,
    output: options.output ?? namer.pathFor(agent, label),
    outputMode: "file-only",
    progress: false,
  };
  assignIf(output, "model", options.model);
  assignIf(output, "reads", parseReads(options.reads));
  assignIf(output, "skill", options.skill?.length ? options.skill : undefined);
  assignIf(output, "acceptance", parseJSONOption(options.acceptanceJson, "--acceptance-json"));
  assignIf(output, "outputSchema", parseJSONOrFile(options.outputSchema, "--output-schema"));
  return output;
}

function parallelOptions(taskArgs: string[], options: ParallelCommandOptions): ParallelOptions {
  const rawTasks = [...(options.task ?? []), ...taskArgs];
  if (rawTasks.length === 0) throw new InvalidArgumentError("parallel requires at least one task");
  const reads = parseReads(options.reads);
  const skill = options.skill?.length ? options.skill : undefined;
  const namer = createOutputNamer(requireOutputDir(options.outputDir));
  const output: ParallelOptions = {
    async: true,
    tasks: rawTasks.map((token) => {
      const task = parseTaskToken(token, true);
      applyTaskOutputDefaults(task, namer);
      assignIf(task, "reads", reads);
      assignIf(task, "skill", skill);
      return task;
    }),
  };
  assignIf(output, "concurrency", options.concurrency);
  assignIf(output, "worktree", options.worktree ? true : undefined);
  assignIf(output, "acceptance", parseJSONOption(options.acceptanceJson, "--acceptance-json"));
  assignIf(output, "control", parseJSONOption(options.controlJson, "--control-json"));
  return output;
}

function chainOptions(options: ChainCommandOptions, items: ChainInlineItem[]): ChainOptions {
  const chain = options.file ? readChainFile(options.file) : items.map(parseChainInlineItem);
  if (chain.length === 0) throw new InvalidArgumentError("chain requires --file or at least one --step/--group");
  const namer = createOutputNamer(requireOutputDir(options.outputDir));
  const output: ChainOptions = { chain, async: true };
  applyChainOutputDefaults(output.chain, namer);
  assignIf(output, "task", options.task);
  assignIf(output, "chainDir", options.chainDir);
  assignIf(output, "concurrency", options.concurrency);
  assignIf(output, "acceptance", parseJSONOption(options.acceptanceJson, "--acceptance-json"));
  assignIf(output, "control", parseJSONOption(options.controlJson, "--control-json"));
  return output;
}

function optionalIDInvocation(command: "status" | "interrupt", id: string | undefined): { command: "status" | "interrupt"; id?: string } {
  return id === undefined ? { command } : { command, id };
}

function resumeOptions(id: string, message: string, index: number | undefined): ResumeOptions {
  const output: ResumeOptions = { id, message };
  assignIf(output, "index", index);
  return output;
}

function waitOptions(id: string, options: WaitCommandOptions): WaitOptions {
  const output: WaitOptions = { id };
  assignIf(output, "timeoutMs", options.timeout);
  assignIf(output, "progress", options.progress ? true : undefined);
  return output;
}

function parseTaskToken(raw: string, requireExplicitLabel = false): SubagentTask {
  const split = raw.indexOf(":");
  if (split <= 0 || split === raw.length - 1) {
    throw new InvalidArgumentError(`Invalid task '${raw}'. Use label=agent:task.`);
  }
  const labelSplit = raw.indexOf("=");
  const label = labelSplit > 0 && labelSplit < split
    ? normalizeLabel(raw.slice(0, labelSplit), "task label")
    : undefined;
  if (requireExplicitLabel && !label) throw new InvalidArgumentError(`Invalid task '${raw}'. Use label=agent:task.`);
  const agentStart = label === undefined ? 0 : labelSplit + 1;
  return {
    ...(label ? { label } : {}),
    agent: raw.slice(agentStart, split),
    task: raw.slice(split + 1),
  };
}

function parseChainInlineItem(item: ChainInlineItem): Record<string, unknown> {
  if (item.kind === "step") return parseTaskToken(item.value, true) as unknown as Record<string, unknown>;
  const parsed = parseJSONOption(item.value, "--group");
  if (!Array.isArray(parsed)) throw new InvalidArgumentError("--group must be a JSON array");
  return { parallel: parsed };
}

function readChainFile(file: string): Record<string, unknown>[] {
  const parsed = parseJSONOption(fs.readFileSync(file, "utf-8"), "--file");
  const chain = Array.isArray(parsed)
    ? parsed
    : parsed && typeof parsed === "object" && Array.isArray((parsed as { chain?: unknown }).chain)
      ? (parsed as { chain: unknown[] }).chain
      : undefined;
  if (!chain) throw new InvalidArgumentError("--file must contain a chain array or an object with a chain array");
  return chain.map((item) => {
    if (!item || typeof item !== "object" || Array.isArray(item)) throw new InvalidArgumentError("chain items must be objects");
    return item as Record<string, unknown>;
  });
}

function parseNonNegativeInteger(value: string): number {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 0) throw new InvalidArgumentError("must be a non-negative integer");
  return parsed;
}

function parsePositiveInteger(value: string): number {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1) throw new InvalidArgumentError("must be a positive integer");
  return parsed;
}

function parseDurationMs(value: string): number {
  const match = value.trim().match(/^(\d+)(ms|s|m|h)?$/i);
  if (!match) throw new InvalidArgumentError("must be a duration such as 500ms, 30s, 10m, or 1h");
  const amount = Number(match[1]);
  if (!Number.isSafeInteger(amount) || amount < 1) throw new InvalidArgumentError("duration must be positive");
  const unit = match[2]?.toLowerCase() ?? "ms";
  const multiplier = unit === "ms" ? 1 : unit === "s" ? 1_000 : unit === "m" ? 60_000 : 3_600_000;
  return amount * multiplier;
}

function collect(value: string, previous: string[]): string[] {
  previous.push(value);
  return previous;
}

function parseReads(value: string | false | undefined): string[] | false | undefined {
  if (value === false) return false;
  if (!value) return undefined;
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}

function parseJSONOrFile(value: string | undefined, label: string): unknown {
  if (!value) return undefined;
  if (fs.existsSync(value)) return parseJSONOption(fs.readFileSync(value, "utf-8"), label);
  return parseJSONOption(value, label);
}

function parseJSONOption(value: string | undefined, label: string): unknown {
  if (value === undefined) return undefined;
  try {
    return JSON.parse(value);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new InvalidArgumentError(`${label} must be valid JSON: ${message}`);
  }
}

function requireString(value: string | undefined, message: string): string {
  if (value === undefined || value.trim() === "") throw new InvalidArgumentError(message);
  return value;
}

function assignIf<T extends object, K extends keyof T>(target: T, key: K, value: T[K] | undefined): void {
  if (value !== undefined) target[key] = value;
}

function requireOutputDir(value: string | undefined): string {
  const outputDir = value ?? ".pi-agents";
  if (!outputDir.trim()) throw new InvalidArgumentError("--output-dir must not be empty");
  return outputDir;
}

function withGlobals<T extends { command: CommandInvocation["command"] }>(value: T, program: Command): T & { json: boolean; cwd?: string } {
  const opts = program.opts<GlobalOptions>();
  return {
    ...value,
    json: opts.json === true,
    ...(opts.cwd ? { cwd: opts.cwd } : {}),
  };
}

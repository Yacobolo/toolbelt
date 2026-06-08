import assert from "node:assert/strict";
import test from "node:test";
import { createProgram, parseArgs } from "../dist/cli-args.js";

test("parses run command as native subagent params", () => {
  assert.deepEqual(parseArgs([
    "run",
    "--json",
    "-a",
    "reviewer",
    "-t",
    "Review diff",
    "--background",
    "--label",
    "diff-review",
    "--output",
    "review.md",
  ]), {
    command: "run",
    json: true,
    attach: false,
    options: {
      agent: "reviewer",
      task: "Review diff",
      label: "diff-review",
      async: true,
      output: "review.md",
      outputMode: "file-only",
      progress: false,
    },
  });
});

test("parses run shorthand and boolean native params", () => {
  assert.deepEqual(withOutputSuffix("fixed001", () => parseArgs([
    "run",
    "reviewer",
    "--label",
    "diff",
    "Review",
    "diff",
    "--no-reads",
  ])), {
    command: "run",
    json: false,
    attach: true,
    options: {
      agent: "reviewer",
      task: "Review diff",
      label: "diff",
      async: true,
      output: ".pi-agents/reviewer-diff-fixed001.md",
      outputMode: "file-only",
      progress: false,
      reads: false,
    },
  });
});

test("parses parallel repeated tasks", () => {
  assert.deepEqual(withOutputSuffix("fixed002", () => parseArgs([
    "parallel",
    "--task",
    "correctness=reviewer:Review correctness",
    "--task",
    "files=scout:Map files",
    "--concurrency",
    "2",
    "--worktree",
    "--control-json",
    "{\"enabled\":true}",
  ])), {
    command: "parallel",
    json: false,
    attach: true,
    options: {
      async: true,
      tasks: [
        {
          agent: "reviewer",
          task: "Review correctness",
          label: "correctness",
          output: ".pi-agents/reviewer-correctness-fixed002.md",
          outputMode: "file-only",
          progress: false,
        },
        {
          agent: "scout",
          task: "Map files",
          label: "files",
          output: ".pi-agents/scout-files-fixed002.md",
          outputMode: "file-only",
          progress: false,
        },
      ],
      concurrency: 2,
      worktree: true,
      control: { enabled: true },
    },
  });
});

test("parses chain inline steps and groups", () => {
  assert.deepEqual(withOutputSuffix("fixed003", () => parseArgs([
    "chain",
    "--step",
    "auth-map=scout:Map auth",
    "--group",
    "[{\"label\":\"correctness\",\"agent\":\"reviewer\",\"task\":\"Review correctness\"}]",
  ])), {
    command: "chain",
    json: false,
    attach: true,
    options: {
      chain: [
        {
          agent: "scout",
          task: "Map auth",
          label: "auth-map",
          output: ".pi-agents/scout-auth-map-fixed003.md",
          outputMode: "file-only",
          progress: false,
        },
        {
          parallel: [{
            agent: "reviewer",
            task: "Review correctness",
            label: "correctness",
            output: ".pi-agents/reviewer-correctness-fixed003.md",
            outputMode: "file-only",
            progress: false,
          }],
        },
      ],
      async: true,
    },
  });
});

test("parses native management actions", () => {
  assert.deepEqual(parseArgs(["get", "reviewer", "--agent"]), {
    command: "get",
    json: false,
    options: { agent: "reviewer" },
  });
  assert.deepEqual(parseArgs(["get", "review-loop", "--chain"]), {
    command: "get",
    json: false,
    options: { chainName: "review-loop" },
  });
  assert.deepEqual(parseArgs(["create", "--config", "{\"name\":\"x\"}"]), {
    command: "create",
    json: false,
    options: { config: { name: "x" } },
  });
  assert.deepEqual(parseArgs(["update", "reviewer", "--agent", "--config", "{\"model\":\"m\"}"]), {
    command: "update",
    json: false,
    options: { agent: "reviewer", config: { model: "m" } },
  });
  assert.deepEqual(parseArgs(["delete", "old-chain", "--chain"]), {
    command: "delete",
    json: false,
    options: { chainName: "old-chain" },
  });
});

test("launch helpers default to async attach and support background", () => {
  assert.deepEqual(withOutputSuffix("fixed004", () => parseArgs(["run", "reviewer", "Review diff", "--label", "diff"])), {
    command: "run",
    json: false,
    attach: true,
    options: {
      agent: "reviewer",
      task: "Review diff",
      label: "diff",
      async: true,
      output: ".pi-agents/reviewer-diff-fixed004.md",
      outputMode: "file-only",
      progress: false,
    },
  });
  assert.deepEqual(withOutputSuffix("fixed005", () => parseArgs(["parallel", "diff=reviewer:Review diff"])), {
    command: "parallel",
    json: false,
    attach: true,
    options: {
      tasks: [{
        agent: "reviewer",
        task: "Review diff",
        label: "diff",
        output: ".pi-agents/reviewer-diff-fixed005.md",
        outputMode: "file-only",
        progress: false,
      }],
      async: true,
    },
  });
  assert.deepEqual(withOutputSuffix("fixed006", () => parseArgs(["chain", "--step", "map=scout:Map"])), {
    command: "chain",
    json: false,
    attach: true,
    options: {
      chain: [{
        agent: "scout",
        task: "Map",
        label: "map",
        output: ".pi-agents/scout-map-fixed006.md",
        outputMode: "file-only",
        progress: false,
      }],
      async: true,
    },
  });
  assert.deepEqual(withOutputSuffix("fixed007", () => parseArgs(["run", "reviewer", "Review diff", "--background", "--label", "diff"])), {
    command: "run",
    json: false,
    attach: false,
    options: {
      agent: "reviewer",
      task: "Review diff",
      label: "diff",
      async: true,
      output: ".pi-agents/reviewer-diff-fixed007.md",
      outputMode: "file-only",
      progress: false,
    },
  });
});

test("launch helpers support output directory and duplicate slugs", () => {
  assert.deepEqual(withOutputSuffix("fixed008", () => parseArgs([
    "parallel",
    "--output-dir",
    ".pi-agents/reviews",
    "correctness=reviewer:Review diff",
    "correctness=reviewer:Review diff",
  ])), {
    command: "parallel",
    json: false,
    attach: true,
    options: {
      async: true,
      tasks: [
        {
          agent: "reviewer",
          task: "Review diff",
          label: "correctness",
          output: ".pi-agents/reviews/reviewer-correctness-fixed008.md",
          outputMode: "file-only",
          progress: false,
        },
        {
          agent: "reviewer",
          task: "Review diff",
          label: "correctness",
          output: ".pi-agents/reviews/reviewer-correctness-2-fixed008.md",
          outputMode: "file-only",
          progress: false,
        },
      ],
    },
  });
});

test("parses status interrupt and resume", () => {
  assert.deepEqual(parseArgs(["status", "abc"]), {
    command: "status",
    json: false,
    id: "abc",
  });
  assert.deepEqual(parseArgs(["wait", "abc", "--timeout", "10m", "--progress"]), {
    command: "wait",
    json: false,
    options: { id: "abc", timeoutMs: 600000, progress: true },
  });
  assert.deepEqual(parseArgs(["interrupt"]), {
    command: "interrupt",
    json: false,
  });
  assert.deepEqual(parseArgs(["resume", "abc", "try", "again", "--index", "1"]), {
    command: "resume",
    json: false,
    options: { id: "abc", message: "try again", index: 1 },
  });
});

test("generated help exposes only opinionated commands and options", () => {
  const program = createProgram(() => 0);
  const rootHelp = program.helpInformation();
  assert.match(rootHelp, /Usage: pi-workers/);
  assert.match(rootHelp, /doctor/);
  assert.match(rootHelp, /parallel/);
  assert.match(rootHelp, /wait/);
  assert.match(rootHelp, /create/);
  assert.doesNotMatch(rootHelp, /call/);
  assert.doesNotMatch(rootHelp, /inspect/);
  assert.doesNotMatch(rootHelp, /artifacts/);
  assert.doesNotMatch(rootHelp, /watch/);
  assert.doesNotMatch(rootHelp, /state-dir/);

  const runCommand = program.commands.find((command) => command.name() === "run");
  assert.ok(runCommand);
  const runHelp = runCommand.helpInformation();
  assert.match(runHelp, /--agent <name>/);
  assert.match(runHelp, /--task <text>/);
  assert.match(runHelp, /--background/);
  assert.match(runHelp, /--label <label>/);
  assert.match(runHelp, /--output <path>/);
  assert.match(runHelp, /--output-dir <dir>/);
  assert.doesNotMatch(runHelp, /--attach/);
  assert.doesNotMatch(runHelp, /--foreground/);
  assert.doesNotMatch(runHelp, /--async/);
  assert.doesNotMatch(runHelp, /--output-mode/);
  assert.doesNotMatch(runHelp, /--no-output/);
  assert.doesNotMatch(runHelp, /--progress/);
  assert.doesNotMatch(runHelp, /--context/);
  assert.doesNotMatch(runHelp, /--fork/);
  assert.doesNotMatch(runHelp, /--fresh/);
  assert.doesNotMatch(runHelp, /--safe-output/);
  assert.doesNotMatch(runHelp, /--pi-defaults/);
  assert.doesNotMatch(runHelp, /--id/);
});

test("rejects removed commands and flags", () => {
  for (const command of ["call", "runs", "inspect", "events", "logs", "artifacts", "watch"]) {
    assert.throws(() => parseArgs([command, "abc"]), /unknown command/i);
  }
  assert.throws(() => parseArgs(["--state-dir", ".state", "list"]), /unknown option/i);
  assert.throws(() => parseArgs(["run", "scout", "Map", "--safe-output"]), /unknown option/i);
  assert.throws(() => parseArgs(["run", "scout", "Map", "--pi-defaults"]), /unknown option/i);
  assert.throws(() => parseArgs(["run", "scout", "Map", "--id", "run-1"]), /unknown option/i);
  assert.throws(() => parseArgs(["run", "scout", "Map", "--attach"]), /unknown option/i);
  assert.throws(() => parseArgs(["run", "scout", "Map", "--async"]), /unknown option/i);
  assert.throws(() => parseArgs(["run", "scout", "Map", "--foreground"]), /unknown option/i);
  assert.throws(() => parseArgs(["run", "scout", "Map", "--output-mode", "inline"]), /unknown option/i);
  assert.throws(() => parseArgs(["run", "scout", "Map", "--no-output"]), /unknown option/i);
  assert.throws(() => parseArgs(["run", "scout", "Map", "--progress"]), /unknown option/i);
  assert.throws(() => parseArgs(["run", "scout", "Map"]), /requires --label/);
  assert.throws(() => parseArgs(["run", "scout", "Map", "--fork"]), /unknown option/i);
  assert.throws(() => parseArgs(["run", "scout", "Map", "--fresh"]), /unknown option/i);
  assert.throws(() => parseArgs(["run", "scout", "Map", "--context", "fresh"]), /unknown option/i);
  assert.throws(() => parseArgs(["parallel", "scout:Map", "--fork"]), /unknown option/i);
  assert.throws(() => parseArgs(["parallel", "scout:Map", "--attach"]), /unknown option/i);
  assert.throws(() => parseArgs(["parallel", "scout:Map"]), /label=agent:task/);
  assert.throws(() => parseArgs(["parallel", "!!!=scout:Map"]), /task label/);
  assert.throws(() => parseArgs(["wait"]), /wait requires an id/);
  assert.throws(() => parseArgs(["wait", "abc", "--timeout", "soon"]), /must be a duration/);
  assert.throws(() => parseArgs(["chain", "--step", "scout:Map", "--context", "fork"]), /unknown option/i);
  assert.throws(() => parseArgs(["chain", "--step", "scout:Map", "--foreground"]), /unknown option/i);
  assert.throws(() => parseArgs(["chain", "--step", "scout:Map"]), /label=agent:task/);
  assert.throws(() => parseArgs(["chain", "--group", "[{\"agent\":\"reviewer\",\"task\":\"Review\"}]"]), /requires label/);
});

test("rejects ambiguous management targets", () => {
  assert.throws(() => parseArgs(["get", "reviewer"]), /choose --agent or --chain/);
  assert.throws(() => parseArgs(["get", "reviewer", "--agent", "--chain"]), /choose only one/);
});

test("rejects invalid command", () => {
  assert.throws(() => parseArgs(["nope"]), /unknown command/i);
});

function withOutputSuffix(suffix, callback) {
  const previous = process.env.PI_WORKERS_OUTPUT_SUFFIX;
  process.env.PI_WORKERS_OUTPUT_SUFFIX = suffix;
  try {
    return callback();
  } finally {
    if (previous === undefined) delete process.env.PI_WORKERS_OUTPUT_SUFFIX;
    else process.env.PI_WORKERS_OUTPUT_SUFFIX = previous;
  }
}

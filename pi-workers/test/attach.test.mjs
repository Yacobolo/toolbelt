import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { attachToAsyncRun } from "../dist/attach.js";

test("attach watches status.json until terminal and emits final status", async () => {
  const asyncDir = mkdtempSync(join(tmpdir(), "pi-workers-attach-"));
  const statusPath = join(asyncDir, "status.json");
  const resultPath = join(asyncDir, "result.json");
  const logPath = join(asyncDir, "subagent-log-run-1.md");
  const outputPath = join(process.cwd(), ".pi-agents", "a.md");
  const writes = [];
  let statusCalls = 0;

  mkdirSync(join(process.cwd(), ".pi-agents"), { recursive: true });
  writeFileSync(resultPath, JSON.stringify({
    results: [{ output: `Output saved to: ${outputPath} (1.2 KB)` }],
  }));
  writeFileSync(statusPath, JSON.stringify({
    runId: "run-1",
    state: "running",
    steps: [{ agent: "reviewer", status: "running" }, { agent: "scout", status: "pending" }],
  }));
  setTimeout(() => {
    writeFileSync(statusPath, JSON.stringify({
      runId: "run-1",
      state: "running",
      steps: [{ agent: "reviewer", status: "running", tokens: 100 }, { agent: "scout", status: "pending" }],
    }));
  }, 10);
  setTimeout(() => {
    writeFileSync(statusPath, JSON.stringify({
      runId: "run-1",
      state: "complete",
      steps: [{ agent: "reviewer", status: "complete" }, { agent: "scout", status: "complete" }],
    }));
  }, 30);

  const final = await attachToAsyncRun(textResult("Async", { runId: "run-1", asyncDir }), {
    json: false,
    write: (text) => writes.push(text),
    status: async (id) => {
      statusCalls += 1;
      assert.equal(id, "run-1");
      return textResult(`Run: run-1\nState: complete\nResult: ${resultPath}\nLog: ${logPath}`);
    },
  });

  assert.equal(statusCalls, 1);
  assert.equal(final.terminal, true);
  assert.match(writes.join(""), /\[attach\] 1 agent running · 0\/2 done/);
  assert.equal(writes.filter((line) => line.includes("[attach] 1 agent running · 0/2 done")).length, 1);
  assert.match(writes.join(""), /\[attach\] complete · 2\/2 done/);
  assert.match(writes.join(""), /\[attach\] complete run-1/);
  assert.match(writes.join(""), /Outputs:\n  \.pi-agents\/a\.md/);
  assert.match(writes.join(""), new RegExp(`Trace:\\n  Result: ${escapeRegExp(tracePath(resultPath))}`));
  assert.match(writes.join(""), new RegExp(`Log:\\s+${escapeRegExp(tracePath(logPath))}`));
});

test("attach progress includes a single agent label", async () => {
  const asyncDir = mkdtempSync(join(tmpdir(), "pi-workers-attach-"));
  const statusPath = join(asyncDir, "status.json");
  const writes = [];

  writeFileSync(statusPath, JSON.stringify({
    runId: "run-2",
    state: "running",
    steps: [{ agent: "scout", status: "running" }],
  }));
  setTimeout(() => {
    writeFileSync(statusPath, JSON.stringify({
      runId: "run-2",
      state: "complete",
      steps: [{ agent: "scout", status: "complete" }],
    }));
  }, 10);

  await attachToAsyncRun(textResult("Async", { runId: "run-2", asyncDir }), {
    json: false,
    write: (text) => writes.push(text),
    status: async () => textResult("Run: run-2\nState: complete"),
  });

  assert.match(writes.join(""), /\[attach\] running scout · 0\/1 done/);
});

test("attach final receipt includes launch param outputs", async () => {
  const asyncDir = mkdtempSync(join(tmpdir(), "pi-workers-attach-"));
  const statusPath = join(asyncDir, "status.json");
  const writes = [];

  writeFileSync(statusPath, JSON.stringify({
    runId: "run-3",
    state: "complete",
    steps: [{ agent: "reviewer", status: "complete" }, { agent: "reviewer", status: "complete" }],
  }));

  await attachToAsyncRun({
    ...textResult("Async", { runId: "run-3", asyncDir }),
    params: {
      tasks: [
        { agent: "reviewer", task: "Correctness", output: ".pi-agents/reviewer-correctness-fixed.md" },
        { agent: "reviewer", task: "Tests", output: ".pi-agents/reviewer-tests-fixed.md" },
      ],
    },
  }, {
    json: false,
    write: (text) => writes.push(text),
    status: async () => textResult("Run: run-3\nState: complete\nLog: /tmp/subagent-log-run-3.md"),
  });

  assert.match(writes.join(""), /Outputs:\n  \.pi-agents\/reviewer-correctness-fixed\.md\n  \.pi-agents\/reviewer-tests-fixed\.md/);
});

function textResult(text, details = {}) {
  return {
    callId: "call-1",
    params: {},
    text,
    isError: false,
    state: text.match(/^State:\s*(\w+)/m)?.[1],
    terminal: /State:\s*complete/m.test(text),
    details,
    raw: { content: [{ type: "text", text }], details },
  };
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function tracePath(value) {
  return value.replace(/^\/var\/folders\/[^/]+\/[^/]+\/T\//, "/var/.../");
}

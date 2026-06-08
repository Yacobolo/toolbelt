import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { waitForRun } from "../dist/wait.js";

test("wait returns final status without progress output by default", async () => {
  const asyncDir = mkdtempSync(join(tmpdir(), "pi-workers-wait-"));
  const statusPath = join(asyncDir, "status.json");
  const writes = [];
  const calls = [];

  mkdirSync(asyncDir, { recursive: true });
  writeFileSync(statusPath, JSON.stringify({ state: "running" }));
  setTimeout(() => {
    writeFileSync(statusPath, JSON.stringify({ state: "complete" }));
  }, 10);

  const final = await waitForRun({
    id: "run-1",
    json: false,
    intervalMs: 50,
    write: (text) => writes.push(text),
    status: async (id) => {
      calls.push(id);
      return calls.length === 1
        ? textResult(`Run: run-1\nState: running\nProgress: 0/1 done\nDir: ${asyncDir}`)
        : textResult("Run: run-1\nState: complete\nProgress: 1/1 done");
    },
  });

  assert.equal(final.state, "complete");
  assert.deepEqual(writes, []);
  assert.deepEqual(calls, ["run-1", "run-1"]);
});

test("wait can emit compact progress changes", async () => {
  const writes = [];
  const statuses = [
    textResult("Run: run-2\nState: running\nProgress: 0/2 done"),
    textResult("Run: run-2\nState: running\nProgress: 1/2 done"),
    textResult("Run: run-2\nState: complete\nProgress: 2/2 done"),
  ];

  const final = await waitForRun({
    id: "run-2",
    json: false,
    progress: true,
    timeoutMs: 100,
    intervalMs: 1,
    write: (text) => writes.push(text),
    status: async () => statuses.shift() ?? textResult("Run: run-2\nState: complete"),
  });

  assert.equal(final.state, "complete");
  assert.deepEqual(writes, [
    "[wait] running · 0/2 done\n",
    "[wait] running · 1/2 done\n",
  ]);
});

function textResult(text) {
  return {
    callId: "call-1",
    params: {},
    text,
    isError: false,
    state: text.match(/^State:\s*(\w+)/m)?.[1],
    terminal: /State:\s*(complete|failed|paused|cancelled|canceled|interrupted)/m.test(text),
    raw: { content: [{ type: "text", text }] },
  };
}

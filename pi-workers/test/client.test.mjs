import assert from "node:assert/strict";
import test from "node:test";
import { PiSubagentClient, PiWorkersError } from "../dist/client.js";

test("execute forwards arbitrary subagent params directly", async () => {
  const calls = [];
  const client = new PiSubagentClient({
    cwd: "/repo",
    idFactory: () => "piw-fixed",
    sessionFactory: async () => fakeSession(async (id, params) => {
      calls.push({ id, params });
      return textResult("Run: abc\nState: complete", { mode: "single" });
    }),
  });

  const result = await client.execute({ action: "status", id: "abc" });
  assert.equal(result.callId, "piw-fixed");
  assert.deepEqual(calls, [{ id: "piw-fixed", params: { action: "status", id: "abc" } }]);
  assert.equal(result.text, "Run: abc\nState: complete");
  await client.close();
});

test("named helpers are strict subagent action mirrors", async () => {
  const calls = [];
  const client = new PiSubagentClient({
    idFactory: () => `call-${calls.length}`,
    sessionFactory: async () => fakeSession(async (id, params) => {
      calls.push({ id, params });
      return textResult("ok");
    }),
  });

  await client.doctor();
  await client.list();
  await client.get({ agent: "reviewer" });
  await client.get({ chainName: "review-loop" });
  await client.create({ config: { name: "x" } });
  await client.update({ agent: "reviewer", config: { model: "m" } });
  await client.delete({ chainName: "old-chain" });
  await client.status({ id: "abc" });
  await client.interrupt({});
  await client.resume({ id: "abc", message: "continue", index: 1 });

  assert.deepEqual(calls.map((call) => call.params), [
    { action: "doctor" },
    { action: "list" },
    { action: "get", agent: "reviewer" },
    { action: "get", chainName: "review-loop" },
    { action: "create", config: { name: "x" } },
    { action: "update", agent: "reviewer", config: { model: "m" } },
    { action: "delete", chainName: "old-chain" },
    { action: "status", id: "abc" },
    { action: "interrupt" },
    { action: "resume", id: "abc", message: "continue", index: 1 },
  ]);
});

test("execution helpers forward only native Pi params", async () => {
  const calls = [];
  const client = new PiSubagentClient({
    idFactory: () => `call-${calls.length}`,
    sessionFactory: async () => fakeSession(async (id, params) => {
      calls.push({ id, params });
      return textResult("ok");
    }),
  });

  await client.spawn({
    agent: "reviewer",
    task: "Review diff",
    label: "diff",
    async: true,
    output: "review.md",
    outputMode: "file-only",
    reads: ["a.ts"],
    skill: ["audit"],
    acceptance: { level: "checked" },
  });
  await client.parallel({
    tasks: [{ agent: "scout", task: "Map files", label: "files", reads: false }],
    async: true,
    concurrency: 2,
    worktree: true,
    control: { enabled: true },
  });
  await client.chain({
    chain: [{ agent: "scout", task: "Map" }],
    async: false,
    task: "Original",
    chainDir: "chain-out",
  });

  assert.deepEqual(calls.map((call) => call.params), [
    {
      agent: "reviewer",
      task: "Review diff",
      label: "diff",
      async: true,
      context: "fresh",
      output: "review.md",
      outputMode: "file-only",
      reads: ["a.ts"],
      skill: ["audit"],
      acceptance: { level: "checked" },
    },
    {
      tasks: [{ agent: "scout", task: "Map files", label: "files", reads: false }],
      async: true,
      context: "fresh",
      concurrency: 2,
      worktree: true,
      control: { enabled: true },
    },
    {
      chain: [{ agent: "scout", task: "Map" }],
      async: false,
      context: "fresh",
      task: "Original",
      chainDir: "chain-out",
    },
  ]);
});

test("missing subagent tool reports install guidance", async () => {
  const client = new PiSubagentClient({
    sessionFactory: async () => ({
      getToolDefinition: () => undefined,
      extensionRunner: { createContext: () => ({}) },
      dispose: () => {},
    }),
  });

  await assert.rejects(() => client.list(), (error) => {
    assert.ok(error instanceof PiWorkersError);
    assert.match(error.message, /pi install npm:pi-subagents/);
    return true;
  });
});

function fakeSession(execute) {
  return {
    getToolDefinition(name) {
      if (name !== "subagent") return undefined;
      return { name: "subagent", execute };
    },
    extensionRunner: { createContext: () => ({}) },
    dispose: () => {},
  };
}

function textResult(text, details = { mode: "single", results: [] }) {
  return { content: [{ type: "text", text }], details };
}

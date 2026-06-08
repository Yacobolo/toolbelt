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

test("lifecycle helpers forward only the params used by the MCP server", async () => {
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
  await client.status({ id: "abc" });
  await client.interrupt({});
  await client.resume({ id: "abc", message: "continue", index: 1 });

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
    { action: "status", id: "abc" },
    { action: "interrupt" },
    { action: "resume", id: "abc", message: "continue", index: 1 },
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

  await assert.rejects(() => client.status(), (error) => {
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

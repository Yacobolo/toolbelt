import assert from "node:assert/strict";
import test from "node:test";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { InMemoryTransport } from "@modelcontextprotocol/sdk/inMemory.js";
import { createPiWorkersMcpServer } from "../dist/mcp.js";

test("mcp server registers pi-workers tools", async () => {
  const { client, server } = await connectMcp(fakePiClient());
  try {
    const listed = await client.listTools();
    const names = listed.tools.map((tool) => tool.name).sort();
    assert.deepEqual(names, [
      "pi_workers_close_agent",
      "pi_workers_followup_task",
      "pi_workers_list_agents",
      "pi_workers_send_message",
      "pi_workers_spawn_agent",
      "pi_workers_wait_agent",
    ]);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp spawn agent forwards opinionated launch defaults", async () => {
  const calls = [];
  const { client, server } = await connectMcp(fakePiClient({ calls }));
  try {
    const result = await withOutputSuffix("fixedmcp1", () => client.callTool({
      name: "pi_workers_spawn_agent",
      arguments: {
        agent_type: "reviewer",
        message: "Review diff",
        task_name: "diff",
      },
    }));

    assert.equal(text(result), "Async run");
    assert.equal(result.structuredContent.output, ".pi-agents/reviewer-diff-fixedmcp1.md");
    assert.deepEqual(calls[0], {
      method: "spawn",
      input: {
        agent: "reviewer",
        task: "Review diff",
        label: "diff",
        async: true,
        output: ".pi-agents/reviewer-diff-fixedmcp1.md",
        outputMode: "file-only",
        progress: false,
      },
    });
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp spawn agent does not expose low-level launch knobs", async () => {
  const { client, server } = await connectMcp(fakePiClient());
  try {
    const listed = await client.listTools();
    const spawn = listed.tools.find((tool) => tool.name === "pi_workers_spawn_agent");
    assert.ok(spawn);
    const properties = spawn.inputSchema.properties;
    assert.deepEqual(Object.keys(properties).sort(), ["agent_type", "message", "task_name"]);
    assert.deepEqual(properties.agent_type.enum, [
      "context-builder",
      "debug",
      "delegate",
      "oracle",
      "pi-subagents",
      "planner",
      "researcher",
      "reviewer",
      "scout",
      "worker",
    ]);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp does not expose parallel or chain orchestration tools", async () => {
  const { client, server } = await connectMcp(fakePiClient());
  try {
    const listed = await client.listTools();
    const names = listed.tools.map((tool) => tool.name);
    assert.equal(names.includes("pi_workers_parallel"), false);
    assert.equal(names.includes("pi_workers_chain"), false);
    assert.equal(names.includes("pi_workers_doctor"), false);
    assert.equal(names.includes("pi_workers_status"), false);
    assert.equal(names.includes("pi_workers_interrupt_agent"), false);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp list agents returns live agents spawned through the server", async () => {
  const { client, server } = await connectMcp(fakePiClient());
  try {
    const empty = await client.callTool({ name: "pi_workers_list_agents", arguments: {} });
    assert.equal(text(empty), "Live agents: none");

    await withOutputSuffix("fixedmcp1", () => client.callTool({
      name: "pi_workers_spawn_agent",
      arguments: {
        agent_type: "reviewer",
        message: "Review diff",
        task_name: "diff",
      },
    }));
    const result = await client.callTool({ name: "pi_workers_list_agents", arguments: {} });

    assert.equal(text(result), "Live agents:\n- diff: running (run-1) — Review diff");
    assert.deepEqual(result.structuredContent.agents, [{
      agent_name: "diff",
      agent_id: "run-1",
      agent_type: "reviewer",
      agent_status: "running",
      last_task_message: "Review diff",
    }]);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp send message forwards to Pi resume", async () => {
  const calls = [];
  const { client, server } = await connectMcp(fakePiClient({ calls }));
  try {
    await client.callTool({
      name: "pi_workers_spawn_agent",
      arguments: { agent_type: "reviewer", message: "Review diff", task_name: "diff" },
    });
    const result = await client.callTool({
      name: "pi_workers_send_message",
      arguments: { target: "diff", message: "Keep going" },
    });
    assert.equal(text(result), "Run: run-1\nState: running");
    assert.deepEqual(calls.slice(1), [{ method: "resume", input: { id: "run-1", message: "Keep going" } }]);

    const listed = await client.callTool({ name: "pi_workers_list_agents", arguments: {} });
    assert.match(text(listed), /Keep going/);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp followup task forwards to Pi resume", async () => {
  const calls = [];
  const { client, server } = await connectMcp(fakePiClient({ calls }));
  try {
    await client.callTool({
      name: "pi_workers_spawn_agent",
      arguments: { agent_type: "reviewer", message: "Review diff", task_name: "diff" },
    });
    const result = await client.callTool({
      name: "pi_workers_followup_task",
      arguments: { target: "diff", message: "Also check tests" },
    });
    assert.equal(text(result), "Run: run-1\nState: running");
    assert.deepEqual(calls.slice(1), [{ method: "resume", input: { id: "run-1", message: "Also check tests" } }]);

    const listed = await client.callTool({ name: "pi_workers_list_agents", arguments: {} });
    assert.match(text(listed), /Also check tests/);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp close agent forwards to Pi interrupt", async () => {
  const calls = [];
  const { client, server } = await connectMcp(fakePiClient({ calls }));
  try {
    await client.callTool({
      name: "pi_workers_spawn_agent",
      arguments: { agent_type: "reviewer", message: "Review diff", task_name: "diff" },
    });
    const result = await client.callTool({
      name: "pi_workers_close_agent",
      arguments: { target: "diff" },
    });
    assert.equal(text(result), "Interrupted");
    assert.deepEqual(calls.slice(1), [{ method: "interrupt", input: { id: "run-1" } }]);

    const listed = await client.callTool({ name: "pi_workers_list_agents", arguments: {} });
    assert.match(text(listed), /diff: interrupted/);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp rejects empty task names", async () => {
  const { client, server } = await connectMcp(fakePiClient());
  try {
    const result = await client.callTool({
      name: "pi_workers_spawn_agent",
      arguments: { agent_type: "scout", message: "Map repo", task_name: "!!!" },
    });
    assert.equal(result.isError, true);
    assert.match(text(result), /task_name/i);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp wait waits for any live agent update", async () => {
  const calls = [];
  const { client, server } = await connectMcp(fakePiClient({
    status: async (id) => {
      calls.push(id);
      return textResult("Run: run-1\nState: complete\nProgress: 1/1 done");
    },
  }));
  try {
    await client.callTool({
      name: "pi_workers_spawn_agent",
      arguments: { agent_type: "reviewer", message: "Review diff", task_name: "diff" },
    });
    const result = await client.callTool({
      name: "pi_workers_wait_agent",
      arguments: { timeout_ms: 10000 },
    });

    assert.deepEqual(calls, ["run-1"]);
    assert.equal(text(result), "diff: complete");
    assert.equal(result.structuredContent.state, "complete");
    assert.equal(result.structuredContent.terminal, true);
    assert.equal(result.structuredContent.timed_out, false);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp wait drains queued mailbox updates", async () => {
  const calls = [];
  const { client, server } = await connectMcp(fakePiClient({
    status: async (id) => {
      calls.push(id);
      return textResult("Run: run-1\nState: complete\nProgress: 1/1 done");
    },
  }));
  try {
    await client.callTool({
      name: "pi_workers_spawn_agent",
      arguments: { agent_type: "reviewer", message: "Review diff", task_name: "diff" },
    });
    await delay(10);
    assert.deepEqual(calls, ["run-1"]);

    const result = await client.callTool({
      name: "pi_workers_wait_agent",
      arguments: { timeout_ms: 10000 },
    });

    assert.equal(text(result), "diff: complete");
    assert.deepEqual(calls, ["run-1"]);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp wait accepts omitted timeout", async () => {
  const calls = [];
  const { client, server } = await connectMcp(fakePiClient({
    status: async (id) => {
      calls.push(id);
      return textResult(`Run: ${id}\nState: complete\nProgress: 1/1 done`);
    },
  }));
  try {
    await client.callTool({
      name: "pi_workers_spawn_agent",
      arguments: { agent_type: "reviewer", message: "Review diff", task_name: "diff" },
    });
    const result = await client.callTool({
      name: "pi_workers_wait_agent",
      arguments: {},
    });

    assert.equal(result.isError, false);
    assert.deepEqual(calls, ["run-1"]);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp wait returns timeout summary instead of error", async () => {
  const { client, server } = await connectMcp(fakePiClient());
  try {
    const result = await client.callTool({
      name: "pi_workers_wait_agent",
      arguments: { timeout_ms: 10000 },
    });

    assert.equal(result.isError, false);
    assert.equal(text(result), "No live agents.");
    assert.equal(result.structuredContent.timed_out, true);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp wait rejects timeout below Codex minimum", async () => {
  const { client, server } = await connectMcp(fakePiClient());
  try {
    const result = await client.callTool({
      name: "pi_workers_wait_agent",
      arguments: { timeout_ms: 1 },
    });

    assert.equal(result.isError, true);
    assert.match(text(result), /10000|minimum|min|Too small/i);
  } finally {
    await client.close();
    await server.close();
  }
});

test("mcp tool errors include Pi install guidance", async () => {
  const { client, server } = await connectMcp(fakePiClient({
    spawn: async () => {
      throw new Error("Pi subagent tool is not available. Install the Pi extension with: pi install npm:pi-subagents");
    },
  }));
  try {
    const result = await client.callTool({
      name: "pi_workers_spawn_agent",
      arguments: { agent_type: "reviewer", message: "Review diff", task_name: "diff" },
    });
    assert.equal(result.isError, true);
    assert.match(text(result), /pi install npm:pi-subagents/);
  } finally {
    await client.close();
    await server.close();
  }
});

async function connectMcp(piClient) {
  const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
  const server = createPiWorkersMcpServer({ cwd: "/repo", clientFactory: () => piClient, monitorIntervalMs: 1 });
  const client = new Client({ name: "pi-workers-test", version: "0.1.0" });
  await server.connect(serverTransport);
  await client.connect(clientTransport);
  return { client, server };
}

function fakePiClient(overrides = {}) {
  const calls = overrides.calls ?? [];
  return {
    spawn: overrides.spawn ?? (async (input) => {
      calls.push({ method: "spawn", input });
      return textResult("Async run", { runId: "run-1" });
    }),
    status: async (input) => overrides.status
      ? overrides.status(input.id)
      : textResult(`Run: ${input.id ?? "all"}\nState: complete`),
    resume: async (input) => {
      calls.push({ method: "resume", input });
      return textResult(`Run: ${input.id}\nState: running`);
    },
    interrupt: async (input) => {
      calls.push({ method: "interrupt", input });
      return textResult("Interrupted");
    },
    close: async () => {},
  };
}

function textResult(text, details = {}) {
  return {
    callId: "call-1",
    params: {},
    text,
    isError: false,
    state: text.match(/^State:\s*(\w+)/m)?.[1],
    terminal: /State:\s*(complete|failed|paused|cancelled|canceled|interrupted)/m.test(text),
    details,
    raw: { content: [{ type: "text", text }], details },
  };
}

function text(result) {
  return result.content.find((item) => item.type === "text")?.text ?? "";
}

async function withOutputSuffix(suffix, callback) {
  const previous = process.env.PI_WORKERS_OUTPUT_SUFFIX;
  process.env.PI_WORKERS_OUTPUT_SUFFIX = suffix;
  try {
    return await callback();
  } finally {
    if (previous === undefined) delete process.env.PI_WORKERS_OUTPUT_SUFFIX;
    else process.env.PI_WORKERS_OUTPUT_SUFFIX = previous;
  }
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

import assert from "node:assert/strict";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { createAgentSession, SessionManager } from "@earendil-works/pi-coding-agent";
import { PiSubagentClient } from "../dist/client.js";

const contractEnabled = process.env.PI_WORKERS_CONTRACT === "1";
const runContractEnabled = process.env.PI_WORKERS_CONTRACT_RUN === "1";

test("real Pi execute parent-visible content matches direct tool execution", { skip: !contractEnabled }, async () => {
  const cwd = mkdtempSync(join(tmpdir(), "pi-workers-contract-"));
  const { session } = await createAgentSession({
    cwd,
    tools: ["subagent"],
    sessionManager: SessionManager.inMemory(),
  });
  const client = new PiSubagentClient({
    cwd,
    sessionFactory: async () => session,
  });

  try {
    const direct = await executeDirect(session, "piw-doctor", { action: "doctor" });
    const viaClient = await client.execute({ action: "doctor" });
    assert.equal(viaClient.text, textOf(direct));
  } finally {
    await client.close();
  }
});

test("real Pi async run preserves parent-visible launch content shape", { skip: !runContractEnabled }, async () => {
  const cwd = mkdtempSync(join(tmpdir(), "pi-workers-contract-run-"));
  const { session } = await createAgentSession({
    cwd,
    tools: ["subagent"],
    sessionManager: SessionManager.inMemory(),
  });
  const client = new PiSubagentClient({
    cwd,
    idFactory: () => "contract-run",
    sessionFactory: async () => session,
  });

  try {
    const result = await client.spawn({
      agent: "scout",
      task: "Say CONTRACT_OK only. Do not modify files.",
      async: true,
    });
    assert.match(result.text, /Async:|Run:/);
    assert.equal(result.params.output, undefined);
    assert.equal(result.params.outputMode, undefined);
  } finally {
    await client.close();
  }
});

async function executeDirect(session, callId, params) {
  const tool = session.getToolDefinition("subagent");
  assert.ok(tool, "subagent tool must be available");
  return tool.execute(callId, params, undefined, undefined, session.extensionRunner.createContext());
}

function textOf(raw) {
  return raw.content
    .filter((part) => part && typeof part === "object" && typeof part.text === "string")
    .map((part) => part.text)
    .join("\n");
}

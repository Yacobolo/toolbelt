import assert from "node:assert/strict";
import test from "node:test";
import { createSubagentClient } from "../dist/index.js";

test("real Pi subagent doctor smoke", { skip: process.env.PI_WORKERS_SMOKE !== "1" }, async () => {
  const client = createSubagentClient();
  try {
    const result = await client.execute({ action: "doctor" });
    assert.match(result.text, /Subagents doctor report|Runtime/);
  } finally {
    await client.close();
  }
});

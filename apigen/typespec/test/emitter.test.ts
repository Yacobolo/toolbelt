import { execFile } from "node:child_process";
import { mkdtemp, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import { describe, expect, it } from "vitest";

const execFileAsync = promisify(execFile);
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const tsp = join(root, "node_modules", "@typespec", "compiler", "cmd", "tsp.js");

describe("APIGen TypeSpec emitter", () => {
  it("emits JSON IR for the todo fixture", async () => {
    const outDir = await mkdtemp(join(tmpdir(), "apigen-typespec-"));
    const irPath = join(outDir, "json-ir.json");

    await compileFixture("todo", irPath);

    const doc = JSON.parse(await readFile(irPath, "utf8"));
    expect(doc.schema_version).toBe("v1");
    expect(doc.info.title).toBe("APIGen Todo Example");
    expect(doc.endpoints.map((x: any) => x.operation_id)).toEqual([
      "listTodos",
      "createTodo",
      "getTodo",
      "completeTodo",
      "deleteTodo",
    ]);
    expect(doc.schemas.Todo.property_order).toEqual(["id", "title", "status"]);
    expect(doc.endpoints[1].request_body.schema.ref).toBe("CreateTodoRequest");
    expect(doc.endpoints[1].cli.command).toEqual(["todos", "create"]);
  });

  it("fails when a request body cannot map to a named IR schema", async () => {
    const outDir = await mkdtemp(join(tmpdir(), "apigen-typespec-"));
    const irPath = join(outDir, "json-ir.json");

    await expect(compileFixture("invalid", irPath)).rejects.toSatisfy((error: any) =>
      `${error.stdout}\n${error.stderr}`.includes(
        "requires request body to resolve to a named model schema",
      ),
    );
  });
});

async function compileFixture(name: string, outputFile: string) {
  await execFileAsync(
    process.execPath,
    [
      tsp,
      "compile",
      join(root, "test", "fixtures", name),
      "--import",
      root,
      "--emit",
      root,
      "--option",
      `@yacobolo/apigen.output-file=${outputFile}`,
      "--option",
      "@yacobolo/apigen.base-path=/",
    ],
    { cwd: root },
  );
}

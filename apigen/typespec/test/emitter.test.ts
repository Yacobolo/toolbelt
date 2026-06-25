import { execFile } from "node:child_process";
import { mkdir, mkdtemp, readFile, stat, writeFile } from "node:fs/promises";
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
    expect(doc.openapi.security).toEqual([{ BearerAuth: [] }, { ApiKeyAuth: [] }]);
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

  it("emits named enum schemas and inherited model properties", async () => {
    const doc = await compileSource(`
      using Http;

      @service(#{ title: "Enum API" })
      namespace EnumAPI;

      enum WidgetStatus {
        active,
        archived,
      }

      model Resource {
        id: string;
      }

      model Widget extends Resource {
        status: WidgetStatus;
        name?: string;
      }

      @route("/widgets/{status}")
      @get
      op list(@path status: WidgetStatus): Widget;

      @route("/widgets/status")
      @post
      op setStatus(@body body: WidgetStatus): Widget;
    `);

    expect(doc.schemas.WidgetStatus).toEqual({
      type: "string",
      enum: ["active", "archived"],
    });
    expect(doc.schemas.Widget.property_order).toEqual(["status", "name", "id"]);
    expect(doc.schemas.Widget.required).toEqual(["status", "id"]);
    expect(doc.schemas.Widget.properties.status.schema).toEqual({ ref: "WidgetStatus" });
    expect(doc.endpoints[0].parameters[0].schema).toEqual({ ref: "WidgetStatus" });
    expect(doc.endpoints[1].request_body.schema).toEqual({ ref: "WidgetStatus" });
  });

  it("fails without writing IR for inline string literal unions", async () => {
    const outDir = await mkdtemp(join(tmpdir(), "apigen-typespec-"));
    const irPath = join(outDir, "json-ir.json");

    await expect(
      compileSource(
        `
          using Http;

          @service(#{ title: "Inline Union API" })
          namespace InlineUnionAPI;

          model Widget {
            status: "active" | "archived";
          }

          @route("/widgets")
          @get
          op list(): Widget;
        `,
        irPath,
      ),
    ).rejects.toSatisfy((error: any) =>
      `${error.stdout}\n${error.stderr}`.includes("Unsupported inline enum-like union"),
    );
    await expect(stat(irPath)).rejects.toMatchObject({ code: "ENOENT" });
  });

  it("fails without writing IR for response status ranges", async () => {
    const outDir = await mkdtemp(join(tmpdir(), "apigen-typespec-"));
    const irPath = join(outDir, "json-ir.json");

    await expect(
      compileSource(
        `
          using Http;

          @service(#{ title: "Range API" })
          namespace RangeAPI;

          model ServerError {
            @statusCode statusCode: "*";
          }

          @route("/widgets")
          @get
          op list(): ServerError;
        `,
        irPath,
      ),
    ).rejects.toSatisfy((error: any) =>
      `${error.stdout}\n${error.stderr}`.includes("statusCode value must be a three digit code"),
    );
    await expect(stat(irPath)).rejects.toMatchObject({ code: "ENOENT" });
  });

  it("emits service and operation auth requirements", async () => {
    const doc = await compileSource(`
      using Http;

      @service(#{ title: "Auth API" })
      @useAuth(BearerAuth | ApiKeyAuth<ApiKeyLocation.header, "X-API-Key">)
      namespace AuthAPI;

      model Widget {
        id: string;
      }

      @route("/default")
      @get
      op byDefault(): Widget;

      @useAuth([BearerAuth, ApiKeyAuth<ApiKeyLocation.header, "X-API-Key">])
      @route("/both")
      @get
      op both(): Widget;
    `);

    expect(doc.openapi.security_schemes).toEqual({
      ApiKeyAuth: { type: "apiKey", in: "header", name: "X-API-Key" },
      BearerAuth: { type: "http", scheme: "Bearer" },
    });
    expect(doc.openapi.security).toEqual([{ BearerAuth: [] }, { ApiKeyAuth: [] }]);
    expect(doc.endpoints[0].security).toBeUndefined();
    expect(doc.endpoints[1].security).toEqual([{ BearerAuth: [], ApiKeyAuth: [] }]);
  });

  it("emits operation vendor extensions from TypeSpec OpenAPI decorators", async () => {
    const doc = await compileSource(`
      using Http;
      using TypeSpec.OpenAPI;

      @service(#{ title: "Extensions API" })
      namespace ExtensionsAPI;

      model Workspace {
        id: string;
      }

      @extension("x-agent", #{
        enabled: true,
        disabled: false,
        name: "list_workspace_assets",
        risk: "read",
        retries: 0,
        score: 1.5,
        scopes: #[],
        tags: #["workspace", "lineage"],
        nested: #{ nullable: null, count: 3, empty: #{} },
      })
      @extension("x-flags", #[])
      @route("/workspaces")
      @get
      op listWorkspaces(): Workspace[];
    `);

    expect(doc.endpoints[0].extensions).toEqual({
      "x-agent": {
        enabled: true,
        disabled: false,
        name: "list_workspace_assets",
        risk: "read",
        retries: 0,
        score: 1.5,
        scopes: [],
        tags: ["workspace", "lineage"],
        nested: { nullable: null, count: 3, empty: {} },
      },
      "x-flags": [],
    });
  });

  it("fails without writing IR for APIGen-reserved generic operation extensions", async () => {
    const outDir = await mkdtemp(join(tmpdir(), "apigen-typespec-"));
    const irPath = join(outDir, "json-ir.json");

    await expect(
      compileSource(
        `
          using Http;
          using TypeSpec.OpenAPI;

          @service(#{ title: "Reserved Extension API" })
          namespace ReservedExtensionAPI;

          @extension("x-authz", #{ mode: "none" })
          @route("/widgets")
          @get
          op list(): string;
        `,
        irPath,
      ),
    ).rejects.toSatisfy((error: any) =>
      `${error.stdout}\n${error.stderr}`.includes("reserved for APIGen-owned metadata"),
    );
    await expect(stat(irPath)).rejects.toMatchObject({ code: "ENOENT" });
  });

  it("fails without writing IR for non-vendor operation extensions", async () => {
    const outDir = await mkdtemp(join(tmpdir(), "apigen-typespec-"));
    const irPath = join(outDir, "json-ir.json");

    await expect(
      compileSource(
        `
          using Http;
          using TypeSpec.OpenAPI;

          @service(#{ title: "Invalid Extension API" })
          namespace InvalidExtensionAPI;

          @extension("agent", true)
          @route("/widgets")
          @get
          op list(): string;
        `,
        irPath,
      ),
    ).rejects.toSatisfy((error: any) =>
      `${error.stdout}\n${error.stderr}`.includes("must start with 'x-'"),
    );
    await expect(stat(irPath)).rejects.toMatchObject({ code: "ENOENT" });
  });

  it("fails without writing IR for x-apigen generic operation extensions", async () => {
    const outDir = await mkdtemp(join(tmpdir(), "apigen-typespec-"));
    const irPath = join(outDir, "json-ir.json");

    await expect(
      compileSource(
        `
          using Http;
          using TypeSpec.OpenAPI;

          @service(#{ title: "Reserved Extension API" })
          namespace ReservedExtensionAPI;

          @extension("x-apigen-tool", true)
          @route("/widgets")
          @get
          op list(): string;
        `,
        irPath,
      ),
    ).rejects.toSatisfy((error: any) =>
      `${error.stdout}\n${error.stderr}`.includes("reserved for APIGen-owned metadata"),
    );
    await expect(stat(irPath)).rejects.toMatchObject({ code: "ENOENT" });
  });

  it("fails for no-auth operation overrides on secured services", async () => {
    const outDir = await mkdtemp(join(tmpdir(), "apigen-typespec-"));
    const irPath = join(outDir, "json-ir.json");

    await expect(
      compileSource(
        `
          using Http;

          @service(#{ title: "NoAuth API" })
          @useAuth(BearerAuth)
          namespace NoAuthAPI;

          model Widget {
            id: string;
          }

          @useAuth(NoAuth)
          @route("/public")
          @get
          op publicOp(): Widget;
        `,
        irPath,
      ),
    ).rejects.toSatisfy((error: any) =>
      `${error.stdout}\n${error.stderr}`.includes("does not support NoAuth operation overrides"),
    );
    await expect(stat(irPath)).rejects.toMatchObject({ code: "ENOENT" });
  });

  it("treats service-level no-auth as unsecured", async () => {
    const doc = await compileSource(`
      using Http;

      @service(#{ title: "Public API" })
      @useAuth(NoAuth)
      namespace PublicAPI;

      @route("/ping")
      @get
      op ping(): string;
    `);

    expect(doc.openapi.security).toBeUndefined();
    expect(doc.openapi.security_schemes).toBeUndefined();
    expect(doc.endpoints[0].security).toBeUndefined();
  });

  it("fails clearly when multiple services are declared", async () => {
    const outDir = await mkdtemp(join(tmpdir(), "apigen-typespec-"));
    const irPath = join(outDir, "json-ir.json");

    await expect(
      compileSource(
        `
          using Http;

          @service(#{ title: "One" })
          namespace One {
            @route("/one")
            @get
            op one(): string;
          }

          @service(#{ title: "Two" })
          namespace Two {
            @route("/two")
            @get
            op two(): string;
          }
        `,
        irPath,
      ),
    ).rejects.toSatisfy((error: any) =>
      `${error.stdout}\n${error.stderr}`.includes("exactly one TypeSpec service"),
    );
    await expect(stat(irPath)).rejects.toMatchObject({ code: "ENOENT" });
  });
});

async function compileFixture(name: string, outputFile: string) {
  await compileDirectory(join(root, "test", "fixtures", name), outputFile);
}

async function compileSource(source: string, outputFile?: string) {
  const outDir = await mkdtemp(join(tmpdir(), "apigen-typespec-"));
  const fixtureDir = join(outDir, "source");
  const irPath = outputFile ?? join(outDir, "json-ir.json");
  await mkdir(fixtureDir, { recursive: true });
  await writeFile(join(fixtureDir, "main.tsp"), source);
  await compileDirectory(fixtureDir, irPath);
  return JSON.parse(await readFile(irPath, "utf8"));
}

async function compileDirectory(sourceDir: string, outputFile: string) {
  await execFileAsync(
    process.execPath,
    [
      tsp,
      "compile",
      sourceDir,
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

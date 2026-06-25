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
    expect(doc.schema_version).toBe("v2");
    expect(doc.info.title).toBe("APIGen Todo Example");
    expect(doc.endpoints.map((x: any) => x.operation_id)).toEqual([
      "listTodos",
      "createTodo",
      "getTodo",
      "completeTodo",
      "deleteTodo",
    ]);
    expect(doc.schemas.Todo.property_order).toEqual(["id", "title", "status"]);
    expect(doc.endpoints[1].request_body.contents[0]).toMatchObject({
      content_type: "application/json",
      body_kind: "json",
      schema: { ref: "CreateTodoRequest" },
    });
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
    expect(doc.endpoints[1].request_body.contents[0].schema).toEqual({ ref: "WidgetStatus" });
  });

  it("emits v2 IR for optimized TypeSpec HTTP authoring", async () => {
    const doc = await compileSource(`
      using Http;

      @service(#{ title: "Optimized API" })
      namespace OptimizedAPI;

      model Error {
        code: int32;
        message: string;
      }

      model Artifact {
        id: string;
      }

      model ArtifactCreate {
        name: string;
      }

      model Metadata {
        name: string;
      }

      model OkJson<T> {
        ...OkResponse;
        ...Body<T>;
      }

      model CreatedJson<T> {
        ...CreatedResponse;
        ...Body<T>;
      }

      model BadRequest {
        ...BadRequestResponse;
        ...Body<Error>;
      }

      model RateLimited {
        ...Response<429>;
        ...Body<Error>;
      }

      alias CommonErrors = BadRequest | RateLimited;

      @route("/artifacts")
      namespace Artifacts {
        @route("/{id}")
        @get
        op get(@path id: string): OkJson<Artifact> | CommonErrors;

        @post
        op create(@body body?: ArtifactCreate): CreatedJson<Artifact> | CommonErrors;

        @route("/{id}/text")
        @put
        op replaceText(@path id: string, @header contentType: "text/plain", @body body: string): OkJson<Artifact> | CommonErrors;

        @route("/{id}/blob")
        @put
        op replaceBlob(@path id: string, @header contentType: "application/octet-stream", @body body: bytes): OkJson<Artifact> | CommonErrors;

        @route("/{id}/file")
        @put
        op replaceFile(@path id: string, @bodyRoot body: File<"application/octet-stream", bytes>): OkJson<Artifact> | CommonErrors;

        @route("/{id}/form")
        @put
        op replaceForm(@path id: string, @header contentType: "application/x-www-form-urlencoded", @body body: Metadata): OkJson<Artifact> | CommonErrors;

        @route("/{id}/multipart")
        @put
        op replaceMultipart(@path id: string, @multipartBody body: {
          metadata: HttpPart<Metadata>;
          artifact: HttpPart<File<"application/octet-stream", bytes>>;
        }): OkJson<Artifact> | CommonErrors;
      }
    `);

    expect(doc.schema_version).toBe("v2");
    expect(doc.endpoints.map((x: any) => x.path)).toEqual([
      "/artifacts/{id}",
      "/artifacts",
      "/artifacts/{id}/text",
      "/artifacts/{id}/blob",
      "/artifacts/{id}/file",
      "/artifacts/{id}/form",
      "/artifacts/{id}/multipart",
    ]);
    expect(doc.endpoints[0].responses.map((x: any) => x.status_code)).toEqual([200, 400, 429]);
    expect(doc.endpoints[1].request_body.required).toBe(false);
    expect(doc.endpoints[2].request_body.contents[0].body_kind).toBe("text");
    expect(doc.endpoints[3].request_body.contents[0]).toMatchObject({
      content_type: "application/octet-stream",
      body_kind: "binary",
      schema: { type: "string", format: "binary" },
    });
    expect(doc.endpoints[4].request_body.contents[0].body_kind).toBe("file");
    expect(doc.endpoints[5].request_body.contents[0].body_kind).toBe("form_urlencoded");
    expect(doc.endpoints[6].request_body.contents[0].body_kind).toBe("multipart");
    expect(doc.endpoints[6].request_body.contents[0].parts.map((x: any) => x.name)).toEqual([
      "metadata",
      "artifact",
    ]);
  });

  it("merges same-status response variants into ordered IR contents", async () => {
    const doc = await compileSource(`
      using Http;

      @service(#{ title: "Multi Content API" })
      namespace MultiContentAPI;

      model Artifact {
        id: string;
      }

      model JSONArtifact {
        ...OkResponse;
        ...Body<Artifact>;
      }

      model BinaryArtifact {
        ...OkResponse;
        @header contentType: "application/octet-stream";
        @body body: bytes;
      }

      @route("/artifacts/{id}")
      @get
      op getArtifact(@path id: string): JSONArtifact | BinaryArtifact;
    `);

    const response = doc.endpoints[0].responses[0];
    expect(doc.endpoints[0].responses).toHaveLength(1);
    expect(response.status_code).toBe(200);
    expect(response.contents).toEqual([
      {
        content_type: "application/json",
        body_kind: "json",
        schema: { ref: "Artifact" },
      },
      {
        content_type: "application/octet-stream",
        body_kind: "binary",
        schema: { type: "string", format: "binary" },
      },
    ]);
  });

  it("coalesces shared-route content variants and literal accept headers", async () => {
    const doc = await compileSource(`
      using Http;

      @service(#{ title: "Shared Route API" })
      namespace SharedRouteAPI;

      model Artifact {
        id: string;
      }

      model JsonArtifact {
        ...OkResponse;
        ...Body<Artifact>;
      }

      model BinaryArtifact {
        ...OkResponse;
        @header contentType: "application/octet-stream";
        @body body: bytes;
      }

      @route("/artifacts/{id}")
      @sharedRoute
      @get
      op getArtifactJson(@path id: string, @header accept: "application/json"): JsonArtifact;

      @route("/artifacts/{id}")
      @sharedRoute
      @get
      op getArtifactBinary(@path id: string, @header accept: "application/octet-stream"): BinaryArtifact;
    `);

    expect(doc.endpoints).toHaveLength(1);
    expect(doc.endpoints[0]).toMatchObject({
      method: "get",
      path: "/artifacts/{id}",
      operation_id: "getArtifactJson",
    });
    expect(doc.endpoints[0].parameters).toEqual([
      { name: "id", in: "path", required: true, schema: { type: "string" } },
      {
        name: "accept",
        in: "header",
        required: true,
        schema: { type: "string", enum: ["application/json", "application/octet-stream"] },
      },
    ]);
    expect(doc.endpoints[0].responses).toHaveLength(1);
    expect(doc.endpoints[0].responses[0].contents).toEqual([
      { content_type: "application/json", body_kind: "json", schema: { ref: "Artifact" } },
      {
        content_type: "application/octet-stream",
        body_kind: "binary",
        schema: { type: "string", format: "binary" },
      },
    ]);
  });

  it("coalesces overload content variants into the overload base operation", async () => {
    const doc = await compileSource(`
      using Http;

      @service(#{ title: "Overload API" })
      namespace OverloadAPI;

      model Artifact {
        id: string;
      }

      model JsonArtifact {
        ...OkResponse;
        ...Body<Artifact>;
      }

      model BinaryArtifact {
        ...OkResponse;
        @header contentType: "application/octet-stream";
        @body body: bytes;
      }

      @route("/artifacts/{id}")
      @get
      op getArtifact(@path id: string, @header accept: "application/json" | "application/octet-stream"): JsonArtifact | BinaryArtifact;

      @overload(getArtifact)
      op getArtifactJson(@path id: string, @header accept: "application/json"): JsonArtifact;

      @overload(getArtifact)
      op getArtifactBinary(@path id: string, @header accept: "application/octet-stream"): BinaryArtifact;
    `);

    expect(doc.endpoints).toHaveLength(1);
    expect(doc.endpoints[0].operation_id).toBe("getArtifact");
    expect(doc.endpoints[0].parameters[1].schema).toEqual({
      type: "string",
      enum: ["application/json", "application/octet-stream"],
    });
    expect(doc.endpoints[0].responses[0].contents.map((x: any) => x.content_type)).toEqual([
      "application/json",
      "application/octet-stream",
    ]);
  });

  it("emits TypeSpec-native file and multipart metadata", async () => {
    const doc = await compileSource(`
      using Http;

      @service(#{ title: "Multipart API" })
      namespace MultipartAPI;

      model Metadata {
        name: string;
      }

      model OkJson<T> {
        ...OkResponse;
        ...Body<T>;
      }

      model Artifact {
        id: string;
      }

      @route("/blob")
      @put
      op uploadBlob(@body body: bytes): OkJson<Artifact>;

      @route("/file")
      @put
      op uploadFile(@bodyRoot body: File<"application/octet-stream", bytes>): OkJson<Artifact>;

      @route("/multipart")
      @post
      op uploadMultipart(@multipartBody body: {
        metadata: HttpPart<Metadata>;
        displayName?: HttpPart<string, #{ name: "display-name" }>;
        attachments: HttpPart<File<"application/octet-stream", bytes>>[];
        samples: HttpPart<string[]>;
      }): OkJson<Artifact>;

      @route("/mixed")
      @post
      op uploadMixed(@header contentType: "multipart/mixed", @multipartBody body: [
        HttpPart<string>,
        HttpPart<File<"application/octet-stream", bytes>, #{ name: "payload" }>,
      ]): OkJson<Artifact>;
    `);

    expect(doc.endpoints[0].request_body.contents[0]).toMatchObject({
      content_type: "application/octet-stream",
      body_kind: "binary",
      schema: { type: "string", format: "binary" },
    });
    expect(doc.endpoints[1].request_body.contents[0]).toMatchObject({
      content_type: "application/octet-stream",
      body_kind: "file",
      schema: { type: "string", format: "binary" },
    });
    expect(doc.endpoints[2].request_body.contents[0].parts).toEqual([
      {
        name: "metadata",
        wire_name: "metadata",
        part_kind: "model",
        required: true,
        content_type: "application/json",
        body_kind: "json",
        schema: { ref: "Metadata" },
      },
      {
        name: "displayName",
        wire_name: "display-name",
        part_kind: "model",
        required: false,
        content_type: "text/plain",
        body_kind: "text",
        schema: { type: "string" },
      },
      {
        name: "attachments",
        wire_name: "attachments",
        part_kind: "model",
        repeated: true,
        required: true,
        content_type: "application/octet-stream",
        body_kind: "file",
        filename: true,
        schema: { type: "string", format: "binary" },
      },
      {
        name: "samples",
        wire_name: "samples",
        part_kind: "model",
        required: true,
        content_type: "application/json",
        body_kind: "json",
        schema: { type: "array", items: { type: "string" } },
      },
    ]);
    expect(doc.endpoints[3].request_body.contents[0]).toMatchObject({
      content_type: "multipart/mixed",
      body_kind: "multipart",
    });
    expect(doc.endpoints[3].request_body.contents[0].parts).toEqual([
      {
        name: "part1",
        part_kind: "tuple",
        required: true,
        content_type: "text/plain",
        body_kind: "text",
        schema: { type: "string" },
      },
      {
        name: "part2",
        wire_name: "payload",
        part_kind: "tuple",
        required: true,
        content_type: "application/octet-stream",
        body_kind: "file",
        filename: true,
        schema: { type: "string", format: "binary" },
      },
    ]);
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

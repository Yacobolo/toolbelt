# apigen

`apigen` compiles authored API contracts into canonical OpenAPI, versioned JSON IR, generated Go server code, generated request-model types, and generated Cobra CLI registries.

Module path: `github.com/Yacobolo/toolbelt/apigen`

## Model

APIGen has two contract layers:

- TypeSpec authoring input for humans
- JSON IR `v2` for generators

Canonical OpenAPI is the published API artifact. JSON IR is the compatibility boundary between TypeSpec and the Go emitters. Repo-owned OpenAPI extensions such as `x-authz` are preserved there.

## CLI

Install the CLI:

```bash
go install github.com/Yacobolo/toolbelt/apigen/cmd/apigen@v0.3.2
```

Or run from this module during local development:

```bash
go run ./cmd/apigen --help
```

Commands:

- `typespec-compile`: TypeSpec -> JSON IR + OpenAPI
- `openapi`: JSON IR -> OpenAPI
- `server`: JSON IR -> server + request models
- `cli`: JSON IR -> Cobra registry
- `all`: JSON IR -> all Go outputs

The CLI supports direct flags or a manifest selected with `-manifest <file>` and `-target <name>`.

Recommended grouped manifest shape:

```yaml
targets:
  - name: example
    typespec_dir: api/typespec
    ir_out: api/gen/json-ir.json
    openapi_out: api/gen/openapi.yaml
    go_out:
      dir: internal/api/gen
    cli_out:
      dir: cmd/cli/gen
```

Manifest target fields:

- `typespec_dir`
- `ir_out`
- `openapi_out`
- `go_out.dir`
- `go_out.package`
- `go_out.server_file`
- `go_out.request_models_file`
- `cli_out.dir`
- `cli_out.package`
- `cli_out.file`

## Public Surface

Supported packages:

- `github.com/Yacobolo/toolbelt/apigen/ir`
- `github.com/Yacobolo/toolbelt/apigen/emit/openapi`
- `github.com/Yacobolo/toolbelt/apigen/emit/requestmodelgo`
- `github.com/Yacobolo/toolbelt/apigen/emit/servergo`
- `github.com/Yacobolo/toolbelt/apigen/emit/cligo`
- `github.com/Yacobolo/toolbelt/apigen/runtime/chi`
- `github.com/Yacobolo/toolbelt/apigen/runtime/cobra`

Package roles:

- `typespec`: TypeSpec emitter package used by `typespec-compile`
- `ir`: versioned generator contract
- `emit/*`: OpenAPI, server, request-model, and CLI emitters
- `runtime/*`: thin runtime helpers used by generated code
- `cmd/apigen`: CLI entrypoint

Public packages must stay isolated from sibling `toolbelt` packages outside `apigen`.

## Using It

Recommended TypeSpec flow:

1. Author API contracts in TypeSpec.
2. Run `typespec-compile` to produce JSON IR and canonical OpenAPI.
3. Run `all` to generate server, request-model, and CLI outputs.
4. Build your service against `runtime/chi` and your CLI against `runtime/cobra`.

The runnable reference showcase lives in `example/`. It is a small todo app with checked-in `json-ir`, OpenAPI, server transport, request-model aliases, CLI registry metadata, handwritten strict handlers, and a generated Cobra CLI.

The in-repo TypeSpec emitter lives in `typespec/` with a checked-in `package-lock.json`. Use `npm ci` there for reproducible local TypeSpec development; `typespec-compile` also bootstraps that pinned toolchain when needed. Project TypeSpec sources may use conventional package imports such as `import "@typespec/http";`, `import "@typespec/openapi";`, and `import "@yacobolo/apigen";`; the CLI resolves those imports from its managed cache.

## Operation Vendor Extensions

Use TypeSpec's native OpenAPI extension decorator to attach downstream-owned operation metadata:

```typespec
using TypeSpec.OpenAPI;

@extension("x-agent", #{
  enabled: true,
  name: "list_workspace_assets",
  risk: "read",
  tags: #["workspace", "lineage"],
})
@route("/workspaces")
@get
op listWorkspaces(): Workspace[];
```

APIGen preserves operation-level `x-*` extensions through TypeSpec -> JSON IR -> canonical OpenAPI -> generated Go operation contracts. Extension values must be JSON-compatible: `null`, strings, booleans, finite numbers, arrays, and objects.

Generated server packages expose extensions through `GenOperationContract.Extensions`:

```go
contract, ok := gen.GetAPIGenOperationContract("listWorkspaces")
if ok {
	agent, _ := contract.Extensions["x-agent"].(map[string]any)
	enabled, _ := agent["enabled"].(bool)
	_ = enabled
}
```

Accessors return defensive copies, so callers may filter or reshape extension metadata without mutating generated global state. APIGen does not interpret downstream extension semantics; policy such as agent allowlists, risk handling, auth checks, and workspace scoping belongs in the consuming application.

APIGen-owned extension keys are reserved. Use APIGen decorators for `x-authz` and `x-apigen-*` metadata instead of generic `@extension`.

Install as a dependency with:

```bash
go get github.com/Yacobolo/toolbelt/apigen@v0.3.2
```

## Contract Notes

JSON IR currently supports schema version `v2`. Required root fields are `schema_version`, `info.title`, `info.version`, and at least one endpoint. Request and response bodies use ordered `contents` entries with explicit `content_type` and `body_kind`. Endpoint extensions preserve operation-level `x-*` vendor metadata; APIGen-owned endpoint extensions include `x-authz` and `x-apigen-manual`. Supported response extensions include `x-apigen-response-shape`.

Generated request bodies are contract-first:

- JSON and form object bodies used in generated Go output should resolve to named IR-owned schemas
- text bodies generate `string`, raw `bytes` bodies generate `[]byte`, and TypeSpec `Http.File` bodies generate `GenFile`
- `GenFile` carries `Contents []byte`, `ContentType string`, and optional `Filename *string`; response writers set `Content-Type` and `Content-Disposition` from that metadata
- multipart bodies generate a `Gen<Operation>MultipartBody` struct; JSON/form parts decode into generated schema types, text parts into `string`, raw bytes into `[]byte`, and `Http.File` parts into `GenFile`
- repeated multipart parts generate slices, optional single parts generate pointers, and `multipart/mixed` tuple parts are decoded in wire order
- generation fails explicitly when an anonymous object body cannot be mapped to a named IR schema
- generated CLI support remains failure-closed for multipart request bodies unless the operation uses an explicit custom override

Generated response writers are content-aware. Single-content responses keep concise names such as `GenGetArtifact200JSONResponse`, `GenGetArtifact200TextResponse`, and `GenGetArtifact200BinaryResponse`. When one status can return multiple media types, APIGen emits one concrete type per content variant using sanitized media names, for example `GenGetArtifact200ApplicationJSONResponse` and `GenGetArtifact200ApplicationOctetStreamResponse`. Each writer sets the authored `Content-Type`.

## Preferred TypeSpec Style

Prefer TypeSpec-native HTTP helpers and aliases over APIGen-shaped response boilerplate:

```typespec
using Http;

model Error {
  code: int32;
  message: string;
}

model OkJson<T> {
  ...OkResponse;
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
  @route("/{id}/blob")
  @put
  op replaceBlob(
    @path id: string,
    @header contentType: "application/octet-stream",
    @body body: bytes,
  ): OkJson<Artifact> | CommonErrors;
}
```

APIGen v0.3.2 follows resolved `@typespec/http` semantics for JSON, text, binary, file, urlencoded form, multipart, optional bodies, response helpers, aliased response unions, and route containers.
Content negotiation can use TypeSpec `@sharedRoute` or `@overload`; APIGen coalesces compatible same-method/same-path operations into one endpoint, merges literal `Accept`/`contentType` headers into enum-like parameters, and fails closed when auth, CLI metadata, parameters, or request bodies disagree.

LibreDash-style contracts should use standard HTTP transport instead of raw-body extensions. Before:

```typespec
model DeploymentArtifactUploadRequest {
  value: bytes;
}

@extension("x-libredash-dispatch", "raw-body")
op uploadDeploymentArtifact(@body body: DeploymentArtifactUploadRequest): UploadDeploymentArtifactOK | BadRequest | Unauthorized | Forbidden;
```

After:

```typespec
alias CommonErrors = BadRequest | Unauthorized | Forbidden;

@route("/api/v1")
namespace Deployments {
  @route("/workspaces/{workspace}/deployments/{deployment}/artifact")
  @put
  op uploadDeploymentArtifact(
    @path workspace: string,
    @path deployment: string,
    @header contentType: "application/octet-stream",
    @body body: bytes,
  ): OkJson<DeploymentArtifactResponse> | CommonErrors;
}
```

## v0.3.2 Migration Notes

- JSON IR changes from `schema_version: "v1"` to `"v2"`.
- `request_body.schema/content_type` becomes `request_body.contents[]`.
- `response.schema/content_type/any_of` becomes `response.contents[]`.
- Generated non-JSON request/response types are no longer named `JSONBody` or `JSONResponse`.
- Multi-content responses now generate media-specific concrete response names such as `ApplicationJSONResponse` and `ApplicationOctetStreamResponse`.
- Raw `bytes` request/response bodies generate `[]byte`; TypeSpec `Http.File` request/response/multipart bodies generate `GenFile`.
- Multipart requests now generate typed multipart body structs and streaming multipart decoding. Repeated parts are slices; optional single parts are pointers; named form-data parts use the TypeSpec part name and mixed tuple parts use wire order.
- `@sharedRoute` and same-endpoint `@overload` operations now coalesce into one APIGen endpoint when their transport metadata is compatible.
- `GenericRequest` inference is removed; name the TypeSpec model you want APIGen to generate.
- v0.3.1 remains the pinned all-JSON release for users who need the old generated shape.

See [`ir/CONTRACT.md`](./ir/CONTRACT.md) for the full IR contract and run `go test ./...` for the module smoke coverage.

# apigen

`apigen` compiles authored API and data contracts into versioned JSON IR, canonical OpenAPI, generated Go server code, generated request-model types, generated Cobra CLI registries, and generated model artifacts.

Module path: `github.com/Yacobolo/toolbelt/apigen`

## Model

APIGen has two contract layers:

- TypeSpec authoring input for humans
- JSON IR `v4` for generators

Canonical OpenAPI is the published API artifact for HTTP targets. JSON IR is the compatibility boundary between TypeSpec and emitters. Repo-owned OpenAPI extensions such as `x-authz` are preserved there. Generic data-contract roots live in the same IR under `contracts[]` and reuse the shared `schemas` registry.

## CLI

Install the CLI:

```bash
go install github.com/Yacobolo/toolbelt/apigen/cmd/apigen@v0.5.0
```

Or run from this module during local development:

```bash
go run ./cmd/apigen --help
```

Commands:

- `typespec-compile`: TypeSpec -> JSON IR, plus OpenAPI for HTTP targets
- `openapi`: JSON IR -> OpenAPI
- `server`: JSON IR -> server + request models
- `cli`: JSON IR -> Cobra registry
- `all`: JSON IR -> all configured outputs

The CLI supports direct flags or a manifest selected with `-manifest <file>` and `-target <name>`.

Recommended grouped manifest shape:

```yaml
targets:
  - name: example
    kind: http
    typespec_dir: api/typespec
    ir_out: api/gen/json-ir.json
    openapi_out: api/gen/openapi.yaml
    go_out:
      dir: internal/api/gen
    cli_out:
      dir: cmd/cli/gen

  - name: data-contracts
    kind: contracts
    typespec_dir: contracts/typespec
    ir_out: contracts/gen/json-ir.json
    go_models_out: internal/contracts/models.gen.go
    go_models_package: contracts
    ts_out: contracts/gen/contracts.ts
    json_schema_out: contracts/gen/contracts.schema.json
```

Manifest target fields:

- `kind` (`http` by default, or `contracts`)
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
- `go_models_out`
- `go_models_package`
- `ts_out`
- `json_schema_out`

HTTP targets require `typespec_dir`, `ir_out`, and `openapi_out`. Contract targets require `typespec_dir`, `ir_out`, and at least one of `go_models_out`, `ts_out`, or `json_schema_out`. `openapi`, `server`, and `cli` are HTTP-target commands and fail clearly for contract targets.

Direct flags support the same split with `-kind http` or `-kind contracts`.

## Public Surface

Supported packages:

- `github.com/Yacobolo/toolbelt/apigen/ir`
- `github.com/Yacobolo/toolbelt/apigen/emit/openapi`
- `github.com/Yacobolo/toolbelt/apigen/emit/requestmodelgo`
- `github.com/Yacobolo/toolbelt/apigen/emit/servergo`
- `github.com/Yacobolo/toolbelt/apigen/emit/cligo`
- `github.com/Yacobolo/toolbelt/apigen/emit/modelgo`
- `github.com/Yacobolo/toolbelt/apigen/emit/modelts`
- `github.com/Yacobolo/toolbelt/apigen/emit/jsonschema`
- `github.com/Yacobolo/toolbelt/apigen/runtime/chi`
- `github.com/Yacobolo/toolbelt/apigen/runtime/cobra`
- `github.com/Yacobolo/toolbelt/apigen/runtime/agenttool`

Package roles:

- `typespec`: TypeSpec emitter package used by `typespec-compile`
- `ir`: versioned generator contract
- `emit/*`: OpenAPI, server, request-model, CLI, model, and JSON Schema emitters
- `runtime/*`: thin runtime helpers used by generated code
- `cmd/apigen`: CLI entrypoint

Public packages must stay isolated from sibling `toolbelt` packages outside `apigen`.

## Using It

Recommended TypeSpec flow:

1. Author API contracts in TypeSpec.
2. Run `typespec-compile` to produce JSON IR and canonical OpenAPI.
3. Run `all` to generate server, request-model, and CLI outputs.
4. Build your service against `runtime/chi` and your CLI against `runtime/cobra`.

The runnable reference showcase lives in `example/`. It is a small todo app with checked-in `json-ir`, OpenAPI, server transport, request-model aliases, CLI registry metadata, handwritten strict handlers, and a generated Cobra CLI. The same example also includes a generic contract target shaped like dashboard UI signal envelopes, with checked-in Go models, TypeScript types, JSON Schema, and IR.

The in-repo TypeSpec emitter lives in `typespec/` with a checked-in `package-lock.json`. Use `npm ci` there for reproducible local TypeSpec development; `typespec-compile` also bootstraps that pinned toolchain when needed. Project TypeSpec sources may use conventional package imports such as `import "@typespec/http";`, `import "@typespec/openapi";`, and `import "@yacobolo/apigen";`; the CLI resolves those imports from its managed cache.

## Typed Agent Tools

Agent tools are endpoint capabilities, not standalone data contracts. Mark a TypeSpec operation with `@apigen.tool`; APIGen derives the model-visible input from its path, query, header, and JSON body fields:

```typespec
@apigen.tool(#{
  name: "list_workspace_assets",
  effect: "read",
  tags: #["workspace", "lineage"],
  input: #{ fields: #[
    #{ source: "path", name: "workspace", mode: "context", contextKey: "workspace" },
    #{ source: "query", name: "limit", default: 25 },
  ] },
  output: #{
    mode: "project",
    select: #[#{ source: "/items", countAs: "count", select: #[#{ source: "/id" }, #{ source: "/title" }] }],
    cursor: #{ source: "/page/nextCursor" },
  },
})
@route("/workspaces/{workspace}/assets")
@get
op listWorkspaceAssets(
  @path workspace: string,
  @query limit?: int32,
): AssetListResponse;
```

Effects are `read`, `idempotent-write`, `write`, and `destructive`. Their minimum confirmation defaults are `never`, `policy`, `policy`, and `always`; authored confirmation may strengthen but never weaken that requirement. Tool names are portable lowercase identifiers and unique across the document.

Input overrides bind endpoint wire fields as model arguments, trusted context, or omitted/defaulted transport values. Tool endpoints accept no body or one JSON body; binary, file, form, and multipart inputs fail closed. Output modes are `raw`, `project`, and `empty`; recursive RFC 6901 projections support object fields, array items, map values, aliases, counts, and cursors.

Generated model-visible schemas use a provider-portable validation subset. Defaults and transport formats remain on typed bindings rather than appearing as `default` or `format` schema annotations.

Generated server packages expose defensive copies of SDK-neutral descriptors:

```go
contract, ok := gen.GetAPIGenToolContract("list_workspace_assets")
if ok {
	request, err := agenttool.BuildRequest(
		contract,
		json.RawMessage(`{"limit":10}`),
		agenttool.Context{"workspace": "sales"},
	)
	_ = request
	_ = err
}
```

`runtime/agenttool` strictly validates arguments, builds HTTP requests, preserves non-2xx responses, and projects successful JSON responses. APIGen remains provider-neutral: authorization, credentials, policy decisions, confirmation UI, agent SDK conversion, and operation dispatch stay in the consumer. Canonical OpenAPI publishes normalized descriptors as `x-apigen-tool`.

Generic operation `x-*` extensions remain available for downstream metadata. `x-agent` is reserved and rejected; there is no raw compatibility parser.

Install as a dependency with:

```bash
go get github.com/Yacobolo/toolbelt/apigen@v0.5.0
```

## Contract Notes

JSON IR emits and accepts schema version `v4` only. Required root fields are `schema_version`, `info.title`, `info.version`, and at least one endpoint or contract root. Request and response bodies use ordered `contents` entries with explicit `content_type` and `body_kind`. Schema composition uses `base`, `one_of`, and `discriminator`; map value schemas remain in `additional_properties`. Endpoint extensions preserve operation-level `x-*` vendor metadata; APIGen-owned endpoint extensions include `x-authz` and `x-apigen-manual`. Typed tools live on `Endpoint.tool` and never create `contracts[]` entries.

HTTP targets can declare generator-owned failures explicitly with `@apigen.transportErrors`. Generated strict registration requires a `GenTransportErrorResponder`; the responder owns the authored wire model, media type, request IDs, logging, and other application policy. Generated code supplies a stable failure kind, configured status/code/public detail, and the original cause without exposing that cause to clients.

Contract targets use the shared `schemas` registry plus `contracts[]` roots:

```json
{
  "name": "DashboardEnvelope",
  "schema": {"ref": "DashboardEnvelope"},
  "kind": "ui-signal",
  "tags": ["dashboard"],
  "extensions": {"x-libredash-surface": "dashboard"}
}
```

Schema and schema-property `extensions` preserve downstream-owned `x-*` metadata. APIGen validates that metadata is JSON-compatible and has `x-*` keys, but does not interpret downstream rules.

Contract TypeSpec sources can use APIGen decorators:

```typespec
import "@yacobolo/apigen";

@apigen.`package`(#{ title: "Data Contracts", version: "1.0.0" })
namespace Contracts;

@apigen.contract(#{ kind: "ui-signal", tags: #["dashboard"] })
@apigen.`metadata`(#{ "x-owner": "analytics" })
model DashboardEnvelope {
  @apigen.`metadata`(#{ "x-libredash-signal-key": "page" })
  page: DashboardPageSignal;
}
```

The contract emitters generate selected contract roots and transitive dependencies:

- `emit/modelgo`: Go structs and aliases; optional TypeSpec properties become pointers.
- `emit/modelts`: TypeScript interfaces and aliases; optional properties use `?`.
- `emit/jsonschema`: draft 2020-12 JSON Schema with `$defs`, `anyOf` contract roots, required fields, maps, arrays, enums, and metadata extensions.

Endpoint parameters support TypeSpec path, query, and header parameters across IR, OpenAPI, generated server binding, and generated CLI flags. Cookie parameters intentionally fail closed.

Supported TypeSpec auth is intentionally narrow and runtime-backed: HTTP Bearer auth and `ApiKeyAuth<ApiKeyLocation.header, "X-API-Key">`. Basic/Digest/custom HTTP schemes, OAuth/OpenID, non-header API keys, and header API keys with other names fail closed instead of emitting misleading runtime or OpenAPI metadata.

Generated request bodies are contract-first:

- JSON and form object bodies used in generated Go output should resolve to named IR-owned schemas
- text bodies generate `string`, raw `bytes` bodies generate `[]byte`, and TypeSpec `Http.File` bodies generate `GenFile`
- `GenFile` carries `Contents []byte`, optional streaming `Reader io.ReadCloser`, `ContentType string`, optional `Filename *string`, and optional `Size *int64`; response writers stream `Reader` when present and set `Content-Type`/`Content-Disposition` from that metadata
- raw `Http.File` request bodies pass `r.Body` through as `GenFile.Reader`; multipart `Http.File` parts spool to temporary files and are cleaned up after the handler returns
- multipart bodies generate a `Gen<Operation>MultipartBody` struct; JSON/form parts decode into generated schema types, text parts into `string`, raw bytes into `[]byte`, and `Http.File` parts into streaming `GenFile`
- repeated multipart parts generate slices, optional single parts generate pointers, and `multipart/mixed` tuple parts are decoded in wire order
- generated multipart server decoding is strict: unknown form-data part names, duplicate non-repeated form-data parts, and extra mixed tuple parts return `400`
- generation fails explicitly when an anonymous object body cannot be mapped to a named IR schema
- generated CLI supports multipart request bodies with repeated `--part name=value`, `--part name=@file`, or `--part name=-` flags; binary and file parts require `@file` or stdin

Generated response writers are content-aware. Single-content responses keep concise names such as `GenGetArtifact200JSONResponse`, `GenGetArtifact200TextResponse`, and `GenGetArtifact200BinaryResponse`. When one status can return multiple media types, APIGen emits one concrete type per content variant using sanitized media names, for example `GenGetArtifact200ApplicationJSONResponse` and `GenGetArtifact200ApplicationOctetStreamResponse`. Each writer sets the authored `Content-Type`. Identical duplicate content variants are deduplicated; incompatible same-status variants with the same `content_type` fail closed instead of being approximated with `anyOf`.

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

APIGen follows resolved `@typespec/http` semantics for JSON, text, binary, file, urlencoded form, multipart, optional bodies, response helpers, aliased response unions, and route containers.
Content negotiation can use TypeSpec `@sharedRoute` or `@overload`; APIGen coalesces compatible same-method/same-path operations into one endpoint, merges literal `Accept`/`contentType` headers into enum-like parameters, and fails closed when auth, APIGen CLI/authz/manual metadata, operation extensions, parameters, or request bodies disagree.

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

## v0.5.0 Migration Notes

- JSON IR `v4` is the only accepted IR version; v3 is intentionally not loaded.
- TypeSpec inheritance and discriminated unions now remain explicit through IR, OpenAPI, Go, TypeScript, JSON Schema, and agent-tool schemas.
- Generated Go union wrappers strictly reject missing/unknown discriminators, unknown fields, and missing required variant properties.
- `Record<T>` retains `T` recursively in generated Go models, including nested arrays and maps.
- `int64` remains `int64` in every generated Go model path.
- Generated strict server registration now requires an injected `GenTransportErrorResponder`; legacy shared `Error` response helpers and the hard-coded writer were removed.
- Regenerate all checked-in IR and generated artifacts when upgrading.

See [`ir/CONTRACT.md`](./ir/CONTRACT.md) for the full IR contract and run `go test ./...` for the module smoke coverage.

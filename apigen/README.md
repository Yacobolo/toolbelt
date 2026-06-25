# apigen

`apigen` compiles authored API contracts into canonical OpenAPI, versioned JSON IR, generated Go server code, generated request-model types, and generated Cobra CLI registries.

Module path: `github.com/Yacobolo/toolbelt/apigen`

## Model

APIGen has two contract layers:

- TypeSpec authoring input for humans
- JSON IR `v1` for generators

Canonical OpenAPI is the published API artifact. JSON IR is the compatibility boundary between TypeSpec and the Go emitters. Repo-owned OpenAPI extensions such as `x-authz` are preserved there.

## CLI

Install the CLI:

```bash
go install github.com/Yacobolo/toolbelt/apigen/cmd/apigen@v0.3.0
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
go get github.com/Yacobolo/toolbelt/apigen@v0.3.0
```

## Contract Notes

JSON IR currently supports schema version `v1`. Required root fields are `schema_version`, `info.title`, `info.version`, and at least one endpoint. Endpoint extensions preserve operation-level `x-*` vendor metadata; APIGen-owned endpoint extensions include `x-authz` and `x-apigen-manual`. Supported response extensions include `x-apigen-response-shape`.

Generated request bodies are contract-first:

- request bodies used in generated server and request-model output must resolve to named IR-owned schemas
- generation fails explicitly when a request body cannot be mapped to a named IR schema

See [`ir/CONTRACT.md`](./ir/CONTRACT.md) for the full IR contract and run `go test ./...` for the module smoke coverage.

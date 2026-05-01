# apigen

`apigen` compiles authored API contracts into canonical OpenAPI, versioned JSON IR, generated Go server code, generated request-model compatibility types, and generated Cobra CLI registries.

Module path: `github.com/Yacobolo/toolbelt/apigen`

## Model

APIGen has two contract layers:

- CUE authoring input for humans
- JSON IR `v1` for generators

Canonical OpenAPI is the published API artifact. JSON IR is the compatibility boundary between the compiler and the Go emitters. Repo-owned OpenAPI extensions such as `x-authz` are preserved there.

## CLI

Run from this module:

```bash
go run ./cmd/apigen --help
```

Commands:

- `cue-compile`: CUE -> JSON IR + OpenAPI
- `cue-bootstrap`: JSON IR -> starter CUE files
- `openapi`: JSON IR -> OpenAPI
- `server`: JSON IR -> server + request models + optional compat types
- `cli`: JSON IR -> Cobra registry
- `all`: JSON IR -> all Go outputs

The CLI supports direct flags or a manifest selected with `-manifest <file>` and `-target <name>`.

When `compat_types_out` is enabled, request-body compatibility aliases must resolve from named IR-owned schemas. Split-package compat generation does not depend on server-package-only `Gen<Operation>IDJSONBody` aliases.

Manifest target fields:

- `cue_dir`
- `ir_out`
- `openapi_out`
- `server_out`
- `server_package`
- `request_models_out`
- `request_models_package`
- `compat_types_out`
- `compat_types_package`
- `cli_out`
- `cli_package`
- `generate_cli`

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

- `cuegen`: compile and bootstrap CUE
- `ir`: versioned generator contract
- `emit/*`: OpenAPI, server, request-model, and CLI emitters
- `runtime/*`: thin runtime helpers used by generated code
- `cmd/apigen`: CLI entrypoint

Public packages must stay isolated from sibling `toolbelt` packages outside `apigen`.

## Using It

Typical flow:

1. Author API contracts in CUE.
2. Run `cue-compile` to produce JSON IR and canonical OpenAPI.
3. Run `all` to generate server, request-model, compat-type, and CLI outputs.
4. Build your service against `runtime/chi` and your CLI against `runtime/cobra`.

The runnable reference consumer lives in `examples/example_consumer`.

Install as a dependency with:

```bash
go get github.com/Yacobolo/toolbelt/apigen
```

## Contract Notes

JSON IR currently supports schema version `v1`. Required root fields are `schema_version`, `info.title`, `info.version`, and at least one endpoint. Supported endpoint extensions include `x-authz` and `x-apigen-manual`; supported response extensions include `x-apigen-response-shape`.

For split-package generation, compat request-body aliases are contract-first:

- compat output may reference `GenSchema...` symbols derived from IR
- compat output must not reference server-only `Gen<Operation>IDJSONBody` symbols
- if a request body cannot be resolved to a named IR schema, compat generation fails explicitly

See [`ir/CONTRACT.md`](./ir/CONTRACT.md) for the full IR contract and run `go test ./...` for the module smoke and compatibility coverage.

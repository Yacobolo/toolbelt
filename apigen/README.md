# apigen

`apigen` compiles authored API contracts into canonical OpenAPI, versioned JSON IR, generated Go server code, generated request-model types, and generated Cobra CLI registries.

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
- `server`: JSON IR -> server + request models
- `cli`: JSON IR -> Cobra registry
- `all`: JSON IR -> all Go outputs

The CLI supports direct flags or a manifest selected with `-manifest <file>` and `-target <name>`.

Recommended grouped manifest shape:

```yaml
targets:
  - name: example
    cue_dir: api/cue
    ir_out: api/gen/json-ir.json
    openapi_out: api/gen/openapi.yaml
    go_out:
      dir: internal/api/gen
    cli_out:
      dir: cmd/cli/gen
```

Manifest target fields:

- `cue_dir`
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
3. Run `all` to generate server, request-model, and CLI outputs.
4. Build your service against `runtime/chi` and your CLI against `runtime/cobra`.

The runnable reference showcase lives in `example/`. It is a small todo app with checked-in `json-ir`, OpenAPI, server transport, request-model aliases, CLI registry metadata, handwritten strict handlers, and a generated Cobra CLI.

Install as a dependency with:

```bash
go get github.com/Yacobolo/toolbelt/apigen
```

## Contract Notes

JSON IR currently supports schema version `v1`. Required root fields are `schema_version`, `info.title`, `info.version`, and at least one endpoint. Supported endpoint extensions include `x-authz` and `x-apigen-manual`; supported response extensions include `x-apigen-response-shape`.

Generated request bodies are contract-first:

- request bodies used in generated server and request-model output must resolve to named IR-owned schemas
- generation fails explicitly when a request body cannot be mapped to a named IR schema

See [`ir/CONTRACT.md`](./ir/CONTRACT.md) for the full IR contract and run `go test ./...` for the module smoke coverage.

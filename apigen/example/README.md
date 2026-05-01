# example

This is the canonical APIGen showcase: a small in-memory todo app with authored CUE input, checked-in generated artifacts, a handwritten strict server, and a generated CLI.

## Handwritten files

- `api/cue/`: authored todo contract
- `apigen.targets.yaml`: regeneration target manifest
- `cmd/server/main.go`: strict server implementation and `/openapi.json` utility route
- `cmd/cli/main.go`: tiny Cobra root that mounts generated commands

## Generated files

- `api/gen/json-ir.json`
- `api/gen/openapi.yaml`
- `internal/api/server.apigen.gen.go`
- `internal/api/gen_request_models.gen.go`
- `internal/api/types.gen.go`
- `cmd/cli/gen/apigen_registry.gen.go`

## Regenerate

From `apigen/`:

```bash
go run ./cmd/apigen cue-compile -manifest ./example/apigen.targets.yaml -target example
go run ./cmd/apigen all -manifest ./example/apigen.targets.yaml -target example
```

## Run

From `example/`:

```bash
go run ./cmd/server
go run ./cmd/cli todos list
go run ./cmd/cli todos list --status completed
go run ./cmd/cli todos create "buy milk"
go run ./cmd/cli todos get todo-1
go run ./cmd/cli todos complete todo-1
go run ./cmd/cli todos delete todo-1 --yes
```

The server starts with two seeded todos so the example is immediately explorable before you create anything new.

Optional:

- `TODO_EXAMPLE_ADDR` overrides the server listen address
- `TODO_EXAMPLE_BASE_URL` or `--base-url` overrides the CLI target URL

## What this shows

- CUE -> JSON IR -> OpenAPI -> generated Go artifacts
- strict handler integration via `RegisterAPIGenStrictRoutes`
- generated request and response types in handwritten business logic
- generated Cobra commands with path args, query params, JSON body input, detail output, collection output, and confirmation

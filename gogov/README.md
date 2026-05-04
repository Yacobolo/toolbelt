# gogov

`gogov` is a local-first Go repository catalog and governance explorer.

Module path: `github.com/Yacobolo/toolbelt/gogov`

## What It Does

- scans one or more local Go repositories from a config file
- stores snapshots per repository in SQLite
- serves a multi-repo catalog for files, packages, lineage, and source
- keeps working even when a repository does not fully compile

## Run It

From this module:

```bash
go run ./cmd/gogov serve
```

Refresh a single configured repository:

```bash
go run ./cmd/gogov refresh --repo ai-platform
```

## Config

`gogov` reads `.governance.yaml` from the module root.

Example:

```yaml
bind_address: "127.0.0.1:8787"
repositories:
  - "/Users/yacobolo/dev/ai-platform"
  - "/Users/yacobolo/dev/duck-demo"
```

## Frontend

Prebuilt frontend assets are checked in under `governance/web/static`.

To install local frontend dependencies for rebuilding assets:

```bash
pnpm --dir web install
```

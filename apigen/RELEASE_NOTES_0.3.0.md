# APIGen 0.3.0 Release Notes

## Breaking Changes

- TypeSpec is now the only supported APIGen authoring language.
- The CUE compiler and bootstrapper have been removed.
- The `cue-compile` and `cue-bootstrap` CLI commands have been removed.
- Manifest targets must use `typespec_dir`; `cue_dir` is no longer supported.
- The `cuegen` Go package has been removed.

## Migration

Before:

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

After:

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

Regenerate artifacts with:

```bash
go run ./cmd/apigen typespec-compile -manifest ./example/apigen.targets.yaml -target example
go run ./cmd/apigen all -manifest ./example/apigen.targets.yaml -target example
```

## Notes

- JSON IR remains schema version `v1`.
- Existing IR-based generation commands remain: `openapi`, `server`, `cli`, and `all`.
- The TypeSpec emitter package and bundled `dist/src` output remain checked in and validated by `npm run check:dist`.

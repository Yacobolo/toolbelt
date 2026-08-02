# toolbelt

This repo is a monorepo for standalone developer tools and supporting modules.

## Tools

- [`apigen`](./apigen/README.md): compile TypeSpec API contracts into OpenAPI, JSON IR, generated Go server code, and Cobra CLI registries.
- [`datastar-url-sync`](./datastar-url-sync/README.md): extracted Datastar pattern for syncing a `urlParams` signal with browser query params.
- [`gogov`](./gogov/README.md): local-first multi-repository Go catalog for files, packages, lineage, and source.
- [`pagestream`](./pagestream/README.md): small Go wrapper around Datastar for signal-only, stream-first pages.
- [`pi-workers`](./pi-workers/README.md): Codex-shaped CLI and STDIO MCP server for orchestrating Pi subagents.
- [`sourcebook`](./sourcebook/README.md): maintain one Codex skill as a table of contents over shallow-cloned reference repositories.

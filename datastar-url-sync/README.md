# datastar-url-sync

Small extracted example of syncing Datastar signals with URL query params.

This example mirrors the pattern used in the Explore surface:

- a `urlParams` Datastar signal stores query state
- the Go view seeds `urlParams` from the request URL
- the sync bridge is explicitly scoped as `window.DatastarURLSync.urlParams`
- TypeScript helpers normalize and serialize query params
- Datastar event handlers update the signal and write the URL
- `popstate` events hydrate the signal back from the URL
- the demo page itself is rendered with gomponents and gomponents-datastar

## Files

- `server.go`: Go server and gomponents-rendered demo page
- `src/main.ts`: single-file TypeScript implementation for the URL param helpers and the Datastar `urlParams` sync bridge
- `dist/main.js`: built browser module
- `go.mod`: local Go module for gomponents dependencies

## Run

Start the demo server:

```bash
cd /Users/yacobolo/dev/toolbelt/datastar-url-sync
task tidy
npm install
task build
task serve
```

Then open:

- `http://localhost:4173`

## Rebuild

```bash
cd /Users/yacobolo/dev/toolbelt/datastar-url-sync
task tidy
npm install
task build
```

## What To Try

1. Type into the search box and toggle filters.
2. Watch the `urlParams` signal and the browser URL stay in sync.
3. Click `Push current state to history`.
4. Change filters again, then use the browser back button.
5. Watch the signal rehydrate from `window.location.search`.

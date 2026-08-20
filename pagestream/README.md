# pagestream

`pagestream` is a small Go wrapper around Datastar for stream-first,
server-rendered pages. It provides:

- a Gomponents document shell that opens one same-origin Datastar update stream;
- signal-only SSE patching and redirects;
- request signal decoding and anonymous client IDs;
- an in-process signal broker with explicit delivery boundaries; and
- optional bounded delivery tracing.

The package does not define routes, authentication, application signals,
commands, or UI components. Applications own those boundaries and use the
database or another durable store as their source of truth.

```go
page := pagestream.RenderPage(pagestream.PageSpec{
	Title:             "Console",
	DatastarScriptURL: "/assets/datastar.js",
	UpdatesURL:        "/updates?route=console",
	Body:              []gomponents.Node{html.Div(html.ID("console"))},
})

func updates(w http.ResponseWriter, r *http.Request) {
	stream := pagestream.NewSignalStream(w, r)
	_ = stream.Patch(pagestream.SignalPatch{"status": "ready"})
}
```

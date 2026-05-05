package main

import (
	"log"
	"net/http"
	"strings"
	"os"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/components"
	ghttp "maragu.dev/gomponents/http"
	. "maragu.dev/gomponents/html"
	data "maragu.dev/gomponents-datastar"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "4173"
	}
	addr := "127.0.0.1:" + port
	log.Printf("serving datastar-url-sync at http://%s\n", addr)

	mux := http.NewServeMux()
	mux.Handle("/dist/", http.StripPrefix("/dist/", http.FileServer(http.Dir("./dist"))))
	mux.HandleFunc("/", ghttp.Adapt(func(w http.ResponseWriter, r *http.Request) (Node, error) {
		return page(r), nil
	}))

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Fatal(server.ListenAndServe())
}

func page(r *http.Request) Node {
	signals := initialSignals(r)
	return HTML5(HTML5Props{
		Title:    "Datastar URL Sync",
		Language: "en",
		Head: []Node{
			Meta(Charset("utf-8")),
			Meta(Name("viewport"), Content("width=device-width, initial-scale=1")),
			Script(Type("module"), Src("/dist/main.js")),
			Script(
				Type("module"),
				Src("https://cdn.jsdelivr.net/gh/starfederation/datastar@v1.0.0-RC.7/bundles/datastar.js"),
			),
			StyleEl(Raw(pageStyles)),
		},
		Body: []Node{
			Main(
				hero(),
				demoShell(signals),
			),
		},
	})
}

func hero() Node {
	return Section(Class("hero"),
		P(Class("eyebrow"), Text("Datastar Pattern")),
		H1(Text("URL params as a signal")),
		P(Class("lede"),
			Text("A clean implementation for keeping a "),
			Code(Text("urlParams")),
			Text(" signal in sync with the browser's address bar. The page seeds the signal from the URL, writes URL changes from Datastar events, and listens for "),
			Code(Text("popstate")),
			Text(" so browser navigation can hydrate the signal back. Perfect for filterable dashboards and search interfaces."),
		),
	)
}

func demoShell(signals map[string]any) Node {
	return Section(
		Class("shell"),
		data.Signals(signals),
		data.Init("$currentURL = window.location.pathname + window.location.search; window.DatastarURLSync.urlParams.bindPopstate($urlParams)"),
		data.On("datastar-url-params-sync", "$urlParams = evt.detail.params; $currentURL = evt.detail.url; $historyStatus = 'Hydrated signal from browser history via popstate.'", data.ModifierWindow),
		Div(
			Class("card controls"),
			searchField(),
			Div(Class("filter-grid"),
				checklistField("Kinds", "kind", []string{"dashboard", "model", "pipeline"}),
				checklistField("Owners", "owner", []string{"alice", "bob", "charlie"}),
			),
			actionRow(),
			exampleLinks(),
			statusPanels(),
		),
	)
}

func initialSignals(r *http.Request) map[string]any {
	return map[string]any{
		"urlParams": map[string]any{
			"q":     strings.TrimSpace(r.URL.Query().Get("q")),
			"kind":  queryValues(r, "kind"),
			"owner": queryValues(r, "owner"),
		},
		"currentURL":    "",
		"historyStatus": "Using replaceState until you push a snapshot.",
	}
}

func queryValues(r *http.Request, key string) []string {
	rawValues := r.URL.Query()[key]
	if len(rawValues) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(rawValues))
	out := make([]string, 0, len(rawValues))
	for _, raw := range rawValues {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	return out
}

func searchField() Node {
	return Div(Class("field"),
		Label(For("search"), Class("label"), Text("Search Query")),
		Input(
			ID("search"),
			Type("search"),
			Placeholder("Search components..."),
			data.Bind("urlParams.q"),
			data.On("input", "$currentURL = window.DatastarURLSync.urlParams.replace($urlParams); $historyStatus = 'Replaced current URL from the urlParams signal.'", data.ModifierDebounce, data.Duration(150000000)),
		),
	)
}

func checklistField(title, key string, values []string) Node {
	return Div(Class("field"),
		Span(Class("label"), Text(title)),
		Div(Class("checklist"),
			Map(values, func(value string) Node {
				return Label(Class("pill"),
					Input(
						Type("checkbox"),
						Name(key),
						Value(value),
						data.Attr("checked", "$urlParams."+key+".includes('"+value+"') ? true : null"),
						data.On("change", "$urlParams = window.DuckUIURLParams.toggleArrayValue($urlParams, evt.target.name, evt.target.value, evt.target.checked); $currentURL = window.DatastarURLSync.urlParams.replace($urlParams); $historyStatus = 'Replaced current URL from the urlParams signal.'"),
					),
					Text(displayLabel(value)),
				)
			}),
		),
	)
}

func actionRow() Node {
	return Div(Class("actions"),
		Button(
			Type("button"),
			data.On("click", "$urlParams = window.DuckUIURLParams.clear($urlParams, ['q', 'kind', 'owner']); $currentURL = window.DatastarURLSync.urlParams.replace($urlParams); $historyStatus = 'Cleared filters and replaced the current URL from the urlParams signal.'"),
			Text("Clear filters"),
		),
		Button(
			Type("button"),
			Class("secondary"),
			data.On("click", "$currentURL = window.DatastarURLSync.urlParams.push($urlParams); $historyStatus = 'Pushed the current urlParams signal state into browser history.'"),
			Text("Push current state to history"),
		),
	)
}

func exampleLinks() Node {
	return Div(Class("example-links"),
		A(Href("/?q=warehouse&kind=dashboard&owner=alice"), Text("Quick link: Alice + Dashboard")),
		A(Href("/?q=metrics&kind=model&owner=bob"), Text("Quick link: Bob + Model")),
	)
}

func statusPanels() Node {
	return Div(Class("status"),
		Div(Class("status-grid"),
			Div(Class("panel"),
				H2(Text("Current URL")),
				Code(data.Text("$currentURL || window.location.pathname")),
			),
			Div(Class("panel"),
				H2(Text("Browser Status")),
				Div(Class("status-copy"), data.Text("$historyStatus")),
			),
			Div(Class("panel panel--wide"),
				H2(Text("Signal preview")),
				Pre(data.JSONSignals(data.Filter{Include: "/^urlParams/"})),
			),
		),
	)
}

func displayLabel(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

const pageStyles = `
:root {
  color-scheme: light;
  --bg: #f8f6f2;
  --surface: #ffffff;
  --surface-strong: #f1ede4;
  --border: #e2d9c8;
  --border-focus: #0d6b6b;
  --ink: #1a1612;
  --muted: #73685a;
  --accent: #0d6b6b;
  --accent-soft: #e6f2f1;
  --shadow: 0 2px 4px rgba(31, 26, 20, 0.04), 0 12px 32px rgba(31, 26, 20, 0.08);
  --radius-lg: 20px;
  --radius-md: 12px;
  --radius-sm: 8px;
}

* {
  box-sizing: border-box;
  -webkit-font-smoothing: antialiased;
}

body {
  margin: 0;
  background:
    radial-gradient(circle at 0% 0%, rgba(13, 107, 107, 0.05), transparent 40%),
    var(--bg);
  color: var(--ink);
  font:
    16px/1.5 "Iowan Old Style", "Palatino Linotype", "Book Antiqua", Palatino, Georgia, serif;
  padding-bottom: 5rem;
}

main {
  width: min(900px, calc(100vw - 40px));
  margin: 0 auto;
  padding-top: 80px;
}

.hero {
  margin-bottom: 48px;
}

.eyebrow {
  display: inline-block;
  margin-bottom: 12px;
  color: var(--accent);
  font:
    700 0.7rem/1 "Inter", "Avenir Next Condensed", "Arial Narrow", sans-serif;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  padding: 4px 8px;
  background: var(--accent-soft);
  border-radius: 4px;
}

h1 {
  margin: 0;
  font-size: clamp(2.5rem, 5vw, 4rem);
  line-height: 1.1;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.lede {
  max-width: 55ch;
  margin: 20px 0 0;
  color: var(--muted);
  font-size: 1.15rem;
}

.shell {
  display: grid;
}

.card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow);
  overflow: hidden;
}

.controls {
  padding: 32px;
  display: grid;
  gap: 32px;
}

.field {
  display: grid;
  gap: 12px;
}

.filter-grid {
  display: grid;
  gap: 20px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.label {
  font:
    700 0.75rem/1 "Inter", sans-serif;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--muted);
}

input[type="search"] {
  width: 100%;
  padding: 14px 18px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--bg);
  color: var(--ink);
  font-family: inherit;
  font-size: 1rem;
  transition: all 0.2s ease;
}

input[type="search"]:focus {
  outline: none;
  border-color: var(--accent);
  background: white;
  box-shadow: 0 0 0 4px var(--accent-soft);
}

.checklist {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.pill {
  position: relative;
  display: inline-flex;
  align-items: center;
  padding: 8px 16px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: white;
  cursor: pointer;
  font: 600 0.9rem/1 "Inter", sans-serif;
  transition: all 0.2s ease;
  user-select: none;
}

.pill:has(input:checked) {
  background: var(--accent);
  color: white;
  border-color: var(--accent);
}

.pill:hover:not(:has(input:checked)) {
  background: var(--surface-strong);
  border-color: var(--muted);
}

.pill input {
  position: absolute;
  opacity: 0;
  cursor: pointer;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 8px;
}

button,
a.button-link {
  appearance: none;
  border: 0;
  border-radius: var(--radius-md);
  padding: 14px 24px;
  background: var(--ink);
  color: white;
  cursor: pointer;
  font:
    700 0.8rem "Inter", "Avenir Next Condensed", "Arial Narrow", sans-serif;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  transition: transform 0.1s ease, opacity 0.2s ease;
}

button:hover {
  opacity: 0.9;
}

button:active {
  transform: translateY(1px);
}

button.secondary,
a.button-link.secondary {
  background: var(--accent-soft);
  color: var(--accent);
}

.status {
  background: #fdfcfb;
  border-top: 1px solid var(--border);
  padding: 32px;
}

.status-grid {
  display: grid;
  gap: 20px;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
}

.panel {
  padding: 20px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: white;
}

.panel--wide {
  grid-column: 1 / -1;
}

.panel h2 {
  margin: 0 0 12px;
  font: 700 0.7rem "Inter", sans-serif;
  text-transform: uppercase;
  color: var(--muted);
}

code,
pre {
  font:
    0.85rem/1.6 "JetBrains Mono", "SFMono-Regular", SFMono-Regular, ui-monospace, Menlo, Consolas, monospace;
  color: var(--accent);
}

pre {
  margin: 0;
  white-space: pre-wrap;
  background: var(--accent-soft);
  padding: 12px;
  border-radius: var(--radius-sm);
  overflow-x: auto;
}

.example-links {
  display: flex;
  gap: 16px;
  font-family: "Inter", sans-serif;
  font-size: 0.85rem;
}

.example-links a {
  color: var(--accent);
  text-decoration: none;
  border-bottom: 1px solid transparent;
}

.example-links a:hover {
  border-bottom-color: var(--accent);
}

.status-copy {
  font-family: "Inter", sans-serif;
  font-size: 0.9rem;
}

@media (max-width: 720px) {
  main {
    width: min(100vw - 24px, 900px);
    padding-top: 48px;
  }

  .controls,
  .status {
    padding: 20px;
  }

  .filter-grid {
    grid-template-columns: 1fr;
  }

  .example-links {
    flex-direction: column;
    gap: 8px;
  }
}
`

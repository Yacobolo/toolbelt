package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	lucide "github.com/eduardolat/gomponents-lucide"
	. "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"
	ghttp "maragu.dev/gomponents/http"
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
			Meta(Name("color-scheme"), Content("light dark")),
			Link(
				Rel("stylesheet"),
				Href("https://cdn.jsdelivr.net/npm/@picocss/pico@2.1.1/css/pico.min.css"),
			),
			Script(Type("module"), Src("/dist/main.js")),
			Script(
				Type("module"),
				Src("https://cdn.jsdelivr.net/gh/starfederation/datastar@v1.0.0-RC.7/bundles/datastar.js"),
			),
		},
		Body: []Node{
			Div(
				data.Signals(signals),
				data.Init(pageInitExpression()),
				data.On("datastar-url-params-sync", "$urlParams = evt.detail.params; $currentURL = evt.detail.url", data.ModifierWindow),
				Header(Class("container"),
					topNav(),
				),
				Main(Class("container"),
					demoShell(),
				),
				Footer(Class("container"),
					Small(
						Text("Built with Datastar • No custom CSS"),
					),
				),
			),
		},
	})
}

func topNav() Node {
	return Nav(
		Ul(
			Li(
				HGroup(
					H1(withIcon(lucide.Link, "URL Signals")),
					P(Text("Real-time address bar synchronization")),
				),
			),
		),
		Ul(
			Li(themeSwitcher()),
		),
	)
}

func themeSwitcher() Node {
	return Button(
		Type("button"),
		Class("outline secondary"),
		data.On("click", themeCycleExpression()),
		Attr("aria-label", "Theme"),
		Title("Theme"),
		themeTriggerIcon(false, "$themeMode != 'auto'", lucide.Monitor),
		themeTriggerIcon(true, "$themeMode != 'light'", lucide.Sun),
		themeTriggerIcon(true, "$themeMode != 'dark'", lucide.Moon),
	)
}

func demoShell() Node {
	return Section(
		data.OnSignalPatch("$currentURL = window.DatastarURLSync.replace($urlParams)", data.ModifierDebounce, data.Duration(150000000)),
		data.OnSignalPatchFilter(data.Filter{Include: "/^urlParams/"}),
		Div(Class("grid"),
			Aside(
				sidebar(),
			),
			Section(
				quickLinksCard(),
				signalDebugger(),
			),
		),
		currentURLPanel(),
	)
}

func initialSignals(r *http.Request) map[string]any {
	return map[string]any{
		"urlParams": map[string]any{
			"q":     strings.TrimSpace(r.URL.Query().Get("q")),
			"kind":  queryValues(r, "kind"),
			"owner": queryValues(r, "owner"),
		},
		"currentURL": "",
		"pushedURL":  "",
		"themeMode":  "auto",
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
	return Label(
		For("search"),
		Text("Query"),
		Input(
			ID("search"),
			Type("search"),
			Placeholder("Search..."),
			data.Bind("urlParams.q"),
		),
	)
}

func checklistField(title, key string, values []string) Node {
	return FieldSet(
		Legend(Strong(Text(title))),
		Map(values, func(value string) Node {
			id := key + "-" + value
			return Label(
				For(id),
				Input(
					ID(id),
					Type("checkbox"),
					Name(key),
					Value(value),
					Role("switch"),
					data.Attr("checked", "$urlParams."+key+".includes('"+value+"') ? true : null"),
					data.On("change", "$urlParams = window.DuckUIURLParams.toggleInput($urlParams, evt.target)"),
				),
				Text(displayLabel(value)),
			)
		}),
	)
}

func actionRow() Node {
	return Div(Class("grid"),
		Button(
			Type("button"),
			Class("contrast"),
			data.Attr("disabled", "$currentURL == $pushedURL ? true : null"),
			data.On("click", "$currentURL = window.DatastarURLSync.push($urlParams); $pushedURL = $currentURL"),
			withButtonIcon(lucide.History, "Push to history"),
		),
		Button(
			Type("button"),
			Class("outline secondary"),
			data.On("click", "$urlParams = window.DuckUIURLParams.clear($urlParams, ['q', 'kind', 'owner'])"),
			withButtonIcon(lucide.Eraser, "Clear all"),
		),
	)
}

func sidebar() Node {
	return Article(
		HGroup(
			H3(Text("Search & Filter")),
			P(Text("Real-time URL sync")),
		),
		Form(
			searchField(),
			checklistField("Categories", "kind", []string{"dashboard", "model", "pipeline"}),
			checklistField("Assignee", "owner", []string{"alice", "bob", "charlie"}),
			actionRow(),
		),
	)
}

func quickLinksCard() Node {
	return Article(
		Header(Strong(Text("Quick Links"))),
		Div(Class("grid"),
			quickLink(
				"/?q=metrics&kind=model",
				"",
				"$urlParams = { q: 'metrics', kind: ['model'], owner: [] }",
				"Metrics + Model",
			),
			quickLink(
				"/?q=warehouse&kind=dashboard&owner=alice",
				"secondary",
				"$urlParams = { q: 'warehouse', kind: ['dashboard'], owner: ['alice'] }",
				"Alice + Dashboard",
			),
			quickLink(
				"/?kind=pipeline&owner=bob",
				"contrast",
				"$urlParams = { q: '', kind: ['pipeline'], owner: ['bob'] }",
				"Bob + Pipeline",
			),
		),
	)
}

func quickLink(href, className, expression, label string) Node {
	return A(
		Href(href),
		If(className != "", Class(className)),
		data.On("click", expression, data.ModifierPrevent),
		Text(label),
	)
}

func signalDebugger() Node {
	return Article(
		Header(Strong(Text("Active Signal State"))),
		Pre(data.JSONSignals(data.Filter{})),
	)
}

func currentURLPanel() Node {
	return Article(
		Header(Strong(Text("Current URL"))),
		Textarea(
			ReadOnly(),
			Rows("3"),
			data.Bind("currentURL"),
		),
	)
}

func withIcon(icon func(...Node) Node, label string) Node {
	return Span(
		Style("display:inline-flex;align-items:center;gap:.45rem"),
		icon(
			Width("18"),
			Height("18"),
			Style("flex:none"),
		),
		Span(Text(label)),
	)
}

func themeTriggerIcon(hidden bool, hiddenWhen string, icon func(...Node) Node) Node {
	return Span(
		If(hidden, Attr("hidden")),
		data.Attr("hidden", hiddenWhen+" ? true : null"),
		icon(
			Width("16"),
			Height("16"),
			Style("display:block"),
		),
	)
}

func pageInitExpression() string {
	return "$currentURL = window.location.pathname + window.location.search; " +
		"$pushedURL = $currentURL; " +
		"window.DatastarURLSync.bindPopstate($urlParams); " +
		"if ($themeMode == 'auto') { document.documentElement.removeAttribute('data-theme') } else { document.documentElement.dataset.theme = $themeMode }"
}

func themeCycleExpression() string {
	return "$themeMode = ($themeMode == 'auto' ? 'light' : ($themeMode == 'light' ? 'dark' : 'auto')); " +
		"if ($themeMode == 'auto') { document.documentElement.removeAttribute('data-theme') } else { document.documentElement.dataset.theme = $themeMode }"
}

func withButtonIcon(icon func(...Node) Node, label string) Node {
	return Span(
		Style("display:inline-flex;align-items:center;justify-content:center;gap:.45rem"),
		icon(
			Width("16"),
			Height("16"),
			Style("flex:none"),
		),
		Span(Text(label)),
	)
}

func displayLabel(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

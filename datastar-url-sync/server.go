package main

import (
	"encoding/json"
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

type preset struct {
	Href      string
	ClassName string
	Label     string
	Params    map[string]any
}

var (
	categoryOptions = []string{"dashboard", "model", "pipeline"}
	assigneeOptions = []string{"alice", "bob", "charlie"}
	presets         = []preset{
		{
			Href:   "/?q=metrics&kind=model",
			Label:  "Metrics + Model",
			Params: map[string]any{"q": "metrics", "kind": []string{"model"}, "owner": []string{}},
		},
		{
			Href:      "/?q=warehouse&kind=dashboard&owner=alice",
			ClassName: "secondary",
			Label:     "Alice + Dashboard",
			Params:    map[string]any{"q": "warehouse", "kind": []string{"dashboard"}, "owner": []string{"alice"}},
		},
		{
			Href:      "/?kind=pipeline&owner=bob",
			ClassName: "contrast",
			Label:     "Bob + Pipeline",
			Params:    map[string]any{"q": "", "kind": []string{"pipeline"}, "owner": []string{"bob"}},
		},
	}
)

const (
	clearAllExpression    = "$urlParams = {...$urlParams, q: '', kind: [], owner: []}"
	pageInitExpression    = "$currentURL = window.location.pathname + window.location.search; $pushedURL = $currentURL; window.DatastarURLSync.bindPopstate($urlParams)"
	pushHistoryExpression = "$currentURL = window.DatastarURLSync.push($urlParams); $pushedURL = $currentURL"
	replaceURLExpression  = "$currentURL = window.DatastarURLSync.replace($urlParams)"
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
			Meta(Name("color-scheme"), Content("light dark")),
			Script(Raw(themeBootstrapScript)),
			StyleEl(Raw(themeIconStyles)),
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
				data.Init(pageInitExpression),
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
		data.On("click", "window.ThemeController && window.ThemeController.cycle()"),
		Attr("aria-label", "Theme"),
		Title("Theme"),
		themeTriggerIcon("auto", lucide.Monitor),
		themeTriggerIcon("light", lucide.Sun),
		themeTriggerIcon("dark", lucide.Moon),
	)
}

func demoShell() Node {
	return Section(
		data.OnSignalPatch(replaceURLExpression, data.ModifierDebounce, data.Duration(150000000)),
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
					data.Effect("el.checked = $urlParams."+key+".includes('"+value+"')"),
					data.On("change", toggleExpression(key, value)),
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
			data.On("click", pushHistoryExpression),
			withButtonIcon(lucide.History, "Push to history"),
		),
		Button(
			Type("button"),
			Class("outline secondary"),
			data.On("click", clearAllExpression),
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
			checklistField("Categories", "kind", categoryOptions),
			checklistField("Assignee", "owner", assigneeOptions),
			actionRow(),
		),
	)
}

func quickLinksCard() Node {
	return Article(
		Header(Strong(Text("Quick Links"))),
		Div(Class("grid"),
			Map(presets, quickLink),
		),
	)
}

func quickLink(preset preset) Node {
	return A(
		Href(preset.Href),
		If(preset.ClassName != "", Class(preset.ClassName)),
		data.On("click", "$urlParams = "+mustJSON(preset.Params), data.ModifierPrevent),
		Text(preset.Label),
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

func themeTriggerIcon(mode string, icon func(...Node) Node) Node {
	return Span(
		Attr("data-theme-icon", mode),
		icon(
			Width("16"),
			Height("16"),
			Style("display:block"),
		),
	)
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

func toggleExpression(key, value string) string {
	return "$urlParams." + key + " = el.checked ? [...$urlParams." + key + ", el.value] : $urlParams." + key + ".filter(item => item !== el.value)"
}

func mustJSON(value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(b)
}

const themeBootstrapScript = `
(() => {
  const key = "theme";
  const valid = new Set(["auto", "light", "dark"]);
  const media = window.matchMedia("(prefers-color-scheme: dark)");

  const read = () => {
    try {
      const saved = localStorage.getItem(key);
      return valid.has(saved) ? saved : "auto";
    } catch {
      return "auto";
    }
  };

  const resolve = (mode) => mode === "auto" ? (media.matches ? "dark" : "light") : mode;

  const apply = (mode, persist) => {
    document.documentElement.dataset.themeMode = mode;
    document.documentElement.dataset.theme = resolve(mode);
    if (persist) {
      try {
        localStorage.setItem(key, mode);
      } catch {}
    }
  };

  window.ThemeController = {
    cycle() {
      const current = document.documentElement.dataset.themeMode || read();
      const next = current === "auto" ? "light" : current === "light" ? "dark" : "auto";
      apply(next, true);
      return next;
    },
  };

  apply(read(), false);

  const handleSystemChange = () => {
    if ((document.documentElement.dataset.themeMode || "auto") === "auto") {
      apply("auto", false);
    }
  };

  if (typeof media.addEventListener === "function") {
    media.addEventListener("change", handleSystemChange);
  } else if (typeof media.addListener === "function") {
    media.addListener(handleSystemChange);
  }
})();
`

const themeIconStyles = `
[data-theme-icon] {
  display: none;
}

html[data-theme-mode="auto"] [data-theme-icon="auto"],
html[data-theme-mode="light"] [data-theme-icon="light"],
html[data-theme-mode="dark"] [data-theme-icon="dark"] {
  display: inline-flex;
}
`

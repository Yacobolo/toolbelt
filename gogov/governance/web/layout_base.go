package web

import (
	"strings"

	"github.com/Yacobolo/toolbelt/gogov/governance/config"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type layoutData struct {
	Title        string
	Message      string
	Section      string
	Breadcrumbs  []breadcrumbItem
	StatusLabel  string
	StatusTone   string
	Repositories []config.Repository
	ActiveRepo   *config.Repository
	RefreshPath  string
	MainClass    string
	ContentClass string
}

type breadcrumbItem struct {
	Label string
	Href  string
}

func governancePage(layout layoutData, body g.Node) g.Node {
	mainClass := "min-h-0 flex-1 overflow-x-hidden overflow-y-auto bg-white"
	if strings.TrimSpace(layout.MainClass) != "" {
		mainClass += " " + strings.TrimSpace(layout.MainClass)
	}

	contentClass := "mx-auto min-h-full w-full max-w-[110rem] px-6 py-8 sm:px-8 xl:px-10"
	if strings.TrimSpace(layout.ContentClass) != "" {
		contentClass = strings.TrimSpace(layout.ContentClass)
	}

	return h.Doctype(
		h.HTML(
			h.Lang("en"),
			h.Head(
				h.Meta(h.Charset("UTF-8")),
				h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1.0")),
				h.TitleEl(g.Text(layout.Title)),
				h.Script(h.Src("https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4")),
				h.Script(h.Type("module"), h.Src("https://cdn.jsdelivr.net/gh/starfederation/datastar@1.0.0-RC.7/bundles/datastar.js")),
				h.Link(h.Rel("stylesheet"), h.Href(assetsRoutePrefix+assetsStyleName)),
				h.Script(h.Type("module"), h.Src(assetsRoutePrefix+assetsScriptName)),
			),
			h.Body(
				h.Class("h-screen overflow-hidden bg-[color:oklch(0.952_0.01_85)] text-stone-900"),
				h.Div(
					h.Class("flex h-full"),
					appShellSidebar(layout),
					h.Div(
						h.Class("flex min-w-0 flex-1 flex-col bg-white"),
						appShellHeader(layout),
						h.Main(
							h.Class(mainClass),
							h.Div(
								h.Class(contentClass),
								g.If(layout.Message != "", appShellMessage(layout.Message)),
								body,
							),
						),
					),
				),
			),
		),
	)
}

func appShellSidebar(layout layoutData) g.Node {
	repoLinks := g.Group{}
	for _, repo := range layout.Repositories {
		repoLinks = append(repoLinks, repoNavLink(repo, layout.ActiveRepo))
	}

	return h.Aside(
		h.Class("hidden h-full w-72 shrink-0 border-r border-stone-200 bg-[color:oklch(0.935_0.01_85)] lg:flex lg:flex-col"),
		h.Div(
			h.Class("border-b border-stone-200 px-6 py-6"),
			h.Div(
				h.Class("space-y-1"),
				h.P(h.Class("text-[1.7rem] font-semibold tracking-[-0.03em] text-stone-950"), g.Text("GoGov")),
				h.P(h.Class("text-xs uppercase tracking-[0.18em] text-stone-500"), g.Text("Repository Catalog")),
			),
		),
		h.Div(
			h.Class("flex-1 overflow-y-auto px-4 py-5"),
			h.Div(
				h.Class("space-y-6"),
				h.Div(
					h.Class("space-y-3"),
					h.P(h.Class("px-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-stone-500"), g.Text("Repositories")),
					h.Nav(h.Class("space-y-1"), repoLinks),
				),
				g.Iff(layout.ActiveRepo != nil, func() g.Node {
					return h.Div(
						h.Class("space-y-3"),
						h.P(h.Class("px-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-stone-500"), g.Text("Catalog")),
						h.Nav(
							h.Class("space-y-1"),
							appShellNavLink(repoBaseHref(layout.ActiveRepo.ID), "overview", layout.Section, "Overview"),
							appShellNavLink(repoRunsHref(layout.ActiveRepo.ID), "runs", layout.Section, "Runs"),
							appShellNavLink(repoFilesHref(layout.ActiveRepo.ID), "files", layout.Section, "Files"),
							appShellNavLink(repoPackagesHref(layout.ActiveRepo.ID), "packages", layout.Section, "Packages"),
						),
					)
				}),
			),
		),
	)
}

func appShellHeader(layout layoutData) g.Node {
	return h.Header(
		h.Class("border-b border-stone-200 bg-[color:rgba(255,255,255,0.96)]"),
		h.Div(
			h.Class("mx-auto flex w-full max-w-[110rem] flex-col px-6 py-5 sm:px-8 xl:px-10"),
			h.Div(
				h.Class("flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between"),
				h.Div(
					h.Class("space-y-2"),
					breadcrumbsNode(layout.Breadcrumbs),
				),
				h.Div(
					h.Class("flex flex-col gap-3 sm:flex-row sm:items-center"),
					g.If(layout.StatusLabel != "", statusBadge(layout.StatusLabel, layout.StatusTone)),
					g.Iff(layout.RefreshPath != "", func() g.Node {
						return h.Form(
							h.Method("post"),
							h.Action(layout.RefreshPath),
							h.Class("flex"),
							h.Button(
								h.Class("inline-flex items-center justify-center gap-2 border border-stone-950 bg-stone-950 px-4 py-2.5 text-sm font-semibold text-stone-50 transition hover:bg-stone-800"),
								h.Type("submit"),
								uiIcon("refresh", "h-4 w-4 shrink-0"),
								g.Text("Run scan"),
							),
						)
					}),
				),
			),
		),
	)
}

func appShellMessage(message string) g.Node {
	return h.Div(
		h.Class("mb-6 border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-950"),
		g.Text(message),
	)
}

func appShellNavLink(href string, section string, active string, label string) g.Node {
	className := "flex items-center gap-3 border-l-2 border-transparent px-3 py-3 text-stone-600 transition hover:bg-white hover:text-stone-950"
	if active == section {
		className = "flex items-center gap-3 border-l-2 border-stone-950 bg-white px-3 py-3 text-stone-950"
	}

	return h.A(
		h.Class(className),
		h.Href(href),
		uiIcon(section, navIconClass(active == section)),
		h.P(h.Class("text-sm font-semibold"), g.Text(label)),
	)
}

func repoNavLink(repo config.Repository, activeRepo *config.Repository) g.Node {
	className := "flex items-center gap-3 border border-transparent px-3 py-3 text-stone-600 transition hover:bg-white hover:text-stone-950"
	iconClass := "h-4 w-4 shrink-0 text-stone-400"
	if activeRepo != nil && activeRepo.ID == repo.ID {
		className = "flex items-center gap-3 border border-stone-200 bg-white px-3 py-3 text-stone-950"
		iconClass = "h-4 w-4 shrink-0 text-stone-950"
	}

	return h.A(
		h.Class(className),
		h.Href(repoBaseHref(repo.ID)),
		uiIcon("repo", iconClass),
		h.Div(
			h.Class("min-w-0"),
			h.P(h.Class("truncate text-sm font-semibold"), g.Text(repo.Name)),
			h.P(h.Class("truncate text-xs text-stone-500"), g.Text(repo.ID)),
		),
	)
}

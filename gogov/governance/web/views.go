package web

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Yacobolo/toolbelt/gogov/governance/config"
	"github.com/Yacobolo/toolbelt/gogov/governance/model"
	lucide "github.com/eduardolat/gomponents-lucide"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

func governancePage(layout layoutData, body g.Node) g.Node {
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
				h.Class("h-screen overflow-hidden bg-[color:oklch(0.965_0.008_85)] text-stone-900"),
				h.Div(
					h.Class("flex h-full"),
					appShellSidebar(layout),
					h.Div(
						h.Class("flex min-w-0 flex-1 flex-col"),
						appShellHeader(layout),
						h.Main(
							h.Class("min-h-0 flex-1 overflow-y-auto"),
							h.Div(
								h.Class("mx-auto w-full max-w-[110rem] px-4 py-6 sm:px-6 xl:px-8"),
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
		h.Class("hidden h-full w-72 shrink-0 border-r border-stone-200 bg-[linear-gradient(180deg,rgba(247,245,240,0.98),rgba(239,236,229,0.98))] lg:flex lg:flex-col"),
		h.Div(
			h.Class("border-b border-stone-200 px-5 py-5"),
			h.Div(
				h.Class("flex items-center gap-3"),
				h.Div(
					h.Class("flex h-11 w-11 items-center justify-center rounded-2xl bg-stone-950 text-sm font-black tracking-[0.22em] text-stone-50"),
					g.Text("GG"),
				),
				h.P(h.Class("text-lg font-black tracking-tight text-stone-950"), g.Text("Governance")),
			),
		),
		h.Div(
			h.Class("flex-1 overflow-y-auto px-4 py-5"),
			h.Div(
				h.Class("space-y-6"),
				h.Nav(h.Class("space-y-1"), repoLinks),
				g.Iff(layout.ActiveRepo != nil, func() g.Node {
					return h.Nav(
						h.Class("space-y-1"),
						appShellNavLink(repoBaseHref(layout.ActiveRepo.ID), "overview", layout.Section, "Overview"),
						appShellNavLink(repoRunsHref(layout.ActiveRepo.ID), "runs", layout.Section, "Runs"),
						appShellNavLink(repoFilesHref(layout.ActiveRepo.ID), "files", layout.Section, "Files"),
						appShellNavLink(repoPackagesHref(layout.ActiveRepo.ID), "packages", layout.Section, "Packages"),
					)
				}),
			),
		),
	)
}

func appShellHeader(layout layoutData) g.Node {
	return h.Header(
		h.Class("border-b border-stone-200 bg-[color:rgba(250,249,246,0.92)] backdrop-blur"),
		h.Div(
			h.Class("mx-auto flex w-full max-w-[110rem] flex-col px-4 py-5 sm:px-6 xl:px-8"),
			h.Div(
				h.Class("flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between"),
				h.Div(
					h.Class("space-y-3"),
					breadcrumbsNode(layout.Breadcrumbs),
					h.H1(h.Class("text-3xl font-black tracking-tight text-stone-950 sm:text-4xl"), g.Text(layout.Title)),
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
								h.Class("inline-flex items-center justify-center gap-2 rounded-full bg-stone-950 px-5 py-3 text-sm font-semibold text-stone-50 transition hover:bg-stone-800"),
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
		h.Class("mb-6 rounded-[1.7rem] border border-amber-200 bg-amber-50 px-5 py-4 text-sm text-amber-950 shadow-sm"),
		g.Text(message),
	)
}

func appShellNavLink(href string, section string, active string, label string) g.Node {
	className := "flex items-center gap-3 rounded-[1.4rem] px-3 py-3 text-stone-600 transition hover:bg-white/80 hover:text-stone-950"
	if active == section {
		className = "flex items-center gap-3 rounded-[1.4rem] bg-white px-3 py-3 text-stone-950 shadow-sm ring-1 ring-stone-200"
	}

	return h.A(
		h.Class(className),
		h.Href(href),
		uiIcon(section, navIconClass(active == section)),
		h.P(h.Class("text-sm font-semibold"), g.Text(label)),
	)
}

func repoNavLink(repo config.Repository, activeRepo *config.Repository) g.Node {
	className := "flex items-center gap-3 rounded-[1.4rem] px-3 py-3 text-stone-600 transition hover:bg-white/80 hover:text-stone-950"
	iconClass := "h-4 w-4 shrink-0 text-stone-400"
	if activeRepo != nil && activeRepo.ID == repo.ID {
		className = "flex items-center gap-3 rounded-[1.4rem] bg-white px-3 py-3 text-stone-950 shadow-sm ring-1 ring-stone-200"
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

func homeView(page homeData) g.Node {
	if len(page.Summaries) == 0 {
		return h.Main(
			h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
			h.P(h.Class("text-sm text-stone-500"), g.Text("No repositories configured.")),
		)
	}

	cards := g.Group{}
	for _, summary := range page.Summaries {
		cards = append(cards, homeRepoCard(summary))
	}

	return h.Main(
		h.Class("grid gap-6 xl:grid-cols-2"),
		cards,
	)
}

func homeRepoCard(summary repoSummary) g.Node {
	statusLabel := snapshotStatusLabel(summary.Meta)
	statusTone := snapshotStatusTone(summary.Meta)
	lastRunText := "No runs yet"
	if summary.LastRun != nil {
		lastRunText = formatTimeValue(summary.LastRun.StartedAt)
	}

	return h.Section(
		h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
		h.Div(
			h.Class("flex flex-col gap-4"),
			h.Div(
				h.Class("flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between"),
				h.Div(
					h.Class("space-y-2"),
					h.H2(h.Class("text-2xl font-black tracking-tight text-stone-950"), g.Text(summary.Repo.Name)),
					h.P(h.Class("text-sm text-stone-500"), g.Text(repoDisplayPath(summary.Repo))),
				),
				statusBadge(statusLabel, statusTone),
			),
			h.Div(
				h.Class("grid gap-4 sm:grid-cols-3"),
				summaryCard("files", "Files", strconv.Itoa(firstOrZero(summary.Meta, func(meta model.SnapshotMeta) int { return meta.FilesCount })), true),
				summaryCard("packages", "Packages", strconv.Itoa(firstOrZero(summary.Meta, func(meta model.SnapshotMeta) int { return meta.PackagesCount })), true),
				h.Div(
					h.Class("rounded-3xl bg-[color:oklch(0.98_0.004_85)] p-5 ring-1 ring-stone-200"),
					labelWithIcon("refresh", "Last run"),
					h.P(h.Class("mt-2 text-sm font-semibold"), g.Text(lastRunText)),
				),
			),
			h.Div(
				h.Class("flex flex-wrap gap-3"),
				h.A(h.Class("inline-flex items-center gap-2 rounded-full border border-stone-300 px-4 py-2 text-sm font-semibold text-stone-700 transition hover:bg-stone-100"), h.Href(repoBaseHref(summary.Repo.ID)), uiIcon("overview", "h-4 w-4 shrink-0"), g.Text("Open catalog")),
				h.Form(
					h.Method("post"),
					h.Action(repoRefreshHref(summary.Repo.ID)),
					h.Button(h.Class("inline-flex items-center gap-2 rounded-full bg-stone-950 px-4 py-2 text-sm font-semibold text-stone-50 transition hover:bg-stone-800"), h.Type("submit"), uiIcon("refresh", "h-4 w-4 shrink-0"), g.Text("Run scan")),
				),
			),
		),
	)
}

func dashboardView(page dashboardData) g.Node {
	return h.Main(
		h.Class("space-y-8"),
		h.Section(
			h.Class("grid gap-4 md:grid-cols-4"),
			summaryCard("repo", "Repo", firstOrFallback(page.Meta, func(meta model.SnapshotMeta) string { return meta.ModulePath }, "No snapshot yet"), false),
			summaryCard("files", "Files", strconv.Itoa(firstOrZero(page.Meta, func(meta model.SnapshotMeta) int { return meta.FilesCount })), true),
			summaryCard("packages", "Packages", strconv.Itoa(firstOrZero(page.Meta, func(meta model.SnapshotMeta) int { return meta.PackagesCount })), true),
			h.Div(
				h.Class("rounded-3xl bg-white p-5 shadow-sm ring-1 ring-stone-200"),
				labelWithIcon("refresh", "Last refreshed"),
				h.P(h.Class("mt-2 text-sm font-semibold"), g.Text(formatTimeMeta(page.Meta))),
			),
		),
		h.Section(
			h.Class("grid gap-8 lg:grid-cols-[1.25fr_0.95fr]"),
			h.Div(
				h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
				h.Div(
					h.Class("mb-4 flex items-center justify-between"),
					h.H2(h.Class("text-2xl font-bold"), g.Text("Largest files")),
					h.A(h.Class("text-sm font-semibold text-stone-700 underline"), h.Href(repoFilesHref(page.RepoID)+"?sort=loc"), g.Text("Open full inventory")),
				),
				h.Div(
					h.Class("overflow-x-auto"),
					h.Table(
						h.Class("min-w-full text-sm"),
						h.THead(
							h.Class("text-left text-stone-500"),
							h.Tr(
								h.Th(h.Class("pb-3"), g.Text("File")),
								h.Th(h.Class("pb-3"), g.Text("LOC")),
								h.Th(h.Class("pb-3"), g.Text("Coverage")),
								h.Th(h.Class("pb-3"), g.Text("Lineage")),
							),
						),
						h.TBody(largestFilesRows(page.RepoID, page.BigFiles)),
					),
				),
			),
			h.Div(
				h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
				h.Div(
					h.Class("mb-4 flex items-center justify-between"),
					h.H2(h.Class("text-2xl font-bold"), g.Text("Largest packages")),
					h.A(h.Class("text-sm font-semibold text-stone-700 underline"), h.Href(repoPackagesHref(page.RepoID)), g.Text("Open package catalog")),
				),
				hotPackagesNodes(page.RepoID, page.HotPackages),
			),
		),
		h.Section(
			h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
			h.Div(
				h.Class("mb-4 flex items-center justify-between"),
				h.H2(h.Class("text-2xl font-bold"), g.Text("Recent runs")),
				h.A(h.Class("text-sm font-semibold text-stone-700 underline"), h.Href(repoRunsHref(page.RepoID)), g.Text("Open run history")),
			),
			recentRunsNodes(page.Runs),
		),
	)
}

func runsView(page runsData) g.Node {
	return h.Main(
		h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
		h.Div(
			h.Class("mb-4 flex items-end justify-between"),
			h.H2(h.Class("text-2xl font-bold"), g.Text("Run history")),
		),
		h.Div(
			h.Class("overflow-x-auto"),
			h.Table(
				h.Class("min-w-full text-sm"),
				h.THead(
					h.Class("text-left text-stone-500"),
					h.Tr(
						h.Th(h.Class("pb-3"), g.Text("Run")),
						h.Th(h.Class("pb-3"), g.Text("Started")),
						h.Th(h.Class("pb-3"), g.Text("Status")),
						h.Th(h.Class("pb-3"), g.Text("Coverage")),
						h.Th(h.Class("pb-3"), g.Text("Files")),
						h.Th(h.Class("pb-3"), g.Text("Packages")),
					),
				),
				h.TBody(runRows(page.Runs)),
			),
		),
	)
}

func filesView(page filesData) g.Node {
	return h.Main(
		h.Class("space-y-6"),
		h.Section(
			h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
			h.Form(
				h.Method("get"),
				h.Class("grid gap-4 lg:grid-cols-[1fr_1fr_180px_auto]"),
				h.Input(h.Class("rounded-2xl border border-stone-300 px-4 py-3"), h.Type("search"), h.Name("q"), h.Placeholder("Filter by path"), h.Value(page.Filter)),
				h.Input(h.Class("rounded-2xl border border-stone-300 px-4 py-3"), h.Type("search"), h.Name("package"), h.Placeholder("Filter by package path"), h.Value(page.PackageFilter)),
				h.Select(
					h.Class("rounded-2xl border border-stone-300 px-4 py-3"),
					h.Name("sort"),
					sortOption("loc", "Sort by LOC", page.Sort),
					sortOption("coverage", "Sort by coverage", page.Sort),
					sortOption("fanin", "Sort by fan-in", page.Sort),
					sortOption("fanout", "Sort by fan-out", page.Sort),
				),
				h.Button(h.Class("rounded-full bg-stone-950 px-5 py-3 text-sm font-semibold text-white"), h.Type("submit"), g.Text("Apply")),
			),
		),
		h.Section(
			h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
			h.Div(
				h.Class("overflow-x-auto"),
				h.Table(
					h.Class("min-w-full text-sm"),
					h.THead(
						h.Class("text-left text-stone-500"),
						h.Tr(
							h.Th(h.Class("pb-3"), g.Text("File")),
							h.Th(h.Class("pb-3"), g.Text("Package")),
							h.Th(h.Class("pb-3"), g.Text("LOC")),
							h.Th(h.Class("pb-3"), g.Text("Coverage")),
							h.Th(h.Class("pb-3"), g.Text("Fan-in")),
							h.Th(h.Class("pb-3"), g.Text("Fan-out")),
						),
					),
					h.TBody(fileRows(page.RepoID, page.Files)),
				),
			),
		),
	)
}

func fileDetailView(page fileDetailData) g.Node {
	return h.Main(
		h.Class("space-y-6"),
		data.Signals(map[string]any{
			"fileDetailTab": "overview",
			"fileGraph":     page.Graph,
			"fileSource":    page.Source,
		}),
		h.Section(
			h.ID("file-header"),
			h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
			h.P(h.Class("text-xs uppercase tracking-[0.2em] text-stone-500"), g.Text(shortPkg(page.File.PackagePath))),
			h.Div(
				h.Class("mt-2 flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between"),
				h.Div(
					h.Class("space-y-3"),
					h.H2(h.Class("text-3xl font-black tracking-tight text-stone-950"), g.Text(page.File.Path)),
				),
				h.A(
					h.Class("inline-flex items-center gap-2 rounded-full border border-stone-300 px-4 py-2 text-sm font-semibold text-stone-700 transition hover:bg-stone-100"),
					h.Href(packageHref(page.RepoID, page.File.PackagePath)),
					uiIcon("packages", "h-4 w-4 shrink-0"),
					g.Text("Open package"),
				),
			),
		),
		detailTabs(
			"fileDetailTab",
			[]detailTab{
				{Value: "overview", Label: "Overview"},
				{Value: "lineage", Label: "Lineage"},
				{Value: "source", Label: "Source"},
			},
		),
		detailPane("fileDetailTab", "overview",
			h.Section(
				h.ID("overview"),
				h.Class("grid gap-4 xl:grid-cols-3"),
				statusCard("Size", strconv.Itoa(page.File.LOC)+" LOC"),
				statusCard("Coverage", coverageText(page.File.CoveragePct)),
				statusCard("Flow", strconv.Itoa(page.File.FanIn)+" in / "+strconv.Itoa(page.File.FanOut)+" out"),
			),
			h.Div(
				h.Class("grid gap-6 xl:grid-cols-[1.1fr_0.9fr]"),
				h.Section(
					h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
					h.H3(h.Class("text-xl font-bold"), g.Text("Key symbols")),
					h.Div(h.Class("mt-4 space-y-2 text-sm"), symbolNodes(topSymbols(page.Symbols, 12))),
				),
				h.Section(
					h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
					h.H3(h.Class("text-xl font-bold"), g.Text("Related tests")),
					h.Div(h.Class("mt-4 space-y-2 text-sm"), relatedTestNodes(page.RepoID, page.RelatedTests)),
				),
			),
			h.Div(
				h.Class("grid gap-6 xl:grid-cols-2"),
				h.Section(
					h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
					h.H3(h.Class("text-xl font-bold"), g.Text("Top dependents")),
					h.Div(h.Class("mt-4 space-y-2 text-sm"), fileEdgeNodes(page.RepoID, topFileEdges(page.Inbound, true, 8), true)),
				),
				h.Section(
					h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
					h.H3(h.Class("text-xl font-bold"), g.Text("Top dependencies")),
					h.Div(h.Class("mt-4 space-y-2 text-sm"), fileEdgeNodes(page.RepoID, topFileEdges(page.Outbound, false, 8), false)),
				),
			),
		),
		detailPane("fileDetailTab", "lineage",
			h.Div(
				h.ID("lineage"),
				h.Class("space-y-4"),
				h.Section(
					h.Class(workspaceCanvasClass("min-h-[40rem] overflow-hidden")),
					g.El(
						"governance-graph-view",
						h.Class("block h-full"),
						g.Attr("graph-title", "File lineage"),
						data.Attr("graph", "$fileGraph"),
					),
				),
				h.Section(
					h.Class(workspacePanelClass()),
					h.Div(
						h.Class("overflow-x-auto"),
						h.Table(
							h.Class("min-w-full text-sm"),
							h.THead(
								h.Class("text-left text-stone-500"),
								h.Tr(
									h.Th(h.Class("pb-3"), g.Text("Direction")),
									h.Th(h.Class("pb-3"), g.Text("Connection")),
									h.Th(h.Class("pb-3"), g.Text("Kind")),
									h.Th(h.Class("pb-3"), g.Text("Weight")),
								),
							),
							h.TBody(fileConnectionRows(page.RepoID, page.Inbound, page.Outbound, 24)),
						),
					),
				),
			),
		),
		detailPane("fileDetailTab", "source",
			h.Div(
				h.ID("source"),
				h.Class("space-y-4"),
				h.Div(
					h.Class("grid gap-4 md:grid-cols-3"),
					statusCard("Package", shortPkg(page.File.PackagePath)),
					statusCard("Functions", strconv.Itoa(page.File.FunctionCount)),
					statusCard("Exports", strconv.Itoa(page.File.ExportedSymbolCount)),
				),
				h.Section(
					h.Class(workspaceCanvasClass("min-h-[44rem] overflow-hidden")),
					g.El(
						"governance-code-viewer",
						h.Class("block h-full"),
						data.Attr("file", "$fileSource"),
					),
				),
			),
		),
	)
}

func packagesView(page packagesData) g.Node {
	return h.Main(
		h.Class("space-y-6"),
		h.Section(
			h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
			data.Signals(map[string]any{"packageGraph": page.Graph}),
			h.H2(h.Class("text-2xl font-bold"), g.Text("Package DAG")),
			g.El(
				"governance-graph-view",
				h.Class("mt-4 block"),
				g.Attr("graph-title", "Package DAG"),
				data.Attr("graph", "$packageGraph"),
			),
		),
		h.Section(
			h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
			h.Div(
				h.Class("overflow-x-auto"),
				h.Table(
					h.Class("min-w-full text-sm"),
					h.THead(
						h.Class("text-left text-stone-500"),
						h.Tr(
							h.Th(h.Class("pb-3"), g.Text("Package")),
							h.Th(h.Class("pb-3"), g.Text("Files")),
							h.Th(h.Class("pb-3"), g.Text("LOC")),
							h.Th(h.Class("pb-3"), g.Text("Imports")),
							h.Th(h.Class("pb-3"), g.Text("Imported by")),
						),
					),
					h.TBody(packageRows(page.RepoID, page.Packages)),
				),
			),
		),
	)
}

func packageDetailView(page packageDetailData) g.Node {
	return h.Main(
		h.Class("space-y-6"),
		data.Signals(map[string]any{
			"packageDetailTab": "overview",
			"packageGraph":     page.Graph,
		}),
		h.Section(
			h.ID("package-header"),
			h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
			h.P(h.Class("text-xs uppercase tracking-[0.2em] text-stone-500"), g.Text(page.Package.Name)),
			h.H2(h.Class("mt-2 text-3xl font-black tracking-tight text-stone-950"), g.Text(page.Package.Path)),
		),
		detailTabs(
			"packageDetailTab",
			[]detailTab{
				{Value: "overview", Label: "Overview"},
				{Value: "neighborhood", Label: "Neighborhood"},
				{Value: "files", Label: "Files"},
			},
		),
		detailPane("packageDetailTab", "overview",
			h.Section(
				h.ID("package-overview"),
				h.Class("grid gap-4 xl:grid-cols-4"),
				statusCard("Files", strconv.Itoa(page.Package.FileCount)),
				statusCard("Size", strconv.Itoa(page.Package.LOC)+" LOC"),
				statusCard("Dependencies", strconv.Itoa(page.Package.ImportsCount)),
				statusCard("Dependents", strconv.Itoa(page.Package.ImportedByCount)),
			),
			h.Div(
				h.Class("grid gap-6 xl:grid-cols-2"),
				h.Section(
					h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
					h.H3(h.Class("text-xl font-bold"), g.Text("Dependencies")),
					h.Div(h.Class("mt-4 space-y-2"), packageEdgeNodes(page.RepoID, topPackageEdges(page.Outbound, false, 10), false)),
				),
				h.Section(
					h.Class("rounded-[2rem] bg-white p-6 shadow-sm ring-1 ring-stone-200"),
					h.H3(h.Class("text-xl font-bold"), g.Text("Dependents")),
					h.Div(h.Class("mt-4 space-y-2"), packageEdgeNodes(page.RepoID, topPackageEdges(page.Inbound, true, 10), true)),
				),
			),
		),
		detailPane("packageDetailTab", "neighborhood",
			h.Div(
				h.ID("neighborhood"),
				h.Class("space-y-4"),
				h.Section(
					h.Class(workspaceCanvasClass("min-h-[40rem] overflow-hidden")),
					g.El(
						"governance-graph-view",
						h.Class("block h-full"),
						g.Attr("graph-title", "Package neighborhood"),
						data.Attr("graph", "$packageGraph"),
					),
				),
				h.Section(
					h.Class(workspacePanelClass()),
					h.Div(
						h.Class("overflow-x-auto"),
						h.Table(
							h.Class("min-w-full text-sm"),
							h.THead(
								h.Class("text-left text-stone-500"),
								h.Tr(
									h.Th(h.Class("pb-3"), g.Text("Direction")),
									h.Th(h.Class("pb-3"), g.Text("Package")),
									h.Th(h.Class("pb-3"), g.Text("Weight")),
								),
							),
							h.TBody(packageConnectionRows(page.RepoID, page.Inbound, page.Outbound, 24)),
						),
					),
				),
			),
		),
		detailPane("packageDetailTab", "files",
			h.Section(
				h.ID("package-files"),
				h.Class(workspaceCanvasClass("overflow-hidden")),
				h.Div(
					h.Class("overflow-x-auto"),
					h.Table(
						h.Class("min-w-full text-sm"),
						h.THead(
							h.Class("text-left text-stone-500"),
							h.Tr(
								h.Th(h.Class("pb-3"), g.Text("File")),
								h.Th(h.Class("pb-3"), g.Text("LOC")),
								h.Th(h.Class("pb-3"), g.Text("Coverage")),
							),
						),
						h.TBody(packageFileRows(page.RepoID, page.Files)),
					),
				),
			),
		),
	)
}

type breadcrumbItem struct {
	Label string
	Href  string
}

type detailTab struct {
	Value string
	Label string
}

func breadcrumbsNode(items []breadcrumbItem) g.Node {
	nodes := g.Group{}
	for idx, item := range items {
		if idx > 0 {
			nodes = append(nodes, h.Span(h.Class("text-stone-300"), g.Text("/")))
		}
		if item.Href == "" {
			nodes = append(nodes, h.Span(h.Class("font-semibold text-stone-600"), g.Text(item.Label)))
			continue
		}
		nodes = append(nodes, h.A(h.Class("text-stone-500 transition hover:text-stone-900"), h.Href(item.Href), g.Text(item.Label)))
	}

	return h.Nav(
		h.Class("flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.2em]"),
		g.Attr("aria-label", "Breadcrumb"),
		nodes,
	)
}

func statusBadge(label string, tone string) g.Node {
	return h.Div(
		h.Class(statusBadgeClass(tone)),
		statusDot(tone),
		h.Span(g.Text(label)),
	)
}

func statusBadgeClass(tone string) string {
	base := "inline-flex items-center gap-2 rounded-full border px-4 py-2 text-sm font-semibold shadow-sm"
	switch tone {
	case "good":
		return base + " border-emerald-200 bg-emerald-50 text-emerald-900"
	case "warn":
		return base + " border-amber-200 bg-amber-50 text-amber-950"
	case "critical":
		return base + " border-rose-200 bg-rose-50 text-rose-950"
	default:
		return base + " border-stone-200 bg-white text-stone-700"
	}
}

func statusDot(tone string) g.Node {
	className := "mt-1 h-2.5 w-2.5 rounded-full bg-stone-400"
	switch tone {
	case "good":
		className = "mt-1 h-2.5 w-2.5 rounded-full bg-emerald-500"
	case "warn":
		className = "mt-1 h-2.5 w-2.5 rounded-full bg-amber-500"
	case "critical":
		className = "mt-1 h-2.5 w-2.5 rounded-full bg-rose-500"
	}
	return h.Span(h.Class(className))
}

func detailTabs(signal string, tabs []detailTab) g.Node {
	buttons := g.Group{}
	for _, tab := range tabs {
		buttons = append(buttons, detailTabButton(signal, tab.Value, tab.Label))
	}

	return h.Section(
		h.Class("rounded-[2rem] bg-[color:oklch(0.965_0.006_75)] p-2 ring-1 ring-stone-200"),
		h.Div(h.Class("flex flex-wrap gap-2"), buttons),
	)
}

func detailTabButton(signal string, value string, label string) g.Node {
	return h.Button(
		h.Type("button"),
		data.On("click", "$"+signal+" = '"+value+"'"),
		data.Attr("class", detailTabClassExpr(signal, value)),
		data.Attr("aria-pressed", "$"+signal+" === '"+value+"' ? 'true' : 'false'"),
		detailTabIcon(signal, value),
		g.Text(label),
	)
}

func detailTabClassExpr(signal string, value string) string {
	return "'inline-flex items-center gap-2 rounded-[1.1rem] px-4 py-2.5 text-sm font-semibold transition ' + ($" + signal + " === '" + value + "' ? 'bg-stone-950 text-stone-50 shadow-sm' : 'text-stone-500 hover:bg-white hover:text-stone-950')"
}

func detailTabIconClass(signal string, value string) string {
	return "'h-4 w-4 shrink-0 ' + ($" + signal + " === '" + value + "' ? 'text-stone-50' : 'text-stone-400')"
}

func detailTabIcon(signal string, value string) g.Node {
	iconClass := data.Attr("class", detailTabIconClass(signal, value))
	switch value {
	case "overview":
		return lucide.House(iconClass)
	case "lineage", "neighborhood":
		return lucide.GitBranch(iconClass)
	case "source", "file":
		return lucide.FileText(iconClass)
	case "files":
		return lucide.FolderSearch(iconClass)
	default:
		return lucide.BookOpen(iconClass)
	}
}

func detailPane(signal string, value string, children ...g.Node) g.Node {
	return h.Div(
		data.Show("$"+signal+" === '"+value+"'"),
		h.Class("space-y-6"),
		g.Group(children),
	)
}

func workspaceCanvasClass(extra string) string {
	base := "rounded-[2rem] border border-stone-200 bg-[color:rgba(255,255,255,0.82)] shadow-sm"
	if extra == "" {
		return base
	}
	return base + " " + extra
}

func workspacePanelClass() string {
	return "rounded-[1.8rem] border border-stone-200 bg-white/88 p-5 shadow-sm"
}

func statusCard(label string, value string) g.Node {
	return h.Div(
		h.Class("rounded-[1.8rem] bg-white p-5 shadow-sm ring-1 ring-stone-200"),
		h.P(h.Class("text-xs uppercase tracking-[0.2em] text-stone-500"), g.Text(label)),
		h.P(h.Class("mt-3 text-2xl font-black tracking-tight text-stone-950"), g.Text(value)),
	)
}

func summaryCard(icon string, label string, value string, emphasize bool) g.Node {
	valueClass := "mt-2 text-xl font-bold"
	if emphasize {
		valueClass = "mt-2 text-3xl font-black"
	}

	return h.Div(
		h.Class("rounded-3xl bg-white p-5 shadow-sm ring-1 ring-stone-200"),
		labelWithIcon(icon, label),
		h.P(h.Class(valueClass), g.Text(value)),
	)
}

func labelWithIcon(icon string, label string) g.Node {
	return h.P(
		h.Class("flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-stone-500"),
		uiIcon(icon, "h-3.5 w-3.5 shrink-0 text-stone-400"),
		h.Span(g.Text(label)),
	)
}

func navIconClass(active bool) string {
	if active {
		return "h-4 w-4 shrink-0 text-stone-950"
	}
	return "h-4 w-4 shrink-0 text-stone-400"
}

func uiIcon(name string, className string) g.Node {
	iconClass := h.Class(className)
	switch name {
	case "overview":
		return lucide.House(iconClass)
	case "runs":
		return lucide.ScrollText(iconClass)
	case "files":
		return lucide.FolderSearch(iconClass)
	case "packages", "repo":
		return lucide.Package(iconClass)
	case "lineage", "neighborhood":
		return lucide.GitBranch(iconClass)
	case "source", "file":
		return lucide.FileText(iconClass)
	case "refresh":
		return lucide.RefreshCw(iconClass)
	case "next":
		return lucide.ArrowRight(iconClass)
	default:
		return lucide.BookOpen(iconClass)
	}
}

func largestFilesRows(repoID string, files []model.File) g.Node {
	if len(files) == 0 {
		return h.Tr(h.Td(h.Class("py-3 text-stone-500"), g.Attr("colspan", "4"), g.Text("Run a refresh to populate file inventory.")))
	}

	rows := g.Group{}
	for _, item := range files {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class("py-3"), fileLink(repoID, item.Path)),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(item.LOC))),
			h.Td(h.Class("py-3"), g.Text(coverageText(item.CoveragePct))),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(item.FanIn)+" in / "+strconv.Itoa(item.FanOut)+" out")),
		))
	}
	return rows
}

func hotPackagesNodes(repoID string, packages []model.Package) g.Node {
	if len(packages) == 0 {
		return h.P(h.Class("text-sm text-stone-500"), g.Text("No packages recorded in the current snapshot."))
	}

	nodes := g.Group{}
	for _, pkg := range packages {
		nodes = append(nodes, h.Div(
			h.Class("rounded-2xl border border-stone-200 p-4"),
			h.P(h.Class("text-sm font-semibold text-stone-900"), packageLink(repoID, pkg.Path)),
			h.P(h.Class("mt-1 text-sm text-stone-700"), g.Text(strconv.Itoa(pkg.LOC)+" LOC")),
			h.P(h.Class("mt-2 text-xs uppercase tracking-[0.18em] text-stone-500"), g.Text(strconv.Itoa(pkg.ImportsCount)+" imports · "+strconv.Itoa(pkg.ImportedByCount)+" dependents")),
		))
	}
	return h.Div(h.Class("space-y-3"), nodes)
}

func recentRunsNodes(runs []model.Run) g.Node {
	if len(runs) == 0 {
		return h.P(h.Class("text-sm text-stone-500"), g.Text("No runs yet."))
	}

	nodes := g.Group{}
	for _, run := range runs {
		nodes = append(nodes, h.Div(
			h.Class("flex flex-col gap-2 rounded-2xl border border-stone-200 px-4 py-3 md:flex-row md:items-center md:justify-between"),
			h.Div(
				h.P(h.Class("font-semibold"), g.Text(run.ID)),
				h.P(h.Class("text-xs text-stone-500"), g.Text(formatTimeValue(run.StartedAt))),
			),
			h.Div(
				h.Class("flex flex-wrap gap-4 text-sm text-stone-700"),
				h.Span(g.Text("Status: "), h.Strong(g.Text(run.Status))),
				h.Span(g.Text("Coverage: "), h.Strong(g.Text(run.CoverageStatus))),
				h.Span(g.Text(strconv.Itoa(run.FilesCount)+" files")),
			),
		))
	}
	return h.Div(h.Class("space-y-3"), nodes)
}

func runRows(runs []model.Run) g.Node {
	if len(runs) == 0 {
		return h.Tr(h.Td(h.Class("py-3 text-stone-500"), g.Attr("colspan", "6"), g.Text("No runs recorded yet.")))
	}

	rows := g.Group{}
	for _, run := range runs {
		rows = append(rows,
			h.Tr(
				h.Class("border-t border-stone-200 align-top"),
				h.Td(h.Class("py-3 font-medium"), g.Text(run.ID)),
				h.Td(h.Class("py-3"), g.Text(formatTimeValue(run.StartedAt))),
				h.Td(h.Class("py-3"), g.Text(run.Status)),
				h.Td(h.Class("py-3"), g.Text(run.CoverageStatus)),
				h.Td(h.Class("py-3"), g.Text(strconv.Itoa(run.FilesCount))),
				h.Td(h.Class("py-3"), g.Text(strconv.Itoa(run.PackagesCount))),
			),
		)
		if run.ErrorText != "" {
			rows = append(rows, h.Tr(
				h.Td(h.Class("pb-4 text-sm text-rose-700"), g.Attr("colspan", "6"), g.Text(run.ErrorText)),
			))
		}
	}
	return rows
}

func sortOption(value string, label string, selected string) g.Node {
	return h.Option(h.Value(value), g.If(value == selected, h.Selected()), g.Text(label))
}

func fileRows(repoID string, files []model.File) g.Node {
	if len(files) == 0 {
		return h.Tr(h.Td(h.Class("py-3 text-stone-500"), g.Attr("colspan", "6"), g.Text("No files matched the current filters.")))
	}

	rows := g.Group{}
	for _, item := range files {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class("py-3"), fileLink(repoID, item.Path)),
			h.Td(h.Class("py-3 text-stone-600"), g.Text(shortPkg(item.PackagePath))),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(item.LOC))),
			h.Td(h.Class("py-3"), g.Text(coverageText(item.CoveragePct))),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(item.FanIn))),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(item.FanOut))),
		))
	}
	return rows
}

func symbolNodes(symbols []model.Symbol) g.Node {
	if len(symbols) == 0 {
		return h.P(h.Class("text-stone-500"), g.Text("No symbols recorded."))
	}

	nodes := g.Group{}
	for _, symbol := range symbols {
		nodes = append(nodes, h.Div(
			h.Class("flex items-center justify-between rounded-2xl border border-stone-200 px-4 py-3"),
			h.Span(h.Class("font-medium"), g.Text(symbol.Name)),
			h.Span(h.Class("text-stone-500"), g.Text(symbol.Kind+" · line "+strconv.Itoa(symbol.Line))),
		))
	}
	return nodes
}

func topSymbols(symbols []model.Symbol, limit int) []model.Symbol {
	if len(symbols) <= limit {
		return symbols
	}
	return symbols[:limit]
}

func relatedTestNodes(repoID string, files []model.File) g.Node {
	if len(files) == 0 {
		return h.P(h.Class("text-stone-500"), g.Text("No test files found in this package directory."))
	}

	nodes := g.Group{}
	for _, item := range files {
		nodes = append(nodes, h.A(
			h.Class("block rounded-2xl border border-stone-200 px-4 py-3 font-medium underline"),
			h.Href(fileHref(repoID, item.Path)),
			g.Text(item.Path),
		))
	}
	return nodes
}

func fileEdgeNodes(repoID string, edges []model.FileEdge, inbound bool) g.Node {
	if len(edges) == 0 {
		message := "No outbound edges."
		if inbound {
			message = "No inbound edges."
		}
		return h.P(h.Class("text-stone-500"), g.Text(message))
	}

	nodes := g.Group{}
	for _, edge := range edges {
		targetPath := edge.ToPath
		if inbound {
			targetPath = edge.FromPath
		}
		nodes = append(nodes, h.Div(
			h.Class("rounded-2xl border border-stone-200 px-4 py-3"),
			h.A(h.Class("font-medium underline"), h.Href(fileHref(repoID, targetPath)), g.Text(targetPath)),
			h.P(h.Class("text-stone-500"), g.Text(edge.Kind+" · weight "+strconv.Itoa(edge.Weight))),
		))
	}
	return nodes
}

func fileConnectionRows(repoID string, inbound []model.FileEdge, outbound []model.FileEdge, limit int) g.Node {
	inboundItems := topFileEdges(inbound, true, limit)
	outboundItems := topFileEdges(outbound, false, limit)
	if len(inboundItems) == 0 && len(outboundItems) == 0 {
		return h.Tr(
			h.Td(
				h.Class("py-3 text-stone-500"),
				g.Attr("colspan", "4"),
				g.Text("No lineage connections recorded."),
			),
		)
	}

	rows := g.Group{}
	for _, edge := range inboundItems {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class("py-3 text-stone-500"), g.Text("Inbound")),
			h.Td(h.Class("py-3"), fileLink(repoID, edge.FromPath)),
			h.Td(h.Class("py-3"), g.Text(edge.Kind)),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(edge.Weight))),
		))
	}
	for _, edge := range outboundItems {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class("py-3 text-stone-500"), g.Text("Outbound")),
			h.Td(h.Class("py-3"), fileLink(repoID, edge.ToPath)),
			h.Td(h.Class("py-3"), g.Text(edge.Kind)),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(edge.Weight))),
		))
	}
	return rows
}

func topFileEdges(edges []model.FileEdge, inbound bool, limit int) []model.FileEdge {
	items := append([]model.FileEdge(nil), edges...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Weight == items[j].Weight {
			left := items[i].ToPath
			right := items[j].ToPath
			if inbound {
				left = items[i].FromPath
				right = items[j].FromPath
			}
			return left < right
		}
		return items[i].Weight > items[j].Weight
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func packageRows(repoID string, packages []model.Package) g.Node {
	if len(packages) == 0 {
		return h.Tr(h.Td(h.Class("py-3 text-stone-500"), g.Attr("colspan", "5"), g.Text("No packages recorded in the current snapshot.")))
	}

	rows := g.Group{}
	for _, item := range packages {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class("py-3"), packageLink(repoID, item.Path)),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(item.FileCount)+" (+"+strconv.Itoa(item.TestFileCount)+" tests)")),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(item.LOC))),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(item.ImportsCount))),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(item.ImportedByCount))),
		))
	}
	return rows
}

func topPackageEdges(edges []model.PackageEdge, inbound bool, limit int) []model.PackageEdge {
	items := append([]model.PackageEdge(nil), edges...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Weight == items[j].Weight {
			left := items[i].ToPath
			right := items[j].ToPath
			if inbound {
				left = items[i].FromPath
				right = items[j].FromPath
			}
			return left < right
		}
		return items[i].Weight > items[j].Weight
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func packageEdgeNodes(repoID string, edges []model.PackageEdge, inbound bool) g.Node {
	if len(edges) == 0 {
		message := "No outbound package dependencies."
		if inbound {
			message = "No inbound package dependents."
		}
		return h.P(h.Class("text-sm text-stone-500"), g.Text(message))
	}

	nodes := g.Group{}
	for _, edge := range edges {
		targetPath := edge.ToPath
		if inbound {
			targetPath = edge.FromPath
		}
		nodes = append(nodes, h.Div(
			h.Class("rounded-2xl border border-stone-200 px-4 py-3 text-sm"),
			h.A(h.Class("font-medium underline"), h.Href(packageHref(repoID, targetPath)), g.Text(targetPath)),
			h.P(h.Class("text-stone-500"), g.Text("weight "+strconv.Itoa(edge.Weight))),
		))
	}
	return nodes
}

func packageConnectionRows(repoID string, inbound []model.PackageEdge, outbound []model.PackageEdge, limit int) g.Node {
	inboundItems := topPackageEdges(inbound, true, limit)
	outboundItems := topPackageEdges(outbound, false, limit)
	if len(inboundItems) == 0 && len(outboundItems) == 0 {
		return h.Tr(
			h.Td(
				h.Class("py-3 text-stone-500"),
				g.Attr("colspan", "3"),
				g.Text("No package connections recorded."),
			),
		)
	}

	rows := g.Group{}
	for _, edge := range inboundItems {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class("py-3 text-stone-500"), g.Text("Inbound")),
			h.Td(h.Class("py-3"), packageLink(repoID, edge.FromPath)),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(edge.Weight))),
		))
	}
	for _, edge := range outboundItems {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class("py-3 text-stone-500"), g.Text("Outbound")),
			h.Td(h.Class("py-3"), packageLink(repoID, edge.ToPath)),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(edge.Weight))),
		))
	}
	return rows
}

func packageFileRows(repoID string, files []model.File) g.Node {
	if len(files) == 0 {
		return h.Tr(h.Td(h.Class("py-3 text-stone-500"), g.Attr("colspan", "3"), g.Text("No files recorded.")))
	}

	rows := g.Group{}
	for _, item := range files {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class("py-3"), fileLink(repoID, item.Path)),
			h.Td(h.Class("py-3"), g.Text(strconv.Itoa(item.LOC))),
			h.Td(h.Class("py-3"), g.Text(coverageText(item.CoveragePct))),
		))
	}
	return rows
}

func fileLink(repoID string, path string) g.Node {
	return h.A(h.Class("font-medium underline"), h.Href(fileHref(repoID, path)), g.Text(path))
}

func packageLink(repoID string, path string) g.Node {
	return h.A(h.Class("font-medium underline"), h.Href(packageHref(repoID, path)), g.Text(path))
}

func repoBaseHref(repoID string) string {
	return "/repos/" + pathEscape(repoID)
}

func repoRunsHref(repoID string) string {
	return repoBaseHref(repoID) + "/runs"
}

func repoFilesHref(repoID string) string {
	return repoBaseHref(repoID) + "/files"
}

func repoPackagesHref(repoID string) string {
	return repoBaseHref(repoID) + "/packages"
}

func repoRefreshHref(repoID string) string {
	return repoBaseHref(repoID) + "/refresh"
}

func fileHref(repoID string, path string) string {
	return repoFilesHref(repoID) + "/" + pathEscape(path)
}

func packageHref(repoID string, path string) string {
	return repoPackagesHref(repoID) + "/" + pathEscape(path)
}

func pathEscape(value string) string {
	return url.PathEscape(value)
}

func repoDisplayPath(repo config.Repository) string {
	if strings.TrimSpace(repo.SourcePath) != "" {
		return repo.SourcePath
	}
	return repo.Root
}

func coverageText(value *float64) string {
	if value == nil {
		return "n/a"
	}
	return strconv.FormatFloat(*value, 'f', 1, 64) + "%"
}

func formatTimeMeta(meta *model.SnapshotMeta) string {
	if meta == nil {
		return "Never"
	}
	return formatTimeValue(meta.RefreshedAt)
}

func formatTimeValue(value time.Time) string {
	return value.Local().Format("2006-01-02 15:04:05")
}

func shortPkg(value string) string {
	parts := strings.Split(value, "/")
	if len(parts) <= 3 {
		return value
	}
	return strings.Join(parts[len(parts)-3:], "/")
}

func snapshotStatusLabel(meta *model.SnapshotMeta) string {
	if meta == nil {
		return "No snapshot yet"
	}
	switch meta.CoverageStatus {
	case model.CoverageStatusAvailable:
		return "Snapshot ready"
	case model.CoverageStatusPending:
		return "Coverage pending"
	case model.CoverageStatusFailed:
		return "Snapshot partial"
	case model.CoverageStatusMissing:
		return "Coverage missing"
	default:
		return "Snapshot captured"
	}
}

func snapshotStatusTone(meta *model.SnapshotMeta) string {
	if meta == nil {
		return "neutral"
	}
	switch meta.CoverageStatus {
	case model.CoverageStatusAvailable:
		return "good"
	case model.CoverageStatusPending, model.CoverageStatusMissing:
		return "warn"
	case model.CoverageStatusFailed:
		return "critical"
	default:
		return "neutral"
	}
}

func fileHealthLabel(file model.File) string {
	switch fileHealthTone(file) {
	case "critical":
		return "Needs review"
	case "warn":
		return "Watch file"
	default:
		return "Stable file"
	}
}

func fileHealthTone(file model.File) string {
	coverage := derefCoverage(file.CoveragePct)
	switch {
	case file.LOC >= 1500 || (file.CoveragePct != nil && coverage < 35):
		return "critical"
	case file.LOC >= 700 || file.FanOut >= 12 || (file.CoveragePct != nil && coverage < 65):
		return "warn"
	default:
		return "good"
	}
}

func packageHealthLabel(pkg model.Package) string {
	switch packageHealthTone(pkg) {
	case "critical":
		return "Boundary risk"
	case "warn":
		return "Watch package"
	default:
		return "Healthy package"
	}
}

func packageHealthTone(pkg model.Package) string {
	switch {
	case pkg.LOC >= 5000:
		return "critical"
	case pkg.LOC >= 2000 || pkg.ImportsCount >= 12 || pkg.ImportedByCount >= 12:
		return "warn"
	default:
		return "good"
	}
}

func firstOrFallback(meta *model.SnapshotMeta, selector func(model.SnapshotMeta) string, fallback string) string {
	if meta == nil {
		return fallback
	}
	return selector(*meta)
}

func firstOrZero(meta *model.SnapshotMeta, selector func(model.SnapshotMeta) int) int {
	if meta == nil {
		return 0
	}
	return selector(*meta)
}

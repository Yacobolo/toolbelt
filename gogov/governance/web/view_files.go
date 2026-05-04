package web

import (
	"sort"
	"strconv"

	"github.com/Yacobolo/toolbelt/gogov/governance/model"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

func filesView(page filesData) g.Node {
	return h.Main(
		h.Class("space-y-6"),
		h.Section(
			h.Class("border-b border-stone-200 pb-5"),
			h.Form(
				h.Method("get"),
				h.Class("grid gap-4 lg:grid-cols-[1fr_1fr_180px_auto]"),
				h.Input(h.Class("border border-stone-300 px-4 py-3"), h.Type("search"), h.Name("q"), h.Placeholder("Filter by path"), h.Value(page.Filter)),
				h.Input(h.Class("border border-stone-300 px-4 py-3"), h.Type("search"), h.Name("package"), h.Placeholder("Filter by package path"), h.Value(page.PackageFilter)),
				h.Select(
					h.Class("border border-stone-300 px-4 py-3"),
					h.Name("sort"),
					sortOption("loc", "Sort by LOC", page.Sort),
					sortOption("coverage", "Sort by coverage", page.Sort),
					sortOption("fanin", "Sort by fan-in", page.Sort),
					sortOption("fanout", "Sort by fan-out", page.Sort),
				),
				h.Button(h.Class("border border-stone-950 bg-stone-950 px-5 py-3 text-sm font-semibold text-white"), h.Type("submit"), g.Text("Apply")),
			),
		),
		h.Section(
			h.Class("border border-stone-200 bg-white p-4 sm:p-6"),
			governanceTable([]string{"File", "Tags", "Package", "LOC", "Coverage", "Fan-in", "Fan-out"}, fileRows(page.RepoID, page.Files)),
		),
	)
}

func fileDetailView(page fileDetailData) g.Node {
	return h.Main(
		h.Class("space-y-6"),
		data.Signals(map[string]any{
			"fileGraph":  page.Graph,
			"fileSource": page.Source,
		}),
		detailTabs(
			page.ActiveTab,
			[]detailTab{
				{Value: "overview", Label: "Overview", Href: detailTabHref(fileHref(page.RepoID, page.File.Path), "overview")},
				{Value: "lineage", Label: "Lineage", Href: detailTabHref(fileHref(page.RepoID, page.File.Path), "lineage")},
				{Value: "source", Label: "Source", Href: detailTabHref(fileHref(page.RepoID, page.File.Path), "source")},
			},
		),
		detailPane(page.ActiveTab, "overview",
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
					h.Class("border border-stone-200 bg-white p-6"),
					h.H3(h.Class("text-xl font-bold"), g.Text("Key symbols")),
					h.Div(h.Class("mt-4 space-y-2 text-sm"), symbolNodes(topSymbols(page.Symbols, 12))),
				),
				h.Section(
					h.Class("border border-stone-200 bg-white p-6"),
					h.H3(h.Class("text-xl font-bold"), g.Text("Related tests")),
					h.Div(h.Class("mt-4 space-y-2 text-sm"), relatedTestNodes(page.RepoID, page.RelatedTests)),
				),
			),
			h.Div(
				h.Class("grid gap-6 xl:grid-cols-2"),
				h.Section(
					h.Class("border border-stone-200 bg-white p-6"),
					h.H3(h.Class("text-xl font-bold"), g.Text("Top dependents")),
					h.Div(h.Class("mt-4 space-y-2 text-sm"), fileEdgeNodes(page.RepoID, topFileEdges(page.Inbound, true, 8), true)),
				),
				h.Section(
					h.Class("border border-stone-200 bg-white p-6"),
					h.H3(h.Class("text-xl font-bold"), g.Text("Top dependencies")),
					h.Div(h.Class("mt-4 space-y-2 text-sm"), fileEdgeNodes(page.RepoID, topFileEdges(page.Outbound, false, 8), false)),
				),
			),
		),
		detailPane(page.ActiveTab, "lineage",
			graphWorkbenchStage(
				"lg:h-[calc(100vh-15rem)]",
				g.El(
					"governance-graph-view",
					h.Class("governance-graph-workbench block h-full"),
					g.Attr("graph-title", "File lineage"),
					data.Attr("graph", "$fileGraph"),
				),
				graphWorkbenchInspector(
					inspectorSection("File lineage", page.File.Path,
						h.P(h.Class("text-sm leading-6 text-stone-600"), g.Text("Trace upstream and downstream relationships without losing the source context of the current file.")),
						h.Div(
							h.Class("mt-4 grid gap-3 sm:grid-cols-2"),
							inspectorMetric("Package", shortPkg(page.File.PackagePath)),
							inspectorMetric("Coverage", coverageText(page.File.CoveragePct)),
							inspectorMetric("Fan-in", strconv.Itoa(page.File.FanIn)),
							inspectorMetric("Fan-out", strconv.Itoa(page.File.FanOut)),
						),
						h.Div(h.Class("mt-4"), fileTagBadges(page.File)),
					),
					inspectorSection("Signals", "Key symbols",
						h.Div(h.Class("space-y-2"), symbolNodes(topSymbols(page.Symbols, 8))),
					),
					inspectorSection("Related", "Test files",
						h.Div(h.Class("space-y-2"), relatedTestNodes(page.RepoID, page.RelatedTests)),
					),
					inspectorSection("Ledger", "Connections",
						governanceTable([]string{"Direction", "Connection", "Kind", "Weight"}, fileConnectionRows(page.RepoID, page.Inbound, page.Outbound, 24)),
					),
				),
			),
		),
		detailPane(page.ActiveTab, "source",
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

func sortOption(value string, label string, selected string) g.Node {
	return h.Option(h.Value(value), g.If(value == selected, h.Selected()), g.Text(label))
}

func fileRows(repoID string, files []model.File) g.Node {
	if len(files) == 0 {
		return h.Tr(h.Td(h.Class(governanceTableCellClass("text-stone-500")), g.Attr("colspan", "7"), g.Text("No files matched the current filters.")))
	}

	rows := g.Group{}
	for _, item := range files {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class(governanceTableCellClass("")), fileLink(repoID, item.Path)),
			h.Td(h.Class(governanceTableCellClass("")), fileTagBadges(item)),
			h.Td(h.Class(governanceTableCellClass("text-stone-600")), g.Text(shortPkg(item.PackagePath))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(item.LOC))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(coverageText(item.CoveragePct))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(item.FanIn))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(item.FanOut))),
		))
	}
	return rows
}

func fileTagBadges(file model.File) g.Node {
	tags := g.Group{}
	if file.IsIgnored {
		tags = append(tags, fileTagBadge("ignored", "border-amber-200 bg-amber-50 text-amber-800"))
	}
	if file.IsGenerated {
		tags = append(tags, fileTagBadge("generated", "border-sky-200 bg-sky-50 text-sky-800"))
	}
	if file.IsTest {
		tags = append(tags, fileTagBadge("test", "border-emerald-200 bg-emerald-50 text-emerald-800"))
	}
	if len(tags) == 0 {
		return nil
	}
	return h.Div(h.Class("flex flex-wrap gap-2"), tags)
}

func fileTagBadge(label string, classes string) g.Node {
	return h.Span(
		h.Class("inline-flex items-center border px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.14em] "+classes),
		g.Text(label),
	)
}

func symbolNodes(symbols []model.Symbol) g.Node {
	if len(symbols) == 0 {
		return h.P(h.Class("text-stone-500"), g.Text("No symbols recorded."))
	}

	nodes := g.Group{}
	for _, symbol := range symbols {
		nodes = append(nodes, h.Div(
			h.Class("flex items-center justify-between border border-stone-200 px-4 py-3"),
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
			h.Class("block border border-stone-200 px-4 py-3 font-medium underline"),
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
			h.Class("border border-stone-200 px-4 py-3"),
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
				h.Class(governanceTableCellClass("text-stone-500")),
				g.Attr("colspan", "4"),
				g.Text("No lineage connections recorded."),
			),
		)
	}

	rows := g.Group{}
	for _, edge := range inboundItems {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class(governanceTableCellClass("text-stone-500")), g.Text("Inbound")),
			h.Td(h.Class(governanceTableCellClass("")), fileLink(repoID, edge.FromPath)),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(edge.Kind)),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(edge.Weight))),
		))
	}
	for _, edge := range outboundItems {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class(governanceTableCellClass("text-stone-500")), g.Text("Outbound")),
			h.Td(h.Class(governanceTableCellClass("")), fileLink(repoID, edge.ToPath)),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(edge.Kind)),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(edge.Weight))),
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

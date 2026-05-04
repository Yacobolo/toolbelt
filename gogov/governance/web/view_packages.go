package web

import (
	"sort"
	"strconv"

	"github.com/Yacobolo/toolbelt/gogov/governance/model"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

func packagesView(page packagesData) g.Node {
	return h.Main(
		h.Class("h-full overflow-hidden"),
		data.Signals(map[string]any{"packageGraph": page.Graph}),
		graphWorkbenchStage(
			"h-full",
			g.El(
				"governance-graph-view",
				h.Class("governance-graph-workbench block h-full"),
				g.Attr("graph-title", "Package DAG"),
				g.Attr("graph-mode", "overview"),
				data.Attr("graph", "$packageGraph"),
			),
			graphWorkbenchInspector(
				inspectorSection("Overview", "Architecture map",
					h.P(h.Class("text-sm leading-6 text-stone-600"), g.Text("Use the canvas as the primary workspace. Pan across the layered dependency flow, then open packages from the inventory on the right.")),
					h.Div(
						h.Class("mt-4 grid gap-3 sm:grid-cols-2"),
						inspectorMetric("Packages", strconv.Itoa(len(page.Packages))),
						inspectorMetric("Total LOC", strconv.Itoa(totalPackageLOC(page.Packages))),
						inspectorMetric("Module", modulePath(page.Meta)),
						inspectorMetric("Snapshot", snapshotStatusLabel(page.Meta)),
					),
				),
				inspectorSection("Inventory", "Package catalog",
					packageInspectorNodes(page.RepoID, modulePath(page.Meta), page.Packages),
				),
			),
		),
	)
}

func packageDetailView(page packageDetailData) g.Node {
	return h.Main(
		h.Class("space-y-6"),
		data.Signals(map[string]any{
			"packageGraph": page.Graph,
		}),
		h.Section(
			h.ID("package-header"),
			h.Class("border-b border-stone-200 pb-6"),
			h.P(h.Class("text-xs uppercase tracking-[0.2em] text-stone-500"), g.Text(page.Package.Name)),
			h.H2(h.Class("mt-2 text-3xl font-black tracking-tight text-stone-950"), g.Text(packageDisplayPath(page.Package))),
		),
		detailTabs(
			page.ActiveTab,
			[]detailTab{
				{Value: "overview", Label: "Overview", Href: detailTabHref(packageHref(page.RepoID, page.Package.Path, page.Meta), "overview")},
				{Value: "neighborhood", Label: "Neighborhood", Href: detailTabHref(packageHref(page.RepoID, page.Package.Path, page.Meta), "neighborhood")},
				{Value: "files", Label: "Files", Href: detailTabHref(packageHref(page.RepoID, page.Package.Path, page.Meta), "files")},
			},
		),
		detailPane(page.ActiveTab, "overview",
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
					h.Class("border border-stone-200 bg-white p-6"),
					h.H3(h.Class("text-xl font-bold"), g.Text("Dependencies")),
					h.Div(h.Class("mt-4 space-y-2"), packageEdgeNodes(page.RepoID, modulePath(page.Meta), topPackageEdges(page.Outbound, false, 10), false)),
				),
				h.Section(
					h.Class("border border-stone-200 bg-white p-6"),
					h.H3(h.Class("text-xl font-bold"), g.Text("Dependents")),
					h.Div(h.Class("mt-4 space-y-2"), packageEdgeNodes(page.RepoID, modulePath(page.Meta), topPackageEdges(page.Inbound, true, 10), true)),
				),
			),
		),
		detailPane(page.ActiveTab, "neighborhood",
			graphWorkbenchStage(
				"lg:h-[calc(100vh-15rem)]",
				g.El(
					"governance-graph-view",
					h.Class("governance-graph-workbench block h-full"),
					g.Attr("graph-title", "Package neighborhood"),
					data.Attr("graph", "$packageGraph"),
				),
				graphWorkbenchInspector(
					inspectorSection("Focused package", packageDisplayPath(page.Package),
						h.P(h.Class("text-sm leading-6 text-stone-600"), g.Text("Inspect the selected package against its immediate neighborhood without collapsing the architectural context around it.")),
						h.Div(
							h.Class("mt-4 grid gap-3 sm:grid-cols-2"),
							inspectorMetric("Files", strconv.Itoa(page.Package.FileCount)),
							inspectorMetric("LOC", strconv.Itoa(page.Package.LOC)),
							inspectorMetric("Dependencies", strconv.Itoa(page.Package.ImportsCount)),
							inspectorMetric("Dependents", strconv.Itoa(page.Package.ImportedByCount)),
						),
					),
					inspectorSection("Outgoing", "Dependencies",
						h.Div(h.Class("space-y-2"), packageEdgeNodes(page.RepoID, modulePath(page.Meta), topPackageEdges(page.Outbound, false, 8), false)),
					),
					inspectorSection("Incoming", "Dependents",
						h.Div(h.Class("space-y-2"), packageEdgeNodes(page.RepoID, modulePath(page.Meta), topPackageEdges(page.Inbound, true, 8), true)),
					),
					inspectorSection("Ledger", "Connections",
						governanceTable([]string{"Direction", "Package", "Weight"}, packageConnectionRows(page.RepoID, modulePath(page.Meta), page.Inbound, page.Outbound, 24)),
					),
				),
			),
		),
		detailPane(page.ActiveTab, "files",
			h.Section(
				h.ID("package-files"),
				h.Class(workspaceCanvasClass("overflow-hidden p-4 sm:p-6")),
				governanceTable([]string{"File", "LOC", "Coverage"}, packageFileRows(page.RepoID, page.Files)),
			),
		),
	)
}

func packageInspectorNodes(repoID string, modulePath string, packages []model.Package) g.Node {
	if len(packages) == 0 {
		return h.P(h.Class("text-sm text-stone-500"), g.Text("No packages recorded in the current snapshot."))
	}

	nodes := g.Group{}
	for _, item := range packages {
		nodes = append(nodes, h.Div(
			h.Class("border-t border-stone-200 py-4 first:border-t-0 first:pt-0"),
			h.A(
				h.Class("block text-sm font-semibold text-stone-950 underline"),
				h.Href(packageHref(repoID, item.Path, &model.SnapshotMeta{ModulePath: modulePath})),
				g.Text(packageListLabel(item.Path, modulePath)),
			),
			h.Div(
				h.Class("mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-xs uppercase tracking-[0.14em] text-stone-500"),
				inspectorListStat("Files", strconv.Itoa(item.FileCount)),
				inspectorListStat("LOC", strconv.Itoa(item.LOC)),
				inspectorListStat("Out", strconv.Itoa(item.ImportsCount)),
				inspectorListStat("In", strconv.Itoa(item.ImportedByCount)),
			),
		))
	}
	return h.Div(h.Class("divide-y divide-transparent"), nodes)
}

func totalPackageLOC(packages []model.Package) int {
	total := 0
	for _, item := range packages {
		total += item.LOC
	}
	return total
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

func packageEdgeNodes(repoID string, modulePath string, edges []model.PackageEdge, inbound bool) g.Node {
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
			h.Class("border border-stone-200 px-4 py-3 text-sm"),
			h.A(h.Class("font-medium underline"), h.Href(packageHref(repoID, targetPath, &model.SnapshotMeta{ModulePath: modulePath})), g.Text(packageListLabel(targetPath, modulePath))),
			h.P(h.Class("text-stone-500"), g.Text("weight "+strconv.Itoa(edge.Weight))),
		))
	}
	return nodes
}

func packageConnectionRows(repoID string, modulePath string, inbound []model.PackageEdge, outbound []model.PackageEdge, limit int) g.Node {
	inboundItems := topPackageEdges(inbound, true, limit)
	outboundItems := topPackageEdges(outbound, false, limit)
	if len(inboundItems) == 0 && len(outboundItems) == 0 {
		return h.Tr(
			h.Td(
				h.Class(governanceTableCellClass("text-stone-500")),
				g.Attr("colspan", "3"),
				g.Text("No package connections recorded."),
			),
		)
	}

	rows := g.Group{}
	for _, edge := range inboundItems {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class(governanceTableCellClass("text-stone-500")), g.Text("Inbound")),
			h.Td(h.Class(governanceTableCellClass("")), packageLink(repoID, modulePath, edge.FromPath)),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(edge.Weight))),
		))
	}
	for _, edge := range outboundItems {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class(governanceTableCellClass("text-stone-500")), g.Text("Outbound")),
			h.Td(h.Class(governanceTableCellClass("")), packageLink(repoID, modulePath, edge.ToPath)),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(edge.Weight))),
		))
	}
	return rows
}

func packageFileRows(repoID string, files []model.File) g.Node {
	if len(files) == 0 {
		return h.Tr(h.Td(h.Class(governanceTableCellClass("text-stone-500")), g.Attr("colspan", "3"), g.Text("No files recorded.")))
	}

	rows := g.Group{}
	for _, item := range files {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class(governanceTableCellClass("")), fileLink(repoID, item.Path)),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(item.LOC))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(coverageText(item.CoveragePct))),
		))
	}
	return rows
}

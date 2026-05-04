package web

import (
	"strconv"

	"github.com/Yacobolo/toolbelt/gogov/governance/model"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func homeView(page homeData) g.Node {
	if len(page.Summaries) == 0 {
		return h.Main(
			h.Class("border border-stone-200 bg-white p-6"),
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
		h.Class("border border-stone-200 bg-white p-6"),
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
					h.Class("border border-stone-200 bg-[color:oklch(0.985_0.003_85)] p-5"),
					labelWithIcon("refresh", "Last run"),
					h.P(h.Class("mt-2 text-sm font-semibold"), g.Text(lastRunText)),
				),
			),
			h.Div(
				h.Class("flex flex-wrap gap-3"),
				h.A(h.Class("inline-flex items-center gap-2 border border-stone-300 px-4 py-2 text-sm font-semibold text-stone-700 transition hover:bg-stone-100"), h.Href(repoBaseHref(summary.Repo.ID)), uiIcon("overview", "h-4 w-4 shrink-0"), g.Text("Open catalog")),
				h.Form(
					h.Method("post"),
					h.Action(repoRefreshHref(summary.Repo.ID)),
					h.Button(h.Class("inline-flex items-center gap-2 border border-stone-950 bg-stone-950 px-4 py-2 text-sm font-semibold text-stone-50 transition hover:bg-stone-800"), h.Type("submit"), uiIcon("refresh", "h-4 w-4 shrink-0"), g.Text("Run scan")),
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
				h.Class("border border-stone-200 bg-white p-5"),
				labelWithIcon("refresh", "Last refreshed"),
				h.P(h.Class("mt-2 text-sm font-semibold"), g.Text(formatTimeMeta(page.Meta))),
			),
		),
		h.Section(
			h.Class("grid gap-8 lg:grid-cols-[1.25fr_0.95fr]"),
			h.Div(
				h.Class("border border-stone-200 bg-white p-6"),
				h.Div(
					h.Class("mb-4 flex items-center justify-between"),
					h.H2(h.Class("text-2xl font-bold"), g.Text("Largest files")),
					h.A(h.Class("text-sm font-semibold text-stone-700 underline"), h.Href(repoFilesHref(page.RepoID)+"?sort=loc"), g.Text("Open full inventory")),
				),
				governanceTable([]string{"File", "LOC", "Coverage", "Lineage"}, largestFilesRows(page.RepoID, page.BigFiles)),
			),
			h.Div(
				h.Class("border border-stone-200 bg-white p-6"),
				h.Div(
					h.Class("mb-4 flex items-center justify-between"),
					h.H2(h.Class("text-2xl font-bold"), g.Text("Largest packages")),
					h.A(h.Class("text-sm font-semibold text-stone-700 underline"), h.Href(repoPackagesHref(page.RepoID)), g.Text("Open package catalog")),
				),
				hotPackagesNodes(page.RepoID, modulePath(page.Meta), page.HotPackages),
			),
		),
		h.Section(
			h.Class("border border-stone-200 bg-white p-6"),
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
		h.Class("border border-stone-200 bg-white p-6"),
		h.Div(
			h.Class("mb-4 flex items-end justify-between"),
			h.H2(h.Class("text-2xl font-bold"), g.Text("Run history")),
		),
		h.Div(
			governanceTable([]string{"Run", "Started", "Status", "Coverage", "Files", "Packages"}, runRows(page.Runs)),
		),
	)
}

func largestFilesRows(repoID string, files []model.File) g.Node {
	if len(files) == 0 {
		return h.Tr(h.Td(h.Class(governanceTableCellClass("text-stone-500")), g.Attr("colspan", "4"), g.Text("Run a refresh to populate file inventory.")))
	}

	rows := g.Group{}
	for _, item := range files {
		rows = append(rows, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class(governanceTableCellClass("")), fileLink(repoID, item.Path)),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(item.LOC))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(coverageText(item.CoveragePct))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(item.FanIn)+" in / "+strconv.Itoa(item.FanOut)+" out")),
		))
	}
	return rows
}

func hotPackagesNodes(repoID string, modulePath string, packages []model.Package) g.Node {
	if len(packages) == 0 {
		return h.P(h.Class("text-sm text-stone-500"), g.Text("No packages recorded in the current snapshot."))
	}

	nodes := g.Group{}
	for _, pkg := range packages {
		nodes = append(nodes, h.Div(
			h.Class("border border-stone-200 p-4"),
			h.P(h.Class("text-sm font-semibold text-stone-900"), packageLink(repoID, modulePath, pkg.Path)),
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
			h.Class("flex flex-col gap-2 border border-stone-200 px-4 py-3 md:flex-row md:items-center md:justify-between"),
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
		return h.Tr(h.Td(h.Class(governanceTableCellClass("text-stone-500")), g.Attr("colspan", "6"), g.Text("No runs recorded yet.")))
	}

	rows := g.Group{}
	for _, run := range runs {
		rows = append(rows,
			h.Tr(
				h.Class("border-t border-stone-200 align-top"),
				h.Td(h.Class(governanceTableCellClass("font-medium")), g.Text(run.ID)),
				h.Td(h.Class(governanceTableCellClass("")), g.Text(formatTimeValue(run.StartedAt))),
				h.Td(h.Class(governanceTableCellClass("")), g.Text(run.Status)),
				h.Td(h.Class(governanceTableCellClass("")), g.Text(run.CoverageStatus)),
				h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(run.FilesCount))),
				h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(run.PackagesCount))),
			),
		)
		if run.ErrorText != "" {
			rows = append(rows, h.Tr(
				h.Td(h.Class(governanceTableCellClass("pt-0 text-sm text-rose-700")), g.Attr("colspan", "6"), g.Text(run.ErrorText)),
			))
		}
	}
	return rows
}

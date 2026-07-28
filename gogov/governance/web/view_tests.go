package web

import (
	"sort"
	"strconv"

	"github.com/Yacobolo/toolbelt/gogov/governance/model"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func testsView(page testsData) g.Node {
	return h.Main(
		h.Class("space-y-6"),
		h.Section(
			h.Class("grid gap-4 md:grid-cols-2 xl:grid-cols-4"),
			testsKpiCard("critical", "Critical gaps", strconv.Itoa(page.Summary.CriticalGaps), "Packages with missing local test contracts on meaningful production surface."),
			testsKpiCard("warn", "High-risk packages with no tests", strconv.Itoa(page.Summary.HighRiskPackagesWithoutTests), "Large or central packages with no local contract files yet."),
			testsKpiCard("warn", "Low-coverage packages with tests", strconv.Itoa(page.Summary.LowCoveragePackagesWithTests), "Existing tests that still leave too much production surface unexercised."),
			testsKpiCard("tests", "Unbacked production files", strconv.Itoa(page.Summary.ProdFilesWithoutLocalBacking), "Production files in packages with no obvious local test backing."),
		),
		h.Section(
			h.Class("space-y-4"),
			h.P(
				h.Class("max-w-3xl text-sm leading-6 text-stone-600"),
				g.Text("Use this page as a contract triage queue. Start with the packages below that carry the highest blast radius and weakest local test posture, then drill into the full contracts, gaps, or raw test inventory only when you need more evidence."),
			),
			detailTabs(
				page.ActiveTab,
				[]detailTab{
					{Value: "recommendations", Label: "Recommendations", Href: testsTabHref(page.RepoID, "recommendations")},
					{Value: "contracts", Label: "Contracts", Href: testsTabHref(page.RepoID, "contracts")},
					{Value: "gaps", Label: "Gaps", Href: testsTabHref(page.RepoID, "gaps")},
					{Value: "inventory", Label: "Inventory", Href: testsTabHref(page.RepoID, "inventory")},
				},
			),
		),
		detailPane(page.ActiveTab, "recommendations",
			h.Section(
				h.Class("border border-stone-200 bg-white p-6"),
				h.Div(
					h.Class("mb-5 space-y-2"),
					h.H2(h.Class("text-2xl font-bold"), g.Text("Recommended next actions")),
					h.P(h.Class("text-sm leading-6 text-stone-600"), g.Text("These packages rank highest by missing contracts, weak coverage, production surface size, and dependency blast radius.")),
				),
				testRecommendationNodes(page.Meta, page.RepoID, page.Recommendations),
			),
		),
		detailPane(page.ActiveTab, "contracts",
			h.Section(
				h.Class("border border-stone-200 bg-white p-6"),
				h.Div(
					h.Class("mb-5 space-y-2"),
					h.H2(h.Class("text-2xl font-bold"), g.Text("Contract coverage")),
					h.P(h.Class("text-sm leading-6 text-stone-600"), g.Text("Read tests as contracts around the production surface. This tab keeps the full package inventory for auditing once you know where to look first.")),
				),
				governanceTable([]string{"Package", "Score", "Dependents", "Coverage", "Tests", "Surface", "Driver"}, testContractRows(page.Meta, page.RepoID, page.Contracts)),
			),
		),
		detailPane(page.ActiveTab, "gaps",
			h.Section(
				h.Class("border border-stone-200 bg-white p-6"),
				h.Div(
					h.Class("mb-5 space-y-2"),
					h.H2(h.Class("text-2xl font-bold"), g.Text("Critical gaps")),
					h.P(h.Class("text-sm leading-6 text-stone-600"), g.Text("These packages have the weakest contract posture based on missing tests, missing coverage, or a large production surface with minimal local test backing.")),
				),
				governanceTable([]string{"Package", "Score", "Dependents", "Coverage", "Tests", "Surface", "Driver"}, testGapRows(page.Meta, page.RepoID, page.Gaps)),
			),
		),
		detailPane(page.ActiveTab, "inventory",
			h.Section(
				h.Class("border border-stone-200 bg-white p-6"),
				h.Div(
					h.Class("mb-5 space-y-2"),
					h.H2(h.Class("text-2xl font-bold"), g.Text("Test inventory")),
					h.P(h.Class("text-sm leading-6 text-stone-600"), g.Text("Inventory of `_test.go` files that act as executable contracts for the repository.")),
				),
				governanceTable([]string{"Test file", "Package", "Test symbols", "Coverage", "Tags"}, testInventoryRows(page.Meta, page.RepoID, page.Inventory)),
			),
		),
	)
}

func buildTestsData(repoID string, activeTab string, meta *model.SnapshotMeta, packages []model.Package, files []model.File, testFiles []model.File) testsData {
	filesByPackage := map[string][]model.File{}
	for _, item := range files {
		filesByPackage[item.PackagePath] = append(filesByPackage[item.PackagePath], item)
	}

	summary := testsSummary{}
	contracts := make([]testContractRow, 0, len(packages))
	recommendations := make([]testRecommendationRow, 0, len(packages))
	inventory := make([]testInventoryRow, 0, len(testFiles))

	for _, item := range testFiles {
		if item.IsGenerated {
			continue
		}
		inventory = append(inventory, testInventoryRow{
			File:            item,
			TestSymbolCount: item.FunctionCount,
		})
	}
	summary.TotalTestFiles = len(inventory)

	for _, pkg := range packages {
		packageFiles := filesByPackage[pkg.Path]
		prodFiles := filterProdFiles(packageFiles)
		prodFileCount := len(prodFiles)
		if prodFileCount == 0 {
			continue
		}
		prodLOC := aggregateLOC(prodFiles)
		effectivePkg := pkg
		effectivePkg.LOC = prodLOC

		testFileCount := countTestFiles(packageFiles)
		if testFileCount > 0 {
			summary.PackagesWithTests++
		} else {
			summary.PackagesWithoutTests++
			summary.ProdFilesWithoutLocalBacking += prodFileCount
			if isHighRiskNoTests(effectivePkg, prodFileCount) {
				summary.HighRiskPackagesWithoutTests++
			}
		}

		coverage := aggregateCoverage(prodFiles)
		if testFileCount > 0 && isLowCoverageContract(coverage) {
			summary.LowCoveragePackagesWithTests++
		}

		statusLabel, statusTone, riskNote := packageContractStatus(effectivePkg, prodFileCount, testFileCount, coverage)
		score := recommendationScore(testContractRow{
			ProdFiles:       prodFileCount,
			TestFiles:       testFileCount,
			CoveragePct:     coverage,
			StatusTone:      statusTone,
			ImportedByCount: effectivePkg.ImportedByCount,
			LOC:             effectivePkg.LOC,
		})
		contract := testContractRow{
			PackagePath:     pkg.Path,
			ProdFiles:       prodFileCount,
			TestFiles:       testFileCount,
			CoveragePct:     coverage,
			Score:           score,
			StatusLabel:     statusLabel,
			StatusTone:      statusTone,
			RiskNote:        riskNote,
			ImportedByCount: effectivePkg.ImportedByCount,
			ImportsCount:    effectivePkg.ImportsCount,
			LOC:             effectivePkg.LOC,
		}
		contracts = append(contracts, contract)
		if contract.StatusTone == "critical" {
			summary.CriticalGaps++
		}
		if recommendation, ok := buildRecommendation(contract); ok {
			recommendations = append(recommendations, recommendation)
		}
	}

	sort.Slice(recommendations, func(i, j int) bool {
		left, right := recommendations[i], recommendations[j]
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if severityRank(left.StatusTone) != severityRank(right.StatusTone) {
			return severityRank(left.StatusTone) > severityRank(right.StatusTone)
		}
		if left.ImportedByCount != right.ImportedByCount {
			return left.ImportedByCount > right.ImportedByCount
		}
		if left.LOC != right.LOC {
			return left.LOC > right.LOC
		}
		return left.PackagePath < right.PackagePath
	})
	if len(recommendations) > 8 {
		recommendations = recommendations[:8]
	}

	sort.Slice(contracts, func(i, j int) bool {
		left, right := contracts[i], contracts[j]
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if severityRank(left.StatusTone) != severityRank(right.StatusTone) {
			return severityRank(left.StatusTone) > severityRank(right.StatusTone)
		}
		if left.ImportedByCount != right.ImportedByCount {
			return left.ImportedByCount > right.ImportedByCount
		}
		if left.LOC != right.LOC {
			return left.LOC > right.LOC
		}
		return left.PackagePath < right.PackagePath
	})

	gaps := make([]testGapRow, 0, len(contracts))
	for _, row := range contracts {
		if row.StatusTone == "good" {
			continue
		}
		gaps = append(gaps, testGapRow{
			Score:           row.Score,
			PackagePath:     row.PackagePath,
			StatusLabel:     row.StatusLabel,
			StatusTone:      row.StatusTone,
			Issue:           row.RiskNote,
			ProdFiles:       row.ProdFiles,
			TestFiles:       row.TestFiles,
			CoveragePct:     row.CoveragePct,
			ImportedByCount: row.ImportedByCount,
			LOC:             row.LOC,
		})
	}

	sort.Slice(gaps, func(i, j int) bool {
		left, right := gaps[i], gaps[j]
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if severityRank(left.StatusTone) != severityRank(right.StatusTone) {
			return severityRank(left.StatusTone) > severityRank(right.StatusTone)
		}
		if left.ImportedByCount != right.ImportedByCount {
			return left.ImportedByCount > right.ImportedByCount
		}
		if left.LOC != right.LOC {
			return left.LOC > right.LOC
		}
		return left.PackagePath < right.PackagePath
	})

	sort.Slice(inventory, func(i, j int) bool {
		if inventory[i].File.PackagePath == inventory[j].File.PackagePath {
			return inventory[i].File.Path < inventory[j].File.Path
		}
		return inventory[i].File.PackagePath < inventory[j].File.PackagePath
	})

	return testsData{
		RepoID:          repoID,
		Meta:            meta,
		ActiveTab:       activeTab,
		Summary:         summary,
		Recommendations: recommendations,
		Contracts:       contracts,
		Gaps:            gaps,
		Inventory:       inventory,
	}
}

func testRecommendationNodes(meta *model.SnapshotMeta, repoID string, rows []testRecommendationRow) g.Node {
	if len(rows) == 0 {
		return h.Div(
			h.Class("border border-stone-200 bg-stone-50 px-5 py-4 text-sm text-stone-600"),
			g.Text("No immediate contract actions stand out in the current snapshot."),
		)
	}

	module := modulePath(meta)
	nodes := g.Group{}
	for _, row := range rows {
		packageURL := packageHref(repoID, row.PackagePath, &model.SnapshotMeta{ModulePath: module})
		nodes = append(nodes, h.Article(
			h.Class("border-t border-stone-200 py-5 first:border-t-0 first:pt-0 last:pb-0"),
			h.Div(
				h.Class("flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between"),
				h.Div(
					h.Class("space-y-3"),
					h.Div(
						h.Class("flex flex-wrap items-center gap-3"),
						h.P(h.Class("text-xs uppercase tracking-[0.18em] text-stone-500"), g.Text(packageListLabel(row.PackagePath, module))),
						contractScoreBadge(row.Score, row.StatusTone),
					),
					h.Div(
						h.Class("space-y-2"),
						h.H3(h.Class("text-xl font-semibold tracking-[-0.03em] text-stone-950"), g.Text(row.Title)),
						h.P(h.Class("max-w-3xl text-sm leading-6 text-stone-600"), g.Text(row.Summary)),
					),
					h.Div(h.Class("flex flex-wrap gap-2 text-xs font-medium text-stone-600"), recommendationFacts(row)),
				),
				h.Div(
					h.Class("flex shrink-0 flex-wrap gap-2"),
					testsActionLink(packageURL, "Open package"),
					testsActionLink(detailTabHref(packageURL, "files"), "Review files"),
				),
			),
		))
	}
	return nodes
}

func recommendationFacts(row testRecommendationRow) g.Node {
	facts := []string{
		strconv.Itoa(row.ProdFiles) + " prod files / " + strconv.Itoa(row.TestFiles) + " test files",
		"Coverage " + coverageText(row.CoveragePct),
		testBlastRadius(testGapRow{ImportedByCount: row.ImportedByCount, LOC: row.LOC}),
	}

	nodes := g.Group{}
	for _, fact := range facts {
		nodes = append(nodes, h.Span(
			h.Class("inline-flex items-center border border-stone-200 bg-stone-50 px-2.5 py-1"),
			g.Text(fact),
		))
	}
	return nodes
}

func testsActionLink(href string, label string) g.Node {
	return h.A(
		h.Class("inline-flex items-center gap-2 border border-stone-300 px-3 py-2 text-sm font-semibold text-stone-700 transition hover:bg-stone-100"),
		h.Href(href),
		uiIcon("next", "h-4 w-4 shrink-0 text-stone-400"),
		g.Text(label),
	)
}

func testsKpiCard(icon string, label string, value string, note string) g.Node {
	return h.Div(
		h.Class("border border-stone-200 bg-white p-5"),
		labelWithIcon(icon, label),
		h.P(h.Class("mt-3 text-3xl font-black tracking-[-0.04em] text-stone-950"), g.Text(value)),
		h.P(h.Class("mt-2 text-sm leading-6 text-stone-600"), g.Text(note)),
	)
}

func buildRecommendation(row testContractRow) (testRecommendationRow, bool) {
	if row.StatusTone == "good" {
		return testRecommendationRow{}, false
	}

	title := "Review contract posture"
	summary := row.RiskNote
	switch {
	case row.TestFiles == 0:
		title = "Add first contract tests"
		summary = "This package has no local test contracts, so changes can land without an executable boundary around the production surface."
	case row.CoveragePct == nil:
		title = "Connect coverage to existing tests"
		summary = "Tests exist, but there is no production coverage signal yet, making it hard to trust how much of the contract is really exercised."
	case derefCoverage(row.CoveragePct) < 55:
		title = "Raise contract coverage"
		summary = "Tests exist, but too much of the package remains unexercised for a dependable contract."
	case row.TestFiles == 1 && hasBroadSurface(row):
		title = "Broaden contract surface"
		summary = "A single test file is carrying a broad package contract and is likely too thin for the amount of code behind it."
	}

	return testRecommendationRow{
		PackagePath:     row.PackagePath,
		StatusLabel:     row.StatusLabel,
		StatusTone:      row.StatusTone,
		Title:           title,
		Summary:         summary,
		ProdFiles:       row.ProdFiles,
		TestFiles:       row.TestFiles,
		CoveragePct:     row.CoveragePct,
		ImportedByCount: row.ImportedByCount,
		LOC:             row.LOC,
		Score:           row.Score,
	}, true
}

func recommendationScore(row testContractRow) int {
	score := 0
	if row.StatusTone == "critical" {
		score += 20
	}
	if row.TestFiles == 0 {
		score += 70
	} else if row.TestFiles == 1 {
		score += 10
	}

	switch {
	case row.CoveragePct == nil:
		score += 20
	case derefCoverage(row.CoveragePct) < 25:
		score += 35
	case derefCoverage(row.CoveragePct) < 55:
		score += 25
	case derefCoverage(row.CoveragePct) < 70:
		score += 10
	}

	switch {
	case row.ImportedByCount >= 12:
		score += 25
	case row.ImportedByCount >= 5:
		score += 15
	case row.ImportedByCount > 0:
		score += 8
	}

	switch {
	case row.LOC >= 1500:
		score += 25
	case row.LOC >= 700:
		score += 15
	case row.LOC >= 250:
		score += 8
	}

	switch {
	case row.ProdFiles >= 8:
		score += 15
	case row.ProdFiles >= 4:
		score += 8
	}

	if row.TestFiles == 1 && hasBroadSurface(row) {
		score += 15
	}

	return score
}

func testContractRows(meta *model.SnapshotMeta, repoID string, rows []testContractRow) g.Node {
	if len(rows) == 0 {
		return h.Tr(h.Td(h.Class(governanceTableCellClass("text-stone-500")), g.Attr("colspan", "7"), g.Text("No production package contracts recorded in the current snapshot.")))
	}

	module := modulePath(meta)
	nodes := g.Group{}
	for _, row := range rows {
		nodes = append(nodes, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class(governanceTableCellClass("")), packageLink(repoID, module, row.PackagePath)),
			h.Td(h.Class(governanceTableCellClass("")), contractScoreBadge(row.Score, row.StatusTone)),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(dependentsText(row.ImportedByCount))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(coverageText(row.CoveragePct))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(testFileText(row.TestFiles))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(surfaceText(row.ProdFiles, row.LOC))),
			h.Td(h.Class(governanceTableCellClass("text-stone-600")), g.Text(contractDriverText(row))),
		))
	}
	return nodes
}

func testGapRows(meta *model.SnapshotMeta, repoID string, rows []testGapRow) g.Node {
	if len(rows) == 0 {
		return h.Tr(h.Td(h.Class(governanceTableCellClass("text-stone-500")), g.Attr("colspan", "7"), g.Text("No contract gaps detected in the current snapshot.")))
	}

	module := modulePath(meta)
	nodes := g.Group{}
	for _, row := range rows {
		nodes = append(nodes, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class(governanceTableCellClass("")), packageLink(repoID, module, row.PackagePath)),
			h.Td(h.Class(governanceTableCellClass("")), contractScoreBadge(row.Score, row.StatusTone)),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(dependentsText(row.ImportedByCount))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(coverageText(row.CoveragePct))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(testFileText(row.TestFiles))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(surfaceText(row.ProdFiles, row.LOC))),
			h.Td(h.Class(governanceTableCellClass("text-stone-600")), g.Text(row.Issue)),
		))
	}
	return nodes
}

func contractScoreBadge(score int, tone string) g.Node {
	return statusBadge(strconv.Itoa(score), tone)
}

func dependentsText(count int) string {
	if count == 1 {
		return "1 dep"
	}
	return strconv.Itoa(count) + " deps"
}

func testFileText(count int) string {
	if count == 1 {
		return "1 file"
	}
	return strconv.Itoa(count) + " files"
}

func surfaceText(prodFiles int, loc int) string {
	return strconv.Itoa(prodFiles) + " files · " + strconv.Itoa(loc) + " LOC"
}

func contractDriverText(row testContractRow) string {
	return row.RiskNote
}

func testInventoryRows(meta *model.SnapshotMeta, repoID string, rows []testInventoryRow) g.Node {
	if len(rows) == 0 {
		return h.Tr(h.Td(h.Class(governanceTableCellClass("text-stone-500")), g.Attr("colspan", "5"), g.Text("No `_test.go` files recorded in the current snapshot.")))
	}

	module := modulePath(meta)
	nodes := g.Group{}
	for _, row := range rows {
		nodes = append(nodes, h.Tr(
			h.Class("border-t border-stone-200"),
			h.Td(h.Class(governanceTableCellClass("")), fileLink(repoID, row.File.Path)),
			h.Td(h.Class(governanceTableCellClass("")), packageLink(repoID, module, row.File.PackagePath)),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(strconv.Itoa(row.TestSymbolCount))),
			h.Td(h.Class(governanceTableCellClass("")), g.Text(coverageText(row.File.CoveragePct))),
			h.Td(h.Class(governanceTableCellClass("")), fileTagBadges(row.File)),
		))
	}
	return nodes
}

func filterProdFiles(files []model.File) []model.File {
	out := make([]model.File, 0, len(files))
	for _, item := range files {
		if item.IsTest || item.IsGenerated {
			continue
		}
		out = append(out, item)
	}
	return out
}

func countTestFiles(files []model.File) int {
	count := 0
	for _, item := range files {
		if item.IsTest && !item.IsGenerated {
			count++
		}
	}
	return count
}

func aggregateLOC(files []model.File) int {
	total := 0
	for _, item := range files {
		total += item.LOC
	}
	return total
}

func aggregateCoverage(files []model.File) *float64 {
	covered := 0
	total := 0
	for _, item := range files {
		if item.TotalStatements <= 0 {
			continue
		}
		covered += item.CoveredStatements
		total += item.TotalStatements
	}
	if total == 0 {
		return nil
	}
	value := (float64(covered) / float64(total)) * 100
	return &value
}

func packageContractStatus(pkg model.Package, prodFiles int, testFiles int, coverage *float64) (string, string, string) {
	meaningfulSurface := hasMeaningfulSurface(pkg, prodFiles)
	if testFiles == 0 {
		if meaningfulSurface {
			return "Missing", "critical", "No local test contracts for this production package."
		}
		return "Thin", "warn", "No local test files yet."
	}

	if coverage == nil {
		return "Watch", "warn", "Tests exist but there is no production coverage signal yet."
	}
	if *coverage < 55 {
		return "Watch", "warn", "Tests exist but aggregate production coverage remains low."
	}
	if testFiles == 1 && (prodFiles >= 5 || pkg.LOC >= 900 || pkg.ImportedByCount >= 8) {
		return "Thin", "warn", "A single test file backs a broad production surface."
	}

	return "Healthy", "good", strconv.Itoa(testFiles) + " local test files back this package contract."
}

func isHighRiskNoTests(pkg model.Package, prodFiles int) bool {
	return prodFiles >= 4 || pkg.LOC >= 700 || pkg.ImportedByCount >= 3
}

func isLowCoverageContract(coverage *float64) bool {
	return coverage == nil || *coverage < 55
}

func hasMeaningfulSurface(pkg model.Package, prodFiles int) bool {
	return prodFiles >= 2 || pkg.LOC >= 120 || pkg.ImportedByCount > 0 || pkg.ImportsCount >= 3
}

func hasBroadSurface(row testContractRow) bool {
	return row.ProdFiles >= 5 || row.LOC >= 900 || row.ImportedByCount >= 8
}

func severityRank(tone string) int {
	switch tone {
	case "critical":
		return 3
	case "warn":
		return 2
	case "good":
		return 1
	default:
		return 0
	}
}

func testBlastRadius(row testGapRow) string {
	if row.ImportedByCount > 0 {
		return strconv.Itoa(row.ImportedByCount) + " dependents"
	}
	return strconv.Itoa(row.LOC) + " LOC"
}

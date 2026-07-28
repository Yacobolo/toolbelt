package web

import (
	"testing"

	"github.com/Yacobolo/toolbelt/gogov/governance/model"
)

func TestBuildTestsDataIgnoresGeneratedFilesForContractMetrics(t *testing.T) {
	t.Parallel()

	packages := []model.Package{
		{
			Path:            "example.com/repo/internal/store",
			Name:            "store",
			Dir:             "internal/store",
			LOC:             4620,
			ImportedByCount: 4,
			ImportsCount:    3,
		},
		{
			Path:            "example.com/repo/internal/store/queries",
			Name:            "queries",
			Dir:             "internal/store/queries",
			LOC:             4500,
			ImportedByCount: 2,
			ImportsCount:    1,
		},
	}

	files := []model.File{
		{
			Path:              "internal/store/repository.go",
			PackagePath:       "example.com/repo/internal/store",
			PackageName:       "store",
			LOC:               120,
			CoveredStatements: 5,
			TotalStatements:   10,
		},
		{
			Path:            "internal/store/queries/queries.sql.go",
			PackagePath:     "example.com/repo/internal/store",
			PackageName:     "store",
			LOC:             4500,
			IsGenerated:     true,
			TotalStatements: 400,
		},
		{
			Path:          "internal/store/repository_test.go",
			PackagePath:   "example.com/repo/internal/store",
			PackageName:   "store",
			LOC:           80,
			IsTest:        true,
			FunctionCount: 3,
		},
		{
			Path:          "internal/store/repository_gen_test.go",
			PackagePath:   "example.com/repo/internal/store",
			PackageName:   "store",
			LOC:           80,
			IsTest:        true,
			IsGenerated:   true,
			FunctionCount: 2,
		},
		{
			Path:        "internal/store/queries/queries.sql.go",
			PackagePath: "example.com/repo/internal/store/queries",
			PackageName: "queries",
			LOC:         4500,
			IsGenerated: true,
			CoveragePct: nil,
		},
	}

	testFiles := []model.File{files[2], files[3]}

	data := buildTestsData("repo", "recommendations", nil, packages, files, testFiles)

	if data.Summary.TotalTestFiles != 1 {
		t.Fatalf("TotalTestFiles = %d, want 1", data.Summary.TotalTestFiles)
	}
	if len(data.Inventory) != 1 {
		t.Fatalf("len(Inventory) = %d, want 1", len(data.Inventory))
	}
	if got := data.Inventory[0].File.Path; got != "internal/store/repository_test.go" {
		t.Fatalf("inventory file = %q, want repository_test.go", got)
	}
	if len(data.Contracts) != 1 {
		t.Fatalf("len(Contracts) = %d, want 1", len(data.Contracts))
	}

	contract := data.Contracts[0]
	if contract.PackagePath != "example.com/repo/internal/store" {
		t.Fatalf("contract.PackagePath = %q, want internal/store package", contract.PackagePath)
	}
	if contract.ProdFiles != 1 {
		t.Fatalf("contract.ProdFiles = %d, want 1", contract.ProdFiles)
	}
	if contract.TestFiles != 1 {
		t.Fatalf("contract.TestFiles = %d, want 1", contract.TestFiles)
	}
	if contract.LOC != 120 {
		t.Fatalf("contract.LOC = %d, want 120", contract.LOC)
	}
	if contract.CoveragePct == nil || *contract.CoveragePct != 50 {
		t.Fatalf("contract.CoveragePct = %v, want 50", contract.CoveragePct)
	}
}

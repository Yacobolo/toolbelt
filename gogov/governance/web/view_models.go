package web

import (
	"github.com/Yacobolo/toolbelt/gogov/governance/config"
	"github.com/Yacobolo/toolbelt/gogov/governance/model"
)

type repoSummary struct {
	Repo    config.Repository
	Meta    *model.SnapshotMeta
	LastRun *model.Run
}

type homeData struct {
	Summaries []repoSummary
}

type dashboardData struct {
	RepoID      string
	Meta        *model.SnapshotMeta
	Runs        []model.Run
	BigFiles    []model.File
	HotPackages []model.Package
}

type runsData struct {
	RepoID string
	Meta   *model.SnapshotMeta
	Runs   []model.Run
}

type filesData struct {
	RepoID        string
	Meta          *model.SnapshotMeta
	Files         []model.File
	Filter        string
	PackageFilter string
	Sort          string
}

type fileDetailData struct {
	RepoID       string
	Meta         *model.SnapshotMeta
	File         model.File
	ActiveTab    string
	Symbols      []model.Symbol
	Inbound      []model.FileEdge
	Outbound     []model.FileEdge
	RelatedTests []model.File
	Graph        graphResponse
	Source       fileSourceResponse
}

type fileDirectoryData struct {
	RepoID          string
	Meta            *model.SnapshotMeta
	Path            string
	Entries         []fileDirectoryEntry
	DirectDirCount  int
	DirectFileCount int
	TotalFileCount  int
	TotalLOC        int
	TestFileCount   int
	GeneratedCount  int
}

type fileDirectoryEntry struct {
	Name           string
	Path           string
	IsDirectory    bool
	LOC            int
	TotalFileCount int
	TestFileCount  int
	GeneratedCount int
	CoveragePct    *float64
	File           *model.File
}

type packagesData struct {
	RepoID   string
	Meta     *model.SnapshotMeta
	Packages []model.Package
	Graph    graphResponse
}

type testsData struct {
	RepoID          string
	Meta            *model.SnapshotMeta
	ActiveTab       string
	Summary         testsSummary
	Recommendations []testRecommendationRow
	Contracts       []testContractRow
	Gaps            []testGapRow
	Inventory       []testInventoryRow
}

type testsSummary struct {
	TotalTestFiles               int
	PackagesWithTests            int
	PackagesWithoutTests         int
	CriticalGaps                 int
	HighRiskPackagesWithoutTests int
	LowCoveragePackagesWithTests int
	ProdFilesWithoutLocalBacking int
}

type testRecommendationRow struct {
	PackagePath     string
	StatusLabel     string
	StatusTone      string
	Title           string
	Summary         string
	ProdFiles       int
	TestFiles       int
	CoveragePct     *float64
	ImportedByCount int
	LOC             int
	Score           int
}

type testContractRow struct {
	PackagePath     string
	ProdFiles       int
	TestFiles       int
	CoveragePct     *float64
	Score           int
	StatusLabel     string
	StatusTone      string
	RiskNote        string
	ImportedByCount int
	ImportsCount    int
	LOC             int
}

type testGapRow struct {
	Score           int
	PackagePath     string
	StatusLabel     string
	StatusTone      string
	Issue           string
	ProdFiles       int
	TestFiles       int
	CoveragePct     *float64
	ImportedByCount int
	LOC             int
}

type testInventoryRow struct {
	File            model.File
	TestSymbolCount int
}

type packageDetailData struct {
	RepoID    string
	Meta      *model.SnapshotMeta
	Package   model.Package
	ActiveTab string
	Files     []model.File
	Inbound   []model.PackageEdge
	Outbound  []model.PackageEdge
	Graph     graphResponse
}

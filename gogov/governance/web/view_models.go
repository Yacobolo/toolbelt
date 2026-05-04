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

type packagesData struct {
	RepoID   string
	Meta     *model.SnapshotMeta
	Packages []model.Package
	Graph    graphResponse
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

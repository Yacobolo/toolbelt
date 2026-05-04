package model

import "time"

const (
	RunStatusRunning   = "running"
	RunStatusSucceeded = "succeeded"
	RunStatusPartial   = "partial"
	RunStatusFailed    = "failed"

	CoverageStatusPending   = "pending"
	CoverageStatusAvailable = "available"
	CoverageStatusMissing   = "missing"
	CoverageStatusFailed    = "failed"
)

type Run struct {
	ID                string
	Status            string
	CoverageStatus    string
	ErrorText         string
	RepoRoot          string
	ModulePath        string
	CommitSHA         string
	StartedAt         time.Time
	CompletedAt       *time.Time
	DurationMS        int64
	FilesCount        int
	PackagesCount     int
	PackageEdgesCount int
	FileEdgesCount    int
	ViolationsCount   int
}

type SnapshotMeta struct {
	RunID             string
	RepoRoot          string
	ModulePath        string
	CommitSHA         string
	RefreshedAt       time.Time
	CoverageStatus    string
	FilesCount        int
	PackagesCount     int
	ViolationsCount   int
	PackageEdgesCount int
	FileEdgesCount    int
}

type Package struct {
	Path            string
	Name            string
	Dir             string
	FileCount       int
	TestFileCount   int
	LOC             int
	NonEmptyLOC     int
	ImportsCount    int
	ImportedByCount int
	ViolationCount  int
}

type File struct {
	Path                string
	Dir                 string
	PackagePath         string
	PackageName         string
	LOC                 int
	NonEmptyLOC         int
	FunctionCount       int
	ExportedSymbolCount int
	IsTest              bool
	IsGenerated         bool
	IsIgnored           bool
	FanIn               int
	FanOut              int
	ViolationCount      int
	CoveredStatements   int
	TotalStatements     int
	CoveragePct         *float64
}

type Symbol struct {
	FilePath string
	Name     string
	Kind     string
	Line     int
	Exported bool
}

type PackageEdge struct {
	FromPath string
	ToPath   string
	Weight   int
}

type FileEdge struct {
	FromPath string
	ToPath   string
	Weight   int
	Kind     string
}

type CoverageFile struct {
	Path              string
	CoveredStatements int
	TotalStatements   int
	CoveragePct       float64
}

type Violation struct {
	ID          int64
	ScopeType   string
	ScopeKey    string
	RuleName    string
	Message     string
	FromPackage string
	ToPackage   string
	Severity    string
}

type Snapshot struct {
	Meta         SnapshotMeta
	Packages     []Package
	Files        []File
	Symbols      []Symbol
	PackageEdges []PackageEdge
	FileEdges    []FileEdge
	Coverage     []CoverageFile
	Violations   []Violation
}

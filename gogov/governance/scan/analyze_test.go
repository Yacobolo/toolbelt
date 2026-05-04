package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yacobolo/toolbelt/gogov/governance/config"
	"github.com/Yacobolo/toolbelt/gogov/governance/model"
)

func TestAnalyzeFixtureRepoBuildsLineageAndCoverage(t *testing.T) {
	t.Parallel()

	repoRoot := copyFixtureRepo(t, filepath.Join("testdata", "fixturemod"))
	repo := config.Repository{
		ID:                 "fixturemod",
		Name:               "fixturemod",
		Root:               repoRoot,
		RuntimeDir:         filepath.Join(t.TempDir(), ".governance", "repos", "fixturemod"),
		DatabasePath:       filepath.Join(t.TempDir(), ".governance", "repos", "fixturemod", "governance.db"),
		LockPath:           filepath.Join(t.TempDir(), ".governance", "repos", "fixturemod", "refresh.lock"),
		CoverageOutputPath: filepath.Join(t.TempDir(), ".governance", "repos", "fixturemod", "coverage.out"),
	}

	result, err := Analyze(context.Background(), repo, nilLogger())
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	assertFilePresent(t, result, "internal/services/usecase_test.go")
	assertEdgePresent(t, result, "internal/services/usecase.go", "internal/services/extra.go", "symbol")
	assertEdgePresent(t, result, "internal/platform/http/handler.go", "internal/services/usecase.go", "symbol")
	assertEdgePresent(t, result, "internal/services/extra.go", "internal/domain/port.go", "implements")

	for _, edge := range result.FileEdges {
		if edge.ToPath == "fmt" || edge.FromPath == "fmt" {
			t.Fatalf("unexpected external edge: %+v", edge)
		}
	}

	if result.Meta.CoverageStatus != "available" {
		t.Fatalf("coverage status = %q", result.Meta.CoverageStatus)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected no violations, got %+v", result.Violations)
	}
}

func copyFixtureRepo(t *testing.T, source string) string {
	t.Helper()

	destination := filepath.Join(t.TempDir(), "fixturemod")
	if err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0750)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, info.Mode()); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("copy fixture repo: %v", err)
	}
	return destination
}

func TestAnalyzeCurrentRepoSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	if os.Getenv("GOVERNANCE_SMOKE") == "" {
		t.Skip("set GOVERNANCE_SMOKE=1 to run the current repo smoke test")
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}

	repo := config.Repository{
		ID:                 "ai-platform",
		Name:               "ai-platform",
		Root:               repoRoot,
		RuntimeDir:         filepath.Join(t.TempDir(), ".governance", "repos", "ai-platform"),
		DatabasePath:       filepath.Join(t.TempDir(), ".governance", "repos", "ai-platform", "governance.db"),
		LockPath:           filepath.Join(t.TempDir(), ".governance", "repos", "ai-platform", "refresh.lock"),
		CoverageOutputPath: filepath.Join(t.TempDir(), ".governance", "repos", "ai-platform", "coverage.out"),
	}

	result, err := Analyze(context.Background(), repo, nilLogger())
	if err != nil {
		t.Fatalf("Analyze() smoke error = %v", err)
	}
	if result.Meta.PackagesCount == 0 || result.Meta.FilesCount == 0 {
		t.Fatalf("unexpected empty result: %+v", result.Meta)
	}
}

func TestAnalyzeBrokenRepoReturnsPartialSnapshot(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join(t.TempDir(), "brokenmod")
	mustWriteFile(t, filepath.Join(repoRoot, "go.mod"), "module example.com/brokenmod\n\ngo 1.24\n")
	mustWriteFile(t, filepath.Join(repoRoot, "main.go"), `package main

import "example.com/brokenmod/lib"

func main() {
	_ = lib.Hello()
}
`)
	mustWriteFile(t, filepath.Join(repoRoot, "lib", "lib.go"), `package lib

func Hello() string {
	return helper()
}
`)
	mustWriteFile(t, filepath.Join(repoRoot, "lib", "helper.go"), `package lib

func helper() string {
	return "hello"
}
`)
	mustWriteFile(t, filepath.Join(repoRoot, "broken", "bad.go"), `package broken

var _ = missingSymbol
`)

	repo := config.Repository{
		ID:                 "brokenmod",
		Name:               "brokenmod",
		Root:               repoRoot,
		RuntimeDir:         filepath.Join(t.TempDir(), ".governance", "repos", "brokenmod"),
		DatabasePath:       filepath.Join(t.TempDir(), ".governance", "repos", "brokenmod", "governance.db"),
		LockPath:           filepath.Join(t.TempDir(), ".governance", "repos", "brokenmod", "refresh.lock"),
		CoverageOutputPath: filepath.Join(t.TempDir(), ".governance", "repos", "brokenmod", "coverage.out"),
	}

	result, err := Analyze(context.Background(), repo, nilLogger())
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Meta.FilesCount == 0 || result.Meta.PackagesCount == 0 {
		t.Fatalf("expected non-empty snapshot, got %+v", result.Meta)
	}
	assertFilePresent(t, result, "broken/bad.go")
	assertEdgePresent(t, result, "main.go", "lib/lib.go", "symbol")
}

func assertFilePresent(t *testing.T, result model.Snapshot, path string) {
	t.Helper()
	for _, file := range result.Files {
		if file.Path == path {
			return
		}
	}
	t.Fatalf("file %q not present", path)
}

func assertEdgePresent(t *testing.T, result model.Snapshot, from string, to string, kind string) {
	t.Helper()
	for _, edge := range result.FileEdges {
		if edge.FromPath == from && edge.ToPath == to && edge.Kind == kind {
			return
		}
	}
	t.Fatalf("edge %s -> %s (%s) not present: %+v", from, to, kind, result.FileEdges)
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

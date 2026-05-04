package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yacobolo/toolbelt/gogov/governance/config"
	"github.com/Yacobolo/toolbelt/gogov/governance/model"
	"github.com/Yacobolo/toolbelt/gogov/governance/store"
)

type fakeRuntime struct {
	stores map[string]*store.Store
	runID  string
	err    error
}

func (f fakeRuntime) Store(repoID string) (*store.Store, bool) {
	st, ok := f.stores[repoID]
	return st, ok
}

func (f fakeRuntime) StartRefresh(context.Context, string) (string, error) {
	return f.runID, f.err
}

func TestRoutesRenderMultiRepoPagesAndRefreshRedirect(t *testing.T) {
	t.Parallel()

	hostRoot := t.TempDir()
	repoOneRoot := filepath.Join(t.TempDir(), "ai-platform")
	repoTwoRoot := filepath.Join(t.TempDir(), "duck-demo")

	mustWriteFile(t, filepath.Join(repoOneRoot, "go.mod"), "module example.com/repo\n\ngo 1.23.0\n")
	mustWriteFile(t, filepath.Join(repoOneRoot, "internal", "services", "usecase.go"), "package services\n\nfunc Run() {}\n")
	mustWriteFile(t, filepath.Join(repoOneRoot, "internal", "ui", "page.go"), "package ui\n\nfunc Page() {}\n")
	mustWriteFile(t, filepath.Join(repoTwoRoot, "go.mod"), "module example.com/duck\n\ngo 1.23.0\n")
	mustWriteFile(t, filepath.Join(repoTwoRoot, "main.go"), "package main\n\nfunc main() {}\n")

	repoOne := config.Repository{
		ID:                 "ai-platform",
		Name:               "ai-platform",
		Root:               repoOneRoot,
		RuntimeDir:         filepath.Join(hostRoot, ".governance", "repos", "ai-platform"),
		DatabasePath:       filepath.Join(hostRoot, ".governance", "repos", "ai-platform", "governance.db"),
		LockPath:           filepath.Join(hostRoot, ".governance", "repos", "ai-platform", "refresh.lock"),
		CoverageOutputPath: filepath.Join(hostRoot, ".governance", "repos", "ai-platform", "coverage.out"),
	}
	repoTwo := config.Repository{
		ID:                 "duck-demo",
		Name:               "duck-demo",
		Root:               repoTwoRoot,
		RuntimeDir:         filepath.Join(hostRoot, ".governance", "repos", "duck-demo"),
		DatabasePath:       filepath.Join(hostRoot, ".governance", "repos", "duck-demo", "governance.db"),
		LockPath:           filepath.Join(hostRoot, ".governance", "repos", "duck-demo", "refresh.lock"),
		CoverageOutputPath: filepath.Join(hostRoot, ".governance", "repos", "duck-demo", "coverage.out"),
	}

	cfg := config.Config{
		HostRoot:     hostRoot,
		BindAddress:  config.DefaultBindAddress,
		Repositories: []config.Repository{repoOne, repoTwo},
	}

	repoOneStore, err := store.Open(repoOne.DatabasePath)
	if err != nil {
		t.Fatalf("store.Open(repoOne) error = %v", err)
	}
	defer func() { _ = repoOneStore.Close() }()

	repoTwoStore, err := store.Open(repoTwo.DatabasePath)
	if err != nil {
		t.Fatalf("store.Open(repoTwo) error = %v", err)
	}
	defer func() { _ = repoTwoStore.Close() }()

	now := time.Now().UTC()
	if err := repoOneStore.CreateRun(context.Background(), model.Run{
		ID:             "run-1",
		Status:         model.RunStatusSucceeded,
		CoverageStatus: model.CoverageStatusAvailable,
		RepoRoot:       repoOneRoot,
		ModulePath:     "example.com/repo",
		StartedAt:      now,
	}); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := repoOneStore.ReplaceSnapshot(context.Background(), model.Snapshot{
		Meta: model.SnapshotMeta{
			RunID:             "run-1",
			RepoRoot:          repoOneRoot,
			ModulePath:        "example.com/repo",
			CommitSHA:         "abc123",
			RefreshedAt:       now,
			CoverageStatus:    model.CoverageStatusAvailable,
			FilesCount:        2,
			PackagesCount:     2,
			PackageEdgesCount: 1,
			FileEdgesCount:    1,
		},
		Packages: []model.Package{
			{Path: "example.com/repo/internal/services", Name: "services", Dir: "internal/services", FileCount: 1, LOC: 20},
			{Path: "example.com/repo/internal/ui", Name: "ui", Dir: "internal/ui", FileCount: 1, LOC: 10},
		},
		Files: []model.File{
			{Path: "internal/services/usecase.go", Dir: "internal/services", PackagePath: "example.com/repo/internal/services", PackageName: "services", LOC: 20, NonEmptyLOC: 18},
			{Path: "internal/ui/page.go", Dir: "internal/ui", PackagePath: "example.com/repo/internal/ui", PackageName: "ui", LOC: 10, NonEmptyLOC: 9},
		},
		Symbols:      []model.Symbol{{FilePath: "internal/services/usecase.go", Name: "Run", Kind: "func", Line: 3}},
		PackageEdges: []model.PackageEdge{{FromPath: "example.com/repo/internal/ui", ToPath: "example.com/repo/internal/services", Weight: 1}},
		FileEdges:    []model.FileEdge{{FromPath: "internal/ui/page.go", ToPath: "internal/services/usecase.go", Weight: 1, Kind: "symbol"}},
	}); err != nil {
		t.Fatalf("ReplaceSnapshot() error = %v", err)
	}

	server := httptest.NewServer(New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), fakeRuntime{
		stores: map[string]*store.Store{
			repoOne.ID: repoOneStore,
			repoTwo.ID: repoTwoStore,
		},
		runID: "run-2",
	}).Routes())
	defer server.Close()

	assertPageContains(t, server.URL+"/", "ai-platform")
	assertPageContains(t, server.URL+"/", "duck-demo")
	assertPageContains(t, server.URL+"/repos/ai-platform", "Largest files")
	assertPageContains(t, server.URL+"/repos/ai-platform", "/repos/duck-demo")
	assertPageContains(t, server.URL+"/repos/ai-platform/files", "internal/services/usecase.go")
	assertPageContains(t, server.URL+"/repos/ai-platform/files/internal/services/usecase.go", "?tab=source")
	assertPageContains(t, server.URL+"/repos/ai-platform/files/internal/services/usecase.go?tab=source", "governance-code-viewer")
	assertPageContains(t, server.URL+"/repos/ai-platform/files/internal/services/usecase.go", "internal/services/usecase.go")
	assertPageContains(t, server.URL+"/repos/ai-platform/packages", "governance-graph-view")
	assertPageContains(t, server.URL+"/repos/ai-platform/packages/internal/ui", "Neighborhood")

	assertStatusCode(t, server.URL+"/architecture", http.StatusNotFound)
	assertStatusCode(t, server.URL+"/repos/ai-platform/architecture", http.StatusNotFound)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequest(http.MethodPost, server.URL+"/repos/ai-platform/refresh", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("refresh status = %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); !strings.Contains(location, "/repos/ai-platform/runs?message=") {
		t.Fatalf("unexpected refresh location: %s", location)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}

func assertPageContains(t *testing.T, url string, needle string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("http.Get(%s) error = %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll(%s) error = %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s returned status %d", url, resp.StatusCode)
	}
	if !strings.Contains(string(body), needle) {
		t.Fatalf("%s body did not contain %q:\n%s", url, needle, string(body))
	}
}

func assertStatusCode(t *testing.T, url string, want int) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("http.Get(%s) error = %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		t.Fatalf("%s returned status %d, want %d", url, resp.StatusCode, want)
	}
}

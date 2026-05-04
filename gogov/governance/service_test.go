package governance

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yacobolo/toolbelt/gogov/governance/config"
	"github.com/Yacobolo/toolbelt/gogov/governance/model"

	"log/slog"
)

func TestRefreshLockPreventsConcurrentRuns(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), ".governance", "refresh.lock")
	lock, err := acquireRefreshLock(lockPath)
	if err != nil {
		t.Fatalf("acquireRefreshLock() error = %v", err)
	}
	defer lock.Release()

	if _, err := acquireRefreshLock(lockPath); err != ErrRefreshRunning {
		t.Fatalf("expected ErrRefreshRunning, got %v", err)
	}
}

func TestRefreshPersistsPartialRunWhenCoverageFails(t *testing.T) {
	t.Parallel()

	repoRoot := copyFixtureRepo(t, filepath.Join("scan", "testdata", "fixturemod"))
	hostRoot := t.TempDir()
	repo := config.Repository{
		ID:                 "fixturemod",
		Name:               "fixturemod",
		Root:               repoRoot,
		RuntimeDir:         filepath.Join(hostRoot, ".governance", "repos", "fixturemod"),
		DatabasePath:       filepath.Join(hostRoot, ".governance", "repos", "fixturemod", "governance.db"),
		LockPath:           filepath.Join(hostRoot, ".governance", "repos", "fixturemod", "refresh.lock"),
		CoverageOutputPath: filepath.Join(hostRoot, ".governance", "repos", "fixturemod", "coverage.out"),
	}
	cfg := config.Config{
		HostRoot:     hostRoot,
		BindAddress:  config.DefaultBindAddress,
		Repositories: []config.Repository{repo},
	}

	app, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = app.Close() }()

	run, err := app.Refresh(context.Background(), repo.ID)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if run.Status != model.RunStatusSucceeded && run.Status != model.RunStatusPartial {
		t.Fatalf("run status = %q", run.Status)
	}
	if run.CoverageStatus != model.CoverageStatusAvailable && run.CoverageStatus != model.CoverageStatusFailed {
		t.Fatalf("coverage status = %q", run.CoverageStatus)
	}

	st, ok := app.Store(repo.ID)
	if !ok {
		t.Fatalf("Store(%q) returned false", repo.ID)
	}
	meta, err := st.GetSnapshotMeta(context.Background())
	if err != nil {
		t.Fatalf("GetSnapshotMeta() error = %v", err)
	}
	if meta.CoverageStatus != run.CoverageStatus {
		t.Fatalf("snapshot coverage status = %q", meta.CoverageStatus)
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

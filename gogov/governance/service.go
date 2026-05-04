package governance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yacobolo/toolbelt/gogov/governance/config"
	"github.com/Yacobolo/toolbelt/gogov/governance/model"
	"github.com/Yacobolo/toolbelt/gogov/governance/scan"
	"github.com/Yacobolo/toolbelt/gogov/governance/store"
	"github.com/Yacobolo/toolbelt/gogov/governance/web"

	"github.com/google/uuid"
)

var ErrRefreshRunning = errors.New("refresh already running")

type App struct {
	cfg      config.Config
	logger   *slog.Logger
	stores   map[string]*store.Store
	ordered  []config.Repository
	storeWeb *web.Server
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	stores := make(map[string]*store.Store, len(cfg.Repositories))
	for _, repo := range cfg.Repositories {
		st, err := store.Open(repo.DatabasePath)
		if err != nil {
			for _, opened := range stores {
				_ = opened.Close()
			}
			return nil, err
		}
		stores[repo.ID] = st
	}

	app := &App{
		cfg:     cfg,
		logger:  logger,
		stores:  stores,
		ordered: append([]config.Repository(nil), cfg.Repositories...),
	}
	app.storeWeb = web.New(cfg, logger, app)
	return app, nil
}

func (a *App) Close() error {
	errs := make([]error, 0, len(a.stores))
	for _, st := range a.stores {
		if err := st.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (a *App) Serve(ctx context.Context) error {
	if err := a.storeWeb.EnsureAssetsBuilt(ctx); err != nil {
		return err
	}

	server := &http.Server{
		Addr:              a.cfg.BindAddress,
		Handler:           a.storeWeb.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("gogov server started", "addr", a.cfg.BindAddress, "repositories", len(a.ordered), "host_root", a.cfg.HostRoot)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (a *App) Repositories() []config.Repository {
	return append([]config.Repository(nil), a.ordered...)
}

func (a *App) Store(repoID string) (*store.Store, bool) {
	st, ok := a.stores[repoID]
	return st, ok
}

func (a *App) Refresh(ctx context.Context, repoID string) (model.Run, error) {
	repo, st, err := a.repoBinding(repoID)
	if err != nil {
		return model.Run{}, err
	}

	lock, err := acquireRefreshLock(repo.LockPath)
	if err != nil {
		return model.Run{}, err
	}
	defer lock.Release()

	return a.refreshWithRunID(ctx, repo, st, uuid.NewString())
}

func (a *App) StartRefresh(ctx context.Context, repoID string) (string, error) {
	repo, st, err := a.repoBinding(repoID)
	if err != nil {
		return "", err
	}

	lock, err := acquireRefreshLock(repo.LockPath)
	if err != nil {
		return "", err
	}

	runID := uuid.NewString()
	go func() {
		defer lock.Release()
		runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if _, err := a.refreshWithRunID(runCtx, repo, st, runID); err != nil {
			a.logger.Error("refresh failed", "repo_id", repo.ID, "run_id", runID, "error", err)
		}
	}()
	return runID, nil
}

func (a *App) repoBinding(repoID string) (config.Repository, *store.Store, error) {
	repo, ok := a.cfg.Repository(repoID)
	if !ok {
		valid := make([]string, 0, len(a.ordered))
		for _, item := range a.ordered {
			valid = append(valid, item.ID)
		}
		return config.Repository{}, nil, fmt.Errorf("unknown repository %q (valid: %s)", repoID, strings.Join(valid, ", "))
	}
	st, ok := a.stores[repoID]
	if !ok {
		return config.Repository{}, nil, fmt.Errorf("repository %q store is unavailable", repoID)
	}
	return repo, st, nil
}

func (a *App) refreshWithRunID(ctx context.Context, repo config.Repository, st *store.Store, runID string) (model.Run, error) {
	startedAt := time.Now().UTC()
	run := model.Run{
		ID:             runID,
		Status:         model.RunStatusRunning,
		CoverageStatus: model.CoverageStatusPending,
		RepoRoot:       repo.Root,
		StartedAt:      startedAt,
	}
	if err := st.CreateRun(ctx, run); err != nil {
		return model.Run{}, err
	}

	result, analyzeErr := scan.Analyze(ctx, repo, a.logger)
	completedAt := time.Now().UTC()

	run.CompletedAt = &completedAt
	run.DurationMS = completedAt.Sub(startedAt).Milliseconds()

	if analyzeErr != nil {
		run.Status = model.RunStatusFailed
		run.CoverageStatus = model.CoverageStatusFailed
		run.ErrorText = analyzeErr.Error()
		if err := st.UpdateRun(ctx, run); err != nil {
			return model.Run{}, errors.Join(analyzeErr, err)
		}
		return run, analyzeErr
	}

	result.Meta.RunID = runID
	result.Meta.RefreshedAt = completedAt
	result.Meta.CommitSHA = GitCommitSHA(ctx, repo.Root)

	run.Status = model.RunStatusSucceeded
	if result.Meta.CoverageStatus != model.CoverageStatusAvailable {
		run.Status = model.RunStatusPartial
	}
	run.CoverageStatus = result.Meta.CoverageStatus
	run.ModulePath = result.Meta.ModulePath
	run.CommitSHA = result.Meta.CommitSHA
	run.FilesCount = result.Meta.FilesCount
	run.PackagesCount = result.Meta.PackagesCount
	run.PackageEdgesCount = result.Meta.PackageEdgesCount
	run.FileEdgesCount = result.Meta.FileEdgesCount
	run.ViolationsCount = result.Meta.ViolationsCount

	if err := st.ReplaceSnapshot(ctx, result); err != nil {
		run.Status = model.RunStatusFailed
		run.CoverageStatus = model.CoverageStatusFailed
		run.ErrorText = err.Error()
		if updateErr := st.UpdateRun(ctx, run); updateErr != nil {
			return model.Run{}, errors.Join(err, updateErr)
		}
		return run, err
	}

	if err := st.UpdateRun(ctx, run); err != nil {
		return model.Run{}, err
	}
	return run, nil
}

type refreshLock struct {
	path string
}

func acquireRefreshLock(path string) (*refreshLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			if stale, staleErr := isStaleLock(path); staleErr == nil && stale {
				_ = os.Remove(path)
				return acquireRefreshLock(path)
			}
			return nil, ErrRefreshRunning
		}
		return nil, fmt.Errorf("acquire refresh lock: %w", err)
	}
	defer file.Close()

	content := fmt.Sprintf("pid=%d\nstarted_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
	if _, err := file.WriteString(content); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("write refresh lock: %w", err)
	}

	return &refreshLock{path: path}, nil
}

func isStaleLock(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return time.Since(info.ModTime()) > time.Hour, nil
}

func (l *refreshLock) Release() {
	_ = os.Remove(l.path)
}

func GitCommitSHA(ctx context.Context, repoRoot string) string {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

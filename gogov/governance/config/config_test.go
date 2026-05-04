package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadResolvesRepositoriesAndRuntimePaths(t *testing.T) {
	t.Parallel()

	hostRoot := t.TempDir()
	repoOne := filepath.Join(t.TempDir(), "ai-platform")
	repoTwo := filepath.Join(t.TempDir(), "duck-demo")

	mustWriteFile(t, filepath.Join(repoOne, "go.mod"), "module example.com/one\n\ngo 1.23.0\n")
	mustWriteFile(t, filepath.Join(repoTwo, "go.mod"), "module example.com/two\n\ngo 1.23.0\n")
	mustWriteFile(t, filepath.Join(hostRoot, DefaultConfigName), "bind_address: 127.0.0.1:8787\nrepositories:\n  - "+repoOne+"\n  - "+repoTwo+"\n")

	cfg, err := Load(hostRoot)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Repositories) != 2 {
		t.Fatalf("len(cfg.Repositories) = %d", len(cfg.Repositories))
	}
	if cfg.Repositories[0].ID != "ai-platform" {
		t.Fatalf("repo 0 id = %q", cfg.Repositories[0].ID)
	}
	if !strings.Contains(cfg.Repositories[0].DatabasePath, filepath.Join(".governance", "repos", "ai-platform", "governance.db")) {
		t.Fatalf("unexpected database path: %s", cfg.Repositories[0].DatabasePath)
	}
}

func TestLoadFailsWhenRepositoryMissingGoMod(t *testing.T) {
	t.Parallel()

	hostRoot := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "broken")
	if err := os.MkdirAll(repoRoot, 0750); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	mustWriteFile(t, filepath.Join(hostRoot, DefaultConfigName), "repositories:\n  - "+repoRoot+"\n")

	if _, err := Load(hostRoot); err == nil {
		t.Fatalf("Load() error = nil")
	}
}

func TestLoadGeneratesStableUniqueIDsForBasenameCollisions(t *testing.T) {
	t.Parallel()

	hostRoot := t.TempDir()
	repoOne := filepath.Join(t.TempDir(), "workspace-a", "service")
	repoTwo := filepath.Join(t.TempDir(), "workspace-b", "service")
	mustWriteFile(t, filepath.Join(repoOne, "go.mod"), "module example.com/a\n\ngo 1.23.0\n")
	mustWriteFile(t, filepath.Join(repoTwo, "go.mod"), "module example.com/b\n\ngo 1.23.0\n")
	mustWriteFile(t, filepath.Join(hostRoot, DefaultConfigName), "repositories:\n  - "+repoOne+"\n  - "+repoTwo+"\n")

	cfg, err := Load(hostRoot)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Repositories[0].ID == cfg.Repositories[1].ID {
		t.Fatalf("expected unique repository ids, got %q", cfg.Repositories[0].ID)
	}
	if !strings.HasPrefix(cfg.Repositories[0].ID, "service-") {
		t.Fatalf("unexpected repo id: %q", cfg.Repositories[0].ID)
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

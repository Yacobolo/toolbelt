package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Yacobolo/toolbelt/gogov/governance/model"
)

func TestReplaceSnapshotKeepsOnlyLatestStateAndRetainsRuns(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".governance", "governance.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	started := time.Now().UTC()
	run1 := model.Run{ID: "run-1", Status: model.RunStatusSucceeded, CoverageStatus: model.CoverageStatusAvailable, RepoRoot: "/tmp/repo", StartedAt: started}
	run2 := model.Run{ID: "run-2", Status: model.RunStatusPartial, CoverageStatus: model.CoverageStatusFailed, RepoRoot: "/tmp/repo", StartedAt: started.Add(time.Second)}
	if err := st.CreateRun(ctx, run1); err != nil {
		t.Fatalf("CreateRun(run1) error = %v", err)
	}
	if err := st.CreateRun(ctx, run2); err != nil {
		t.Fatalf("CreateRun(run2) error = %v", err)
	}

	snapshot1 := model.Snapshot{
		Meta: model.SnapshotMeta{
			RunID:           "run-1",
			RepoRoot:        "/tmp/repo",
			ModulePath:      "example.com/one",
			RefreshedAt:     started,
			CoverageStatus:  model.CoverageStatusAvailable,
			FilesCount:      1,
			PackagesCount:   1,
			ViolationsCount: 0,
		},
		Packages: []model.Package{{Path: "example.com/one", Name: "one", FileCount: 1}},
		Files:    []model.File{{Path: "one.go", PackagePath: "example.com/one", PackageName: "one", LOC: 10, NonEmptyLOC: 8}},
	}
	if err := st.ReplaceSnapshot(ctx, snapshot1); err != nil {
		t.Fatalf("ReplaceSnapshot(snapshot1) error = %v", err)
	}

	snapshot2 := model.Snapshot{
		Meta: model.SnapshotMeta{
			RunID:           "run-2",
			RepoRoot:        "/tmp/repo",
			ModulePath:      "example.com/two",
			RefreshedAt:     started.Add(2 * time.Second),
			CoverageStatus:  model.CoverageStatusFailed,
			FilesCount:      1,
			PackagesCount:   1,
			ViolationsCount: 1,
		},
		Packages:   []model.Package{{Path: "example.com/two", Name: "two", FileCount: 1, ViolationCount: 1}},
		Files:      []model.File{{Path: "two.go", PackagePath: "example.com/two", PackageName: "two", LOC: 12, NonEmptyLOC: 11, ViolationCount: 1, IsGenerated: true, IsIgnored: true}},
		Violations: []model.Violation{{ScopeType: "package", ScopeKey: "example.com/two", RuleName: "rule"}},
	}
	if err := st.ReplaceSnapshot(ctx, snapshot2); err != nil {
		t.Fatalf("ReplaceSnapshot(snapshot2) error = %v", err)
	}

	meta, err := st.GetSnapshotMeta(ctx)
	if err != nil {
		t.Fatalf("GetSnapshotMeta() error = %v", err)
	}
	if meta.RunID != "run-2" || meta.ModulePath != "example.com/two" {
		t.Fatalf("unexpected latest meta: %+v", meta)
	}

	files, err := st.ListFiles(ctx)
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "two.go" {
		t.Fatalf("unexpected latest files: %+v", files)
	}
	if !files[0].IsGenerated || !files[0].IsIgnored {
		t.Fatalf("expected stored tags to round-trip, got %+v", files[0])
	}

	runs, err := st.ListRuns(ctx, 10)
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
}

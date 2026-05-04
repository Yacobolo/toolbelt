package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yacobolo/toolbelt/gogov/governance/model"

	_ "modernc.org/sqlite" // Register SQLite driver.
)

const schema = `
CREATE TABLE IF NOT EXISTS scan_runs (
	id TEXT PRIMARY KEY,
	status TEXT NOT NULL,
	coverage_status TEXT NOT NULL,
	error_text TEXT NOT NULL DEFAULT '',
	repo_root TEXT NOT NULL,
	module_path TEXT NOT NULL DEFAULT '',
	commit_sha TEXT NOT NULL DEFAULT '',
	started_at TEXT NOT NULL,
	completed_at TEXT,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	files_count INTEGER NOT NULL DEFAULT 0,
	packages_count INTEGER NOT NULL DEFAULT 0,
	package_edges_count INTEGER NOT NULL DEFAULT 0,
	file_edges_count INTEGER NOT NULL DEFAULT 0,
	violations_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS current_snapshot (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	run_id TEXT NOT NULL,
	repo_root TEXT NOT NULL,
	module_path TEXT NOT NULL,
	commit_sha TEXT NOT NULL,
	refreshed_at TEXT NOT NULL,
	coverage_status TEXT NOT NULL,
	files_count INTEGER NOT NULL DEFAULT 0,
	packages_count INTEGER NOT NULL DEFAULT 0,
	package_edges_count INTEGER NOT NULL DEFAULT 0,
	file_edges_count INTEGER NOT NULL DEFAULT 0,
	violations_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS packages (
	path TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	dir TEXT NOT NULL,
	file_count INTEGER NOT NULL,
	test_file_count INTEGER NOT NULL,
	loc INTEGER NOT NULL,
	non_empty_loc INTEGER NOT NULL,
	imports_count INTEGER NOT NULL,
	imported_by_count INTEGER NOT NULL,
	violation_count INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
	path TEXT PRIMARY KEY,
	dir TEXT NOT NULL,
	package_path TEXT NOT NULL,
	package_name TEXT NOT NULL,
	loc INTEGER NOT NULL,
	non_empty_loc INTEGER NOT NULL,
	function_count INTEGER NOT NULL,
	exported_symbol_count INTEGER NOT NULL,
	is_test INTEGER NOT NULL,
	fan_in INTEGER NOT NULL,
	fan_out INTEGER NOT NULL,
	violation_count INTEGER NOT NULL,
	covered_statements INTEGER NOT NULL,
	total_statements INTEGER NOT NULL,
	coverage_pct REAL
);

CREATE TABLE IF NOT EXISTS file_symbols (
	file_path TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL,
	line INTEGER NOT NULL,
	exported INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS package_edges (
	from_path TEXT NOT NULL,
	to_path TEXT NOT NULL,
	weight INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS file_edges (
	from_path TEXT NOT NULL,
	to_path TEXT NOT NULL,
	weight INTEGER NOT NULL,
	kind TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS coverage_files (
	path TEXT PRIMARY KEY,
	covered_statements INTEGER NOT NULL,
	total_statements INTEGER NOT NULL,
	coverage_pct REAL NOT NULL
);

CREATE TABLE IF NOT EXISTS violations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	scope_type TEXT NOT NULL,
	scope_key TEXT NOT NULL,
	rule_name TEXT NOT NULL,
	message TEXT NOT NULL,
	from_package TEXT NOT NULL,
	to_package TEXT NOT NULL,
	severity TEXT NOT NULL
);
`

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, fmt.Errorf("create governance directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open governance database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx := context.Background()
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("set pragma: %w", err)
		}
	}

	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize governance schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateRun(ctx context.Context, run model.Run) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scan_runs (
			id, status, coverage_status, error_text, repo_root, module_path, commit_sha,
			started_at, completed_at, duration_ms, files_count, packages_count,
			package_edges_count, file_edges_count, violations_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID,
		run.Status,
		run.CoverageStatus,
		run.ErrorText,
		run.RepoRoot,
		run.ModulePath,
		run.CommitSHA,
		run.StartedAt.UTC().Format(time.RFC3339Nano),
		nullableTime(run.CompletedAt),
		run.DurationMS,
		run.FilesCount,
		run.PackagesCount,
		run.PackageEdgesCount,
		run.FileEdgesCount,
		run.ViolationsCount,
	)
	if err != nil {
		return fmt.Errorf("insert run %s: %w", run.ID, err)
	}
	return nil
}

func (s *Store) UpdateRun(ctx context.Context, run model.Run) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE scan_runs
		SET status = ?,
			coverage_status = ?,
			error_text = ?,
			repo_root = ?,
			module_path = ?,
			commit_sha = ?,
			started_at = ?,
			completed_at = ?,
			duration_ms = ?,
			files_count = ?,
			packages_count = ?,
			package_edges_count = ?,
			file_edges_count = ?,
			violations_count = ?
		WHERE id = ?`,
		run.Status,
		run.CoverageStatus,
		run.ErrorText,
		run.RepoRoot,
		run.ModulePath,
		run.CommitSHA,
		run.StartedAt.UTC().Format(time.RFC3339Nano),
		nullableTime(run.CompletedAt),
		run.DurationMS,
		run.FilesCount,
		run.PackagesCount,
		run.PackageEdgesCount,
		run.FileEdgesCount,
		run.ViolationsCount,
		run.ID,
	)
	if err != nil {
		return fmt.Errorf("update run %s: %w", run.ID, err)
	}
	return nil
}

func (s *Store) ReplaceSnapshot(ctx context.Context, snapshot model.Snapshot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin snapshot transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, stmt := range []string{
		"DELETE FROM current_snapshot",
		"DELETE FROM packages",
		"DELETE FROM files",
		"DELETE FROM file_symbols",
		"DELETE FROM package_edges",
		"DELETE FROM file_edges",
		"DELETE FROM coverage_files",
		"DELETE FROM violations",
	} {
		if _, err = tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("reset snapshot tables: %w", err)
		}
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO current_snapshot (
			id, run_id, repo_root, module_path, commit_sha, refreshed_at, coverage_status,
			files_count, packages_count, package_edges_count, file_edges_count, violations_count
		) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.Meta.RunID,
		snapshot.Meta.RepoRoot,
		snapshot.Meta.ModulePath,
		snapshot.Meta.CommitSHA,
		snapshot.Meta.RefreshedAt.UTC().Format(time.RFC3339Nano),
		snapshot.Meta.CoverageStatus,
		snapshot.Meta.FilesCount,
		snapshot.Meta.PackagesCount,
		snapshot.Meta.PackageEdgesCount,
		snapshot.Meta.FileEdgesCount,
		snapshot.Meta.ViolationsCount,
	); err != nil {
		return fmt.Errorf("insert current snapshot: %w", err)
	}

	for _, item := range snapshot.Packages {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO packages (
				path, name, dir, file_count, test_file_count, loc, non_empty_loc,
				imports_count, imported_by_count, violation_count
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.Path,
			item.Name,
			item.Dir,
			item.FileCount,
			item.TestFileCount,
			item.LOC,
			item.NonEmptyLOC,
			item.ImportsCount,
			item.ImportedByCount,
			item.ViolationCount,
		); err != nil {
			return fmt.Errorf("insert package %s: %w", item.Path, err)
		}
	}

	for _, item := range snapshot.Files {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO files (
				path, dir, package_path, package_name, loc, non_empty_loc, function_count,
				exported_symbol_count, is_test, fan_in, fan_out, violation_count,
				covered_statements, total_statements, coverage_pct
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.Path,
			item.Dir,
			item.PackagePath,
			item.PackageName,
			item.LOC,
			item.NonEmptyLOC,
			item.FunctionCount,
			item.ExportedSymbolCount,
			boolToInt(item.IsTest),
			item.FanIn,
			item.FanOut,
			item.ViolationCount,
			item.CoveredStatements,
			item.TotalStatements,
			item.CoveragePct,
		); err != nil {
			return fmt.Errorf("insert file %s: %w", item.Path, err)
		}
	}

	for _, item := range snapshot.Symbols {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO file_symbols (file_path, name, kind, line, exported)
			VALUES (?, ?, ?, ?, ?)`,
			item.FilePath,
			item.Name,
			item.Kind,
			item.Line,
			boolToInt(item.Exported),
		); err != nil {
			return fmt.Errorf("insert symbol %s:%s: %w", item.FilePath, item.Name, err)
		}
	}

	for _, item := range snapshot.PackageEdges {
		if _, err = tx.ExecContext(ctx, `INSERT INTO package_edges (from_path, to_path, weight) VALUES (?, ?, ?)`,
			item.FromPath, item.ToPath, item.Weight); err != nil {
			return fmt.Errorf("insert package edge %s -> %s: %w", item.FromPath, item.ToPath, err)
		}
	}

	for _, item := range snapshot.FileEdges {
		if _, err = tx.ExecContext(ctx, `INSERT INTO file_edges (from_path, to_path, weight, kind) VALUES (?, ?, ?, ?)`,
			item.FromPath, item.ToPath, item.Weight, item.Kind); err != nil {
			return fmt.Errorf("insert file edge %s -> %s: %w", item.FromPath, item.ToPath, err)
		}
	}

	for _, item := range snapshot.Coverage {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO coverage_files (path, covered_statements, total_statements, coverage_pct)
			VALUES (?, ?, ?, ?)`,
			item.Path,
			item.CoveredStatements,
			item.TotalStatements,
			item.CoveragePct,
		); err != nil {
			return fmt.Errorf("insert coverage row %s: %w", item.Path, err)
		}
	}

	for _, item := range snapshot.Violations {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO violations (
				scope_type, scope_key, rule_name, message, from_package, to_package, severity
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			item.ScopeType,
			item.ScopeKey,
			item.RuleName,
			item.Message,
			item.FromPackage,
			item.ToPackage,
			item.Severity,
		); err != nil {
			return fmt.Errorf("insert violation %s: %w", item.RuleName, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit snapshot transaction: %w", err)
	}
	return nil
}

func (s *Store) GetSnapshotMeta(ctx context.Context) (model.SnapshotMeta, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT run_id, repo_root, module_path, commit_sha, refreshed_at, coverage_status,
		       files_count, packages_count, package_edges_count, file_edges_count, violations_count
		FROM current_snapshot
		WHERE id = 1`)

	var meta model.SnapshotMeta
	var refreshed string
	if err := row.Scan(
		&meta.RunID,
		&meta.RepoRoot,
		&meta.ModulePath,
		&meta.CommitSHA,
		&refreshed,
		&meta.CoverageStatus,
		&meta.FilesCount,
		&meta.PackagesCount,
		&meta.PackageEdgesCount,
		&meta.FileEdgesCount,
		&meta.ViolationsCount,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.SnapshotMeta{}, sql.ErrNoRows
		}
		return model.SnapshotMeta{}, fmt.Errorf("query snapshot meta: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, refreshed)
	if err != nil {
		return model.SnapshotMeta{}, fmt.Errorf("parse refreshed_at: %w", err)
	}
	meta.RefreshedAt = parsed
	return meta, nil
}

func (s *Store) ListRuns(ctx context.Context, limit int) ([]model.Run, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, status, coverage_status, error_text, repo_root, module_path, commit_sha,
		       started_at, completed_at, duration_ms, files_count, packages_count,
		       package_edges_count, file_edges_count, violations_count
		FROM scan_runs
		ORDER BY started_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	var runs []model.Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runs: %w", err)
	}
	return runs, nil
}

func (s *Store) ListFiles(ctx context.Context) ([]model.File, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, dir, package_path, package_name, loc, non_empty_loc, function_count,
		       exported_symbol_count, is_test, fan_in, fan_out, violation_count,
		       covered_statements, total_statements, coverage_pct
		FROM files`)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	defer rows.Close()

	var files []model.File
	for rows.Next() {
		item, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate files: %w", err)
	}
	return files, nil
}

func (s *Store) GetFile(ctx context.Context, path string) (model.File, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT path, dir, package_path, package_name, loc, non_empty_loc, function_count,
		       exported_symbol_count, is_test, fan_in, fan_out, violation_count,
		       covered_statements, total_statements, coverage_pct
		FROM files WHERE path = ?`, path)
	item, err := scanFile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.File{}, sql.ErrNoRows
		}
		return model.File{}, fmt.Errorf("get file %s: %w", path, err)
	}
	return item, nil
}

func (s *Store) ListPackages(ctx context.Context) ([]model.Package, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, name, dir, file_count, test_file_count, loc, non_empty_loc,
		       imports_count, imported_by_count, violation_count
		FROM packages ORDER BY path ASC`)
	if err != nil {
		return nil, fmt.Errorf("list packages: %w", err)
	}
	defer rows.Close()

	var items []model.Package
	for rows.Next() {
		var item model.Package
		if err := rows.Scan(
			&item.Path,
			&item.Name,
			&item.Dir,
			&item.FileCount,
			&item.TestFileCount,
			&item.LOC,
			&item.NonEmptyLOC,
			&item.ImportsCount,
			&item.ImportedByCount,
			&item.ViolationCount,
		); err != nil {
			return nil, fmt.Errorf("scan package: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate packages: %w", err)
	}
	return items, nil
}

func (s *Store) GetPackage(ctx context.Context, path string) (model.Package, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT path, name, dir, file_count, test_file_count, loc, non_empty_loc,
		       imports_count, imported_by_count, violation_count
		FROM packages WHERE path = ?`, path)
	var item model.Package
	if err := row.Scan(
		&item.Path,
		&item.Name,
		&item.Dir,
		&item.FileCount,
		&item.TestFileCount,
		&item.LOC,
		&item.NonEmptyLOC,
		&item.ImportsCount,
		&item.ImportedByCount,
		&item.ViolationCount,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Package{}, sql.ErrNoRows
		}
		return model.Package{}, fmt.Errorf("get package %s: %w", path, err)
	}
	return item, nil
}

func (s *Store) ListPackageEdges(ctx context.Context) ([]model.PackageEdge, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT from_path, to_path, weight FROM package_edges ORDER BY from_path, to_path`)
	if err != nil {
		return nil, fmt.Errorf("list package edges: %w", err)
	}
	defer rows.Close()

	var items []model.PackageEdge
	for rows.Next() {
		var item model.PackageEdge
		if err := rows.Scan(&item.FromPath, &item.ToPath, &item.Weight); err != nil {
			return nil, fmt.Errorf("scan package edge: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate package edges: %w", err)
	}
	return items, nil
}

func (s *Store) ListFileEdgesForFile(ctx context.Context, path string) ([]model.FileEdge, []model.FileEdge, error) {
	outbound, err := s.listFileEdges(ctx, `SELECT from_path, to_path, weight, kind FROM file_edges WHERE from_path = ? ORDER BY weight DESC, to_path ASC`, path)
	if err != nil {
		return nil, nil, err
	}
	inbound, err := s.listFileEdges(ctx, `SELECT from_path, to_path, weight, kind FROM file_edges WHERE to_path = ? ORDER BY weight DESC, from_path ASC`, path)
	if err != nil {
		return nil, nil, err
	}
	return inbound, outbound, nil
}

func (s *Store) listFileEdges(ctx context.Context, query string, path string) ([]model.FileEdge, error) {
	rows, err := s.db.QueryContext(ctx, query, path)
	if err != nil {
		return nil, fmt.Errorf("list file edges: %w", err)
	}
	defer rows.Close()

	var items []model.FileEdge
	for rows.Next() {
		var item model.FileEdge
		if err := rows.Scan(&item.FromPath, &item.ToPath, &item.Weight, &item.Kind); err != nil {
			return nil, fmt.Errorf("scan file edge: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate file edges: %w", err)
	}
	return items, nil
}

func (s *Store) ListSymbolsByFile(ctx context.Context, path string) ([]model.Symbol, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT file_path, name, kind, line, exported
		FROM file_symbols
		WHERE file_path = ?
		ORDER BY line ASC, name ASC`, path)
	if err != nil {
		return nil, fmt.Errorf("list symbols: %w", err)
	}
	defer rows.Close()

	var items []model.Symbol
	for rows.Next() {
		var item model.Symbol
		var exported int
		if err := rows.Scan(&item.FilePath, &item.Name, &item.Kind, &item.Line, &exported); err != nil {
			return nil, fmt.Errorf("scan symbol: %w", err)
		}
		item.Exported = exported == 1
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate symbols: %w", err)
	}
	return items, nil
}

func (s *Store) ListViolations(ctx context.Context) ([]model.Violation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, scope_type, scope_key, rule_name, message, from_package, to_package, severity
		FROM violations
		ORDER BY severity DESC, rule_name ASC, scope_key ASC`)
	if err != nil {
		return nil, fmt.Errorf("list violations: %w", err)
	}
	defer rows.Close()

	var items []model.Violation
	for rows.Next() {
		var item model.Violation
		if err := rows.Scan(
			&item.ID,
			&item.ScopeType,
			&item.ScopeKey,
			&item.RuleName,
			&item.Message,
			&item.FromPackage,
			&item.ToPackage,
			&item.Severity,
		); err != nil {
			return nil, fmt.Errorf("scan violation: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate violations: %w", err)
	}
	return items, nil
}

func (s *Store) ListViolationsForScope(ctx context.Context, scopeType string, scopeKey string) ([]model.Violation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, scope_type, scope_key, rule_name, message, from_package, to_package, severity
		FROM violations
		WHERE scope_type = ? AND scope_key = ?
		ORDER BY severity DESC, rule_name ASC`, scopeType, scopeKey)
	if err != nil {
		return nil, fmt.Errorf("list scope violations: %w", err)
	}
	defer rows.Close()

	var items []model.Violation
	for rows.Next() {
		var item model.Violation
		if err := rows.Scan(
			&item.ID,
			&item.ScopeType,
			&item.ScopeKey,
			&item.RuleName,
			&item.Message,
			&item.FromPackage,
			&item.ToPackage,
			&item.Severity,
		); err != nil {
			return nil, fmt.Errorf("scan scope violation: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scope violations: %w", err)
	}
	return items, nil
}

func (s *Store) ListPackageFiles(ctx context.Context, packagePath string) ([]model.File, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, dir, package_path, package_name, loc, non_empty_loc, function_count,
		       exported_symbol_count, is_test, fan_in, fan_out, violation_count,
		       covered_statements, total_statements, coverage_pct
		FROM files
		WHERE package_path = ?
		ORDER BY is_test ASC, path ASC`, packagePath)
	if err != nil {
		return nil, fmt.Errorf("list package files: %w", err)
	}
	defer rows.Close()

	var files []model.File
	for rows.Next() {
		item, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate package files: %w", err)
	}
	return files, nil
}

func (s *Store) ListPackageEdgeNeighborhood(ctx context.Context, packagePath string) ([]model.PackageEdge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT from_path, to_path, weight
		FROM package_edges
		WHERE from_path = ? OR to_path = ?
		ORDER BY from_path ASC, to_path ASC`, packagePath, packagePath)
	if err != nil {
		return nil, fmt.Errorf("list package edge neighborhood: %w", err)
	}
	defer rows.Close()

	var items []model.PackageEdge
	for rows.Next() {
		var item model.PackageEdge
		if err := rows.Scan(&item.FromPath, &item.ToPath, &item.Weight); err != nil {
			return nil, fmt.Errorf("scan package edge neighborhood: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate package edge neighborhood: %w", err)
	}
	return items, nil
}

func (s *Store) ListRelatedTestFiles(ctx context.Context, dir string, packagePath string) ([]model.File, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, dir, package_path, package_name, loc, non_empty_loc, function_count,
		       exported_symbol_count, is_test, fan_in, fan_out, violation_count,
		       covered_statements, total_statements, coverage_pct
		FROM files
		WHERE dir = ? AND package_path = ? AND is_test = 1
		ORDER BY path ASC`, dir, packagePath)
	if err != nil {
		return nil, fmt.Errorf("list related test files: %w", err)
	}
	defer rows.Close()

	var files []model.File
	for rows.Next() {
		item, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate related test files: %w", err)
	}
	return files, nil
}

func scanRun(scanner interface{ Scan(dest ...any) error }) (model.Run, error) {
	var run model.Run
	var started string
	var completed sql.NullString
	if err := scanner.Scan(
		&run.ID,
		&run.Status,
		&run.CoverageStatus,
		&run.ErrorText,
		&run.RepoRoot,
		&run.ModulePath,
		&run.CommitSHA,
		&started,
		&completed,
		&run.DurationMS,
		&run.FilesCount,
		&run.PackagesCount,
		&run.PackageEdgesCount,
		&run.FileEdgesCount,
		&run.ViolationsCount,
	); err != nil {
		return model.Run{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, started)
	if err != nil {
		return model.Run{}, fmt.Errorf("parse run started_at: %w", err)
	}
	run.StartedAt = parsed
	if completed.Valid && strings.TrimSpace(completed.String) != "" {
		parsedCompleted, err := time.Parse(time.RFC3339Nano, completed.String)
		if err != nil {
			return model.Run{}, fmt.Errorf("parse run completed_at: %w", err)
		}
		run.CompletedAt = &parsedCompleted
	}
	return run, nil
}

func scanFile(scanner interface{ Scan(dest ...any) error }) (model.File, error) {
	var item model.File
	var isTest int
	var coverage sql.NullFloat64
	if err := scanner.Scan(
		&item.Path,
		&item.Dir,
		&item.PackagePath,
		&item.PackageName,
		&item.LOC,
		&item.NonEmptyLOC,
		&item.FunctionCount,
		&item.ExportedSymbolCount,
		&isTest,
		&item.FanIn,
		&item.FanOut,
		&item.ViolationCount,
		&item.CoveredStatements,
		&item.TotalStatements,
		&coverage,
	); err != nil {
		return model.File{}, err
	}
	item.IsTest = isTest == 1
	if coverage.Valid {
		item.CoveragePct = &coverage.Float64
	}
	return item, nil
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

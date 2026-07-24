package upgrade

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunInstallsNewerRelease(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{
		candidate: candidate{version: "1.3.0"},
		found:     true,
	}
	manager := testManager(backend)

	result, err := manager.Run(context.Background(), "v1.2.0", false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.UpdateAvailable || !result.Updated {
		t.Fatalf("Run() result = %+v, want available and updated", result)
	}
	if result.CurrentVersion != "v1.2.0" || result.LatestVersion != "v1.3.0" {
		t.Fatalf("Run() versions = %+v", result)
	}
	if backend.installedPath != "/resolved/sourcebook" {
		t.Fatalf("installed path = %q, want resolved executable", backend.installedPath)
	}
}

func TestRunCheckOnlyDoesNotInstall(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{
		candidate: candidate{version: "2.0.0"},
		found:     true,
	}
	manager := testManager(backend)

	result, err := manager.Run(context.Background(), "1.9.0", true)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.UpdateAvailable || result.Updated {
		t.Fatalf("Run() result = %+v, want available but not updated", result)
	}
	if backend.installedPath != "" {
		t.Fatalf("check-only installed to %q", backend.installedPath)
	}
}

func TestRunReportsCurrentReleaseAsUpToDate(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{
		candidate: candidate{version: "1.2.0"},
		found:     true,
	}
	manager := testManager(backend)

	result, err := manager.Run(context.Background(), "v1.2.0", false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.UpdateAvailable || result.Updated {
		t.Fatalf("Run() result = %+v, want up to date", result)
	}
	if backend.installedPath != "" {
		t.Fatalf("up-to-date run installed to %q", backend.installedPath)
	}
}

func TestRunDoesNotDowngradeNewerLocalVersion(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{
		candidate: candidate{version: "1.2.0"},
		found:     true,
	}
	manager := testManager(backend)

	result, err := manager.Run(context.Background(), "1.3.0", false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.UpdateAvailable || result.Updated {
		t.Fatalf("Run() result = %+v, want no downgrade", result)
	}
}

func TestRunRejectsDevelopmentBuild(t *testing.T) {
	t.Parallel()

	manager := testManager(&fakeBackend{})
	_, err := manager.Run(context.Background(), "dev", false)
	if err == nil || !strings.Contains(err.Error(), "development build") {
		t.Fatalf("Run() error = %v, want development build error", err)
	}
}

func TestRunFailsWhenNoCompatibleReleaseExists(t *testing.T) {
	t.Parallel()

	manager := testManager(&fakeBackend{})
	_, err := manager.Run(context.Background(), "1.2.0", false)
	if err == nil || !strings.Contains(err.Error(), "compatible Sourcebook release") {
		t.Fatalf("Run() error = %v, want compatible-release error", err)
	}
}

func TestRunPreservesDetectionAndInstallErrors(t *testing.T) {
	t.Parallel()

	t.Run("detection", func(t *testing.T) {
		expected := errors.New("release API unavailable")
		manager := testManager(&fakeBackend{latestErr: expected})
		_, err := manager.Run(context.Background(), "1.2.0", false)
		if !errors.Is(err, expected) {
			t.Fatalf("Run() error = %v, want %v", err, expected)
		}
	})

	t.Run("installation", func(t *testing.T) {
		expected := errors.New("permission denied")
		manager := testManager(&fakeBackend{
			candidate:  candidate{version: "1.3.0"},
			found:      true,
			installErr: expected,
		})
		_, err := manager.Run(context.Background(), "1.2.0", false)
		if !errors.Is(err, expected) {
			t.Fatalf("Run() error = %v, want %v", err, expected)
		}
	})
}

func TestAssetFilterMatchesOnlySourcebookPlatformArchives(t *testing.T) {
	t.Parallel()

	filter := assetFilter("darwin", "arm64")
	for _, name := range []string{
		"sourcebook_1.2.3_darwin_arm64.tar.gz",
		"sourcebook_2.0.0-rc.1_darwin_arm64.tar.gz",
	} {
		if !filter.MatchString(name) {
			t.Errorf("filter does not match %q", name)
		}
	}
	for _, name := range []string{
		"apigen_9.0.0_darwin_arm64.tar.gz",
		"sourcebook_1.2.3_linux_arm64.tar.gz",
		"sourcebook_1.2.3_darwin_amd64.tar.gz",
		"sourcebook_1.2.3_darwin_arm64.zip",
		"https://example.com/sourcebook_1.2.3_darwin_arm64.tar.gz",
	} {
		if filter.MatchString(name) {
			t.Errorf("filter unexpectedly matches %q", name)
		}
	}
}

func TestAssetFilterUsesZipOnWindows(t *testing.T) {
	t.Parallel()

	filter := assetFilter("windows", "amd64")
	if !filter.MatchString("sourcebook_1.2.3_windows_amd64.zip") {
		t.Fatal("Windows filter does not match the expected zip archive")
	}
	if filter.MatchString("sourcebook_1.2.3_windows_amd64.tar.gz") {
		t.Fatal("Windows filter matches a tar archive")
	}
}

func TestNoticeChecksAndPrintsAtMostOnceEvery24Hours(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{
		candidate: candidate{version: "1.3.0"},
		found:     true,
	}
	manager := testManager(backend)
	manager.noticeCachePath = filepath.Join(t.TempDir(), "updates", "notice.json")
	manager.now = func() time.Time {
		return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	}

	notice, err := manager.Notice(context.Background(), "v1.2.0")
	if err != nil {
		t.Fatalf("Notice() error = %v", err)
	}
	if !strings.Contains(notice, "v1.3.0") || !strings.Contains(notice, "sourcebook upgrade") {
		t.Fatalf("Notice() = %q", notice)
	}
	notice, err = manager.Notice(context.Background(), "v1.2.0")
	if err != nil {
		t.Fatalf("cached Notice() error = %v", err)
	}
	if notice != "" {
		t.Fatalf("cached Notice() = %q, want empty", notice)
	}
	if backend.latestCalls != 1 {
		t.Fatalf("latest calls = %d, want 1", backend.latestCalls)
	}
	if _, err := os.Stat(manager.noticeCachePath); err != nil {
		t.Fatalf("notice cache stat error = %v", err)
	}
}

func TestNoticeRefreshesExpiredCache(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	backend := &fakeBackend{
		candidate: candidate{version: "1.4.0"},
		found:     true,
	}
	manager := testManager(backend)
	manager.noticeCachePath = filepath.Join(t.TempDir(), "notice.json")
	manager.now = func() time.Time { return now }
	if err := writeNoticeCache(manager.noticeCachePath, noticeCache{
		CheckedAt:     now.Add(-25 * time.Hour),
		LatestVersion: "1.3.0",
	}); err != nil {
		t.Fatal(err)
	}

	notice, err := manager.Notice(context.Background(), "1.2.0")
	if err != nil {
		t.Fatalf("Notice() error = %v", err)
	}
	if !strings.Contains(notice, "v1.4.0") {
		t.Fatalf("Notice() = %q", notice)
	}
	if backend.latestCalls != 1 {
		t.Fatalf("latest calls = %d, want 1", backend.latestCalls)
	}
}

func TestNoticeIsEmptyForCurrentAndDevelopmentBuilds(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{
		candidate: candidate{version: "1.2.0"},
		found:     true,
	}
	manager := testManager(backend)
	manager.noticeCachePath = filepath.Join(t.TempDir(), "notice.json")
	manager.now = time.Now

	notice, err := manager.Notice(context.Background(), "v1.2.0")
	if err != nil {
		t.Fatalf("Notice() error = %v", err)
	}
	if notice != "" {
		t.Fatalf("Notice() = %q, want empty", notice)
	}
	notice, err = manager.Notice(context.Background(), "dev")
	if err != nil {
		t.Fatalf("Notice(dev) error = %v", err)
	}
	if notice != "" {
		t.Fatalf("Notice(dev) = %q, want empty", notice)
	}
}

func testManager(backend backend) *Manager {
	return &Manager{
		backend: backend,
		executablePath: func() (string, error) {
			return "/path/sourcebook", nil
		},
		evalSymlinks: func(string) (string, error) {
			return "/resolved/sourcebook", nil
		},
		now:       time.Now,
		noticeTTL: 24 * time.Hour,
	}
}

type fakeBackend struct {
	candidate     candidate
	found         bool
	latestErr     error
	installErr    error
	installedPath string
	latestCalls   int
}

func (b *fakeBackend) latest(context.Context) (candidate, bool, error) {
	b.latestCalls++
	return b.candidate, b.found, b.latestErr
}

func (b *fakeBackend) install(_ context.Context, _ candidate, executablePath string) error {
	b.installedPath = executablePath
	return b.installErr
}

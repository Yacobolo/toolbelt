package upgrade

import (
	"context"
	"errors"
	"strings"
	"testing"
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

func TestRunWithProgressReportsInstallationPhase(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{
		candidate: candidate{version: "1.3.0"},
		found:     true,
	}
	manager := testManager(backend)
	var events []ProgressEvent

	result, err := manager.RunWithProgress(context.Background(), "v1.2.0", false, func(event ProgressEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("RunWithProgress() error = %v", err)
	}
	if !result.Updated || len(events) != 1 || events[0].Phase != "Downloading, verifying, and installing Sourcebook v1.3.0..." {
		t.Fatalf("result = %+v, events = %#v", result, events)
	}
}

func TestRunWithProgressDoesNotReportCheckOnlyInstallation(t *testing.T) {
	t.Parallel()

	manager := testManager(&fakeBackend{
		candidate: candidate{version: "1.3.0"},
		found:     true,
	})
	var events []ProgressEvent
	if _, err := manager.RunWithProgress(context.Background(), "v1.2.0", true, func(event ProgressEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatalf("RunWithProgress() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("check-only events = %#v, want none", events)
	}
}

func TestRunWithProgressStopsBeforeInstallWhenProgressReportingFails(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{
		candidate: candidate{version: "1.3.0"},
		found:     true,
	}
	manager := testManager(backend)
	expected := errors.New("stdout closed")

	_, err := manager.RunWithProgress(context.Background(), "v1.2.0", false, func(ProgressEvent) error {
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("RunWithProgress() error = %v, want %v", err, expected)
	}
	if backend.installedPath != "" {
		t.Fatalf("installed path = %q, want no install after progress failure", backend.installedPath)
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

func testManager(backend backend) *Manager {
	return &Manager{
		backend: backend,
		executablePath: func() (string, error) {
			return "/path/sourcebook", nil
		},
		evalSymlinks: func(string) (string, error) {
			return "/resolved/sourcebook", nil
		},
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

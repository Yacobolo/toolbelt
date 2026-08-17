package upgrade

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	selfupdate "github.com/creativeprojects/go-selfupdate"
)

const repositorySlug = "Yacobolo/toolbelt"

var errDevelopmentBuild = errors.New("development build cannot self-upgrade; install a released Sourcebook build first")

// Result describes the outcome of an update check or installation.
type Result struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	Updated         bool
}

// ProgressEvent describes a user-visible phase of a self-update.
type ProgressEvent struct {
	Phase string
}

// ProgressReporter receives coarse-grained self-update phases. The updater
// dependency does not expose byte-level download progress, so phases are kept
// stable and useful for both humans and automated callers. Returning an error
// aborts the update before the executable is replaced.
type ProgressReporter func(ProgressEvent) error

type candidate struct {
	version string
	release *selfupdate.Release
}

type backend interface {
	latest(context.Context) (candidate, bool, error)
	install(context.Context, candidate, string) error
}

// Manager checks GitHub releases and safely replaces the current executable.
type Manager struct {
	backend        backend
	executablePath func() (string, error)
	evalSymlinks   func(string) (string, error)
}

// New creates an updater for Sourcebook's GitHub releases.
func New() (*Manager, error) {
	filter := assetFilter(runtime.GOOS, runtime.GOARCH)
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Filters: []string{filter.String()},
		Validator: &selfupdate.ChecksumValidator{
			UniqueFilename: "checksums.txt",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("configure updater: %w", err)
	}
	return &Manager{
		backend: githubBackend{
			updater:    updater,
			repository: selfupdate.ParseSlug(repositorySlug),
		},
		executablePath: os.Executable,
		evalSymlinks:   filepath.EvalSymlinks,
	}, nil
}

// Run checks for the latest compatible release and installs it unless checkOnly
// is true.
func (m *Manager) Run(ctx context.Context, currentVersion string, checkOnly bool) (Result, error) {
	return m.RunWithProgress(ctx, currentVersion, checkOnly, nil)
}

// RunWithProgress checks GitHub releases and installs the latest compatible
// release unless checkOnly is true, reporting the installation phase when a
// release is available.
func (m *Manager) RunWithProgress(ctx context.Context, currentVersion string, checkOnly bool, report ProgressReporter) (Result, error) {
	current, err := parseCurrentVersion(currentVersion)
	if err != nil {
		return Result{}, err
	}
	result := Result{CurrentVersion: displayVersion(current)}

	latest, found, err := m.backend.latest(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("check GitHub releases: %w", err)
	}
	if !found {
		return Result{}, fmt.Errorf("no compatible Sourcebook release found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	latestVersion, err := semver.NewVersion(latest.version)
	if err != nil {
		return Result{}, fmt.Errorf("parse latest release version %q: %w", latest.version, err)
	}
	result.LatestVersion = displayVersion(latestVersion)
	if !latestVersion.GreaterThan(current) {
		return result, nil
	}

	result.UpdateAvailable = true
	if checkOnly {
		return result, nil
	}

	executablePath, err := m.executablePath()
	if err != nil {
		return Result{}, fmt.Errorf("locate Sourcebook executable: %w", err)
	}
	executablePath, err = m.evalSymlinks(executablePath)
	if err != nil {
		return Result{}, fmt.Errorf("resolve Sourcebook executable %q: %w", executablePath, err)
	}
	if report != nil {
		if err := report(ProgressEvent{Phase: fmt.Sprintf("Downloading, verifying, and installing Sourcebook %s...", result.LatestVersion)}); err != nil {
			return Result{}, fmt.Errorf("report upgrade progress: %w", err)
		}
	}
	if err := m.backend.install(ctx, latest, executablePath); err != nil {
		return Result{}, fmt.Errorf("install %s: %w", result.LatestVersion, err)
	}
	result.Updated = true
	return result, nil
}

func parseCurrentVersion(version string) (*semver.Version, error) {
	version = strings.TrimSpace(version)
	if version == "" || version == "dev" || version == "(devel)" {
		return nil, errDevelopmentBuild
	}
	parsed, err := semver.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("installed Sourcebook version %q is not valid semantic versioning: %w", version, err)
	}
	return parsed, nil
}

func displayVersion(version *semver.Version) string {
	return "v" + version.String()
}

func assetFilter(goos, goarch string) *regexp.Regexp {
	extension := `tar\.gz`
	if goos == "windows" {
		extension = `zip`
	}
	return regexp.MustCompile(
		`^sourcebook_[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?_` +
			regexp.QuoteMeta(goos) + `_` + regexp.QuoteMeta(goarch) + `\.` + extension + `$`,
	)
}

type githubBackend struct {
	updater    *selfupdate.Updater
	repository selfupdate.Repository
}

func (b githubBackend) latest(ctx context.Context) (candidate, bool, error) {
	release, found, err := b.updater.DetectLatest(ctx, b.repository)
	if err != nil || !found {
		return candidate{}, found, err
	}
	return candidate{
		version: release.Version(),
		release: release,
	}, true, nil
}

func (b githubBackend) install(ctx context.Context, release candidate, executablePath string) error {
	if release.release == nil {
		return errors.New("release metadata is missing")
	}
	return b.updater.UpdateTo(ctx, release.release, executablePath)
}

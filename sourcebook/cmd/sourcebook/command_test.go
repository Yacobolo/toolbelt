package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/upgrade"
)

func TestRootCommandWithoutArgumentsShowsHelpSuccessfully(t *testing.T) {
	t.Parallel()

	command, stdout, _ := testCommand(t, "1.2.3")
	command.SetArgs(nil)
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	output := stdout.String()
	for _, expected := range []string{"Sourcebook", "sourcebook add", "sourcebook update", "sourcebook upgrade", "sourcebook version"} {
		if !strings.Contains(output, expected) {
			t.Errorf("help output does not contain %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "sourcebook completion") {
		t.Fatalf("help output contains removed completion command:\n%s", output)
	}
	if strings.Contains(output, "missing command") {
		t.Fatalf("help output contains missing-command error:\n%s", output)
	}
}

func TestRootCommandRejectsUnknownCommand(t *testing.T) {
	t.Parallel()

	command, _, _ := testCommand(t, "dev")
	command.SetArgs([]string{"udpate"})
	err := command.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("ExecuteContext() error = %v, want unknown command", err)
	}
}

func TestVersionCommandAndFlag(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"version"}, {"--version"}} {
		command, stdout, _ := testCommand(t, "1.2.3")
		command.SetArgs(args)
		if err := command.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("ExecuteContext(%v) error = %v", args, err)
		}
		if got, want := stdout.String(), "sourcebook 1.2.3\n"; got != want {
			t.Fatalf("ExecuteContext(%v) output = %q, want %q", args, got, want)
		}
	}
}

func TestUpgradeCommandInstallsLatestRelease(t *testing.T) {
	t.Parallel()

	runner := &commandUpgradeRunner{
		result: upgrade.Result{
			CurrentVersion:  "v1.2.0",
			LatestVersion:   "v1.3.0",
			UpdateAvailable: true,
			Updated:         true,
		},
	}
	command, stdout, _ := testCommandWithUpgrade(t, "v1.2.0", runner)
	command.SetArgs([]string{"upgrade"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if runner.currentVersion != "v1.2.0" || runner.checkOnly {
		t.Fatalf("upgrade call = version %q, checkOnly %t", runner.currentVersion, runner.checkOnly)
	}
	if !strings.Contains(stdout.String(), "upgraded from v1.2.0 to v1.3.0") {
		t.Fatalf("upgrade output = %q", stdout.String())
	}
}

func TestUpgradeCheckReportsAvailableReleaseWithoutInstalling(t *testing.T) {
	t.Parallel()

	runner := &commandUpgradeRunner{
		result: upgrade.Result{
			CurrentVersion:  "v1.2.0",
			LatestVersion:   "v1.3.0",
			UpdateAvailable: true,
		},
	}
	command, stdout, _ := testCommandWithUpgrade(t, "v1.2.0", runner)
	command.SetArgs([]string{"upgrade", "--check"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if !runner.checkOnly {
		t.Fatal("upgrade --check did not request check-only mode")
	}
	if !strings.Contains(stdout.String(), "v1.3.0 is available") {
		t.Fatalf("upgrade --check output = %q", stdout.String())
	}
}

func TestUpgradeReportsCurrentVersionAsUpToDate(t *testing.T) {
	t.Parallel()

	runner := &commandUpgradeRunner{
		result: upgrade.Result{
			CurrentVersion: "v1.2.0",
			LatestVersion:  "v1.2.0",
		},
	}
	command, stdout, _ := testCommandWithUpgrade(t, "v1.2.0", runner)
	command.SetArgs([]string{"upgrade"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "already up to date") {
		t.Fatalf("upgrade output = %q", stdout.String())
	}
}

func TestCommandsPrintCachedUpgradeNoticeToStderr(t *testing.T) {
	t.Parallel()

	runner := &commandUpgradeRunner{
		notice: "Sourcebook v1.3.0 is available; run sourcebook upgrade.",
	}
	command, _, stderr := testCommandWithUpgrade(t, "v1.2.0", runner)
	command.SetArgs([]string{"list"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if got := stderr.String(); !strings.Contains(got, runner.notice) {
		t.Fatalf("stderr = %q, want update notice", got)
	}
	if runner.noticeCalls != 1 {
		t.Fatalf("notice calls = %d, want 1", runner.noticeCalls)
	}
}

func TestUpgradeCommandDoesNotPrintASecondUpdateNotice(t *testing.T) {
	t.Parallel()

	runner := &commandUpgradeRunner{
		result: upgrade.Result{
			CurrentVersion: "v1.2.0",
			LatestVersion:  "v1.2.0",
		},
		notice: "unexpected notice",
	}
	command, _, stderr := testCommandWithUpgrade(t, "v1.2.0", runner)
	command.SetArgs([]string{"upgrade"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if runner.noticeCalls != 0 {
		t.Fatalf("notice calls = %d, want 0", runner.noticeCalls)
	}
	if strings.Contains(stderr.String(), runner.notice) {
		t.Fatalf("stderr contains redundant update notice: %q", stderr.String())
	}
}

func TestResolveVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		injected      string
		moduleVersion string
		want          string
	}{
		{name: "linker version wins", injected: "v1.2.3", moduleVersion: "v9.9.9", want: "v1.2.3"},
		{name: "go install module version", injected: "dev", moduleVersion: "v1.2.3", want: "v1.2.3"},
		{name: "local build", injected: "dev", moduleVersion: "(devel)", want: "dev"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveVersion(test.injected, test.moduleVersion); got != test.want {
				t.Fatalf("resolveVersion(%q, %q) = %q, want %q", test.injected, test.moduleVersion, got, test.want)
			}
		})
	}
}

func TestCompletionCommandIsNotAvailable(t *testing.T) {
	t.Parallel()

	command, _, _ := testCommand(t, "dev")
	command.SetArgs([]string{"completion", "zsh"})
	err := command.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("ExecuteContext() error = %v, want unknown command", err)
	}
}

func TestRemoveWithoutNameRequiresInteractiveTerminal(t *testing.T) {
	t.Parallel()

	command, _, _ := testCommand(t, "dev")
	command.SetArgs([]string{"remove"})
	err := command.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "source name is required") {
		t.Fatalf("ExecuteContext() error = %v, want non-interactive source-name error", err)
	}
}

func TestAddPresetNonInteractively(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	app := sourcebook.New(skillDir, commandTestCloner{})
	if err := app.RegisterProvider(sourcebook.ProviderDefinition{
		ID:       "example",
		Provider: commandTestProvider{},
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.RegisterCatalogEntry(sourcebook.CatalogEntry{
		ID:          "example-docs",
		DisplayName: "Example documentation",
		Provider:    "example",
		SourceName:  "example-docs",
		SourceURL:   "https://docs.example.com/",
	}); err != nil {
		t.Fatal(err)
	}
	command := newRootCommand(app, bytes.NewReader(nil), stdout, stderr, "dev")
	command.SetArgs([]string{"add", "--preset", "example-docs"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "references", "example-docs", "index.md")); err != nil {
		t.Fatalf("built-in reference stat error = %v", err)
	}
}

func TestRegisterBuiltinCatalogueIncludesDatastarNetSuiteAndPowerBI(t *testing.T) {
	t.Parallel()

	app := sourcebook.New(filepath.Join(t.TempDir(), "sourcebook"), commandTestCloner{})
	if err := registerBuiltinCatalogue(app); err != nil {
		t.Fatalf("registerBuiltinCatalogue() error = %v", err)
	}

	definitions := app.CatalogEntries()
	var ids []string
	for _, definition := range definitions {
		ids = append(ids, definition.ID)
	}
	if got := strings.Join(ids, ","); got != "azure-docs,datastar-docs,dbt-docs,duckdb-docs,ducklake-docs,netsuite-docs,powerbi-docs" {
		t.Fatalf("catalogue preset IDs = %q", got)
	}
}

func TestAddRejectsPresetTogetherWithRepositoryURL(t *testing.T) {
	t.Parallel()

	command, _, _ := testCommand(t, "dev")
	command.SetArgs([]string{"add", "--preset", "netsuite-docs", "https://example.com/repository.git"})
	err := command.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("ExecuteContext() error = %v, want conflicting input error", err)
	}
}

func TestAddProviderFlagIsNotAvailable(t *testing.T) {
	t.Parallel()

	command, _, _ := testCommand(t, "dev")
	command.SetArgs([]string{"add", "--provider", "netsuite"})
	err := command.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("ExecuteContext() error = %v, want unknown flag", err)
	}
}

func TestUpdateRequiresSelectionWhenNonInteractive(t *testing.T) {
	t.Parallel()

	command, _, _ := testCommand(t, "dev")
	command.SetArgs([]string{"update"})
	err := command.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "source names or --all") {
		t.Fatalf("ExecuteContext() error = %v, want explicit selection error", err)
	}
}

func TestUpdateRejectsAllTogetherWithSourceNames(t *testing.T) {
	t.Parallel()

	command, _, _ := testCommand(t, "dev")
	command.SetArgs([]string{"update", "--all", "alpha"})
	err := command.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("ExecuteContext() error = %v, want conflicting selection error", err)
	}
}

func TestUpdateSelectedSourceNonInteractively(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	cloner := &commandRecordingCloner{}
	app := sourcebook.New(skillDir, cloner)
	for _, repositoryURL := range []string{
		"https://example.com/alpha.git",
		"https://example.com/beta.git",
	} {
		if err := app.Add(context.Background(), repositoryURL); err != nil {
			t.Fatal(err)
		}
	}
	cloner.urls = nil

	command := newRootCommand(app, bytes.NewReader(nil), new(bytes.Buffer), new(bytes.Buffer), "dev")
	command.SetArgs([]string{"update", "beta"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if got, want := strings.Join(cloner.urls, ","), "https://example.com/beta.git"; got != want {
		t.Fatalf("updated URLs = %q, want %q", got, want)
	}
}

func TestUpdateAllNonInteractively(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	cloner := &commandRecordingCloner{}
	app := sourcebook.New(skillDir, cloner)
	if err := app.Add(context.Background(), "https://example.com/alpha.git"); err != nil {
		t.Fatal(err)
	}
	cloner.urls = nil

	command := newRootCommand(app, bytes.NewReader(nil), new(bytes.Buffer), new(bytes.Buffer), "dev")
	command.SetArgs([]string{"update", "--all"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if got, want := strings.Join(cloner.urls, ","), "https://example.com/alpha.git"; got != want {
		t.Fatalf("updated URLs = %q, want %q", got, want)
	}
}

func testCommand(t *testing.T, version string) (command interface {
	SetArgs([]string)
	ExecuteContext(context.Context) error
}, stdout, stderr *bytes.Buffer) {
	t.Helper()
	stdout = new(bytes.Buffer)
	stderr = new(bytes.Buffer)
	app := sourcebook.New(filepath.Join(t.TempDir(), "sourcebook"), commandTestCloner{})
	return newRootCommand(app, bytes.NewReader(nil), stdout, stderr, version), stdout, stderr
}

func testCommandWithUpgrade(t *testing.T, version string, runner upgradeRunner) (command interface {
	SetArgs([]string)
	ExecuteContext(context.Context) error
}, stdout, stderr *bytes.Buffer) {
	t.Helper()
	stdout = new(bytes.Buffer)
	stderr = new(bytes.Buffer)
	app := sourcebook.New(filepath.Join(t.TempDir(), "sourcebook"), commandTestCloner{})
	return newRootCommand(app, bytes.NewReader(nil), stdout, stderr, version, runner), stdout, stderr
}

type commandUpgradeRunner struct {
	result         upgrade.Result
	err            error
	currentVersion string
	checkOnly      bool
	notice         string
	noticeErr      error
	noticeCalls    int
}

func (r *commandUpgradeRunner) Run(_ context.Context, currentVersion string, checkOnly bool) (upgrade.Result, error) {
	r.currentVersion = currentVersion
	r.checkOnly = checkOnly
	return r.result, r.err
}

func (r *commandUpgradeRunner) Notice(context.Context, string) (string, error) {
	r.noticeCalls++
	return r.notice, r.noticeErr
}

type commandTestCloner struct{}

func (commandTestCloner) Clone(_ context.Context, request sourcebook.CloneRequest) error {
	return os.MkdirAll(request.Destination, 0o755)
}

type commandTestProvider struct{}

func (commandTestProvider) Update(_ context.Context, request sourcebook.ProviderRequest, _ sourcebook.ProviderProgressReporter) error {
	if err := os.MkdirAll(request.DestinationDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(request.DestinationDir, "index.md"), []byte("# Example\n"), 0o644)
}

type commandRecordingCloner struct {
	urls []string
}

func (c *commandRecordingCloner) Clone(_ context.Context, request sourcebook.CloneRequest) error {
	c.urls = append(c.urls, request.URL)
	if err := os.MkdirAll(request.Destination, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(request.Destination, "README.md"), []byte(request.URL), 0o644)
}

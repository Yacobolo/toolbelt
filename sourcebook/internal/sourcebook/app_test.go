package sourcebook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDefaultSkillDirUsesCodexHome(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join("users", "example")
	codexHome := filepath.Join("custom", "codex-home")
	if got, want := DefaultSkillDir(homeDir, codexHome), filepath.Join(codexHome, "skills", "sourcebook"); got != want {
		t.Fatalf("DefaultSkillDir() = %q, want %q", got, want)
	}
}

func TestDefaultSkillDirFallsBackToUserCodexDirectory(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join("users", "example")
	if got, want := DefaultSkillDir(homeDir, ""), filepath.Join(homeDir, ".codex", "skills", "sourcebook"); got != want {
		t.Fatalf("DefaultSkillDir() = %q, want %q", got, want)
	}
}

func TestAddCreatesSourcebookSkill(t *testing.T) {
	t.Parallel()
	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	cloner := &fakeCloner{contents: map[string]string{
		"https://example.com/acme/widgets.git": "widgets version one",
	}}
	app := New(skillDir, cloner)
	updatedAt := time.Date(2026, time.July, 22, 8, 30, 0, 0, time.UTC)
	app.now = func() time.Time { return updatedAt }

	if err := app.Add(context.Background(), "https://example.com/acme/widgets.git"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	assertFileContents(t, filepath.Join(skillDir, "references", "widgets", "README.md"), "widgets version one")
	wantManifest := "{\n  \"version\": 2,\n  \"sources\": [\n    {\n      \"name\": \"widgets\",\n      \"provider\": \"git\",\n      \"url\": \"https://example.com/acme/widgets.git\",\n      \"size_bytes\": 19,\n      \"updated_at\": \"2026-07-22T08:30:00Z\"\n    }\n  ]\n}\n"
	assertFileContents(t, filepath.Join(skillDir, "sources.json"), wantManifest)
	wantSkill := "---\nname: sourcebook\ndescription: Source code and documentation for widgets. Use when a task involves widgets or related technologies.\n---\n\n# Sourcebook\n\n## Sources\n\n- [widgets](references/widgets/) — https://example.com/acme/widgets.git\n"
	assertFileContents(t, filepath.Join(skillDir, "SKILL.md"), wantSkill)
}

func TestAddAndUpdatePersistInstalledSize(t *testing.T) {
	t.Parallel()

	const repositoryURL = "https://example.com/acme/widgets.git"
	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	cloner := &fakeCloner{contents: map[string]string{
		repositoryURL: "first",
	}}
	app := New(skillDir, cloner)

	if err := app.Add(context.Background(), repositoryURL); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	sources, err := app.Sources()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := sources[0].SizeBytes, int64Pointer(5); !reflect.DeepEqual(got, want) {
		t.Fatalf("size after add = %v, want %v", got, want)
	}

	cloner.contents[repositoryURL] = "second version"
	if err := app.Update(context.Background()); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	sources, err = app.Sources()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := sources[0].SizeBytes, int64Pointer(14); !reflect.DeepEqual(got, want) {
		t.Fatalf("size after update = %v, want %v", got, want)
	}
	if manifest := readFile(t, filepath.Join(skillDir, "sources.json")); !strings.Contains(manifest, `"size_bytes": 14`) {
		t.Fatalf("sources.json does not contain installed size:\n%s", manifest)
	}
}

func TestAddGitHubTreeURLCreatesRootedGitSource(t *testing.T) {
	t.Parallel()

	const (
		treeURL       = "https://github.com/Infisical/infisical/tree/main/docs"
		repositoryURL = "https://github.com/Infisical/infisical.git"
	)
	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	cloner := &fakeCloner{contents: map[string]string{
		repositoryURL: "Infisical documentation",
	}}
	app := New(skillDir, cloner)

	if err := app.Add(context.Background(), treeURL); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if got, want := len(cloner.requests), 1; got != want {
		t.Fatalf("clone requests = %d, want %d", got, want)
	}
	if got, want := cloner.requests, []CloneRequest{{
		URL:         repositoryURL,
		Ref:         "main",
		Root:        "docs",
		TextOnly:    true,
		Destination: cloner.requests[0].Destination,
	}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("clone requests = %#v, want %#v", got, want)
	}
	manifest := readFile(t, filepath.Join(skillDir, "sources.json"))
	for _, expected := range []string{
		`"name": "infisical"`,
		`"url": "https://github.com/Infisical/infisical.git"`,
		`"git_ref": "main"`,
		`"git_root": "docs"`,
		`"git_text_only": true`,
	} {
		if !strings.Contains(manifest, expected) {
			t.Errorf("sources.json does not contain %q:\n%s", expected, manifest)
		}
	}
	var listed bytes.Buffer
	if err := app.List(&listed); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !strings.Contains(listed.String(), "\t"+treeURL+"\t") {
		t.Errorf("List() does not display tree URL %q:\n%s", treeURL, listed.String())
	}
	skill := readFile(t, filepath.Join(skillDir, "SKILL.md"))
	if !strings.Contains(skill, treeURL) {
		t.Errorf("SKILL.md does not display tree URL %q:\n%s", treeURL, skill)
	}

	cloner.requests = nil
	if err := app.Update(context.Background()); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got, want := len(cloner.requests), 1; got != want {
		t.Fatalf("update clone requests = %d, want %d", got, want)
	}
	if got, want := cloner.requests[0].URL, repositoryURL; got != want {
		t.Errorf("updated repository URL = %q, want %q", got, want)
	}
	if got, want := cloner.requests[0].Ref, "main"; got != want {
		t.Errorf("updated Git ref = %q, want %q", got, want)
	}
	if got, want := cloner.requests[0].Root, "docs"; got != want {
		t.Errorf("updated Git root = %q, want %q", got, want)
	}
}

func TestUpdateMigratesExistingRootedGitSourceToTextOnly(t *testing.T) {
	t.Parallel()

	const repositoryURL = "https://github.com/Infisical/infisical.git"
	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	if err := os.MkdirAll(filepath.Join(skillDir, "references", "infisical"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "{\n  \"version\": 2,\n  \"sources\": [{\n    \"name\": \"infisical\",\n    \"provider\": \"git\",\n    \"url\": \"" + repositoryURL + "\",\n    \"git_ref\": \"main\",\n    \"git_root\": \"docs\"\n  }]\n}\n"
	if err := os.WriteFile(filepath.Join(skillDir, "sources.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	cloner := &fakeCloner{}
	app := New(skillDir, cloner)
	if err := app.Update(context.Background()); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got, want := len(cloner.requests), 1; got != want {
		t.Fatalf("clone requests = %d, want %d", got, want)
	}
	if !cloner.requests[0].TextOnly {
		t.Fatal("updated rooted Git source did not enable text-only checkout")
	}
	if manifest := readFile(t, filepath.Join(skillDir, "sources.json")); !strings.Contains(manifest, `"git_text_only": true`) {
		t.Fatalf("sources.json does not contain migrated text-only selection:\n%s", manifest)
	}
}

func TestAddRejectsGitHubTreeURLWithoutFolder(t *testing.T) {
	t.Parallel()

	cloner := &fakeCloner{}
	app := New(filepath.Join(t.TempDir(), "sourcebook"), cloner)
	err := app.Add(context.Background(), "https://github.com/acme/widgets/tree/main")
	if err == nil || !strings.Contains(err.Error(), "folder") {
		t.Fatalf("Add() error = %v, want missing folder error", err)
	}
	if len(cloner.calls) != 0 {
		t.Fatalf("clone calls = %v, want none", cloner.calls)
	}
}

func TestAddPresetMigratesLegacyRepositoriesToSources(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	if err := os.MkdirAll(filepath.Join(skillDir, "references", "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "{\n  \"version\": 1,\n  \"repositories\": [\n    {\n      \"name\": \"alpha\",\n      \"url\": \"https://example.com/alpha.git\"\n    }\n  ]\n}\n"
	if err := os.WriteFile(filepath.Join(skillDir, "repos.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	app := New(skillDir, &fakeCloner{})
	docs := &fakeProvider{contents: "# Documentation\n"}
	registerTestPreset(t, app, docs, "https://docs.example.com/")
	if err := app.AddPreset(context.Background(), "example-docs", nil); err != nil {
		t.Fatalf("AddPreset() error = %v", err)
	}

	assertFileContents(t, filepath.Join(skillDir, "references", "example-docs", "index.md"), "# Documentation\n")
	sources := readFile(t, filepath.Join(skillDir, "sources.json"))
	for _, expected := range []string{
		`"version": 2`,
		`"name": "alpha"`,
		`"provider": "git"`,
		`"name": "example-docs"`,
		`"provider": "example"`,
	} {
		if !strings.Contains(sources, expected) {
			t.Errorf("sources.json does not contain %q:\n%s", expected, sources)
		}
	}
	if _, err := os.Stat(filepath.Join(skillDir, "repos.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy repos.json stat error = %v, want not exist", err)
	}

	skill := readFile(t, filepath.Join(skillDir, "SKILL.md"))
	for _, expected := range []string{
		"## Sources",
		"[alpha](references/alpha/)",
		"[example-docs](references/example-docs/)",
		"Example documentation",
	} {
		if !strings.Contains(skill, expected) {
			t.Errorf("SKILL.md does not contain %q:\n%s", expected, skill)
		}
	}
}

func TestProviderUpdateReportsPageProgress(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	app := New(skillDir, &fakeCloner{})
	docs := &fakeProvider{contents: "first"}
	registerTestPreset(t, app, docs, "https://docs.example.com/")
	if err := app.AddPreset(context.Background(), "example-docs", nil); err != nil {
		t.Fatal(err)
	}
	docs.contents = "second"

	var events []UpdateEvent
	if err := app.UpdateWithProgress(context.Background(), func(event UpdateEvent) {
		events = append(events, event)
	}); err != nil {
		t.Fatalf("UpdateWithProgress() error = %v", err)
	}

	found := false
	for _, event := range events {
		if event.Source == "example-docs" && event.State == UpdateRunning &&
			event.Phase == "scraping" && event.Current == 2 && event.Total == 4 {
			found = true
		}
	}
	if !found {
		t.Fatalf("UpdateWithProgress() events = %#v, want provider page progress", events)
	}
	assertFileContents(t, filepath.Join(skillDir, "references", "example-docs", "index.md"), "second")
}

func TestAddWithProgressReportsGitPhases(t *testing.T) {
	t.Parallel()

	cloner := &fakeCloner{phases: []string{
		"selecting repository folder",
		"checking out files",
	}}
	app := New(filepath.Join(t.TempDir(), "sourcebook"), cloner)
	var events []UpdateEvent
	err := app.AddWithProgress(
		context.Background(),
		"https://github.com/acme/widgets/tree/main/docs",
		func(event UpdateEvent) {
			events = append(events, event)
		},
	)
	if err != nil {
		t.Fatalf("AddWithProgress() error = %v", err)
	}

	var phases []string
	for _, event := range events {
		if event.State == UpdateRunning {
			phases = append(phases, event.Phase)
		}
	}
	wantPhases := append([]string{"cloning repository"}, cloner.phases...)
	if got, want := strings.Join(phases, ","), strings.Join(wantPhases, ","); got != want {
		t.Fatalf("reported phases = %q, want %q", got, want)
	}
	if got := events[len(events)-1].State; got != UpdateCompleted {
		t.Fatalf("final event state = %q, want %q", got, UpdateCompleted)
	}
}

func TestFailedProviderUpdateLeavesAllSourcesUntouched(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	cloner := &fakeCloner{contents: map[string]string{
		"https://example.com/acme/alpha.git": "alpha one",
	}}
	app := New(skillDir, cloner)
	if err := app.Add(context.Background(), "https://example.com/acme/alpha.git"); err != nil {
		t.Fatal(err)
	}
	docs := &fakeProvider{contents: "docs one"}
	registerTestPreset(t, app, docs, "https://docs.example.com/")
	if err := app.AddPreset(context.Background(), "example-docs", nil); err != nil {
		t.Fatal(err)
	}

	cloner.contents["https://example.com/acme/alpha.git"] = "alpha two"
	docs.contents = "docs two"
	docs.err = errors.New("scrape failed")
	if err := app.Update(context.Background()); err == nil {
		t.Fatal("Update() error = nil, want provider failure")
	}

	assertFileContents(t, filepath.Join(skillDir, "references", "alpha", "README.md"), "alpha one")
	assertFileContents(t, filepath.Join(skillDir, "references", "example-docs", "index.md"), "docs one")
}

func TestUpdateSelectedOnlyRefreshesNamedSources(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	cloner := &fakeCloner{contents: map[string]string{
		"https://example.com/alpha.git": "alpha one",
		"https://example.com/beta.git":  "beta one",
	}}
	app := New(skillDir, cloner)
	initialTime := time.Date(2026, time.July, 22, 8, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return initialTime }
	for _, repositoryURL := range []string{
		"https://example.com/alpha.git",
		"https://example.com/beta.git",
	} {
		if err := app.Add(context.Background(), repositoryURL); err != nil {
			t.Fatal(err)
		}
	}

	cloner.contents["https://example.com/alpha.git"] = "alpha two"
	cloner.contents["https://example.com/beta.git"] = "beta two"
	updateTime := initialTime.Add(time.Hour)
	app.now = func() time.Time { return updateTime }
	if err := app.UpdateSelectedWithProgress(context.Background(), []string{"alpha"}, nil); err != nil {
		t.Fatalf("UpdateSelectedWithProgress() error = %v", err)
	}

	assertFileContents(t, filepath.Join(skillDir, "references", "alpha", "README.md"), "alpha two")
	assertFileContents(t, filepath.Join(skillDir, "references", "beta", "README.md"), "beta one")
	sources, err := app.Sources()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		want := initialTime
		if source.Name == "alpha" {
			want = updateTime
		}
		if !source.UpdatedAt.Equal(want) {
			t.Errorf("%s UpdatedAt = %v, want %v", source.Name, source.UpdatedAt, want)
		}
	}
}

func TestUpdateSelectedRejectsUnknownAndDuplicateNamesBeforeCloning(t *testing.T) {
	t.Parallel()

	cloner := &fakeCloner{}
	app := New(filepath.Join(t.TempDir(), "sourcebook"), cloner)
	if err := app.Add(context.Background(), "https://example.com/alpha.git"); err != nil {
		t.Fatal(err)
	}
	cloner.mu.Lock()
	cloner.calls = nil
	cloner.mu.Unlock()

	for _, names := range [][]string{{"missing"}, {"alpha", "alpha"}, nil} {
		err := app.UpdateSelectedWithProgress(context.Background(), names, nil)
		if err == nil {
			t.Fatalf("UpdateSelectedWithProgress(%v) error = nil", names)
		}
	}
	cloner.mu.Lock()
	defer cloner.mu.Unlock()
	if len(cloner.calls) != 0 {
		t.Fatalf("clone calls = %v, want none", cloner.calls)
	}
}

func registerTestPreset(t *testing.T, app *App, provider Provider, sourceURL string) {
	t.Helper()
	if err := app.RegisterProvider(ProviderDefinition{ID: "example", Provider: provider}); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	if err := app.RegisterCatalogEntry(CatalogEntry{
		ID:          "example-docs",
		DisplayName: "Example documentation",
		Description: "Example documentation fixture",
		Provider:    "example",
		SourceName:  "example-docs",
		SourceURL:   sourceURL,
	}); err != nil {
		t.Fatalf("RegisterCatalogEntry() error = %v", err)
	}
}

func TestFailedUpdateMetadataWriteRestoresPreviousReferences(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	cloner := &fakeCloner{contents: map[string]string{
		"https://example.com/acme/alpha.git": "alpha one",
	}}
	app := New(skillDir, cloner)
	if err := app.Add(context.Background(), "https://example.com/acme/alpha.git"); err != nil {
		t.Fatal(err)
	}
	manifestBefore := readFile(t, filepath.Join(skillDir, "sources.json"))
	cloner.contents["https://example.com/acme/alpha.git"] = "alpha two"
	if err := os.Remove(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(skillDir, "SKILL.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := app.Update(context.Background())
	if err == nil || !strings.Contains(err.Error(), "write SKILL.md") {
		t.Fatalf("Update() error = %v, want SKILL.md write error", err)
	}
	assertFileContents(t, filepath.Join(skillDir, "references", "alpha", "README.md"), "alpha one")
	assertFileContents(t, filepath.Join(skillDir, "sources.json"), manifestBefore)
}

func TestRenderedSkillDescriptionListsCurrentRepositories(t *testing.T) {
	t.Parallel()

	repositories := []Source{
		{Name: "zeta", URL: "https://example.com/zeta.git"},
		{Name: "alpha", URL: "https://example.com/alpha.git"},
	}
	skill := string(renderSkill(repositories))
	wantDescription := "description: Source code and documentation for alpha and zeta. Use when a task involves one of these sources or related technologies.\n"
	if !strings.Contains(skill, wantDescription) {
		t.Fatalf("SKILL.md does not contain source-aware description %q:\n%s", wantDescription, skill)
	}
	if strings.Contains(skill, "Open only the repository") {
		t.Fatalf("SKILL.md contains unwanted usage instruction:\n%s", skill)
	}
}

func TestRenderedSkillDescriptionStaysWithinSpecificationLimit(t *testing.T) {
	t.Parallel()

	repositories := make([]Source, 30)
	for i := range repositories {
		repositories[i] = Source{Name: fmt.Sprintf("repository-%02d-with-a-long-descriptive-name", i)}
	}
	description := renderDescription(repositories)
	if len(description) > 1024 {
		t.Fatalf("description length = %d, want at most 1024", len(description))
	}
	if !strings.Contains(description, " more") {
		t.Fatalf("truncated description = %q, want omitted repository count", description)
	}
}

func TestAddRejectsDuplicateRepository(t *testing.T) {
	t.Parallel()
	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	cloner := &fakeCloner{}
	app := New(skillDir, cloner)

	if err := app.Add(context.Background(), "https://example.com/acme/widgets.git"); err != nil {
		t.Fatalf("first Add() error = %v", err)
	}
	if err := app.Add(context.Background(), "ssh://git@example.com/other/widgets.git"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second Add() error = %v, want duplicate error", err)
	}
	if got, want := len(cloner.calls), 1; got != want {
		t.Fatalf("clone calls = %d, want %d", got, want)
	}
}

func TestAddRejectsCredentialBearingRepositoryURL(t *testing.T) {
	t.Parallel()

	for _, repositoryURL := range []string{
		"https://token@github.com/acme/widgets.git",
		"https://user:secret@github.com/acme/widgets.git",
		"https://github.com/acme/widgets.git?token=secret",
	} {
		t.Run(repositoryURL, func(t *testing.T) {
			cloner := &fakeCloner{}
			app := New(filepath.Join(t.TempDir(), "sourcebook"), cloner)
			err := app.Add(context.Background(), repositoryURL)
			if err == nil || !strings.Contains(err.Error(), "credentials") {
				t.Fatalf("Add(%q) error = %v, want credential error", repositoryURL, err)
			}
			if len(cloner.calls) != 0 {
				t.Fatalf("clone calls = %v, want none", cloner.calls)
			}
		})
	}
}

func TestUpdateReclonesEveryRepository(t *testing.T) {
	t.Parallel()
	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	cloner := &fakeCloner{contents: map[string]string{
		"https://example.com/acme/alpha.git": "alpha one",
		"https://example.com/acme/beta.git":  "beta one",
	}}
	app := New(skillDir, cloner)
	addRepositories(t, app)

	cloner.contents["https://example.com/acme/alpha.git"] = "alpha two"
	cloner.contents["https://example.com/acme/beta.git"] = "beta two"
	if err := app.Update(context.Background()); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	assertFileContents(t, filepath.Join(skillDir, "references", "alpha", "README.md"), "alpha two")
	assertFileContents(t, filepath.Join(skillDir, "references", "beta", "README.md"), "beta two")
}

func TestUpdateRecordsSuccessfulRefreshTime(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	app := New(skillDir, &fakeCloner{})
	addedAt := time.Date(2026, time.July, 21, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.July, 22, 9, 15, 0, 0, time.UTC)
	app.now = func() time.Time { return addedAt }
	addRepositories(t, app)
	app.now = func() time.Time { return updatedAt }

	if err := app.Update(context.Background()); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	repositories, err := app.Sources()
	if err != nil {
		t.Fatalf("Repositories() error = %v", err)
	}
	for _, repository := range repositories {
		if !repository.UpdatedAt.Equal(updatedAt) {
			t.Errorf("%s UpdatedAt = %v, want %v", repository.Name, repository.UpdatedAt, updatedAt)
		}
	}
}

func TestUpdateRegeneratesSkillMetadataAndTableOfContents(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	app := New(skillDir, &fakeCloner{})
	if err := app.Add(context.Background(), "https://example.com/acme/alpha.git"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("outdated"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := app.Update(context.Background()); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	skill := readFile(t, filepath.Join(skillDir, "SKILL.md"))
	for _, expected := range []string{
		"description: Source code and documentation for alpha.",
		"[alpha](references/alpha/)",
	} {
		if !strings.Contains(skill, expected) {
			t.Errorf("SKILL.md does not contain %q:\n%s", expected, skill)
		}
	}
}

func TestFailedUpdateLeavesEveryRepositoryUntouched(t *testing.T) {
	t.Parallel()
	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	cloner := &fakeCloner{contents: map[string]string{
		"https://example.com/acme/alpha.git": "alpha one",
		"https://example.com/acme/beta.git":  "beta one",
	}}
	app := New(skillDir, cloner)
	addRepositories(t, app)

	cloner.contents["https://example.com/acme/alpha.git"] = "alpha two"
	cloner.failURL = "https://example.com/acme/beta.git"
	events := make(chan UpdateEvent, 8)
	if err := app.UpdateWithProgress(context.Background(), func(event UpdateEvent) { events <- event }); err == nil {
		t.Fatal("Update() error = nil, want clone failure")
	}
	close(events)

	assertFileContents(t, filepath.Join(skillDir, "references", "alpha", "README.md"), "alpha one")
	assertFileContents(t, filepath.Join(skillDir, "references", "beta", "README.md"), "beta one")
	foundFailure := false
	for event := range events {
		if event.Source == "beta" && event.State == UpdateFailed && event.Err != nil {
			foundFailure = true
		}
	}
	if !foundFailure {
		t.Fatal("UpdateWithProgress() did not report beta failure")
	}
}

func TestUpdateClonesWithBoundedConcurrency(t *testing.T) {
	t.Parallel()

	const workerCount = 4
	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	app := New(skillDir, &fakeCloner{})
	for i := 0; i < workerCount+2; i++ {
		repositoryURL := fmt.Sprintf("https://example.com/acme/repo-%d.git", i)
		if err := app.Add(context.Background(), repositoryURL); err != nil {
			t.Fatalf("Add(%q) error = %v", repositoryURL, err)
		}
	}

	cloner := newBlockingCloner(workerCount + 2)
	app.cloner = cloner
	done := make(chan error, 1)
	go func() {
		done <- app.Update(context.Background())
	}()

	released := false
	finished := false
	defer func() {
		if !released {
			close(cloner.release)
		}
		if !finished {
			<-done
		}
	}()
	for i := 0; i < workerCount; i++ {
		select {
		case <-cloner.started:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d clones started concurrently, want %d", i, workerCount)
		}
	}
	select {
	case <-cloner.started:
		t.Fatalf("more than %d clones started before a worker was released", workerCount)
	case <-time.After(100 * time.Millisecond):
	}

	close(cloner.release)
	released = true
	if err := <-done; err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	finished = true
	if got := cloner.maximum(); got != workerCount {
		t.Fatalf("maximum concurrent clones = %d, want %d", got, workerCount)
	}
}

func TestMutationCommandsRejectConcurrentExecution(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	app := New(skillDir, &fakeCloner{})
	addRepositories(t, app)
	cloner := newBlockingCloner(2)
	app.cloner = cloner

	updateDone := make(chan error, 1)
	go func() {
		updateDone <- app.Update(context.Background())
	}()
	<-cloner.started
	removeErr := app.Remove("alpha")
	close(cloner.release)
	if updateErr := <-updateDone; updateErr != nil {
		t.Fatalf("Update() error = %v", updateErr)
	}
	if removeErr == nil || !strings.Contains(removeErr.Error(), "another sourcebook command is already running") {
		t.Fatalf("concurrent Remove() error = %v, want lock error", removeErr)
	}
}

func TestUpdateReportsRepositoryProgress(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	app := New(skillDir, &fakeCloner{})
	addRepositories(t, app)

	events := make(chan UpdateEvent, 8)
	err := app.UpdateWithProgress(context.Background(), func(event UpdateEvent) {
		events <- event
	})
	close(events)
	if err != nil {
		t.Fatalf("UpdateWithProgress() error = %v", err)
	}

	states := map[string][]UpdateState{}
	for event := range events {
		states[event.Source] = append(states[event.Source], event.State)
		if event.State == UpdateCompleted && event.Duration < 0 {
			t.Errorf("completed event duration = %v, want non-negative", event.Duration)
		}
	}
	for _, repository := range []string{"alpha", "beta"} {
		want := []UpdateState{UpdateRunning, UpdateCompleted}
		if got := states[repository]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s states = %v, want %v", repository, got, want)
		}
	}
	if got := states[""]; !reflect.DeepEqual(got, []UpdateState{UpdateInstalling}) {
		t.Errorf("global update states = %v, want [%s]", got, UpdateInstalling)
	}
}

func TestCanceledUpdateReportsCanceledClones(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	app := New(skillDir, &fakeCloner{})
	addRepositories(t, app)
	cloner := newBlockingCloner(2)
	app.cloner = cloner

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan UpdateEvent, 8)
	done := make(chan error, 1)
	go func() {
		done <- app.UpdateWithProgress(ctx, func(event UpdateEvent) { events <- event })
	}()
	<-cloner.started
	cancel()
	err := <-done
	close(events)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateWithProgress() error = %v, want context canceled", err)
	}

	foundCanceled := false
	for event := range events {
		if event.State == UpdateCanceled {
			foundCanceled = true
		}
	}
	if !foundCanceled {
		t.Fatal("UpdateWithProgress() did not report a canceled clone")
	}
}

func TestRemoveDeletesRepositoryAndRegeneratesFiles(t *testing.T) {
	t.Parallel()
	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	app := New(skillDir, &fakeCloner{})
	addRepositories(t, app)

	if err := app.Remove("alpha"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(skillDir, "references", "alpha")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed repository stat error = %v, want not exist", err)
	}
	skill := readFile(t, filepath.Join(skillDir, "SKILL.md"))
	if strings.Contains(skill, "alpha") || !strings.Contains(skill, "references/beta/") {
		t.Fatalf("SKILL.md after remove = %q", skill)
	}
	manifest := readFile(t, filepath.Join(skillDir, "sources.json"))
	if strings.Contains(manifest, "alpha") || !strings.Contains(manifest, "beta") {
		t.Fatalf("sources.json after remove = %q", manifest)
	}
}

func TestFailedSkillWriteRollsBackManifestAndRepository(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	app := New(skillDir, &fakeCloner{})
	if err := app.Add(context.Background(), "https://example.com/acme/alpha.git"); err != nil {
		t.Fatalf("Add(alpha) error = %v", err)
	}
	manifestBefore := readFile(t, filepath.Join(skillDir, "sources.json"))
	if err := os.Remove(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(skillDir, "SKILL.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := app.Add(context.Background(), "https://example.com/acme/beta.git")
	if err == nil || !strings.Contains(err.Error(), "write SKILL.md") {
		t.Fatalf("Add(beta) error = %v, want SKILL.md write error", err)
	}
	assertFileContents(t, filepath.Join(skillDir, "sources.json"), manifestBefore)
	if _, err := os.Stat(filepath.Join(skillDir, "references", "beta")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rolled-back beta repository stat error = %v, want not exist", err)
	}
}

func TestListIsSortedByRepositoryName(t *testing.T) {
	t.Parallel()
	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	app := New(skillDir, &fakeCloner{})
	updatedAt := time.Date(2026, time.July, 22, 8, 30, 0, 0, time.UTC)
	app.now = func() time.Time { return updatedAt }
	for _, url := range []string{"https://example.com/acme/zeta.git", "https://example.com/acme/alpha.git"} {
		if err := app.Add(context.Background(), url); err != nil {
			t.Fatalf("Add(%q) error = %v", url, err)
		}
	}

	var output bytes.Buffer
	if err := app.List(&output); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := "alpha\tgit\thttps://example.com/acme/alpha.git\t2026-07-22T08:30:00Z\t34\nzeta\tgit\thttps://example.com/acme/zeta.git\t2026-07-22T08:30:00Z\t33\n"
	if got := output.String(); got != want {
		t.Fatalf("List() = %q, want %q", got, want)
	}

	wantTOC := "## Sources\n\n- [alpha](references/alpha/) — https://example.com/acme/alpha.git\n- [zeta](references/zeta/) — https://example.com/acme/zeta.git\n"
	if skill := readFile(t, filepath.Join(skillDir, "SKILL.md")); !strings.Contains(skill, wantTOC) {
		t.Fatalf("SKILL.md does not contain compact repository TOC %q:\n%s", wantTOC, skill)
	}
}

func TestListShowsNeverForLegacyRepositoryWithoutRefreshTime(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := "{\n  \"repositories\": [{\"name\": \"alpha\", \"url\": \"https://example.com/alpha.git\"}]\n}\n"
	if err := os.WriteFile(filepath.Join(skillDir, "repos.json"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := New(skillDir, &fakeCloner{}).List(&output); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := "alpha\tgit\thttps://example.com/alpha.git\tnever\tunknown\n"
	if got := output.String(); got != want {
		t.Fatalf("List() = %q, want %q", got, want)
	}
}

func TestListMeasuresInstalledSizeMissingFromExistingManifest(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	referenceDir := filepath.Join(skillDir, "references", "alpha")
	if err := os.MkdirAll(referenceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(referenceDir, "README.md"), []byte("existing reference"), 0o644); err != nil {
		t.Fatal(err)
	}
	contents := "{\n  \"version\": 2,\n  \"sources\": [{\"name\": \"alpha\", \"provider\": \"git\", \"url\": \"https://example.com/alpha.git\"}]\n}\n"
	if err := os.WriteFile(filepath.Join(skillDir, "sources.json"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := New(skillDir, &fakeCloner{}).List(&output); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := "alpha\tgit\thttps://example.com/alpha.git\tnever\t18\n"
	if got := output.String(); got != want {
		t.Fatalf("List() = %q, want %q", got, want)
	}
	if manifest := readFile(t, filepath.Join(skillDir, "sources.json")); !strings.Contains(manifest, `"size_bytes": 18`) {
		t.Fatalf("sources.json does not cache measured size:\n%s", manifest)
	}
}

func TestRepositoryName(t *testing.T) {
	t.Parallel()
	for _, url := range []string{
		"https://github.com/acme/widgets.git",
		"git@github.com:acme/widgets.git",
		"https://github.com/acme/widgets/",
	} {
		got, err := repositoryName(url)
		if err != nil {
			t.Fatalf("repositoryName(%q) error = %v", url, err)
		}
		if got != "widgets" {
			t.Fatalf("repositoryName(%q) = %q, want widgets", url, got)
		}
	}

	for _, url := range []string{"", ".", "..", "https://example.com/.git"} {
		if _, err := repositoryName(url); err == nil {
			t.Errorf("repositoryName(%q) error = nil", url)
		}
	}
}

func TestInvalidManifestRepositoryNameIsRejected(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "{\n  \"repositories\": [{\"name\": \"../escape\", \"url\": \"https://example.com/escape.git\"}]\n}\n"
	if err := os.WriteFile(filepath.Join(skillDir, "repos.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	cloner := &fakeCloner{}
	err := New(skillDir, cloner).Update(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid repository name") {
		t.Fatalf("Update() error = %v, want invalid repository name", err)
	}
	if len(cloner.calls) != 0 {
		t.Fatalf("clone calls = %v, want none", cloner.calls)
	}
}

func TestLegacyManifestWithoutVersionIsAccepted(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := "{\n  \"repositories\": [{\"name\": \"alpha\", \"url\": \"https://example.com/alpha.git\"}]\n}\n"
	if err := os.WriteFile(filepath.Join(skillDir, "repos.json"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	repositories, err := New(skillDir, &fakeCloner{}).Sources()
	if err != nil {
		t.Fatalf("Repositories() error = %v", err)
	}
	want := []Source{{Name: "alpha", Provider: ProviderGit, URL: "https://example.com/alpha.git"}}
	if !reflect.DeepEqual(repositories, want) {
		t.Fatalf("Repositories() = %#v, want %#v", repositories, want)
	}
}

func TestUnsupportedManifestVersionIsRejected(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := "{\n  \"version\": 99,\n  \"repositories\": []\n}\n"
	if err := os.WriteFile(filepath.Join(skillDir, "repos.json"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := New(skillDir, &fakeCloner{}).Sources()
	if err == nil || !strings.Contains(err.Error(), "unsupported version 99") {
		t.Fatalf("Repositories() error = %v, want unsupported version", err)
	}
}

func addRepositories(t *testing.T, app *App) {
	t.Helper()
	for _, url := range []string{"https://example.com/acme/alpha.git", "https://example.com/acme/beta.git"} {
		if err := app.Add(context.Background(), url); err != nil {
			t.Fatalf("Add(%q) error = %v", url, err)
		}
	}
}

func int64Pointer(value int64) *int64 {
	return &value
}

type fakeCloner struct {
	contents map[string]string
	failURL  string
	phases   []string
	mu       sync.Mutex
	calls    []string
	requests []CloneRequest
}

type fakeProvider struct {
	contents string
	err      error
}

func (f *fakeProvider) Update(_ context.Context, request ProviderRequest, report ProviderProgressReporter) error {
	if err := os.MkdirAll(request.DestinationDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(request.DestinationDir, "index.md"), []byte(f.contents), 0o644); err != nil {
		return err
	}
	if report != nil {
		report(ProviderProgress{Phase: "scraping", Current: 2, Total: 4})
	}
	return f.err
}

func (f *fakeCloner) Clone(_ context.Context, request CloneRequest, report CloneProgressReporter) error {
	f.mu.Lock()
	f.calls = append(f.calls, request.URL)
	f.requests = append(f.requests, request)
	f.mu.Unlock()
	for _, phase := range f.phases {
		if report != nil {
			report(phase)
		}
	}
	if request.URL == f.failURL {
		return errors.New("clone failed")
	}
	if err := os.MkdirAll(request.Destination, 0o755); err != nil {
		return err
	}
	contents := request.URL
	if value, ok := f.contents[request.URL]; ok {
		contents = value
	}
	return os.WriteFile(filepath.Join(request.Destination, "README.md"), []byte(contents), 0o644)
}

type blockingCloner struct {
	mu        sync.Mutex
	active    int
	maxActive int
	started   chan struct{}
	release   chan struct{}
}

func newBlockingCloner(repositoryCount int) *blockingCloner {
	return &blockingCloner{
		started: make(chan struct{}, repositoryCount),
		release: make(chan struct{}),
	}
}

func (c *blockingCloner) Clone(ctx context.Context, request CloneRequest, _ CloneProgressReporter) error {
	c.mu.Lock()
	c.active++
	if c.active > c.maxActive {
		c.maxActive = c.active
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.active--
		c.mu.Unlock()
	}()

	c.started <- struct{}{}
	select {
	case <-c.release:
	case <-ctx.Done():
		return ctx.Err()
	}

	return os.MkdirAll(request.Destination, 0o755)
}

func (c *blockingCloner) maximum() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxActive
}

func assertFileContents(t *testing.T, path, want string) {
	t.Helper()
	if got := readFile(t, path); got != want {
		t.Fatalf("%s contents = %q, want %q", path, got, want)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(contents)
}

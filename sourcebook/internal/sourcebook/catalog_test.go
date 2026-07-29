package sourcebook

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRegisterCatalogEntryValidatesEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		entry CatalogEntry
		want  string
	}{
		{
			name: "unknown provider",
			entry: CatalogEntry{
				ID: "example-docs", DisplayName: "Example documentation",
				Provider: "missing", SourceName: "example-docs", SourceURL: "https://example.com/docs",
			},
			want: `uses unavailable provider "missing"`,
		},
		{
			name: "git credentials",
			entry: CatalogEntry{
				ID: "private-docs", DisplayName: "Private documentation",
				Provider: ProviderGit, SourceName: "private-docs", SourceURL: "https://token@example.com/docs.git",
			},
			want: "must not contain embedded credentials",
		},
		{
			name: "missing display name",
			entry: CatalogEntry{
				ID: "example-docs", Provider: ProviderGit,
				SourceName: "example-docs", SourceURL: "https://example.com/docs.git",
			},
			want: "has no display name",
		},
		{
			name: "root on scraper",
			entry: CatalogEntry{
				ID: "example-docs", DisplayName: "Example documentation",
				Provider: "missing", SourceName: "example-docs", SourceURL: "https://example.com/docs",
				GitRoot: "docs",
			},
			want: `uses unavailable provider "missing"`,
		},
		{
			name: "unsafe Git root",
			entry: CatalogEntry{
				ID: "example-docs", DisplayName: "Example documentation",
				Provider: ProviderGit, SourceName: "example-docs",
				SourceURL: "https://example.com/docs.git", GitRoot: "../docs",
			},
			want: "Git root",
		},
		{
			name: "unsafe Git ref",
			entry: CatalogEntry{
				ID: "example-docs", DisplayName: "Example documentation",
				Provider: ProviderGit, SourceName: "example-docs",
				SourceURL: "https://example.com/docs.git", GitRef: "--upload-pack=bad",
			},
			want: "Git ref",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := New(t.TempDir(), &fakeCloner{})
			err := app.RegisterCatalogEntry(test.entry)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("RegisterCatalogEntry() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAddPresetRoutesPowerBIDocsThroughGitProvider(t *testing.T) {
	t.Parallel()

	const repositoryURL = "https://github.com/MicrosoftDocs/powerbi-docs.git"
	const selectionURL = "https://github.com/MicrosoftDocs/powerbi-docs/tree/main/powerbi-docs"
	skillDir := t.TempDir()
	cloner := &fakeCloner{contents: map[string]string{repositoryURL: "Power BI docs"}}
	app := New(skillDir, cloner)
	entry := CatalogEntry{
		ID:          "powerbi-docs",
		DisplayName: "Power BI documentation",
		Description: "Official Microsoft Power BI documentation repository",
		Provider:    ProviderGit,
		SourceName:  "powerbi-docs",
		SourceURL:   repositoryURL,
		GitRef:      "main",
		GitRoot:     "powerbi-docs",
		GitTextOnly: true,
	}
	if err := app.RegisterCatalogEntry(entry); err != nil {
		t.Fatalf("RegisterCatalogEntry() error = %v", err)
	}
	if err := app.AddPreset(context.Background(), "powerbi-docs", nil); err != nil {
		t.Fatalf("AddPreset() error = %v", err)
	}

	assertFileContents(t, skillDir+"/references/powerbi-docs/README.md", "Power BI docs")
	manifest := readFile(t, skillDir+"/sources.json")
	for _, want := range []string{
		`"name": "powerbi-docs"`,
		`"provider": "git"`,
		`"url": "` + repositoryURL + `"`,
		`"title": "Power BI documentation"`,
		`"preset": "powerbi-docs"`,
		`"git_ref": "main"`,
		`"git_root": "powerbi-docs"`,
		`"git_text_only": true`,
	} {
		if !strings.Contains(manifest, want) {
			t.Errorf("sources.json does not contain %q:\n%s", want, manifest)
		}
	}
	skill := readFile(t, skillDir+"/SKILL.md")
	for _, want := range []string{
		"[powerbi-docs](references/powerbi-docs/)",
		"Power BI documentation",
		selectionURL,
	} {
		if !strings.Contains(skill, want) {
			t.Errorf("SKILL.md does not contain %q:\n%s", want, skill)
		}
	}
}

func TestRegisterCatalogEntryRejectsDuplicateIDAndSourceName(t *testing.T) {
	t.Parallel()

	app := New(t.TempDir(), &fakeCloner{})
	first := CatalogEntry{
		ID: "alpha", DisplayName: "Alpha", Provider: ProviderGit,
		SourceName: "shared", SourceURL: "https://example.com/alpha.git",
	}
	if err := app.RegisterCatalogEntry(first); err != nil {
		t.Fatal(err)
	}
	if err := app.RegisterCatalogEntry(CatalogEntry{
		ID: "alpha", DisplayName: "Other", Provider: ProviderGit,
		SourceName: "other", SourceURL: "https://example.com/other.git",
	}); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("duplicate ID error = %v", err)
	}
	if err := app.RegisterCatalogEntry(CatalogEntry{
		ID: "other", DisplayName: "Other", Provider: ProviderGit,
		SourceName: "shared", SourceURL: "https://example.com/other.git",
	}); err == nil || !strings.Contains(err.Error(), `source name "shared"`) {
		t.Fatalf("duplicate source name error = %v", err)
	}
}

func TestRegisterCatalogEntryRejectsGitOptionsForScraper(t *testing.T) {
	t.Parallel()

	app := New(t.TempDir(), &fakeCloner{})
	if err := app.RegisterProvider(ProviderDefinition{ID: "scraper", Provider: &fakeProvider{}}); err != nil {
		t.Fatal(err)
	}
	err := app.RegisterCatalogEntry(CatalogEntry{
		ID:          "example-docs",
		DisplayName: "Example documentation",
		Provider:    "scraper",
		SourceName:  "example-docs",
		SourceURL:   "https://example.com/docs",
		GitRoot:     "docs",
	})
	if err == nil || !strings.Contains(err.Error(), "has Git options") {
		t.Fatalf("RegisterCatalogEntry() error = %v, want Git options error", err)
	}
}

func TestUpdateMigratesExistingCatalogueSourceToCurrentGitSelection(t *testing.T) {
	t.Parallel()

	skillDir := t.TempDir()
	if err := os.MkdirAll(skillDir+"/references/powerbi-docs", 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "version": 2,
  "sources": [{
    "name": "powerbi-docs",
    "provider": "git",
    "url": "https://github.com/MicrosoftDocs/powerbi-docs.git",
    "title": "Power BI documentation"
  }]
}`
	if err := os.WriteFile(skillDir+"/sources.json", []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	cloner := &fakeCloner{}
	app := New(skillDir, cloner)
	if err := app.RegisterCatalogEntry(CatalogEntry{
		ID:          "powerbi-docs",
		DisplayName: "Power BI documentation",
		Provider:    ProviderGit,
		SourceName:  "powerbi-docs",
		SourceURL:   "https://github.com/MicrosoftDocs/powerbi-docs.git",
		GitRef:      "main",
		GitRoot:     "powerbi-docs",
		GitTextOnly: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.Update(context.Background()); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	cloner.mu.Lock()
	requests := append([]CloneRequest(nil), cloner.requests...)
	cloner.mu.Unlock()
	if len(requests) != 1 || requests[0].Ref != "main" || requests[0].Root != "powerbi-docs" {
		t.Fatalf("clone requests = %#v, want current Power BI selection", requests)
	}
	updated := readFile(t, skillDir+"/sources.json")
	for _, want := range []string{
		`"preset": "powerbi-docs"`,
		`"git_ref": "main"`,
		`"git_root": "powerbi-docs"`,
		`"git_text_only": true`,
	} {
		if !strings.Contains(updated, want) {
			t.Errorf("sources.json does not contain %q:\n%s", want, updated)
		}
	}
}

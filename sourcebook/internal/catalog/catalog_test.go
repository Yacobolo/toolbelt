package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
)

func TestPowerBIDocsPresetUsesGitProvider(t *testing.T) {
	t.Parallel()

	cloner := &recordingCloner{}
	skillDir := filepath.Join(t.TempDir(), "sourcebook")
	app := sourcebook.New(skillDir, cloner)
	if err := Register(app); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	entries := app.CatalogEntries()
	var powerBI sourcebook.CatalogEntry
	for _, entry := range entries {
		if entry.ID == PowerBIDocsID {
			powerBI = entry
			break
		}
	}
	if powerBI.Provider != sourcebook.ProviderGit {
		t.Fatalf("powerbi-docs provider = %q, want git", powerBI.Provider)
	}
	if powerBI.GitRef != "main" || powerBI.GitRoot != "powerbi-docs" || !powerBI.GitTextOnly {
		t.Fatalf("powerbi-docs Git selection = %q:%q, want main:powerbi-docs", powerBI.GitRef, powerBI.GitRoot)
	}
	if err := app.AddPreset(context.Background(), PowerBIDocsID, nil); err != nil {
		t.Fatalf("AddPreset() error = %v", err)
	}
	if got, want := cloner.url, "https://github.com/MicrosoftDocs/powerbi-docs.git"; got != want {
		t.Fatalf("clone URL = %q, want %q", got, want)
	}
	if cloner.ref != "main" || cloner.root != "powerbi-docs" || !cloner.textOnly {
		t.Fatalf("clone selection = %q:%q, want main:powerbi-docs", cloner.ref, cloner.root)
	}
}

type recordingCloner struct {
	url      string
	ref      string
	root     string
	textOnly bool
}

func (c *recordingCloner) Clone(_ context.Context, request sourcebook.CloneRequest, _ sourcebook.CloneProgressReporter) error {
	c.url = request.URL
	c.ref = request.Ref
	c.root = request.Root
	c.textOnly = request.TextOnly
	return os.MkdirAll(request.Destination, 0o755)
}

func TestGitCatalogueEntries(t *testing.T) {
	t.Parallel()

	app := sourcebook.New(filepath.Join(t.TempDir(), "sourcebook"), &recordingCloner{})
	if err := Register(app); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	want := map[string]struct {
		url      string
		ref      string
		root     string
		textOnly bool
	}{
		"azure-docs":    {"https://github.com/MicrosoftDocs/azure-docs.git", "main", "articles", true},
		"dbt-docs":      {"https://github.com/dbt-labs/docs.getdbt.com.git", "current", "website/docs", true},
		"duckdb-docs":   {"https://github.com/duckdb/duckdb-web.git", "main", "docs", true},
		"ducklake-docs": {"https://github.com/duckdb/ducklake-web.git", "main", "docs", true},
		"powerbi-docs":  {"https://github.com/MicrosoftDocs/powerbi-docs.git", "main", "powerbi-docs", true},
	}
	for _, entry := range app.CatalogEntries() {
		expected, exists := want[entry.ID]
		if !exists {
			continue
		}
		if entry.Provider != sourcebook.ProviderGit ||
			entry.SourceURL != expected.url ||
			entry.GitRef != expected.ref ||
			entry.GitRoot != expected.root ||
			entry.GitTextOnly != expected.textOnly {
			t.Errorf("%s = %#v, want Git %s %s:%s", entry.ID, entry, expected.url, expected.ref, expected.root)
		}
		delete(want, entry.ID)
	}
	for id := range want {
		t.Errorf("catalogue is missing %q", id)
	}
}

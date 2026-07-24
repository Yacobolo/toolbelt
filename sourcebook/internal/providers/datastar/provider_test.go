package datastar_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/providers/datastar"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
)

func TestProviderSplitsOfficialMarkdownIntoReferenceArticles(t *testing.T) {
	t.Parallel()

	markdown := `Datastar Docs

> HTML version available elsewhere.

# Getting Started

Welcome to Datastar.

` + "```md" + `
# This Is Not An Article
` + "```" + `

# Actions & Requests

Use @get('/endpoint').
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte(markdown))
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "datastar-docs")
	provider := datastar.New(datastar.Config{
		SourceURL: server.URL + "/docs.md",
		Client:    server.Client(),
	})
	var progress []sourcebook.ProviderProgress
	err := provider.Update(context.Background(), sourcebook.ProviderRequest{
		DestinationDir: destination,
	}, func(event sourcebook.ProviderProgress) {
		progress = append(progress, event)
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	index := readFile(t, filepath.Join(destination, "index.md"))
	for _, expected := range []string{
		"# Datastar Documentation",
		"[Getting Started](01-getting-started.md)",
		"[Actions & Requests](02-actions-and-requests.md)",
		server.URL + "/docs.md",
	} {
		if !strings.Contains(index, expected) {
			t.Errorf("index.md does not contain %q:\n%s", expected, index)
		}
	}
	gettingStarted := readFile(t, filepath.Join(destination, "01-getting-started.md"))
	if !strings.Contains(gettingStarted, "Datastar Docs") ||
		!strings.Contains(gettingStarted, "# This Is Not An Article") {
		t.Fatalf("unexpected first article:\n%s", gettingStarted)
	}
	actions := readFile(t, filepath.Join(destination, "02-actions-and-requests.md"))
	if !strings.Contains(actions, `title: "Actions & Requests"`) ||
		!strings.Contains(actions, "@get('/endpoint')") {
		t.Fatalf("unexpected actions article:\n%s", actions)
	}

	for _, unwanted := range []string{"SKILL.md", "docs.md", "scripts"} {
		if _, err := os.Stat(filepath.Join(destination, unwanted)); !os.IsNotExist(err) {
			t.Errorf("%s exists in scraped references; stat error = %v", unwanted, err)
		}
	}

	var completed bool
	for _, event := range progress {
		if event.Phase == "writing articles" && event.Current == 2 && event.Total == 2 {
			completed = true
		}
	}
	if !completed {
		t.Fatalf("progress = %#v, want completed article progress", progress)
	}
}

func TestProviderCreatesUniqueNamesForDuplicateHeadings(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("# Reference\n\nOne.\n\n# Reference\n\nTwo.\n"))
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "datastar-docs")
	provider := datastar.New(datastar.Config{SourceURL: server.URL, Client: server.Client()})
	if err := provider.Update(context.Background(), sourcebook.ProviderRequest{
		DestinationDir: destination,
	}, nil); err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{"01-reference.md", "02-reference-2.md"} {
		if _, err := os.Stat(filepath.Join(destination, filename)); err != nil {
			t.Errorf("%s stat error = %v", filename, err)
		}
	}
}

func TestProviderFailsWithoutTopLevelArticles(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("## Only a subsection\n"))
	}))
	defer server.Close()

	provider := datastar.New(datastar.Config{SourceURL: server.URL, Client: server.Client()})
	err := provider.Update(context.Background(), sourcebook.ProviderRequest{
		DestinationDir: filepath.Join(t.TempDir(), "datastar-docs"),
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "top-level") {
		t.Fatalf("Update() error = %v, want top-level article error", err)
	}
}

func TestLiveDatastarProviderSmoke(t *testing.T) {
	if os.Getenv("SOURCEBOOK_DATASTAR_LIVE") != "1" {
		t.Skip("set SOURCEBOOK_DATASTAR_LIVE=1 to run against data-star.dev")
	}

	destination := filepath.Join(t.TempDir(), "datastar-docs")
	provider := datastar.New(datastar.Config{Limit: 3})
	if err := provider.Update(context.Background(), sourcebook.ProviderRequest{
		DestinationDir: destination,
	}, nil); err != nil {
		t.Fatalf("live Update() error = %v", err)
	}

	index := readFile(t, filepath.Join(destination, "index.md"))
	if !strings.Contains(index, datastar.DefaultSourceURL) {
		t.Fatalf("live index does not contain source URL:\n%s", index)
	}
	matches, err := filepath.Glob(filepath.Join(destination, "[0-9][0-9]-*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 {
		t.Fatalf("live article count = %d, want 3", len(matches))
	}
}

func readFile(t *testing.T, filename string) string {
	t.Helper()
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

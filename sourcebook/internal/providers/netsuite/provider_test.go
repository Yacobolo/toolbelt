package netsuite_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/providers/netsuite"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
)

func TestProviderWritesNetSuiteDocumentationAndReportsProgress(t *testing.T) {
	t.Parallel()

	var betaHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/toc.htm":
			_, _ = w.Write([]byte(`<html><body><article><ul>
				<li><a href="alpha.html">Alpha</a>
					<ul>
						<li><a href="beta.html">Beta</a></li>
						<li><a href="beta.html">Beta Again</a></li>
					</ul>
				</li>
			</ul></article></body></html>`))
		case "/alpha.html":
			_, _ = w.Write([]byte(`<html><body><article><h1>Alpha</h1><p>Alpha body.</p></article></body></html>`))
		case "/beta.html":
			betaHits.Add(1)
			_, _ = w.Write([]byte(`<html><body><article><h1>Beta</h1><p><a href="alpha.html">Alpha link</a></p></article></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "netsuite-docs")
	provider := netsuite.New(netsuite.Config{
		TOCURL:      server.URL + "/toc.htm",
		Client:      server.Client(),
		Concurrency: 2,
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
	if betaHits.Load() != 1 {
		t.Fatalf("beta page hits = %d, want 1", betaHits.Load())
	}

	alpha := findFileContaining(t, destination, "title: \"Alpha\"")
	if !strings.Contains(alpha, "# Alpha") || !strings.Contains(alpha, "Alpha body.") {
		t.Fatalf("unexpected Alpha markdown:\n%s", alpha)
	}
	beta := findFileContaining(t, destination, "title: \"Beta\"")
	if !strings.Contains(beta, server.URL+"/alpha.html") {
		t.Fatalf("relative URL was not made absolute:\n%s", beta)
	}

	var completed bool
	for _, event := range progress {
		if event.Phase == "scraping" && event.Current == 2 && event.Total == 2 {
			completed = true
		}
	}
	if !completed {
		t.Fatalf("progress = %#v, want completed scrape event", progress)
	}
}

func TestProviderSpacesRequestsAcrossConcurrentWorkers(t *testing.T) {
	t.Parallel()

	var requestTimes []time.Time
	var requestTimesMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/toc.htm":
			_, _ = w.Write([]byte(`<article><ul>
				<li><a href="one.html">One</a></li>
				<li><a href="two.html">Two</a></li>
			</ul></article>`))
		case "/one.html", "/two.html":
			requestTimesMu.Lock()
			requestTimes = append(requestTimes, time.Now())
			requestTimesMu.Unlock()
			_, _ = w.Write([]byte(`<article><h1>Page</h1></article>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := netsuite.New(netsuite.Config{
		TOCURL:      server.URL + "/toc.htm",
		Client:      server.Client(),
		Concurrency: 2,
		Delay:       30 * time.Millisecond,
	})
	if err := provider.Update(context.Background(), sourcebook.ProviderRequest{
		DestinationDir: filepath.Join(t.TempDir(), "netsuite-docs"),
	}, nil); err != nil {
		t.Fatal(err)
	}

	requestTimesMu.Lock()
	defer requestTimesMu.Unlock()
	if len(requestTimes) != 2 {
		t.Fatalf("page request count = %d, want 2", len(requestTimes))
	}
	difference := requestTimes[1].Sub(requestTimes[0])
	if difference < 20*time.Millisecond {
		t.Fatalf("concurrent page request spacing = %v, want at least 20ms", difference)
	}
}

func TestProviderReturnsAnErrorWhenAnyPageFails(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/toc.htm" {
			_, _ = w.Write([]byte(`<article><ul><li><a href="missing.html">Missing</a></li></ul></article>`))
			return
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	provider := netsuite.New(netsuite.Config{
		TOCURL:      server.URL + "/toc.htm",
		Client:      server.Client(),
		Concurrency: 1,
		Retries:     1,
	})
	err := provider.Update(context.Background(), sourcebook.ProviderRequest{
		DestinationDir: filepath.Join(t.TempDir(), "netsuite-docs"),
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "1 page") {
		t.Fatalf("Update() error = %v, want page failure", err)
	}
}

func TestLiveNetSuiteProviderSmoke(t *testing.T) {
	if os.Getenv("SOURCEBOOK_NETSUITE_LIVE") != "1" {
		t.Skip("set SOURCEBOOK_NETSUITE_LIVE=1 to run against Oracle NetSuite documentation")
	}

	destination := filepath.Join(t.TempDir(), "netsuite-docs")
	provider := netsuite.New(netsuite.Config{
		Concurrency: 2,
		Delay:       100 * time.Millisecond,
		Limit:       3,
	})
	if err := provider.Update(context.Background(), sourcebook.ProviderRequest{
		DestinationDir: destination,
	}, nil); err != nil {
		t.Fatalf("live Update() error = %v", err)
	}

	var markdownFiles int
	err := filepath.WalkDir(destination, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && filepath.Ext(path) == ".md" {
			markdownFiles++
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if markdownFiles != 3 {
		t.Fatalf("live Markdown file count = %d, want 3", markdownFiles)
	}
}

func findFileContaining(t *testing.T, root, needle string) string {
	t.Helper()
	var match string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(contents), needle) {
			match = string(contents)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if match == "" {
		t.Fatalf("no Markdown file beneath %s contains %q", root, needle)
	}
	return match
}

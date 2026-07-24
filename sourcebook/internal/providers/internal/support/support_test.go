package support_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/providers/internal/support"
)

func TestFetcherRetriesTransientFailuresAndSetsUserAgent(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != "sourcebook-test" {
			t.Errorf("User-Agent = %q, want sourcebook-test", r.UserAgent())
		}
		if requests.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("documentation"))
	}))
	defer server.Close()

	fetcher := support.NewFetcher(server.Client(), "sourcebook-test", 1, 1024, 0)
	body, err := fetcher.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "documentation" {
		t.Fatalf("body = %q, want documentation", body)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want 2", requests.Load())
	}
}

func TestFetcherRejectsOversizedResponses(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("too large"))
	}))
	defer server.Close()

	fetcher := support.NewFetcher(server.Client(), "", 0, 4, 0)
	_, err := fetcher.Get(context.Background(), server.URL)
	if err == nil || !strings.Contains(err.Error(), "exceeds 4 bytes") {
		t.Fatalf("Get() error = %v, want response-size error", err)
	}
}

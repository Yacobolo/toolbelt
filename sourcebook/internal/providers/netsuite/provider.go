package netsuite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/providers/internal/support"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
)

const (
	ProviderID    = "netsuite"
	SourceName    = "netsuite-docs"
	DisplayName   = "NetSuite documentation"
	DefaultTOCURL = "https://docs.oracle.com/en/cloud/saas/netsuite/ns-online-help/toc.htm"
)

type Config struct {
	TOCURL      string
	Client      *http.Client
	Concurrency int
	Delay       time.Duration
	Retries     int
	Limit       int
	UserAgent   string
}

type Provider struct {
	config  Config
	fetcher *support.Fetcher
}

func New(config Config) *Provider {
	if config.TOCURL == "" {
		config.TOCURL = DefaultTOCURL
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 12
	}
	if config.Delay <= 0 {
		config.Delay = 20 * time.Millisecond
	}
	if config.Retries < 0 {
		config.Retries = 0
	}
	if config.Retries == 0 {
		config.Retries = 3
	}
	return &Provider{
		config: config,
		fetcher: support.NewFetcher(
			config.Client,
			config.UserAgent,
			config.Retries,
			support.DefaultMaxBytes,
			config.Delay,
		),
	}
}

func Definition() sourcebook.ProviderDefinition {
	return sourcebook.ProviderDefinition{
		ID:       ProviderID,
		Provider: New(Config{}),
	}
}

func CatalogEntry() sourcebook.CatalogEntry {
	return sourcebook.CatalogEntry{
		ID:          SourceName,
		DisplayName: DisplayName,
		Description: "Oracle NetSuite online help converted to Markdown",
		Provider:    ProviderID,
		SourceName:  SourceName,
		SourceURL:   DefaultTOCURL,
	}
}

func (p *Provider) Update(ctx context.Context, request sourcebook.ProviderRequest, report sourcebook.ProviderProgressReporter) error {
	if request.DestinationDir == "" {
		return errors.New("destination directory is required")
	}
	tocURLValue := p.config.TOCURL
	if request.Source.URL != "" {
		tocURLValue = request.Source.URL
	}
	tocURL, err := url.Parse(tocURLValue)
	if err != nil {
		return fmt.Errorf("parse table-of-contents URL: %w", err)
	}
	if tocURL.Scheme != "http" && tocURL.Scheme != "https" {
		return fmt.Errorf("unsupported table-of-contents URL scheme %q", tocURL.Scheme)
	}
	if err := os.MkdirAll(request.DestinationDir, 0o755); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	emitProviderProgress(report, sourcebook.ProviderProgress{Phase: "fetching table of contents"})
	tocHTML, err := p.fetcher.Get(ctx, tocURLValue)
	if err != nil {
		return fmt.Errorf("fetch table of contents: %w", err)
	}
	entries, err := parseTOC(bytes.NewReader(tocHTML), tocURL, request.DestinationDir)
	if err != nil {
		return err
	}
	if p.config.Limit > 0 && p.config.Limit < len(entries) {
		entries = entries[:p.config.Limit]
	}

	unique := make(map[string][]entry)
	for _, item := range entries {
		unique[item.URL] = append(unique[item.URL], item)
	}
	urls := make([]string, 0, len(unique))
	for rawURL := range unique {
		urls = append(urls, rawURL)
	}
	sort.Strings(urls)

	type result struct {
		url string
		err error
	}
	jobs := make(chan string)
	results := make(chan result)
	workers := min(p.config.Concurrency, len(urls))
	if workers == 0 {
		return errors.New("table of contents contains no documentation pages")
	}

	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			for rawURL := range jobs {
				err := p.scrapeURL(ctx, rawURL, unique[rawURL])
				select {
				case results <- result{url: rawURL, err: err}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, rawURL := range urls {
			select {
			case jobs <- rawURL:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		group.Wait()
		close(results)
	}()

	completed := 0
	var failures []error
	for result := range results {
		completed++
		if result.err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", result.url, result.err))
		}
		if completed == len(urls) || completed%25 == 0 {
			emitProviderProgress(report, sourcebook.ProviderProgress{
				Phase:   "scraping",
				Current: completed,
				Total:   len(urls),
			})
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(failures) > 0 {
		sample := failures[:min(5, len(failures))]
		return fmt.Errorf("%d page(s) failed: %w", len(failures), errors.Join(sample...))
	}
	return nil
}

func (p *Provider) scrapeURL(ctx context.Context, rawURL string, entries []entry) error {
	body, err := p.fetcher.Get(ctx, rawURL)
	if err != nil {
		return err
	}
	pageURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	markdown, err := extractMarkdown(body, pageURL)
	if err != nil {
		return err
	}
	scrapedAt := time.Now().UTC()
	var failures []error
	for _, item := range entries {
		if err := writeMarkdown(item, markdown, scrapedAt); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func emitProviderProgress(report sourcebook.ProviderProgressReporter, progress sourcebook.ProviderProgress) {
	if report != nil {
		report(progress)
	}
}

func writeMarkdown(item entry, markdown string, scrapedAt time.Time) error {
	var frontMatter strings.Builder
	frontMatter.WriteString("---\n")
	writeJSONScalar(&frontMatter, "title", item.Title)
	writeJSONScalar(&frontMatter, "source_url", item.URL)
	writeJSONScalar(&frontMatter, "oracle_path", item.OraclePath)
	writeJSONScalar(&frontMatter, "scraped_at", scrapedAt.Format(time.RFC3339))
	frontMatter.WriteString("breadcrumbs:\n")
	for _, crumb := range item.Breadcrumbs {
		encoded, _ := json.Marshal(crumb)
		fmt.Fprintf(&frontMatter, "  - %s\n", encoded)
	}
	frontMatter.WriteString("---\n\n")
	return support.WriteFileAtomically(item.OutputPath, []byte(frontMatter.String()+markdown))
}

func writeJSONScalar(builder *strings.Builder, key, value string) {
	encoded, _ := json.Marshal(value)
	fmt.Fprintf(builder, "%s: %s\n", key, encoded)
}

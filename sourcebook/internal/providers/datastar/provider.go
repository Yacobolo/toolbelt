package datastar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/providers/internal/support"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
)

const (
	ProviderID       = "datastar"
	SourceName       = "datastar-docs"
	DisplayName      = "Datastar documentation"
	DefaultSourceURL = "https://data-star.dev/docs.md"
)

type Config struct {
	SourceURL string
	Client    *http.Client
	Retries   int
	Limit     int
	UserAgent string
}

type Provider struct {
	config  Config
	fetcher *support.Fetcher
}

func New(config Config) *Provider {
	if config.SourceURL == "" {
		config.SourceURL = DefaultSourceURL
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
			0,
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
		Description: "Official data-star.dev documentation converted into searchable Markdown articles",
		Provider:    ProviderID,
		SourceName:  SourceName,
		SourceURL:   DefaultSourceURL,
	}
}

func (p *Provider) Update(ctx context.Context, request sourcebook.ProviderRequest, report sourcebook.ProviderProgressReporter) error {
	if request.DestinationDir == "" {
		return errors.New("destination directory is required")
	}
	sourceURL := p.config.SourceURL
	if request.Source.URL != "" {
		sourceURL = request.Source.URL
	}
	parsedURL, err := url.Parse(sourceURL)
	if err != nil {
		return fmt.Errorf("parse documentation URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported documentation URL scheme %q", parsedURL.Scheme)
	}

	emitProgress(report, sourcebook.ProviderProgress{Phase: "fetching documentation"})
	markdown, err := p.fetcher.Get(ctx, sourceURL)
	if err != nil {
		return fmt.Errorf("fetch documentation: %w", err)
	}
	articles, err := splitArticles(string(markdown))
	if err != nil {
		return err
	}
	if p.config.Limit > 0 && p.config.Limit < len(articles) {
		articles = articles[:p.config.Limit]
	}
	filenames := articleFilenames(articles)

	if err := os.MkdirAll(request.DestinationDir, 0o755); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	fetchedAt := time.Now().UTC()
	for index, article := range articles {
		contents, err := renderArticle(article, sourceURL, fetchedAt)
		if err != nil {
			return err
		}
		if err := support.WriteFileAtomically(filepath.Join(request.DestinationDir, filenames[index]), contents); err != nil {
			return fmt.Errorf("write article %q: %w", article.Title, err)
		}
		emitProgress(report, sourcebook.ProviderProgress{
			Phase:   "writing articles",
			Current: index + 1,
			Total:   len(articles),
		})
	}
	if err := support.WriteFileAtomically(
		filepath.Join(request.DestinationDir, "index.md"),
		renderIndex(articles, filenames, sourceURL),
	); err != nil {
		return fmt.Errorf("write documentation index: %w", err)
	}
	return nil
}

type article struct {
	Title string
	Body  string
}

func splitArticles(markdown string) ([]article, error) {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	markdown = strings.ReplaceAll(markdown, "\r", "\n")
	lines := strings.Split(markdown, "\n")

	var articles []article
	var preamble []string
	var currentTitle string
	var currentLines []string
	var fenceMarker string

	appendCurrent := func() {
		if currentTitle == "" {
			return
		}
		body := strings.TrimSpace(strings.Join(currentLines, "\n")) + "\n"
		articles = append(articles, article{Title: currentTitle, Body: body})
	}

	for _, line := range lines {
		trimmedLeft := strings.TrimLeft(line, " \t")
		if fenceMarker == "" {
			if marker := openingFence(trimmedLeft); marker != "" {
				fenceMarker = marker
			}
		} else if strings.HasPrefix(trimmedLeft, fenceMarker) {
			fenceMarker = ""
		}

		title := ""
		if fenceMarker == "" && strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
		if title != "" {
			appendCurrent()
			currentTitle = title
			currentLines = []string{line}
			if len(articles) == 0 && len(preamble) > 0 {
				currentLines = append(append(append([]string(nil), preamble...), ""), line)
			}
			continue
		}
		if currentTitle == "" {
			if strings.TrimSpace(line) != "" || len(preamble) > 0 {
				preamble = append(preamble, line)
			}
			continue
		}
		currentLines = append(currentLines, line)
	}
	appendCurrent()
	if len(articles) == 0 {
		return nil, errors.New("documentation contains no top-level H1 articles")
	}
	return articles, nil
}

func openingFence(line string) string {
	for _, marker := range []byte{'`', '~'} {
		count := 0
		for count < len(line) && line[count] == marker {
			count++
		}
		if count >= 3 {
			return line[:count]
		}
	}
	return ""
}

func articleFilenames(articles []article) []string {
	seen := make(map[string]int)
	filenames := make([]string, len(articles))
	for index, article := range articles {
		base := slugify(article.Title)
		if base == "" {
			base = "article"
		}
		seen[base]++
		slug := base
		if seen[base] > 1 {
			slug = fmt.Sprintf("%s-%d", base, seen[base])
		}
		filenames[index] = fmt.Sprintf("%02d-%s.md", index+1, slug)
	}
	return filenames
}

func slugify(value string) string {
	value = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "&", " and ")
	var builder strings.Builder
	lastDash := false
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			if character <= unicode.MaxASCII {
				builder.WriteRune(character)
				lastDash = false
			}
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func renderArticle(article article, sourceURL string, fetchedAt time.Time) ([]byte, error) {
	title, err := jsonString(article.Title)
	if err != nil {
		return nil, err
	}
	source, err := jsonString(sourceURL)
	if err != nil {
		return nil, err
	}
	var contents strings.Builder
	contents.WriteString("---\n")
	fmt.Fprintf(&contents, "title: %s\n", title)
	fmt.Fprintf(&contents, "source_url: %s\n", source)
	fmt.Fprintf(&contents, "fetched_at: %q\n", fetchedAt.Format(time.RFC3339))
	contents.WriteString("---\n\n")
	contents.WriteString(strings.TrimSpace(article.Body))
	contents.WriteByte('\n')
	return []byte(contents.String()), nil
}

func jsonString(value string) (string, error) {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSpace(encoded.String()), nil
}

func renderIndex(articles []article, filenames []string, sourceURL string) []byte {
	var index strings.Builder
	index.WriteString("# Datastar Documentation\n\n")
	fmt.Fprintf(&index, "Source: %s\n\n", sourceURL)
	for position, article := range articles {
		fmt.Fprintf(&index, "%02d. [%s](%s)\n", position+1, article.Title, filenames[position])
	}
	return []byte(index.String())
}

func emitProgress(report sourcebook.ProviderProgressReporter, progress sourcebook.ProviderProgress) {
	if report != nil {
		report(progress)
	}
}

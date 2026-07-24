package netsuite

import (
	"net/url"
	"strings"
	"testing"
)

func TestExtractMarkdownKeepsArticleAndRewritesLinks(t *testing.T) {
	t.Parallel()

	pageURL, _ := url.Parse("https://docs.example.com/help/section.html")
	raw := []byte(`<html><body>
		<nav>outside navigation</nav>
		<article>
			<header><ol class="breadcrumb"><li>Breadcrumb</li></ol><h1>Inventory</h1></header>
			<p>See <a href="related.html">Related</a>.</p>
			<p><a href="#local">Local</a></p>
			<img src="images/example.png">
		</article>
	</body></html>`)

	markdown, err := extractMarkdown(raw, pageURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"# Inventory",
		"https://docs.example.com/help/related.html",
		"https://docs.example.com/help/images/example.png",
		"(#local)",
	} {
		if !strings.Contains(markdown, expected) {
			t.Errorf("Markdown does not contain %q:\n%s", expected, markdown)
		}
	}
	for _, unwanted := range []string{"outside navigation", "Breadcrumb"} {
		if strings.Contains(markdown, unwanted) {
			t.Errorf("Markdown unexpectedly contains %q:\n%s", unwanted, markdown)
		}
	}
}

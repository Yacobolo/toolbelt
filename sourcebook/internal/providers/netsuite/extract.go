package netsuite

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"golang.org/x/net/html"
)

func extractMarkdown(raw []byte, pageURL *url.URL) (string, error) {
	cleanHTML, err := extractArticleHTML(raw)
	if err != nil || strings.TrimSpace(cleanHTML) == "" {
		fallback, fallbackErr := readability.FromReader(bytes.NewReader(raw), pageURL)
		if fallbackErr != nil {
			return "", fmt.Errorf("extract article: %v; readability fallback: %w", err, fallbackErr)
		}
		var rendered bytes.Buffer
		if err := fallback.RenderHTML(&rendered); err != nil {
			return "", err
		}
		cleanHTML = rendered.String()
	}
	rewritten, err := absolutizeLinks(cleanHTML, pageURL)
	if err != nil {
		return "", err
	}
	markdown, err := htmltomarkdown.ConvertString(rewritten)
	if err != nil {
		return "", err
	}
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return "", errors.New("empty Markdown")
	}
	return markdown + "\n", nil
}

func extractArticleHTML(raw []byte) (string, error) {
	document, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	article := firstElement(document, "article")
	if article == nil {
		return "", errors.New("missing article")
	}
	removeMatching(article, func(node *html.Node) bool {
		if node.Type != html.ElementNode {
			return false
		}
		if node.Data == "script" || node.Data == "style" {
			return true
		}
		class := attr(node, "class")
		return strings.Contains(class, "noscript") || strings.Contains(class, "breadcrumb")
	})
	var rendered bytes.Buffer
	if err := html.Render(&rendered, article); err != nil {
		return "", err
	}
	return rendered.String(), nil
}

func absolutizeLinks(fragment string, base *url.URL) (string, error) {
	nodes, err := html.ParseFragment(strings.NewReader(fragment), nil)
	if err != nil {
		return "", err
	}
	var rendered bytes.Buffer
	for _, node := range nodes {
		rewriteNodeLinks(node, base)
		if err := html.Render(&rendered, node); err != nil {
			return "", err
		}
	}
	return rendered.String(), nil
}

func rewriteNodeLinks(node *html.Node, base *url.URL) {
	if node.Type == html.ElementNode {
		for index, attribute := range node.Attr {
			if attribute.Key != "href" && attribute.Key != "src" {
				continue
			}
			value := strings.TrimSpace(attribute.Val)
			if value == "" || strings.HasPrefix(value, "#") ||
				strings.HasPrefix(value, "mailto:") || strings.HasPrefix(value, "javascript:") {
				continue
			}
			resolved, err := base.Parse(value)
			if err == nil {
				node.Attr[index].Val = resolved.String()
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		rewriteNodeLinks(child, base)
	}
}

func removeMatching(node *html.Node, match func(*html.Node) bool) {
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		if match(child) {
			node.RemoveChild(child)
		} else {
			removeMatching(child, match)
		}
		child = next
	}
}

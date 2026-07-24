package netsuite

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

type tocNode struct {
	Title       string
	URL         string
	OraclePath  string
	OutputPath  string
	Breadcrumbs []string
	Children    []*tocNode
}

type entry struct {
	Title       string
	URL         string
	OraclePath  string
	OutputPath  string
	Breadcrumbs []string
}

func parseTOC(reader io.Reader, base *url.URL, outputDir string) ([]entry, error) {
	document, err := html.Parse(reader)
	if err != nil {
		return nil, err
	}
	article := firstElement(document, "article")
	if article == nil {
		return nil, errors.New("table of contents has no article element")
	}
	root := firstChildElement(article, "ul")
	if root == nil {
		return nil, errors.New("table of contents has no root list")
	}
	nodes := parseList(root, base, nil)
	assignOutputPaths(nodes, outputDir, nil)
	var entries []entry
	flattenNodes(nodes, &entries)
	return entries, nil
}

func parseList(list *html.Node, base *url.URL, breadcrumbs []string) []*tocNode {
	var nodes []*tocNode
	for child := list.FirstChild; child != nil; child = child.NextSibling {
		if !isElement(child, "li") {
			continue
		}
		link := firstDescendantElement(child, "a")
		if link == nil {
			continue
		}
		href := attr(link, "href")
		resolved, err := base.Parse(href)
		if err != nil || resolved.Host != base.Host {
			continue
		}
		resolved.Fragment = ""
		title := strings.TrimSpace(nodeText(link))
		if title == "" {
			title = path.Base(resolved.Path)
		}
		node := &tocNode{
			Title:       title,
			URL:         resolved.String(),
			OraclePath:  path.Base(resolved.Path),
			Breadcrumbs: append(append([]string(nil), breadcrumbs...), title),
		}
		if nested := directChildList(child); nested != nil {
			node.Children = parseList(nested, base, node.Breadcrumbs)
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func assignOutputPaths(nodes []*tocNode, outputDir string, parent []string) {
	used := make(map[string]int)
	for index, node := range nodes {
		stem := strings.TrimSuffix(path.Base(node.OraclePath), path.Ext(node.OraclePath))
		slug := fmt.Sprintf("%04d-%s", index+1, slugify(node.Title))
		if slug == fmt.Sprintf("%04d-", index+1) {
			slug = fmt.Sprintf("%04d-%s", index+1, stem)
		}
		if used[slug] > 0 {
			slug += "-" + slugify(stem)
		}
		used[slug]++
		parts := append(append([]string(nil), parent...), slug)
		node.OutputPath = filepath.Join(append([]string{outputDir}, append(parts, "index.md")...)...)
		assignOutputPaths(node.Children, outputDir, parts)
	}
}

func flattenNodes(nodes []*tocNode, entries *[]entry) {
	for _, node := range nodes {
		*entries = append(*entries, entry{
			Title:       node.Title,
			URL:         node.URL,
			OraclePath:  node.OraclePath,
			OutputPath:  node.OutputPath,
			Breadcrumbs: append([]string(nil), node.Breadcrumbs...),
		})
		flattenNodes(node.Children, entries)
	}
}

func directChildList(node *html.Node) *html.Node {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if isElement(child, "ul") {
			return child
		}
	}
	return nil
}

func firstElement(node *html.Node, tag string) *html.Node {
	if isElement(node, tag) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := firstElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func firstChildElement(node *html.Node, tag string) *html.Node {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if isElement(child, tag) {
			return child
		}
	}
	return nil
}

func firstDescendantElement(node *html.Node, tag string) *html.Node {
	if isElement(node, tag) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := firstDescendantElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func isElement(node *html.Node, tag string) bool {
	return node != nil && node.Type == html.ElementNode && node.Data == tag
}

func attr(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ""
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.Join(strings.Fields(builder.String()), " ")
}

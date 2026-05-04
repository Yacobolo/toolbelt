package web

import (
	lucide "github.com/eduardolat/gomponents-lucide"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type detailTab struct {
	Value string
	Label string
	Href  string
}

func breadcrumbsNode(items []breadcrumbItem) g.Node {
	nodes := g.Group{}
	for idx, item := range items {
		if idx > 0 {
			nodes = append(nodes, h.Span(h.Class("text-stone-300"), g.Text("/")))
		}
		if item.Href == "" {
			nodes = append(nodes, h.Span(h.Class("font-semibold text-stone-600"), g.Text(item.Label)))
			continue
		}
		nodes = append(nodes, h.A(h.Class("text-stone-500 transition hover:text-stone-900"), h.Href(item.Href), g.Text(item.Label)))
	}

	return h.Nav(
		h.Class("flex flex-wrap items-center gap-2 text-[11px] uppercase tracking-[0.18em]"),
		g.Attr("aria-label", "Breadcrumb"),
		nodes,
	)
}

func detailTabs(active string, tabs []detailTab) g.Node {
	buttons := g.Group{}
	for _, tab := range tabs {
		buttons = append(buttons, detailTabButton(active, tab))
	}

	return h.Section(
		h.Class("border-b border-stone-200"),
		h.Div(h.Class("flex flex-wrap gap-6"), buttons),
	)
}

func detailTabButton(active string, tab detailTab) g.Node {
	return h.A(
		h.Class(detailTabClass(active, tab.Value)),
		h.Href(tab.Href),
		g.Attr("aria-current", detailAriaCurrent(active, tab.Value)),
		detailTabIcon(active, tab.Value),
		g.Text(tab.Label),
	)
}

func detailTabClass(active string, value string) string {
	base := "inline-flex items-center gap-2 border-b-2 px-1 py-3 text-sm font-semibold transition"
	if active == value {
		return base + " border-stone-950 text-stone-950"
	}
	return base + " border-transparent text-stone-500 hover:text-stone-950"
}

func detailAriaCurrent(active string, value string) string {
	if active == value {
		return "page"
	}
	return "false"
}

func detailTabIconClass(active string, value string) string {
	if active == value {
		return "h-4 w-4 shrink-0 text-stone-950"
	}
	return "h-4 w-4 shrink-0 text-stone-400"
}

func detailTabIcon(active string, value string) g.Node {
	iconClass := h.Class(detailTabIconClass(active, value))
	switch value {
	case "overview":
		return lucide.House(iconClass)
	case "lineage", "neighborhood":
		return lucide.GitBranch(iconClass)
	case "source", "file":
		return lucide.FileText(iconClass)
	case "files":
		return lucide.FolderSearch(iconClass)
	default:
		return lucide.BookOpen(iconClass)
	}
}

func detailPane(active string, value string, children ...g.Node) g.Node {
	if active != value {
		return nil
	}
	return h.Div(h.Class("space-y-6"), g.Group(children))
}

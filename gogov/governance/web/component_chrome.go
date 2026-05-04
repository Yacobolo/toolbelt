package web

import (
	lucide "github.com/eduardolat/gomponents-lucide"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func statusBadge(label string, tone string) g.Node {
	return h.Div(
		h.Class(statusBadgeClass(tone)),
		statusDot(tone),
		h.Span(g.Text(label)),
	)
}

func statusBadgeClass(tone string) string {
	base := "inline-flex items-center gap-2 border px-3 py-2 text-sm font-semibold"
	switch tone {
	case "good":
		return base + " border-emerald-200 bg-emerald-50 text-emerald-900"
	case "warn":
		return base + " border-amber-200 bg-amber-50 text-amber-950"
	case "critical":
		return base + " border-rose-200 bg-rose-50 text-rose-950"
	default:
		return base + " border-stone-200 bg-white text-stone-700"
	}
}

func statusDot(tone string) g.Node {
	className := "mt-1 h-2.5 w-2.5 rounded-full bg-stone-400"
	switch tone {
	case "good":
		className = "mt-1 h-2.5 w-2.5 rounded-full bg-emerald-500"
	case "warn":
		className = "mt-1 h-2.5 w-2.5 rounded-full bg-amber-500"
	case "critical":
		className = "mt-1 h-2.5 w-2.5 rounded-full bg-rose-500"
	}
	return h.Span(h.Class(className))
}

func statusCard(label string, value string) g.Node {
	return h.Div(
		h.Class("border border-stone-200 bg-white p-5"),
		h.P(h.Class("text-xs uppercase tracking-[0.2em] text-stone-500"), g.Text(label)),
		h.P(h.Class("mt-3 text-2xl font-semibold tracking-[-0.03em] text-stone-950"), g.Text(value)),
	)
}

func summaryCard(icon string, label string, value string, emphasize bool) g.Node {
	valueClass := "mt-2 text-xl font-bold"
	if emphasize {
		valueClass = "mt-2 text-3xl font-black"
	}

	return h.Div(
		h.Class("border border-stone-200 bg-white p-5"),
		labelWithIcon(icon, label),
		h.P(h.Class(valueClass), g.Text(value)),
	)
}

func labelWithIcon(icon string, label string) g.Node {
	return h.P(
		h.Class("flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-stone-500"),
		uiIcon(icon, "h-3.5 w-3.5 shrink-0 text-stone-400"),
		h.Span(g.Text(label)),
	)
}

func navIconClass(active bool) string {
	if active {
		return "h-4 w-4 shrink-0 text-stone-950"
	}
	return "h-4 w-4 shrink-0 text-stone-400"
}

func uiIcon(name string, className string) g.Node {
	iconClass := h.Class(className)
	switch name {
	case "overview":
		return lucide.House(iconClass)
	case "runs":
		return lucide.ScrollText(iconClass)
	case "files":
		return lucide.FolderSearch(iconClass)
	case "packages", "repo":
		return lucide.Package(iconClass)
	case "lineage", "neighborhood":
		return lucide.GitBranch(iconClass)
	case "source", "file":
		return lucide.FileText(iconClass)
	case "refresh":
		return lucide.RefreshCw(iconClass)
	case "next":
		return lucide.ArrowRight(iconClass)
	default:
		return lucide.BookOpen(iconClass)
	}
}

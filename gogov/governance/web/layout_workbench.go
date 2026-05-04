package web

import (
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func workspaceCanvasClass(extra string) string {
	base := "border border-stone-200 bg-white"
	if extra == "" {
		return base
	}
	return base + " " + extra
}

func graphWorkbenchStage(heightClass string, graph g.Node, inspector g.Node) g.Node {
	return h.Section(
		h.Class(graphWorkbenchStageClass(heightClass)),
		h.Div(
			h.Class("grid h-full min-h-[42rem] lg:min-h-0 lg:grid-cols-[minmax(0,1fr)_24rem] xl:grid-cols-[minmax(0,1fr)_28rem]"),
			h.Div(
				h.Class("min-h-[32rem] border-b border-stone-200 bg-[color:oklch(0.972_0.004_95)] lg:min-h-0 lg:border-b-0 lg:border-r"),
				graph,
			),
			inspector,
		),
	)
}

func graphWorkbenchStageClass(extra string) string {
	base := "relative left-1/2 w-screen max-w-none -translate-x-1/2 overflow-hidden border-y border-stone-200 bg-[color:oklch(0.985_0.004_95)] lg:w-[calc(100vw-18rem)]"
	if extra == "" {
		return base
	}
	return base + " " + extra
}

func graphWorkbenchInspector(children ...g.Node) g.Node {
	return h.Aside(
		h.Class("flex min-h-0 flex-col overflow-y-auto bg-[color:oklch(0.988_0.004_95)]"),
		h.Div(
			h.Class("flex min-h-full flex-col gap-6 px-5 py-5 lg:px-6"),
			g.Group(children),
		),
	)
}

func inspectorSection(eyebrow string, title string, children ...g.Node) g.Node {
	nodes := g.Group{}
	if strings.TrimSpace(eyebrow) != "" {
		nodes = append(nodes, h.P(h.Class("text-[11px] font-semibold uppercase tracking-[0.18em] text-stone-500"), g.Text(eyebrow)))
	}
	if strings.TrimSpace(title) != "" {
		nodes = append(nodes, h.H3(h.Class("mt-2 text-lg font-semibold tracking-[-0.03em] text-stone-950"), g.Text(title)))
	}
	nodes = append(nodes, children...)

	return h.Section(
		h.Class("border-b border-stone-200 pb-6 last:border-b-0 last:pb-0"),
		nodes,
	)
}

func inspectorMetric(label string, value string) g.Node {
	return h.Div(
		h.Class("border-t border-stone-200 pt-3"),
		h.P(h.Class("text-[11px] font-semibold uppercase tracking-[0.18em] text-stone-500"), g.Text(label)),
		h.P(h.Class("mt-2 text-sm font-semibold text-stone-900"), g.Text(value)),
	)
}

func inspectorListStat(label string, value string) g.Node {
	return h.Div(
		h.Class("space-y-1"),
		h.P(g.Text(label)),
		h.P(h.Class("text-sm font-semibold normal-case tracking-normal text-stone-900"), g.Text(value)),
	)
}

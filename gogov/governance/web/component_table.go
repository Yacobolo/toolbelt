package web

import (
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func governanceTable(headers []string, body g.Node) g.Node {
	headerCells := g.Group{}
	for _, header := range headers {
		headerCells = append(headerCells, h.Th(h.Class(governanceTableHeadCellClass()), g.Text(header)))
	}

	return h.Div(
		h.Class("overflow-x-auto"),
		h.Table(
			h.Class("min-w-full border-separate border-spacing-0 text-sm"),
			h.THead(
				h.Tr(headerCells),
			),
			h.TBody(body),
		),
	)
}

func governanceTableHeadCellClass() string {
	return "border-b border-stone-200 px-3 py-3 text-left text-[11px] font-semibold uppercase tracking-[0.18em] text-stone-500 first:pl-0 last:pr-0"
}

func governanceTableCellClass(extra string) string {
	base := "px-3 py-4 align-top text-stone-900 first:pl-0 last:pr-0"
	if strings.TrimSpace(extra) == "" {
		return base
	}
	return base + " " + strings.TrimSpace(extra)
}

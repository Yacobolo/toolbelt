package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type dashboardItem struct {
	source sourcebook.Source
	now    time.Time
}

func (item dashboardItem) FilterValue() string {
	return item.source.Name + " " + item.source.Provider + " " + item.source.DisplayURL()
}

type dashboardDelegate struct {
	nameWidth     int
	providerWidth int
	selected      map[string]struct{}
	status        map[string]string
}

type dashboardColumnLayout struct {
	available     int
	nameWidth     int
	providerWidth int
	updatedWidth  int
	showProvider  bool
	showUpdated   bool
}

const (
	dashboardColumnGap    = 2
	dashboardUpdatedWidth = 12
	dashboardMinNameWidth = 8
	dashboardPrefixWidth  = 3
)

func newDashboardDelegate(
	sources []sourcebook.Source,
	selected map[string]struct{},
	status map[string]string,
) dashboardDelegate {
	delegate := dashboardDelegate{selected: selected, status: status}
	for _, source := range sources {
		delegate.nameWidth = max(delegate.nameWidth, len([]rune(source.Name)))
		delegate.providerWidth = max(delegate.providerWidth, len([]rune(displayProvider(source))))
	}
	return delegate
}

func (d dashboardDelegate) Height() int  { return 1 }
func (d dashboardDelegate) Spacing() int { return 0 }
func (d dashboardDelegate) Update(tea.Msg, *list.Model) tea.Cmd {
	return nil
}

func (d dashboardDelegate) Render(output io.Writer, model list.Model, index int, value list.Item) {
	item, ok := value.(dashboardItem)
	if !ok {
		return
	}

	focused := index == model.Index() && model.FilterState() != list.Filtering
	_, checked := d.selected[item.source.Name]
	cursor := " "
	if focused {
		cursor = "│"
	}
	check := " "
	if checked {
		check = "✓"
	}
	prefix := cursor + check + " "

	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB"))
	if focused {
		nameStyle = nameStyle.Foreground(lipgloss.Color("#22D3EE")).Bold(true)
	}
	markerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))
	if checked {
		markerStyle = markerStyle.Foreground(lipgloss.Color("#22D3EE"))
	}
	providerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	updatedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	gapStyle := lipgloss.NewStyle()
	if focused {
		background := lipgloss.Color("#29283F")
		nameStyle = nameStyle.Background(background)
		markerStyle = markerStyle.Background(background)
		providerStyle = providerStyle.Background(background)
		updatedStyle = updatedStyle.Background(background)
		gapStyle = gapStyle.Background(background)
	}

	provider := displayProvider(item.source)
	updated := "never"
	if !item.source.UpdatedAt.IsZero() {
		updated = relativeTime(item.source.UpdatedAt, item.now)
	}
	if status := d.status[item.source.Name]; status != "" {
		updated = status
	}
	layout := d.columns(model.Width(), provider)

	var row string
	if layout.showProvider {
		row = nameStyle.Render(fmt.Sprintf("%-*s", layout.nameWidth, truncate(item.source.Name, layout.nameWidth))) +
			gapStyle.Render(strings.Repeat(" ", dashboardColumnGap)) +
			providerStyle.Render(fmt.Sprintf("%-*s", layout.providerWidth, provider)) +
			gapStyle.Render(strings.Repeat(" ", dashboardColumnGap)) +
			updatedStyle.Render(fmt.Sprintf(
				"%*s",
				layout.updatedWidth,
				truncate(updated, layout.updatedWidth),
			))
	} else if layout.showUpdated {
		row = nameStyle.Render(fmt.Sprintf("%-*s", layout.nameWidth, truncate(item.source.Name, layout.nameWidth))) +
			gapStyle.Render(strings.Repeat(" ", dashboardColumnGap)) +
			updatedStyle.Render(fmt.Sprintf(
				"%*s",
				layout.updatedWidth,
				truncate(updated, layout.updatedWidth),
			))
	} else {
		row = nameStyle.Render(truncate(item.source.Name, layout.available))
	}

	_, _ = fmt.Fprint(output, markerStyle.Render(prefix), row)
}

func (d dashboardDelegate) Header(width int) string {
	layout := d.columns(width, "")
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Bold(true)
	gap := strings.Repeat(" ", dashboardColumnGap)
	prefix := strings.Repeat(" ", dashboardPrefixWidth)

	switch {
	case layout.showProvider:
		return prefix +
			headerStyle.Render(fmt.Sprintf("%-*s", layout.nameWidth, truncate("NAME", layout.nameWidth))) +
			gap +
			headerStyle.Render(fmt.Sprintf("%-*s", layout.providerWidth, "TYPE")) +
			gap +
			headerStyle.Render(fmt.Sprintf("%*s", layout.updatedWidth, "UPDATED"))
	case layout.showUpdated:
		return prefix +
			headerStyle.Render(fmt.Sprintf("%-*s", layout.nameWidth, truncate("NAME", layout.nameWidth))) +
			gap +
			headerStyle.Render(fmt.Sprintf("%*s", layout.updatedWidth, "UPDATED"))
	default:
		return prefix + headerStyle.Render(truncate("NAME", layout.available))
	}
}

func (d dashboardDelegate) columns(width int, provider string) dashboardColumnLayout {
	available := max(width-dashboardPrefixWidth, 1)
	providerWidth := max(d.providerWidth, len([]rune(provider)))
	layout := dashboardColumnLayout{
		available:     available,
		nameWidth:     min(d.nameWidth, available),
		providerWidth: providerWidth,
		updatedWidth:  dashboardUpdatedWidth,
	}
	fullFixedWidth := dashboardColumnGap +
		providerWidth +
		dashboardColumnGap +
		dashboardUpdatedWidth
	switch {
	case available >= dashboardMinNameWidth+fullFixedWidth:
		layout.nameWidth = min(layout.nameWidth, available-fullFixedWidth)
		layout.showProvider = true
		layout.showUpdated = true
	case available >= dashboardMinNameWidth+dashboardColumnGap+dashboardUpdatedWidth:
		layout.nameWidth = min(
			layout.nameWidth,
			available-dashboardColumnGap-dashboardUpdatedWidth,
		)
		layout.showUpdated = true
	}
	return layout
}

func dashboardItems(sources []sourcebook.Source) []list.Item {
	items := make([]list.Item, len(sources))
	now := time.Now()
	for index, source := range sources {
		items[index] = dashboardItem{source: source, now: now}
	}
	return items
}

func displayProvider(source sourcebook.Source) string {
	switch source.Provider {
	case sourcebook.ProviderGit:
		if source.Preset != "" {
			return "Git docs"
		}
		return "Git"
	case "datastar", "netsuite":
		return "Docs"
	case "":
		return "Source"
	default:
		return source.Provider
	}
}

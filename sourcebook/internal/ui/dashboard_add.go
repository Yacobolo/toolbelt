package ui

import (
	"fmt"
	"strings"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type dashboardAddItem struct {
	presetID    string
	title       string
	description string
	installed   bool
	repository  bool
}

type dashboardAddPaletteModel struct {
	input         textinput.Model
	entries       []sourcebook.CatalogEntry
	installed     map[string]struct{}
	results       []dashboardAddItem
	cursor        int
	width         int
	height        int
	presetID      string
	repositoryURL string
	submitted     bool
	err           error
}

func newDashboardAddPaletteModel(
	entries []sourcebook.CatalogEntry,
	sources []sourcebook.Source,
) dashboardAddPaletteModel {
	input := textinput.New()
	input.Prompt = "> "
	input.Placeholder = "Search catalogue or paste a Git repository URL…"
	input.CharLimit = 2048
	_ = input.Focus()

	installed := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		installed[source.Name] = struct{}{}
	}
	model := dashboardAddPaletteModel{
		input:     input,
		entries:   append([]sourcebook.CatalogEntry(nil), entries...),
		installed: installed,
		width:     64,
		height:    14,
	}
	model.resize(80, 24)
	model.refreshResults()
	return model
}

func (m dashboardAddPaletteModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m dashboardAddPaletteModel) Update(message tea.Msg) (dashboardAddPaletteModel, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.resize(message.Width, message.Height)
		return m, nil
	case tea.KeyPressMsg:
		switch message.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			return m.submit()
		}
	}

	var command tea.Cmd
	m.input, command = m.input.Update(message)
	m.err = nil
	m.refreshResults()
	return m, command
}

func (m *dashboardAddPaletteModel) resize(width, height int) {
	if width <= 48 {
		m.width = max(1, width)
		m.input.Placeholder = "Search or paste Git URL…"
	} else {
		m.width = max(20, min(width-4, 68))
		m.input.Placeholder = "Search catalogue or paste a Git repository URL…"
	}
	m.height = max(7, min(height-4, 20))
	m.input.SetWidth(max(1, m.width-8))
}

func (m *dashboardAddPaletteModel) refreshResults() {
	query := strings.ToLower(strings.TrimSpace(m.input.Value()))
	results := make([]dashboardAddItem, 0, len(m.entries)+1)
	if query == "" || looksLikeRepositoryURL(query) ||
		strings.Contains("git repository url", query) {
		results = append(results, dashboardAddItem{
			title:       "Git repository URL",
			description: "Paste a Git repository URL above and press Enter",
			repository:  true,
		})
	}
	for _, entry := range m.entries {
		searchable := strings.ToLower(
			entry.ID + " " + entry.DisplayName + " " + entry.Description,
		)
		if query != "" && !strings.Contains(searchable, query) {
			continue
		}
		sourceName := entry.SourceName
		if sourceName == "" {
			sourceName = entry.ID
		}
		_, installed := m.installed[sourceName]
		results = append(results, dashboardAddItem{
			presetID:    entry.ID,
			title:       entry.DisplayName,
			description: entry.Description,
			installed:   installed,
		})
	}
	m.results = results
	if len(m.results) == 0 {
		m.cursor = 0
		return
	}
	m.cursor = min(m.cursor, len(m.results)-1)
}

func (m dashboardAddPaletteModel) submit() (dashboardAddPaletteModel, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	if value != "" && looksLikeRepositoryURL(value) {
		normalized, err := sourcebook.ValidateRepositoryURL(value)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.repositoryURL = normalized
		m.submitted = true
		return m, nil
	}
	if len(m.results) == 0 {
		m.err = fmt.Errorf("no matching catalogue sources")
		return m, nil
	}
	selected := m.results[m.cursor]
	if selected.repository {
		m.err = fmt.Errorf("paste a Git repository URL above")
		return m, nil
	}
	if selected.installed {
		m.err = fmt.Errorf("%s is already installed", selected.title)
		return m, nil
	}
	m.presetID = selected.presetID
	m.submitted = true
	return m, nil
}

func looksLikeRepositoryURL(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(value, "://") ||
		strings.HasPrefix(value, "git@") ||
		strings.HasPrefix(value, "ssh:")
}

func (m dashboardAddPaletteModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A78BFA"))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#22D3EE"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Background(lipgloss.Color("#111827")).
		Padding(1, 2).
		Width(m.width).
		Height(m.height)

	var view strings.Builder
	view.WriteString(titleStyle.Render("Add source"))
	view.WriteString("\n\n")
	view.WriteString(m.input.View())
	view.WriteString("\n\n")

	maxResults := max(1, m.height-11)
	if m.err != nil {
		maxResults = max(1, maxResults-1)
	}
	start := 0
	if m.cursor >= maxResults {
		start = m.cursor - maxResults + 1
	}
	end := min(len(m.results), start+maxResults)
	if len(m.results) == 0 {
		view.WriteString(mutedStyle.Render("  No matching sources"))
		view.WriteString("\n")
	} else {
		for index := start; index < end; index++ {
			item := m.results[index]
			marker := "  "
			style := normalStyle
			if index == m.cursor {
				marker = "› "
				style = selectedStyle
			}
			label := item.title
			if item.installed {
				label = "✓ " + label + "  Installed"
				style = mutedStyle
			}
			view.WriteString(marker)
			view.WriteString(style.Render(truncate(label, m.width-8)))
			view.WriteString("\n")
		}
	}

	view.WriteString("\n")
	if len(m.results) > 0 {
		view.WriteString(mutedStyle.Render(truncate(m.results[m.cursor].description, m.width-6)))
		view.WriteString("\n")
	}
	if m.err != nil {
		view.WriteString(errorStyle.Render(truncate(m.err.Error(), m.width-6)))
		view.WriteString("\n")
	}
	footer := "↑/↓ navigate  ·  enter add  ·  esc close"
	if m.width <= 48 {
		footer = "↑/↓ move · enter add · esc close"
	}
	view.WriteString(mutedStyle.Render(footer))
	return borderStyle.Render(view.String())
}

package ui

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type sourceItem struct {
	name        string
	description string
}

func (item sourceItem) Title() string       { return item.name }
func (item sourceItem) Description() string { return item.description }
func (item sourceItem) FilterValue() string { return item.name + " " + item.description }

type sourcePickerModel struct {
	list     list.Model
	source   string
	selected bool
}

func newSourcePickerModel(sources []sourcebook.Source) sourcePickerModel {
	items := make([]list.Item, len(sources))
	for index, source := range sources {
		description := source.Provider
		if source.URL != "" {
			description += " · " + source.URL
		}
		items[index] = sourceItem{name: source.Name, description: description}
	}
	sourceList := newPickerList(items, "Remove source", "source", "sources")
	return sourcePickerModel{list: sourceList}
}

func (m sourcePickerModel) Init() tea.Cmd {
	return nil
}

func (m sourcePickerModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(min(message.Width, 100), min(message.Height, 24))
		return m, nil
	case tea.KeyPressMsg:
		if message.String() == "enter" && m.list.FilterState() != list.Filtering {
			if selected, ok := m.list.SelectedItem().(sourceItem); ok {
				m.source = selected.name
				m.selected = true
				return m, tea.Quit
			}
		}
	}
	var command tea.Cmd
	m.list, command = m.list.Update(message)
	return m, command
}

func (m sourcePickerModel) View() tea.View {
	return tea.NewView(m.list.View())
}

func SelectSource(ctx context.Context, input io.Reader, output io.Writer, sources []sourcebook.Source) (string, bool, error) {
	program := tea.NewProgram(
		newSourcePickerModel(sources),
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithoutSignalHandler(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return "", false, err
	}
	completed, ok := finalModel.(sourcePickerModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected Bubble Tea model %T", finalModel)
	}
	return completed.source, completed.selected, nil
}

type updateSelectionItem struct {
	name        string
	description string
	all         bool
	selected    map[string]struct{}
	total       int
}

func (item updateSelectionItem) Title() string {
	marker := "○"
	_, checked := item.selected[item.name]
	if item.all {
		checked = item.total > 0 && len(item.selected) == item.total
	}
	if checked {
		marker = "●"
	}
	return marker + " " + item.name
}

func (item updateSelectionItem) Description() string { return item.description }
func (item updateSelectionItem) FilterValue() string { return item.name + " " + item.description }

type updateSelectionModel struct {
	list      list.Model
	sources   []sourcebook.Source
	selected  map[string]struct{}
	names     []string
	confirmed bool
}

func newUpdateSelectionModel(sources []sourcebook.Source) updateSelectionModel {
	sources = append([]sourcebook.Source(nil), sources...)
	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
	selected := make(map[string]struct{}, len(sources))
	items := make([]list.Item, 0, len(sources)+1)
	items = append(items, updateSelectionItem{
		name:        "Update all",
		description: fmt.Sprintf("Refresh all %d sources", len(sources)),
		all:         true,
		selected:    selected,
		total:       len(sources),
	})
	for _, source := range sources {
		description := source.Provider
		if !source.UpdatedAt.IsZero() {
			description += " · updated " + source.UpdatedAt.UTC().Format("2006-01-02 15:04 UTC")
		}
		items = append(items, updateSelectionItem{
			name: source.Name, description: description, selected: selected, total: len(sources),
		})
	}
	sourceList := newPickerList(items, "Select sources to update · space toggles · enter confirms", "choice", "choices")
	return updateSelectionModel{
		list:     sourceList,
		sources:  sources,
		selected: selected,
	}
}

func (m updateSelectionModel) Init() tea.Cmd {
	return nil
}

func (m updateSelectionModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(min(message.Width, 100), min(message.Height, 24))
		return m, nil
	case tea.KeyPressMsg:
		if m.list.FilterState() != list.Filtering {
			switch message.String() {
			case " ", "space":
				if selected, ok := m.list.SelectedItem().(updateSelectionItem); ok {
					m.toggle(selected)
					return m, nil
				}
			case "enter":
				if len(m.selected) == 0 {
					if selected, ok := m.list.SelectedItem().(updateSelectionItem); ok {
						m.toggle(selected)
					}
				}
				if len(m.selected) > 0 {
					m.names = m.selectedNames()
					m.confirmed = true
					return m, tea.Quit
				}
			}
		}
	}
	var command tea.Cmd
	m.list, command = m.list.Update(message)
	return m, command
}

func (m updateSelectionModel) View() tea.View {
	return tea.NewView(m.list.View())
}

func (m *updateSelectionModel) toggle(item updateSelectionItem) {
	if item.all {
		if len(m.selected) == len(m.sources) {
			clear(m.selected)
			return
		}
		for _, source := range m.sources {
			m.selected[source.Name] = struct{}{}
		}
		return
	}
	if _, exists := m.selected[item.name]; exists {
		delete(m.selected, item.name)
		return
	}
	m.selected[item.name] = struct{}{}
}

func (m updateSelectionModel) selectedNames() []string {
	names := make([]string, 0, len(m.selected))
	for _, source := range m.sources {
		if _, exists := m.selected[source.Name]; exists {
			names = append(names, source.Name)
		}
	}
	return names
}

func SelectSourcesForUpdate(ctx context.Context, input io.Reader, output io.Writer, sources []sourcebook.Source) ([]string, bool, error) {
	program := tea.NewProgram(
		newUpdateSelectionModel(sources),
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithoutSignalHandler(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return nil, false, err
	}
	completed, ok := finalModel.(updateSelectionModel)
	if !ok {
		return nil, false, fmt.Errorf("unexpected Bubble Tea model %T", finalModel)
	}
	return completed.names, completed.confirmed, nil
}

type presetItem struct {
	id          string
	displayName string
	description string
}

func (item presetItem) Title() string       { return item.displayName }
func (item presetItem) Description() string { return item.description }
func (item presetItem) FilterValue() string {
	return item.id + " " + item.displayName + " " + item.description
}

type presetPickerModel struct {
	list     list.Model
	preset   string
	selected bool
}

func newPresetPickerModel(entries []sourcebook.CatalogEntry) presetPickerModel {
	items := make([]list.Item, len(entries))
	for index, entry := range entries {
		items[index] = presetItem{
			id:          entry.ID,
			displayName: entry.DisplayName,
			description: entry.Description,
		}
	}
	presetList := newPickerList(items, "Add source", "source", "sources")
	return presetPickerModel{list: presetList}
}

func (m presetPickerModel) Init() tea.Cmd {
	return nil
}

func (m presetPickerModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(min(message.Width, 100), min(message.Height, 24))
		return m, nil
	case tea.KeyPressMsg:
		if message.String() == "enter" && m.list.FilterState() != list.Filtering {
			if selected, ok := m.list.SelectedItem().(presetItem); ok {
				m.preset = selected.id
				m.selected = true
				return m, tea.Quit
			}
		}
	}
	var command tea.Cmd
	m.list, command = m.list.Update(message)
	return m, command
}

func (m presetPickerModel) View() tea.View {
	return tea.NewView(m.list.View())
}

func SelectPreset(ctx context.Context, input io.Reader, output io.Writer, entries []sourcebook.CatalogEntry) (string, bool, error) {
	program := tea.NewProgram(
		newPresetPickerModel(entries),
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithoutSignalHandler(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return "", false, err
	}
	completed, ok := finalModel.(presetPickerModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected Bubble Tea model %T", finalModel)
	}
	return completed.preset, completed.selected, nil
}

func newPickerList(items []list.Item, title, singular, plural string) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		BorderForeground(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#22D3EE"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		BorderForeground(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#9CA3AF"))

	picker := list.New(items, delegate, 80, 20)
	picker.Title = title
	picker.SetStatusBarItemName(singular, plural)
	picker.Styles.Title = picker.Styles.Title.
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)
	return picker
}

package ui

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"

	"charm.land/bubbles/v2/key"
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
		if displayURL := source.DisplayURL(); displayURL != "" {
			description += " · " + displayURL
		}
		items[index] = sourceItem{name: source.Name, description: description}
	}
	sourceList := newPickerList(items, "Remove source", "source", "sources")
	sourceList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{pickerChooseKey}
	}
	sourceList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{pickerChooseKey}
	}
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

type removeConfirmationModel struct {
	source    sourcebook.Source
	confirmed bool
}

func newRemoveConfirmationModel(source sourcebook.Source) removeConfirmationModel {
	return removeConfirmationModel{source: source}
}

func (m removeConfirmationModel) Init() tea.Cmd { return nil }

func (m removeConfirmationModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.KeyPressMsg:
		switch message.String() {
		case "y", "Y":
			m.confirmed = true
			return m, tea.Quit
		case "enter", "n", "N", "esc", "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m removeConfirmationModel) View() tea.View {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444"))
	name := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#22D3EE"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	var view strings.Builder
	view.WriteString(title.Render("Remove " + m.source.Name + "?"))
	view.WriteString("\n\n")
	view.WriteString("This removes ")
	view.WriteString(name.Render(m.source.Name))
	view.WriteString(" from the local Sourcebook skill.\n")
	if displayURL := m.source.DisplayURL(); displayURL != "" {
		view.WriteString(muted.Render(displayURL))
		view.WriteString("\n")
	}
	view.WriteString(muted.Render("The upstream source is unaffected."))
	view.WriteString("\n\n")
	view.WriteString(title.Render("y"))
	view.WriteString(" remove  ")
	view.WriteString(muted.Render("enter/n/esc cancel"))
	view.WriteString("\n")
	return tea.NewView(view.String())
}

// ConfirmRemoval asks for explicit confirmation before deleting a source chosen
// from the interactive picker. Enter defaults to cancel.
func ConfirmRemoval(ctx context.Context, input io.Reader, output io.Writer, source sourcebook.Source) (bool, error) {
	program := tea.NewProgram(
		newRemoveConfirmationModel(source),
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithoutSignalHandler(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return false, err
	}
	completed, ok := finalModel.(removeConfirmationModel)
	if !ok {
		return false, fmt.Errorf("unexpected Bubble Tea model %T", finalModel)
	}
	return completed.confirmed, nil
}

type updateSelectionItem struct {
	name        string
	description string
	selected    map[string]struct{}
}

func (item updateSelectionItem) Title() string {
	marker := "○"
	_, checked := item.selected[item.name]
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
	items := make([]list.Item, 0, len(sources))
	for _, source := range sources {
		description := source.Provider
		if !source.UpdatedAt.IsZero() {
			description += " · updated " + source.UpdatedAt.UTC().Format("2006-01-02 15:04 UTC")
		}
		items = append(items, updateSelectionItem{
			name: source.Name, description: description, selected: selected,
		})
	}
	sourceList := newPickerList(items, "Update sources", "source", "sources")
	sourceList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{updateToggleKey, updateAllKey, updateConfirmKey}
	}
	sourceList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{updateToggleKey, updateAllKey, updateConfirmKey}
	}
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
			switch {
			case key.Matches(message, updateToggleKey):
				if selected, ok := m.list.SelectedItem().(updateSelectionItem); ok {
					m.toggle(selected)
					return m, nil
				}
			case key.Matches(message, updateAllKey):
				m.toggleAll()
				return m, nil
			case key.Matches(message, updateConfirmKey):
				if len(m.selected) > 0 {
					m.names = m.selectedNames()
					m.confirmed = true
					return m, tea.Quit
				}
				return m, m.list.NewStatusMessage("Select at least one source")
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
	if _, exists := m.selected[item.name]; exists {
		delete(m.selected, item.name)
		return
	}
	m.selected[item.name] = struct{}{}
}

func (m *updateSelectionModel) toggleAll() {
	if len(m.selected) == len(m.sources) {
		clear(m.selected)
		return
	}
	for _, source := range m.sources {
		m.selected[source.Name] = struct{}{}
	}
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
	installed   bool
}

// GitRepositorySelection is returned by the add picker when the user wants to
// enter an ad-hoc Git repository URL.
const GitRepositorySelection = "__git_repository_url__"

func (item presetItem) Title() string {
	if item.installed {
		return "✓ " + item.displayName + "  Installed"
	}
	return item.displayName
}
func (item presetItem) Description() string { return item.description }
func (item presetItem) FilterValue() string {
	return item.id + " " + item.displayName + " " + item.description
}

type presetPickerModel struct {
	list     list.Model
	preset   string
	selected bool
}

func newPresetPickerModel(entries []sourcebook.CatalogEntry, installed map[string]struct{}) presetPickerModel {
	items := make([]list.Item, len(entries)+1)
	items[0] = presetItem{
		id:          GitRepositorySelection,
		displayName: "Git repository URL",
		description: "Clone any Git repository into Sourcebook",
	}
	for index, entry := range entries {
		sourceName := entry.SourceName
		if sourceName == "" {
			sourceName = entry.ID
		}
		_, exists := installed[sourceName]
		items[index+1] = presetItem{
			id:          entry.ID,
			displayName: entry.DisplayName,
			description: entry.Description,
			installed:   exists,
		}
	}
	presetList := newPickerList(items, "Add source", "source", "sources")
	presetList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{pickerChooseKey}
	}
	presetList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{pickerChooseKey}
	}
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
				if selected.installed {
					return m, nil
				}
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

func SelectPreset(
	ctx context.Context,
	input io.Reader,
	output io.Writer,
	entries []sourcebook.CatalogEntry,
	sources []sourcebook.Source,
) (string, bool, error) {
	installed := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		installed[source.Name] = struct{}{}
	}
	program := tea.NewProgram(
		newPresetPickerModel(entries, installed),
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

var (
	updateToggleKey = key.NewBinding(
		key.WithKeys(" ", "space"),
		key.WithHelp("space", "toggle"),
	)
	updateAllKey = key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "all"),
	)
	updateConfirmKey = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "update"),
	)
	pickerChooseKey = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "choose"),
	)
)

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

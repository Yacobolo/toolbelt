package ui

import (
	"context"
	"fmt"
	"io"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type repositoryItem sourcebook.Repository

func (item repositoryItem) Title() string       { return item.Name }
func (item repositoryItem) Description() string { return item.URL }
func (item repositoryItem) FilterValue() string { return item.Name + " " + item.URL }

type repositoryPickerModel struct {
	list       list.Model
	repository string
	selected   bool
}

func newRepositoryPickerModel(repositories []sourcebook.Repository) repositoryPickerModel {
	items := make([]list.Item, len(repositories))
	for index, repository := range repositories {
		items[index] = repositoryItem(repository)
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		BorderForeground(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#22D3EE"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		BorderForeground(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#9CA3AF"))

	repositoryList := list.New(items, delegate, 80, 20)
	repositoryList.Title = "Remove repository"
	repositoryList.SetStatusBarItemName("repository", "repositories")
	repositoryList.Styles.Title = repositoryList.Styles.Title.
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	return repositoryPickerModel{list: repositoryList}
}

func (m repositoryPickerModel) Init() tea.Cmd {
	return nil
}

func (m repositoryPickerModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(min(message.Width, 100), min(message.Height, 24))
		return m, nil
	case tea.KeyPressMsg:
		if message.String() == "enter" && m.list.FilterState() != list.Filtering {
			if selected, ok := m.list.SelectedItem().(repositoryItem); ok {
				m.repository = selected.Name
				m.selected = true
				return m, tea.Quit
			}
		}
	}

	var command tea.Cmd
	m.list, command = m.list.Update(message)
	return m, command
}

func (m repositoryPickerModel) View() tea.View {
	return tea.NewView(m.list.View())
}

func SelectRepository(ctx context.Context, input io.Reader, output io.Writer, repositories []sourcebook.Repository) (string, bool, error) {
	program := tea.NewProgram(
		newRepositoryPickerModel(repositories),
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithoutSignalHandler(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return "", false, err
	}
	completed, ok := finalModel.(repositoryPickerModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected Bubble Tea model %T", finalModel)
	}
	return completed.repository, completed.selected, nil
}

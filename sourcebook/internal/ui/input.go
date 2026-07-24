package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type repositoryInputModel struct {
	input         textinput.Model
	repositoryURL string
	submitted     bool
}

func newRepositoryInputModel() repositoryInputModel {
	input := textinput.New()
	input.Prompt = "Repository URL  "
	input.Placeholder = "https://github.com/owner/repository.git"
	input.CharLimit = 2048
	input.SetWidth(72)
	input.Validate = func(value string) error {
		if strings.TrimSpace(value) == "" {
			return nil
		}
		_, err := sourcebook.ValidateRepositoryURL(value)
		return err
	}
	return repositoryInputModel{input: input}
}

func (m repositoryInputModel) Init() tea.Cmd {
	return m.input.Focus()
}

func (m repositoryInputModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.input.SetWidth(max(20, min(message.Width-18, 90)))
	case tea.KeyPressMsg:
		switch message.String() {
		case "enter":
			value := strings.TrimSpace(m.input.Value())
			if value == "" {
				m.input.Err = errors.New("repository URL is required")
				return m, nil
			}
			normalized, err := sourcebook.ValidateRepositoryURL(value)
			if err != nil {
				m.input.Err = err
				return m, nil
			}
			m.repositoryURL = normalized
			m.submitted = true
			return m, tea.Quit
		case "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	var command tea.Cmd
	m.input, command = m.input.Update(message)
	return m, command
}

func (m repositoryInputModel) View() tea.View {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	failure := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	var view strings.Builder
	view.WriteString(title.Render("Add Git repository"))
	view.WriteString("\n\n")
	view.WriteString(m.input.View())
	view.WriteString("\n")
	if m.input.Err != nil {
		view.WriteString(failure.Render(m.input.Err.Error()))
		view.WriteString("\n")
	}
	view.WriteString(muted.Render("enter add  ·  esc cancel"))
	view.WriteString("\n")
	return tea.NewView(view.String())
}

// InputRepositoryURL prompts for an ad-hoc Git repository URL.
func InputRepositoryURL(ctx context.Context, input io.Reader, output io.Writer) (string, bool, error) {
	program := tea.NewProgram(
		newRepositoryInputModel(),
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithoutSignalHandler(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return "", false, err
	}
	completed, ok := finalModel.(repositoryInputModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected Bubble Tea model %T", finalModel)
	}
	return completed.repositoryURL, completed.submitted, nil
}

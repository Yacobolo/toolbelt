package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

type Action struct {
	Working string
	Success string
	Run     func(context.Context) error
}

type reportedError struct {
	err error
}

func (e reportedError) Error() string {
	return e.err.Error()
}

func (e reportedError) Unwrap() error {
	return e.err
}

func markReported(err error) error {
	if err == nil {
		return nil
	}
	return reportedError{err: err}
}

func WasReported(err error) bool {
	var reported reportedError
	return errors.As(err, &reported)
}

type actionFinishedMsg struct {
	err error
}

type actionModel struct {
	ctx     context.Context
	action  Action
	spinner spinner.Model
	done    bool
	err     error
}

func newActionModel(action Action) actionModel {
	indicator := spinner.New(spinner.WithSpinner(spinner.Dot))
	indicator.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))
	return actionModel{
		ctx:     context.Background(),
		action:  action,
		spinner: indicator,
	}
}

func (m actionModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		if m.action.Run == nil {
			return actionFinishedMsg{}
		}
		return actionFinishedMsg{err: m.action.Run(m.ctx)}
	})
}

func (m actionModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case actionFinishedMsg:
		m.done = true
		m.err = message.err
		return m, tea.Quit
	case spinner.TickMsg:
		var command tea.Cmd
		m.spinner, command = m.spinner.Update(message)
		return m, command
	default:
		return m, nil
	}
}

func (m actionModel) View() tea.View {
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")).Bold(true)
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E")).Bold(true)
	failure := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	if m.done {
		if m.err != nil {
			content := failure.Render("✗") + " " + m.action.Working + " failed\n"
			content += muted.Render("  "+m.err.Error()) + "\n"
			return tea.NewView(content)
		}
		return tea.NewView(success.Render("✓") + " " + m.action.Success + "\n")
	}

	content := m.spinner.View() + " " + accent.Render(m.action.Working) + muted.Render("  ctrl+c to cancel")
	return tea.NewView(content)
}

func RunAction(ctx context.Context, input io.Reader, output io.Writer, interactive bool, action Action) error {
	if !interactive {
		if _, err := fmt.Fprintf(output, "%s...\n", action.Working); err != nil {
			return err
		}
		if err := action.Run(ctx); err != nil {
			return err
		}
		_, err := fmt.Fprintf(output, "%s.\n", action.Success)
		return err
	}

	model := newActionModel(action)
	model.ctx = ctx
	program := tea.NewProgram(
		model,
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithoutSignalHandler(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return err
	}
	completed, ok := finalModel.(actionModel)
	if !ok {
		return fmt.Errorf("unexpected Bubble Tea model %T", finalModel)
	}
	return markReported(completed.err)
}

func RenderSources(sources []sourcebook.Source, color bool) string {
	sources = append([]sourcebook.Source(nil), sources...)
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Name < sources[j].Name
	})

	title := lipgloss.NewStyle()
	subtitle := lipgloss.NewStyle()
	header := lipgloss.NewStyle()
	name := lipgloss.NewStyle()
	url := lipgloss.NewStyle()
	hint := lipgloss.NewStyle()
	if color {
		title = title.Bold(true).Foreground(lipgloss.Color("#7C3AED"))
		subtitle = subtitle.Foreground(lipgloss.Color("#6B7280"))
		header = header.Bold(true).Foreground(lipgloss.Color("#A78BFA"))
		name = name.Bold(true).Foreground(lipgloss.Color("#22D3EE"))
		url = url.Foreground(lipgloss.Color("#9CA3AF"))
		hint = hint.Foreground(lipgloss.Color("#6B7280"))
	}

	var view strings.Builder
	view.WriteString(title.Render("Sourcebook"))
	view.WriteString("\n")
	if len(sources) == 0 {
		view.WriteString(subtitle.Render("No sources yet."))
		view.WriteString("\n\n")
		view.WriteString(hint.Render("Add Git with sourcebook add <repository-url>, or run sourcebook add"))
		view.WriteString("\n")
		return view.String()
	}

	count := fmt.Sprintf("%d sources", len(sources))
	if len(sources) == 1 {
		count = "1 source"
	}
	view.WriteString(subtitle.Render(count))
	view.WriteString("\n\n")

	nameWidth := len("NAME")
	providerWidth := len("TYPE")
	urlWidth := len("SOURCE")
	for _, source := range sources {
		if len(source.Name) > nameWidth {
			nameWidth = len(source.Name)
		}
		if len(source.Provider) > providerWidth {
			providerWidth = len(source.Provider)
		}
		if len(source.URL) > urlWidth {
			urlWidth = len(source.URL)
		}
	}
	fmt.Fprintf(&view, "%s  %s  %s  %s\n",
		header.Render(fmt.Sprintf("%-*s", nameWidth, "NAME")),
		header.Render(fmt.Sprintf("%-*s", providerWidth, "TYPE")),
		header.Render(fmt.Sprintf("%-*s", urlWidth, "SOURCE")),
		header.Render("LAST UPDATED"),
	)
	for _, source := range sources {
		fmt.Fprintf(&view, "%s  %s  %s  %s\n",
			name.Render(fmt.Sprintf("%-*s", nameWidth, source.Name)),
			hint.Render(fmt.Sprintf("%-*s", providerWidth, source.Provider)),
			url.Render(fmt.Sprintf("%-*s", urlWidth, source.URL)),
			hint.Render(formatUpdatedAt(source.UpdatedAt)),
		)
	}
	return view.String()
}

func formatUpdatedAt(updatedAt time.Time) string {
	if updatedAt.IsZero() {
		return "never"
	}
	return updatedAt.UTC().Format("2006-01-02 15:04 UTC")
}

func RenderHelp(color bool) string {
	title := lipgloss.NewStyle()
	heading := lipgloss.NewStyle()
	command := lipgloss.NewStyle()
	description := lipgloss.NewStyle()
	if color {
		title = title.Bold(true).Foreground(lipgloss.Color("#7C3AED"))
		heading = heading.Bold(true).Foreground(lipgloss.Color("#A78BFA"))
		command = command.Foreground(lipgloss.Color("#22D3EE"))
		description = description.Foreground(lipgloss.Color("#9CA3AF"))
	}

	var view strings.Builder
	view.WriteString(title.Render("Sourcebook"))
	view.WriteString("\n")
	view.WriteString(description.Render("Build one Codex skill from Git repositories and documentation sources."))
	view.WriteString("\n\n")
	view.WriteString(heading.Render("Usage"))
	view.WriteString("\n")
	rows := [][2]string{
		{"sourcebook add <repository-url>", "Add a Git repository"},
		{"sourcebook add", "Select a catalogue source"},
		{"sourcebook update", "Select sources to refresh"},
		{"sourcebook remove [name]", "Select and remove a source"},
		{"sourcebook list", "List sources"},
		{"sourcebook upgrade", "Upgrade Sourcebook"},
		{"sourcebook version", "Print the version"},
	}
	for _, row := range rows {
		fmt.Fprintf(&view, "  %s  %s\n", command.Render(fmt.Sprintf("%-36s", row[0])), description.Render(row[1]))
	}
	return view.String()
}

func IsTerminal(output io.Writer) bool {
	file, ok := output.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

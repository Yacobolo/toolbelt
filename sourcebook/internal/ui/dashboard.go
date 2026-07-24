package ui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// DashboardAction is the interactive action selected from the Sourcebook home
// screen.
type DashboardAction int

const (
	DashboardQuit DashboardAction = iota
	DashboardAdd
	DashboardUpdate
	DashboardRemove
)

type dashboardItem struct {
	source sourcebook.Source
	now    time.Time
}

func (item dashboardItem) Title() string { return item.source.Name }
func (item dashboardItem) Description() string {
	description := displayProvider(item.source)
	if !item.source.UpdatedAt.IsZero() {
		description += " · updated " + relativeTime(item.source.UpdatedAt, item.now)
	}
	return description
}
func (item dashboardItem) FilterValue() string {
	return item.source.Name + " " + item.source.Provider + " " + item.source.URL
}

type dashboardModel struct {
	list      list.Model
	version   string
	skillDir  string
	action    DashboardAction
	hasSource bool
	width     int
}

func newDashboardModel(version, skillDir string, sources []sourcebook.Source) dashboardModel {
	items := make([]list.Item, len(sources))
	now := time.Now()
	for index, source := range sources {
		items[index] = dashboardItem{source: source, now: now}
	}
	sourceList := newPickerList(items, "", "source", "sources")
	sourceList.SetShowTitle(false)
	sourceList.SetShowHelp(true)
	hasSources := len(sources) > 0
	if !hasSources {
		sourceList.SetShowStatusBar(false)
	}
	sourceList.AdditionalShortHelpKeys = func() []key.Binding {
		keys := []key.Binding{dashboardAddKey}
		if hasSources {
			keys = append(keys, dashboardUpdateKey, dashboardRemoveKey)
		}
		return keys
	}
	sourceList.AdditionalFullHelpKeys = func() []key.Binding {
		keys := []key.Binding{dashboardAddKey}
		if hasSources {
			keys = append(keys, dashboardUpdateKey, dashboardRemoveKey)
		}
		return keys
	}
	return dashboardModel{
		list:      sourceList,
		version:   version,
		skillDir:  skillDir,
		hasSource: hasSources,
		width:     80,
	}
}

func (m dashboardModel) Init() tea.Cmd { return nil }

func (m dashboardModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width = max(message.Width, 1)
		m.list.SetSize(min(m.width, 100), max(1, min(message.Height-7, 24)))
		return m, nil
	case tea.KeyPressMsg:
		if m.list.FilterState() != list.Filtering {
			switch message.String() {
			case "a":
				m.action = DashboardAdd
				return m, tea.Quit
			case "u":
				if m.hasSource {
					m.action = DashboardUpdate
					return m, tea.Quit
				}
				return m, m.list.NewStatusMessage("Add a source before updating")
			case "r":
				if m.hasSource {
					m.action = DashboardRemove
					return m, tea.Quit
				}
				return m, m.list.NewStatusMessage("No sources to remove")
			}
		}
	}
	var command tea.Cmd
	m.list, command = m.list.Update(message)
	return m, command
}

func (m dashboardModel) View() tea.View {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	version := strings.TrimSpace(m.version)
	if version != "" {
		version = " " + version
	}
	var view strings.Builder
	view.WriteString(title.Render("Sourcebook" + version))
	view.WriteString("\n")
	view.WriteString(muted.Render(truncate(m.skillDir, m.width)))
	view.WriteString("\n\n")
	view.WriteString(m.list.View())
	view.WriteString("\n")
	return tea.NewView(view.String())
}

func relativeTime(value, now time.Time) string {
	elapsed := now.Sub(value)
	if elapsed < 0 {
		elapsed = 0
	}
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	case elapsed < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	default:
		return value.UTC().Format("2006-01-02")
	}
}

var (
	dashboardAddKey = key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add"),
	)
	dashboardUpdateKey = key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "update"),
	)
	dashboardRemoveKey = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "remove"),
	)
)

// SelectDashboardAction runs the interactive Sourcebook home screen.
func SelectDashboardAction(
	ctx context.Context,
	input io.Reader,
	output io.Writer,
	version string,
	skillDir string,
	sources []sourcebook.Source,
) (DashboardAction, error) {
	program := tea.NewProgram(
		newDashboardModel(version, skillDir, sources),
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithoutSignalHandler(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return DashboardQuit, err
	}
	completed, ok := finalModel.(dashboardModel)
	if !ok {
		return DashboardQuit, fmt.Errorf("unexpected Bubble Tea model %T", finalModel)
	}
	return completed.action, nil
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

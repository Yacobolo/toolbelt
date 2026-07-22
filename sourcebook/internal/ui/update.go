package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type UpdateAction func(context.Context, sourcebook.UpdateReporter) error

type updateProgressMsg struct {
	event sourcebook.UpdateEvent
}

type updateFinishedMsg struct {
	err error
}

type repositoryProgress struct {
	repository sourcebook.Repository
	state      sourcebook.UpdateState
	started    time.Time
	duration   time.Duration
	err        error
}

type updateModel struct {
	ctx          context.Context
	cancel       context.CancelFunc
	run          UpdateAction
	messages     chan tea.Msg
	spinner      spinner.Model
	repositories map[string]repositoryProgress
	started      time.Time
	duration     time.Duration
	done         bool
	canceling    bool
	installing   bool
	err          error
}

func newUpdateModel(ctx context.Context, repositories []sourcebook.Repository, run UpdateAction) updateModel {
	ctx, cancel := context.WithCancel(ctx)
	indicator := spinner.New(spinner.WithSpinner(spinner.Dot))
	indicator.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))
	progress := make(map[string]repositoryProgress, len(repositories))
	for _, repository := range repositories {
		progress[repository.Name] = repositoryProgress{repository: repository}
	}
	return updateModel{
		ctx:          ctx,
		cancel:       cancel,
		run:          run,
		messages:     make(chan tea.Msg, len(repositories)*3+1),
		spinner:      indicator,
		repositories: progress,
		started:      time.Now(),
	}
}

func (m updateModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.startUpdate(), waitForUpdateMessage(m.messages))
}

func (m updateModel) startUpdate() tea.Cmd {
	return func() tea.Msg {
		var err error
		if m.run != nil {
			err = m.run(m.ctx, func(event sourcebook.UpdateEvent) {
				select {
				case m.messages <- updateProgressMsg{event: event}:
				case <-m.ctx.Done():
				}
			})
		}
		m.messages <- updateFinishedMsg{err: err}
		return nil
	}
}

func waitForUpdateMessage(messages <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-messages
	}
}

func (m updateModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case updateProgressMsg:
		if message.event.State == sourcebook.UpdateInstalling {
			m.installing = true
			return m, waitForUpdateMessage(m.messages)
		}
		progress, ok := m.repositories[message.event.Repository]
		if !ok {
			return m, waitForUpdateMessage(m.messages)
		}
		progress.state = message.event.State
		progress.duration = message.event.Duration
		progress.err = message.event.Err
		if message.event.State == sourcebook.UpdateCloning {
			progress.started = time.Now()
		}
		m.repositories[message.event.Repository] = progress
		return m, waitForUpdateMessage(m.messages)
	case updateFinishedMsg:
		m.done = true
		m.err = message.err
		m.duration = time.Since(m.started)
		m.cancel()
		return m, tea.Quit
	case spinner.TickMsg:
		var command tea.Cmd
		m.spinner, command = m.spinner.Update(message)
		return m, command
	case tea.KeyPressMsg:
		if message.String() == "ctrl+c" && !m.canceling {
			m.canceling = true
			m.cancel()
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m updateModel) View() tea.View {
	accent := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	success := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#22C55E"))
	failure := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444"))
	active := lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE"))

	if m.done {
		if m.err == nil {
			text := fmt.Sprintf("✓ Updated %d repositories in %s\n", len(m.repositories), formatDuration(m.duration))
			return tea.NewView(success.Render(text))
		}
		label := "Update failed"
		if errors.Is(m.err, context.Canceled) {
			label = "Update canceled"
		} else if failed := m.failedRepository(); failed != "" {
			label = failed + " failed"
		}
		var view strings.Builder
		view.WriteString(failure.Render("✗ " + label))
		view.WriteString(" after ")
		view.WriteString(formatDuration(m.duration))
		view.WriteString("\n")
		view.WriteString(muted.Render("  " + m.err.Error()))
		view.WriteString("\n\n")
		view.WriteString(muted.Render("Existing references were left unchanged."))
		view.WriteString("\n")
		return tea.NewView(view.String())
	}

	repositories := m.sortedProgress()
	completed := 0
	nameWidth := len("repository")
	for _, progress := range repositories {
		if progress.state == sourcebook.UpdateCompleted {
			completed++
		}
		if len(progress.repository.Name) > nameWidth {
			nameWidth = len(progress.repository.Name)
		}
	}

	var view strings.Builder
	view.WriteString(accent.Render("Sourcebook update"))
	view.WriteString("\n")
	fmt.Fprintf(&view, "%s · %s elapsed\n\n", muted.Render(fmt.Sprintf("%d/%d repositories completed", completed, len(repositories))), formatDuration(time.Since(m.started)))
	for _, progress := range repositories {
		marker := muted.Render("·")
		detail := muted.Render("queued")
		switch progress.state {
		case sourcebook.UpdateCompleted:
			marker = success.Render("✓")
			detail = muted.Render(formatDuration(progress.duration))
		case sourcebook.UpdateCloning:
			marker = m.spinner.View()
			detail = active.Render("cloning") + muted.Render(" · "+formatDuration(time.Since(progress.started)))
		case sourcebook.UpdateFailed:
			marker = failure.Render("✗")
			detail = failure.Render("failed") + muted.Render(" · "+formatDuration(progress.duration))
		case sourcebook.UpdateCanceled:
			marker = muted.Render("–")
			detail = muted.Render("canceled")
		}
		fmt.Fprintf(&view, "%s %-*s  %s\n", marker, nameWidth, progress.repository.Name, detail)
	}
	view.WriteString("\n")
	if m.installing {
		view.WriteString(m.spinner.View())
		view.WriteString(" ")
		view.WriteString(accent.Render("Installing updated references…"))
	} else if m.canceling {
		view.WriteString(muted.Render("Canceling active clones…"))
	} else {
		view.WriteString(muted.Render("ctrl+c to cancel"))
	}
	return tea.NewView(view.String())
}

func (m updateModel) failedRepository() string {
	for _, progress := range m.repositories {
		if progress.state == sourcebook.UpdateFailed {
			return progress.repository.Name
		}
	}
	return ""
}

func (m updateModel) sortedProgress() []repositoryProgress {
	progress := make([]repositoryProgress, 0, len(m.repositories))
	for _, repository := range m.repositories {
		progress = append(progress, repository)
	}
	sort.Slice(progress, func(i, j int) bool {
		leftRank := updateStateRank(progress[i].state)
		rightRank := updateStateRank(progress[j].state)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return progress[i].repository.Name < progress[j].repository.Name
	})
	return progress
}

func updateStateRank(state sourcebook.UpdateState) int {
	switch state {
	case sourcebook.UpdateCompleted:
		return 0
	case sourcebook.UpdateCloning:
		return 1
	case sourcebook.UpdateFailed:
		return 2
	case sourcebook.UpdateCanceled:
		return 3
	default:
		return 4
	}
}

func formatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	if duration < time.Minute {
		return fmt.Sprintf("%.1fs", duration.Seconds())
	}
	return fmt.Sprintf("%dm%02ds", int(duration.Minutes()), int(duration.Seconds())%60)
}

func RunUpdate(ctx context.Context, input io.Reader, output io.Writer, interactive bool, repositories []sourcebook.Repository, run UpdateAction) error {
	if !interactive {
		return RunAction(ctx, input, output, false, Action{
			Working: "Refreshing repositories",
			Success: "Sourcebook updated",
			Run: func(ctx context.Context) error {
				return run(ctx, nil)
			},
		})
	}

	model := newUpdateModel(ctx, repositories, run)
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
	completed, ok := finalModel.(updateModel)
	if !ok {
		return fmt.Errorf("unexpected Bubble Tea model %T", finalModel)
	}
	return markReported(completed.err)
}

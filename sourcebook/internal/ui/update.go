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

type sourceProgress struct {
	source   sourcebook.Source
	state    sourcebook.UpdateState
	phase    string
	current  int
	total    int
	started  time.Time
	duration time.Duration
	err      error
}

type updateModel struct {
	ctx         context.Context
	cancel      context.CancelFunc
	run         UpdateAction
	messages    chan tea.Msg
	spinner     spinner.Model
	sources     map[string]sourceProgress
	started     time.Time
	duration    time.Duration
	done        bool
	canceling   bool
	installing  bool
	err         error
	title       string
	successVerb string
}

func newUpdateModel(ctx context.Context, sources []sourcebook.Source, run UpdateAction) updateModel {
	ctx, cancel := context.WithCancel(ctx)
	indicator := spinner.New(spinner.WithSpinner(spinner.Dot))
	indicator.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))
	progress := make(map[string]sourceProgress, len(sources))
	for _, source := range sources {
		progress[source.Name] = sourceProgress{source: source}
	}
	return updateModel{
		ctx:         ctx,
		cancel:      cancel,
		run:         run,
		messages:    make(chan tea.Msg, len(sources)*4+1),
		spinner:     indicator,
		sources:     progress,
		started:     time.Now(),
		title:       "Sourcebook update",
		successVerb: "Updated",
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
		progress, ok := m.sources[message.event.Source]
		if !ok {
			return m, waitForUpdateMessage(m.messages)
		}
		progress.state = message.event.State
		progress.phase = message.event.Phase
		progress.current = message.event.Current
		progress.total = message.event.Total
		progress.duration = message.event.Duration
		progress.err = message.event.Err
		if message.event.State == sourcebook.UpdateRunning && progress.started.IsZero() {
			progress.started = time.Now()
		}
		m.sources[message.event.Source] = progress
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
			text := fmt.Sprintf("✓ %s %d sources in %s\n", m.successVerb, len(m.sources), formatDuration(m.duration))
			return tea.NewView(success.Render(text))
		}
		label := "Update failed"
		if errors.Is(m.err, context.Canceled) {
			label = "Update canceled"
		} else if failed := m.failedSource(); failed != "" {
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

	sources := m.sortedProgress()
	completed := 0
	nameWidth := len("source")
	for _, progress := range sources {
		if progress.state == sourcebook.UpdateCompleted {
			completed++
		}
		if len(progress.source.Name) > nameWidth {
			nameWidth = len(progress.source.Name)
		}
	}

	var view strings.Builder
	view.WriteString(accent.Render(m.title))
	view.WriteString("\n")
	fmt.Fprintf(&view, "%s · %s elapsed\n\n", muted.Render(fmt.Sprintf("%d/%d sources completed", completed, len(sources))), formatDuration(time.Since(m.started)))
	for _, progress := range sources {
		marker := muted.Render("·")
		detail := muted.Render("queued")
		switch progress.state {
		case sourcebook.UpdateCompleted:
			marker = success.Render("✓")
			detail = muted.Render(formatDuration(progress.duration))
		case sourcebook.UpdateRunning:
			marker = m.spinner.View()
			phase := progress.phase
			if phase == "" {
				phase = "updating"
			}
			detail = active.Render(phase)
			if progress.total > 0 {
				detail += muted.Render(fmt.Sprintf(" · %d/%d", progress.current, progress.total))
			}
			detail += muted.Render(" · " + formatDuration(time.Since(progress.started)))
		case sourcebook.UpdateFailed:
			marker = failure.Render("✗")
			detail = failure.Render("failed") + muted.Render(" · "+formatDuration(progress.duration))
		case sourcebook.UpdateCanceled:
			marker = muted.Render("–")
			detail = muted.Render("canceled")
		}
		fmt.Fprintf(&view, "%s %-*s  %s\n", marker, nameWidth, progress.source.Name, detail)
	}
	view.WriteString("\n")
	if m.installing {
		view.WriteString(m.spinner.View())
		view.WriteString(" ")
		view.WriteString(accent.Render("Installing updated references…"))
	} else if m.canceling {
		view.WriteString(muted.Render("Canceling active updates…"))
	} else {
		view.WriteString(muted.Render("ctrl+c to cancel"))
	}
	return tea.NewView(view.String())
}

func (m updateModel) failedSource() string {
	for _, progress := range m.sortedProgress() {
		if progress.state == sourcebook.UpdateFailed {
			return progress.source.Name
		}
	}
	return ""
}

func (m updateModel) sortedProgress() []sourceProgress {
	progress := make([]sourceProgress, 0, len(m.sources))
	for _, source := range m.sources {
		progress = append(progress, source)
	}
	sort.Slice(progress, func(i, j int) bool {
		return progress[i].source.Name < progress[j].source.Name
	})
	return progress
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

func RunUpdate(ctx context.Context, input io.Reader, output io.Writer, interactive bool, sources []sourcebook.Source, run UpdateAction) error {
	return runSourceOperation(ctx, input, output, interactive, sources, run, sourceOperation{
		title:       "Sourcebook update",
		successVerb: "Updated",
		working:     "Refreshing sources",
		success:     "Sourcebook updated",
	})
}

func RunSourceAdd(ctx context.Context, input io.Reader, output io.Writer, interactive bool, source sourcebook.Source, run UpdateAction) error {
	return runSourceOperation(ctx, input, output, interactive, []sourcebook.Source{source}, run, sourceOperation{
		title:       "Add source",
		successVerb: "Added",
		working:     "Fetching " + source.Name,
		success:     source.Name + " added to Sourcebook",
	})
}

type sourceOperation struct {
	title       string
	successVerb string
	working     string
	success     string
}

func runSourceOperation(ctx context.Context, input io.Reader, output io.Writer, interactive bool, sources []sourcebook.Source, run UpdateAction, operation sourceOperation) error {
	if !interactive {
		return RunAction(ctx, input, output, false, Action{
			Working: operation.working,
			Success: operation.success,
			Run: func(ctx context.Context) error {
				return run(ctx, nil)
			},
		})
	}

	model := newUpdateModel(ctx, sources, run)
	model.title = operation.title
	model.successVerb = operation.successVerb
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

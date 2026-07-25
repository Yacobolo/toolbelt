package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// DashboardActions supplies the mutations used by the persistent interactive
// dashboard.
type DashboardActions struct {
	Catalog       []sourcebook.CatalogEntry
	Reload        func() ([]sourcebook.Source, error)
	Update        func(context.Context, []string) error
	Remove        func(string) error
	AddPreset     func(context.Context, string) error
	AddRepository func(context.Context, string) error
}

type dashboardMode uint8

const (
	dashboardBrowse dashboardMode = iota
	dashboardAddPicker
)

type dashboardOperation uint8

const (
	dashboardUpdating dashboardOperation = iota
	dashboardRemoving
	dashboardAdding
)

type dashboardOperationFinishedMsg struct {
	operation dashboardOperation
	names     []string
	sources   []sourcebook.Source
	reloaded  bool
	err       error
}

type clearDashboardStatusMsg struct {
	names []string
}

type startDashboardOperationMsg struct {
	generation uint64
}

type dashboardAnimationTickMsg struct {
	generation uint64
}

type dashboardQueuedOperation struct {
	operation dashboardOperation
	names     []string
	label     string
	run       func([]string) error
}

type dashboardModel struct {
	list                list.Model
	sources             []sourcebook.Source
	selected            map[string]struct{}
	rowStatus           map[string]string
	pendingAdds         map[string]sourcebook.Source
	version             string
	skillDir            string
	width               int
	height              int
	mode                dashboardMode
	active              *dashboardQueuedOperation
	queue               []dashboardQueuedOperation
	startGeneration     uint64
	animationGeneration uint64
	animationFrame      int
	confirmName         string
	addPalette          dashboardAddPaletteModel
	actions             DashboardActions
	ctx                 context.Context
}

func newDashboardModel(version, skillDir string, sources []sourcebook.Source) dashboardModel {
	selected := make(map[string]struct{})
	rowStatus := make(map[string]string)
	model := dashboardModel{
		sources:     sortedDashboardSources(sources),
		selected:    selected,
		rowStatus:   rowStatus,
		pendingAdds: make(map[string]sourcebook.Source),
		version:     version,
		skillDir:    skillDir,
		width:       80,
		height:      24,
		ctx:         context.Background(),
	}
	model.list = newDashboardList(model.sources, selected, rowStatus)
	model.resize()
	return model
}

func newDashboardList(
	sources []sourcebook.Source,
	selected map[string]struct{},
	rowStatus map[string]string,
) list.Model {
	items := dashboardItems(sources)
	sourceList := newPickerList(items, "", "source", "sources")
	sourceList.SetDelegate(newDashboardDelegate(sources, selected, rowStatus))
	sourceList.SetShowTitle(false)
	sourceList.SetShowFilter(false)
	sourceList.SetShowHelp(true)
	sourceList.KeyMap.ShowFullHelp.SetHelp("?", "help")
	sourceList.Styles.StatusBar = sourceList.Styles.StatusBar.
		Foreground(lipgloss.Color("#9CA3AF"))
	sourceList.Styles.ActivePaginationDot = sourceList.Styles.ActivePaginationDot.
		Foreground(lipgloss.Color("#22D3EE"))
	sourceList.Styles.InactivePaginationDot = sourceList.Styles.InactivePaginationDot.
		Foreground(lipgloss.Color("#4B5563"))
	sourceList.Help.Styles.ShortKey = sourceList.Help.Styles.ShortKey.
		Foreground(lipgloss.Color("#A78BFA"))
	sourceList.Help.Styles.ShortDesc = sourceList.Help.Styles.ShortDesc.
		Foreground(lipgloss.Color("#9CA3AF"))
	sourceList.Help.Styles.ShortSeparator = sourceList.Help.Styles.ShortSeparator.
		Foreground(lipgloss.Color("#4B5563"))
	sourceList.Help.Styles.FullKey = sourceList.Help.Styles.FullKey.
		Foreground(lipgloss.Color("#A78BFA"))
	sourceList.Help.Styles.FullDesc = sourceList.Help.Styles.FullDesc.
		Foreground(lipgloss.Color("#9CA3AF"))
	configureDashboardList(&sourceList, len(sources) > 0, selected)
	return sourceList
}

func configureDashboardList(
	sourceList *list.Model,
	hasSources bool,
	selected map[string]struct{},
) {
	sourceList.SetShowStatusBar(false)
	sourceList.AdditionalShortHelpKeys = func() []key.Binding {
		if !hasSources {
			return nil
		}
		keys := []key.Binding{dashboardSelectKey}
		if len(selected) > 0 {
			keys = append(keys, dashboardClearSelectionKey)
		}
		return keys
	}
	sourceList.AdditionalFullHelpKeys = func() []key.Binding {
		keys := []key.Binding{dashboardAddKey}
		if hasSources {
			keys = append(
				keys,
				dashboardUpdateKey,
				dashboardUpdateAllKey,
				dashboardRemoveKey,
				dashboardSelectKey,
				dashboardClearSelectionKey,
			)
		}
		return keys
	}
}

func sortedDashboardSources(sources []sourcebook.Source) []sourcebook.Source {
	sorted := append([]sourcebook.Source(nil), sources...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	return sorted
}

func (m dashboardModel) Init() tea.Cmd { return nil }

func (m dashboardModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width = max(message.Width, 1)
		m.height = max(message.Height, 1)
		m.resize()
		return m, nil
	case dashboardOperationFinishedMsg:
		return m.finishOperation(message)
	case clearDashboardStatusMsg:
		for _, name := range message.names {
			delete(m.rowStatus, name)
		}
		return m, nil
	case startDashboardOperationMsg:
		if message.generation != m.startGeneration || m.active != nil || len(m.queue) == 0 {
			return m, nil
		}
		next := m.queue[0]
		m.queue = m.queue[1:]
		return m.activateOperation(next)
	case dashboardAnimationTickMsg:
		if message.generation != m.animationGeneration || m.active == nil {
			return m, nil
		}
		m.animationFrame = (m.animationFrame + 1) % len(dashboardAnimationFrames)
		m.setOperationRowStatus(*m.active, true)
		return m, scheduleDashboardAnimation(m.animationGeneration)
	case tea.KeyPressMsg:
		switch m.mode {
		case dashboardAddPicker:
			return m.updateAddPicker(message)
		default:
			return m.updateBrowse(message)
		}
	}

	if m.mode == dashboardAddPicker {
		return m.updateAddPicker(message)
	}
	var command tea.Cmd
	m.list, command = m.list.Update(message)
	m.syncFilterVisibility()
	return m, command
}

func (m *dashboardModel) resize() {
	fixedRows := 4
	if len(m.sources) > 0 {
		fixedRows++
	}
	listHeight := max(1, min(m.height-fixedRows, 24))
	m.list.SetSize(min(m.width, 100), listHeight)
	if m.mode == dashboardAddPicker {
		m.addPalette.resize(m.width, m.height)
	}
}

func (m dashboardModel) updateBrowse(message tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.confirmName != "" {
		switch message.String() {
		case "y", "Y":
			name := m.confirmName
			m.confirmName = ""
			return m.startRemove(name)
		case "n", "N", "esc", "enter":
			m.confirmName = ""
			return m, nil
		}
		return m, nil
	}

	if m.active != nil && m.list.FilterState() != list.Filtering {
		switch message.String() {
		case "q":
			return m, m.list.NewStatusMessage("Wait for queued operations to finish")
		}
	}

	if m.list.FilterState() != list.Filtering {
		switch message.String() {
		case "a":
			return m.openAddPicker()
		case "u":
			names := m.availableUpdateTargets(m.updateTargets())
			if len(names) == 0 {
				return m, m.list.NewStatusMessage("Selected sources already have pending operations")
			}
			return m.startUpdate(names)
		case "U":
			names := make([]string, len(m.sources))
			for index, source := range m.sources {
				names[index] = source.Name
			}
			names = m.availableUpdateTargets(names)
			if len(names) == 0 {
				return m, m.list.NewStatusMessage("All sources already have pending operations")
			}
			return m.startUpdate(names)
		case "r":
			if source, ok := m.currentSource(); ok {
				if m.sourceHasPendingOperation(source.Name) {
					return m, m.list.NewStatusMessage("Wait for this source's operation to finish")
				}
				m.confirmName = source.Name
				return m, nil
			}
			return m, m.list.NewStatusMessage("No sources to remove")
		case " ", "space":
			if source, ok := m.currentSource(); ok {
				if _, exists := m.selected[source.Name]; exists {
					delete(m.selected, source.Name)
				} else {
					m.selected[source.Name] = struct{}{}
				}
				return m, nil
			}
		case "esc":
			if len(m.selected) > 0 {
				clear(m.selected)
				return m, nil
			}
		}
	}

	var command tea.Cmd
	m.list, command = m.list.Update(message)
	m.syncFilterVisibility()
	return m, command
}

func (m *dashboardModel) syncFilterVisibility() {
	m.list.SetShowFilter(m.list.FilterState() == list.Filtering)
}

func (m dashboardModel) openAddPicker() (tea.Model, tea.Cmd) {
	m.addPalette = newDashboardAddPaletteModel(m.actions.Catalog, m.sources)
	m.mode = dashboardAddPicker
	m.resize()
	return m, m.addPalette.Init()
}

func (m dashboardModel) updateAddPicker(message tea.Msg) (tea.Model, tea.Cmd) {
	if keyMessage, ok := message.(tea.KeyPressMsg); ok && keyMessage.String() == "esc" {
		m.addPalette.input.Blur()
		m.mode = dashboardBrowse
		return m, nil
	}

	var command tea.Cmd
	m.addPalette, command = m.addPalette.Update(message)
	if !m.addPalette.submitted {
		return m, command
	}

	m.addPalette.submitted = false
	m.addPalette.input.Blur()
	if m.addPalette.repositoryURL != "" {
		return m.startAddRepository(m.addPalette.repositoryURL)
	}
	return m.startAddPreset(m.addPalette.presetID)
}

func (m dashboardModel) startUpdate(names []string) (tea.Model, tea.Cmd) {
	if m.actions.Update == nil {
		return m, m.list.NewStatusMessage("Update is unavailable")
	}
	clear(m.selected)
	names = append([]string(nil), names...)
	return m.enqueueOperation(dashboardQueuedOperation{
		operation: dashboardUpdating,
		names:     names,
		label:     "Updating " + sourceCountLabel(names),
		run: func(batchNames []string) error {
			return m.actions.Update(m.ctx, batchNames)
		},
	})
}

func (m dashboardModel) startRemove(name string) (tea.Model, tea.Cmd) {
	if m.actions.Remove == nil {
		return m, m.list.NewStatusMessage("Remove is unavailable")
	}
	return m.enqueueOperation(dashboardQueuedOperation{
		operation: dashboardRemoving,
		names:     []string{name},
		label:     "Removing " + name,
		run: func([]string) error {
			return m.actions.Remove(name)
		},
	})
}

func (m dashboardModel) startAddPreset(presetID string) (tea.Model, tea.Cmd) {
	if m.actions.AddPreset == nil {
		m.mode = dashboardBrowse
		return m, m.list.NewStatusMessage("Adding catalogue sources is unavailable")
	}
	entry, found := m.catalogEntry(presetID)
	if !found {
		m.mode = dashboardBrowse
		return m, m.list.NewStatusMessage("Catalogue source is unavailable")
	}
	sourceName := entry.SourceName
	if sourceName == "" {
		sourceName = entry.ID
	}
	pending := sourcebook.Source{
		Name:     sourceName,
		Provider: entry.Provider,
		URL:      entry.SourceURL,
		Title:    entry.DisplayName,
		Preset:   entry.ID,
	}
	m.pendingAdds[sourceName] = pending
	m.setSources(append(m.sources, pending))
	m.mode = dashboardBrowse
	return m.enqueueOperation(dashboardQueuedOperation{
		operation: dashboardAdding,
		names:     []string{sourceName},
		label:     "Adding " + sourceName,
		run: func([]string) error {
			return m.actions.AddPreset(m.ctx, presetID)
		},
	})
}

func (m dashboardModel) startAddRepository(repositoryURL string) (tea.Model, tea.Cmd) {
	if m.actions.AddRepository == nil {
		m.mode = dashboardBrowse
		return m, m.list.NewStatusMessage("Adding Git repositories is unavailable")
	}
	m.mode = dashboardBrowse
	return m.enqueueOperation(dashboardQueuedOperation{
		operation: dashboardAdding,
		label:     "Adding Git repository",
		run: func([]string) error {
			return m.actions.AddRepository(m.ctx, repositoryURL)
		},
	})
}

func (m dashboardModel) enqueueOperation(
	operation dashboardQueuedOperation,
) (tea.Model, tea.Cmd) {
	if operation.operation == dashboardUpdating {
		if len(m.queue) > 0 && m.queue[len(m.queue)-1].operation == dashboardUpdating {
			last := &m.queue[len(m.queue)-1]
			last.names = appendUniqueNames(last.names, operation.names...)
			last.label = "Updating " + sourceCountLabel(last.names)
			if m.active == nil {
				m.setOperationStartingStatus(operation)
				command := m.scheduleOperationStart()
				return m, command
			}
			m.setOperationRowStatus(operation, false)
			return m, nil
		}
		if m.active == nil {
			m.queue = append(m.queue, operation)
			m.setOperationStartingStatus(operation)
			command := m.scheduleOperationStart()
			return m, command
		}
	}

	if m.active != nil {
		m.queue = append(m.queue, operation)
		m.setOperationRowStatus(operation, false)
		return m, nil
	}
	if len(m.queue) > 0 {
		m.queue = append(m.queue, operation)
		next := m.queue[0]
		m.queue = m.queue[1:]
		return m.activateOperation(next)
	}
	return m.activateOperation(operation)
}

func (m *dashboardModel) scheduleOperationStart() tea.Cmd {
	m.startGeneration++
	generation := m.startGeneration
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return startDashboardOperationMsg{generation: generation}
	})
}

var dashboardAnimationFrames = [...]string{
	"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
}

func scheduleDashboardAnimation(generation uint64) tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg {
		return dashboardAnimationTickMsg{generation: generation}
	})
}

func (m dashboardModel) activateOperation(
	operation dashboardQueuedOperation,
) (tea.Model, tea.Cmd) {
	m.active = &operation
	m.animationGeneration++
	m.animationFrame = 0
	m.setOperationRowStatus(operation, true)
	return m, tea.Batch(
		m.operationCommand(operation),
		scheduleDashboardAnimation(m.animationGeneration),
	)
}

func (m dashboardModel) setOperationStartingStatus(operation dashboardQueuedOperation) {
	for _, name := range operation.names {
		m.rowStatus[name] = "◌ starting…"
	}
}

func (m dashboardModel) setOperationRowStatus(operation dashboardQueuedOperation, active bool) {
	var status string
	switch operation.operation {
	case dashboardUpdating:
		status = dashboardAnimationFrames[m.animationFrame] + " updating…"
	case dashboardRemoving:
		status = dashboardAnimationFrames[m.animationFrame] + " removing…"
	case dashboardAdding:
		status = dashboardAnimationFrames[m.animationFrame] + " adding…"
	}
	if !active {
		status = "• queued"
	}
	for _, name := range operation.names {
		m.rowStatus[name] = status
	}
}

func (m dashboardModel) operationCommand(operation dashboardQueuedOperation) tea.Cmd {
	actions := m.actions
	return func() tea.Msg {
		err := operation.run(operation.names)
		var sources []sourcebook.Source
		reloaded := false
		if actions.Reload != nil {
			reloaded = true
			var reloadErr error
			sources, reloadErr = actions.Reload()
			err = errors.Join(err, reloadErr)
		}
		return dashboardOperationFinishedMsg{
			operation: operation.operation,
			names:     append([]string(nil), operation.names...),
			sources:   sources,
			reloaded:  reloaded,
			err:       err,
		}
	}
}

func appendUniqueNames(existing []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(existing)+len(additions))
	for _, name := range existing {
		seen[name] = struct{}{}
	}
	for _, name := range additions {
		if _, exists := seen[name]; exists {
			continue
		}
		existing = append(existing, name)
		seen[name] = struct{}{}
	}
	return existing
}

func (m dashboardModel) finishOperation(message dashboardOperationFinishedMsg) (tea.Model, tea.Cmd) {
	if m.active == nil {
		return m, nil
	}
	if message.operation == dashboardAdding {
		for _, name := range message.names {
			delete(m.pendingAdds, name)
		}
	}
	m.animationGeneration++
	m.active = nil
	if message.reloaded {
		m.setSources(message.sources)
	}

	var completionCommand tea.Cmd
	if message.err != nil {
		for _, name := range message.names {
			m.rowStatus[name] = "! failed"
		}
		completionCommand = m.list.NewStatusMessage(message.err.Error())
	} else {
		switch message.operation {
		case dashboardUpdating:
			for _, name := range message.names {
				m.rowStatus[name] = "✓ just now"
			}
		case dashboardAdding:
			for _, name := range message.names {
				m.rowStatus[name] = "✓ added"
			}
		case dashboardRemoving:
			// The removed row disappears when refreshed sources are installed.
		}
		if len(message.names) == 0 {
			completionCommand = m.list.NewStatusMessage("Source added")
		}
	}

	if len(m.queue) > 0 {
		next := m.queue[0]
		m.queue = m.queue[1:]
		for _, name := range message.names {
			if message.err == nil {
				delete(m.rowStatus, name)
			}
		}
		activated, nextCommand := m.activateOperation(next)
		m = activated.(dashboardModel)
		if completionCommand != nil {
			return m, tea.Batch(nextCommand, completionCommand)
		}
		return m, nextCommand
	}

	if completionCommand != nil {
		return m, completionCommand
	}
	names := append([]string(nil), message.names...)
	if len(names) > 0 {
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearDashboardStatusMsg{names: names}
		})
	}
	return m, nil
}

func (m *dashboardModel) setSources(sources []sourcebook.Source) {
	currentName := ""
	if current, ok := m.currentSource(); ok {
		currentName = current.Name
	}
	m.sources = sortedDashboardSources(sources)
	for name, pending := range m.pendingAdds {
		found := false
		for _, source := range m.sources {
			if source.Name == name {
				found = true
				break
			}
		}
		if !found {
			m.sources = append(m.sources, pending)
		}
	}
	m.sources = sortedDashboardSources(m.sources)
	available := make(map[string]struct{}, len(m.sources))
	for _, source := range m.sources {
		available[source.Name] = struct{}{}
	}
	for name := range m.selected {
		if _, exists := available[name]; !exists {
			delete(m.selected, name)
		}
	}
	for name := range m.rowStatus {
		if _, exists := available[name]; !exists {
			delete(m.rowStatus, name)
		}
	}
	_ = m.list.SetItems(dashboardItems(m.sources))
	m.list.SetDelegate(newDashboardDelegate(m.sources, m.selected, m.rowStatus))
	configureDashboardList(&m.list, len(m.sources) > 0, m.selected)
	if currentName != "" {
		for index, source := range m.sources {
			if source.Name == currentName {
				m.list.Select(index)
				break
			}
		}
	}
	m.resize()
}

func (m dashboardModel) currentSource() (sourcebook.Source, bool) {
	item, ok := m.list.SelectedItem().(dashboardItem)
	if !ok {
		return sourcebook.Source{}, false
	}
	return item.source, true
}

func (m dashboardModel) selectedNames() []string {
	names := make([]string, 0, len(m.selected))
	for _, source := range m.sources {
		if _, exists := m.selected[source.Name]; exists {
			names = append(names, source.Name)
		}
	}
	return names
}

func (m dashboardModel) updateTargets() []string {
	if names := m.selectedNames(); len(names) > 0 {
		return names
	}
	if source, ok := m.currentSource(); ok {
		return []string{source.Name}
	}
	return nil
}

func (m dashboardModel) availableUpdateTargets(names []string) []string {
	available := make([]string, 0, len(names))
	for _, name := range names {
		if !m.sourceHasPendingOperation(name) {
			available = append(available, name)
		}
	}
	return available
}

func (m dashboardModel) sourceHasPendingOperation(name string) bool {
	if m.active != nil {
		for _, activeName := range m.active.names {
			if activeName == name {
				return true
			}
		}
	}
	for _, operation := range m.queue {
		for _, queuedName := range operation.names {
			if queuedName == name {
				return true
			}
		}
	}
	return false
}

func (m dashboardModel) catalogEntry(id string) (sourcebook.CatalogEntry, bool) {
	for _, entry := range m.actions.Catalog {
		if entry.ID == id {
			return entry, true
		}
	}
	return sourcebook.CatalogEntry{}, false
}

func (m dashboardModel) View() tea.View {
	background := m.dashboardView()
	if m.mode != dashboardAddPicker {
		return tea.NewView(background)
	}

	palette := m.addPalette.View()
	paletteWidth := lipgloss.Width(palette)
	paletteHeight := lipgloss.Height(palette)
	x := max((m.width-paletteWidth)/2, 0)
	maxY := max(m.height-paletteHeight, 0)
	y := max((m.height-paletteHeight)/2, min(3, maxY))
	backgroundLayer := lipgloss.NewLayer(
		lipgloss.NewStyle().Faint(true).Render(background),
	).X(0).Y(0).Z(0)
	paletteLayer := lipgloss.NewLayer(palette).X(x).Y(y).Z(1)
	canvas := lipgloss.NewCanvas(m.width, m.height)
	canvas.Compose(lipgloss.NewCompositor(backgroundLayer, paletteLayer))
	return tea.NewView(canvas.Render())
}

func (m dashboardModel) dashboardView() string {
	var view strings.Builder
	view.WriteString(m.headerView())
	view.WriteString("\n")
	view.WriteString(m.actionBarView())
	view.WriteString("\n")
	if len(m.sources) > 0 {
		view.WriteString(m.sourceSummaryView())
		view.WriteString("\n")
		delegate := newDashboardDelegate(m.sources, m.selected, m.rowStatus)
		view.WriteString(delegate.Header(min(m.width, 100)))
		view.WriteString("\n")
	}
	view.WriteString(m.list.View())
	return view.String()
}

func (m dashboardModel) headerView() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))

	version := strings.TrimSpace(m.version)
	var view strings.Builder
	view.WriteString(title.Render("Sourcebook"))
	if version != "" {
		view.WriteString(" ")
		view.WriteString(versionStyle.Render(version))
	}
	view.WriteString("\n")
	view.WriteString(labelStyle.Render("Skill"))
	view.WriteString("  ")
	view.WriteString(pathStyle.Render(truncate(compactHomePath(m.skillDir), max(m.width-7, 1))))
	return view.String()
}

func compactHomePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	relative, err := filepath.Rel(home, path)
	if err != nil ||
		relative == "." ||
		relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(relative) {
		return path
	}
	return filepath.Join("~", relative)
}

func (m dashboardModel) sourceSummaryView() string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9CA3AF"))
	contextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	visible := len(m.list.VisibleItems())
	total := len(m.sources)
	count := fmt.Sprintf("%d", total)
	if visible != total {
		count = fmt.Sprintf("%d/%d", visible, total)
	}

	parts := []string{labelStyle.Render("Sources") + "  " + contextStyle.Render(count)}
	if len(m.selected) > 0 {
		parts = append(parts, contextStyle.Render(fmt.Sprintf("%d selected", len(m.selected))))
	}
	for _, operation := range []dashboardOperation{
		dashboardUpdating,
		dashboardAdding,
		dashboardRemoving,
	} {
		count := m.pendingOperationCount(operation)
		if count == 0 {
			continue
		}
		parts = append(parts, contextStyle.Render(fmt.Sprintf(
			"%d %s",
			count,
			dashboardOperationPresentParticiple(operation),
		)))
	}
	return strings.Join(parts, contextStyle.Render(" · "))
}

func (m dashboardModel) pendingOperationCount(operation dashboardOperation) int {
	names := make(map[string]struct{})
	collect := func(candidate dashboardQueuedOperation) {
		if candidate.operation != operation {
			return
		}
		for _, name := range candidate.names {
			names[name] = struct{}{}
		}
	}
	if m.active != nil {
		collect(*m.active)
	}
	for _, queued := range m.queue {
		collect(queued)
	}
	return len(names)
}

func dashboardOperationPresentParticiple(operation dashboardOperation) string {
	switch operation {
	case dashboardAdding:
		return "adding"
	case dashboardRemoving:
		return "removing"
	default:
		return "updating"
	}
}

func (m dashboardModel) actionBarView() string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#22D3EE"))
	actionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB"))
	dangerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	separator := lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563")).Render("   ")
	action := func(key, label string) string {
		return keyStyle.Render("["+key+"]") + " " + actionStyle.Render(label)
	}

	if m.confirmName != "" {
		return dangerStyle.Render("Remove "+m.confirmName+"?") + separator +
			action("Y", "Confirm") + separator + action("N", "Cancel")
	}
	actions := []string{action("A", "Add source")}
	if _, ok := m.currentSource(); ok {
		actions = append(
			actions,
			action("U", "Update"),
			action("R", "Remove"),
			action("⇧U", "All"),
		)
	}
	bar := strings.Join(actions, separator)
	if lipgloss.Width(bar) <= m.width {
		return bar
	}
	actions = []string{
		action("A", "Add"),
		action("U", "Update"),
		action("R", "Remove"),
		action("⇧U", "All"),
	}
	bar = strings.Join(actions, separator)
	if lipgloss.Width(bar) <= m.width {
		return bar
	}
	bar = strings.Join(actions[:3], lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4B5563")).
		Render("  "))
	if lipgloss.Width(bar) <= m.width {
		return bar
	}
	return strings.Join(
		[]string{action("A", "+"), action("U", "↻"), action("R", "×")},
		" ",
	)
}

func sourceCountLabel(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return fmt.Sprintf("%d sources", len(names))
}

func relativeTime(value, now time.Time) string {
	elapsed := now.Sub(value)
	if elapsed < 0 {
		elapsed = 0
	}
	switch {
	case elapsed < time.Minute:
		return "now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh", int(elapsed.Hours()))
	case elapsed < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(elapsed.Hours()/24))
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
	dashboardUpdateAllKey = key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("U", "update all"),
	)
	dashboardRemoveKey = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "remove"),
	)
	dashboardSelectKey = key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "select"),
	)
	dashboardClearSelectionKey = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "clear"),
	)
	dashboardBackKey = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	)
)

// RunDashboard runs the persistent interactive Sourcebook dashboard.
func RunDashboard(
	ctx context.Context,
	input io.Reader,
	output io.Writer,
	version string,
	skillDir string,
	sources []sourcebook.Source,
	actions DashboardActions,
) error {
	model := newDashboardModel(version, skillDir, sources)
	model.ctx = ctx
	model.actions = actions
	_, err := tea.NewProgram(
		model,
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithoutSignalHandler(),
	).Run()
	return err
}

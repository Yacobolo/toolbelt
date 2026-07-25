package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
	"github.com/charmbracelet/x/ansi"
)

func TestDashboardRendersVersionSkillPathAndSources(t *testing.T) {
	t.Parallel()

	model := newDashboardModel(
		"v0.3.0",
		"/tmp/codex/skills/sourcebook",
		[]sourcebook.Source{{
			Name:      "datastar-docs",
			Provider:  "datastar",
			UpdatedAt: time.Date(2026, time.July, 24, 9, 0, 0, 0, time.UTC),
		}},
	)
	view := ansi.Strip(model.View().Content)
	for _, expected := range []string{"Sourcebook v0.3.0", "/tmp/codex/skills/sourcebook", "datastar-docs", "[A] Add source", "[U] Update", "[R] Remove"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("dashboard does not contain %q:\n%s", expected, view)
		}
	}
}

func TestDashboardUsesAlignedScannableColumns(t *testing.T) {
	t.Parallel()

	updatedAt := time.Now().Add(-90 * time.Minute)
	model := newDashboardModel(
		"v0.3.0",
		"/tmp/sourcebook",
		[]sourcebook.Source{
			{Name: "alpha", Provider: sourcebook.ProviderGit, UpdatedAt: updatedAt},
			{Name: "powerbi-docs", Provider: sourcebook.ProviderGit, Preset: "powerbi-docs", UpdatedAt: updatedAt},
		},
	)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := ansi.Strip(updated.(dashboardModel).View().Content)
	alpha := lineContainingAll(t, view, "alpha", "Git")
	powerbi := lineContainingAll(t, view, "powerbi-docs", "Git docs")

	if !strings.HasPrefix(alpha, "│ ") {
		t.Fatalf("selected dashboard row = %q, want a clear selection marker", alpha)
	}
	if strings.Contains(alpha, "updated") || strings.Contains(powerbi, "updated") {
		t.Fatalf("dashboard rows repeat the update label:\n%s", view)
	}
	if !strings.Contains(view, "NAME") ||
		!strings.Contains(view, "TYPE") ||
		!strings.Contains(view, "UPDATED") {
		t.Fatalf("dashboard does not label its source columns:\n%s", view)
	}
	if got, want := runeIndex(alpha, "Git"), runeIndex(powerbi, "Git docs"); got != want {
		t.Fatalf("provider columns are not aligned:\n%s", view)
	}
	if got, want := runeIndex(alpha, "1h"), runeIndex(powerbi, "1h"); got != want {
		t.Fatalf("last-updated columns are not aligned:\n%s", view)
	}
	if strings.Contains(alpha, "ago") || strings.Contains(powerbi, "ago") {
		t.Fatalf("labeled updated column uses redundant age suffix:\n%s", view)
	}
	if !strings.Contains(view, "Skill  /tmp/sourcebook") {
		t.Fatalf("dashboard does not label its skill path:\n%s", view)
	}
}

func TestDashboardCompactsHomeRelativeSkillPath(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(home, ".codex", "skills", "sourcebook")
	view := ansi.Strip(newDashboardModel(
		"v0.3.0",
		skillDir,
		[]sourcebook.Source{{Name: "alpha", Provider: "git"}},
	).View().Content)
	want := filepath.Join("~", ".codex", "skills", "sourcebook")
	if !strings.Contains(view, "Skill  "+want) {
		t.Fatalf("dashboard does not compact the home-relative skill path:\n%s", view)
	}
	if strings.Contains(view, home) {
		t.Fatalf("dashboard exposes the redundant full home path:\n%s", view)
	}
}

func TestDashboardKeepsSourceSummaryAndColumnsCompact(t *testing.T) {
	t.Parallel()

	view := ansi.Strip(newDashboardModel(
		"v0.3.0",
		"/tmp/sourcebook",
		[]sourcebook.Source{{Name: "alpha", Provider: "git"}},
	).View().Content)
	lines := strings.Split(view, "\n")
	actionIndex := lineIndexContaining(t, lines, "[A] Add source")
	summaryIndex := lineIndexContaining(t, lines, "Sources  1")
	columnsIndex := lineIndexContaining(t, lines, "NAME")
	sourceIndex := lineIndexContaining(t, lines, "alpha")
	if summaryIndex != actionIndex+1 ||
		columnsIndex != summaryIndex+1 ||
		sourceIndex != columnsIndex+1 {
		t.Fatalf("dashboard source section has unnecessary vertical gaps:\n%s", view)
	}
}

func TestDashboardOnlyShowsFilterRowWhileFiltering(t *testing.T) {
	t.Parallel()

	model := newDashboardModel(
		"v0.3.0",
		"/tmp/sourcebook",
		[]sourcebook.Source{{Name: "alpha", Provider: "git"}},
	)
	if view := ansi.Strip(model.View().Content); strings.Contains(view, "Filter:") {
		t.Fatalf("idle dashboard reserves a filter row:\n%s", view)
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	filtering := updated.(dashboardModel)
	if view := ansi.Strip(filtering.View().Content); !strings.Contains(view, "Filter:") {
		t.Fatalf("dashboard does not show its active filter:\n%s", view)
	}
	updated, _ = filtering.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if view := ansi.Strip(updated.(dashboardModel).View().Content); strings.Contains(view, "Filter:") {
		t.Fatalf("dashboard retains the filter row after cancellation:\n%s", view)
	}
}

func TestDashboardFitsThirteenSourcesAtStandardTerminalHeight(t *testing.T) {
	t.Parallel()

	sources := make([]sourcebook.Source, 16)
	for index := range sources {
		sources[index] = sourcebook.Source{
			Name:     fmt.Sprintf("source-%02d", index),
			Provider: sourcebook.ProviderGit,
		}
	}
	model := newDashboardModel("v0.3.0", "/tmp/sourcebook", sources)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := ansi.Strip(updated.(dashboardModel).View().Content)

	for index := range 13 {
		name := fmt.Sprintf("source-%02d", index)
		if !strings.Contains(view, name) {
			t.Fatalf("dashboard at 80x24 does not fit %q:\n%s", name, view)
		}
	}
}

func lineContaining(t *testing.T, value, substring string) string {
	t.Helper()
	for line := range strings.SplitSeq(value, "\n") {
		if strings.Contains(line, substring) {
			return line
		}
	}
	t.Fatalf("view does not contain a line with %q:\n%s", substring, value)
	return ""
}

func lineContainingAll(t *testing.T, value string, substrings ...string) string {
	t.Helper()
	for line := range strings.SplitSeq(value, "\n") {
		matches := true
		for _, substring := range substrings {
			if !strings.Contains(line, substring) {
				matches = false
				break
			}
		}
		if matches {
			return line
		}
	}
	t.Fatalf("view does not contain a line with %q:\n%s", substrings, value)
	return ""
}

func runeIndex(value, substring string) int {
	byteIndex := strings.Index(value, substring)
	if byteIndex < 0 {
		return -1
	}
	return len([]rune(value[:byteIndex]))
}

func lineIndexContaining(t *testing.T, lines []string, substring string) int {
	t.Helper()
	for index, line := range lines {
		if strings.Contains(line, substring) {
			return index
		}
	}
	t.Fatalf("view does not contain a line with %q:\n%s", substring, strings.Join(lines, "\n"))
	return -1
}

func startScheduledDashboardOperation(
	t *testing.T,
	model dashboardModel,
	scheduleCommand tea.Cmd,
) (dashboardModel, tea.Cmd) {
	t.Helper()
	updated, operationCommand := model.Update(scheduleCommand())
	if operationCommand == nil {
		t.Fatal("scheduled dashboard operation did not start")
	}
	return updated.(dashboardModel), dashboardWorkCommand(t, operationCommand)
}

func dashboardWorkCommand(t *testing.T, command tea.Cmd) tea.Cmd {
	t.Helper()
	message := command()
	batch, ok := message.(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("dashboard operation command returned %T, want a non-empty batch", message)
	}
	return batch[0]
}

func TestDashboardShowsPrimaryActionsAboveTheSourceList(t *testing.T) {
	t.Parallel()

	view := ansi.Strip(newDashboardModel(
		"v0.3.0",
		"/tmp/sourcebook",
		[]sourcebook.Source{{Name: "alpha", Provider: "git"}},
	).View().Content)
	for _, expected := range []string{"[A] Add source", "[U] Update", "[R] Remove"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("dashboard action bar does not contain %q:\n%s", expected, view)
		}
	}
	if strings.Contains(view, "Update alpha") || strings.Contains(view, "Remove alpha") {
		t.Fatalf("dashboard action bar repeats the highlighted source name:\n%s", view)
	}
}

func TestDashboardUpdatesHighlightedSourceInline(t *testing.T) {
	t.Parallel()

	var updatedNames []string
	source := sourcebook.Source{Name: "alpha", Provider: "git"}
	model := newDashboardModel("v0.3.0", "/tmp/sourcebook", []sourcebook.Source{source})
	model.actions = DashboardActions{
		Update: func(_ context.Context, names []string) error {
			updatedNames = append([]string(nil), names...)
			return nil
		},
		Reload: func() ([]sourcebook.Source, error) {
			source.UpdatedAt = time.Now()
			return []sourcebook.Source{source}, nil
		},
	}

	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Text: "u"}))
	queued := updated.(dashboardModel)
	if command == nil || !strings.Contains(ansi.Strip(queued.View().Content), "starting") {
		t.Fatalf("update did not start inline:\n%s", ansi.Strip(queued.View().Content))
	}
	if strings.Contains(ansi.Strip(queued.View().Content), "queued") {
		t.Fatalf("first update is shown as queued with no active operation:\n%s", ansi.Strip(queued.View().Content))
	}
	running, operationCommand := startScheduledDashboardOperation(t, queued, command)
	if !strings.Contains(ansi.Strip(running.View().Content), "updating") {
		t.Fatalf("update did not start inline:\n%s", ansi.Strip(running.View().Content))
	}
	completedMessage := operationCommand()
	updated, _ = running.Update(completedMessage)
	completed := updated.(dashboardModel)
	if got, want := strings.Join(updatedNames, ","), "alpha"; got != want {
		t.Fatalf("updated sources = %q, want %q", got, want)
	}
	if view := ansi.Strip(completed.View().Content); !strings.Contains(view, "✓ just now") {
		t.Fatalf("dashboard does not show inline completion:\n%s", view)
	}
}

func TestDashboardSelectsMultipleSourcesForInlineUpdate(t *testing.T) {
	t.Parallel()

	var updatedNames []string
	sources := []sourcebook.Source{
		{Name: "alpha", Provider: "git"},
		{Name: "beta", Provider: "git"},
	}
	model := newDashboardModel("v0.3.0", "/tmp/sourcebook", sources)
	model.actions = DashboardActions{
		Update: func(_ context.Context, names []string) error {
			updatedNames = append([]string(nil), names...)
			return nil
		},
		Reload: func() ([]sourcebook.Source, error) { return sources, nil },
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))
	updated, _ = updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	updated, _ = updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))
	updated, _ = updated.(dashboardModel).Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	selected := updated.(dashboardModel)
	if view := ansi.Strip(selected.View().Content); !strings.Contains(view, "[U] Update") ||
		!strings.Contains(view, "esc clear") ||
		!strings.Contains(view, "Sources  2 · 2 selected") ||
		!strings.Contains(lineContaining(t, view, "alpha"), "✓") ||
		!strings.Contains(lineContaining(t, view, "beta"), "✓") {
		t.Fatalf("dashboard does not show multi-source action:\n%s", view)
	}
	updated, scheduleCommand := selected.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Text: "u"}))
	if scheduleCommand == nil {
		t.Fatal("multi-source update did not start")
	}
	if view := ansi.Strip(updated.(dashboardModel).View().Content); !strings.Contains(
		view,
		"Sources  2 · 2 updating",
	) {
		t.Fatalf("dashboard does not summarize pending updates:\n%s", view)
	}
	_, operationCommand := startScheduledDashboardOperation(t, updated.(dashboardModel), scheduleCommand)
	_ = operationCommand()
	if got, want := strings.Join(updatedNames, ","), "alpha,beta"; got != want {
		t.Fatalf("updated sources = %q, want %q", got, want)
	}
}

func TestDashboardUppercaseUUpdatesAllSources(t *testing.T) {
	t.Parallel()

	var updatedNames []string
	sources := []sourcebook.Source{
		{Name: "alpha", Provider: "git"},
		{Name: "beta", Provider: "git"},
	}
	model := newDashboardModel("v0.3.0", "/tmp/sourcebook", sources)
	model.actions = DashboardActions{
		Update: func(_ context.Context, names []string) error {
			updatedNames = append([]string(nil), names...)
			return nil
		},
		Reload: func() ([]sourcebook.Source, error) { return sources, nil },
	}
	updated, scheduleCommand := model.Update(tea.KeyPressMsg(tea.Key{Code: 'U', Text: "U"}))
	if scheduleCommand == nil {
		t.Fatal("uppercase U did not start an all-source update")
	}
	_, operationCommand := startScheduledDashboardOperation(t, updated.(dashboardModel), scheduleCommand)
	_ = operationCommand()
	if got, want := strings.Join(updatedNames, ","), "alpha,beta"; got != want {
		t.Fatalf("updated sources = %q, want %q", got, want)
	}
}

func TestDashboardQueuesActionsWhileUpdateRuns(t *testing.T) {
	t.Parallel()

	var updates [][]string
	sources := []sourcebook.Source{
		{Name: "alpha", Provider: "git"},
		{Name: "beta", Provider: "git"},
	}
	model := newDashboardModel("v0.3.0", "/tmp/sourcebook", sources)
	model.actions = DashboardActions{
		Update: func(_ context.Context, names []string) error {
			updates = append(updates, append([]string(nil), names...))
			return nil
		},
		Reload: func() ([]sourcebook.Source, error) { return sources, nil },
	}

	updated, scheduleCommand := model.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Text: "u"}))
	queuedFirst := updated.(dashboardModel)
	if scheduleCommand == nil {
		t.Fatal("first update did not start")
	}
	running, firstCommand := startScheduledDashboardOperation(t, queuedFirst, scheduleCommand)
	updated, _ = running.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	adding := updated.(dashboardModel)
	if adding.mode != dashboardAddPicker {
		t.Fatal("add palette did not remain available during an update")
	}
	if view := ansi.Strip(adding.View().Content); !strings.Contains(view, "Git repository URL") {
		t.Fatalf("add palette is not usable during an update:\n%s", view)
	}
	updated, _ = adding.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	running = updated.(dashboardModel)
	updated, _ = running.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	updated, secondCommand := updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: 'u', Text: "u"}))
	queued := updated.(dashboardModel)
	if secondCommand != nil {
		t.Fatal("queued update started before the active update completed")
	}
	if view := ansi.Strip(queued.View().Content); !strings.Contains(view, "beta") ||
		!strings.Contains(view, "queued") || !strings.Contains(view, "[A] Add source") {
		t.Fatalf("dashboard is not interactive while an update runs:\n%s", view)
	}

	updated, nextCommand := queued.Update(firstCommand())
	if nextCommand == nil {
		t.Fatal("queued update did not start after the first completed")
	}
	completed := updated.(dashboardModel)
	_, _ = completed.Update(dashboardWorkCommand(t, nextCommand)())
	if got, want := fmt.Sprint(updates), "[[alpha] [beta]]"; got != want {
		t.Fatalf("updates = %s, want %s", got, want)
	}
}

func TestDashboardCoalescesQuicklyQueuedUpdatesIntoConcurrentBatch(t *testing.T) {
	t.Parallel()

	var updates [][]string
	sources := []sourcebook.Source{
		{Name: "alpha", Provider: "git"},
		{Name: "beta", Provider: "git"},
		{Name: "gamma", Provider: "git"},
	}
	model := newDashboardModel("v0.3.0", "/tmp/sourcebook", sources)
	model.actions = DashboardActions{
		Update: func(_ context.Context, names []string) error {
			updates = append(updates, append([]string(nil), names...))
			return nil
		},
		Reload: func() ([]sourcebook.Source, error) { return sources, nil },
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Text: "u"}))
	updated, _ = updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	updated, _ = updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: 'u', Text: "u"}))
	updated, _ = updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	updated, startCommand := updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: 'u', Text: "u"}))
	queued := updated.(dashboardModel)
	if startCommand == nil {
		t.Fatal("coalesced update batch did not schedule a start")
	}

	updated, operationCommand := queued.Update(startCommand())
	if operationCommand == nil {
		t.Fatal("coalesced update batch did not start")
	}
	_, _ = updated.(dashboardModel).Update(dashboardWorkCommand(t, operationCommand)())
	if got, want := fmt.Sprint(updates), "[[alpha beta gamma]]"; got != want {
		t.Fatalf("updates = %s, want one concurrent batch %s", got, want)
	}
}

func TestDashboardAnimatesActiveOperationRow(t *testing.T) {
	t.Parallel()

	source := sourcebook.Source{Name: "alpha", Provider: "git"}
	model := newDashboardModel("v0.3.0", "/tmp/sourcebook", []sourcebook.Source{source})
	model.actions = DashboardActions{
		Update: func(context.Context, []string) error { return nil },
		Reload: func() ([]sourcebook.Source, error) { return []sourcebook.Source{source}, nil },
	}
	updated, scheduleCommand := model.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Text: "u"}))
	running, _ := startScheduledDashboardOperation(t, updated.(dashboardModel), scheduleCommand)
	before := ansi.Strip(running.View().Content)

	updated, nextTick := running.Update(dashboardAnimationTickMsg{
		generation: running.animationGeneration,
	})
	animated := updated.(dashboardModel)
	after := ansi.Strip(animated.View().Content)
	if nextTick == nil {
		t.Fatal("active operation did not schedule another animation frame")
	}
	if before == after {
		t.Fatalf("active operation row did not animate:\n%s", after)
	}
}

func TestDashboardConfirmsRemovalInline(t *testing.T) {
	t.Parallel()

	removed := ""
	model := newDashboardModel(
		"v0.3.0",
		"/tmp/sourcebook",
		[]sourcebook.Source{{Name: "alpha", Provider: "git"}},
	)
	model.actions = DashboardActions{
		Remove: func(name string) error {
			removed = name
			return nil
		},
		Reload: func() ([]sourcebook.Source, error) { return nil, nil },
	}
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Text: "r"}))
	confirming := updated.(dashboardModel)
	if command != nil {
		t.Fatal("remove left the dashboard before confirmation")
	}
	if view := ansi.Strip(confirming.View().Content); !strings.Contains(view, "Remove alpha?") ||
		!strings.Contains(view, "[Y] Confirm") {
		t.Fatalf("dashboard does not show inline confirmation:\n%s", view)
	}
	updated, command = confirming.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	if command == nil {
		t.Fatal("confirmed removal did not start")
	}
	completedMessage := dashboardWorkCommand(t, command)()
	_, _ = updated.(dashboardModel).Update(completedMessage)
	if removed != "alpha" {
		t.Fatalf("removed source = %q, want alpha", removed)
	}
}

func TestDashboardOpensAddPaletteWithoutQuitting(t *testing.T) {
	t.Parallel()

	model := newDashboardModel(
		"v0.3.0",
		"/tmp/sourcebook",
		[]sourcebook.Source{{Name: "alpha", Provider: "git"}},
	)
	model.actions.Catalog = []sourcebook.CatalogEntry{{
		ID:          "powerbi-docs",
		DisplayName: "Power BI documentation",
		SourceName:  "powerbi-docs",
	}}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	adding := updated.(dashboardModel)
	view := ansi.Strip(adding.View().Content)
	for _, expected := range []string{
		"╭", "Add source", "Git repository URL", "Power BI documentation",
		"[U] Update", "esc close",
	} {
		if !strings.Contains(strings.ToLower(view), strings.ToLower(expected)) {
			t.Fatalf("add palette does not contain %q:\n%s", expected, view)
		}
	}
}

func TestDashboardAddPaletteSearchesImmediatelyAndPreservesListState(t *testing.T) {
	t.Parallel()

	model := newDashboardModel(
		"v0.3.0",
		"/tmp/sourcebook",
		[]sourcebook.Source{
			{Name: "alpha", Provider: "git"},
			{Name: "beta", Provider: "git"},
		},
	)
	model.actions.Catalog = []sourcebook.CatalogEntry{
		{ID: "azure-docs", DisplayName: "Azure documentation", SourceName: "azure-docs"},
		{ID: "powerbi-docs", DisplayName: "Power BI documentation", SourceName: "powerbi-docs"},
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	updated, _ = updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))
	updated, _ = updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	updated, _ = updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: 'p', Text: "power"}))
	adding := updated.(dashboardModel)
	view := ansi.Strip(adding.View().Content)
	if !strings.Contains(view, "Power BI documentation") || strings.Contains(view, "Azure documentation") {
		t.Fatalf("add palette does not filter as the user types:\n%s", view)
	}

	updated, _ = adding.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	closed := updated.(dashboardModel)
	current, ok := closed.currentSource()
	if !ok || current.Name != "beta" {
		t.Fatalf("closing add palette changed list cursor to %+v", current)
	}
	if _, selected := closed.selected["beta"]; !selected {
		t.Fatal("closing add palette cleared the dashboard selection")
	}
}

func TestDashboardAddPaletteUsesFullWidthAndCompactCopyOnNarrowTerminals(t *testing.T) {
	t.Parallel()

	model := newDashboardModel(
		"v0.3.0",
		"/tmp/sourcebook",
		[]sourcebook.Source{{Name: "alpha", Provider: "git"}},
	)
	model.actions.Catalog = []sourcebook.CatalogEntry{{
		ID:          "powerbi-docs",
		DisplayName: "Power BI documentation",
		SourceName:  "powerbi-docs",
	}}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	updated, _ = updated.(dashboardModel).Update(
		tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}),
	)
	adding := updated.(dashboardModel)

	if got := lipgloss.Width(adding.addPalette.View()); got != 40 {
		t.Fatalf("narrow add palette width = %d, want 40", got)
	}
	initialHeight := lipgloss.Height(adding.addPalette.View())
	filtered, _ := adding.Update(
		tea.KeyPressMsg(tea.Key{Code: 'p', Text: "power"}),
	)
	if got := lipgloss.Height(filtered.(dashboardModel).addPalette.View()); got != initialHeight {
		t.Fatalf("filtered palette height = %d, want stable height %d", got, initialHeight)
	}
	view := ansi.Strip(adding.View().Content)
	if !strings.Contains(view, "Search or paste Git URL") {
		t.Fatalf("narrow add palette does not use compact input copy:\n%s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > 40 {
			t.Fatalf("narrow dashboard line width = %d, want at most 40:\n%q", got, line)
		}
	}
}

func TestDashboardAddsCatalogueSourceInline(t *testing.T) {
	t.Parallel()

	entry := sourcebook.CatalogEntry{
		ID:          "powerbi-docs",
		DisplayName: "Power BI documentation",
		Provider:    sourcebook.ProviderGit,
		SourceName:  "powerbi-docs",
		SourceURL:   "https://github.com/MicrosoftDocs/powerbi-docs.git",
	}
	addedPreset := ""
	addedSource := sourcebook.Source{
		Name:      entry.SourceName,
		Provider:  entry.Provider,
		URL:       entry.SourceURL,
		Preset:    entry.ID,
		UpdatedAt: time.Now(),
	}
	model := newDashboardModel("v0.3.0", "/tmp/sourcebook", nil)
	model.actions = DashboardActions{
		Catalog: []sourcebook.CatalogEntry{entry},
		AddPreset: func(_ context.Context, presetID string) error {
			addedPreset = presetID
			return nil
		},
		Reload: func() ([]sourcebook.Source, error) {
			return []sourcebook.Source{addedSource}, nil
		},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	updated, _ = updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	updated, command := updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	adding := updated.(dashboardModel)
	if command == nil || adding.mode != dashboardBrowse {
		t.Fatalf("catalogue add did not return to the dashboard")
	}
	if view := ansi.Strip(adding.View().Content); !strings.Contains(view, "adding") {
		t.Fatalf("catalogue add does not show an inline pending row:\n%s", view)
	}
	updated, _ = adding.Update(dashboardWorkCommand(t, command)())
	completed := updated.(dashboardModel)
	if addedPreset != entry.ID {
		t.Fatalf("added preset = %q, want %q", addedPreset, entry.ID)
	}
	if view := ansi.Strip(completed.View().Content); !strings.Contains(view, "✓ added") {
		t.Fatalf("catalogue add does not show inline completion:\n%s", view)
	}
}

func TestDashboardAddsRepositoryFromInlineURLPrompt(t *testing.T) {
	t.Parallel()

	addedURL := ""
	source := sourcebook.Source{
		Name:      "widgets",
		Provider:  sourcebook.ProviderGit,
		URL:       "https://github.com/acme/widgets.git",
		UpdatedAt: time.Now(),
	}
	model := newDashboardModel("v0.3.0", "/tmp/sourcebook", nil)
	model.actions = DashboardActions{
		AddRepository: func(_ context.Context, repositoryURL string) error {
			addedURL = repositoryURL
			return nil
		},
		Reload: func() ([]sourcebook.Source, error) {
			return []sourcebook.Source{source}, nil
		},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	updated, _ = updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{
		Code: 'h',
		Text: source.URL,
	}))
	updated, command := updated.(dashboardModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	adding := updated.(dashboardModel)
	if command == nil || adding.mode != dashboardBrowse {
		t.Fatalf("repository add did not return to the dashboard")
	}
	updated, _ = adding.Update(dashboardWorkCommand(t, command)())
	completed := updated.(dashboardModel)
	if addedURL != source.URL {
		t.Fatalf("added URL = %q, want %q", addedURL, source.URL)
	}
	if view := ansi.Strip(completed.View().Content); !strings.Contains(view, "widgets") {
		t.Fatalf("added repository did not appear in dashboard:\n%s", view)
	}
}

func TestDashboardExpandedHelpIncludesActions(t *testing.T) {
	t.Parallel()

	model := newDashboardModel(
		"v0.3.0",
		"/tmp/sourcebook",
		[]sourcebook.Source{{Name: "alpha", Provider: "git"}},
	)
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '?', Text: "?"}))
	view := strings.ToLower(ansi.Strip(updated.(dashboardModel).View().Content))
	for _, expected := range []string{"update all", "space select", "close help"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("dashboard expanded help does not contain %q:\n%s", expected, view)
		}
	}
}

func TestDashboardShortHelpUsesClearHelpLabel(t *testing.T) {
	t.Parallel()

	view := strings.ToLower(ansi.Strip(newDashboardModel(
		"v0.3.0",
		"/tmp/sourcebook",
		[]sourcebook.Source{{Name: "alpha", Provider: "git"}},
	).View().Content))
	if !strings.Contains(view, "? help") || strings.Contains(view, "? more") {
		t.Fatalf("dashboard short help is ambiguous:\n%s", view)
	}
}

func TestRelativeTimeUsesCompactHumanDurations(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		value time.Time
		want  string
	}{
		{value: now.Add(-30 * time.Second), want: "now"},
		{value: now.Add(-12 * time.Minute), want: "12m"},
		{value: now.Add(-3 * time.Hour), want: "3h"},
		{value: now.Add(-4 * 24 * time.Hour), want: "4d"},
	}
	for _, test := range tests {
		if got := relativeTime(test.value, now); got != test.want {
			t.Errorf("relativeTime(%v) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestEmptyDashboardOnlyOffersAdd(t *testing.T) {
	t.Parallel()

	view := strings.ToLower(ansi.Strip(newDashboardModel(
		"v0.3.0",
		"/tmp/sourcebook",
		nil,
	).View().Content))
	if !strings.Contains(view, "[a] add source") {
		t.Fatalf("empty dashboard does not offer add:\n%s", view)
	}
	if strings.Contains(view, "[u] update") || strings.Contains(view, "[r] remove") {
		t.Fatalf("empty dashboard offers unavailable actions:\n%s", view)
	}
	if strings.Count(view, "no sources") != 1 {
		t.Fatalf("empty dashboard repeats empty state:\n%s", view)
	}
}

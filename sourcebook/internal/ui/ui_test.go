package ui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
)

func TestRunActionWithoutTerminalUsesPlainStableOutput(t *testing.T) {
	t.Parallel()

	called := false
	var output bytes.Buffer
	err := RunAction(context.Background(), nil, &output, false, Action{
		Working: "Updating references",
		Success: "Updated references",
		Run: func(context.Context) error {
			called = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunAction() error = %v", err)
	}
	if !called {
		t.Fatal("RunAction() did not execute action")
	}
	if got, want := output.String(), "Updating references...\nUpdated references.\n"; got != want {
		t.Fatalf("RunAction() output = %q, want %q", got, want)
	}
}

func TestActionModelRendersCompletion(t *testing.T) {
	t.Parallel()

	model := newActionModel(Action{Working: "Adding datastar", Success: "Added datastar"})
	updated, command := model.Update(actionFinishedMsg{})
	if command == nil {
		t.Fatal("Update() command = nil, want quit command")
	}
	view := updated.(actionModel).View().Content
	if !strings.Contains(view, "✓") || !strings.Contains(view, "Added datastar") {
		t.Fatalf("completed view = %q", view)
	}
}

func TestActionModelRendersFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("network unavailable")
	model := newActionModel(Action{Working: "Adding datastar", Success: "Added datastar"})
	updated, _ := model.Update(actionFinishedMsg{err: wantErr})
	completed := updated.(actionModel)
	if !errors.Is(completed.err, wantErr) {
		t.Fatalf("completed error = %v, want %v", completed.err, wantErr)
	}
	if view := completed.View().Content; !strings.Contains(view, "✗") || !strings.Contains(view, "Adding datastar failed") || !strings.Contains(view, "network unavailable") {
		t.Fatalf("failed view = %q", view)
	}
}

func TestReportedErrorCanBeDetectedAndUnwrapped(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("clone failed")
	err := markReported(wantErr)
	if !WasReported(err) {
		t.Fatalf("WasReported(%v) = false", err)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("errors.Is(%v, %v) = false", err, wantErr)
	}
}

func TestRenderSourcesPlain(t *testing.T) {
	t.Parallel()

	alphaSize := int64(1536)
	netsuiteSize := int64(2 * 1024 * 1024)
	sources := []sourcebook.Source{
		{Name: "alpha", Provider: "git", URL: "https://example.com/alpha", SizeBytes: &alphaSize, UpdatedAt: time.Date(2026, time.July, 22, 8, 30, 0, 0, time.UTC)},
		{Name: "netsuite-docs", Provider: "netsuite", URL: "https://docs.example.com/", SizeBytes: &netsuiteSize, UpdatedAt: time.Date(2026, time.July, 21, 17, 5, 0, 0, time.UTC)},
	}
	want := "Sourcebook\n2 sources\n\nNAME           TYPE      SIZE     SOURCE                     LAST UPDATED\nalpha          git       1.5 KiB  https://example.com/alpha  2026-07-22 08:30 UTC\nnetsuite-docs  netsuite  2 MiB    https://docs.example.com/  2026-07-21 17:05 UTC\n"
	if got := RenderSources(sources, false, 120); got != want {
		t.Fatalf("RenderSources() = %q, want %q", got, want)
	}
}

func TestRenderSourcesDisplaysGitHubTreeURL(t *testing.T) {
	t.Parallel()

	source := sourcebook.Source{
		Name:     "infisical",
		Provider: "git",
		URL:      "https://github.com/Infisical/infisical.git",
		GitRef:   "main",
		GitRoot:  "docs",
	}
	view := RenderSources([]sourcebook.Source{source}, false, 140)
	if !strings.Contains(view, source.DisplayURL()) {
		t.Fatalf("RenderSources() does not display tree URL %q:\n%s", source.DisplayURL(), view)
	}
	if strings.Contains(view, source.URL) {
		t.Fatalf("RenderSources() displays canonical clone URL %q:\n%s", source.URL, view)
	}
}

func TestRenderSourcesFitsNarrowTerminal(t *testing.T) {
	t.Parallel()

	sources := []sourcebook.Source{
		{
			Name:      "powerbi-docs",
			Provider:  "git",
			URL:       "https://github.com/MicrosoftDocs/powerbi-docs.git",
			UpdatedAt: time.Date(2026, time.July, 22, 8, 30, 0, 0, time.UTC),
		},
	}
	view := RenderSources(sources, false, 60)
	for _, line := range strings.Split(strings.TrimSuffix(view, "\n"), "\n") {
		if len([]rune(line)) > 60 {
			t.Fatalf("narrow source line is %d columns, want at most 60:\n%s", len([]rune(line)), view)
		}
	}
	if strings.Contains(view, "https://github.com/MicrosoftDocs/powerbi-docs.git") {
		t.Fatalf("narrow source view contains an untruncated URL:\n%s", view)
	}
	if !strings.Contains(view, "powerbi-docs") || !strings.Contains(view, "2026-07-22") {
		t.Fatalf("narrow source view omits essential fields:\n%s", view)
	}
}

func TestUpdateModelRendersRepositoryProgress(t *testing.T) {
	t.Parallel()

	sources := []sourcebook.Source{
		{Name: "alpha", Provider: "git", URL: "https://example.com/alpha"},
		{Name: "netsuite-docs", Provider: "netsuite"},
	}
	model := newUpdateModel(context.Background(), sources, nil)
	model.started = time.Now().Add(-2 * time.Second)

	updated, _ := model.Update(updateProgressMsg{event: sourcebook.UpdateEvent{
		Source:  "netsuite-docs",
		State:   sourcebook.UpdateRunning,
		Phase:   "scraping",
		Current: 42,
		Total:   100,
	}})
	model = updated.(updateModel)
	view := model.View().Content
	if !strings.Contains(view, "0/2 sources completed") || !strings.Contains(view, "netsuite-docs") ||
		!strings.Contains(view, "scraping") || !strings.Contains(view, "42/100") {
		t.Fatalf("scraping view = %q", view)
	}

	updated, _ = model.Update(updateProgressMsg{event: sourcebook.UpdateEvent{
		Source:   "netsuite-docs",
		State:    sourcebook.UpdateCompleted,
		Duration: 1500 * time.Millisecond,
	}})
	view = updated.(updateModel).View().Content
	if !strings.Contains(view, "1/2 sources completed") || !strings.Contains(view, "1.5s") {
		t.Fatalf("completed view = %q", view)
	}

	updated, _ = updated.(updateModel).Update(updateProgressMsg{event: sourcebook.UpdateEvent{State: sourcebook.UpdateInstalling}})
	if view := updated.(updateModel).View().Content; !strings.Contains(view, "Installing updated references") {
		t.Fatalf("installing view = %q", view)
	}
}

func TestUpdateModelExplainsAtomicFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("clone failed")
	model := newUpdateModel(context.Background(), []sourcebook.Source{{Name: "alpha", Provider: "git"}}, nil)
	model.started = time.Now()
	updated, command := model.Update(updateFinishedMsg{err: wantErr})
	if command == nil {
		t.Fatal("Update() command = nil, want quit command")
	}
	completed := updated.(updateModel)
	if !errors.Is(completed.err, wantErr) {
		t.Fatalf("completed error = %v, want %v", completed.err, wantErr)
	}
	view := completed.View().Content
	if !strings.Contains(view, "Existing references were left unchanged") {
		t.Fatalf("failed update view = %q", view)
	}
}

func TestSourcePickerSelectsHighlightedSource(t *testing.T) {
	t.Parallel()

	model := newSourcePickerModel([]sourcebook.Source{
		{Name: "alpha", Provider: "git", URL: "https://example.com/alpha"},
		{Name: "beta", Provider: "git", URL: "https://example.com/beta"},
	})
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	updated, command := updated.(sourcePickerModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if command == nil {
		t.Fatal("Update(enter) command = nil, want quit command")
	}
	selected := updated.(sourcePickerModel)
	if !selected.selected || selected.source != "beta" {
		t.Fatalf("selected source = %q, selected = %t; want beta, true", selected.source, selected.selected)
	}
}

func TestSourcePickerDisplaysGitHubTreeURL(t *testing.T) {
	t.Parallel()

	source := sourcebook.Source{
		Name:     "infisical",
		Provider: "git",
		URL:      "https://github.com/Infisical/infisical.git",
		GitRef:   "main",
		GitRoot:  "docs",
	}
	model := newSourcePickerModel([]sourcebook.Source{source})
	view := model.View().Content
	if !strings.Contains(view, source.DisplayURL()) {
		t.Fatalf("source picker does not display tree URL %q:\n%s", source.DisplayURL(), view)
	}
}

func TestSourcePickerCanBeCanceled(t *testing.T) {
	t.Parallel()

	model := newSourcePickerModel([]sourcebook.Source{{Name: "alpha", Provider: "git"}})
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if command == nil {
		t.Fatal("Update(escape) command = nil, want quit command")
	}
	if updated.(sourcePickerModel).selected {
		t.Fatal("canceled picker selected a repository")
	}
}

func TestSourcePickerHelpIncludesChooseKey(t *testing.T) {
	t.Parallel()

	model := newSourcePickerModel([]sourcebook.Source{{Name: "alpha", Provider: "git"}})
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '?', Text: "?"}))
	view := strings.ToLower(updated.(sourcePickerModel).View().Content)
	if !strings.Contains(view, "enter") || !strings.Contains(view, "choose") {
		t.Fatalf("source picker help omits enter/choose:\n%s", view)
	}
}

func TestPresetPickerSelectsPreset(t *testing.T) {
	t.Parallel()

	model := newPresetPickerModel([]sourcebook.CatalogEntry{
		{ID: "netsuite-docs", DisplayName: "NetSuite documentation", Description: "Oracle NetSuite help"},
	}, nil)
	model.list.Select(1)
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if command == nil {
		t.Fatal("Update(enter) command = nil, want quit command")
	}
	selected := updated.(presetPickerModel)
	if !selected.selected || selected.preset != "netsuite-docs" {
		t.Fatalf("selected preset = %q, selected = %t", selected.preset, selected.selected)
	}
}

func TestPresetPickerMarksInstalledSourcesAndDoesNotSelectThem(t *testing.T) {
	t.Parallel()

	model := newPresetPickerModel(
		[]sourcebook.CatalogEntry{
			{ID: "netsuite-docs", SourceName: "netsuite-docs", DisplayName: "NetSuite documentation", Description: "Oracle NetSuite help"},
		},
		map[string]struct{}{"netsuite-docs": {}},
	)
	model.list.Select(1)
	item := model.list.SelectedItem().(presetItem)
	if !item.installed || !strings.Contains(item.Title(), "Installed") {
		t.Fatalf("installed preset title = %q, installed = %t", item.Title(), item.installed)
	}
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if command != nil {
		t.Fatal("installed preset enter command is non-nil, want picker to remain open")
	}
	if updated.(presetPickerModel).selected {
		t.Fatal("installed preset was selected for addition")
	}
}

func TestPresetPickerOffersGitRepositoryURL(t *testing.T) {
	t.Parallel()

	model := newPresetPickerModel(nil, nil)
	item := model.list.SelectedItem().(presetItem)
	if item.id != GitRepositorySelection || !strings.Contains(item.Title(), "Git repository URL") {
		t.Fatalf("first add item = %#v, want Git repository URL", item)
	}
}

func TestRepositoryInputSubmitsTrimmedURL(t *testing.T) {
	t.Parallel()

	model := newRepositoryInputModel()
	model.input.SetValue("  https://github.com/example/docs.git  ")
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if command == nil {
		t.Fatal("repository input enter command = nil, want quit")
	}
	completed := updated.(repositoryInputModel)
	if !completed.submitted || completed.repositoryURL != "https://github.com/example/docs.git" {
		t.Fatalf("repository input = %q, submitted = %t", completed.repositoryURL, completed.submitted)
	}
}

func TestRepositoryInputRejectsEmbeddedCredentials(t *testing.T) {
	t.Parallel()

	model := newRepositoryInputModel()
	model.input.SetValue("https://token@github.com/example/docs.git")
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if command != nil {
		t.Fatal("invalid repository input command is non-nil, want input to remain open")
	}
	completed := updated.(repositoryInputModel)
	if completed.submitted || completed.input.Err == nil {
		t.Fatalf("invalid repository input submitted = %t, error = %v", completed.submitted, completed.input.Err)
	}
}

func TestUpdateSelectionPickerSelectsSubset(t *testing.T) {
	t.Parallel()

	model := newUpdateSelectionModel([]sourcebook.Source{
		{Name: "alpha", Provider: "git"},
		{Name: "beta", Provider: "git"},
	})
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))
	updated, _ = updated.(updateSelectionModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	updated, _ = updated.(updateSelectionModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))
	updated, command := updated.(updateSelectionModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if command == nil {
		t.Fatal("Update(enter) command = nil, want quit command")
	}
	selected := updated.(updateSelectionModel)
	if !selected.confirmed || strings.Join(selected.names, ",") != "alpha,beta" {
		t.Fatalf("selected names = %v, confirmed = %t", selected.names, selected.confirmed)
	}
}

func TestUpdateSelectionPickerDoesNotDefaultToAll(t *testing.T) {
	t.Parallel()

	model := newUpdateSelectionModel([]sourcebook.Source{
		{Name: "alpha", Provider: "git"},
		{Name: "beta", Provider: "git"},
	})
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	selected := updated.(updateSelectionModel)
	if selected.confirmed || len(selected.names) != 0 || len(selected.selected) != 0 {
		t.Fatalf("selected names = %v, confirmed = %t, selected = %v; want empty", selected.names, selected.confirmed, selected.selected)
	}
}

func TestUpdateSelectionPickerCanChooseAllWithExplicitKey(t *testing.T) {
	t.Parallel()

	model := newUpdateSelectionModel([]sourcebook.Source{
		{Name: "alpha", Provider: "git"},
		{Name: "beta", Provider: "git"},
	})
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	updated, command := updated.(updateSelectionModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if command == nil {
		t.Fatal("Update(enter) command = nil after select-all, want quit command")
	}
	selected := updated.(updateSelectionModel)
	if !selected.confirmed || strings.Join(selected.names, ",") != "alpha,beta" {
		t.Fatalf("selected names = %v, confirmed = %t", selected.names, selected.confirmed)
	}
}

func TestUpdateSelectionHelpIncludesCustomKeys(t *testing.T) {
	t.Parallel()

	model := newUpdateSelectionModel([]sourcebook.Source{{Name: "alpha", Provider: "git"}})
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '?', Text: "?"}))
	view := updated.(updateSelectionModel).View().Content
	for _, expected := range []string{"space", "toggle", "a", "all", "enter", "update"} {
		if !strings.Contains(strings.ToLower(view), expected) {
			t.Fatalf("expanded update help does not contain %q:\n%s", expected, view)
		}
	}
}

func TestUpdateProgressRowsRemainAlphabetical(t *testing.T) {
	t.Parallel()

	model := newUpdateModel(context.Background(), []sourcebook.Source{
		{Name: "alpha", Provider: "git"},
		{Name: "beta", Provider: "git"},
	}, nil)
	updated, _ := model.Update(updateProgressMsg{event: sourcebook.UpdateEvent{
		Source: "beta",
		State:  sourcebook.UpdateCompleted,
	}})
	progress := updated.(updateModel).sortedProgress()
	if got := progress[0].source.Name + "," + progress[1].source.Name; got != "alpha,beta" {
		t.Fatalf("progress order = %q, want alpha,beta", got)
	}
}

func TestRemoveConfirmationRequiresExplicitYes(t *testing.T) {
	t.Parallel()

	model := newRemoveConfirmationModel(sourcebook.Source{
		Name: "powerbi-docs",
		URL:  "https://github.com/MicrosoftDocs/powerbi-docs",
	})
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if command == nil || updated.(removeConfirmationModel).confirmed {
		t.Fatal("enter should cancel removal by default")
	}

	model = newRemoveConfirmationModel(sourcebook.Source{Name: "powerbi-docs"})
	updated, command = model.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	if command == nil || !updated.(removeConfirmationModel).confirmed {
		t.Fatal("y should explicitly confirm removal")
	}
}

func TestRemoveConfirmationDisplaysGitHubTreeURL(t *testing.T) {
	t.Parallel()

	source := sourcebook.Source{
		Name:    "infisical",
		URL:     "https://github.com/Infisical/infisical.git",
		GitRef:  "main",
		GitRoot: "docs",
	}
	view := newRemoveConfirmationModel(source).View().Content
	if !strings.Contains(view, source.DisplayURL()) {
		t.Fatalf("remove confirmation does not display tree URL %q:\n%s", source.DisplayURL(), view)
	}
}

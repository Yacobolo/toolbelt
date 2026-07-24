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

	sources := []sourcebook.Source{
		{Name: "alpha", Provider: "git", URL: "https://example.com/alpha", UpdatedAt: time.Date(2026, time.July, 22, 8, 30, 0, 0, time.UTC)},
		{Name: "netsuite-docs", Provider: "netsuite", URL: "https://docs.example.com/", UpdatedAt: time.Date(2026, time.July, 21, 17, 5, 0, 0, time.UTC)},
	}
	want := "Sourcebook\n2 sources\n\nNAME           TYPE      SOURCE                     LAST UPDATED\nalpha          git       https://example.com/alpha  2026-07-22 08:30 UTC\nnetsuite-docs  netsuite  https://docs.example.com/  2026-07-21 17:05 UTC\n"
	if got := RenderSources(sources, false); got != want {
		t.Fatalf("RenderSources() = %q, want %q", got, want)
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

func TestPresetPickerSelectsPreset(t *testing.T) {
	t.Parallel()

	model := newPresetPickerModel([]sourcebook.CatalogEntry{
		{ID: "netsuite-docs", DisplayName: "NetSuite documentation", Description: "Oracle NetSuite help"},
	})
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if command == nil {
		t.Fatal("Update(enter) command = nil, want quit command")
	}
	selected := updated.(presetPickerModel)
	if !selected.selected || selected.preset != "netsuite-docs" {
		t.Fatalf("selected preset = %q, selected = %t", selected.preset, selected.selected)
	}
}

func TestUpdateSelectionPickerSelectsSubset(t *testing.T) {
	t.Parallel()

	model := newUpdateSelectionModel([]sourcebook.Source{
		{Name: "alpha", Provider: "git"},
		{Name: "beta", Provider: "git"},
	})
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	updated, _ = updated.(updateSelectionModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))
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

func TestUpdateSelectionPickerCanChooseAll(t *testing.T) {
	t.Parallel()

	model := newUpdateSelectionModel([]sourcebook.Source{
		{Name: "alpha", Provider: "git"},
		{Name: "beta", Provider: "git"},
	})
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if command == nil {
		t.Fatal("Update(enter) command = nil, want quit command")
	}
	selected := updated.(updateSelectionModel)
	if !selected.confirmed || strings.Join(selected.names, ",") != "alpha,beta" {
		t.Fatalf("selected names = %v, confirmed = %t", selected.names, selected.confirmed)
	}
}

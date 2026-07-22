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

func TestRenderRepositoriesPlain(t *testing.T) {
	t.Parallel()

	repositories := []sourcebook.Repository{
		{Name: "alpha", URL: "https://example.com/alpha", UpdatedAt: time.Date(2026, time.July, 22, 8, 30, 0, 0, time.UTC)},
		{Name: "zeta", URL: "https://example.com/zeta", UpdatedAt: time.Date(2026, time.July, 21, 17, 5, 0, 0, time.UTC)},
	}
	want := "Sourcebook\n2 repositories\n\nNAME   REPOSITORY                 LAST UPDATED\nalpha  https://example.com/alpha  2026-07-22 08:30 UTC\nzeta   https://example.com/zeta   2026-07-21 17:05 UTC\n"
	if got := RenderRepositories(repositories, false); got != want {
		t.Fatalf("RenderRepositories() = %q, want %q", got, want)
	}
}

func TestUpdateModelRendersRepositoryProgress(t *testing.T) {
	t.Parallel()

	repositories := []sourcebook.Repository{
		{Name: "alpha", URL: "https://example.com/alpha"},
		{Name: "beta", URL: "https://example.com/beta"},
	}
	model := newUpdateModel(context.Background(), repositories, nil)
	model.started = time.Now().Add(-2 * time.Second)

	updated, _ := model.Update(updateProgressMsg{event: sourcebook.UpdateEvent{
		Repository: "alpha",
		State:      sourcebook.UpdateCloning,
	}})
	model = updated.(updateModel)
	view := model.View().Content
	if !strings.Contains(view, "0/2 repositories completed") || !strings.Contains(view, "alpha") || !strings.Contains(view, "cloning") {
		t.Fatalf("cloning view = %q", view)
	}

	updated, _ = model.Update(updateProgressMsg{event: sourcebook.UpdateEvent{
		Repository: "alpha",
		State:      sourcebook.UpdateCompleted,
		Duration:   1500 * time.Millisecond,
	}})
	view = updated.(updateModel).View().Content
	if !strings.Contains(view, "1/2 repositories completed") || !strings.Contains(view, "1.5s") {
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
	model := newUpdateModel(context.Background(), []sourcebook.Repository{{Name: "alpha"}}, nil)
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

func TestRepositoryPickerSelectsHighlightedRepository(t *testing.T) {
	t.Parallel()

	model := newRepositoryPickerModel([]sourcebook.Repository{
		{Name: "alpha", URL: "https://example.com/alpha"},
		{Name: "beta", URL: "https://example.com/beta"},
	})
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	updated, command := updated.(repositoryPickerModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if command == nil {
		t.Fatal("Update(enter) command = nil, want quit command")
	}
	selected := updated.(repositoryPickerModel)
	if !selected.selected || selected.repository != "beta" {
		t.Fatalf("selected repository = %q, selected = %t; want beta, true", selected.repository, selected.selected)
	}
}

func TestRepositoryPickerCanBeCanceled(t *testing.T) {
	t.Parallel()

	model := newRepositoryPickerModel([]sourcebook.Repository{{Name: "alpha"}})
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if command == nil {
		t.Fatal("Update(escape) command = nil, want quit command")
	}
	if updated.(repositoryPickerModel).selected {
		t.Fatal("canceled picker selected a repository")
	}
}

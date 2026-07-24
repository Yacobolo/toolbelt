package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
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
	for _, expected := range []string{"Sourcebook v0.3.0", "/tmp/codex/skills/sourcebook", "datastar-docs", "a add", "u update", "r remove"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("dashboard does not contain %q:\n%s", expected, view)
		}
	}
}

func TestDashboardSelectsActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  rune
		want DashboardAction
	}{
		{key: 'a', want: DashboardAdd},
		{key: 'u', want: DashboardUpdate},
		{key: 'r', want: DashboardRemove},
	}
	for _, test := range tests {
		model := newDashboardModel(
			"v0.3.0",
			"/tmp/sourcebook",
			[]sourcebook.Source{{Name: "alpha", Provider: "git"}},
		)
		updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: test.key, Text: string(test.key)}))
		if command == nil {
			t.Fatalf("dashboard key %q did not quit with an action", test.key)
		}
		if got := updated.(dashboardModel).action; got != test.want {
			t.Fatalf("dashboard key %q action = %v, want %v", test.key, got, test.want)
		}
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
	for _, expected := range []string{"a add", "u update", "r remove", "close help"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("dashboard expanded help does not contain %q:\n%s", expected, view)
		}
	}
}

func TestRelativeTimeUsesCompactHumanDurations(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		value time.Time
		want  string
	}{
		{value: now.Add(-30 * time.Second), want: "just now"},
		{value: now.Add(-12 * time.Minute), want: "12m ago"},
		{value: now.Add(-3 * time.Hour), want: "3h ago"},
		{value: now.Add(-4 * 24 * time.Hour), want: "4d ago"},
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
	if !strings.Contains(view, "a add") {
		t.Fatalf("empty dashboard does not offer add:\n%s", view)
	}
	if strings.Contains(view, "u update") || strings.Contains(view, "r remove") {
		t.Fatalf("empty dashboard offers unavailable actions:\n%s", view)
	}
	if strings.Count(view, "no sources") != 1 {
		t.Fatalf("empty dashboard repeats empty state:\n%s", view)
	}
}

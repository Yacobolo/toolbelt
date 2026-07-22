package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
)

func TestRootCommandWithoutArgumentsShowsHelpSuccessfully(t *testing.T) {
	t.Parallel()

	command, stdout, _ := testCommand(t, "1.2.3")
	command.SetArgs(nil)
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	output := stdout.String()
	for _, expected := range []string{"Sourcebook", "sourcebook add", "sourcebook update", "sourcebook version"} {
		if !strings.Contains(output, expected) {
			t.Errorf("help output does not contain %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "sourcebook completion") {
		t.Fatalf("help output contains removed completion command:\n%s", output)
	}
	if strings.Contains(output, "missing command") {
		t.Fatalf("help output contains missing-command error:\n%s", output)
	}
}

func TestRootCommandRejectsUnknownCommand(t *testing.T) {
	t.Parallel()

	command, _, _ := testCommand(t, "dev")
	command.SetArgs([]string{"udpate"})
	err := command.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("ExecuteContext() error = %v, want unknown command", err)
	}
}

func TestVersionCommandAndFlag(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"version"}, {"--version"}} {
		command, stdout, _ := testCommand(t, "1.2.3")
		command.SetArgs(args)
		if err := command.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("ExecuteContext(%v) error = %v", args, err)
		}
		if got, want := stdout.String(), "sourcebook 1.2.3\n"; got != want {
			t.Fatalf("ExecuteContext(%v) output = %q, want %q", args, got, want)
		}
	}
}

func TestResolveVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		injected      string
		moduleVersion string
		want          string
	}{
		{name: "linker version wins", injected: "v1.2.3", moduleVersion: "v9.9.9", want: "v1.2.3"},
		{name: "go install module version", injected: "dev", moduleVersion: "v1.2.3", want: "v1.2.3"},
		{name: "local build", injected: "dev", moduleVersion: "(devel)", want: "dev"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveVersion(test.injected, test.moduleVersion); got != test.want {
				t.Fatalf("resolveVersion(%q, %q) = %q, want %q", test.injected, test.moduleVersion, got, test.want)
			}
		})
	}
}

func TestCompletionCommandIsNotAvailable(t *testing.T) {
	t.Parallel()

	command, _, _ := testCommand(t, "dev")
	command.SetArgs([]string{"completion", "zsh"})
	err := command.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("ExecuteContext() error = %v, want unknown command", err)
	}
}

func TestRemoveWithoutNameRequiresInteractiveTerminal(t *testing.T) {
	t.Parallel()

	command, _, _ := testCommand(t, "dev")
	command.SetArgs([]string{"remove"})
	err := command.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "repository name is required") {
		t.Fatalf("ExecuteContext() error = %v, want non-interactive repository-name error", err)
	}
}

func testCommand(t *testing.T, version string) (command interface {
	SetArgs([]string)
	ExecuteContext(context.Context) error
}, stdout, stderr *bytes.Buffer) {
	t.Helper()
	stdout = new(bytes.Buffer)
	stderr = new(bytes.Buffer)
	app := sourcebook.New(filepath.Join(t.TempDir(), "sourcebook"), commandTestCloner{})
	return newRootCommand(app, bytes.NewReader(nil), stdout, stderr, version), stdout, stderr
}

type commandTestCloner struct{}

func (commandTestCloner) Clone(_ context.Context, _ string, destination string) error {
	return os.MkdirAll(destination, 0o755)
}

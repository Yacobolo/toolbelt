package sourcebook

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitClonerCreatesShallowClone(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	runGit(t, root, "init", source)
	runGit(t, source, "config", "user.name", "Sourcebook Test")
	runGit(t, source, "config", "user.email", "sourcebook@example.com")
	for i := 1; i <= 3; i++ {
		if err := os.WriteFile(filepath.Join(source, "README.md"), []byte(fmt.Sprintf("version %d", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, source, "add", "README.md")
		runGit(t, source, "commit", "-m", fmt.Sprintf("version %d", i))
	}

	destination := filepath.Join(root, "clone")
	if err := (GitCloner{}).Clone(context.Background(), "file://"+source, destination); err != nil {
		t.Fatalf("Clone() error = %v", err)
	}
	if output := strings.TrimSpace(runGit(t, destination, "rev-list", "--count", "HEAD")); output != "1" {
		t.Fatalf("clone commit count = %q, want 1", output)
	}
	if _, err := os.Stat(filepath.Join(destination, ".git", "shallow")); err != nil {
		t.Fatalf("shallow marker: %v", err)
	}
}

func runGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

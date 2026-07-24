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
	if err := (GitCloner{}).Clone(context.Background(), CloneRequest{
		URL:         "file://" + source,
		Destination: destination,
	}); err != nil {
		t.Fatalf("Clone() error = %v", err)
	}
	if output := strings.TrimSpace(runGit(t, destination, "rev-list", "--count", "HEAD")); output != "1" {
		t.Fatalf("clone commit count = %q, want 1", output)
	}
	if _, err := os.Stat(filepath.Join(destination, ".git", "shallow")); err != nil {
		t.Fatalf("shallow marker: %v", err)
	}
}

func TestGitClonerSparseChecksOutRefAndFlattensRoot(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	runGit(t, root, "init", "-b", "main", source)
	runGit(t, source, "config", "user.name", "Sourcebook Test")
	runGit(t, source, "config", "user.email", "sourcebook@example.com")
	writeGitFixture(t, source, "website/docs/index.md", "main documentation")
	writeGitFixture(t, source, "website/docs/media/image.png", "binary fixture")
	writeGitFixture(t, source, "website/app.js", "not documentation")
	runGit(t, source, "add", ".")
	runGit(t, source, "commit", "-m", "main")
	runGit(t, source, "switch", "-c", "current")
	writeGitFixture(t, source, "website/docs/index.md", "current documentation")
	writeGitFixture(t, source, "outside.txt", "exclude me")
	runGit(t, source, "add", ".")
	runGit(t, source, "commit", "-m", "current")

	destination := filepath.Join(root, "clone")
	if err := (GitCloner{}).Clone(context.Background(), CloneRequest{
		URL:         "file://" + source,
		Ref:         "current",
		Root:        "website/docs",
		TextOnly:    true,
		Destination: destination,
	}); err != nil {
		t.Fatalf("Clone() error = %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(destination, "index.md"))
	if err != nil {
		t.Fatalf("read flattened documentation: %v", err)
	}
	if got, want := string(contents), "current documentation"; got != want {
		t.Fatalf("flattened documentation = %q, want %q", got, want)
	}
	for _, excluded := range []string{".git", "website", "outside.txt", "media"} {
		if _, err := os.Stat(filepath.Join(destination, excluded)); !os.IsNotExist(err) {
			t.Fatalf("%s exists in flattened destination; error = %v", excluded, err)
		}
	}
}

func TestGitClonerSparseRootMustExist(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	runGit(t, root, "init", "-b", "main", source)
	runGit(t, source, "config", "user.name", "Sourcebook Test")
	runGit(t, source, "config", "user.email", "sourcebook@example.com")
	writeGitFixture(t, source, "README.md", "fixture")
	runGit(t, source, "add", ".")
	runGit(t, source, "commit", "-m", "fixture")

	destination := filepath.Join(root, "clone")
	err := (GitCloner{}).Clone(context.Background(), CloneRequest{
		URL:         "file://" + source,
		Root:        "missing",
		Destination: destination,
	})
	if err == nil || !strings.Contains(err.Error(), `documentation root "missing"`) {
		t.Fatalf("Clone() error = %v, want missing documentation root", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("destination exists after failed clone; error = %v", err)
	}
}

func writeGitFixture(t *testing.T, repository, name, contents string) {
	t.Helper()
	filename := filepath.Join(repository, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
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

package sourcebook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type GitCloner struct {
	Stdin io.Reader
}

func (g GitCloner) Clone(ctx context.Context, request CloneRequest) error {
	if request.Destination == "" {
		return errors.New("clone destination is required")
	}
	if err := validateGitRef(request.Ref); err != nil {
		return fmt.Errorf("invalid Git ref: %w", err)
	}
	root, err := normalizeGitRoot(request.Root)
	if err != nil {
		return fmt.Errorf("invalid Git root: %w", err)
	}
	if root == "" {
		return g.runClone(ctx, request, request.Destination, false)
	}

	parent := filepath.Dir(request.Destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create clone staging directory: %w", err)
	}
	temporary, err := os.MkdirTemp(parent, ".sourcebook-git-")
	if err != nil {
		return fmt.Errorf("create clone staging directory: %w", err)
	}
	defer os.RemoveAll(temporary)

	repository := filepath.Join(temporary, "repository")
	if err := g.runClone(ctx, request, repository, true); err != nil {
		return err
	}
	var sparseErr error
	if request.TextOnly {
		patterns := documentationTextPatterns(root)
		sparseErr = runGitCommand(
			ctx,
			strings.NewReader(strings.Join(patterns, "\n")+"\n"),
			"-C", repository, "sparse-checkout", "set", "--no-cone", "--stdin",
		)
	} else {
		sparseErr = runGitCommand(ctx, nil, "-C", repository, "sparse-checkout", "set", "--cone", root)
	}
	if sparseErr != nil {
		return fmt.Errorf("select documentation root %q: %w", root, sparseErr)
	}

	selected := filepath.Join(repository, filepath.FromSlash(root))
	info, err := os.Lstat(selected)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("documentation root %q does not exist at ref %q", root, displayRef(request.Ref))
		}
		return fmt.Errorf("inspect documentation root %q: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("documentation root %q is not a directory", root)
	}
	if err := os.Rename(selected, request.Destination); err != nil {
		return fmt.Errorf("flatten documentation root %q: %w", root, err)
	}
	return nil
}

func documentationTextPatterns(root string) []string {
	extensions := []string{
		".md", ".mdx", ".markdown",
		".txt", ".yml", ".yaml", ".json", ".toml", ".ini", ".cfg", ".conf", ".properties", ".xml",
		".html", ".htm", ".sql", ".csv", ".tsv",
		".c", ".cc", ".cpp", ".cs", ".css", ".dart", ".fs", ".fsx", ".go", ".h", ".hpp",
		".java", ".js", ".jsx", ".jl", ".kt", ".kts", ".lua", ".php", ".pl", ".py", ".r", ".rb",
		".rs", ".scala", ".sh", ".bash", ".zsh", ".ps1", ".psm1", ".swift", ".ts", ".tsx",
		".MD", ".MDX", ".YML", ".YAML", ".JSON", ".TXT", ".SQL",
	}
	patterns := make([]string, len(extensions))
	for index, extension := range extensions {
		patterns[index] = "/" + root + "/**/*" + extension
	}
	return patterns
}

func (g GitCloner) runClone(ctx context.Context, request CloneRequest, destination string, sparse bool) error {
	args := []string{"clone", "--depth", "1", "--single-branch", "--no-tags", "--quiet"}
	if request.Ref != "" {
		args = append(args, "--branch", request.Ref)
	}
	if sparse {
		args = append(args, "--filter=blob:none", "--sparse")
	}
	args = append(args, "--", request.URL, destination)
	if err := runGitCommand(ctx, g.Stdin, args...); err != nil {
		return fmt.Errorf("clone repository: %w", err)
	}
	return nil
}

func runGitCommand(ctx context.Context, stdin io.Reader, args ...string) error {
	command := exec.CommandContext(ctx, "git", args...)
	command.Stdin = stdin
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, message)
	}
	return nil
}

func displayRef(ref string) string {
	if ref == "" {
		return "default branch"
	}
	return ref
}

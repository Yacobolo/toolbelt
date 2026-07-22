package sourcebook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type GitCloner struct {
	Stdin io.Reader
}

func (g GitCloner) Clone(ctx context.Context, repositoryURL, destination string) error {
	command := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--single-branch", "--no-tags", "--quiet", "--", repositoryURL, destination)
	command.Stdin = g.Stdin
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

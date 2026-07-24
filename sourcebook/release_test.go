package sourcebook_test

import (
	"os"
	"strings"
	"testing"
)

func TestSourcebookReleaseAutomationExists(t *testing.T) {
	t.Parallel()

	workflow, err := os.ReadFile("../.github/workflows/sourcebook-release.yml")
	if err != nil {
		t.Fatalf("read Sourcebook release workflow: %v", err)
	}
	workflowText := string(workflow)
	for _, required := range []string{
		"sourcebook/v*",
		"goreleaser/goreleaser-action@v7",
		"sourcebook/dist/",
		"gh release create",
	} {
		if !strings.Contains(workflowText, required) {
			t.Errorf("release workflow does not contain %q", required)
		}
	}

	config, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatalf("read GoReleaser config: %v", err)
	}
	configText := string(config)
	for _, required := range []string{
		"sourcebook_{{ .Env.SOURCEBOOK_VERSION }}_{{ .Os }}_{{ .Arch }}",
		"checksums.txt",
		"main.version=v{{ .Env.SOURCEBOOK_VERSION }}",
		"goos: windows",
		"formats: [zip]",
	} {
		if !strings.Contains(configText, required) {
			t.Errorf("GoReleaser config does not contain %q", required)
		}
	}
}

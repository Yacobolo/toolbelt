package sourcebook

import (
	"strings"
	"testing"
)

func TestResolveGitSourceURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want gitSourceSelection
	}{
		{
			name: "GitHub tree URL",
			url:  "https://github.com/Infisical/infisical/tree/main/docs",
			want: gitSourceSelection{
				URL:  "https://github.com/Infisical/infisical.git",
				Ref:  "main",
				Root: "docs",
			},
		},
		{
			name: "nested GitHub folder",
			url:  "https://github.com/acme/widgets/tree/v2/website/docs",
			want: gitSourceSelection{
				URL:  "https://github.com/acme/widgets.git",
				Ref:  "v2",
				Root: "website/docs",
			},
		},
		{
			name: "ordinary GitHub repository",
			url:  "https://github.com/acme/widgets.git",
			want: gitSourceSelection{
				URL: "https://github.com/acme/widgets.git",
			},
		},
		{
			name: "non-GitHub repository",
			url:  "https://git.example.com/acme/widgets.git",
			want: gitSourceSelection{
				URL: "https://git.example.com/acme/widgets.git",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveGitSourceURL(test.url)
			if err != nil {
				t.Fatalf("resolveGitSourceURL() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("resolveGitSourceURL() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestResolveGitSourceURLRejectsMalformedGitHubTreeURL(t *testing.T) {
	t.Parallel()

	for _, repositoryURL := range []string{
		"https://github.com/acme/widgets/tree/main",
		"https://github.com/acme/widgets/tree/main/../docs",
	} {
		t.Run(repositoryURL, func(t *testing.T) {
			t.Parallel()
			_, err := resolveGitSourceURL(repositoryURL)
			if err == nil || !strings.Contains(err.Error(), "GitHub tree") {
				t.Fatalf("resolveGitSourceURL() error = %v, want GitHub tree URL error", err)
			}
		})
	}
}

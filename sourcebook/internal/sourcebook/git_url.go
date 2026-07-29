package sourcebook

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type gitSourceSelection struct {
	URL  string
	Ref  string
	Root string
}

// DisplayURL returns a selected GitHub folder URL when it can be reconstructed
// from persisted clone metadata.
func (source Source) DisplayURL() string {
	if source.GitRef == "" || source.GitRoot == "" {
		return source.URL
	}
	parsed, err := url.Parse(source.URL)
	if err != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		!strings.EqualFold(parsed.Hostname(), "github.com") {
		return source.URL
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) != 2 {
		return source.URL
	}
	repository := strings.TrimSuffix(segments[1], ".git")
	if segments[0] == "" || repository == "" {
		return source.URL
	}
	parsed.Path = "/" + segments[0] + "/" + repository +
		"/tree/" + source.GitRef + "/" + source.GitRoot
	parsed.RawPath = ""
	return parsed.String()
}

// ResolveGitSource converts a repository or GitHub tree URL into the Git source
// metadata that Sourcebook will persist.
func ResolveGitSource(repositoryURL string) (Source, error) {
	selection, err := resolveGitSourceURL(repositoryURL)
	if err != nil {
		return Source{}, err
	}
	name, err := repositoryName(selection.URL)
	if err != nil {
		return Source{}, err
	}
	return Source{
		Name:        name,
		Provider:    ProviderGit,
		URL:         selection.URL,
		GitRef:      selection.Ref,
		GitRoot:     selection.Root,
		GitTextOnly: selection.Root != "",
	}, nil
}

func resolveGitSourceURL(repositoryURL string) (gitSourceSelection, error) {
	validatedURL, err := ValidateRepositoryURL(repositoryURL)
	if err != nil {
		return gitSourceSelection{}, err
	}
	selection := gitSourceSelection{URL: validatedURL}

	parsed, err := url.Parse(validatedURL)
	if err != nil {
		return gitSourceSelection{}, fmt.Errorf("parse repository URL: %w", err)
	}
	if !strings.EqualFold(parsed.Hostname(), "github.com") ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return selection, nil
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 3 || segments[2] != "tree" {
		return selection, nil
	}
	if len(segments) < 5 {
		return gitSourceSelection{}, errors.New("GitHub tree URL must include a repository folder")
	}

	repository := strings.TrimSuffix(segments[1], ".git")
	if err := validateRepositoryName(repository); err != nil {
		return gitSourceSelection{}, fmt.Errorf("invalid GitHub repository name: %w", err)
	}
	ref := segments[3]
	if err := validateGitRef(ref); err != nil {
		return gitSourceSelection{}, fmt.Errorf("invalid GitHub tree ref: %w", err)
	}
	root, err := normalizeGitRoot(strings.Join(segments[4:], "/"))
	if err != nil {
		return gitSourceSelection{}, fmt.Errorf("invalid GitHub tree folder: %w", err)
	}

	parsed.Path = "/" + segments[0] + "/" + repository + ".git"
	parsed.RawPath = ""
	selection.URL = parsed.String()
	selection.Ref = ref
	selection.Root = root
	return selection, nil
}

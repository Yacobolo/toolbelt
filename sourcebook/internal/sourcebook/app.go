package sourcebook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

const (
	skillName              = "sourcebook"
	manifestFilename       = "repos.json"
	skillFilename          = "SKILL.md"
	updateCloneConcurrency = 4
	manifestVersion        = 1
	maxDescriptionLength   = 1024
)

var ErrMutationLocked = errors.New("another sourcebook command is already running")

type Cloner interface {
	Clone(ctx context.Context, url, destination string) error
}

type Repository struct {
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

type UpdateState string

const (
	UpdateCloning    UpdateState = "cloning"
	UpdateCompleted  UpdateState = "completed"
	UpdateFailed     UpdateState = "failed"
	UpdateCanceled   UpdateState = "canceled"
	UpdateInstalling UpdateState = "installing"
)

type UpdateEvent struct {
	Repository string
	State      UpdateState
	Duration   time.Duration
	Err        error
}

type UpdateReporter func(UpdateEvent)

type manifest struct {
	Version      int          `json:"version"`
	Repositories []Repository `json:"repositories"`
}

type App struct {
	skillDir string
	cloner   Cloner
	now      func() time.Time
}

func New(skillDir string, cloner Cloner) *App {
	return &App{skillDir: skillDir, cloner: cloner, now: time.Now}
}

func DefaultSkillDir(homeDir string) string {
	return filepath.Join(homeDir, ".codex", "skills", skillName)
}

func (a *App) Add(ctx context.Context, repositoryURL string) (returnErr error) {
	release, err := a.acquireMutationLock()
	if err != nil {
		return err
	}
	defer func() {
		if err := release(); returnErr == nil && err != nil {
			returnErr = err
		}
	}()

	repositories, err := a.loadRepositories()
	if err != nil {
		return err
	}

	repositoryURL, err = validateRepositoryURL(repositoryURL)
	if err != nil {
		return err
	}
	name, err := repositoryName(repositoryURL)
	if err != nil {
		return err
	}
	for _, repository := range repositories {
		if repository.Name == name {
			return fmt.Errorf("repository %q already exists", name)
		}
	}

	stageDir, err := a.newStageDir()
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageDir)

	stagedRepository := filepath.Join(stageDir, name)
	if err := a.cloner.Clone(ctx, repositoryURL, stagedRepository); err != nil {
		return fmt.Errorf("clone %q: %w", repositoryURL, err)
	}

	referencesDir := filepath.Join(a.skillDir, "references")
	if err := os.MkdirAll(referencesDir, 0o755); err != nil {
		return fmt.Errorf("create references directory: %w", err)
	}
	target := filepath.Join(referencesDir, name)
	if err := os.Rename(stagedRepository, target); err != nil {
		return fmt.Errorf("install repository %q: %w", name, err)
	}

	repositories = append(repositories, Repository{Name: name, URL: repositoryURL, UpdatedAt: a.now().UTC()})
	if err := a.persist(repositories); err != nil {
		_ = os.RemoveAll(target)
		return err
	}
	return nil
}

func (a *App) Update(ctx context.Context) error {
	return a.UpdateWithProgress(ctx, nil)
}

func (a *App) UpdateWithProgress(ctx context.Context, report UpdateReporter) (returnErr error) {
	release, err := a.acquireMutationLock()
	if err != nil {
		return err
	}
	defer func() {
		if err := release(); returnErr == nil && err != nil {
			returnErr = err
		}
	}()

	repositories, err := a.loadRepositories()
	if err != nil {
		return err
	}
	if len(repositories) == 0 {
		return a.persist(repositories)
	}

	stageDir, err := a.newStageDir()
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageDir)

	stagedReferences := filepath.Join(stageDir, "references")
	if err := os.MkdirAll(stagedReferences, 0o755); err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	if err := a.cloneRepositories(ctx, repositories, stagedReferences, report); err != nil {
		return err
	}
	emitUpdate(report, UpdateEvent{State: UpdateInstalling})
	updatedAt := a.now().UTC()
	for index := range repositories {
		repositories[index].UpdatedAt = updatedAt
	}
	if err := a.persist(repositories); err != nil {
		return err
	}

	referencesDir := filepath.Join(a.skillDir, "references")
	previousReferences := filepath.Join(stageDir, "previous-references")
	if err := os.Rename(referencesDir, previousReferences); err != nil {
		return fmt.Errorf("prepare reference update: %w", err)
	}
	if err := os.Rename(stagedReferences, referencesDir); err != nil {
		_ = os.Rename(previousReferences, referencesDir)
		return fmt.Errorf("install reference update: %w", err)
	}
	return nil
}

func (a *App) cloneRepositories(ctx context.Context, repositories []Repository, destination string, report UpdateReporter) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	workerCount := min(updateCloneConcurrency, len(repositories))
	jobs := make(chan Repository)
	errorResults := make(chan error, 1)
	var workers sync.WaitGroup

	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case repository, ok := <-jobs:
					if !ok {
						return
					}
					repositoryDestination := filepath.Join(destination, repository.Name)
					started := time.Now()
					emitUpdate(report, UpdateEvent{Repository: repository.Name, State: UpdateCloning})
					if err := a.cloner.Clone(ctx, repository.URL, repositoryDestination); err != nil {
						state := UpdateFailed
						if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
							state = UpdateCanceled
						}
						emitUpdate(report, UpdateEvent{
							Repository: repository.Name,
							State:      state,
							Duration:   time.Since(started),
							Err:        err,
						})
						select {
						case errorResults <- fmt.Errorf("clone %q: %w", repository.URL, err):
						default:
						}
						cancel()
						return
					}
					emitUpdate(report, UpdateEvent{
						Repository: repository.Name,
						State:      UpdateCompleted,
						Duration:   time.Since(started),
					})
				}
			}
		}()
	}

sendJobs:
	for _, repository := range sortedRepositories(repositories) {
		select {
		case jobs <- repository:
		case <-ctx.Done():
			break sendJobs
		}
	}
	close(jobs)
	workers.Wait()

	select {
	case err := <-errorResults:
		return err
	default:
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func emitUpdate(report UpdateReporter, event UpdateEvent) {
	if report != nil {
		report(event)
	}
}

func (a *App) Remove(name string) (returnErr error) {
	release, err := a.acquireMutationLock()
	if err != nil {
		return err
	}
	defer func() {
		if err := release(); returnErr == nil && err != nil {
			returnErr = err
		}
	}()

	repositories, err := a.loadRepositories()
	if err != nil {
		return err
	}

	name = strings.TrimSpace(name)
	remaining := make([]Repository, 0, len(repositories))
	found := false
	for _, repository := range repositories {
		if repository.Name == name {
			found = true
			continue
		}
		remaining = append(remaining, repository)
	}
	if !found {
		return fmt.Errorf("repository %q does not exist", name)
	}

	stageDir, err := a.newStageDir()
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageDir)

	target := filepath.Join(a.skillDir, "references", name)
	backup := filepath.Join(stageDir, name)
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("remove repository %q: %w", name, err)
	}
	if err := a.persist(remaining); err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	return nil
}

func (a *App) List(output io.Writer) error {
	repositories, err := a.Repositories()
	if err != nil {
		return err
	}
	for _, repository := range repositories {
		updatedAt := "never"
		if !repository.UpdatedAt.IsZero() {
			updatedAt = repository.UpdatedAt.UTC().Format(time.RFC3339)
		}
		if _, err := fmt.Fprintf(output, "%s\t%s\t%s\n", repository.Name, repository.URL, updatedAt); err != nil {
			return fmt.Errorf("write repository list: %w", err)
		}
	}
	return nil
}

func (a *App) Repositories() ([]Repository, error) {
	repositories, err := a.loadRepositories()
	if err != nil {
		return nil, err
	}
	return sortedRepositories(repositories), nil
}

func (a *App) newStageDir() (string, error) {
	parent := filepath.Dir(a.skillDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create skills directory: %w", err)
	}
	stageDir, err := os.MkdirTemp(parent, ".sourcebook-")
	if err != nil {
		return "", fmt.Errorf("create staging directory: %w", err)
	}
	return stageDir, nil
}

func (a *App) acquireMutationLock() (func() error, error) {
	parent := filepath.Dir(a.skillDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, fmt.Errorf("create skills directory: %w", err)
	}
	fileLock := flock.New(filepath.Join(parent, ".sourcebook.lock"), flock.SetPermissions(0o644))
	locked, err := fileLock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("lock sourcebook: %w", err)
	}
	if !locked {
		return nil, ErrMutationLocked
	}
	return func() error {
		if err := fileLock.Unlock(); err != nil {
			return fmt.Errorf("unlock sourcebook: %w", err)
		}
		return nil
	}, nil
}

func (a *App) loadRepositories() ([]Repository, error) {
	contents, err := os.ReadFile(filepath.Join(a.skillDir, manifestFilename))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", manifestFilename, err)
	}

	var stored manifest
	if err := json.Unmarshal(contents, &stored); err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestFilename, err)
	}
	if stored.Version != 0 && stored.Version != manifestVersion {
		return nil, fmt.Errorf("parse %s: unsupported version %d", manifestFilename, stored.Version)
	}
	seen := make(map[string]struct{}, len(stored.Repositories))
	for _, repository := range stored.Repositories {
		if err := validateRepositoryName(repository.Name); err != nil {
			return nil, fmt.Errorf("parse %s: %w", manifestFilename, err)
		}
		if _, err := validateRepositoryURL(repository.URL); err != nil {
			return nil, fmt.Errorf("parse %s: repository %q: %w", manifestFilename, repository.Name, err)
		}
		if _, exists := seen[repository.Name]; exists {
			return nil, fmt.Errorf("parse %s: repository %q appears more than once", manifestFilename, repository.Name)
		}
		seen[repository.Name] = struct{}{}
	}
	return stored.Repositories, nil
}

func (a *App) persist(repositories []Repository) error {
	repositories = sortedRepositories(repositories)
	manifestContents, err := json.MarshalIndent(manifest{Version: manifestVersion, Repositories: repositories}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", manifestFilename, err)
	}
	manifestContents = append(manifestContents, '\n')

	manifestPath := filepath.Join(a.skillDir, manifestFilename)
	previousManifest, err := snapshotFile(manifestPath)
	if err != nil {
		return fmt.Errorf("snapshot %s: %w", manifestFilename, err)
	}
	if err := writeFileAtomically(manifestPath, manifestContents); err != nil {
		return fmt.Errorf("write %s: %w", manifestFilename, err)
	}
	if err := writeFileAtomically(filepath.Join(a.skillDir, skillFilename), renderSkill(repositories)); err != nil {
		writeErr := fmt.Errorf("write %s: %w", skillFilename, err)
		if restoreErr := restoreFile(manifestPath, previousManifest); restoreErr != nil {
			return errors.Join(writeErr, fmt.Errorf("restore %s: %w", manifestFilename, restoreErr))
		}
		return writeErr
	}
	return nil
}

type fileSnapshot struct {
	contents []byte
	exists   bool
}

func snapshotFile(filename string) (fileSnapshot, error) {
	contents, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return fileSnapshot{}, nil
	}
	if err != nil {
		return fileSnapshot{}, err
	}
	return fileSnapshot{contents: contents, exists: true}, nil
}

func restoreFile(filename string, snapshot fileSnapshot) error {
	if snapshot.exists {
		return writeFileAtomically(filename, snapshot.contents)
	}
	if err := os.Remove(filename); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func renderSkill(repositories []Repository) []byte {
	var skill strings.Builder
	skill.WriteString("---\n")
	skill.WriteString("name: sourcebook\n")
	skill.WriteString("description: ")
	skill.WriteString(renderDescription(repositories))
	skill.WriteString("\n")
	skill.WriteString("---\n\n")
	skill.WriteString("# Sourcebook\n\n")
	skill.WriteString("## Repositories\n")
	if len(repositories) > 0 {
		skill.WriteString("\n")
	}
	for _, repository := range sortedRepositories(repositories) {
		fmt.Fprintf(&skill, "- [%s](references/%s/) — %s\n", repository.Name, repository.Name, repository.URL)
	}
	return []byte(skill.String())
}

func renderDescription(repositories []Repository) string {
	repositories = sortedRepositories(repositories)
	if len(repositories) == 0 {
		return "Source code and documentation collected by Sourcebook. Use when a task needs its local repository references."
	}

	names := make([]string, len(repositories))
	for index, repository := range repositories {
		names[index] = repository.Name
	}
	for included := len(names); included >= 1; included-- {
		listedNames := formatNames(names[:included])
		if omitted := len(names) - included; omitted > 0 {
			listedNames += fmt.Sprintf(" and %d more", omitted)
		}

		var description string
		if len(names) == 1 {
			description = fmt.Sprintf("Source code and documentation for the %s repository. Use when a task involves %s or related technologies.", listedNames, names[0])
		} else {
			description = fmt.Sprintf("Source code and documentation for the %s repositories. Use when a task involves one of these repositories or related technologies.", listedNames)
		}
		if len(description) <= maxDescriptionLength {
			return description
		}
	}

	return fmt.Sprintf("Source code and documentation for %d repositories managed by Sourcebook. Use when a task involves one of its repositories or related technologies.", len(repositories))
}

func formatNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}

func writeFileAtomically(filename string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(filename), ".sourcebook-file-")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, filename)
}

func repositoryName(repositoryURL string) (string, error) {
	cleaned := strings.TrimRight(strings.TrimSpace(repositoryURL), "/")
	if cleaned == "" {
		return "", errors.New("repository URL is empty")
	}
	name := strings.TrimSuffix(path.Base(cleaned), ".git")
	if err := validateRepositoryName(name); err != nil {
		return "", fmt.Errorf("cannot derive repository name from %q", repositoryURL)
	}
	return name, nil
}

func validateRepositoryURL(repositoryURL string) (string, error) {
	repositoryURL = strings.TrimSpace(repositoryURL)
	if repositoryURL == "" {
		return "", errors.New("repository URL is empty")
	}
	parsed, err := url.Parse(repositoryURL)
	if err != nil {
		return "", fmt.Errorf("parse repository URL: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.User != nil && (scheme == "http" || scheme == "https")) {
		return "", errors.New("repository URL must not contain embedded credentials or query parameters; use Git credential helpers or SSH")
	}
	if parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword {
			return "", errors.New("repository URL must not contain embedded credentials; use Git credential helpers or SSH")
		}
	}
	return repositoryURL, nil
}

func validateRepositoryName(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid repository name %q", name)
	}
	for _, character := range name {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return fmt.Errorf("invalid repository name %q: contains unsupported characters", name)
	}
	return nil
}

func sortedRepositories(repositories []Repository) []Repository {
	sorted := append([]Repository(nil), repositories...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

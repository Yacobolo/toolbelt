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
	skillName               = "sourcebook"
	manifestFilename        = "sources.json"
	legacyManifestFilename  = "repos.json"
	skillFilename           = "SKILL.md"
	updateSourceConcurrency = 4
	manifestVersion         = 2
	maxDescriptionLength    = 1024
	ProviderGit             = "git"
)

var ErrMutationLocked = errors.New("another sourcebook command is already running")

type Cloner interface {
	Clone(context.Context, CloneRequest) error
}

type CloneRequest struct {
	URL         string
	Ref         string
	Root        string
	TextOnly    bool
	Destination string
}

type Source struct {
	Name        string    `json:"name"`
	Provider    string    `json:"provider"`
	URL         string    `json:"url"`
	Title       string    `json:"title,omitempty"`
	Preset      string    `json:"preset,omitempty"`
	GitRef      string    `json:"git_ref,omitempty"`
	GitRoot     string    `json:"git_root,omitempty"`
	GitTextOnly bool      `json:"git_text_only,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitzero"`
}

type ProviderRequest struct {
	Source         Source
	PreviousDir    string
	DestinationDir string
}

type ProviderProgress struct {
	Phase   string
	Current int
	Total   int
}

type ProviderProgressReporter func(ProviderProgress)

type Provider interface {
	Update(context.Context, ProviderRequest, ProviderProgressReporter) error
}

type ProviderDefinition struct {
	ID       string
	Provider Provider
}

// CatalogEntry describes a selectable source. Its Provider identifies the
// retrieval implementation used to materialize the source.
type CatalogEntry struct {
	ID          string
	DisplayName string
	Description string
	Provider    string
	SourceName  string
	SourceURL   string
	GitRef      string
	GitRoot     string
	GitTextOnly bool
}

type UpdateState string

const (
	UpdateRunning    UpdateState = "running"
	UpdateCompleted  UpdateState = "completed"
	UpdateFailed     UpdateState = "failed"
	UpdateCanceled   UpdateState = "canceled"
	UpdateInstalling UpdateState = "installing"
)

type UpdateEvent struct {
	Source   string
	Provider string
	Phase    string
	Current  int
	Total    int
	State    UpdateState
	Duration time.Duration
	Err      error
}

type UpdateReporter func(UpdateEvent)

type manifest struct {
	Version int      `json:"version"`
	Sources []Source `json:"sources"`
}

type legacyManifest struct {
	Version      int      `json:"version"`
	Repositories []Source `json:"repositories"`
}

type App struct {
	skillDir  string
	cloner    Cloner
	providers map[string]ProviderDefinition
	catalog   map[string]CatalogEntry
	now       func() time.Time
}

func New(skillDir string, cloner Cloner) *App {
	app := &App{
		skillDir:  skillDir,
		cloner:    cloner,
		providers: make(map[string]ProviderDefinition),
		catalog:   make(map[string]CatalogEntry),
		now:       time.Now,
	}
	app.providers[ProviderGit] = ProviderDefinition{
		ID:       ProviderGit,
		Provider: gitProvider{app: app},
	}
	return app
}

func DefaultSkillDir(homeDir, codexHome string) string {
	if codexHome == "" {
		codexHome = filepath.Join(homeDir, ".codex")
	}
	return filepath.Join(codexHome, "skills", skillName)
}

type gitProvider struct {
	app *App
}

func (p gitProvider) Update(ctx context.Context, request ProviderRequest, report ProviderProgressReporter) error {
	if report != nil {
		report(ProviderProgress{Phase: "cloning"})
	}
	return p.app.cloner.Clone(ctx, CloneRequest{
		URL:         request.Source.URL,
		Ref:         request.Source.GitRef,
		Root:        request.Source.GitRoot,
		TextOnly:    request.Source.GitTextOnly,
		Destination: request.DestinationDir,
	})
}

func (a *App) RegisterProvider(definition ProviderDefinition) error {
	if err := validateRepositoryName(definition.ID); err != nil {
		return fmt.Errorf("invalid provider ID: %w", err)
	}
	if definition.ID == ProviderGit {
		return errors.New("the git provider is built in")
	}
	if definition.Provider == nil {
		return fmt.Errorf("provider %q has no implementation", definition.ID)
	}
	if _, exists := a.providers[definition.ID]; exists {
		return fmt.Errorf("provider %q is already registered", definition.ID)
	}
	a.providers[definition.ID] = definition
	return nil
}

func (a *App) RegisterCatalogEntry(entry CatalogEntry) error {
	if err := validateRepositoryName(entry.ID); err != nil {
		return fmt.Errorf("invalid catalogue entry ID: %w", err)
	}
	if strings.TrimSpace(entry.DisplayName) == "" {
		return fmt.Errorf("catalogue entry %q has no display name", entry.ID)
	}
	if err := validateRepositoryName(entry.SourceName); err != nil {
		return fmt.Errorf("catalogue entry %q source name: %w", entry.ID, err)
	}
	if _, exists := a.providers[entry.Provider]; !exists {
		return fmt.Errorf("catalogue entry %q uses unavailable provider %q", entry.ID, entry.Provider)
	}
	if strings.TrimSpace(entry.SourceURL) == "" {
		return fmt.Errorf("catalogue entry %q has no source URL", entry.ID)
	}
	if entry.Provider == ProviderGit {
		validatedURL, err := ValidateRepositoryURL(entry.SourceURL)
		if err != nil {
			return fmt.Errorf("catalogue entry %q repository URL: %w", entry.ID, err)
		}
		entry.SourceURL = validatedURL
		if err := validateGitRef(entry.GitRef); err != nil {
			return fmt.Errorf("catalogue entry %q Git ref: %w", entry.ID, err)
		}
		root, err := normalizeGitRoot(entry.GitRoot)
		if err != nil {
			return fmt.Errorf("catalogue entry %q Git root: %w", entry.ID, err)
		}
		entry.GitRoot = root
		if entry.GitTextOnly && entry.GitRoot == "" {
			return fmt.Errorf("catalogue entry %q enables Git text filtering without a Git root", entry.ID)
		}
	} else if entry.GitRef != "" || entry.GitRoot != "" || entry.GitTextOnly {
		return fmt.Errorf("catalogue entry %q has Git options but uses provider %q", entry.ID, entry.Provider)
	}
	if _, exists := a.catalog[entry.ID]; exists {
		return fmt.Errorf("catalogue entry %q is already registered", entry.ID)
	}
	for _, existing := range a.catalog {
		if existing.SourceName == entry.SourceName {
			return fmt.Errorf("catalogue source name %q is already registered by %q", entry.SourceName, existing.ID)
		}
	}
	a.catalog[entry.ID] = entry
	return nil
}

func (a *App) CatalogEntries() []CatalogEntry {
	entries := make([]CatalogEntry, 0, len(a.catalog))
	for _, entry := range a.catalog {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		left := strings.ToLower(entries[i].DisplayName)
		right := strings.ToLower(entries[j].DisplayName)
		if left == right {
			return entries[i].ID < entries[j].ID
		}
		return left < right
	})
	return entries
}

// SkillDir returns the generated Sourcebook skill directory.
func (a *App) SkillDir() string {
	return a.skillDir
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

	sources, err := a.loadSources()
	if err != nil {
		return err
	}

	selection, err := resolveGitSourceURL(repositoryURL)
	if err != nil {
		return err
	}
	name, err := repositoryName(selection.URL)
	if err != nil {
		return err
	}
	for _, source := range sources {
		if source.Name == name {
			return fmt.Errorf("source %q already exists", name)
		}
	}

	source := Source{
		Name:      name,
		Provider:  ProviderGit,
		URL:       selection.URL,
		GitRef:    selection.Ref,
		GitRoot:   selection.Root,
		UpdatedAt: a.now().UTC(),
	}
	return a.addSource(ctx, sources, source, nil)
}

func (a *App) AddPreset(ctx context.Context, presetID string, report UpdateReporter) (returnErr error) {
	release, err := a.acquireMutationLock()
	if err != nil {
		return err
	}
	defer func() {
		if err := release(); returnErr == nil && err != nil {
			returnErr = err
		}
	}()

	entry, exists := a.catalog[presetID]
	if !exists {
		return fmt.Errorf("unknown catalogue preset %q", presetID)
	}
	sources, err := a.loadSources()
	if err != nil {
		return err
	}
	for _, source := range sources {
		if source.Name == entry.SourceName {
			return fmt.Errorf("source %q already exists", entry.SourceName)
		}
	}
	source := Source{
		Name:        entry.SourceName,
		Provider:    entry.Provider,
		URL:         entry.SourceURL,
		Title:       entry.DisplayName,
		Preset:      entry.ID,
		GitRef:      entry.GitRef,
		GitRoot:     entry.GitRoot,
		GitTextOnly: entry.GitTextOnly,
		UpdatedAt:   a.now().UTC(),
	}
	started := time.Now()
	providerReport := func(progress ProviderProgress) {
		emitUpdate(report, UpdateEvent{
			Source:   source.Name,
			Provider: source.Provider,
			State:    UpdateRunning,
			Phase:    progress.Phase,
			Current:  progress.Current,
			Total:    progress.Total,
			Duration: time.Since(started),
		})
	}
	err = a.addSource(ctx, sources, source, providerReport)
	if err != nil {
		state := UpdateFailed
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			state = UpdateCanceled
		}
		emitUpdate(report, UpdateEvent{
			Source:   source.Name,
			Provider: source.Provider,
			State:    state,
			Duration: time.Since(started),
			Err:      err,
		})
		return err
	}
	emitUpdate(report, UpdateEvent{
		Source:   source.Name,
		Provider: source.Provider,
		State:    UpdateCompleted,
		Duration: time.Since(started),
	})
	return nil
}

func (a *App) addSource(ctx context.Context, sources []Source, source Source, report ProviderProgressReporter) error {
	definition, exists := a.providers[source.Provider]
	if !exists {
		return fmt.Errorf("source %q uses unavailable provider %q", source.Name, source.Provider)
	}
	stageDir, err := a.newStageDir()
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageDir)

	stagedSource := filepath.Join(stageDir, source.Name)
	request := ProviderRequest{
		Source:         source,
		DestinationDir: stagedSource,
	}
	if err := definition.Provider.Update(ctx, request, report); err != nil {
		return fmt.Errorf("update source %q: %w", source.Name, err)
	}

	referencesDir := filepath.Join(a.skillDir, "references")
	if err := os.MkdirAll(referencesDir, 0o755); err != nil {
		return fmt.Errorf("create references directory: %w", err)
	}
	target := filepath.Join(referencesDir, source.Name)
	if err := os.Rename(stagedSource, target); err != nil {
		return fmt.Errorf("install source %q: %w", source.Name, err)
	}

	sources = append(sources, source)
	if err := a.persist(sources); err != nil {
		_ = os.RemoveAll(target)
		return err
	}
	return nil
}

func (a *App) Update(ctx context.Context) error {
	return a.UpdateWithProgress(ctx, nil)
}

func (a *App) UpdateWithProgress(ctx context.Context, report UpdateReporter) (returnErr error) {
	return a.updateWithProgress(ctx, nil, true, report)
}

func (a *App) UpdateSelectedWithProgress(ctx context.Context, names []string, report UpdateReporter) (returnErr error) {
	return a.updateWithProgress(ctx, names, false, report)
}

func (a *App) updateWithProgress(ctx context.Context, names []string, all bool, report UpdateReporter) (returnErr error) {
	release, err := a.acquireMutationLock()
	if err != nil {
		return err
	}
	defer func() {
		if err := release(); returnErr == nil && err != nil {
			returnErr = err
		}
	}()

	sources, err := a.loadSources()
	if err != nil {
		return err
	}
	selected, selectedNames, err := selectSourcesForUpdate(sources, names, all)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return a.persist(sources)
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
	if err := a.updateSources(ctx, selected, stagedReferences, report); err != nil {
		return err
	}
	emitUpdate(report, UpdateEvent{State: UpdateInstalling})
	updatedAt := a.now().UTC()
	for index := range sources {
		if _, exists := selectedNames[sources[index].Name]; exists {
			sources[index].UpdatedAt = updatedAt
		}
	}

	referencesDir := filepath.Join(a.skillDir, "references")
	backupsDir := filepath.Join(stageDir, "previous-references")
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return fmt.Errorf("create reference backup directory: %w", err)
	}
	var installed []string
	rollback := func() error {
		var rollbackErrors []error
		for index := len(installed) - 1; index >= 0; index-- {
			name := installed[index]
			target := filepath.Join(referencesDir, name)
			backup := filepath.Join(backupsDir, name)
			if err := os.RemoveAll(target); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("remove updated source %q: %w", name, err))
				continue
			}
			if err := os.Rename(backup, target); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("restore source %q: %w", name, err))
			}
		}
		return errors.Join(rollbackErrors...)
	}
	for _, source := range sortedSources(selected) {
		target := filepath.Join(referencesDir, source.Name)
		backup := filepath.Join(backupsDir, source.Name)
		if err := os.Rename(target, backup); err != nil {
			if rollbackErr := rollback(); rollbackErr != nil {
				return errors.Join(fmt.Errorf("prepare source %q update: %w", source.Name, err), rollbackErr)
			}
			return fmt.Errorf("prepare source %q update: %w", source.Name, err)
		}
		installed = append(installed, source.Name)
		staged := filepath.Join(stagedReferences, source.Name)
		if err := os.Rename(staged, target); err != nil {
			if rollbackErr := rollback(); rollbackErr != nil {
				return errors.Join(fmt.Errorf("install source %q update: %w", source.Name, err), rollbackErr)
			}
			return fmt.Errorf("install source %q update: %w", source.Name, err)
		}
	}
	if err := a.persist(sources); err != nil {
		if rollbackErr := rollback(); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	return nil
}

func selectSourcesForUpdate(sources []Source, names []string, all bool) ([]Source, map[string]struct{}, error) {
	if all {
		selectedNames := make(map[string]struct{}, len(sources))
		for _, source := range sources {
			selectedNames[source.Name] = struct{}{}
		}
		return append([]Source(nil), sources...), selectedNames, nil
	}
	if len(names) == 0 {
		return nil, nil, errors.New("no sources selected")
	}

	available := make(map[string]Source, len(sources))
	for _, source := range sources {
		available[source.Name] = source
	}
	selected := make([]Source, 0, len(names))
	selectedNames := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if _, exists := selectedNames[name]; exists {
			return nil, nil, fmt.Errorf("source %q was selected more than once", name)
		}
		source, exists := available[name]
		if !exists {
			return nil, nil, fmt.Errorf("source %q does not exist", name)
		}
		selectedNames[name] = struct{}{}
		selected = append(selected, source)
	}
	return selected, selectedNames, nil
}

func (a *App) updateSources(ctx context.Context, sources []Source, destination string, report UpdateReporter) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	workerCount := min(updateSourceConcurrency, len(sources))
	jobs := make(chan Source)
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
				case source, ok := <-jobs:
					if !ok {
						return
					}
					definition, exists := a.providers[source.Provider]
					if !exists {
						select {
						case errorResults <- fmt.Errorf("source %q uses unavailable provider %q", source.Name, source.Provider):
						default:
						}
						cancel()
						return
					}
					sourceDestination := filepath.Join(destination, source.Name)
					started := time.Now()
					request := ProviderRequest{
						Source:         source,
						PreviousDir:    filepath.Join(a.skillDir, "references", source.Name),
						DestinationDir: sourceDestination,
					}
					providerReport := func(progress ProviderProgress) {
						emitUpdate(report, UpdateEvent{
							Source:   source.Name,
							Provider: source.Provider,
							State:    UpdateRunning,
							Phase:    progress.Phase,
							Current:  progress.Current,
							Total:    progress.Total,
							Duration: time.Since(started),
						})
					}
					if err := definition.Provider.Update(ctx, request, providerReport); err != nil {
						state := UpdateFailed
						if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
							state = UpdateCanceled
						}
						emitUpdate(report, UpdateEvent{
							Source:   source.Name,
							Provider: source.Provider,
							State:    state,
							Duration: time.Since(started),
							Err:      err,
						})
						select {
						case errorResults <- fmt.Errorf("update source %q: %w", source.Name, err):
						default:
						}
						cancel()
						return
					}
					emitUpdate(report, UpdateEvent{
						Source:   source.Name,
						Provider: source.Provider,
						State:    UpdateCompleted,
						Duration: time.Since(started),
					})
				}
			}
		}()
	}

sendJobs:
	for _, source := range sortedSources(sources) {
		select {
		case jobs <- source:
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

	sources, err := a.loadSources()
	if err != nil {
		return err
	}

	name = strings.TrimSpace(name)
	remaining := make([]Source, 0, len(sources))
	found := false
	for _, source := range sources {
		if source.Name == name {
			found = true
			continue
		}
		remaining = append(remaining, source)
	}
	if !found {
		return fmt.Errorf("source %q does not exist", name)
	}

	stageDir, err := a.newStageDir()
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageDir)

	target := filepath.Join(a.skillDir, "references", name)
	backup := filepath.Join(stageDir, name)
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("remove source %q: %w", name, err)
	}
	if err := a.persist(remaining); err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	return nil
}

func (a *App) List(output io.Writer) error {
	sources, err := a.Sources()
	if err != nil {
		return err
	}
	for _, source := range sources {
		updatedAt := "never"
		if !source.UpdatedAt.IsZero() {
			updatedAt = source.UpdatedAt.UTC().Format(time.RFC3339)
		}
		if _, err := fmt.Fprintf(output, "%s\t%s\t%s\t%s\n", source.Name, source.Provider, source.URL, updatedAt); err != nil {
			return fmt.Errorf("write source list: %w", err)
		}
	}
	return nil
}

func (a *App) Sources() ([]Source, error) {
	sources, err := a.loadSources()
	if err != nil {
		return nil, err
	}
	return sortedSources(sources), nil
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

func (a *App) loadSources() ([]Source, error) {
	contents, err := os.ReadFile(filepath.Join(a.skillDir, manifestFilename))
	if err == nil {
		var stored manifest
		if err := json.Unmarshal(contents, &stored); err != nil {
			return nil, fmt.Errorf("parse %s: %w", manifestFilename, err)
		}
		if stored.Version != manifestVersion {
			return nil, fmt.Errorf("parse %s: unsupported version %d", manifestFilename, stored.Version)
		}
		stored.Sources = a.resolveCatalogSources(stored.Sources)
		if err := a.validateSources(stored.Sources, manifestFilename); err != nil {
			return nil, err
		}
		return stored.Sources, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", manifestFilename, err)
	}

	legacyContents, err := os.ReadFile(filepath.Join(a.skillDir, legacyManifestFilename))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", legacyManifestFilename, err)
	}
	var stored legacyManifest
	if err := json.Unmarshal(legacyContents, &stored); err != nil {
		return nil, fmt.Errorf("parse %s: %w", legacyManifestFilename, err)
	}
	if stored.Version != 0 && stored.Version != 1 {
		return nil, fmt.Errorf("parse %s: unsupported version %d", legacyManifestFilename, stored.Version)
	}
	sources := make([]Source, len(stored.Repositories))
	for index, repository := range stored.Repositories {
		repository.Provider = ProviderGit
		sources[index] = repository
	}
	sources = a.resolveCatalogSources(sources)
	if err := a.validateSources(sources, legacyManifestFilename); err != nil {
		return nil, err
	}
	return sources, nil
}

func (a *App) validateSources(sources []Source, filename string) error {
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		if err := validateRepositoryName(source.Name); err != nil {
			return fmt.Errorf("parse %s: %w", filename, err)
		}
		if source.Provider == "" {
			return fmt.Errorf("parse %s: source %q has no provider", filename, source.Name)
		}
		if _, exists := a.providers[source.Provider]; !exists {
			return fmt.Errorf("parse %s: source %q uses unavailable provider %q", filename, source.Name, source.Provider)
		}
		if source.Provider == ProviderGit {
			if _, err := ValidateRepositoryURL(source.URL); err != nil {
				return fmt.Errorf("parse %s: source %q: %w", filename, source.Name, err)
			}
			if err := validateGitRef(source.GitRef); err != nil {
				return fmt.Errorf("parse %s: source %q Git ref: %w", filename, source.Name, err)
			}
			if _, err := normalizeGitRoot(source.GitRoot); err != nil {
				return fmt.Errorf("parse %s: source %q Git root: %w", filename, source.Name, err)
			}
			if source.GitTextOnly && source.GitRoot == "" {
				return fmt.Errorf("parse %s: source %q enables Git text filtering without a Git root", filename, source.Name)
			}
		} else if source.GitRef != "" || source.GitRoot != "" || source.GitTextOnly {
			return fmt.Errorf("parse %s: source %q has Git options but uses provider %q", filename, source.Name, source.Provider)
		}
		if source.Preset != "" {
			if err := validateRepositoryName(source.Preset); err != nil {
				return fmt.Errorf("parse %s: source %q preset: %w", filename, source.Name, err)
			}
		}
		if _, exists := seen[source.Name]; exists {
			return fmt.Errorf("parse %s: source %q appears more than once", filename, source.Name)
		}
		seen[source.Name] = struct{}{}
	}
	return nil
}

func (a *App) resolveCatalogSources(sources []Source) []Source {
	resolved := append([]Source(nil), sources...)
	for index, source := range resolved {
		entry, exists := a.catalog[source.Preset]
		if source.Preset == "" || !exists || entry.SourceName != source.Name {
			exists = false
			for _, candidate := range a.catalog {
				if candidate.SourceName == source.Name &&
					candidate.Provider == source.Provider &&
					candidate.SourceURL == source.URL {
					entry = candidate
					exists = true
					break
				}
			}
		}
		if !exists {
			continue
		}
		source.Provider = entry.Provider
		source.URL = entry.SourceURL
		source.Title = entry.DisplayName
		source.Preset = entry.ID
		source.GitRef = entry.GitRef
		source.GitRoot = entry.GitRoot
		source.GitTextOnly = entry.GitTextOnly
		resolved[index] = source
	}
	return resolved
}

func (a *App) persist(sources []Source) error {
	sources = sortedSources(sources)
	manifestContents, err := json.MarshalIndent(manifest{Version: manifestVersion, Sources: sources}, "", "  ")
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
	if err := writeFileAtomically(filepath.Join(a.skillDir, skillFilename), renderSkill(sources)); err != nil {
		writeErr := fmt.Errorf("write %s: %w", skillFilename, err)
		if restoreErr := restoreFile(manifestPath, previousManifest); restoreErr != nil {
			return errors.Join(writeErr, fmt.Errorf("restore %s: %w", manifestFilename, restoreErr))
		}
		return writeErr
	}
	legacyPath := filepath.Join(a.skillDir, legacyManifestFilename)
	_ = os.Remove(legacyPath)
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

func renderSkill(sources []Source) []byte {
	var skill strings.Builder
	skill.WriteString("---\n")
	skill.WriteString("name: sourcebook\n")
	skill.WriteString("description: ")
	skill.WriteString(renderDescription(sources))
	skill.WriteString("\n")
	skill.WriteString("---\n\n")
	skill.WriteString("# Sourcebook\n\n")
	skill.WriteString("## Sources\n")
	if len(sources) > 0 {
		skill.WriteString("\n")
	}
	for _, source := range sortedSources(sources) {
		detail := source.Title
		if detail == "" {
			detail = source.URL
		} else if source.URL != "" {
			detail += " — " + source.URL
		}
		if detail == "" {
			fmt.Fprintf(&skill, "- [%s](references/%s/)\n", source.Name, source.Name)
			continue
		}
		fmt.Fprintf(&skill, "- [%s](references/%s/) — %s\n", source.Name, source.Name, detail)
	}
	return []byte(skill.String())
}

func renderDescription(sources []Source) string {
	sources = sortedSources(sources)
	if len(sources) == 0 {
		return "Source code and documentation collected by Sourcebook. Use when a task needs its local references."
	}

	names := make([]string, len(sources))
	for index, source := range sources {
		if source.Title != "" {
			names[index] = source.Title
		} else {
			names[index] = source.Name
		}
	}
	for included := len(names); included >= 1; included-- {
		listedNames := formatNames(names[:included])
		if omitted := len(names) - included; omitted > 0 {
			listedNames += fmt.Sprintf(" and %d more", omitted)
		}

		var description string
		if len(names) == 1 {
			description = fmt.Sprintf("Source code and documentation for %s. Use when a task involves %s or related technologies.", listedNames, names[0])
		} else {
			description = fmt.Sprintf("Source code and documentation for %s. Use when a task involves one of these sources or related technologies.", listedNames)
		}
		if len(description) <= maxDescriptionLength {
			return description
		}
	}

	return fmt.Sprintf("Source code and documentation for %d sources managed by Sourcebook. Use when a task involves one of its sources or related technologies.", len(sources))
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

// ValidateRepositoryURL validates and normalizes a repository URL before it is
// displayed or persisted.
func ValidateRepositoryURL(repositoryURL string) (string, error) {
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

func validateGitRef(ref string) error {
	if ref == "" {
		return nil
	}
	if strings.TrimSpace(ref) != ref {
		return errors.New("must not have surrounding whitespace")
	}
	if strings.HasPrefix(ref, "-") || strings.HasPrefix(ref, "/") || strings.HasSuffix(ref, "/") {
		return errors.New("must be a branch or tag name")
	}
	if strings.Contains(ref, "..") || strings.Contains(ref, "//") || strings.Contains(ref, "@{") {
		return errors.New("must be a branch or tag name")
	}
	if strings.ContainsAny(ref, ` ~^:?*[\`) || strings.HasSuffix(ref, ".") || strings.HasSuffix(ref, ".lock") {
		return errors.New("must be a branch or tag name")
	}
	for _, component := range strings.Split(ref, "/") {
		if component == "" || component == "." || component == ".." || strings.HasPrefix(component, ".") {
			return errors.New("must be a branch or tag name")
		}
	}
	return nil
}

func normalizeGitRoot(root string) (string, error) {
	if root == "" {
		return "", nil
	}
	if strings.TrimSpace(root) != root || strings.Contains(root, `\`) {
		return "", errors.New("must be a clean repository-relative directory")
	}
	cleaned := path.Clean(root)
	if cleaned == "." || path.IsAbs(cleaned) || cleaned != root ||
		cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("must be a clean repository-relative directory")
	}
	for _, component := range strings.Split(cleaned, "/") {
		if component == "" || component == "." || component == ".." || strings.HasPrefix(component, "-") {
			return "", errors.New("must be a clean repository-relative directory")
		}
		for _, character := range component {
			if !((character >= 'a' && character <= 'z') ||
				(character >= 'A' && character <= 'Z') ||
				(character >= '0' && character <= '9') ||
				character == '.' || character == '_' || character == '-') {
				return "", errors.New("must contain only letters, digits, dots, underscores, hyphens, and slashes")
			}
		}
	}
	return cleaned, nil
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

func sortedSources(sources []Source) []Source {
	sorted := append([]Source(nil), sources...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

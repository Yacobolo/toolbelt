package config

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultBindAddress = "127.0.0.1:8787"
	DefaultConfigName  = ".governance.yaml"
)

type Repository struct {
	ID                 string
	Name               string
	SourcePath         string
	Root               string
	RuntimeDir         string
	DatabasePath       string
	LockPath           string
	CoverageOutputPath string
}

type Config struct {
	HostRoot        string       `yaml:"-"`
	BindAddress     string       `yaml:"bind_address"`
	RepositoryPaths []string     `yaml:"repositories"`
	Repositories    []Repository `yaml:"-"`
}

func Load(startDir string) (Config, error) {
	hostRoot, err := filepath.Abs(startDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve start directory: %w", err)
	}

	cfg := Config{
		HostRoot:    hostRoot,
		BindAddress: DefaultBindAddress,
	}

	configPath := filepath.Join(hostRoot, DefaultConfigName)
	if data, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse %s: %w", DefaultConfigName, err)
		}
	} else if !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("read %s: %w", DefaultConfigName, err)
	}

	if strings.TrimSpace(cfg.BindAddress) == "" {
		cfg.BindAddress = DefaultBindAddress
	}

	if len(cfg.RepositoryPaths) == 0 {
		if hasGoMod(hostRoot) {
			cfg.RepositoryPaths = []string{hostRoot}
		} else {
			return Config{}, fmt.Errorf("%s must define at least one repository path", DefaultConfigName)
		}
	}

	repos, err := resolveRepositories(hostRoot, cfg.RepositoryPaths)
	if err != nil {
		return Config{}, err
	}
	cfg.Repositories = repos
	return cfg, nil
}

func (c Config) GovernanceDir() string {
	return filepath.Join(c.HostRoot, ".governance")
}

func (c Config) Repository(id string) (Repository, bool) {
	for _, repo := range c.Repositories {
		if repo.ID == id {
			return repo, true
		}
	}
	return Repository{}, false
}

func DefaultIgnorePaths() []string {
	return []string{".git", ".governance", "node_modules"}
}

func DefaultCoverageArgs(outputPath string) []string {
	return []string{"test", "-coverprofile=" + outputPath, "./..."}
}

func DefaultCoverageArgsWithDevTag(outputPath string) []string {
	return []string{"test", "-tags=dev", "-coverprofile=" + outputPath, "./..."}
}

func resolveRepositories(hostRoot string, paths []string) ([]Repository, error) {
	normalized := make([]string, 0, len(paths))
	baseCounts := make(map[string]int, len(paths))

	for _, item := range paths {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		absPath, err := filepath.Abs(value)
		if err != nil {
			return nil, fmt.Errorf("resolve repository path %q: %w", value, err)
		}
		if _, err := os.Stat(absPath); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("repository path %s does not exist", absPath)
			}
			return nil, fmt.Errorf("stat repository path %s: %w", absPath, err)
		}
		moduleRoot, err := resolveModuleRoot(absPath)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, absPath+"|"+moduleRoot)
		baseCounts[slugify(filepath.Base(absPath))]++
	}

	if len(normalized) == 0 {
		return nil, fmt.Errorf("no valid repository paths configured")
	}

	repos := make([]Repository, 0, len(normalized))
	for _, entry := range normalized {
		sourcePath, moduleRoot, _ := strings.Cut(entry, "|")
		baseID := slugify(filepath.Base(sourcePath))
		repoID := baseID
		if baseCounts[baseID] > 1 {
			repoID = baseID + "-" + shortHash(sourcePath)
		}
		runtimeDir := filepath.Join(hostRoot, ".governance", "repos", repoID)
		repos = append(repos, Repository{
			ID:                 repoID,
			Name:               filepath.Base(sourcePath),
			SourcePath:         sourcePath,
			Root:               moduleRoot,
			RuntimeDir:         runtimeDir,
			DatabasePath:       filepath.Join(runtimeDir, "governance.db"),
			LockPath:           filepath.Join(runtimeDir, "refresh.lock"),
			CoverageOutputPath: filepath.Join(runtimeDir, "coverage.out"),
		})
	}

	return repos, nil
}

func hasGoMod(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return false
	}
	return true
}

func resolveModuleRoot(root string) (string, error) {
	if hasGoMod(root) {
		return root, nil
	}

	candidates := make([]string, 0)
	for _, child := range []string{"main"} {
		candidate := filepath.Join(root, child)
		if hasGoMod(candidate) {
			return candidate, nil
		}
	}

	for _, pattern := range []string{
		filepath.Join(root, "*", "go.mod"),
		filepath.Join(root, "*", "*", "go.mod"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", fmt.Errorf("scan repository path %s: %w", root, err)
		}
		for _, match := range matches {
			candidates = append(candidates, filepath.Dir(match))
		}
	}

	unique := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		unique = append(unique, candidate)
	}
	if len(unique) == 1 {
		return unique[0], nil
	}
	if len(unique) == 0 {
		return "", fmt.Errorf("repository path %s does not contain go.mod", root)
	}
	return "", fmt.Errorf("repository path %s contains multiple Go modules; expected root or main module", root)
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "repo"
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "repo"
	}
	return slug
}

func shortHash(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:8]
}

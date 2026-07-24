package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver/v3"
)

const defaultNoticeTTL = 24 * time.Hour

type noticeCache struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version,omitempty"`
	NotifiedAt    time.Time `json:"notified_at,omitempty"`
}

// Notice returns a short update hint. Successful checks are cached so normal
// Sourcebook commands contact GitHub at most once per notice TTL.
func (m *Manager) Notice(ctx context.Context, currentVersion string) (string, error) {
	if m.noticeCachePath == "" {
		return "", nil
	}
	current, err := parseCurrentVersion(currentVersion)
	if errors.Is(err, errDevelopmentBuild) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	now := m.now()
	cache, cacheErr := readNoticeCache(m.noticeCachePath)
	if cacheErr == nil && !cache.CheckedAt.After(now) && now.Sub(cache.CheckedAt) < m.noticeTTL {
		if !cache.NotifiedAt.IsZero() {
			return "", nil
		}
		notice, err := noticeForVersions(current, cache.LatestVersion)
		if err != nil || notice == "" {
			return notice, err
		}
		cache.NotifiedAt = now
		if err := writeNoticeCache(m.noticeCachePath, cache); err != nil {
			return "", fmt.Errorf("cache update notice: %w", err)
		}
		return notice, nil
	}

	latest, found, err := m.backend.latest(ctx)
	if err != nil {
		return "", fmt.Errorf("check GitHub releases: %w", err)
	}
	cache = noticeCache{CheckedAt: now}
	if found {
		cache.LatestVersion = latest.version
	}
	notice, err := noticeForVersions(current, cache.LatestVersion)
	if err != nil {
		return "", err
	}
	if notice != "" {
		cache.NotifiedAt = now
	}
	if err := writeNoticeCache(m.noticeCachePath, cache); err != nil {
		return "", fmt.Errorf("cache update check: %w", err)
	}
	return notice, nil
}

func noticeForVersions(current *semver.Version, latestText string) (string, error) {
	if latestText == "" {
		return "", nil
	}
	latest, err := semver.NewVersion(latestText)
	if err != nil {
		return "", fmt.Errorf("parse cached release version %q: %w", latestText, err)
	}
	if !latest.GreaterThan(current) {
		return "", nil
	}
	return fmt.Sprintf(
		"Sourcebook %s is available; run sourcebook upgrade.",
		displayVersion(latest),
	), nil
}

func readNoticeCache(filename string) (noticeCache, error) {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return noticeCache{}, err
	}
	var cache noticeCache
	if err := json.Unmarshal(contents, &cache); err != nil {
		return noticeCache{}, err
	}
	return cache, nil
}

func writeNoticeCache(filename string, cache noticeCache) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(filename), ".notice-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	encoder := json.NewEncoder(temporary)
	if err := encoder.Encode(cache); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(temporaryName, filename)
}

func defaultNoticeCachePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cacheDir, "sourcebook", "update-check.json")
}

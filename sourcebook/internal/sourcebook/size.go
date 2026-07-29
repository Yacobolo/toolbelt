package sourcebook

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type measuredSourceSize struct {
	Provider string
	URL      string
	Bytes    int64
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("walk %s: %w", root, err)
	}
	return total, nil
}

func (a *App) cacheMeasuredSizes(measured map[string]measuredSourceSize) (returnErr error) {
	if len(measured) == 0 {
		return nil
	}
	if _, err := os.Stat(filepath.Join(a.skillDir, manifestFilename)); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect %s: %w", manifestFilename, err)
	}

	release, err := a.acquireMutationLock()
	if errors.Is(err, ErrMutationLocked) {
		return nil
	}
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
	changed := false
	for index := range sources {
		size, exists := measured[sources[index].Name]
		if !exists ||
			sources[index].Provider != size.Provider ||
			sources[index].URL != size.URL ||
			sources[index].SizeBytes != nil {
			continue
		}
		sizeBytes := size.Bytes
		sources[index].SizeBytes = &sizeBytes
		changed = true
	}
	if !changed {
		return nil
	}
	return a.persist(sources)
}

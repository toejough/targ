package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SystemOps provides system operations for dependency injection.
type SystemOps struct {
	Getwd        func() (string, error)
	Stat         func(string) (os.FileInfo, error)
	UserCacheDir func() (string, error)
}

// DefaultSystemOps returns the standard OS implementations.
func DefaultSystemOps() *SystemOps {
	return &SystemOps{
		Getwd:        os.Getwd,
		Stat:         os.Stat,
		UserCacheDir: os.UserCacheDir,
	}
}

// Newer reports whether inputs are newer than outputs, or when outputs are empty,
// whether the input match set or file modtimes changed since the last run.
// If ops is nil, DefaultSystemOps() is used.
// If fileOps is nil, DefaultFileOps() is used.
func Newer(
	inputs, outputs []string,
	matchFn func([]string) ([]string, error),
	ops *SystemOps,
	fileOps *FileOps,
) (bool, error) {
	if len(inputs) == 0 {
		return false, ErrNoPatterns
	}

	if ops == nil {
		ops = DefaultSystemOps()
	}

	if fileOps == nil {
		fileOps = DefaultFileOps()
	}

	if len(outputs) > 0 {
		return newerWithOutputs(inputs, outputs, matchFn, ops)
	}

	return newerWithCache(inputs, matchFn, ops, fileOps)
}

type newerCache struct {
	Pattern string           `json:"pattern"`
	CWD     string           `json:"cwd"`
	Matches []string         `json:"matches"`
	Files   map[string]int64 `json:"files"`
}

func anyOutputOlderThan(outputs []string, threshold time.Time, ops *SystemOps) bool {
	for _, path := range outputs {
		info, err := ops.Stat(path)
		if err != nil {
			return true
		}

		if info.ModTime().Before(threshold) {
			return true
		}
	}

	return false
}

func cacheEqual(a, b *newerCache) bool {
	if len(a.Matches) != len(b.Matches) {
		return false
	}

	for i := range a.Matches {
		if a.Matches[i] != b.Matches[i] {
			return false
		}
	}

	if len(a.Files) != len(b.Files) {
		return false
	}

	for path, mod := range a.Files {
		if b.Files[path] != mod {
			return false
		}
	}

	return true
}

func cacheFilePath(cwd, pattern string, ops *SystemOps, fileOps *FileOps) (string, error) {
	cacheDir, err := ops.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("getting user cache dir: %w", err)
	}

	encoded := hashString(cwd + "::" + pattern)

	dir := filepath.Join(cacheDir, "targ", "newer")
	//nolint:mnd // standard cache directory permissions
	err = fileOps.MkdirAll(dir, 0o755)
	if err != nil {
		return "", fmt.Errorf("creating cache directory: %w", err)
	}

	return filepath.Join(dir, encoded+".json"), nil
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func latestModTime(paths []string, ops *SystemOps) (time.Time, bool) {
	latest := time.Time{}

	for _, path := range paths {
		info, err := ops.Stat(path)
		if err != nil {
			return time.Time{}, true
		}

		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}

	return latest, false
}

func newerWithCache(
	inputs []string,
	matchFn func([]string) ([]string, error),
	ops *SystemOps,
	fileOps *FileOps,
) (bool, error) {
	cwd, err := ops.Getwd()
	if err != nil {
		return false, fmt.Errorf("getting working directory: %w", err)
	}

	changed := false

	for _, pattern := range inputs {
		cachePath, err := cacheFilePath(cwd, pattern, ops, fileOps)
		if err != nil {
			return false, err
		}

		prev, _ := readCache(cachePath, fileOps)

		next, err := snapshotPattern(cwd, pattern, matchFn, ops)
		if err != nil {
			return false, err
		}

		if prev == nil || !cacheEqual(prev, next) {
			changed = true
		}

		err = writeCache(cachePath, next, fileOps)
		if err != nil {
			return false, err
		}
	}

	return changed, nil
}

func newerWithOutputs(
	inputs, outputs []string,
	matchFn func([]string) ([]string, error),
	ops *SystemOps,
) (bool, error) {
	inMatches, err := matchFn(inputs)
	if err != nil {
		return false, err
	}

	outMatches, err := matchFn(outputs)
	if err != nil {
		return false, err
	}

	if len(outMatches) == 0 {
		return true, nil
	}

	latestInput, inputMissing := latestModTime(inMatches, ops)
	if inputMissing || latestInput.IsZero() {
		return true, nil
	}

	return anyOutputOlderThan(outMatches, latestInput, ops), nil
}

func readCache(path string, fileOps *FileOps) (*newerCache, error) {
	data, err := fileOps.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cache file: %w", err)
	}

	var cache newerCache

	err = json.Unmarshal(data, &cache)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling cache: %w", err)
	}

	return &cache, nil
}

func snapshotPattern(
	cwd, pattern string,
	matchFn func([]string) ([]string, error),
	ops *SystemOps,
) (*newerCache, error) {
	matches, err := matchFn([]string{pattern})
	if err != nil {
		return nil, err
	}

	files := make(map[string]int64, len(matches))
	for _, path := range matches {
		info, err := ops.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("getting file info for %s: %w", path, err)
		}

		files[path] = info.ModTime().UnixNano()
	}

	sort.Strings(matches)

	return &newerCache{
		Pattern: pattern,
		CWD:     cwd,
		Matches: matches,
		Files:   files,
	}, nil
}

func writeCache(path string, cache *newerCache, fileOps *FileOps) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}

	//nolint:mnd // standard cache file permissions
	err = fileOps.WriteFile(path, data, 0o644)
	if err != nil {
		return fmt.Errorf("writing cache file: %w", err)
	}

	return nil
}

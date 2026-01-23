package file

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

// Newer reports whether inputs are newer than outputs, or when outputs are empty,
// whether the input match set or file modtimes changed since the last run.
func Newer(inputs, outputs []string) (bool, error) {
	if len(inputs) == 0 {
		return false, ErrNoPatterns
	}

	if len(outputs) > 0 {
		return newerWithOutputs(inputs, outputs)
	}

	return newerWithCache(inputs)
}

// unexported variables.
var (
	getwd        = os.Getwd
	statFile     = os.Stat
	userCacheDir = os.UserCacheDir
)

type newerCache struct {
	Pattern string           `json:"pattern"`
	CWD     string           `json:"cwd"`
	Matches []string         `json:"matches"`
	Files   map[string]int64 `json:"files"`
}

func anyOutputOlderThan(outputs []string, threshold time.Time) bool {
	for _, path := range outputs {
		info, err := statFile(path)
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

func cacheFilePath(cwd, pattern string) (string, error) {
	cacheDir, err := userCacheDir()
	if err != nil {
		return "", fmt.Errorf("getting user cache dir: %w", err)
	}

	encoded := hashString(cwd + "::" + pattern)

	dir := filepath.Join(cacheDir, "targ", "newer")
	//nolint:mnd // standard cache directory permissions
	err = mkdirAll(dir, 0o755)
	if err != nil {
		return "", fmt.Errorf("creating cache directory: %w", err)
	}

	return filepath.Join(dir, encoded+".json"), nil
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func latestModTime(paths []string) (time.Time, bool) {
	latest := time.Time{}

	for _, path := range paths {
		info, err := statFile(path)
		if err != nil {
			return time.Time{}, true
		}

		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}

	return latest, false
}

func newerWithCache(inputs []string) (bool, error) {
	cwd, err := getwd()
	if err != nil {
		return false, fmt.Errorf("getting working directory: %w", err)
	}

	changed := false

	for _, pattern := range inputs {
		cachePath, err := cacheFilePath(cwd, pattern)
		if err != nil {
			return false, err
		}

		prev, _ := readCache(cachePath)

		next, err := snapshotPattern(cwd, pattern)
		if err != nil {
			return false, err
		}

		if prev == nil || !cacheEqual(prev, next) {
			changed = true
		}

		err = writeCache(cachePath, next)
		if err != nil {
			return false, err
		}
	}

	return changed, nil
}

func newerWithOutputs(inputs, outputs []string) (bool, error) {
	inMatches, err := Match(inputs...)
	if err != nil {
		return false, err
	}

	outMatches, err := Match(outputs...)
	if err != nil {
		return false, err
	}

	if len(outMatches) == 0 {
		return true, nil
	}

	latestInput, inputMissing := latestModTime(inMatches)
	if inputMissing || latestInput.IsZero() {
		return true, nil
	}

	return anyOutputOlderThan(outMatches, latestInput), nil
}

func readCache(path string) (*newerCache, error) {
	data, err := readFile(path)
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

func snapshotPattern(cwd, pattern string) (*newerCache, error) {
	matches, err := Match(pattern)
	if err != nil {
		return nil, err
	}

	files := make(map[string]int64, len(matches))
	for _, path := range matches {
		info, err := statFile(path)
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

func writeCache(path string, cache *newerCache) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}

	//nolint:mnd // standard cache file permissions
	err = writeFile(path, data, 0o644)
	if err != nil {
		return fmt.Errorf("writing cache file: %w", err)
	}

	return nil
}

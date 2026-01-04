package targ

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

type newerCache struct {
	Pattern string           `json:"pattern"`
	CWD     string           `json:"cwd"`
	Matches []string         `json:"matches"`
	Files   map[string]int64 `json:"files"`
}

// Newer reports whether inputs are newer than outputs, or when outputs are empty,
// whether the input match set or file modtimes changed since the last run.
func Newer(inputs []string, outputs []string) (bool, error) {
	if len(inputs) == 0 {
		return false, fmt.Errorf("no input patterns provided")
	}
	if len(outputs) > 0 {
		return newerWithOutputs(inputs, outputs)
	}
	return newerWithCache(inputs)
}

func newerWithOutputs(inputs []string, outputs []string) (bool, error) {
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
	latestInput := time.Time{}
	for _, path := range inMatches {
		info, err := os.Stat(path)
		if err != nil {
			return true, nil
		}
		if info.ModTime().After(latestInput) {
			latestInput = info.ModTime()
		}
	}
	if latestInput.IsZero() {
		return true, nil
	}
	for _, path := range outMatches {
		info, err := os.Stat(path)
		if err != nil {
			return true, nil
		}
		if info.ModTime().Before(latestInput) {
			return true, nil
		}
	}
	return false, nil
}

func newerWithCache(inputs []string) (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, err
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
		if err := writeCache(cachePath, next); err != nil {
			return false, err
		}
	}
	return changed, nil
}

func snapshotPattern(cwd string, pattern string) (*newerCache, error) {
	matches, err := Match(pattern)
	if err != nil {
		return nil, err
	}
	files := make(map[string]int64, len(matches))
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
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

func cacheEqual(a *newerCache, b *newerCache) bool {
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

func cacheFilePath(cwd string, pattern string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	encoded := hashString(cwd + "::" + pattern)
	dir := filepath.Join(cacheDir, "targ", "newer")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, encoded+".json"), nil
}

func readCache(path string) (*newerCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cache newerCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func writeCache(path string, cache *newerCache) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

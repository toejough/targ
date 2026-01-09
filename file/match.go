package file

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Match expands one or more patterns using fish-style globs (including ** and {a,b}).
func Match(patterns ...string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no patterns provided")
	}
	seen := make(map[string]bool)
	var matches []string
	for _, pattern := range patterns {
		pattern = filepath.Clean(pattern)
		fs, base := patternFS(pattern)
		expanded, err := expandBraces(pattern)
		if err != nil {
			return nil, err
		}
		for _, exp := range expanded {
			if base != "" {
				exp = filepath.Clean(strings.TrimPrefix(exp, base))
			}
			list, err := doublestar.Glob(fs, exp)
			if err != nil {
				return nil, err
			}
			for _, match := range list {
				path := match
				if base != "" {
					path = filepath.Join(base, match)
				}
				if !seen[path] {
					seen[path] = true
					matches = append(matches, path)
				}
			}
		}
	}
	sort.Strings(matches)
	return matches, nil
}

func patternFS(pattern string) (fs.FS, string) {
	if filepath.IsAbs(pattern) {
		volume := filepath.VolumeName(pattern)
		base := volume + string(filepath.Separator)
		return os.DirFS(base), base
	}
	return os.DirFS("."), ""
}

func expandBraces(pattern string) ([]string, error) {
	start := strings.Index(pattern, "{")
	if start == -1 {
		return []string{pattern}, nil
	}
	depth := 0
	for i := start; i < len(pattern); i++ {
		switch pattern[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				before := pattern[:start]
				after := pattern[i+1:]
				parts := splitBraceOptions(pattern[start+1 : i])
				var result []string
				for _, part := range parts {
					expanded, err := expandBraces(before + part + after)
					if err != nil {
						return nil, err
					}
					result = append(result, expanded...)
				}
				return result, nil
			}
		}
	}
	return nil, fmt.Errorf("unmatched brace in pattern %q", pattern)
}

func splitBraceOptions(content string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, content[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, content[start:])
	return parts
}

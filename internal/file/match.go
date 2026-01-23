package internal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Exported variables.
var (
	ErrNoPatterns     = errors.New("no patterns provided")
	ErrUnmatchedBrace = errors.New("unmatched brace in pattern")
)

// Match expands one or more patterns using fish-style globs (including ** and {a,b}).
func Match(patterns ...string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, ErrNoPatterns
	}

	seen := make(map[string]bool)

	var matches []string

	for _, pattern := range patterns {
		pattern = filepath.Clean(pattern)
		patternFileSys, base := patternFS(pattern)

		expanded, err := expandBraces(pattern)
		if err != nil {
			return nil, err
		}

		for _, exp := range expanded {
			if base != "" {
				exp = filepath.Clean(strings.TrimPrefix(exp, base))
			}

			list, err := doublestar.Glob(patternFileSys, exp)
			if err != nil {
				return nil, fmt.Errorf("matching pattern %q: %w", exp, err)
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

func expandBraces(pattern string) ([]string, error) {
	start := strings.Index(pattern, "{")
	if start == -1 {
		return []string{pattern}, nil
	}

	depth := 0

	for idx := start; idx < len(pattern); idx++ {
		switch pattern[idx] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				before := pattern[:start]
				after := pattern[idx+1:]
				parts := splitBraceOptions(pattern[start+1 : idx])

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

	return nil, fmt.Errorf("%w: %q", ErrUnmatchedBrace, pattern)
}

func patternFS(pattern string) (fs.FS, string) {
	if filepath.IsAbs(pattern) {
		volume := filepath.VolumeName(pattern)
		base := volume + string(filepath.Separator)

		return os.DirFS(base), base
	}

	return os.DirFS("."), ""
}

func splitBraceOptions(content string) []string {
	var parts []string

	depth := 0
	start := 0

	for idx := range len(content) {
		switch content[idx] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, content[start:idx])
				start = idx + 1
			}
		}
	}

	parts = append(parts, content[start:])

	return parts
}

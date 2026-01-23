package file

import (
	internal "github.com/toejough/targ/internal/file"
)

// Exported variables.
var (
	ErrNoPatterns     = internal.ErrNoPatterns
	ErrUnmatchedBrace = internal.ErrUnmatchedBrace
)

// Match expands one or more patterns using fish-style globs (including ** and {a,b}).
func Match(patterns ...string) ([]string, error) {
	return internal.Match(patterns...)
}

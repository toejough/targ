// Package file provides utilities for file operations in build scripts.
package file

import (
	internal "github.com/toejough/targ/internal/file"
)

// Exported variables.
var (
	ErrEmptyDest       = internal.ErrEmptyDest
	ErrNoInputPatterns = internal.ErrNoInputPatterns
)

// Checksum reports whether the content hash of inputs differs from the stored hash at dest.
// When the hash changes, the new hash is written to dest.
func Checksum(inputs []string, dest string) (bool, error) {
	return internal.Checksum(inputs, dest, func(patterns []string) ([]string, error) {
		return Match(patterns...)
	})
}

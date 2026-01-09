package targ

import (
	"context"

	"github.com/toejough/targ/file"
)

// Match expands one or more patterns using fish-style globs.
// Deprecated: Use file.Match instead.
func Match(patterns ...string) ([]string, error) {
	return file.Match(patterns...)
}

// Newer reports whether inputs are newer than outputs.
// Deprecated: Use file.Newer instead.
func Newer(inputs []string, outputs []string) (bool, error) {
	return file.Newer(inputs, outputs)
}

// ChangeSet contains the files that changed between snapshots.
// Deprecated: Use file.ChangeSet instead.
type ChangeSet = file.ChangeSet

// WatchOptions configures the Watch function.
// Deprecated: Use file.WatchOptions instead.
type WatchOptions = file.WatchOptions

// Watch polls patterns for changes and invokes fn with any detected changes.
// Deprecated: Use file.Watch instead.
func Watch(ctx context.Context, patterns []string, opts WatchOptions, fn func(ChangeSet) error) error {
	return file.Watch(ctx, patterns, opts, fn)
}

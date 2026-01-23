package file

import (
	"context"

	internal "github.com/toejough/targ/internal/file"
)

// ChangeSet holds the files that changed between watch polls.
type ChangeSet = internal.ChangeSet

// WatchOptions configures file watching behavior.
type WatchOptions = internal.WatchOptions

// Watch polls patterns for changes and invokes callback with any detected changes.
func Watch(
	ctx context.Context,
	patterns []string,
	opts WatchOptions,
	callback func(ChangeSet) error,
) error {
	return internal.Watch(ctx, patterns, opts, callback, func(p []string) ([]string, error) {
		return Match(p...)
	}, nil)
}

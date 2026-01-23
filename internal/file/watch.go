package internal

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"
)

// ChangeSet holds the files that changed between watch polls.
type ChangeSet struct {
	Added    []string
	Removed  []string
	Modified []string
}

// Ticker abstracts time.Ticker for testing.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// WatchOps provides watch operations for dependency injection.
type WatchOps struct {
	NewTicker func(time.Duration) Ticker
	Stat      func(string) (os.FileInfo, error)
}

// WatchOptions configures file watching behavior.
type WatchOptions struct {
	Interval time.Duration
}

// DefaultWatchOps returns the standard implementations.
func DefaultWatchOps() *WatchOps {
	return &WatchOps{
		NewTicker: func(d time.Duration) Ticker { return &realTicker{ticker: time.NewTicker(d)} },
		Stat:      os.Stat,
	}
}

// Watch polls patterns for changes and invokes callback with any detected changes.
// If ops is nil, DefaultWatchOps() is used.
func Watch(
	ctx context.Context,
	patterns []string,
	opts WatchOptions,
	callback func(ChangeSet) error,
	matchFn func([]string) ([]string, error),
	ops *WatchOps,
) error {
	if len(patterns) == 0 {
		return ErrNoPatterns
	}

	if ops == nil {
		ops = DefaultWatchOps()
	}

	interval := opts.Interval
	if interval == 0 {
		interval = defaultWatchInterval
	}

	prev, err := snapshot(patterns, matchFn, ops)
	if err != nil {
		return err
	}

	ticker := ops.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("watch cancelled: %w", ctx.Err())
		case <-ticker.C():
			prev, err = processTick(prev, patterns, matchFn, ops, callback)
			if err != nil {
				return err
			}
		}
	}
}

// unexported constants.
const (
	defaultWatchInterval = 250 * time.Millisecond
)

type fileSnapshot struct {
	Files map[string]int64
	List  []string
}

// realTicker wraps time.Ticker to implement Ticker interface.
type realTicker struct {
	ticker *time.Ticker
}

func (t *realTicker) C() <-chan time.Time { return t.ticker.C }

func (t *realTicker) Stop() { t.ticker.Stop() }

func diffSnapshot(prev, next *fileSnapshot) *ChangeSet {
	added := []string{}
	removed := []string{}
	modified := []string{}

	for _, path := range next.List {
		if _, ok := prev.Files[path]; !ok {
			added = append(added, path)
			continue
		}

		if prev.Files[path] != next.Files[path] {
			modified = append(modified, path)
		}
	}

	for _, path := range prev.List {
		if _, ok := next.Files[path]; !ok {
			removed = append(removed, path)
		}
	}

	if len(added) == 0 && len(removed) == 0 && len(modified) == 0 {
		return nil
	}

	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(modified)

	return &ChangeSet{
		Added:    added,
		Removed:  removed,
		Modified: modified,
	}
}

// processTick handles a single watch tick, returning the new snapshot if changes occurred.
func processTick(
	prev *fileSnapshot,
	patterns []string,
	matchFn func([]string) ([]string, error),
	ops *WatchOps,
	callback func(ChangeSet) error,
) (*fileSnapshot, error) {
	next, err := snapshot(patterns, matchFn, ops)
	if err != nil {
		return prev, err
	}

	changes := diffSnapshot(prev, next)
	if changes != nil {
		err := callback(*changes)
		if err != nil {
			return prev, err
		}

		return next, nil
	}

	return prev, nil
}

func snapshot(
	patterns []string,
	matchFn func([]string) ([]string, error),
	ops *WatchOps,
) (*fileSnapshot, error) {
	matches, err := matchFn(patterns)
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

	return &fileSnapshot{
		Files: files,
		List:  matches,
	}, nil
}

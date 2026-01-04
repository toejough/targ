package targ

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"
)

type ChangeSet struct {
	Added    []string
	Removed  []string
	Modified []string
}

type WatchOptions struct {
	Interval time.Duration
}

// Watch polls patterns for changes and invokes fn with any detected changes.
func Watch(ctx context.Context, patterns []string, opts WatchOptions, fn func(ChangeSet) error) error {
	if len(patterns) == 0 {
		return fmt.Errorf("no patterns provided")
	}
	interval := opts.Interval
	if interval == 0 {
		interval = 250 * time.Millisecond
	}
	prev, err := snapshot(patterns)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			next, err := snapshot(patterns)
			if err != nil {
				return err
			}
			changes := diffSnapshot(prev, next)
			if changes != nil {
				if err := fn(*changes); err != nil {
					return err
				}
				prev = next
			}
		}
	}
}

type fileSnapshot struct {
	Files map[string]int64
	List  []string
}

func snapshot(patterns []string) (*fileSnapshot, error) {
	matches, err := Match(patterns...)
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
	return &fileSnapshot{
		Files: files,
		List:  matches,
	}, nil
}

func diffSnapshot(prev *fileSnapshot, next *fileSnapshot) *ChangeSet {
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

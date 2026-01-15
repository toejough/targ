package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchDetectsAddModifyRemove(t *testing.T) {
	dir := t.TempDir()
	pattern := filepath.Join(dir, "*.txt")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changesCh := make(chan ChangeSet, 8)
	done := make(chan error, 1)

	interval := 20 * time.Millisecond

	go func() {
		done <- Watch(ctx, []string{pattern}, WatchOptions{Interval: interval}, func(set ChangeSet) error {
			changesCh <- set
			return nil
		})
	}()

	time.Sleep(2 * interval)

	file := filepath.Join(dir, "a.txt")

	performFileOperations(t, file)
	waitForAllChanges(t, changesCh, cancel)

	cancel()

	err := <-done
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
}

// performFileOperations creates, modifies, and removes a test file.
func performFileOperations(t *testing.T, file string) {
	t.Helper()

	requireNoError(t, os.WriteFile(file, []byte("one"), 0o644))
	time.Sleep(40 * time.Millisecond)

	requireNoError(t, os.WriteFile(file, []byte("two"), 0o644))
	time.Sleep(40 * time.Millisecond)

	requireNoError(t, os.Remove(file))
}

// requireNoError fails the test if err is not nil.
func requireNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// waitForAllChanges waits until Added, Modified, and Removed events are seen.
func waitForAllChanges(t *testing.T, changesCh <-chan ChangeSet, cancel context.CancelFunc) {
	t.Helper()

	var added, modified, removed bool

	timeout := time.After(800 * time.Millisecond)

	for !added || !modified || !removed {
		select {
		case set := <-changesCh:
			added = added || len(set.Added) > 0
			modified = modified || len(set.Modified) > 0
			removed = removed || len(set.Removed) > 0
		case <-timeout:
			cancel()
			t.Fatalf("watch timed out (added=%v modified=%v removed=%v)", added, modified, removed)
		}
	}
}

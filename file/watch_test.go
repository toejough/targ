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

func TestWatchReturnsErrorFromCallback(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	pattern := filepath.Join(dir, "*.txt")

	requireNoError(t, os.WriteFile(file, []byte("initial"), 0o644))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callbackErr := errors.New("callback error")
	done := make(chan error, 1)

	go func() {
		done <- Watch(ctx, []string{pattern}, WatchOptions{Interval: 10 * time.Millisecond}, func(ChangeSet) error {
			return callbackErr
		})
	}()

	time.Sleep(20 * time.Millisecond)

	// Modify file to trigger callback
	requireNoError(t, os.WriteFile(file, []byte("modified"), 0o644))

	select {
	case err := <-done:
		if !errors.Is(err, callbackErr) {
			t.Fatalf("expected callback error, got: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		cancel()
		t.Fatal("watch did not return callback error")
	}
}

func TestWatchReturnsErrorOnNoPatterns(t *testing.T) {
	err := Watch(context.Background(), nil, WatchOptions{}, func(ChangeSet) error { return nil })
	if err == nil || err.Error() != "no patterns provided" {
		t.Fatalf("expected 'no patterns provided' error, got: %v", err)
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

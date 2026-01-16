package file_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/toejough/targ/file"
)

func TestWatchDetectsAddModifyRemove(t *testing.T) {
	dir := t.TempDir()
	pattern := filepath.Join(dir, "*.txt")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changesCh := make(chan file.ChangeSet, 8)
	done := make(chan error, 1)

	interval := 20 * time.Millisecond

	go func() {
		done <- file.Watch(ctx, []string{pattern}, file.WatchOptions{Interval: interval}, func(set file.ChangeSet) error {
			changesCh <- set
			return nil
		})
	}()

	time.Sleep(2 * interval)

	f := filepath.Join(dir, "a.txt")

	performFileOperations(t, f)
	waitForAllChanges(t, changesCh, cancel)

	cancel()

	err := <-done
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWatchReturnsErrorFromCallback(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	pattern := filepath.Join(dir, "*.txt")

	requireNoError(t, os.WriteFile(f, []byte("initial"), 0o644))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callbackErr := errors.New("callback error")
	done := make(chan error, 1)

	go func() {
		opts := file.WatchOptions{Interval: 10 * time.Millisecond}
		done <- file.Watch(ctx, []string{pattern}, opts, func(file.ChangeSet) error {
			return callbackErr
		})
	}()

	time.Sleep(20 * time.Millisecond)

	// Modify file to trigger callback
	requireNoError(t, os.WriteFile(f, []byte("modified"), 0o644))

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
	err := file.Watch(context.Background(), nil, file.WatchOptions{}, func(file.ChangeSet) error { return nil })
	if !errors.Is(err, file.ErrNoPatterns) {
		t.Fatalf("expected ErrNoPatterns error, got: %v", err)
	}
}

// performFileOperations creates, modifies, and removes a test file.
func performFileOperations(t *testing.T, f string) {
	t.Helper()

	requireNoError(t, os.WriteFile(f, []byte("one"), 0o644))
	time.Sleep(40 * time.Millisecond)

	requireNoError(t, os.WriteFile(f, []byte("two"), 0o644))
	time.Sleep(40 * time.Millisecond)

	requireNoError(t, os.Remove(f))
}

// requireNoError fails the test if err is not nil.
func requireNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// waitForAllChanges waits until Added, Modified, and Removed events are seen.
func waitForAllChanges(t *testing.T, changesCh <-chan file.ChangeSet, cancel context.CancelFunc) {
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

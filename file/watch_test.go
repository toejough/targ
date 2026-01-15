package file

import (
	"context"
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
		err := Watch(
			ctx,
			[]string{pattern},
			WatchOptions{Interval: interval},
			func(set ChangeSet) error {
				changesCh <- set
				return nil
			},
		)
		done <- err
	}()

	time.Sleep(2 * interval)

	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("one"), 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	if err := os.WriteFile(file, []byte("two"), 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	if err := os.Remove(file); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	added := false
	modified := false
	removed := false
	timeout := time.After(800 * time.Millisecond)
	for !added || !modified || !removed {
		select {
		case set := <-changesCh:
			if len(set.Added) > 0 {
				added = true
			}
			if len(set.Modified) > 0 {
				modified = true
			}
			if len(set.Removed) > 0 {
				removed = true
			}
		case <-timeout:
			cancel()
			t.Fatalf("watch timed out (added=%v modified=%v removed=%v)", added, modified, removed)
		}
	}
	cancel()
	if err := <-done; err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
}

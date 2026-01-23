package file_test

import (
	"context"
	"errors"
	"io/fs"
	"sync"
	"testing"
	"time"

	"github.com/toejough/targ/file"
	internal "github.com/toejough/targ/internal/file"
)

func TestWatchDetectsAddedFile(t *testing.T) {
	ticker, state, restore := setupWatchMocks(t)
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changesCh := make(chan internal.ChangeSet, 10)
	done := make(chan error, 1)

	go func() {
		done <- runWatch(ctx, state, changesCh)
	}()

	time.Sleep(10 * time.Millisecond)

	// Add a file
	state.setFile("/test/a.txt", time.Now())
	state.setMatches([]string{"/test/a.txt"})
	ticker.tick()

	select {
	case set := <-changesCh:
		if len(set.Added) != 1 || set.Added[0] != "/test/a.txt" {
			t.Fatalf("expected added file, got: %+v", set)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for add change")
	}

	cancel()
	<-done
}

func TestWatchDetectsModifiedFile(t *testing.T) {
	ticker, state, restore := setupWatchMocks(t)
	defer restore()

	// Start with a file already present
	state.setFile("/test/a.txt", time.Now())
	state.setMatches([]string{"/test/a.txt"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changesCh := make(chan internal.ChangeSet, 10)
	done := make(chan error, 1)

	go func() {
		done <- runWatch(ctx, state, changesCh)
	}()

	time.Sleep(10 * time.Millisecond)

	// Modify the file
	state.setFile("/test/a.txt", time.Now().Add(time.Second))
	ticker.tick()

	select {
	case set := <-changesCh:
		if len(set.Modified) != 1 || set.Modified[0] != "/test/a.txt" {
			t.Fatalf("expected modified file, got: %+v", set)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for modify change")
	}

	cancel()
	<-done
}

func TestWatchDetectsRemovedFile(t *testing.T) {
	ticker, state, restore := setupWatchMocks(t)
	defer restore()

	// Start with a file already present
	state.setFile("/test/a.txt", time.Now())
	state.setMatches([]string{"/test/a.txt"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changesCh := make(chan internal.ChangeSet, 10)
	done := make(chan error, 1)

	go func() {
		done <- runWatch(ctx, state, changesCh)
	}()

	time.Sleep(10 * time.Millisecond)

	// Remove the file
	state.deleteFile("/test/a.txt")
	state.setMatches([]string{})
	ticker.tick()

	select {
	case set := <-changesCh:
		if len(set.Removed) != 1 || set.Removed[0] != "/test/a.txt" {
			t.Fatalf("expected removed file, got: %+v", set)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for remove change")
	}

	cancel()
	<-done
}

func TestWatchReturnsErrorFromCallback(t *testing.T) {
	ticker, state, restore := setupWatchMocks(t)
	defer restore()

	state.setFile("/test/test.txt", time.Now())
	state.setMatches([]string{"/test/test.txt"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callbackErr := errors.New("callback error")
	done := make(chan error, 1)

	go func() {
		opts := internal.WatchOptions{Interval: time.Millisecond}
		done <- internal.Watch(ctx, []string{"*.txt"}, opts, func(internal.ChangeSet) error {
			return callbackErr
		}, state.match)
	}()

	time.Sleep(10 * time.Millisecond)

	// Modify file to trigger callback
	state.setFile("/test/test.txt", time.Now().Add(time.Second))
	ticker.tick()

	select {
	case err := <-done:
		if !errors.Is(err, callbackErr) {
			t.Fatalf("expected callback error, got: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		cancel()
		t.Fatal("watch did not return callback error")
	}
}

func TestWatchReturnsErrorOnNoPatterns(t *testing.T) {
	err := file.Watch(
		context.Background(),
		nil,
		file.WatchOptions{},
		func(file.ChangeSet) error { return nil },
	)
	if !errors.Is(err, file.ErrNoPatterns) {
		t.Fatalf("expected ErrNoPatterns error, got: %v", err)
	}
}

// fileState holds mock file system state with thread-safe access.
type fileState struct {
	mu      sync.RWMutex
	files   map[string]time.Time
	matches []string
}

func (s *fileState) deleteFile(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.files, path)
}

func (s *fileState) match(_ []string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, len(s.matches))
	copy(result, s.matches)

	return result, nil
}

func (s *fileState) setFile(path string, modTime time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.files[path] = modTime
}

func (s *fileState) setMatches(matches []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.matches = matches
}

func (s *fileState) stat(path string) (fs.FileInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if modTime, ok := s.files[path]; ok {
		return mockFileInfo{modTime: modTime}, nil
	}

	return nil, fs.ErrNotExist
}

// mockFileInfo implements fs.FileInfo for testing.
type mockFileInfo struct {
	modTime time.Time
}

func (m mockFileInfo) IsDir() bool { return false }

func (m mockFileInfo) ModTime() time.Time { return m.modTime }

func (m mockFileInfo) Mode() fs.FileMode { return 0 }

func (m mockFileInfo) Name() string { return "test" }

func (m mockFileInfo) Size() int64 { return 0 }

func (m mockFileInfo) Sys() any { return nil }

// mockTicker is a controllable ticker for deterministic testing.
type mockTicker struct {
	ch chan time.Time
}

func (t *mockTicker) C() <-chan time.Time { return t.ch }

func (t *mockTicker) Stop() {}

// tick sends a tick to the mock ticker.
func (t *mockTicker) tick() { t.ch <- time.Now() }

// runWatch runs the watch with standard options.
func runWatch(ctx context.Context, state *fileState, changesCh chan<- internal.ChangeSet) error {
	opts := internal.WatchOptions{Interval: time.Millisecond}

	return internal.Watch(ctx, []string{"*.txt"}, opts, func(set internal.ChangeSet) error {
		changesCh <- set
		return nil
	}, state.match)
}

// setupWatchMocks configures DI points for watch testing.
// Returns the mock ticker, file state, and a cleanup function.
func setupWatchMocks(t *testing.T) (*mockTicker, *fileState, func()) {
	t.Helper()

	origNewTicker := internal.NewTicker
	origStatFile := internal.StatFile

	ticker := &mockTicker{ch: make(chan time.Time)}
	internal.NewTicker = func(time.Duration) internal.Ticker { return ticker }

	state := &fileState{files: make(map[string]time.Time)}
	internal.StatFile = state.stat

	restore := func() {
		internal.NewTicker = origNewTicker
		internal.StatFile = origStatFile
	}

	return ticker, state, restore
}

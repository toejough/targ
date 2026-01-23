package file_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toejough/targ/file"
	internal "github.com/toejough/targ/internal/file"
)

func TestNewerRequiresInputs(t *testing.T) {
	t.Parallel()

	_, err := file.Newer(nil, nil)
	if err == nil {
		t.Fatal("expected error for empty inputs")
	}
}

func TestNewerWithCacheTracksChanges(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	dir := t.TempDir()

	f := filepath.Join(dir, "main.go")

	err := os.WriteFile(f, []byte("one"), 0o644)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err := file.Newer([]string{f}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("expected first run to report changed")
	}

	changed, err = file.Newer([]string{f}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if changed {
		t.Fatal("expected no change on second run")
	}

	time.Sleep(10 * time.Millisecond)

	err = os.WriteFile(f, []byte("two"), 0o644)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	future := time.Now().Add(2 * time.Second)

	err = os.Chtimes(f, future, future)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err = file.Newer([]string{f}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("expected change after modification")
	}
}

func TestNewerWithOutputs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	input := filepath.Join(dir, "input.txt")
	output := filepath.Join(dir, "output.txt")

	writeFile(t, input, "one")

	assertNewer(t, []string{input}, []string{output}, true, "expected change when output missing")

	time.Sleep(10 * time.Millisecond)
	writeFile(t, output, "out")
	setFutureTime(t, output, 2*time.Second)

	assertNewer(t, []string{input}, []string{output}, false, "expected output to be up-to-date")

	time.Sleep(10 * time.Millisecond)
	writeFile(t, input, "two")
	setFutureTime(t, input, 3*time.Second)

	assertNewer(t, []string{input}, []string{output}, true, "expected change when input newer")
}

func TestNewerWithOutputs_InputPatternMatchesNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	output := filepath.Join(dir, "output.txt")

	writeFile(t, output, "out")

	// Input pattern matches nothing - should return true (need to rebuild)
	nonexistent := filepath.Join(dir, "*.nonexistent")
	assertNewer(
		t,
		[]string{nonexistent},
		[]string{output},
		true,
		"expected change when no inputs match",
	)
}

func TestNewer_CacheDetectsAddedFile(t *testing.T) {
	// Tests cacheEqual branch where match count differs
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	dir := t.TempDir()

	firstFile := filepath.Join(dir, "a.go")
	writeFile(t, firstFile, "one")

	pattern := filepath.Join(dir, "*.go")

	// First run: 1 file
	changed, err := file.Newer([]string{pattern}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("expected first run to report changed")
	}

	// Second run: same file, no change
	changed, err = file.Newer([]string{pattern}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if changed {
		t.Fatal("expected no change on second run")
	}

	// Add a new file - triggers cacheEqual mismatch on Matches length
	newFile := filepath.Join(dir, "b.go")
	writeFile(t, newFile, "two")

	changed, err = file.Newer([]string{pattern}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("expected change after adding file")
	}
}

func TestNewer_CacheDetectsRemovedFile(t *testing.T) {
	// Tests cacheEqual branch where file count differs
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	dir := t.TempDir()

	firstFile := filepath.Join(dir, "a.go")
	secondFile := filepath.Join(dir, "b.go")

	writeFile(t, firstFile, "one")
	writeFile(t, secondFile, "two")

	pattern := filepath.Join(dir, "*.go")

	// First run: 2 files
	changed, err := file.Newer([]string{pattern}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("expected first run to report changed")
	}

	// Second run: same files, no change
	changed, err = file.Newer([]string{pattern}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if changed {
		t.Fatal("expected no change on second run")
	}

	// Remove a file - triggers cacheEqual mismatch on Files length
	if err := os.Remove(secondFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err = file.Newer([]string{pattern}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("expected change after removing file")
	}
}

func TestNewer_CacheMkdirError(t *testing.T) {
	t.Parallel()

	sysOps := internal.DefaultSystemOps()
	sysOps.UserCacheDir = func() (string, error) { return "/cache", nil }

	fileOps := internal.DefaultFileOps()
	fileOps.MkdirAll = func(_ string, _ fs.FileMode) error {
		return errors.New("mkdir error")
	}

	matchFn := func(patterns []string) ([]string, error) { return patterns, nil }

	_, err := internal.Newer([]string{"*.go"}, nil, matchFn, sysOps, fileOps)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "creating cache directory") {
		t.Errorf("expected mkdir error, got: %v", err)
	}
}

// DI-based error path tests

func TestNewer_GetwdError(t *testing.T) {
	t.Parallel()

	sysOps := internal.DefaultSystemOps()
	sysOps.Getwd = func() (string, error) {
		return "", errors.New("getwd error")
	}

	matchFn := func(patterns []string) ([]string, error) { return patterns, nil }

	// Empty outputs triggers cache mode which uses Getwd
	_, err := internal.Newer([]string{"*.go"}, nil, matchFn, sysOps, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "getting working directory") {
		t.Errorf("expected getwd error, got: %v", err)
	}
}

func TestNewer_StatFileError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")

	// Create a file so Match returns something
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	sysOps := internal.DefaultSystemOps()
	sysOps.Stat = func(_ string) (os.FileInfo, error) {
		return nil, errors.New("stat error")
	}

	// Return the actual file path, not the pattern
	matchFn := func(_ []string) ([]string, error) { return []string{testFile}, nil }

	// Empty outputs triggers cache mode which stats files
	_, err := internal.Newer([]string{filepath.Join(dir, "*.txt")}, nil, matchFn, sysOps, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "getting file info") {
		t.Errorf("expected stat error, got: %v", err)
	}
}

func TestNewer_UserCacheDirError(t *testing.T) {
	t.Parallel()

	sysOps := internal.DefaultSystemOps()
	sysOps.UserCacheDir = func() (string, error) {
		return "", errors.New("cache dir error")
	}

	matchFn := func(patterns []string) ([]string, error) { return patterns, nil }

	// Empty outputs triggers cache mode
	_, err := internal.Newer([]string{"*.go"}, nil, matchFn, sysOps, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "getting user cache dir") {
		t.Errorf("expected cache dir error, got: %v", err)
	}
}

func TestNewer_WriteCacheError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")

	// Create a file so Match returns something
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	fileOps := internal.DefaultFileOps()
	fileOps.WriteFile = func(_ string, _ []byte, _ fs.FileMode) error {
		return errors.New("write error")
	}

	// Return the actual file path, not the pattern
	matchFn := func(_ []string) ([]string, error) { return []string{testFile}, nil }

	_, err := internal.Newer([]string{filepath.Join(dir, "*.txt")}, nil, matchFn, nil, fileOps)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "writing cache file") {
		t.Errorf("expected write error, got: %v", err)
	}
}

// assertNewer checks Newer result and fails with msg if expectation not met.
func assertNewer(t *testing.T, inputs, outputs []string, expectChanged bool, msg string) {
	t.Helper()

	changed, err := file.Newer(inputs, outputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if changed != expectChanged {
		t.Fatal(msg)
	}
}

// setFutureTime sets a file's mod time to now + offset.
func setFutureTime(t *testing.T, path string, offset time.Duration) {
	t.Helper()

	future := time.Now().Add(offset)

	err := os.Chtimes(path, future, future)
	if err != nil {
		t.Fatalf("unexpected error setting time on %s: %v", path, err)
	}
}

// writeFile writes content to a file, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("unexpected error writing %s: %v", path, err)
	}
}

package file_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/toejough/targ/file"
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

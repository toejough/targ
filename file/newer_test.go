package file

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewerRequiresInputs(t *testing.T) {
	if _, err := Newer(nil, nil); err == nil {
		t.Fatal("expected error for empty inputs")
	}
}

func TestNewerWithCacheTracksChanges(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte("one"), 0644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err := Newer([]string{file}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("expected first run to report changed")
	}

	changed, err = Newer([]string{file}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatal("expected no change on second run")
	}

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(file, []byte("two"), 0644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(file, future, future); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err = Newer([]string{file}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("expected change after modification")
	}
}

func TestNewerWithOutputs(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.txt")
	output := filepath.Join(dir, "output.txt")
	if err := os.WriteFile(input, []byte("one"), 0644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err := Newer([]string{input}, []string{output})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("expected change when output missing")
	}

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(output, []byte("out"), 0644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(output, future, future); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err = Newer([]string{input}, []string{output})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatal("expected output to be up-to-date")
	}

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(input, []byte("two"), 0644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	future = time.Now().Add(3 * time.Second)
	if err := os.Chtimes(input, future, future); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err = Newer([]string{input}, []string{output})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("expected change when input newer")
	}
}

package commander

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMatchBraceAndGlob(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		filepath.Join(dir, "a.txt"),
		filepath.Join(dir, "b.txt"),
		filepath.Join(dir, "c.md"),
		filepath.Join(dir, "dir", "d.txt"),
	}
	if err := os.MkdirAll(filepath.Join(dir, "dir"), 0755); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, path := range files {
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	matches, err := Match(filepath.Join(dir, "{a,b}.txt"), filepath.Join(dir, "**", "*.txt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		filepath.Join(dir, "a.txt"),
		filepath.Join(dir, "b.txt"),
		filepath.Join(dir, "dir", "d.txt"),
	}
	if !reflect.DeepEqual(matches, expected) {
		t.Fatalf("expected matches %v, got %v", expected, matches)
	}
}

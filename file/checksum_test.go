package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChecksumDetectsChanges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	input := filepath.Join(dir, "a.txt")
	dest := filepath.Join(dir, "hash.txt")

	if err := os.WriteFile(input, []byte("one"), 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err := Checksum([]string{input}, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("expected first checksum to report change")
	}

	changed, err = Checksum([]string{input}, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if changed {
		t.Fatal("expected checksum to be unchanged")
	}

	if err := os.WriteFile(input, []byte("two"), 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err = Checksum([]string{input}, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("expected checksum to change after edits")
	}
}

func TestChecksumDetectsMatchSetChanges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dest := filepath.Join(dir, "hash.txt")
	pattern := filepath.Join(dir, "*.txt")

	first := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(first, []byte("one"), 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err := Checksum([]string{pattern}, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("expected initial checksum change")
	}

	second := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(second, []byte("one"), 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err = Checksum([]string{pattern}, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("expected checksum to change on new match")
	}
}

func TestChecksumRequiresInputs(t *testing.T) {
	t.Parallel()

	_, err := Checksum(nil, "dest")
	if err == nil {
		t.Fatal("expected error for empty inputs")
	}

	_, err = Checksum([]string{"file"}, "")
	if err == nil {
		t.Fatal("expected error for empty dest")
	}
}

package file_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/toejough/targ/file"
)

func TestChecksumDetectsChanges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	input := filepath.Join(dir, "a.txt")
	dest := filepath.Join(dir, "hash.txt")

	if err := os.WriteFile(input, []byte("one"), 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err := file.Checksum([]string{input}, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("expected first checksum to report change")
	}

	changed, err = file.Checksum([]string{input}, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if changed {
		t.Fatal("expected checksum to be unchanged")
	}

	if err := os.WriteFile(input, []byte("two"), 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	changed, err = file.Checksum([]string{input}, dest)
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

	changed, err := file.Checksum([]string{pattern}, dest)
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

	changed, err = file.Checksum([]string{pattern}, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !changed {
		t.Fatal("expected checksum to change on new match")
	}
}

func TestChecksumRequiresInputs(t *testing.T) {
	t.Parallel()

	_, err := file.Checksum(nil, "dest")
	if err == nil {
		t.Fatal("expected error for empty inputs")
	}

	_, err = file.Checksum([]string{"file"}, "")
	if err == nil {
		t.Fatal("expected error for empty dest")
	}
}

func TestChecksum_MatchError(t *testing.T) {
	t.Parallel()

	// Invalid pattern (unmatched brace) should error
	_, err := file.Checksum([]string{"{unmatched"}, "/some/dest")
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

func TestChecksum_WriteError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	input := filepath.Join(dir, "a.txt")

	if err := os.WriteFile(input, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}

	// Create a read-only directory for dest
	readOnlyDir := filepath.Join(dir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0o555); err != nil {
		t.Fatalf("failed to create read-only dir: %v", err)
	}

	// Dest in read-only directory should fail to write
	dest := filepath.Join(readOnlyDir, "subdir", "hash.txt")

	_, err := file.Checksum([]string{input}, dest)
	if err == nil {
		t.Fatal("expected error writing to read-only directory")
	}
}

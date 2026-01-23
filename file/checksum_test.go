package file_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toejough/targ/file"
	internal "github.com/toejough/targ/internal/file"
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

func TestChecksum_CloseFileError(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	orig := internal.OpenFile

	defer func() { internal.OpenFile = orig }()

	internal.OpenFile = func(_ string) (io.ReadCloser, error) {
		return errorCloser{Reader: strings.NewReader("content")}, nil
	}

	_, err := file.Checksum([]string{testFile}, filepath.Join(dir, "checksum.txt"))
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "closing") {
		t.Errorf("expected closing error, got: %v", err)
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

func TestChecksum_MkdirError(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	orig := internal.MkdirAll

	defer func() { internal.MkdirAll = orig }()

	internal.MkdirAll = func(_ string, _ fs.FileMode) error {
		return errors.New("mkdir error")
	}

	// Use a nested path to trigger mkdir
	_, err := file.Checksum([]string{testFile}, filepath.Join(dir, "nested", "checksum.txt"))
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "creating checksum directory") {
		t.Errorf("expected mkdir error, got: %v", err)
	}
}

// DI-based error path tests

func TestChecksum_OpenFileError(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	orig := internal.OpenFile

	defer func() { internal.OpenFile = orig }()

	internal.OpenFile = func(_ string) (io.ReadCloser, error) {
		return nil, errors.New("open error")
	}

	_, err := file.Checksum([]string{testFile}, filepath.Join(dir, "checksum.txt"))
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "opening") {
		t.Errorf("expected opening error, got: %v", err)
	}
}

func TestChecksum_ReadExistingChecksumError(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	checksumFile := filepath.Join(dir, "checksum.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create existing checksum file
	if err := os.WriteFile(checksumFile, []byte("oldhash"), 0o644); err != nil {
		t.Fatalf("failed to create checksum file: %v", err)
	}

	orig := internal.ReadFile

	defer func() { internal.ReadFile = orig }()

	internal.ReadFile = func(_ string) ([]byte, error) {
		return nil, errors.New("permission denied")
	}

	_, err := file.Checksum([]string{testFile}, checksumFile)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected permission denied error, got: %v", err)
	}
}

func TestChecksum_ReadFileError(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	orig := internal.OpenFile

	defer func() { internal.OpenFile = orig }()

	internal.OpenFile = func(_ string) (io.ReadCloser, error) {
		return errorReader{}, nil
	}

	_, err := file.Checksum([]string{testFile}, filepath.Join(dir, "checksum.txt"))
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("expected reading error, got: %v", err)
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

func TestChecksum_WriteFileError(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	orig := internal.WriteFile

	defer func() { internal.WriteFile = orig }()

	internal.WriteFile = func(_ string, _ []byte, _ fs.FileMode) error {
		return errors.New("write error")
	}

	_, err := file.Checksum([]string{testFile}, filepath.Join(dir, "checksum.txt"))
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "writing checksum file") {
		t.Errorf("expected write error, got: %v", err)
	}
}

// Helper types for error injection

type errorCloser struct {
	io.Reader
}

func (errorCloser) Close() error {
	return errors.New("close error")
}

type errorReader struct{}

func (errorReader) Close() error {
	return nil
}

func (errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read error")
}

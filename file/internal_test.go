package file

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestCacheEqual_FileModTimesDiffer(t *testing.T) {
	a := &newerCache{
		Matches: []string{"a.txt"},
		Files:   map[string]int64{"a.txt": 100},
	}
	b := &newerCache{
		Matches: []string{"a.txt"},
		Files:   map[string]int64{"a.txt": 200}, // Different mod time
	}

	if cacheEqual(a, b) {
		t.Error("expected caches with different mod times to not be equal")
	}
}

func TestCacheEqual_MatchesDiffer(t *testing.T) {
	a := &newerCache{
		Matches: []string{"a.txt", "b.txt"},
		Files:   map[string]int64{"a.txt": 100, "b.txt": 200},
	}
	b := &newerCache{
		Matches: []string{"a.txt", "c.txt"}, // Different second match
		Files:   map[string]int64{"a.txt": 100, "c.txt": 200},
	}

	if cacheEqual(a, b) {
		t.Error("expected caches with different matches to not be equal")
	}
}

func TestCacheFilePath_MkdirError(t *testing.T) {
	origUserCacheDir := userCacheDir
	origMkdirAll := mkdirAll

	defer func() {
		userCacheDir = origUserCacheDir
		mkdirAll = origMkdirAll
	}()

	userCacheDir = func() (string, error) { return "/cache", nil }
	mkdirAll = func(_ string, _ fs.FileMode) error {
		return errors.New("mkdir error")
	}

	_, err := cacheFilePath("/cwd", "*.go")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "creating cache directory") {
		t.Errorf("expected mkdir error, got: %v", err)
	}
}

func TestCacheFilePath_UserCacheDirError(t *testing.T) {
	origUserCacheDir := userCacheDir

	defer func() { userCacheDir = origUserCacheDir }()

	userCacheDir = func() (string, error) {
		return "", errors.New("cache dir error")
	}

	_, err := cacheFilePath("/cwd", "*.go")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "getting user cache dir") {
		t.Errorf("expected cache dir error, got: %v", err)
	}
}

func TestComputeChecksum_CloseError(t *testing.T) {
	origOpenFile := openFile

	defer func() { openFile = origOpenFile }()

	openFile = func(_ string) (io.ReadCloser, error) {
		return errorCloser{Reader: strings.NewReader("content")}, nil
	}

	_, err := computeChecksum([]string{"test.txt"})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "closing") {
		t.Errorf("expected closing error, got: %v", err)
	}
}

func TestComputeChecksum_OpenError(t *testing.T) {
	origOpenFile := openFile

	defer func() { openFile = origOpenFile }()

	openFile = func(_ string) (io.ReadCloser, error) {
		return nil, errors.New("open error")
	}

	_, err := computeChecksum([]string{"test.txt"})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "opening") {
		t.Errorf("expected opening error, got: %v", err)
	}
}

func TestComputeChecksum_ReadError(t *testing.T) {
	origOpenFile := openFile

	defer func() { openFile = origOpenFile }()

	openFile = func(_ string) (io.ReadCloser, error) {
		return errorReader{}, nil
	}

	_, err := computeChecksum([]string{"test.txt"})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("expected reading error, got: %v", err)
	}
}

func TestNewerWithCache_GetwdError(t *testing.T) {
	origGetwd := getwd

	defer func() { getwd = origGetwd }()

	getwd = func() (string, error) {
		return "", errors.New("getwd error")
	}

	_, err := newerWithCache([]string{"*.go"})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "getting working directory") {
		t.Errorf("expected getwd error, got: %v", err)
	}
}

func TestNewerWithOutputs_InputMatchError(t *testing.T) {
	// Unmatched brace causes Match to return an error
	_, err := newerWithOutputs([]string{"{unmatched"}, []string{"output.txt"})
	if err == nil {
		t.Fatal("expected error for invalid input pattern")
	}
}

func TestNewerWithOutputs_OutputMatchError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a valid input file so input matching succeeds
	if err := os.WriteFile(dir+"/input.txt", []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create input file: %v", err)
	}

	// Unmatched brace in output causes Match to return an error
	_, err := newerWithOutputs([]string{dir + "/input.txt"}, []string{"{unmatched"})
	if err == nil {
		t.Fatal("expected error for invalid output pattern")
	}
}

func TestReadChecksum_Error(t *testing.T) {
	// Not parallel - modifies global readFile
	origReadFile := readFile

	defer func() { readFile = origReadFile }()

	readFile = func(_ string) ([]byte, error) {
		return nil, errors.New("permission denied")
	}

	_, err := readChecksum("/some/path")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected permission denied error, got: %v", err)
	}
}

func TestSnapshotPattern_StatError(t *testing.T) {
	// Not parallel - modifies global statFile
	dir := t.TempDir()

	// Create a file so Match returns something
	if err := os.WriteFile(dir+"/test.txt", []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	origStatFile := statFile

	defer func() { statFile = origStatFile }()

	statFile = func(_ string) (os.FileInfo, error) {
		return nil, errors.New("stat error")
	}

	_, err := snapshotPattern(dir, dir+"/*.txt")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "getting file info") {
		t.Errorf("expected stat error, got: %v", err)
	}
}

func TestSplitBraceOptions_NestedBraces(t *testing.T) {
	t.Parallel()

	// Commas inside braces should not split
	parts := splitBraceOptions("a,{b,c},d")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d: %v", len(parts), parts)
	}

	if parts[0] != "a" || parts[1] != "{b,c}" || parts[2] != "d" {
		t.Errorf("unexpected parts: %v", parts)
	}
}

func TestSplitBraceOptions_Simple(t *testing.T) {
	t.Parallel()

	parts := splitBraceOptions("a,b,c")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d: %v", len(parts), parts)
	}

	if parts[0] != "a" || parts[1] != "b" || parts[2] != "c" {
		t.Errorf("unexpected parts: %v", parts)
	}
}

func TestSplitBraceOptions_UnmatchedCloseBrace(t *testing.T) {
	t.Parallel()

	// Unmatched close brace at depth 0 should be ignored
	parts := splitBraceOptions("a},b")
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d: %v", len(parts), parts)
	}

	if parts[0] != "a}" || parts[1] != "b" {
		t.Errorf("unexpected parts: %v", parts)
	}
}

func TestWriteCache_WriteError(t *testing.T) {
	origWriteFile := writeFile

	defer func() { writeFile = origWriteFile }()

	writeFile = func(_ string, _ []byte, _ fs.FileMode) error {
		return errors.New("write error")
	}

	err := writeCache("/cache.json", &newerCache{})
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "writing cache file") {
		t.Errorf("expected write error, got: %v", err)
	}
}

func TestWriteChecksum_MkdirError(t *testing.T) {
	origMkdirAll := mkdirAll

	defer func() { mkdirAll = origMkdirAll }()

	mkdirAll = func(_ string, _ fs.FileMode) error {
		return errors.New("mkdir error")
	}

	err := writeChecksum("/some/dir/checksum.txt", "hash")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "creating checksum directory") {
		t.Errorf("expected mkdir error, got: %v", err)
	}
}

func TestWriteChecksum_WriteError(t *testing.T) {
	origMkdirAll := mkdirAll
	origWriteFile := writeFile

	defer func() {
		mkdirAll = origMkdirAll
		writeFile = origWriteFile
	}()

	mkdirAll = func(_ string, _ fs.FileMode) error { return nil }
	writeFile = func(_ string, _ []byte, _ fs.FileMode) error {
		return errors.New("write error")
	}

	err := writeChecksum("/some/dir/checksum.txt", "hash")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "writing checksum file") {
		t.Errorf("expected write error, got: %v", err)
	}
}

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

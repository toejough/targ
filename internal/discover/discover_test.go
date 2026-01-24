package discover_test

// LEGACY_TESTS: This file contains tests being evaluated for redundancy.
// Property-based replacements are in *_properties_test.go files.
// Do not add new tests here. See docs/test-migration.md for details.

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/toejough/targ/internal/discover"
)

func TestDiscover_DetectsAliasedImport(t *testing.T) {
	t.Parallel()

	// File with aliased targ import
	srcWithAlias := `//go:build targ

package build

import t "github.com/toejough/targ"

func init() {
	t.Register(Build)
}

var Build = t.Targ(func() {})
`

	filesystem := &mockFileSystem{
		files: map[string][]byte{
			"test/targs.go": []byte(srcWithAlias),
		},
		dirs: map[string][]fs.DirEntry{
			".":    {mockDirEntry{name: "test", isDir: true}},
			"test": {mockDirEntry{name: "targs.go", isDir: false}},
		},
	}

	infos, err := discover.Discover(filesystem, discover.Options{StartDir: ".", BuildTag: "targ"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package, got %d", len(infos))
	}

	if !infos[0].UsesExplicitRegistration {
		t.Error("expected UsesExplicitRegistration to be true with aliased import")
	}
}

func TestDiscover_DetectsExplicitRegistration(t *testing.T) {
	t.Parallel()

	// File with targ.Register() call
	srcWithRegister := `//go:build targ

package build

import "github.com/toejough/targ"

func init() {
	targ.Register(Build)
}

var Build = targ.Targ(func() {})
`

	filesystem := &mockFileSystem{
		files: map[string][]byte{
			"test/targs.go": []byte(srcWithRegister),
		},
		dirs: map[string][]fs.DirEntry{
			".":    {mockDirEntry{name: "test", isDir: true}},
			"test": {mockDirEntry{name: "targs.go", isDir: false}},
		},
	}

	infos, err := discover.Discover(filesystem, discover.Options{StartDir: ".", BuildTag: "targ"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package, got %d", len(infos))
	}

	if !infos[0].UsesExplicitRegistration {
		t.Error("expected UsesExplicitRegistration to be true")
	}
}

func TestDiscover_DetectsNoExplicitRegistration(t *testing.T) {
	t.Parallel()

	// File without targ.Register() call
	srcWithoutRegister := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

	filesystem := &mockFileSystem{
		files: map[string][]byte{
			"test/targs.go": []byte(srcWithoutRegister),
		},
		dirs: map[string][]fs.DirEntry{
			".":    {mockDirEntry{name: "test", isDir: true}},
			"test": {mockDirEntry{name: "targs.go", isDir: false}},
		},
	}

	infos, err := discover.Discover(filesystem, discover.Options{StartDir: ".", BuildTag: "targ"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package, got %d", len(infos))
	}

	if infos[0].UsesExplicitRegistration {
		t.Error("expected UsesExplicitRegistration to be false")
	}
}

func TestDiscover_NonRegisterCallNotDetected(t *testing.T) {
	t.Parallel()

	// File with init() but no Register call
	srcWithOtherInit := `//go:build targ

package build

import "fmt"
import "github.com/toejough/targ"

func init() {
	fmt.Println("hello")
}

var Build = targ.Targ(func() {})
`

	filesystem := &mockFileSystem{
		files: map[string][]byte{
			"test/targs.go": []byte(srcWithOtherInit),
		},
		dirs: map[string][]fs.DirEntry{
			".":    {mockDirEntry{name: "test", isDir: true}},
			"test": {mockDirEntry{name: "targs.go", isDir: false}},
		},
	}

	infos, err := discover.Discover(filesystem, discover.Options{StartDir: ".", BuildTag: "targ"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 package, got %d", len(infos))
	}

	if infos[0].UsesExplicitRegistration {
		t.Error("expected UsesExplicitRegistration to be false when no Register call")
	}
}

// unexported variables.
var (
	errInfoNotImplemented = errors.New("Info() not implemented in mock")
)

// mockDirEntry implements fs.DirEntry for testing.
type mockDirEntry struct {
	name  string
	isDir bool
}

func (m mockDirEntry) Info() (fs.FileInfo, error) { return nil, errInfoNotImplemented }

func (m mockDirEntry) IsDir() bool { return m.isDir }

func (m mockDirEntry) Name() string { return m.name }

func (m mockDirEntry) Type() fs.FileMode { return 0 }

// mockFileSystem is a simple in-memory file system for testing.
type mockFileSystem struct {
	files map[string][]byte
	dirs  map[string][]fs.DirEntry
}

func (m *mockFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	if entries, ok := m.dirs[name]; ok {
		return entries, nil
	}

	return nil, fs.ErrNotExist
}

func (m *mockFileSystem) ReadFile(name string) ([]byte, error) {
	if content, ok := m.files[name]; ok {
		return content, nil
	}

	return nil, fs.ErrNotExist
}

func (m *mockFileSystem) WriteFile(_ string, _ []byte, _ fs.FileMode) error {
	return nil
}

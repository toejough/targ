package discover_test

import (
	"errors"
	"io/fs"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/discover"
)

func TestProperty_Discovery(t *testing.T) {
	t.Parallel()

	t.Run("DetectsAliasedImport", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

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

		infos, err := discover.Discover(
			filesystem,
			discover.Options{StartDir: ".", BuildTag: "targ"},
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(infos).To(HaveLen(1))
		g.Expect(infos[0].UsesExplicitRegistration).To(BeTrue())
	})

	t.Run("NonRegisterCallNotDetected", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

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

		infos, err := discover.Discover(
			filesystem,
			discover.Options{StartDir: ".", BuildTag: "targ"},
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(infos).To(HaveLen(1))
		g.Expect(infos[0].UsesExplicitRegistration).To(BeFalse())
	})
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

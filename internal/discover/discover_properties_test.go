package discover_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/discover"
)

func TestProperty_Discovery(t *testing.T) {
	t.Parallel()

	t.Run("FindsTaggedFilesInDirectory", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			pkgName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "pkgName")

			src := `//go:build targ

package ` + pkgName + `

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

			filesystem := &mockFileSystem{
				files: map[string][]byte{
					"test/targs.go": []byte(src),
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
			g.Expect(infos[0].Package).To(Equal(pkgName))
		})
	})

	t.Run("DetectsExplicitRegistration", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

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

		infos, err := discover.Discover(
			filesystem,
			discover.Options{StartDir: ".", BuildTag: "targ"},
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(infos).To(HaveLen(1))
		g.Expect(infos[0].UsesExplicitRegistration).To(BeTrue())
	})

	t.Run("DetectsAliasedImport", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			alias := rapid.StringMatching(`[a-z]{2,8}`).Draw(t, "alias")

			srcWithAlias := `//go:build targ

package build

import ` + alias + ` "github.com/toejough/targ"

func init() {
	` + alias + `.Register(Build)
}

var Build = ` + alias + `.Targ(func() {})
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

	t.Run("RejectsMainFunction", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		srcWithMain := `//go:build targ

package main

import "github.com/toejough/targ"

func main() {}

var Build = targ.Targ(func() {})
`

		filesystem := &mockFileSystem{
			files: map[string][]byte{
				"test/targs.go": []byte(srcWithMain),
			},
			dirs: map[string][]fs.DirEntry{
				".":    {mockDirEntry{name: "test", isDir: true}},
				"test": {mockDirEntry{name: "targs.go", isDir: false}},
			},
		}

		_, err := discover.Discover(
			filesystem,
			discover.Options{StartDir: ".", BuildTag: "targ"},
		)
		g.Expect(err).To(MatchError(ContainSubstring("main()")))
	})

	t.Run("SkipsTestFiles", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			base := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "base")
			testFile := base + "_test.go"

			src := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

			filesystem := &mockFileSystem{
				files: map[string][]byte{
					filepath.Join("test", testFile): []byte(src),
				},
				dirs: map[string][]fs.DirEntry{
					".":    {mockDirEntry{name: "test", isDir: true}},
					"test": {mockDirEntry{name: testFile, isDir: false}},
				},
			}

			infos, err := discover.Discover(
				filesystem,
				discover.Options{StartDir: ".", BuildTag: "targ"},
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(infos).To(BeEmpty(), "test files should be skipped")
		})
	})

	t.Run("SkipsGeneratedFiles", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			suffix := rapid.StringMatching(`[a-z_]+`).Draw(t, "suffix")
			genFile := "generated_targ_" + suffix + ".go"

			src := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

			filesystem := &mockFileSystem{
				files: map[string][]byte{
					filepath.Join("test", genFile): []byte(src),
				},
				dirs: map[string][]fs.DirEntry{
					".":    {mockDirEntry{name: "test", isDir: true}},
					"test": {mockDirEntry{name: genFile, isDir: false}},
				},
			}

			infos, err := discover.Discover(
				filesystem,
				discover.Options{StartDir: ".", BuildTag: "targ"},
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(infos).To(BeEmpty(), "generated files should be skipped")
		})
	})

	t.Run("SkipsSpecialDirectories", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		src := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

		for _, specialDir := range []string{"vendor", "testdata", "internal", ".git"} {
			filesystem := &mockFileSystem{
				files: map[string][]byte{
					filepath.Join(specialDir, "targs.go"): []byte(src),
				},
				dirs: map[string][]fs.DirEntry{
					".":        {mockDirEntry{name: specialDir, isDir: true}},
					specialDir: {mockDirEntry{name: "targs.go", isDir: false}},
				},
			}

			infos, err := discover.Discover(
				filesystem,
				discover.Options{StartDir: ".", BuildTag: "targ"},
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(infos).To(BeEmpty(), "special dir %q should be skipped", specialDir)
		}
	})

	t.Run("FindsFilesRecursively", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		src := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

		filesystem := &mockFileSystem{
			files: map[string][]byte{
				"a/b/c/targs.go": []byte(src),
			},
			dirs: map[string][]fs.DirEntry{
				".":     {mockDirEntry{name: "a", isDir: true}},
				"a":     {mockDirEntry{name: "b", isDir: true}},
				"a/b":   {mockDirEntry{name: "c", isDir: true}},
				"a/b/c": {mockDirEntry{name: "targs.go", isDir: false}},
			},
		}

		infos, err := discover.Discover(
			filesystem,
			discover.Options{StartDir: ".", BuildTag: "targ"},
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(infos).To(HaveLen(1))
		g.Expect(infos[0].Dir).To(Equal("a/b/c"))
	})

	t.Run("ExtractsPackageDoc", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			// Generate doc text with letters only (no spaces that could cause trim issues)
			docText := rapid.StringMatching(`[A-Za-z]{5,30}`).Draw(t, "docText")

			src := `//go:build targ

// ` + docText + `
package build

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

			filesystem := &mockFileSystem{
				files: map[string][]byte{
					"test/targs.go": []byte(src),
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
			g.Expect(infos[0].Doc).To(Equal(docText))
		})
	})

	t.Run("SortsFilesAlphabetically", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		src := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

		filesystem := &mockFileSystem{
			files: map[string][]byte{
				"test/zebra.go":  []byte(src),
				"test/alpha.go":  []byte(src),
				"test/middle.go": []byte(src),
			},
			dirs: map[string][]fs.DirEntry{
				".": {mockDirEntry{name: "test", isDir: true}},
				"test": {
					mockDirEntry{name: "zebra.go", isDir: false},
					mockDirEntry{name: "alpha.go", isDir: false},
					mockDirEntry{name: "middle.go", isDir: false},
				},
			},
		}

		infos, err := discover.Discover(
			filesystem,
			discover.Options{StartDir: ".", BuildTag: "targ"},
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(infos).To(HaveLen(1))
		g.Expect(infos[0].Files).To(HaveLen(3))
		g.Expect(infos[0].Files[0].Base).To(Equal("alpha"))
		g.Expect(infos[0].Files[1].Base).To(Equal("middle"))
		g.Expect(infos[0].Files[2].Base).To(Equal("zebra"))
	})

	t.Run("RejectsMultiplePackageNames", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		src1 := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

		src2 := `//go:build targ

package other

import "github.com/toejough/targ"

var Other = targ.Targ(func() {})
`

		filesystem := &mockFileSystem{
			files: map[string][]byte{
				"test/build.go": []byte(src1),
				"test/other.go": []byte(src2),
			},
			dirs: map[string][]fs.DirEntry{
				".": {mockDirEntry{name: "test", isDir: true}},
				"test": {
					mockDirEntry{name: "build.go", isDir: false},
					mockDirEntry{name: "other.go", isDir: false},
				},
			},
		}

		_, err := discover.Discover(
			filesystem,
			discover.Options{StartDir: ".", BuildTag: "targ"},
		)
		g.Expect(err).To(MatchError(ContainSubstring("multiple package names")))
	})

	t.Run("TaggedFilesReturnsAllFiles", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		src := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

		filesystem := &mockFileSystem{
			files: map[string][]byte{
				"a/targs.go": []byte(src),
				"b/targs.go": []byte(src),
			},
			dirs: map[string][]fs.DirEntry{
				".": {
					mockDirEntry{name: "a", isDir: true},
					mockDirEntry{name: "b", isDir: true},
				},
				"a": {mockDirEntry{name: "targs.go", isDir: false}},
				"b": {mockDirEntry{name: "targs.go", isDir: false}},
			},
		}

		files, err := discover.TaggedFiles(
			filesystem,
			discover.Options{StartDir: ".", BuildTag: "targ"},
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(files).To(HaveLen(2))
	})

	t.Run("UsesDefaultBuildTagWhenEmpty", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		src := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

		filesystem := &mockFileSystem{
			files: map[string][]byte{
				"test/targs.go": []byte(src),
			},
			dirs: map[string][]fs.DirEntry{
				".":    {mockDirEntry{name: "test", isDir: true}},
				"test": {mockDirEntry{name: "targs.go", isDir: false}},
			},
		}

		infos, err := discover.Discover(
			filesystem,
			discover.Options{StartDir: ".", BuildTag: ""}, // Empty should default to "targ"
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(infos).To(HaveLen(1))
	})

	t.Run("UsesDefaultStartDirWhenEmpty", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		src := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

		filesystem := &mockFileSystem{
			files: map[string][]byte{
				"test/targs.go": []byte(src),
			},
			dirs: map[string][]fs.DirEntry{
				".":    {mockDirEntry{name: "test", isDir: true}},
				"test": {mockDirEntry{name: "targs.go", isDir: false}},
			},
		}

		infos, err := discover.Discover(
			filesystem,
			discover.Options{StartDir: "", BuildTag: "targ"}, // Empty should default to "."
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(infos).To(HaveLen(1))
	})

	t.Run("IgnoresFilesWithoutTag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		srcWithTag := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ(func() {})
`

		srcWithoutTag := `package build

import "github.com/toejough/targ"

var Other = targ.Targ(func() {})
`

		filesystem := &mockFileSystem{
			files: map[string][]byte{
				"test/tagged.go":   []byte(srcWithTag),
				"test/untagged.go": []byte(srcWithoutTag),
			},
			dirs: map[string][]fs.DirEntry{
				".": {mockDirEntry{name: "test", isDir: true}},
				"test": {
					mockDirEntry{name: "tagged.go", isDir: false},
					mockDirEntry{name: "untagged.go", isDir: false},
				},
			},
		}

		infos, err := discover.Discover(
			filesystem,
			discover.Options{StartDir: ".", BuildTag: "targ"},
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(infos).To(HaveLen(1))
		g.Expect(infos[0].Files).To(HaveLen(1))
		g.Expect(infos[0].Files[0].Base).To(Equal("tagged"))
	})
}

// unexported test helpers.

var errInfoNotImplemented = errors.New("Info() not implemented in mock")

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

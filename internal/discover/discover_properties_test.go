//nolint:maintidx // Test functions with many subtests have low maintainability index by design
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
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			varName := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "varName")

			srcWithRegister := `//go:build targ

package build

import "github.com/toejough/targ"

func init() {
	targ.Register(` + varName + `)
}

var ` + varName + ` = targ.Targ(func() {})
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
	})

	t.Run("DetectsAliasedImport", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			// Use prefix 'x' to avoid Go keywords like "go", "if", "for"
			alias := "x" + rapid.StringMatching(`[a-z]{1,7}`).Draw(t, "alias")

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
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			msg := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "msg")
			varName := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "varName")

			srcWithOtherInit := `//go:build targ

package build

import "fmt"
import "github.com/toejough/targ"

func init() {
	fmt.Println("` + msg + `")
}

var ` + varName + ` = targ.Targ(func() {})
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
	})

	t.Run("RejectsMainFunction", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			varName := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "varName")

			srcWithMain := `//go:build targ

package main

import "github.com/toejough/targ"

func main() {}

var ` + varName + ` = targ.Targ(func() {})
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
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			varName := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "varName")
			specialDir := rapid.SampledFrom([]string{"vendor", "testdata", "internal", ".git"}).
				Draw(t, "specialDir")

			src := `//go:build targ

package build

import "github.com/toejough/targ"

var ` + varName + ` = targ.Targ(func() {})
`

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
		})
	})

	t.Run("FindsFilesRecursively", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			dir1 := rapid.StringMatching(`[a-z]{2,5}`).Draw(t, "dir1")
			dir2 := rapid.StringMatching(`[a-z]{2,5}`).Draw(t, "dir2")
			dir3 := rapid.StringMatching(`[a-z]{2,5}`).Draw(t, "dir3")
			varName := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "varName")

			src := `//go:build targ

package build

import "github.com/toejough/targ"

var ` + varName + ` = targ.Targ(func() {})
`
			deepPath := dir1 + "/" + dir2 + "/" + dir3

			filesystem := &mockFileSystem{
				files: map[string][]byte{
					deepPath + "/targs.go": []byte(src),
				},
				dirs: map[string][]fs.DirEntry{
					".":               {mockDirEntry{name: dir1, isDir: true}},
					dir1:              {mockDirEntry{name: dir2, isDir: true}},
					dir1 + "/" + dir2: {mockDirEntry{name: dir3, isDir: true}},
					deepPath:          {mockDirEntry{name: "targs.go", isDir: false}},
				},
			}

			infos, err := discover.Discover(
				filesystem,
				discover.Options{StartDir: ".", BuildTag: "targ"},
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(infos).To(HaveLen(1))
			g.Expect(infos[0].Dir).To(Equal(deepPath))
		})
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
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			varName := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "varName")
			file1 := "a" + rapid.StringMatching(`[a-z]{2,5}`).Draw(t, "file1")
			file2 := "m" + rapid.StringMatching(`[a-z]{2,5}`).Draw(t, "file2")
			file3 := "z" + rapid.StringMatching(`[a-z]{2,5}`).Draw(t, "file3")

			src := `//go:build targ

package build

import "github.com/toejough/targ"

var ` + varName + ` = targ.Targ(func() {})
`

			filesystem := &mockFileSystem{
				files: map[string][]byte{
					"test/" + file3 + ".go": []byte(src),
					"test/" + file1 + ".go": []byte(src),
					"test/" + file2 + ".go": []byte(src),
				},
				dirs: map[string][]fs.DirEntry{
					".": {mockDirEntry{name: "test", isDir: true}},
					"test": {
						mockDirEntry{name: file3 + ".go", isDir: false},
						mockDirEntry{name: file1 + ".go", isDir: false},
						mockDirEntry{name: file2 + ".go", isDir: false},
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
			g.Expect(infos[0].Files[0].Base).To(Equal(file1))
			g.Expect(infos[0].Files[1].Base).To(Equal(file2))
			g.Expect(infos[0].Files[2].Base).To(Equal(file3))
		})
	})

	t.Run("RejectsMultiplePackageNames", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			pkg1 := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "pkg1")

			pkg2 := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "pkg2")
			if pkg1 == pkg2 {
				return
			}

			var1 := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "var1")
			var2 := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "var2")

			src1 := `//go:build targ

package ` + pkg1 + `

import "github.com/toejough/targ"

var ` + var1 + ` = targ.Targ(func() {})
`

			src2 := `//go:build targ

package ` + pkg2 + `

import "github.com/toejough/targ"

var ` + var2 + ` = targ.Targ(func() {})
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
	})

	t.Run("TaggedFilesReturnsAllFiles", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			dir1 := rapid.StringMatching(`[a-z]{2,5}`).Draw(t, "dir1")

			dir2 := rapid.StringMatching(`[a-z]{2,5}`).Draw(t, "dir2")
			if dir1 == dir2 {
				return
			}

			varName := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "varName")

			src := `//go:build targ

package build

import "github.com/toejough/targ"

var ` + varName + ` = targ.Targ(func() {})
`

			filesystem := &mockFileSystem{
				files: map[string][]byte{
					dir1 + "/targs.go": []byte(src),
					dir2 + "/targs.go": []byte(src),
				},
				dirs: map[string][]fs.DirEntry{
					".": {
						mockDirEntry{name: dir1, isDir: true},
						mockDirEntry{name: dir2, isDir: true},
					},
					dir1: {mockDirEntry{name: "targs.go", isDir: false}},
					dir2: {mockDirEntry{name: "targs.go", isDir: false}},
				},
			}

			files, err := discover.TaggedFiles(
				filesystem,
				discover.Options{StartDir: ".", BuildTag: "targ"},
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(files).To(HaveLen(2))
		})
	})

	t.Run("UsesDefaultBuildTagWhenEmpty", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			varName := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "varName")
			dirName := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "dirName")

			src := `//go:build targ

package build

import "github.com/toejough/targ"

var ` + varName + ` = targ.Targ(func() {})
`

			filesystem := &mockFileSystem{
				files: map[string][]byte{
					dirName + "/targs.go": []byte(src),
				},
				dirs: map[string][]fs.DirEntry{
					".":     {mockDirEntry{name: dirName, isDir: true}},
					dirName: {mockDirEntry{name: "targs.go", isDir: false}},
				},
			}

			infos, err := discover.Discover(
				filesystem,
				discover.Options{StartDir: ".", BuildTag: ""}, // Empty should default to "targ"
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(infos).To(HaveLen(1))
		})
	})

	t.Run("UsesDefaultStartDirWhenEmpty", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			varName := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "varName")
			dirName := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "dirName")

			src := `//go:build targ

package build

import "github.com/toejough/targ"

var ` + varName + ` = targ.Targ(func() {})
`

			filesystem := &mockFileSystem{
				files: map[string][]byte{
					dirName + "/targs.go": []byte(src),
				},
				dirs: map[string][]fs.DirEntry{
					".":     {mockDirEntry{name: dirName, isDir: true}},
					dirName: {mockDirEntry{name: "targs.go", isDir: false}},
				},
			}

			infos, err := discover.Discover(
				filesystem,
				discover.Options{StartDir: "", BuildTag: "targ"}, // Empty should default to "."
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(infos).To(HaveLen(1))
		})
	})

	t.Run("IgnoresFilesWithoutTag", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			var1 := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "var1")
			var2 := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(t, "var2")
			taggedFile := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "taggedFile")

			untaggedFile := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "untaggedFile")
			if taggedFile == untaggedFile {
				return
			}

			srcWithTag := `//go:build targ

package build

import "github.com/toejough/targ"

var ` + var1 + ` = targ.Targ(func() {})
`

			srcWithoutTag := `package build

import "github.com/toejough/targ"

var ` + var2 + ` = targ.Targ(func() {})
`

			filesystem := &mockFileSystem{
				files: map[string][]byte{
					"test/" + taggedFile + ".go":   []byte(srcWithTag),
					"test/" + untaggedFile + ".go": []byte(srcWithoutTag),
				},
				dirs: map[string][]fs.DirEntry{
					".": {mockDirEntry{name: "test", isDir: true}},
					"test": {
						mockDirEntry{name: taggedFile + ".go", isDir: false},
						mockDirEntry{name: untaggedFile + ".go", isDir: false},
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
			g.Expect(infos[0].Files[0].Base).To(Equal(taggedFile))
		})
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

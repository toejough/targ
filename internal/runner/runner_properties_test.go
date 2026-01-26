//nolint:maintidx,cyclop // Test functions with many subtests have high complexity by design
package runner_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/runner"
)

// MemoryFileOps implements runner.FileOps using in-memory storage.
type MemoryFileOps struct {
	Files map[string][]byte
	Dirs  map[string][]fs.DirEntry
}

// NewMemoryFileOps creates a new in-memory file system.
func NewMemoryFileOps() *MemoryFileOps {
	return &MemoryFileOps{
		Files: make(map[string][]byte),
		Dirs:  make(map[string][]fs.DirEntry),
	}
}

func (m *MemoryFileOps) MkdirAll(path string, _ fs.FileMode) error {
	// Create all parent directories
	parts := strings.Split(path, string(filepath.Separator))
	current := ""

	for _, part := range parts {
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}

		if _, ok := m.Dirs[current]; !ok {
			m.Dirs[current] = []fs.DirEntry{}
		}
	}

	return nil
}

func (m *MemoryFileOps) ReadDir(name string) ([]fs.DirEntry, error) {
	if entries, ok := m.Dirs[name]; ok {
		return entries, nil
	}
	// Return empty for root
	if name == "." || name == "" {
		return []fs.DirEntry{}, nil
	}

	return nil, fs.ErrNotExist
}

func (m *MemoryFileOps) ReadFile(name string) ([]byte, error) {
	if content, ok := m.Files[name]; ok {
		return content, nil
	}

	return nil, fs.ErrNotExist
}

func (m *MemoryFileOps) Stat(name string) (fs.FileInfo, error) {
	if content, ok := m.Files[name]; ok {
		return &mockFileInfo{name: filepath.Base(name), size: int64(len(content))}, nil
	}

	return nil, fs.ErrNotExist
}

func (m *MemoryFileOps) WriteFile(name string, data []byte, _ fs.FileMode) error {
	m.Files[name] = data
	// Update directory listing
	dir := filepath.Dir(name)
	base := filepath.Base(name)
	m.addEntry(dir, base, false)

	return nil
}

func (m *MemoryFileOps) addEntry(dir, name string, isDir bool) {
	if m.Dirs[dir] == nil {
		m.Dirs[dir] = []fs.DirEntry{}
	}

	// Check if already exists
	for _, e := range m.Dirs[dir] {
		if e.Name() == name {
			return
		}
	}

	m.Dirs[dir] = append(m.Dirs[dir], memDirEntry{name: name, isDir: isDir})
}

func TestProperty_CodeGeneration(t *testing.T) {
	t.Parallel()

	// Property: Valid target names match the kebab-case pattern
	t.Run("ValidTargetNamesMatchPattern", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			// Generate valid names: lowercase letters, may contain hyphens (not at start/end)
			name := rapid.StringMatching(`[a-z][a-z0-9]*(-[a-z0-9]+)*`).Draw(t, "name")
			g.Expect(runner.IsValidTargetName(name)).
				To(BeTrue(), "name %q should be valid", name)
		})
	})

	// Property: Invalid target names are rejected
	t.Run("InvalidTargetNamesRejected", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			// Generate names that violate the pattern
			invalidType := rapid.IntRange(0, 5).Draw(t, "invalidType")

			var name string

			switch invalidType {
			case 0: // Empty
				name = ""
			case 1: // Starts with number
				name = rapid.StringMatching(`[0-9][a-z0-9-]*`).Draw(t, "name")
			case 2: // Starts with hyphen
				name = "-" + rapid.StringMatching(`[a-z0-9-]*`).Draw(t, "name")
			case 3: // Ends with hyphen
				name = rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name") + "-"
			case 4: // Contains uppercase
				name = rapid.StringMatching(`[a-z]*[A-Z][a-z]*`).Draw(t, "name")
			case 5: // Contains special chars
				name = rapid.StringMatching(`[a-z]+[_@.][a-z]+`).Draw(t, "name")
			}

			g.Expect(runner.IsValidTargetName(name)).
				To(BeFalse(), "name %q should be invalid", name)
		})
	})

	// Property: Adding a target creates valid Go code with correct name
	t.Run("AddingTargetCreatesValidCode", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			fileOps := NewMemoryFileOps()
			targetName := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(t, "targetName")
			shellCmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "shellCmd")

			// Set up initial file content
			initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
			fileOps.Files["targs.go"] = []byte(initial)

			err := runner.AddTargetToFileWithFileOps(fileOps, "targs.go", runner.CreateOptions{
				Name:     targetName,
				ShellCmd: shellCmd,
			})
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files["targs.go"])

			// Property: Output contains the variable declaration
			expectedVar := "var " + runner.KebabToPascal(targetName) + " = targ.Targ"
			g.Expect(content).To(ContainSubstring(expectedVar))

			// Property: Output contains the Name() call
			g.Expect(content).To(ContainSubstring(`.Name("` + targetName + `")`))

			// Property: Output preserves the build tag
			g.Expect(content).To(HavePrefix("//go:build targ"))
		})
	})

	// Property: Duplicate targets are rejected
	t.Run("DuplicateTargetsRejected", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			fileOps := NewMemoryFileOps()
			targetName := rapid.StringMatching(`[a-z][a-z0-9]{2,8}`).Draw(t, "targetName")

			initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
			fileOps.Files["targs.go"] = []byte(initial)

			// First add succeeds
			err := runner.AddTargetToFileWithFileOps(fileOps, "targs.go", runner.CreateOptions{
				Name:     targetName,
				ShellCmd: "echo hello",
			})
			g.Expect(err).NotTo(HaveOccurred())

			// Second add fails
			err = runner.AddTargetToFileWithFileOps(fileOps, "targs.go", runner.CreateOptions{
				Name:     targetName,
				ShellCmd: "echo world",
			})
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("already exists"))
		})
	})

	// Property: Cache patterns are included in generated code
	t.Run("CachePatternsIncluded", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			fileOps := NewMemoryFileOps()
			targetName := rapid.StringMatching(`[a-z][a-z0-9]{2,8}`).Draw(t, "targetName")
			numPatterns := rapid.IntRange(1, 3).Draw(t, "numPatterns")

			patterns := make([]string, numPatterns)
			for i := range numPatterns {
				patterns[i] = rapid.StringMatching(`\*\*/\*\.[a-z]{2,4}`).Draw(t, "pattern")
			}

			initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
			fileOps.Files["targs.go"] = []byte(initial)

			err := runner.AddTargetToFileWithFileOps(fileOps, "targs.go", runner.CreateOptions{
				Name:     targetName,
				ShellCmd: "go build",
				Cache:    patterns,
			})
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files["targs.go"])
			g.Expect(content).To(ContainSubstring(".Cache("))

			for _, p := range patterns {
				g.Expect(content).To(ContainSubstring(p))
			}
		})
	})

	// Property: FindOrCreateTargFile finds existing file with build tag
	t.Run("FindsExistingTargFile", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		fileOps := NewMemoryFileOps()
		fileOps.Files["testdir/build.go"] = []byte("//go:build targ\n\npackage build\n")
		fileOps.Dirs["testdir"] = []fs.DirEntry{
			memDirEntry{name: "build.go", isDir: false},
		}

		path, err := runner.FindOrCreateTargFileWithFileOps(fileOps, "testdir")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(path).To(Equal(filepath.Join("testdir", "build.go")))
	})

	// Property: FindOrCreateTargFile creates new file when none exists
	t.Run("CreatesNewTargFile", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		fileOps := NewMemoryFileOps()
		fileOps.Dirs["testdir"] = []fs.DirEntry{} // Empty directory

		path, err := runner.FindOrCreateTargFileWithFileOps(fileOps, "testdir")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(path).To(Equal(filepath.Join("testdir", "targs.go")))

		// New file should have build tag
		content, ok := fileOps.Files[path]
		g.Expect(ok).To(BeTrue())
		g.Expect(string(content)).To(HavePrefix("//go:build targ"))
	})

	// Property: HasTargBuildTag correctly identifies files with the tag
	t.Run("HasTargBuildTagDetectsTag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		fileOps := NewMemoryFileOps()

		// File with tag
		fileOps.Files["with_tag.go"] = []byte("//go:build targ\n\npackage foo\n")
		g.Expect(runner.HasTargBuildTagWithFileOps(fileOps, "with_tag.go")).To(BeTrue())

		// File without tag
		fileOps.Files["without_tag.go"] = []byte("package foo\n")
		g.Expect(runner.HasTargBuildTagWithFileOps(fileOps, "without_tag.go")).To(BeFalse())

		// File with different tag
		fileOps.Files["other_tag.go"] = []byte("//go:build integration\n\npackage foo\n")
		g.Expect(runner.HasTargBuildTagWithFileOps(fileOps, "other_tag.go")).To(BeFalse())
	})

	// Property: AddImportToTargFile adds blank import correctly
	t.Run("AddImportAddsBlankImport", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			fileOps := NewMemoryFileOps()
			pkgPath := rapid.StringMatching(`github\.com/[a-z]+/[a-z]+`).Draw(t, "pkgPath")

			initial := `//go:build targ

package build

import "github.com/toejough/targ"

var Lint = targ.Targ("golangci-lint run")
`
			fileOps.Files["targs.go"] = []byte(initial)

			err := runner.AddImportToTargFileWithFileOps(fileOps, "targs.go", pkgPath)
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files["targs.go"])
			// Property: Import is added
			g.Expect(content).To(ContainSubstring(`_ "` + pkgPath + `"`))
			// Property: Original import preserved
			g.Expect(content).To(ContainSubstring(`"github.com/toejough/targ"`))
			// Property: Original code preserved
			g.Expect(content).To(ContainSubstring("var Lint"))
		})
	})

	// Property: CheckImportExists correctly detects existing imports
	t.Run("CheckImportExistsDetectsImport", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		fileOps := NewMemoryFileOps()
		fileOps.Files["targs.go"] = []byte(`//go:build targ

package build

import (
	"github.com/toejough/targ"
	_ "github.com/foo/bar"
)
`)

		exists, err := runner.CheckImportExistsWithFileOps(
			fileOps,
			"targs.go",
			"github.com/foo/bar",
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(exists).To(BeTrue())

		exists, err = runner.CheckImportExistsWithFileOps(
			fileOps,
			"targs.go",
			"github.com/not/there",
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(exists).To(BeFalse())
	})

	// Property: KebabToPascal converts correctly
	t.Run("KebabToPascalConverts", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			// Generate kebab-case input
			parts := rapid.SliceOfN(
				rapid.StringMatching(`[a-z]+`),
				1, 4,
			).Draw(t, "parts")
			input := strings.Join(parts, "-")

			result := runner.KebabToPascal(input)

			// Property: Result contains no hyphens
			g.Expect(result).NotTo(ContainSubstring("-"))

			// Property: Each original part has first letter capitalized
			for _, part := range parts {
				if len(part) > 0 {
					expected := strings.ToUpper(part[:1]) + part[1:]
					g.Expect(result).To(ContainSubstring(expected))
				}
			}
		})
	})

	// Property: ExtractTargFlags extracts flags correctly
	t.Run("ExtractTargFlagsWorks", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// --no-binary-cache is extracted
		flags, remaining := runner.ExtractTargFlags([]string{"--no-binary-cache", "build"})
		g.Expect(flags.NoBinaryCache).To(BeTrue())
		g.Expect(remaining).To(Equal([]string{"build"}))

		// Deprecated --no-cache also works
		flags, _ = runner.ExtractTargFlags([]string{"--no-cache", "build"})
		g.Expect(flags.NoBinaryCache).To(BeTrue())

		// -s after target is not extracted
		flags, remaining = runner.ExtractTargFlags([]string{"build", "-s", "value"})
		g.Expect(flags.SourceDir).To(BeEmpty())
		g.Expect(remaining).To(Equal([]string{"build", "-s", "value"}))
	})

	// Property: ParseCreateArgs parses valid arguments
	t.Run("ParseCreateArgsWorks", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		opts, err := runner.ParseCreateArgs([]string{"lint", "golangci-lint run"})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(opts.Name).To(Equal("lint"))
		g.Expect(opts.ShellCmd).To(Equal("golangci-lint run"))

		// With path and options
		opts, err = runner.ParseCreateArgs([]string{
			"dev", "build",
			"--deps", "lint", "test",
			"--cache", "**/*.go",
			"go build ./...",
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(opts.Name).To(Equal("build"))
		g.Expect(opts.Path).To(Equal([]string{"dev"}))
		g.Expect(opts.Deps).To(Equal([]string{"lint", "test"}))
		g.Expect(opts.Cache).To(Equal([]string{"**/*.go"}))
		g.Expect(opts.ShellCmd).To(Equal("go build ./..."))
	})

	// Property: ParseSyncArgs validates package paths
	t.Run("ParseSyncArgsValidatesPath", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		opts, err := runner.ParseSyncArgs([]string{"github.com/foo/bar"})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(opts.PackagePath).To(Equal("github.com/foo/bar"))

		_, err = runner.ParseSyncArgs([]string{"invalid-path"})
		g.Expect(err).To(HaveOccurred())
	})

	// Property: ParseHelpRequest distinguishes top-level vs target help
	t.Run("ParseHelpRequestDistinguishes", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		help, target := runner.ParseHelpRequest([]string{"--help"})
		g.Expect(help).To(BeTrue())
		g.Expect(target).To(BeFalse())

		help, target = runner.ParseHelpRequest([]string{"issues", "--help"})
		g.Expect(help).To(BeTrue())
		g.Expect(target).To(BeTrue())
	})

	// Property: NamespacePaths computes correct paths
	t.Run("NamespacePathsComputes", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		files := []string{
			"/root/tools/issues/issues.go",
			"/root/tools/other/foo.go",
			"/root/tools/other/bar.go",
		}

		paths, err := runner.NamespacePaths(files, "/root")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(paths["/root/tools/issues/issues.go"]).To(Equal([]string{"issues"}))
		g.Expect(paths["/root/tools/other/foo.go"]).To(Equal([]string{"other", "foo"}))
		g.Expect(paths["/root/tools/other/bar.go"]).To(Equal([]string{"other", "bar"}))
	})
}

// unexported variables.
var (
	errInfoNotImplemented = errors.New("Info() not implemented in mock")
)

type memDirEntry struct {
	name  string
	isDir bool
}

func (e memDirEntry) Info() (fs.FileInfo, error) { return nil, errInfoNotImplemented }

func (e memDirEntry) IsDir() bool { return e.isDir }

func (e memDirEntry) Name() string { return e.name }

func (e memDirEntry) Type() fs.FileMode { return 0 }

// mockFileInfo implements fs.FileInfo for testing.
type mockFileInfo struct {
	name string
	size int64
}

func (m *mockFileInfo) IsDir() bool { return false }

func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }

func (m *mockFileInfo) Mode() fs.FileMode { return 0o644 }

func (m *mockFileInfo) Name() string { return m.name }

func (m *mockFileInfo) Size() int64 { return m.size }

func (m *mockFileInfo) Sys() any { return nil }

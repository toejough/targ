// TEST-024: Runner properties - validates target scaffolding and sync operations
// traces: ARCH-008, ARCH-009

//nolint:maintidx,cyclop // Test functions with many subtests have high complexity by design
package runner_test

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
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

			// Property: Output contains inline targ.Targ() in Register() (ISSUE-021)
			g.Expect(content).To(ContainSubstring("targ.Register(targ.Targ"),
				"should contain inline targ.Targ() expression")

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
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			dirName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "dirName")
			fileName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "fileName") + ".go"

			fileOps := NewMemoryFileOps()
			filePath := filepath.Join(dirName, fileName)
			fileOps.Files[filePath] = []byte("//go:build targ\n\npackage build\n")
			fileOps.Dirs[dirName] = []fs.DirEntry{
				memDirEntry{name: fileName, isDir: false},
			}

			path, err := runner.FindOrCreateTargFileWithFileOps(fileOps, dirName)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(path).To(Equal(filePath))
		})
	})

	// Property: FindOrCreateTargFile finds targ file in a descendant directory
	t.Run("FindsDescendantTargFile", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			root := "/" + rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "root")
			parent := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "parent")
			child := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "child")

			startDir := filepath.Join(root, parent)
			descendantDir := filepath.Join(root, parent, child)

			fileOps := NewMemoryFileOps()
			g.Expect(fileOps.MkdirAll(descendantDir, 0o755)).To(Succeed())

			fileOps.Dirs[root] = []fs.DirEntry{memDirEntry{name: parent, isDir: true}}
			fileOps.Dirs[startDir] = []fs.DirEntry{memDirEntry{name: child, isDir: true}}

			descendantTarg := filepath.Join(descendantDir, "targets.go")
			content := `//go:build targ

package build
`

			g.Expect(fileOps.WriteFile(descendantTarg, []byte(content), 0o644)).To(Succeed())

			path, err := runner.FindOrCreateTargFileWithFileOps(fileOps, startDir)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(path).To(Equal(descendantTarg))
			g.Expect(fileOps.Files[filepath.Join(startDir, "targs.go")]).To(BeNil(),
				"should not create a new targ file when one exists in descendants")
		})
	})

	// Property: AddTargetToFile adds target to targ.Register in init block.
	t.Run("AddTargetToFileUpdatesRegister", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			dirName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "dirName")
			targetName := rapid.StringMatching(`[a-z][a-z0-9]{2,8}`).Draw(t, "targetName")

			fileOps := NewMemoryFileOps()
			path := filepath.Join(dirName, "targets.go")

			initial := `//go:build targ

package build

import "github.com/toejough/targ"

var Existing = targ.Targ(func() {}).Name("existing")

func init() {
	targ.Register(
		Existing,
	)
}
`
			fileOps.Files[path] = []byte(initial)
			fileOps.Dirs[dirName] = []fs.DirEntry{
				memDirEntry{name: "targets.go", isDir: false},
			}

			err := runner.AddTargetToFileWithFileOps(fileOps, path, runner.CreateOptions{
				Name:     targetName,
				ShellCmd: "echo hello",
			})
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files[path])
			// New pattern uses inline targ.Targ() expression (may be mixed with existing vars)
			g.Expect(content).To(ContainSubstring("targ.Targ(\"echo hello\")"),
				"should contain inline targ.Targ() expression")
			g.Expect(content).To(ContainSubstring(fmt.Sprintf(".Name(%q)", targetName)),
				"inline expression must contain .Name(%q)", targetName)
		})
	})

	// Property: FindOrCreateTargFile creates new file when none exists
	t.Run("CreatesNewTargFile", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			dirName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "dirName")

			fileOps := NewMemoryFileOps()
			fileOps.Dirs[dirName] = []fs.DirEntry{} // Empty directory

			path, err := runner.FindOrCreateTargFileWithFileOps(fileOps, dirName)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(path).To(Equal(filepath.Join(dirName, "targs.go")))

			// New file should have build tag
			content, ok := fileOps.Files[path]
			g.Expect(ok).To(BeTrue())
			g.Expect(string(content)).To(HavePrefix("//go:build targ"))
		})
	})

	// Property: FindOrCreateTargFile creates file with explicit registration pattern
	// ISSUE-021: New targ files must use init() with targ.Register(), not var _ = targ.Targ
	t.Run("CreatesNewTargFileWithExplicitRegistration", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			dirName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "dirName")

			fileOps := NewMemoryFileOps()
			fileOps.Dirs[dirName] = []fs.DirEntry{} // Empty directory

			path, err := runner.FindOrCreateTargFileWithFileOps(fileOps, dirName)
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files[path])

			// Property: New file must have init() function
			g.Expect(content).To(ContainSubstring("func init()"),
				"new targ file must have init() function for explicit registration")

			// Property: New file must have targ.Register() call
			g.Expect(content).To(ContainSubstring("targ.Register("),
				"new targ file must use targ.Register() for explicit registration")

			// Property: New file must NOT use the old var _ = targ.Targ pattern
			g.Expect(content).NotTo(ContainSubstring("var _ = targ.Targ"),
				"new targ file must not use deprecated var _ = targ.Targ pattern")
		})
	})

	// Property: AddTargetToFile creates init with targ.Register if missing
	// ISSUE-021: Adding target to file without init() should create one
	t.Run("AddTargetCreatesInitIfMissing", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			fileOps := NewMemoryFileOps()
			targetName := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(t, "targetName")
			shellCmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "shellCmd")

			// File without init() or targ.Register()
			initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
			fileOps.Files["targs.go"] = []byte(initial)

			err := runner.AddTargetToFileWithFileOps(fileOps, "targs.go", runner.CreateOptions{
				Name:     targetName,
				ShellCmd: shellCmd,
			})
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files["targs.go"])

			// Property: File must have init() after adding target
			g.Expect(content).To(ContainSubstring("func init()"),
				"file must have init() function after adding target")

			// Property: File must have targ.Register() with inline expression (ISSUE-021)
			// New pattern: targ.Register(targ.Targ("cmd").Name("name"))
			g.Expect(content).To(ContainSubstring("targ.Register(targ.Targ"),
				"targ.Register() must contain inline targ.Targ() expression")
			g.Expect(content).To(ContainSubstring(fmt.Sprintf(".Name(%q)", targetName)),
				"inline expression must contain .Name(%q)", targetName)
		})
	})

	// Property: HasTargBuildTag correctly identifies files with the tag
	t.Run("HasTargBuildTagDetectsTag", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			pkgName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "pkgName")

			fileOps := NewMemoryFileOps()

			// File with tag
			fileOps.Files["with_tag.go"] = []byte("//go:build targ\n\npackage " + pkgName + "\n")
			g.Expect(runner.HasTargBuildTagWithFileOps(fileOps, "with_tag.go")).To(BeTrue())

			// File without tag
			fileOps.Files["without_tag.go"] = []byte("package " + pkgName + "\n")
			g.Expect(runner.HasTargBuildTagWithFileOps(fileOps, "without_tag.go")).To(BeFalse())

			// File with different tag
			fileOps.Files["other_tag.go"] = []byte(
				"//go:build integration\n\npackage " + pkgName + "\n",
			)
			g.Expect(runner.HasTargBuildTagWithFileOps(fileOps, "other_tag.go")).To(BeFalse())
		})
	})

	// Property: AddImportToTargFile adds blank import and DeregisterFrom call
	t.Run("AddImportAddsBlankImportAndDeregisterFrom", func(t *testing.T) {
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
			// Property: Blank import is added
			g.Expect(content).To(ContainSubstring(`_ "` + pkgPath + `"`))
			// Property: Original import preserved
			g.Expect(content).To(ContainSubstring(`"github.com/toejough/targ"`))
			// Property: Original code preserved
			g.Expect(content).To(ContainSubstring("var Lint"))
			// Property: DeregisterFrom call is added
			g.Expect(content).To(ContainSubstring(`targ.DeregisterFrom("` + pkgPath + `")`))
			// Property: init() function exists
			g.Expect(content).To(ContainSubstring("func init()"))
		})
	})

	// Property: AddImportToTargFile appends DeregisterFrom to existing init()
	t.Run("AddImportAppendsToExistingInit", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			fileOps := NewMemoryFileOps()
			pkg1 := rapid.StringMatching(`github\.com/[a-z]+/[a-z]+`).Draw(t, "pkg1")
			pkg2 := rapid.StringMatching(`github\.com/[a-z]+/[a-z]+`).
				Filter(func(s string) bool { return s != pkg1 }).
				Draw(t, "pkg2")

			// Start with a file that already has init() with a DeregisterFrom call
			initial := `//go:build targ

package build

import (
	"github.com/toejough/targ"

	_ "` + pkg1 + `"
)

func init() {
	_ = targ.DeregisterFrom("` + pkg1 + `")
}

var Lint = targ.Targ("golangci-lint run")
`
			fileOps.Files["targs.go"] = []byte(initial)

			err := runner.AddImportToTargFileWithFileOps(fileOps, "targs.go", pkg2)
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files["targs.go"])
			// Property: Both DeregisterFrom calls present
			g.Expect(content).To(ContainSubstring(`targ.DeregisterFrom("` + pkg1 + `")`))
			g.Expect(content).To(ContainSubstring(`targ.DeregisterFrom("` + pkg2 + `")`))
			// Property: Both blank imports present
			g.Expect(content).To(ContainSubstring(`_ "` + pkg1 + `"`))
			g.Expect(content).To(ContainSubstring(`_ "` + pkg2 + `"`))
			// Property: Still valid Go (has single init function)
			g.Expect(strings.Count(content, "func init()")).To(Equal(1))
		})
	})

	// Property: CheckImportExists correctly detects existing imports
	t.Run("CheckImportExistsDetectsImport", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			existingPkg := rapid.StringMatching(`github\.com/[a-z]+/[a-z]+`).Draw(t, "existingPkg")

			missingPkg := rapid.StringMatching(`github\.com/[a-z]+/[a-z]+`).Draw(t, "missingPkg")
			if existingPkg == missingPkg {
				return
			}

			fileOps := NewMemoryFileOps()
			fileOps.Files["targs.go"] = []byte(`//go:build targ

package build

import (
	"github.com/toejough/targ"
	_ "` + existingPkg + `"
)
`)

			exists, err := runner.CheckImportExistsWithFileOps(
				fileOps,
				"targs.go",
				existingPkg,
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exists).To(BeTrue())

			exists, err = runner.CheckImportExistsWithFileOps(
				fileOps,
				"targs.go",
				missingPkg,
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exists).To(BeFalse())
		})
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
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			targetName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "targetName")

			// --no-binary-cache is extracted
			flags, remaining := runner.ExtractTargFlags([]string{"--no-binary-cache", targetName})
			g.Expect(flags.NoBinaryCache).To(BeTrue())
			g.Expect(remaining).To(Equal([]string{targetName}))

			// Deprecated --no-cache also works
			flags, _ = runner.ExtractTargFlags([]string{"--no-cache", targetName})
			g.Expect(flags.NoBinaryCache).To(BeTrue())

			// -s after target is not extracted
			value := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "value")
			flags, remaining = runner.ExtractTargFlags([]string{targetName, "-s", value})
			g.Expect(flags.SourceDir).To(BeEmpty())
			g.Expect(remaining).To(Equal([]string{targetName, "-s", value}))
		})
	})

	// Property: ParseCreateArgs parses valid arguments
	t.Run("ParseCreateArgsWorks", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			targetName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "targetName")
			shellCmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "shellCmd")

			opts, err := runner.ParseCreateArgs([]string{targetName, shellCmd})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.Name).To(Equal(targetName))
			g.Expect(opts.ShellCmd).To(Equal(shellCmd))

			// With path and options
			pathPart := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "pathPart")
			dep1 := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "dep1")
			dep2 := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "dep2")
			cachePattern := "**/*.go"

			opts, err = runner.ParseCreateArgs([]string{
				pathPart, targetName,
				"--deps", dep1, dep2,
				"--cache", cachePattern,
				shellCmd,
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.Name).To(Equal(targetName))
			g.Expect(opts.Path).To(Equal([]string{pathPart}))
			g.Expect(opts.Deps).To(Equal([]string{dep1, dep2}))
			g.Expect(opts.Cache).To(Equal([]string{cachePattern}))
			g.Expect(opts.ShellCmd).To(Equal(shellCmd))
		})
	})

	// Property: ParseCreateArgs parses --watch patterns
	t.Run("ParseCreateArgsWatch", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			targetName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "targetName")
			shellCmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "shellCmd")
			watchPattern := rapid.StringMatching(`\*\*/\*\.[a-z]{2,4}`).Draw(t, "watchPattern")

			opts, err := runner.ParseCreateArgs([]string{
				targetName, "--watch", watchPattern, shellCmd,
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.Watch).To(Equal([]string{watchPattern}))
		})
	})

	// Property: ParseCreateArgs parses --timeout
	t.Run("ParseCreateArgsTimeout", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			targetName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "targetName")
			shellCmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "shellCmd")
			secs := rapid.IntRange(1, 300).Draw(t, "secs")
			timeout := strconv.Itoa(secs) + "s"

			opts, err := runner.ParseCreateArgs([]string{
				targetName, "--timeout", timeout, shellCmd,
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.Timeout).To(Equal(timeout))
		})
	})

	// Property: ParseCreateArgs parses --times
	t.Run("ParseCreateArgsTimes", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			targetName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "targetName")
			shellCmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "shellCmd")
			times := rapid.IntRange(1, 100).Draw(t, "times")

			opts, err := runner.ParseCreateArgs([]string{
				targetName, "--times", strconv.Itoa(times), shellCmd,
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.Times).To(Equal(times))
		})
	})

	// Property: ParseCreateArgs parses --retry
	t.Run("ParseCreateArgsRetry", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			targetName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "targetName")
			shellCmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "shellCmd")

			opts, err := runner.ParseCreateArgs([]string{
				targetName, "--retry", shellCmd,
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.Retry).To(BeTrue())
		})
	})

	// Property: ParseCreateArgs parses --backoff
	t.Run("ParseCreateArgsBackoff", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			targetName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "targetName")
			shellCmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "shellCmd")
			secs := rapid.IntRange(1, 10).Draw(t, "secs")
			mult := rapid.Float64Range(1.1, 5.0).Draw(t, "mult")
			backoff := strconv.Itoa(secs) + "s," + strconv.FormatFloat(mult, 'f', 1, 64)

			opts, err := runner.ParseCreateArgs([]string{
				targetName, "--backoff", backoff, shellCmd,
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.Backoff).To(Equal(backoff))
		})
	})

	// Property: ParseCreateArgs parses --dep-mode
	t.Run("ParseCreateArgsDepMode", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			targetName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "targetName")
			shellCmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "shellCmd")
			mode := rapid.SampledFrom([]string{"serial", "parallel"}).Draw(t, "mode")

			opts, err := runner.ParseCreateArgs([]string{
				targetName, "--dep-mode", mode, shellCmd,
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.DepMode).To(Equal(mode))
		})
	})

	// Property: ParseCreateArgs rejects invalid --dep-mode
	t.Run("ParseCreateArgsInvalidDepMode", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			targetName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "targetName")
			shellCmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "shellCmd")
			badMode := rapid.StringMatching(`[a-z]{3,10}`).
				Filter(func(s string) bool { return s != "serial" && s != "parallel" }).
				Draw(t, "badMode")

			_, err := runner.ParseCreateArgs([]string{
				targetName, "--dep-mode", badMode, shellCmd,
			})
			g.Expect(err).To(HaveOccurred())
		})
	})

	// Property: Code gen includes .Watch() when watch patterns specified
	t.Run("WatchPatternsIncluded", func(t *testing.T) {
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
				Watch:    patterns,
			})
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files["targs.go"])
			g.Expect(content).To(ContainSubstring(".Watch("))

			for _, p := range patterns {
				g.Expect(content).To(ContainSubstring(p))
			}
		})
	})

	// Property: Code gen includes .Timeout() with time import
	t.Run("TimeoutCodeGen", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			fileOps := NewMemoryFileOps()
			targetName := rapid.StringMatching(`[a-z][a-z0-9]{2,8}`).Draw(t, "targetName")
			secs := rapid.IntRange(1, 300).Draw(t, "secs")

			initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
			fileOps.Files["targs.go"] = []byte(initial)

			err := runner.AddTargetToFileWithFileOps(fileOps, "targs.go", runner.CreateOptions{
				Name:     targetName,
				ShellCmd: "go test",
				Timeout:  strconv.Itoa(secs) + "s",
			})
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files["targs.go"])
			g.Expect(content).To(ContainSubstring(".Timeout("))
			g.Expect(content).To(MatchRegexp(`time\.(Second|Minute|Hour|Duration)`))
			g.Expect(content).To(ContainSubstring(`"time"`))
		})
	})

	// Property: Code gen includes .Times()
	t.Run("TimesCodeGen", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			fileOps := NewMemoryFileOps()
			targetName := rapid.StringMatching(`[a-z][a-z0-9]{2,8}`).Draw(t, "targetName")
			times := rapid.IntRange(1, 100).Draw(t, "times")

			initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
			fileOps.Files["targs.go"] = []byte(initial)

			err := runner.AddTargetToFileWithFileOps(fileOps, "targs.go", runner.CreateOptions{
				Name:     targetName,
				ShellCmd: "go test",
				Times:    times,
			})
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files["targs.go"])
			g.Expect(content).To(ContainSubstring(fmt.Sprintf(".Times(%d)", times)))
		})
	})

	// Property: Code gen includes .Retry()
	t.Run("RetryCodeGen", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			fileOps := NewMemoryFileOps()
			targetName := rapid.StringMatching(`[a-z][a-z0-9]{2,8}`).Draw(t, "targetName")

			initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
			fileOps.Files["targs.go"] = []byte(initial)

			err := runner.AddTargetToFileWithFileOps(fileOps, "targs.go", runner.CreateOptions{
				Name:     targetName,
				ShellCmd: "go test",
				Retry:    true,
			})
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files["targs.go"])
			g.Expect(content).To(ContainSubstring(".Retry()"))
		})
	})

	// Property: Code gen includes .Backoff() with time import
	t.Run("BackoffCodeGen", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			fileOps := NewMemoryFileOps()
			targetName := rapid.StringMatching(`[a-z][a-z0-9]{2,8}`).Draw(t, "targetName")

			initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
			fileOps.Files["targs.go"] = []byte(initial)

			err := runner.AddTargetToFileWithFileOps(fileOps, "targs.go", runner.CreateOptions{
				Name:     targetName,
				ShellCmd: "go test",
				Backoff:  "1s,2.0",
			})
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files["targs.go"])
			g.Expect(content).To(ContainSubstring(".Backoff("))
			g.Expect(content).To(ContainSubstring("time.Second"))
			g.Expect(content).To(ContainSubstring(`"time"`))
		})
	})

	// Property: Code gen includes dep-mode in .Deps() call
	t.Run("DepModeCodeGen", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			fileOps := NewMemoryFileOps()
			targetName := rapid.StringMatching(`[a-z][a-z0-9]{2,8}`).Draw(t, "targetName")
			dep := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "dep")

			initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
			fileOps.Files["targs.go"] = []byte(initial)

			err := runner.AddTargetToFileWithFileOps(fileOps, "targs.go", runner.CreateOptions{
				Name:     targetName,
				ShellCmd: "go test",
				Deps:     []string{dep},
				DepMode:  "parallel",
			})
			g.Expect(err).NotTo(HaveOccurred())

			content := string(fileOps.Files["targs.go"])
			g.Expect(content).To(ContainSubstring("targ.DepModeParallel"))
		})
	})

	// Codegen for dep groups: Single-group codegen is sufficient for targ create.
	// Users needing chained groups (e.g., .Deps(a).Deps(b, c, parallel).Deps(d))
	// can manually edit generated code. The generated single-group code is
	// compatible with the new group-based dep system.
	t.Run("DepGroupCodeGenSingleGroup", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		fileOps := NewMemoryFileOps()
		initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
		fileOps.Files["targs.go"] = []byte(initial)

		// Generate a target with serial deps
		err := runner.AddTargetToFileWithFileOps(fileOps, "targs.go", runner.CreateOptions{
			Name:     "main",
			ShellCmd: "go build",
			Deps:     []string{"dep-one", "dep-two"},
			DepMode:  "serial",
		})
		g.Expect(err).NotTo(HaveOccurred())

		content := string(fileOps.Files["targs.go"])
		// Should generate single .Deps() call (creates single dep group)
		g.Expect(content).To(ContainSubstring(".Deps(DepOne, DepTwo)"))
		// Serial is default, so no mode specified
		g.Expect(content).NotTo(ContainSubstring("DepModeParallel"))
	})

	// Property: ParseSyncArgs validates package paths
	t.Run("ParseSyncArgsValidatesPath", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			user := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "user")
			repo := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "repo")
			validPath := "github.com/" + user + "/" + repo

			opts, err := runner.ParseSyncArgs([]string{validPath})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.PackagePath).To(Equal(validPath))

			invalidPath := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "invalidPath")
			_, err = runner.ParseSyncArgs([]string{invalidPath})
			g.Expect(err).To(HaveOccurred())
		})
	})

	// Property: ParseHelpRequest distinguishes top-level vs target help
	t.Run("ParseHelpRequestDistinguishes", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			targetName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "targetName")

			help, target := runner.ParseHelpRequest([]string{"--help"})
			g.Expect(help).To(BeTrue())
			g.Expect(target).To(BeFalse())

			help, target = runner.ParseHelpRequest([]string{targetName, "--help"})
			g.Expect(help).To(BeTrue())
			g.Expect(target).To(BeTrue())
		})
	})

	// Property: NamespacePaths computes correct paths
	t.Run("NamespacePathsComputes", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			rootDir := "/" + rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "rootDir")
			subDir1 := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "subDir1")
			subDir2 := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "subDir2")
			file1 := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "file1")
			file2 := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "file2")

			if subDir1 == subDir2 || file1 == file2 || file1 == subDir2 || file2 == subDir2 {
				t.Skip("avoid duplicate namespace compression cases")
			}

			files := []string{
				rootDir + "/tools/" + subDir1 + "/" + subDir1 + ".go",
				rootDir + "/tools/" + subDir2 + "/" + file1 + ".go",
				rootDir + "/tools/" + subDir2 + "/" + file2 + ".go",
			}

			paths, err := runner.NamespacePaths(files, rootDir)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(paths[files[0]]).To(Equal([]string{subDir1}))
			g.Expect(paths[files[1]]).To(Equal([]string{subDir2, file1}))
			g.Expect(paths[files[2]]).To(Equal([]string{subDir2, file2}))
		})
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

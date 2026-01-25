package runner_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/runner"
)

func TestProperty_CodeGeneration(t *testing.T) {
	t.Parallel()

	t.Run("ValidTargetNameRules", func(t *testing.T) {
		t.Parallel()

		t.Run("ValidNames", func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				g := NewWithT(t)
				// Generate valid names: lowercase letters, may contain hyphens (not at start/end)
				name := rapid.StringMatching(`[a-z][a-z0-9]*(-[a-z0-9]+)*`).Draw(t, "name")
				g.Expect(runner.IsValidTargetName(name)).
					To(BeTrue(), "name %q should be valid", name)
			})
		})

		t.Run("InvalidNames", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			invalidNames := []string{
				"",          // empty
				"123",       // starts with number
				"-lint",     // starts with hyphen
				"lint-",     // ends with hyphen
				"Lint",      // uppercase
				"my_target", // underscore
				"my.target", // dot
				"my target", // space
			}

			for _, name := range invalidNames {
				g.Expect(runner.IsValidTargetName(name)).
					To(BeFalse(), "name %q should be invalid", name)
			}
		})
	})

	t.Run("AddTargetToFile", func(t *testing.T) {
		t.Parallel()

		t.Run("GeneratesValidTarget", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()
			targFile := filepath.Join(dir, "targs.go")

			initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
			err := os.WriteFile(targFile, []byte(initial), 0o644)
			g.Expect(err).NotTo(HaveOccurred())

			err = runner.AddTargetToFile(targFile, "my-lint", "golangci-lint run")
			g.Expect(err).NotTo(HaveOccurred())

			content, err := os.ReadFile(targFile)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(string(content)).To(ContainSubstring("var MyLint = targ.Targ"))
			g.Expect(string(content)).To(ContainSubstring("golangci-lint run"))
			g.Expect(string(content)).To(ContainSubstring(`.Name("my-lint")`))
		})

		t.Run("RejectsDuplicateTargets", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()
			targFile := filepath.Join(dir, "targs.go")

			initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
			err := os.WriteFile(targFile, []byte(initial), 0o644)
			g.Expect(err).NotTo(HaveOccurred())

			err = runner.AddTargetToFile(targFile, "lint", "golangci-lint run")
			g.Expect(err).NotTo(HaveOccurred())

			err = runner.AddTargetToFile(targFile, "lint", "different command")
			g.Expect(err).To(HaveOccurred())
		})
	})

	t.Run("AddTargetWithOptions", func(t *testing.T) {
		t.Parallel()

		t.Run("AddsToExistingGroup", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()
			targFile := filepath.Join(dir, "targs.go")

			initial := `//go:build targ

package build

import "github.com/toejough/targ"

var DevLintSlow = targ.Targ("golangci-lint run").Name("slow")
var DevLint = targ.Group("lint", DevLintSlow)
var Dev = targ.Group("dev", DevLint)
`
			err := os.WriteFile(targFile, []byte(initial), 0o644)
			g.Expect(err).NotTo(HaveOccurred())

			opts := runner.CreateOptions{
				Name:     "fast",
				ShellCmd: "golangci-lint run --fast",
				Path:     []string{"dev", "lint"},
			}

			err = runner.AddTargetToFileWithOptions(targFile, opts)
			g.Expect(err).NotTo(HaveOccurred())

			content, _ := os.ReadFile(targFile)
			contentStr := string(content)

			g.Expect(contentStr).To(ContainSubstring("var DevLintFast = "))
			g.Expect(contentStr).
				To(ContainSubstring(`var DevLint = targ.Group("lint", DevLintSlow, DevLintFast)`))
		})

		t.Run("IncludesCachePatterns", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()
			targFile := filepath.Join(dir, "targs.go")

			initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"
			err := os.WriteFile(targFile, []byte(initial), 0o644)
			g.Expect(err).NotTo(HaveOccurred())

			opts := runner.CreateOptions{
				Name:     "build",
				ShellCmd: "go build",
				Cache:    []string{"**/*.go", "go.mod"},
			}

			err = runner.AddTargetToFileWithOptions(targFile, opts)
			g.Expect(err).NotTo(HaveOccurred())

			content, _ := os.ReadFile(targFile)
			g.Expect(string(content)).To(ContainSubstring(`.Cache("**/*.go", "go.mod")`))
		})
	})

	t.Run("ConvertFuncToString", func(t *testing.T) {
		t.Parallel()

		t.Run("PreservesCommand", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()
			targFile := filepath.Join(dir, "targs.go")

			initial := `//go:build targ

package build

import "github.com/toejough/targ"
import "github.com/toejough/targ/sh"

var Lint = targ.Targ(lint).Name("lint")

func lint() error {
	return sh.Run("golangci-lint", "run")
}
`
			err := os.WriteFile(targFile, []byte(initial), 0o644)
			g.Expect(err).NotTo(HaveOccurred())

			ok, err := runner.ConvertFuncTargetToString(targFile, "lint")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(ok).To(BeTrue())

			content, _ := os.ReadFile(targFile)
			contentStr := string(content)

			g.Expect(contentStr).To(ContainSubstring(`targ.Targ("golangci-lint run")`))
			g.Expect(contentStr).NotTo(ContainSubstring("func lint()"))
		})

		t.Run("RejectsComplexFunc", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()
			targFile := filepath.Join(dir, "targs.go")

			initial := `//go:build targ

package build

import "github.com/toejough/targ"
import "github.com/toejough/targ/sh"
import "fmt"

var Lint = targ.Targ(lint).Name("lint")

func lint() error {
	fmt.Println("Running lint...")
	return sh.Run("golangci-lint", "run")
}
`
			err := os.WriteFile(targFile, []byte(initial), 0o644)
			g.Expect(err).NotTo(HaveOccurred())

			_, err = runner.ConvertFuncTargetToString(targFile, "lint")
			g.Expect(err).To(HaveOccurred())
		})
	})

	t.Run("ConvertStringToFunc", func(t *testing.T) {
		t.Parallel()

		t.Run("AddsFuncDeclaration", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()
			targFile := filepath.Join(dir, "targs.go")

			initial := `//go:build targ

package build

import "github.com/toejough/targ"

var Lint = targ.Targ("golangci-lint run").Name("lint")
`
			err := os.WriteFile(targFile, []byte(initial), 0o644)
			g.Expect(err).NotTo(HaveOccurred())

			ok, err := runner.ConvertStringTargetToFunc(targFile, "lint")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(ok).To(BeTrue())

			content, _ := os.ReadFile(targFile)
			contentStr := string(content)

			g.Expect(contentStr).NotTo(ContainSubstring(`targ.Targ("golangci-lint run")`))
			g.Expect(contentStr).To(ContainSubstring("func lint()"))
			g.Expect(contentStr).To(ContainSubstring("targ.Run("))
		})

		t.Run("NotFoundReturnsFalse", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()
			targFile := filepath.Join(dir, "targs.go")

			initial := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ("go build").Name("build")
`
			err := os.WriteFile(targFile, []byte(initial), 0o644)
			g.Expect(err).NotTo(HaveOccurred())

			ok, err := runner.ConvertStringTargetToFunc(targFile, "lint")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(ok).To(BeFalse())
		})
	})

	t.Run("FindOrCreateTargFile", func(t *testing.T) {
		t.Parallel()

		t.Run("CreatesNewFile", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()

			targFile, err := runner.FindOrCreateTargFile(dir)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(targFile).To(Equal(filepath.Join(dir, "targs.go")))

			content, err := os.ReadFile(targFile)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(string(content)).To(ContainSubstring("//go:build targ"))
			g.Expect(string(content)).To(ContainSubstring(`import "github.com/toejough/targ"`))
		})

		t.Run("FindsExistingFile", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()
			existingFile := filepath.Join(dir, "build.go")
			content := "//go:build targ\n\npackage build\n"

			err := os.WriteFile(existingFile, []byte(content), 0o644)
			g.Expect(err).NotTo(HaveOccurred())

			targFile, err := runner.FindOrCreateTargFile(dir)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(targFile).To(Equal(existingFile))
		})
	})

	t.Run("HasTargBuildTag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		dir := t.TempDir()

		withTag := filepath.Join(dir, "with_tag.go")
		err := os.WriteFile(withTag, []byte("//go:build targ\n\npackage foo\n"), 0o644)
		g.Expect(err).NotTo(HaveOccurred())

		withoutTag := filepath.Join(dir, "without_tag.go")
		err = os.WriteFile(withoutTag, []byte("package foo\n"), 0o644)
		g.Expect(err).NotTo(HaveOccurred())

		otherTag := filepath.Join(dir, "other_tag.go")
		err = os.WriteFile(otherTag, []byte("//go:build integration\n\npackage foo\n"), 0o644)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(runner.HasTargBuildTag(withTag)).To(BeTrue())
		g.Expect(runner.HasTargBuildTag(withoutTag)).To(BeFalse())
		g.Expect(runner.HasTargBuildTag(otherTag)).To(BeFalse())
	})

	t.Run("AddImportToTargFile", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		dir := t.TempDir()
		targFile := filepath.Join(dir, "targs.go")

		initial := `//go:build targ

package build

import "github.com/toejough/targ"

var Lint = targ.Targ("golangci-lint run")
`
		err := os.WriteFile(targFile, []byte(initial), 0o644)
		g.Expect(err).NotTo(HaveOccurred())

		err = runner.AddImportToTargFile(targFile, "github.com/foo/bar")
		g.Expect(err).NotTo(HaveOccurred())

		content, _ := os.ReadFile(targFile)
		contentStr := string(content)

		g.Expect(contentStr).To(ContainSubstring(`_ "github.com/foo/bar"`))
		g.Expect(contentStr).To(ContainSubstring(`"github.com/toejough/targ"`))
		g.Expect(contentStr).To(ContainSubstring("var Lint"))
	})

	t.Run("CheckImportExists", func(t *testing.T) {
		t.Parallel()

		t.Run("ReturnsTrue", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()
			targFile := filepath.Join(dir, "targs.go")

			content := `//go:build targ

package build

import (
	"github.com/toejough/targ"
	_ "github.com/foo/bar"
)
`
			err := os.WriteFile(targFile, []byte(content), 0o644)
			g.Expect(err).NotTo(HaveOccurred())

			exists, err := runner.CheckImportExists(targFile, "github.com/foo/bar")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exists).To(BeTrue())
		})

		t.Run("ReturnsFalse", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()
			targFile := filepath.Join(dir, "targs.go")

			content := `//go:build targ

package build

import "github.com/toejough/targ"
`
			err := os.WriteFile(targFile, []byte(content), 0o644)
			g.Expect(err).NotTo(HaveOccurred())

			exists, err := runner.CheckImportExists(targFile, "github.com/foo/bar")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exists).To(BeFalse())
		})
	})

	t.Run("FindModuleForPath", func(t *testing.T) {
		t.Parallel()

		t.Run("NoModule", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dir := t.TempDir()

			root, modulePath, found, err := runner.FindModuleForPath(dir)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeFalse())
			g.Expect(root).To(BeEmpty())
			g.Expect(modulePath).To(BeEmpty())
		})

		t.Run("WalksUp", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			parent := t.TempDir()
			modContent := "module example.com/parent\n\ngo 1.21\n"
			err := os.WriteFile(filepath.Join(parent, "go.mod"), []byte(modContent), 0o644)
			g.Expect(err).NotTo(HaveOccurred())

			child := filepath.Join(parent, "child")
			err = os.MkdirAll(child, 0o755)
			g.Expect(err).NotTo(HaveOccurred())

			root, modulePath, found, err := runner.FindModuleForPath(child)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(root).To(Equal(parent))
			g.Expect(modulePath).To(Equal("example.com/parent"))
		})
	})

	t.Run("ExtractTargFlags", func(t *testing.T) {
		t.Parallel()

		t.Run("NoBinaryCache", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			flags, remaining := runner.ExtractTargFlags([]string{"--no-binary-cache", "build"})
			g.Expect(flags.NoBinaryCache).To(BeTrue())
			g.Expect(remaining).To(Equal([]string{"build"}))
		})

		t.Run("DeprecatedNoCache", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			flags, _ := runner.ExtractTargFlags([]string{"--no-cache", "build"})
			g.Expect(flags.NoBinaryCache).To(BeTrue())
		})

		t.Run("ShortSAfterTarget", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			flags, remaining := runner.ExtractTargFlags([]string{"build", "-s", "value"})
			g.Expect(flags.SourceDir).To(BeEmpty())
			g.Expect(remaining).To(Equal([]string{"build", "-s", "value"}))
		})
	})

	t.Run("ParseCreateArgs", func(t *testing.T) {
		t.Parallel()

		t.Run("Basic", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			opts, err := runner.ParseCreateArgs([]string{"lint", "golangci-lint run"})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.Name).To(Equal("lint"))
			g.Expect(opts.ShellCmd).To(Equal("golangci-lint run"))
			g.Expect(opts.Path).To(BeEmpty())
		})

		t.Run("FullOptions", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			opts, err := runner.ParseCreateArgs([]string{
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
	})

	t.Run("ParseSyncArgs", func(t *testing.T) {
		t.Parallel()

		t.Run("ValidPath", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			opts, err := runner.ParseSyncArgs([]string{"github.com/foo/bar"})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.PackagePath).To(Equal("github.com/foo/bar"))
		})

		t.Run("InvalidPath", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			_, err := runner.ParseSyncArgs([]string{"invalid-path"})
			g.Expect(err).To(HaveOccurred())
		})
	})

	t.Run("ParseHelpRequest", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		help, target := runner.ParseHelpRequest([]string{"issues", "--help"})
		g.Expect(help && !target).To(BeFalse(), "expected help to be scoped to target")

		help, target = runner.ParseHelpRequest([]string{"--help"})
		g.Expect(help && !target).To(BeTrue(), "expected top-level help")
	})

	t.Run("NamespacePaths", func(t *testing.T) {
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

	t.Run("WriteBootstrapFile", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		dir := t.TempDir()
		data := []byte("package main\n")

		path, cleanup, err := runner.WriteBootstrapFile(dir, data)
		g.Expect(err).NotTo(HaveOccurred())

		_, err = os.Stat(path)
		g.Expect(err).NotTo(HaveOccurred())

		err = cleanup()
		g.Expect(err).NotTo(HaveOccurred())

		_, err = os.Stat(path)
		g.Expect(os.IsNotExist(err)).To(BeTrue())
	})

	t.Run("EnsureFallbackModuleRoot", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("symlink behavior is restricted on windows")
		}

		g := NewWithT(t)

		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("ok"), 0o644)
		g.Expect(err).NotTo(HaveOccurred())

		root, err := runner.EnsureFallbackModuleRoot(
			dir,
			"targ.local",
			runner.TargDependency{ModulePath: "github.com/toejough/targ", Version: "v0.0.0"},
		)
		g.Expect(err).NotTo(HaveOccurred())

		_, err = os.Stat(filepath.Join(root, "go.mod"))
		g.Expect(err).NotTo(HaveOccurred())

		link := filepath.Join(root, "file.txt")
		info, err := os.Lstat(link)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(info.Mode() & os.ModeSymlink).NotTo(Equal(os.FileMode(0)))
	})
}

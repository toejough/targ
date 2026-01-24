package runner_test

// LEGACY_TESTS: This file contains tests being evaluated for redundancy.
// Property-based replacements are in *_properties_test.go files.
// Do not add new tests here. See docs/test-migration.md for details.

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/toejough/targ/internal/runner"
)

func TestAddImportToTargFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	initial := `//go:build targ

package build

import "github.com/toejough/targ"

var Lint = targ.Targ("golangci-lint run")
`
	if err := os.WriteFile(targFile, []byte(initial), 0o644); err != nil {
		t.Fatalf("write error: %v", err)
	}

	err := runner.AddImportToTargFile(targFile, "github.com/foo/bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(targFile)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	contentStr := string(content)

	// Should have the new blank import
	if !strings.Contains(contentStr, `_ "github.com/foo/bar"`) {
		t.Errorf("expected blank import, got:\n%s", contentStr)
	}

	// Should still have the original import
	if !strings.Contains(contentStr, `"github.com/toejough/targ"`) {
		t.Errorf("expected targ import to remain, got:\n%s", contentStr)
	}

	// Should still have the target
	if !strings.Contains(contentStr, "var Lint") {
		t.Errorf("expected Lint variable to remain, got:\n%s", contentStr)
	}
}

func TestAddImportToTargFile_GroupedImports(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	initial := `//go:build targ

package build

import (
	"github.com/toejough/targ"
)

var Lint = targ.Targ("golangci-lint run")
`
	if err := os.WriteFile(targFile, []byte(initial), 0o644); err != nil {
		t.Fatalf("write error: %v", err)
	}

	err := runner.AddImportToTargFile(targFile, "github.com/foo/bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(targFile)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	contentStr := string(content)

	// Should have both imports in the import block
	if !strings.Contains(contentStr, `_ "github.com/foo/bar"`) {
		t.Errorf("expected blank import, got:\n%s", contentStr)
	}
}

func TestAddTargetToFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	// Create initial file
	initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"

	err := os.WriteFile(targFile, []byte(initial), 0o644)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	// Add a target
	err = runner.AddTargetToFile(targFile, "my-lint", "golangci-lint run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(targFile)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	if !strings.Contains(string(content), "var MyLint = targ.Targ") {
		t.Error("expected MyLint variable in file")
	}

	if !strings.Contains(string(content), "golangci-lint run") {
		t.Error("expected shell command in file")
	}

	if !strings.Contains(string(content), ".Name(\"my-lint\")") {
		t.Error("expected Name method call in file")
	}

	// Try adding duplicate
	err = runner.AddTargetToFile(targFile, "my-lint", "different command")
	if err == nil {
		t.Error("expected error for duplicate target")
	}
}

func TestAddTargetToFileWithOptions_AddsToExistingGroup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	// Initial file with existing groups and target
	initial := `//go:build targ

package build

import "github.com/toejough/targ"

var DevLintSlow = targ.Targ("golangci-lint run").Name("slow")
var DevLint = targ.Group("lint", DevLintSlow)
var Dev = targ.Group("dev", DevLint)
`

	err := os.WriteFile(targFile, []byte(initial), 0o644)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	// Add a new target to the existing path
	opts := runner.CreateOptions{
		Name:     "fast",
		ShellCmd: "golangci-lint run --fast",
		Path:     []string{"dev", "lint"},
	}

	err = runner.AddTargetToFileWithOptions(targFile, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(targFile)
	contentStr := string(content)

	// Should have the new target
	if !strings.Contains(contentStr, "var DevLintFast = ") {
		t.Errorf("expected DevLintFast variable, got:\n%s", contentStr)
	}

	// DevLint group should now contain both targets
	if !strings.Contains(
		contentStr,
		`var DevLint = targ.Group("lint", DevLintSlow, DevLintFast)`,
	) {
		t.Errorf("expected DevLint group to contain both targets, got:\n%s", contentStr)
	}

	// Dev group should remain unchanged (still contains DevLint)
	if !strings.Contains(contentStr, `var Dev = targ.Group("dev", DevLint)`) {
		t.Errorf("expected Dev group unchanged, got:\n%s", contentStr)
	}

	// Should NOT have duplicate group declarations
	if strings.Count(contentStr, "var DevLint = ") != 1 {
		t.Errorf("expected exactly one DevLint declaration, got:\n%s", contentStr)
	}
}

func TestAddTargetToFileWithOptions_AddsToPartialPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	// Initial file with only top-level group
	initial := `//go:build targ

package build

import "github.com/toejough/targ"

var DevBuild = targ.Targ("go build").Name("build")
var Dev = targ.Group("dev", DevBuild)
`

	err := os.WriteFile(targFile, []byte(initial), 0o644)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	// Add a new target with a deeper path (dev/lint/fast)
	opts := runner.CreateOptions{
		Name:     "fast",
		ShellCmd: "golangci-lint run --fast",
		Path:     []string{"dev", "lint"},
	}

	err = runner.AddTargetToFileWithOptions(targFile, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(targFile)
	contentStr := string(content)

	// Should have the new target
	if !strings.Contains(contentStr, "var DevLintFast = ") {
		t.Errorf("expected DevLintFast variable, got:\n%s", contentStr)
	}

	// Should have new DevLint group (didn't exist before)
	if !strings.Contains(contentStr, `var DevLint = targ.Group("lint", DevLintFast)`) {
		t.Errorf("expected new DevLint group, got:\n%s", contentStr)
	}

	// Dev group should now contain both DevBuild and DevLint
	if !strings.Contains(contentStr, `var Dev = targ.Group("dev", DevBuild, DevLint)`) {
		t.Errorf("expected Dev group to contain both members, got:\n%s", contentStr)
	}
}

func TestAddTargetToFileWithOptions_WithCache(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"

	err := os.WriteFile(targFile, []byte(initial), 0o644)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	opts := runner.CreateOptions{
		Name:     "build",
		ShellCmd: "go build",
		Cache:    []string{"**/*.go", "go.mod"},
	}

	err = runner.AddTargetToFileWithOptions(targFile, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(targFile)
	contentStr := string(content)

	if !strings.Contains(contentStr, `.Cache("**/*.go", "go.mod")`) {
		t.Errorf("expected .Cache pattern in generated code, got:\n%s", contentStr)
	}
}

func TestAddTargetToFileWithOptions_WithDeps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"

	err := os.WriteFile(targFile, []byte(initial), 0o644)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	opts := runner.CreateOptions{
		Name:     "build",
		ShellCmd: "go build",
		Deps:     []string{"lint", "test"},
	}

	err = runner.AddTargetToFileWithOptions(targFile, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(targFile)
	contentStr := string(content)

	if !strings.Contains(contentStr, ".Deps(Lint, Test)") {
		t.Error("expected .Deps(Lint, Test) in generated code")
	}
}

func TestAddTargetToFileWithOptions_WithPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	initial := "//go:build targ\n\npackage build\n\nimport \"github.com/toejough/targ\"\n"

	err := os.WriteFile(targFile, []byte(initial), 0o644)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	opts := runner.CreateOptions{
		Name:     "fast",
		ShellCmd: "golangci-lint run --fast",
		Path:     []string{"dev", "lint"},
	}

	err = runner.AddTargetToFileWithOptions(targFile, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(targFile)
	contentStr := string(content)

	// Should have the target with full path name
	if !strings.Contains(contentStr, "var DevLintFast = ") {
		t.Errorf("expected DevLintFast variable, got:\n%s", contentStr)
	}

	// Should have groups
	if !strings.Contains(contentStr, `var DevLint = targ.Group("lint", DevLintFast)`) {
		t.Errorf("expected DevLint group, got:\n%s", contentStr)
	}

	if !strings.Contains(contentStr, `var Dev = targ.Group("dev", DevLint)`) {
		t.Errorf("expected Dev group, got:\n%s", contentStr)
	}
}

func TestCheckImportExists_Exists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	content := `//go:build targ

package build

import (
	"github.com/toejough/targ"
	_ "github.com/foo/bar"
)
`
	if err := os.WriteFile(targFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write error: %v", err)
	}

	exists, err := runner.CheckImportExists(targFile, "github.com/foo/bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !exists {
		t.Error("expected import to exist")
	}
}

func TestCheckImportExists_NotExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	content := `//go:build targ

package build

import "github.com/toejough/targ"
`
	if err := os.WriteFile(targFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write error: %v", err)
	}

	exists, err := runner.CheckImportExists(targFile, "github.com/foo/bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exists {
		t.Error("expected import to not exist")
	}
}

func TestConvertFuncTargetToString(t *testing.T) {
	t.Parallel()

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
	if err := os.WriteFile(targFile, []byte(initial), 0o644); err != nil {
		t.Fatalf("write error: %v", err)
	}

	ok, err := runner.ConvertFuncTargetToString(targFile, "lint")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ok {
		t.Fatal("expected conversion to succeed")
	}

	content, err := os.ReadFile(targFile)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	contentStr := string(content)

	// Should have string instead of function reference
	if !strings.Contains(contentStr, `targ.Targ("golangci-lint run")`) {
		t.Errorf("expected string target, got:\n%s", contentStr)
	}

	// Should NOT have function declaration
	if strings.Contains(contentStr, "func lint()") {
		t.Errorf("expected lint function to be removed, got:\n%s", contentStr)
	}
}

func TestConvertFuncTargetToString_ComplexFunc(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	// Function with multiple statements should not be converted
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
	if err := os.WriteFile(targFile, []byte(initial), 0o644); err != nil {
		t.Fatalf("write error: %v", err)
	}

	_, err := runner.ConvertFuncTargetToString(targFile, "lint")
	if err == nil {
		t.Error("expected error for complex function")
	}
}

func TestConvertStringTargetToFunc(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	initial := `//go:build targ

package build

import "github.com/toejough/targ"

var Lint = targ.Targ("golangci-lint run").Name("lint")
`
	if err := os.WriteFile(targFile, []byte(initial), 0o644); err != nil {
		t.Fatalf("write error: %v", err)
	}

	ok, err := runner.ConvertStringTargetToFunc(targFile, "lint")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ok {
		t.Fatal("expected conversion to succeed")
	}

	content, err := os.ReadFile(targFile)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	contentStr := string(content)

	// Should have function reference instead of string
	if strings.Contains(contentStr, `targ.Targ("golangci-lint run")`) {
		t.Errorf("expected string to be replaced, got:\n%s", contentStr)
	}

	// Should have function declaration
	if !strings.Contains(contentStr, "func lint()") {
		t.Errorf("expected lint function, got:\n%s", contentStr)
	}

	// Should have targ.Run call
	if !strings.Contains(contentStr, "targ.Run(") {
		t.Errorf("expected targ.Run call, got:\n%s", contentStr)
	}
}

func TestConvertStringTargetToFunc_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targFile := filepath.Join(dir, "targs.go")

	initial := `//go:build targ

package build

import "github.com/toejough/targ"

var Build = targ.Targ("go build").Name("build")
`
	if err := os.WriteFile(targFile, []byte(initial), 0o644); err != nil {
		t.Fatalf("write error: %v", err)
	}

	ok, err := runner.ConvertStringTargetToFunc(targFile, "lint")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ok {
		t.Error("expected conversion to return false for non-existent target")
	}
}

func TestCreateGroupMemberPatch(t *testing.T) {
	t.Parallel()

	content := `var DevLint = targ.Group("lint", DevLintSlow)`

	patch := runner.CreateGroupMemberPatch(content, "DevLint", "DevLintFast")
	if patch == nil {
		t.Fatal("expected patch, got nil")
	}

	if patch.Old != `var DevLint = targ.Group("lint", DevLintSlow)` {
		t.Errorf("unexpected Old: %q", patch.Old)
	}

	if patch.New != `var DevLint = targ.Group("lint", DevLintSlow, DevLintFast)` {
		t.Errorf("unexpected New: %q", patch.New)
	}
}

func TestCreateGroupMemberPatch_AlreadyExists(t *testing.T) {
	t.Parallel()

	content := `var DevLint = targ.Group("lint", DevLintSlow, DevLintFast)`

	patch := runner.CreateGroupMemberPatch(content, "DevLint", "DevLintFast")
	if patch != nil {
		t.Error("expected nil patch when member already exists")
	}
}

func TestEnsureFallbackModuleRoot(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior is restricted on windows")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	root, err := runner.EnsureFallbackModuleRoot(
		dir,
		"targ.local",
		runner.TargDependency{ModulePath: "github.com/toejough/targ", Version: "v0.0.0"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = os.Stat(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("expected go.mod, got error: %v", err)
	}

	link := filepath.Join(root, "file.txt")

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("expected link, got error: %v", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink at %s", link)
	}
}

func TestExtractTargFlags_DeprecatedNoCache(t *testing.T) {
	t.Parallel()

	args := []string{"--no-cache", "build"}

	flags, _ := runner.ExtractTargFlags(args)

	if !flags.NoBinaryCache {
		t.Error("expected NoBinaryCache to be true (via deprecated --no-cache)")
	}
}

func TestExtractTargFlags_Empty(t *testing.T) {
	t.Parallel()

	args := []string{}

	flags, remaining := runner.ExtractTargFlags(args)

	if flags.NoBinaryCache {
		t.Error("expected NoBinaryCache to be false")
	}

	if flags.SourceDir != "" {
		t.Errorf("expected empty source dir, got %q", flags.SourceDir)
	}

	if len(remaining) != 0 {
		t.Errorf("expected empty remaining, got %v", remaining)
	}
}

func TestExtractTargFlags_NoBinaryCache(t *testing.T) {
	t.Parallel()

	args := []string{"--no-binary-cache", "build"}

	flags, remaining := runner.ExtractTargFlags(args)

	if !flags.NoBinaryCache {
		t.Error("expected NoBinaryCache to be true")
	}

	if len(remaining) != 1 || remaining[0] != "build" {
		t.Errorf("expected remaining [build], got %v", remaining)
	}
}

func TestExtractTargFlags_ShortSAfterTarget(t *testing.T) {
	t.Parallel()
	// -s AFTER target name is passed through (for target's own flags)
	args := []string{"build", "-s", "value"}

	flags, remaining := runner.ExtractTargFlags(args)

	if flags.SourceDir != "" {
		t.Errorf("expected empty source dir (position-sensitive), got %q", flags.SourceDir)
	}

	if len(remaining) != 3 {
		t.Errorf("expected remaining [build -s value], got %v", remaining)
	}
}

func TestExtractTargFlags_SourceAfterTarget(t *testing.T) {
	t.Parallel()
	// --source AFTER target name is passed through (not recognized as global)
	args := []string{"build", "--source", "/some/path"}

	flags, remaining := runner.ExtractTargFlags(args)

	if flags.SourceDir != "" {
		t.Errorf("expected empty source dir (position-sensitive), got %q", flags.SourceDir)
	}

	if len(remaining) != 3 {
		t.Errorf("expected remaining to have 3 elements, got %v", remaining)
	}
}

func TestExtractTargFlags_SourceLong(t *testing.T) {
	t.Parallel()

	args := []string{"--source", "/some/path", "build"}

	flags, remaining := runner.ExtractTargFlags(args)

	if flags.SourceDir != "/some/path" {
		t.Errorf("expected source dir '/some/path', got %q", flags.SourceDir)
	}

	if len(remaining) != 1 || remaining[0] != "build" {
		t.Errorf("expected remaining [build], got %v", remaining)
	}
}

func TestExtractTargFlags_SourceMissingPath(t *testing.T) {
	t.Parallel()
	// Test when --source is provided without a following argument
	args := []string{"--source"}

	flags, _ := runner.ExtractTargFlags(args)

	// Source should be empty since no path argument followed
	if flags.SourceDir != "" {
		t.Errorf("expected empty source dir when path missing, got %q", flags.SourceDir)
	}
}

func TestExtractTargFlags_SourceShort(t *testing.T) {
	t.Parallel()

	args := []string{"-s", "/some/path", "build"}

	flags, remaining := runner.ExtractTargFlags(args)

	if flags.SourceDir != "/some/path" {
		t.Errorf("expected source dir '/some/path', got %q", flags.SourceDir)
	}

	if len(remaining) != 1 || remaining[0] != "build" {
		t.Errorf("expected remaining [build], got %v", remaining)
	}
}

func TestExtractTargFlags_SourceWithEquals(t *testing.T) {
	t.Parallel()
	// The current implementation doesn't support --source=/path format
	// This test documents current behavior (equals sign not supported)
	args := []string{"--source=/some/path", "build"}

	flags, remaining := runner.ExtractTargFlags(args)

	// Current behavior: --source=/some/path is not recognized, passed through
	if flags.SourceDir != "" {
		t.Errorf("unexpected source dir %q (equals format not supported)", flags.SourceDir)
	}

	if len(remaining) != 2 {
		t.Errorf("expected remaining to have 2 elements, got %v", remaining)
	}
}

func TestExtractTargFlags_SourceWithOtherFlags(t *testing.T) {
	t.Parallel()

	args := []string{"--no-binary-cache", "-s", "/some/path", "build"}

	flags, remaining := runner.ExtractTargFlags(args)

	if !flags.NoBinaryCache {
		t.Error("expected NoBinaryCache to be true")
	}

	if flags.SourceDir != "/some/path" {
		t.Errorf("expected source dir '/some/path', got %q", flags.SourceDir)
	}

	if len(remaining) != 1 || remaining[0] != "build" {
		t.Errorf("expected remaining [build], got %v", remaining)
	}
}

func TestFindModuleForPath_NoModule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	root, modulePath, found, err := runner.FindModuleForPath(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if found {
		t.Fatal("expected no module to be found")
	}

	if root != "" || modulePath != "" {
		t.Fatalf("expected empty results, got root=%q module=%q", root, modulePath)
	}
}

func TestFindModuleForPath_WalksUp(t *testing.T) {
	t.Parallel()
	// Create parent with go.mod
	parent := t.TempDir()

	modContent := "module example.com/parent\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(parent, "go.mod"), []byte(modContent), 0o644); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	// Create child without go.mod
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("unexpected mkdir error: %v", err)
	}

	// Should find parent's go.mod by walking up
	root, modulePath, found, err := runner.FindModuleForPath(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !found {
		t.Fatal("expected module to be found by walking up")
	}

	if root != parent {
		t.Fatalf("expected root=%q, got %q", parent, root)
	}

	if modulePath != "example.com/parent" {
		t.Fatalf("expected modulePath=%q, got %q", "example.com/parent", modulePath)
	}
}

func TestFindModuleForPath_WithFile(t *testing.T) {
	t.Parallel()
	// Test that FindModuleForPath works when given a file path
	parent := t.TempDir()

	modContent := "module example.com/parent\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(parent, "go.mod"), []byte(modContent), 0o644); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	// Create a subdirectory with a file
	child := filepath.Join(parent, "pkg")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("unexpected mkdir error: %v", err)
	}

	targetFile := filepath.Join(child, "main.go")
	if err := os.WriteFile(targetFile, []byte("package main"), 0o644); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	// Should find parent's go.mod when given a file path
	root, modulePath, found, err := runner.FindModuleForPath(targetFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !found {
		t.Fatal("expected module to be found by walking up from file")
	}

	if root != parent {
		t.Fatalf("expected root=%q, got %q", parent, root)
	}

	if modulePath != "example.com/parent" {
		t.Fatalf("expected modulePath=%q, got %q", "example.com/parent", modulePath)
	}
}

func TestFindModuleForPath_WithModule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	modContent := "module example.com/test\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(modContent), 0o644); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	root, modulePath, found, err := runner.FindModuleForPath(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !found {
		t.Fatal("expected module to be found")
	}

	if root != dir {
		t.Fatalf("expected root=%q, got %q", dir, root)
	}

	if modulePath != "example.com/test" {
		t.Fatalf("expected modulePath=%q, got %q", "example.com/test", modulePath)
	}
}

func TestFindOrCreateTargFile_CreatesNew(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	targFile, err := runner.FindOrCreateTargFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if targFile != filepath.Join(dir, "targs.go") {
		t.Errorf("expected targs.go, got %s", targFile)
	}

	content, err := os.ReadFile(targFile)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	if !strings.Contains(string(content), "//go:build targ") {
		t.Error("expected targ build tag in created file")
	}

	if !strings.Contains(string(content), "import \"github.com/toejough/targ\"") {
		t.Error("expected targ import in created file")
	}
}

func TestFindOrCreateTargFile_FindsExisting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create an existing targ file
	existingFile := filepath.Join(dir, "build.go")
	content := "//go:build targ\n\npackage build\n"

	err := os.WriteFile(existingFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	targFile, err := runner.FindOrCreateTargFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if targFile != existingFile {
		t.Errorf("expected %s, got %s", existingFile, targFile)
	}
}

func TestHasTargBuildTag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// File with targ build tag
	withTag := filepath.Join(dir, "with_tag.go")

	err := os.WriteFile(withTag, []byte("//go:build targ\n\npackage foo\n"), 0o644)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	// File without targ build tag
	withoutTag := filepath.Join(dir, "without_tag.go")

	err = os.WriteFile(withoutTag, []byte("package foo\n"), 0o644)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	// File with different build tag
	otherTag := filepath.Join(dir, "other_tag.go")

	err = os.WriteFile(otherTag, []byte("//go:build integration\n\npackage foo\n"), 0o644)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	if !runner.HasTargBuildTag(withTag) {
		t.Error("expected HasTargBuildTag to return true for file with targ tag")
	}

	if runner.HasTargBuildTag(withoutTag) {
		t.Error("expected HasTargBuildTag to return false for file without tag")
	}

	if runner.HasTargBuildTag(otherTag) {
		t.Error("expected HasTargBuildTag to return false for file with different tag")
	}
}

func TestIsValidTargetName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		valid bool
	}{
		{"lint", true},
		{"my-target", true},
		{"build123", true},
		{"a", true},
		{"", false},
		{"123", false},       // starts with number
		{"-lint", false},     // starts with hyphen
		{"lint-", false},     // ends with hyphen
		{"Lint", false},      // uppercase
		{"my_target", false}, // underscore
		{"my.target", false}, // dot
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := runner.IsValidTargetName(tt.name); got != tt.valid {
				t.Errorf("IsValidTargetName(%q) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}

func TestKebabToPascal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"lint", "Lint"},
		{"my-target", "MyTarget"},
		{"build-and-test", "BuildAndTest"},
		{"a", "A"},
		{"abc-def-ghi", "AbcDefGhi"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			if got := runner.KebabToPascal(tt.input); got != tt.expected {
				t.Errorf("KebabToPascal(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNamespacePaths_CompressesSegments(t *testing.T) {
	t.Parallel()

	files := []string{
		"/root/tools/issues/issues.go",
		"/root/tools/other/foo.go",
		"/root/tools/other/bar.go",
	}

	paths, err := runner.NamespacePaths(files, "/root")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expect := map[string][]string{
		"/root/tools/issues/issues.go": {"issues"},
		"/root/tools/other/foo.go":     {"other", "foo"},
		"/root/tools/other/bar.go":     {"other", "bar"},
	}
	for file, want := range expect {
		if got := paths[file]; !reflect.DeepEqual(got, want) {
			t.Fatalf("paths[%q] = %v, want %v", file, got, want)
		}
	}
}

func TestParseCreateArgs_Basic(t *testing.T) {
	t.Parallel()

	opts, err := runner.ParseCreateArgs([]string{"lint", "golangci-lint run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Name != "lint" {
		t.Errorf("expected name 'lint', got %q", opts.Name)
	}

	if opts.ShellCmd != "golangci-lint run" {
		t.Errorf("expected shell command 'golangci-lint run', got %q", opts.ShellCmd)
	}

	if len(opts.Path) != 0 {
		t.Errorf("expected empty path, got %v", opts.Path)
	}
}

func TestParseCreateArgs_FullOptions(t *testing.T) {
	t.Parallel()

	opts, err := runner.ParseCreateArgs([]string{
		"dev", "build",
		"--deps", "lint", "test",
		"--cache", "**/*.go",
		"go build ./...",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Name != "build" {
		t.Errorf("expected name 'build', got %q", opts.Name)
	}

	if !reflect.DeepEqual(opts.Path, []string{"dev"}) {
		t.Errorf("expected path [dev], got %v", opts.Path)
	}

	if !reflect.DeepEqual(opts.Deps, []string{"lint", "test"}) {
		t.Errorf("expected deps [lint test], got %v", opts.Deps)
	}

	if !reflect.DeepEqual(opts.Cache, []string{"**/*.go"}) {
		t.Errorf("expected cache [**/*.go], got %v", opts.Cache)
	}

	if opts.ShellCmd != "go build ./..." {
		t.Errorf("expected shell cmd 'go build ./...', got %q", opts.ShellCmd)
	}
}

func TestParseCreateArgs_TooFewArgs(t *testing.T) {
	t.Parallel()

	_, err := runner.ParseCreateArgs([]string{"lint"})
	if err == nil {
		t.Error("expected error for too few args")
	}
}

func TestParseCreateArgs_UnknownFlag(t *testing.T) {
	t.Parallel()

	_, err := runner.ParseCreateArgs([]string{"lint", "--unknown", "cmd"})
	if err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestParseCreateArgs_WithCache(t *testing.T) {
	t.Parallel()

	opts, err := runner.ParseCreateArgs(
		[]string{"build", "--cache", "**/*.go", "go.mod", "go build"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Name != "build" {
		t.Errorf("expected name 'build', got %q", opts.Name)
	}

	expectedCache := []string{"**/*.go", "go.mod"}
	if !reflect.DeepEqual(opts.Cache, expectedCache) {
		t.Errorf("expected cache %v, got %v", expectedCache, opts.Cache)
	}
}

func TestParseCreateArgs_WithDeps(t *testing.T) {
	t.Parallel()

	opts, err := runner.ParseCreateArgs([]string{"build", "--deps", "lint", "test", "go build"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Name != "build" {
		t.Errorf("expected name 'build', got %q", opts.Name)
	}

	expectedDeps := []string{"lint", "test"}
	if !reflect.DeepEqual(opts.Deps, expectedDeps) {
		t.Errorf("expected deps %v, got %v", expectedDeps, opts.Deps)
	}
}

func TestParseCreateArgs_WithPath(t *testing.T) {
	t.Parallel()

	opts, err := runner.ParseCreateArgs([]string{"dev", "lint", "fast", "golangci-lint run --fast"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Name != "fast" {
		t.Errorf("expected name 'fast', got %q", opts.Name)
	}

	if opts.ShellCmd != "golangci-lint run --fast" {
		t.Errorf("expected shell command 'golangci-lint run --fast', got %q", opts.ShellCmd)
	}

	expectedPath := []string{"dev", "lint"}
	if !reflect.DeepEqual(opts.Path, expectedPath) {
		t.Errorf("expected path %v, got %v", expectedPath, opts.Path)
	}
}

func TestParseHelpRequestIgnoresSubcommandHelp(t *testing.T) {
	t.Parallel()

	help, target := runner.ParseHelpRequest([]string{"issues", "--help"})
	if help && !target {
		t.Fatal("expected help to be scoped to target")
	}

	help, target = runner.ParseHelpRequest([]string{"--help"})
	if !help || target {
		t.Fatal("expected top-level help without target")
	}
}

func TestParseSyncArgs_Basic(t *testing.T) {
	t.Parallel()

	opts, err := runner.ParseSyncArgs([]string{"github.com/foo/bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.PackagePath != "github.com/foo/bar" {
		t.Errorf("expected package path 'github.com/foo/bar', got %q", opts.PackagePath)
	}
}

func TestParseSyncArgs_InvalidPath(t *testing.T) {
	t.Parallel()

	_, err := runner.ParseSyncArgs([]string{"invalid-path"})
	if err == nil {
		t.Error("expected error for invalid package path")
	}
}

func TestParseSyncArgs_NoArgs(t *testing.T) {
	t.Parallel()

	_, err := runner.ParseSyncArgs([]string{})
	if err == nil {
		t.Error("expected error for no args")
	}
}

func TestPathToPascal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     []string
		expected string
	}{
		{[]string{"lint"}, "Lint"},
		{[]string{"dev", "lint"}, "DevLint"},
		{[]string{"dev", "lint", "fast"}, "DevLintFast"},
		{[]string{"my-group", "my-target"}, "MyGroupMyTarget"},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.path, "/"), func(t *testing.T) {
			t.Parallel()

			got := runner.PathToPascal(tt.path)
			if got != tt.expected {
				t.Errorf("PathToPascal(%v) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestWriteBootstrapFileCleanup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	data := []byte("package main\n")

	path, cleanup, err := runner.WriteBootstrapFile(dir, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = os.Stat(path)
	if err != nil {
		t.Fatalf("expected bootstrap file to exist: %v", err)
	}

	err = cleanup()
	if err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}

	_, err = os.Stat(path)
	if !os.IsNotExist(err) {
		t.Fatalf("expected bootstrap file to be removed, got: %v", err)
	}
}

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"text/template"

	"github.com/toejough/targ/buildtool"
)

func TestBuildBootstrapData_Namespaces(t *testing.T) {
	infos, collapsedPaths := namespaceTestData()

	data, err := buildBootstrapData(infos, "/repo", "/repo", "example.com/proj", collapsedPaths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data.RootExprs) != 2 {
		t.Fatalf("expected 2 root exprs, got %v", data.RootExprs)
	}

	otherNode, ok := findNode(data.Nodes, "other")
	if !ok {
		t.Fatalf("expected other node, got %v", nodeNames(data.Nodes))
	}

	issuesNode, ok := findNode(data.Nodes, "issues")
	if !ok {
		t.Fatalf("expected issues node, got %v", nodeNames(data.Nodes))
	}

	if !hasField(otherNode.Fields, "Foo") || !hasField(otherNode.Fields, "Bar") {
		t.Fatalf(
			"expected other node to have Foo and Bar fields, got %v",
			fieldNames(otherNode.Fields),
		)
	}

	if !hasField(issuesNode.Fields, "List") {
		t.Fatalf("expected issues node to have List field, got %v", fieldNames(issuesNode.Fields))
	}
}

func TestBuildBootstrapData_RootCommands(t *testing.T) {
	infos := []buildtool.PackageInfo{
		{
			Dir:     "/repo/app",
			Package: "app",
			Files:   []buildtool.FileInfo{{Path: "/repo/app/tasks.go"}},
			Commands: []buildtool.CommandInfo{
				{Name: "Build", Kind: buildtool.CommandStruct, File: "/repo/app/tasks.go"},
				{Name: "Lint", Kind: buildtool.CommandFunc, File: "/repo/app/tasks.go"},
			},
		},
	}

	// Compute collapsed paths for the test files
	collapsedPaths, _ := namespacePaths([]string{"/repo/app/tasks.go"}, "/repo/app")

	data, err := buildBootstrapData(infos, "/repo/app", "/repo", "example.com/proj", collapsedPaths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.AllowDefault {
		t.Fatal("expected AllowDefault to be false for build-tool mode")
	}

	if len(data.Nodes) != 0 {
		t.Fatalf("expected no namespace nodes, got %d", len(data.Nodes))
	}

	if len(data.RootExprs) != 2 {
		t.Fatalf("expected 2 root exprs, got %v", data.RootExprs)
	}

	rendered := renderBootstrap(t, data)
	if !strings.Contains(rendered, "type AppLintFunc struct") {
		t.Fatalf("expected func wrapper in template, got:\n%s", rendered)
	}

	if !strings.Contains(rendered, "&AppLintFunc{}") {
		t.Fatalf("expected func wrapper in roots, got:\n%s", rendered)
	}
}

func TestCheckDestConflict_OnlyChecksNamespaces(t *testing.T) {
	// Scenario: user has dev/targets.go with Lint function
	// They run: targ --move lint "targets.lint*"
	// This should NOT conflict because "lint" is not a top-level namespace
	// (only "targets" and "issues" are top-level namespaces)
	infos := []buildtool.PackageInfo{
		{
			Dir:     "/repo/dev",
			Package: "dev",
			Files:   []buildtool.FileInfo{{Path: "/repo/dev/targets.go"}},
			Commands: []buildtool.CommandInfo{
				{Name: "Lint", Kind: buildtool.CommandFunc, File: "/repo/dev/targets.go"},
			},
		},
	}

	// "lint" should NOT conflict - it's not a top-level namespace
	err := checkDestConflict(infos, "lint")
	if err != nil {
		t.Fatalf("'lint' should not conflict, got: %v", err)
	}

	// "targets" SHOULD conflict - it's a top-level namespace
	err = checkDestConflict(infos, "targets")
	if err == nil {
		t.Fatal("'targets' should conflict with existing namespace")
	}
}

func TestCommandSummariesFromCommands(t *testing.T) {
	cmds := []buildtool.CommandInfo{
		{Name: "ListItems"},
		{Name: "DoWork"},
	}
	summaries := commandSummariesFromCommands(cmds)

	names := make([]string, 0, len(summaries))
	for _, cmd := range summaries {
		names = append(names, cmd.Name)
	}

	if got := strings.Join(names, ","); got != "do-work,list-items" {
		t.Fatalf("unexpected commands: %s", got)
	}
}

func TestEnsureFallbackModuleRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior is restricted on windows")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	root, err := ensureFallbackModuleRoot(
		dir,
		"targ.local",
		targDependency{ModulePath: "github.com/toejough/targ", Version: "v0.0.0"},
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

func TestFindModuleForPath_NoModule(t *testing.T) {
	dir := t.TempDir()

	root, modulePath, found, err := findModuleForPath(dir)
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
	root, modulePath, found, err := findModuleForPath(child)
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
	// Test that findModuleForPath works when given a file path
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
	root, modulePath, found, err := findModuleForPath(targetFile)
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
	dir := t.TempDir()

	modContent := "module example.com/test\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(modContent), 0o644); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	root, modulePath, found, err := findModuleForPath(dir)
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

func TestFindSourcePackageByNamespace(t *testing.T) {
	// Package is "dev" but namespace shown to user is "targets" (from filename)
	// User should be able to use "targets" in --move pattern
	infos := []buildtool.PackageInfo{
		{
			Dir:     "/repo/dev",
			Package: "dev",
			Files:   []buildtool.FileInfo{{Path: "/repo/dev/targets.go"}},
			Commands: []buildtool.CommandInfo{
				{Name: "Lint", Kind: buildtool.CommandFunc, File: "/repo/dev/targets.go"},
				{Name: "LintFast", Kind: buildtool.CommandFunc, File: "/repo/dev/targets.go"},
			},
		},
	}

	// Should find package by namespace "targets"
	pkg := findSourcePackageByName(infos, "targets", "/repo")
	if pkg == nil {
		t.Fatal("expected to find package by namespace 'targets'")
	}

	if pkg.Package != "dev" {
		t.Fatalf("expected package 'dev', got %q", pkg.Package)
	}

	// Should NOT match by Go package name - only namespace
	pkg = findSourcePackageByName(infos, "dev", "/repo")
	if pkg != nil {
		t.Fatal("should NOT find package by Go package name 'dev', only namespace 'targets'")
	}
}

func TestMoveCommands_GeneratesCode(t *testing.T) {
	// Create temp directory with a targets file
	dir := t.TempDir()
	targetsFile := filepath.Join(dir, "targets.go")

	initialContent := `//go:build targ

package dev

func Lint() error { return nil }
func LintFast() error { return nil }
`
	if err := os.WriteFile(targetsFile, []byte(initialContent), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Run moveCommands (this requires more setup, so we'll test generateMoveStruct directly)
	subcommands := []movedCommand{
		{info: buildtool.CommandInfo{Name: "LintFast", UsesContext: false}, newName: "fast"},
	}
	exactMatch := &buildtool.CommandInfo{Name: "Lint", UsesContext: false}

	code := generateMoveStruct("lint", exactMatch, subcommands, targetsFile)

	// Verify the generated code contains expected elements
	if !strings.Contains(code, "type Lint struct") {
		t.Errorf("expected 'type Lint struct' in generated code, got:\n%s", code)
	}

	if !strings.Contains(code, "func (c *Lint) Run() error") {
		t.Errorf("expected Run method in generated code, got:\n%s", code)
	}

	if !strings.Contains(code, "Fast *LintFastWrapper") {
		t.Errorf("expected Fast subcommand field in generated code, got:\n%s", code)
	}
}

func TestMoveCommands_RenamesOriginalFunctionsToUnexported(t *testing.T) {
	// Create temp directory with a targets file
	dir := t.TempDir()
	targetsFile := filepath.Join(dir, "targets.go")

	initialContent := `//go:build targ

package dev

import "context"

func Lint(ctx context.Context) error { return nil }
func LintFast(ctx context.Context) error { return nil }
func LintForFail(ctx context.Context) error { return nil }
`
	if err := os.WriteFile(targetsFile, []byte(initialContent), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Test that the rename functions work correctly
	// First verify our helper function works
	renamed := toUnexportedName("Lint")
	if renamed != "lint" {
		t.Errorf("expected 'lint', got %q", renamed)
	}

	renamed = toUnexportedName("LintFast")
	if renamed != "lintFast" {
		t.Errorf("expected 'lintFast', got %q", renamed)
	}

	// Test that generateMoveStruct calls unexported versions
	subcommands := []movedCommand{
		{info: buildtool.CommandInfo{Name: "LintFast", UsesContext: true}, newName: "fast"},
	}
	exactMatch := &buildtool.CommandInfo{Name: "Lint", UsesContext: true}

	code := generateMoveStruct("lint", exactMatch, subcommands, targetsFile)

	// The generated Run() method should call lint() (unexported), not Lint() (exported)
	if !strings.Contains(code, "return lint(ctx)") {
		t.Errorf("expected generated code to call 'lint(ctx)' (unexported), got:\n%s", code)
	}

	// The generated wrapper should call lintFast() (unexported), not LintFast()
	if !strings.Contains(code, "return lintFast(ctx)") {
		t.Errorf("expected generated code to call 'lintFast(ctx)' (unexported), got:\n%s", code)
	}
}

func TestNamespacePaths_CompressesSegments(t *testing.T) {
	files := []string{
		"/root/tools/issues/issues.go",
		"/root/tools/other/foo.go",
		"/root/tools/other/bar.go",
	}

	paths, err := namespacePaths(files, "/root")
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

func TestParseHelpRequestIgnoresSubcommandHelp(t *testing.T) {
	help, target := parseHelpRequest([]string{"issues", "--help"})
	if help && !target {
		t.Fatal("expected help to be scoped to target")
	}

	help, target = parseHelpRequest([]string{"--help"})
	if !help || target {
		t.Fatal("expected top-level help without target")
	}
}

func TestPrintBuildToolUsageIncludesSummary(t *testing.T) {
	var buf bytes.Buffer
	printBuildToolUsage(&buf)

	out := buf.String()
	if !strings.Contains(out, "build-tool runner") {
		t.Fatalf("expected summary in usage output, got: %s", out)
	}

	if strings.Contains(out, "More info:") {
		t.Fatalf("did not expect epilog in usage output, got: %s", out)
	}
}

func TestRenameFunction_UpdatesAllReferences(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")
	writeTestFile(t, filepath.Join(dir, "lint.go"), `package testmod

func Lint() error {
	return nil
}

func LintFast() error {
	return Lint() // calls Lint
}
`)
	writeTestFile(t, filepath.Join(dir, "check.go"), `package testmod

func Check() error {
	if err := Lint(); err != nil {
		return err
	}
	return LintFast()
}
`)

	err := renameFunction(dir, "testmod", "Lint", "lint")
	if err != nil {
		t.Fatalf("renameFunction failed: %v", err)
	}

	lintContent := readTestFile(t, filepath.Join(dir, "lint.go"))
	checkContent := readTestFile(t, filepath.Join(dir, "check.go"))

	// Verify definition and call sites were renamed
	if !strings.Contains(lintContent, "func lint()") {
		t.Errorf("expected 'func lint()' in lint.go, got:\n%s", lintContent)
	}

	if !strings.Contains(lintContent, "return lint()") {
		t.Errorf("expected 'return lint()' in lint.go, got:\n%s", lintContent)
	}

	if !strings.Contains(checkContent, "if err := lint();") {
		t.Errorf("expected 'lint()' call in check.go, got:\n%s", checkContent)
	}

	// Verify LintFast was NOT renamed
	if !strings.Contains(lintContent, "func LintFast()") {
		t.Errorf("LintFast should NOT be renamed, got:\n%s", lintContent)
	}
}

func TestRenameFunction_WithTargBuildTag(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")
	// File with targ build constraint - simulates real targ target files
	writeTestFile(t, filepath.Join(dir, "targets.go"), `//go:build targ

package testmod

func Lint() error {
	return nil
}
`)
	// Regular file that calls the targ-tagged function
	writeTestFile(t, filepath.Join(dir, "caller.go"), `//go:build targ

package testmod

func Check() error {
	return Lint()
}
`)

	err := renameFunction(dir, "testmod", "Lint", "lint")
	if err != nil {
		t.Fatalf("renameFunction failed: %v", err)
	}

	targetsContent := readTestFile(t, filepath.Join(dir, "targets.go"))
	callerContent := readTestFile(t, filepath.Join(dir, "caller.go"))

	// Verify function was renamed in declaration
	if !strings.Contains(targetsContent, "func lint()") {
		t.Errorf("expected 'func lint()' in targets.go, got:\n%s", targetsContent)
	}

	// Verify call site was updated
	if !strings.Contains(callerContent, "return lint()") {
		t.Errorf("expected 'return lint()' in caller.go, got:\n%s", callerContent)
	}
}

func TestRenameFunctionsToUnexported(t *testing.T) {
	// Create temp file with exported functions
	dir := t.TempDir()
	targetsFile := filepath.Join(dir, "targets.go")

	initialContent := `//go:build targ

package dev

import "context"

func Lint(ctx context.Context) error { return nil }
func LintFast(ctx context.Context) error { return nil }
func Other() error { return nil }
`
	if err := os.WriteFile(targetsFile, []byte(initialContent), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Rename Lint and LintFast (but not Other)
	namesToRename := []string{"Lint", "LintFast"}

	err := renameFunctionsToUnexported(targetsFile, namesToRename)
	if err != nil {
		t.Fatalf("renameFunctionsToUnexported failed: %v", err)
	}

	// Read the file and verify
	content, err := os.ReadFile(targetsFile)
	if err != nil {
		t.Fatalf("failed to read modified file: %v", err)
	}

	contentStr := string(content)

	// Should have unexported versions
	if !strings.Contains(contentStr, "func lint(ctx context.Context)") {
		t.Errorf("expected 'func lint(' in modified file, got:\n%s", contentStr)
	}

	if !strings.Contains(contentStr, "func lintFast(ctx context.Context)") {
		t.Errorf("expected 'func lintFast(' in modified file, got:\n%s", contentStr)
	}

	// Other should remain exported (not in the rename list)
	if !strings.Contains(contentStr, "func Other()") {
		t.Errorf("expected 'func Other()' to remain unchanged, got:\n%s", contentStr)
	}

	// Should NOT have the original exported versions
	if strings.Contains(contentStr, "func Lint(ctx") {
		t.Errorf("should NOT have 'func Lint(' (should be renamed), got:\n%s", contentStr)
	}

	if strings.Contains(contentStr, "func LintFast(ctx") {
		t.Errorf("should NOT have 'func LintFast(' (should be renamed), got:\n%s", contentStr)
	}
}

func TestWriteBootstrapFileCleanup(t *testing.T) {
	dir := t.TempDir()
	data := []byte("package main\n")

	path, cleanup, err := writeBootstrapFile(dir, data, false)
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

	path, cleanup, err = writeBootstrapFile(dir, data, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = cleanup()
	if err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}

	_, err = os.Stat(path)
	if err != nil {
		t.Fatalf("expected bootstrap file to remain: %v", err)
	}

	_ = os.Remove(path)
}

func fieldNames(fields []bootstrapField) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}

	return names
}

func findNode(nodes []bootstrapNode, name string) (bootstrapNode, bool) {
	for _, node := range nodes {
		if node.Name == name {
			return node, true
		}
	}

	return bootstrapNode{}, false
}

func hasField(fields []bootstrapField, name string) bool {
	for _, field := range fields {
		if field.Name == name {
			return true
		}
	}

	return false
}

func namespaceTestData() ([]buildtool.PackageInfo, map[string][]string) {
	infos := []buildtool.PackageInfo{
		{
			Dir:     "/repo/tools/issues",
			Package: "issues",
			Files:   []buildtool.FileInfo{{Path: "/repo/tools/issues/issues.go"}},
			Commands: []buildtool.CommandInfo{
				{Name: "List", Kind: buildtool.CommandStruct, File: "/repo/tools/issues/issues.go"},
			},
		},
		{
			Dir:     "/repo/tools/other",
			Package: "other",
			Files: []buildtool.FileInfo{
				{Path: "/repo/tools/other/foo.go"},
				{Path: "/repo/tools/other/bar.go"},
			},
			Commands: []buildtool.CommandInfo{
				{Name: "Thing", Kind: buildtool.CommandStruct, File: "/repo/tools/other/foo.go"},
				{Name: "Ship", Kind: buildtool.CommandStruct, File: "/repo/tools/other/bar.go"},
			},
		},
	}

	var filePaths []string

	for _, info := range infos {
		for _, cmd := range info.Commands {
			filePaths = append(filePaths, cmd.File)
		}
	}

	collapsedPaths, _ := namespacePaths(filePaths, "/repo")

	return infos, collapsedPaths
}

func nodeNames(nodes []bootstrapNode) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}

	return names
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	return string(content)
}

func renderBootstrap(t *testing.T, data bootstrapData) string {
	t.Helper()

	tmpl := template.Must(template.New("main").Parse(bootstrapTemplate))

	var buf bytes.Buffer

	err := tmpl.Execute(&buf, data)
	if err != nil {
		t.Fatalf("unexpected template error: %v", err)
	}

	return buf.String()
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

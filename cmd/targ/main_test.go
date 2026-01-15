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

	// Compute collapsed paths for the test files
	var filePaths []string

	for _, info := range infos {
		for _, cmd := range info.Commands {
			filePaths = append(filePaths, cmd.File)
		}
	}

	collapsedPaths, _ := namespacePaths(filePaths, "/repo")

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

func TestCommandSummariesFromCommands(t *testing.T) {
	cmds := []buildtool.CommandInfo{
		{Name: "ListItems"},
		{Name: "DoWork"},
	}
	summaries := commandSummariesFromCommands(cmds)

	var names []string
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

	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
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

func TestWriteBootstrapFileCleanup(t *testing.T) {
	dir := t.TempDir()
	data := []byte("package main\n")

	path, cleanup, err := writeBootstrapFile(dir, data, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected bootstrap file to exist: %v", err)
	}

	if err := cleanup(); err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected bootstrap file to be removed, got: %v", err)
	}

	path, cleanup, err = writeBootstrapFile(dir, data, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cleanup(); err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
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

func nodeNames(nodes []bootstrapNode) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}

	return names
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

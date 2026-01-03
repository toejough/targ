package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"text/template"

	"targs/buildtool"
)

func TestBuildBootstrapData_SinglePackage_Local(t *testing.T) {
	infos := []buildtool.PackageInfo{
		{
			Dir:     "/repo/app",
			Package: "app",
			Structs: []string{"Build"},
			Funcs:   []string{"Lint"},
		},
	}

	data, err := buildBootstrapData(infos, "/repo/app", "/repo", "example.com/proj", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.MultiPackage {
		t.Fatal("expected multipackage to be false")
	}
	if data.UsePackageWrapper {
		t.Fatal("expected no package wrapper for single package")
	}
	if data.AllowDefault {
		t.Fatal("expected AllowDefault to be false for build-tool mode")
	}
	if len(data.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %v", data.Targets)
	}
	if data.BannerLit == "" {
		t.Fatal("expected banner for single package")
	}
	if len(data.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(data.Packages))
	}
	pkg := data.Packages[0]
	if !pkg.Local || pkg.ImportPath != "" || pkg.ImportName != "" {
		t.Fatalf("expected local package with no import, got %+v", pkg)
	}
	if pkg.TypeName != "App" || pkg.VarName != "app" {
		t.Fatalf("unexpected type or var name: %+v", pkg)
	}
}

func TestBuildBootstrapData_MultiPackage_Remote(t *testing.T) {
	infos := []buildtool.PackageInfo{
		{
			Dir:     "/repo/pkg1",
			Package: "alpha",
			Structs: []string{"Build"},
			Funcs:   []string{"Lint"},
		},
		{
			Dir:     "/repo/pkg2",
			Package: "beta",
			Structs: []string{"Ship"},
		},
	}

	data, err := buildBootstrapData(infos, "/repo/app", "/repo", "example.com/proj", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !data.MultiPackage {
		t.Fatal("expected multipackage to be true")
	}
	if !data.UsePackageWrapper {
		t.Fatal("expected package wrapper for package grouping")
	}
	if len(data.Targets) != 0 {
		t.Fatalf("expected no targets when package grouping is enabled, got %v", data.Targets)
	}
	if !hasImport(data.Imports, "example.com/proj/pkg1", "alpha") {
		t.Fatalf("expected import for pkg1, got %v", data.Imports)
	}
	if !hasImport(data.Imports, "example.com/proj/pkg2", "beta") {
		t.Fatalf("expected import for pkg2, got %v", data.Imports)
	}

	first := data.Packages[0]
	if first.ImportName != "alpha" {
		t.Fatalf("expected import alias alpha, got %q", first.ImportName)
	}
	if len(first.Commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(first.Commands))
	}
	if first.Commands[0].ValueExpr != "&alpha.Build{}" {
		t.Fatalf("unexpected struct value expr: %s", first.Commands[0].ValueExpr)
	}
	if first.Commands[1].ValueExpr != "alpha.Lint" {
		t.Fatalf("unexpected func value expr: %s", first.Commands[1].ValueExpr)
	}
}

func TestBootstrapTemplate_SinglePackage(t *testing.T) {
	infos := []buildtool.PackageInfo{
		{
			Dir:     "/repo/app",
			Package: "app",
			Structs: []string{"Build"},
			Funcs:   []string{"Lint"},
		},
	}
	data, err := buildBootstrapData(infos, "/repo/app", "/repo", "example.com/proj", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rendered := renderBootstrap(t, data)
	if strings.Contains(rendered, "type App struct") {
		t.Fatalf("did not expect package wrapper type in template, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "RunWithOptions(targs.RunOptions{AllowDefault: false}") {
		t.Fatalf("expected RunWithOptions in template, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Loaded tasks from package") {
		t.Fatalf("expected banner in template, got:\n%s", rendered)
	}
}

func TestBootstrapTemplate_MultiPackage(t *testing.T) {
	infos := []buildtool.PackageInfo{
		{
			Dir:     "/repo/pkg1",
			Package: "alpha",
			Structs: []string{"Build"},
		},
	}
	data, err := buildBootstrapData(infos, "/repo/app", "/repo", "example.com/proj", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rendered := renderBootstrap(t, data)
	if !strings.Contains(rendered, "type Alpha struct") {
		t.Fatalf("expected package wrapper type in template, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "`targs:\"subcommand\"`") {
		t.Fatalf("expected subcommand tag in template, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "RunWithOptions(targs.RunOptions{AllowDefault: false}") {
		t.Fatalf("expected RunWithOptions in template, got:\n%s", rendered)
	}
}

func renderBootstrap(t *testing.T, data bootstrapData) string {
	t.Helper()
	tmpl := template.Must(template.New("main").Parse(bootstrapTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("unexpected template error: %v", err)
	}
	return buf.String()
}

func hasImport(imports []bootstrapImport, path string, alias string) bool {
	for _, imp := range imports {
		if imp.Path == path && imp.Alias == alias {
			return true
		}
	}
	return false
}

func TestBuildBootstrapData_DuplicatePackageName(t *testing.T) {
	infos := []buildtool.PackageInfo{
		{
			Dir:     "/repo/pkg1",
			Package: "alpha",
			Structs: []string{"Build"},
		},
		{
			Dir:     "/repo/pkg2",
			Package: "alpha",
			Structs: []string{"Ship"},
		},
	}

	_, err := buildBootstrapData(infos, "/repo/app", "/repo", "example.com/proj", true)
	if err == nil {
		t.Fatal("expected duplicate package name error")
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

func TestPrintBuildToolUsageIncludesSummaryAndEpilog(t *testing.T) {
	var buf bytes.Buffer
	printBuildToolUsage(&buf)
	out := buf.String()
	if !strings.Contains(out, "build-tool runner") {
		t.Fatalf("expected summary in usage output, got: %s", out)
	}
	if !strings.Contains(out, "More info:") {
		t.Fatalf("expected epilog in usage output, got: %s", out)
	}
	if !strings.Contains(out, "github.com/toejough/targs") {
		t.Fatalf("expected README link, got: %s", out)
	}
}

func TestSummarizePackagesFormatsCommands(t *testing.T) {
	infos := []buildtool.PackageInfo{
		{
			Dir:     "/repo/tools/issues",
			Package: "issues",
			Doc:     "Issue tools.",
			Structs: []string{"ListItems"},
			Funcs:   []string{"DoWork"},
		},
	}
	summaries := summarizePackages(infos, "/repo")
	if len(summaries) != 1 {
		t.Fatalf("expected one summary, got %d", len(summaries))
	}
	summary := summaries[0]
	if summary.Path != "tools/issues" {
		t.Fatalf("unexpected path: %s", summary.Path)
	}
	if summary.Doc != "Issue tools." {
		t.Fatalf("unexpected doc: %s", summary.Doc)
	}
	if got := strings.Join(summary.Commands, ","); got != "do-work,list-items" {
		t.Fatalf("unexpected commands: %s", got)
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

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"commander/buildtool"
)

type bootstrapCommand struct {
	Name      string
	TypeExpr  string
	ValueExpr string
}

type bootstrapPackage struct {
	Name       string
	ImportPath string
	ImportName string
	Local      bool
	TypeName   string
	VarName    string
	Commands   []bootstrapCommand
	DescLit    string
}

type bootstrapImport struct {
	Path  string
	Alias string
}

type bootstrapData struct {
	PackageGrouping bool
	Imports         []bootstrapImport
	Packages        []bootstrapPackage
	Targets         []string
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "gen" {
		if err := runGenerate(); err != nil {
			fmt.Printf("Error generating wrappers: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var packageGrouping bool

	fs := flag.NewFlagSet("commander", flag.ContinueOnError)
	fs.BoolVar(&packageGrouping, "package", false, "group commands under package name (recursive package-scoped discovery)")
	fs.BoolVar(&packageGrouping, "p", false, "alias for --package")
	fs.SetOutput(os.Stdout)
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}
	args := fs.Args()

	startDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error resolving working directory: %v\n", err)
		os.Exit(1)
	}

	taggedDirs, err := buildtool.SelectTaggedDirs(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:        startDir,
		PackageGrouping: packageGrouping,
		BuildTag:        "commander",
	})
	if err != nil {
		fmt.Printf("Error discovering commands: %v\n", err)
		os.Exit(1)
	}

	for _, dir := range taggedDirs {
		if _, err := buildtool.GenerateFunctionWrappers(buildtool.OSFileSystem{}, buildtool.GenerateOptions{
			Dir:        dir.Path,
			BuildTag:   "commander",
			OnlyTagged: true,
		}); err != nil {
			fmt.Printf("Error generating command wrappers: %v\n", err)
			os.Exit(1)
		}
	}

	infos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:        startDir,
		PackageGrouping: packageGrouping,
		BuildTag:        "commander",
	})
	if err != nil {
		fmt.Printf("Error discovering commands: %v\n", err)
		os.Exit(1)
	}

	moduleRoot, modulePath, err := findModuleRootAndPath(startDir)
	if err != nil {
		fmt.Printf("Error finding module root: %v\n", err)
		os.Exit(1)
	}

	data, err := buildBootstrapData(infos, startDir, moduleRoot, modulePath, packageGrouping)
	if err != nil {
		fmt.Printf("Error preparing bootstrap: %v\n", err)
		os.Exit(1)
	}

	tmpl := template.Must(template.New("main").Parse(bootstrapTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		fmt.Printf("Error generating code: %v\n", err)
		os.Exit(1)
	}

	tempDir := filepath.Join(moduleRoot, ".commander", "tmp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		fmt.Printf("Error creating bootstrap dir: %v\n", err)
		os.Exit(1)
	}
	tempFile, err := os.CreateTemp(tempDir, "bootstrap-*.go")
	if err != nil {
		fmt.Printf("Error creating bootstrap file: %v\n", err)
		os.Exit(1)
	}
	filename := tempFile.Name()
	_ = tempFile.Close()
	if err := os.WriteFile(filename, buf.Bytes(), 0644); err != nil {
		fmt.Printf("Error writing bootstrap file: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(filename)

	runArgs := []string{"run", "-tags", "commander", filename}
	runArgs = append(runArgs, args...)

	cmd := exec.Command("go", runArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func runGenerate() error {
	var buildTag string
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	fs.StringVar(&buildTag, "tag", "", "optional build tag for generated wrappers")
	fs.SetOutput(os.Stdout)
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error resolving working directory: %w", err)
	}

	_, err = buildtool.GenerateFunctionWrappers(buildtool.OSFileSystem{}, buildtool.GenerateOptions{
		Dir:        dir,
		BuildTag:   buildTag,
		OnlyTagged: false,
	})
	return err
}

func findModuleRootAndPath(startDir string) (string, string, error) {
	dir := startDir
	for {
		modPath := filepath.Join(dir, "go.mod")
		data, err := os.ReadFile(modPath)
		if err == nil {
			modulePath := parseModulePath(string(data))
			if modulePath == "" {
				return "", "", fmt.Errorf("module path not found in %s", modPath)
			}
			return dir, modulePath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", "", fmt.Errorf("go.mod not found from %s", startDir)
}

func parseModulePath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func buildBootstrapData(
	infos []buildtool.PackageInfo,
	startDir string,
	moduleRoot string,
	modulePath string,
	packageGrouping bool,
) (bootstrapData, error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return bootstrapData{}, err
	}
	imports := []bootstrapImport{{Path: "commander"}}
	usedImports := map[string]bool{"commander": true}
	var packages []bootstrapPackage
	var targets []string
	seenPackages := make(map[string]string)

	for _, info := range infos {
		if len(info.Structs) == 0 && len(info.Funcs) == 0 {
			return bootstrapData{}, fmt.Errorf("no commands found in package %s", info.Package)
		}
		if existing, ok := seenPackages[info.Package]; ok {
			return bootstrapData{}, fmt.Errorf("duplicate package name %q in %s and %s", info.Package, existing, info.Dir)
		}
		seenPackages[info.Package] = info.Dir

		local := sameDir(absStart, info.Dir)
		importPath := ""
		importName := ""
		prefix := ""
		if !local {
			rel, err := filepath.Rel(moduleRoot, info.Dir)
			if err != nil {
				return bootstrapData{}, err
			}
			importPath = modulePath
			if rel != "." {
				importPath = modulePath + "/" + filepath.ToSlash(rel)
			}
			importName = uniqueImportName(info.Package, usedImports)
			prefix = importName + "."
			imports = append(imports, bootstrapImport{
				Path:  importPath,
				Alias: importName,
			})
		}

		var commands []bootstrapCommand
		for _, name := range info.Structs {
			commands = append(commands, bootstrapCommand{
				Name:      name,
				TypeExpr:  "*" + prefix + name,
				ValueExpr: "&" + prefix + name + "{}",
			})
		}
		for _, name := range info.Funcs {
			commands = append(commands, bootstrapCommand{
				Name:      name,
				TypeExpr:  "func()",
				ValueExpr: prefix + name,
			})
		}

		pkg := bootstrapPackage{
			Name:       info.Package,
			ImportPath: importPath,
			ImportName: importName,
			Local:      local,
			TypeName:   exportTypeName(info.Package),
			VarName:    lowerFirst(exportTypeName(info.Package)),
			Commands:   commands,
			DescLit:    strconv.Quote(packageDescription(info.Doc, info.Dir)),
		}
		packages = append(packages, pkg)

		if !packageGrouping {
			for _, cmd := range commands {
				targets = append(targets, cmd.ValueExpr)
			}
		}
	}

	return bootstrapData{
		PackageGrouping: packageGrouping,
		Imports:         imports,
		Packages:        packages,
		Targets:         targets,
	}, nil
}

func sameDir(a string, b string) bool {
	absA, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		return false
	}
	return absA == absB
}

func uniqueImportName(name string, used map[string]bool) string {
	candidate := name
	if candidate == "" {
		candidate = "pkg"
	}
	if candidate == "commander" {
		candidate = "cmdpkg"
	}
	for used[candidate] {
		candidate += "pkg"
	}
	used[candidate] = true
	return candidate
}

func exportTypeName(name string) string {
	if name == "" {
		return "Package"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func lowerFirst(name string) string {
	if name == "" {
		return "pkg"
	}
	return strings.ToLower(name[:1]) + name[1:]
}

func packageDescription(doc string, dir string) string {
	doc = strings.TrimSpace(doc)
	pathLine := fmt.Sprintf("Path: %s", dir)
	if doc == "" {
		return pathLine
	}
	return doc + "\n" + pathLine
}

const bootstrapTemplate = `
package main

import (
	"commander"
{{- range .Imports }}
{{- if and (ne .Path "commander") (ne .Alias "") }}
	{{ .Alias }} "{{ .Path }}"
{{- else if ne .Path "commander" }}
	"{{ .Path }}"
{{- end }}
{{- end }}
)

{{- if .PackageGrouping }}
{{- range .Packages }}
type {{ .TypeName }} struct {
{{- range .Commands }}
	{{ .Name }} {{ .TypeExpr }} ` + "`commander:\"subcommand\"`" + `
{{- end }}
}

func (p *{{ .TypeName }}) Description() string {
	return {{ .DescLit }}
}
{{- end }}
{{- end }}

func main() {
{{- if .PackageGrouping }}
{{- range .Packages }}
	{{ .VarName }} := &{{ .TypeName }}{
{{- range .Commands }}
		{{ .Name }}: {{ .ValueExpr }},
{{- end }}
	}
{{- end }}

	roots := []interface{}{
{{- range .Packages }}
		{{ .VarName }},
{{- end }}
	}

	commander.RunWithOptions(commander.RunOptions{AllowDefault: false}, roots...)
{{- else }}
	cmds := []interface{}{
{{- range .Targets }}
		{{ . }},
{{- end }}
	}
	commander.RunWithOptions(commander.RunOptions{AllowDefault: false}, cmds...)
{{- end }}
}
`

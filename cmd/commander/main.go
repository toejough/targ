package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf8"

	"commander/buildtool"
)

type bootstrapCommand struct {
	Name      string
	TypeExpr  string
	ValueExpr string
	IsFunc    bool
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
	MultiPackage      bool
	UsePackageWrapper bool
	AllowDefault      bool
	BannerLit         string
	Imports           []bootstrapImport
	Packages          []bootstrapPackage
	Targets           []string
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "gen" {
		if err := runGenerate(); err != nil {
			fmt.Printf("Error generating wrappers: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var multiPackage bool
	var noCache bool

	fs := flag.NewFlagSet("commander", flag.ContinueOnError)
	fs.BoolVar(&multiPackage, "multipackage", false, "enable multipackage mode (recursive package-scoped discovery)")
	fs.BoolVar(&multiPackage, "m", false, "alias for --multipackage")
	fs.BoolVar(&noCache, "no-cache", false, "disable cached build tool binaries")
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "Usage: commander [--multipackage|-m] [--no-cache] [args]")
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintln(os.Stdout, "Flags:")
		fmt.Fprintln(os.Stdout, "  --multipackage, -m    enable multipackage mode (recursive package-scoped discovery)")
		fmt.Fprintln(os.Stdout, "  --no-cache            disable cached build tool binaries")
		fmt.Fprintln(os.Stdout, "  --completion [bash|zsh|fish]")
	}
	fs.SetOutput(os.Stdout)
	parseArgs := make([]string, 0, len(os.Args[1:]))
	for _, arg := range os.Args[1:] {
		if arg == "--multipackage" {
			parseArgs = append(parseArgs, "-multipackage")
			continue
		}
		parseArgs = append(parseArgs, arg)
	}
	if err := fs.Parse(parseArgs); err != nil {
		os.Exit(1)
	}
	args := fs.Args()

	startDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error resolving working directory: %v\n", err)
		os.Exit(1)
	}

	taggedDirs, err := buildtool.SelectTaggedDirs(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:     startDir,
		MultiPackage: multiPackage,
		BuildTag:     "commander",
	})
	if err != nil {
		var multiErr *buildtool.MultipleTaggedDirsError
		if errors.As(err, &multiErr) {
			if err := printMultiPackageError(startDir, multiErr); err == nil {
				os.Exit(1)
			}
		}
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
		StartDir:     startDir,
		MultiPackage: multiPackage,
		BuildTag:     "commander",
	})
	if err != nil {
		var multiErr *buildtool.MultipleTaggedDirsError
		if errors.As(err, &multiErr) {
			if err := printMultiPackageError(startDir, multiErr); err == nil {
				os.Exit(1)
			}
		}
		fmt.Printf("Error discovering commands: %v\n", err)
		os.Exit(1)
	}

	moduleRoot, modulePath, err := findModuleRootAndPath(startDir)
	if err != nil {
		fmt.Printf("Error finding module root: %v\n", err)
		os.Exit(1)
	}

	data, err := buildBootstrapData(infos, startDir, moduleRoot, modulePath, multiPackage)
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

	taggedFiles, err := buildtool.TaggedFiles(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:     startDir,
		MultiPackage: multiPackage,
		BuildTag:     "commander",
	})
	if err != nil {
		fmt.Printf("Error gathering tagged files: %v\n", err)
		os.Exit(1)
	}
	cacheKey, err := computeCacheKey(modulePath, moduleRoot, "commander", buf.Bytes(), taggedFiles)
	if err != nil {
		fmt.Printf("Error computing cache key: %v\n", err)
		os.Exit(1)
	}

	tempFile := filepath.Join(tempDir, fmt.Sprintf("commander_bootstrap_%s.go", cacheKey))
	if err := os.WriteFile(tempFile, buf.Bytes(), 0644); err != nil {
		fmt.Printf("Error writing bootstrap file: %v\n", err)
		os.Exit(1)
	}

	cacheDir := filepath.Join(moduleRoot, ".commander", "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Printf("Error creating cache directory: %v\n", err)
		os.Exit(1)
	}
	binaryPath := filepath.Join(cacheDir, fmt.Sprintf("commander_%s", cacheKey))

	if !noCache {
		if info, err := os.Stat(binaryPath); err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			cmd := exec.Command(binaryPath, args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
					os.Exit(exitErr.ExitCode())
				}
				fmt.Printf("Error running command: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	buildArgs := []string{"build", "-tags", "commander", "-o", binaryPath, tempFile}
	buildCmd := exec.Command("go", buildArgs...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Printf("Error building command: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(binaryPath, args...)
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

func printMultiPackageError(startDir string, multiErr *buildtool.MultipleTaggedDirsError) error {
	infos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:     startDir,
		MultiPackage: true,
		BuildTag:     "commander",
	})
	if err != nil {
		return err
	}
	byPath := make(map[string]buildtool.PackageInfo, len(infos))
	for _, info := range infos {
		byPath[info.Dir] = info
	}

	paths := append([]string(nil), multiErr.Paths...)
	sort.Strings(paths)

	fmt.Println("Error: multiple packages found. Please run again from a directory tree that only includes one,")
	fmt.Println("  or run again with `--multipackage`")
	fmt.Println("")
	for _, path := range paths {
		pkg := "<unknown>"
		if info, ok := byPath[path]; ok {
			pkg = info.Package
		}
		fmt.Printf("  tasks found in package %q at %q\n", pkg, path)
	}
	fmt.Println("")
	fmt.Println("For more information, run `commander --help`.")
	return nil
}

func buildBootstrapData(
	infos []buildtool.PackageInfo,
	startDir string,
	moduleRoot string,
	modulePath string,
	multiPackage bool,
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
	usePackageWrapper := multiPackage

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
				IsFunc:    false,
			})
		}
		for _, name := range info.Funcs {
			commands = append(commands, bootstrapCommand{
				Name:      name,
				TypeExpr:  "func()",
				ValueExpr: prefix + name,
				IsFunc:    true,
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

		if !usePackageWrapper {
			for _, cmd := range commands {
				targets = append(targets, cmd.ValueExpr)
			}
		}
	}

	allowDefault := false
	bannerLit := ""
	if !multiPackage && len(infos) == 1 {
		bannerLit = strconv.Quote(singlePackageBanner(infos[0]))
	}
	return bootstrapData{
		MultiPackage:      multiPackage,
		UsePackageWrapper: usePackageWrapper,
		AllowDefault:      allowDefault,
		BannerLit:         bannerLit,
		Imports:           imports,
		Packages:          packages,
		Targets:           targets,
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

func singlePackageBanner(info buildtool.PackageInfo) string {
	lines := []string{
		fmt.Sprintf("Loaded tasks from package %q.", info.Package),
	}
	doc := strings.TrimSpace(info.Doc)
	if doc != "" {
		lines = append(lines, doc)
	}
	lines = append(lines, fmt.Sprintf("Path: %s", info.Dir))
	return strings.Join(lines, "\n")
}

func computeCacheKey(modulePath string, moduleRoot string, buildTag string, bootstrap []byte, tagged []buildtool.TaggedFile) (string, error) {
	hasher := sha256.New()
	write := func(value string) {
		hasher.Write([]byte(value))
		hasher.Write([]byte{0})
	}
	write("module:" + modulePath)
	write("root:" + moduleRoot)
	write("tag:" + buildTag)
	write("bootstrap:")
	hasher.Write(bootstrap)
	hasher.Write([]byte{0})

	sort.Slice(tagged, func(i, j int) bool {
		return tagged[i].Path < tagged[j].Path
	})
	for _, file := range tagged {
		if !utf8.ValidString(file.Path) {
			return "", fmt.Errorf("invalid utf-8 path in tagged file: %q", file.Path)
		}
		write("file:" + file.Path)
		hasher.Write(file.Content)
		hasher.Write([]byte{0})
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

const bootstrapTemplate = `
package main

import (
	"commander"
{{- if .BannerLit }}
	"fmt"
	"os"
{{- end }}
{{- range .Imports }}
{{- if and (ne .Path "commander") (ne .Alias "") }}
	{{ .Alias }} "{{ .Path }}"
{{- else if ne .Path "commander" }}
	"{{ .Path }}"
{{- end }}
{{- end }}
)

{{- if .UsePackageWrapper }}
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
{{- if .BannerLit }}
	if len(os.Args) == 1 || (len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help")) {
		fmt.Println({{ .BannerLit }})
		fmt.Println()
	}
{{- end }}
{{- if .UsePackageWrapper }}
{{- range .Packages }}
	{{ .VarName }} := &{{ .TypeName }}{
{{- range .Commands }}
{{- if .IsFunc }}
		{{ .Name }}: {{ .ValueExpr }},
{{- end }}
{{- end }}
	}
{{- end }}

	roots := []interface{}{
{{- range .Packages }}
		{{ .VarName }},
{{- end }}
	}

	commander.RunWithOptions(commander.RunOptions{AllowDefault: {{ .AllowDefault }}}, roots...)
{{- else }}
	cmds := []interface{}{
{{- range .Targets }}
		{{ . }},
{{- end }}
	}
	commander.RunWithOptions(commander.RunOptions{AllowDefault: {{ .AllowDefault }}}, cmds...)
{{- end }}
}
`

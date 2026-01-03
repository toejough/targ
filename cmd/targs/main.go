package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf8"

	"targs"
	"targs/buildtool"
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
	var keepBootstrap bool
	var completionShell string

	fs := flag.NewFlagSet("targs", flag.ContinueOnError)
	fs.BoolVar(&multiPackage, "multipackage", false, "enable multipackage mode (recursive package-scoped discovery)")
	fs.BoolVar(&multiPackage, "m", false, "alias for --multipackage")
	fs.BoolVar(&noCache, "no-cache", false, "disable cached build tool binaries")
	fs.BoolVar(&keepBootstrap, "keep", false, "keep generated bootstrap file")
	fs.StringVar(&completionShell, "completion", "", "print shell completion (bash|zsh|fish)")
	fs.Usage = func() {
		printBuildToolUsage(os.Stdout)
	}
	fs.SetOutput(io.Discard)
	rawArgs := os.Args[1:]
	helpRequested, helpTargets := parseHelpRequest(rawArgs)
	parseArgs := make([]string, 0, len(rawArgs))
	completionRequested := false
	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if arg == "--multipackage" {
			parseArgs = append(parseArgs, "-multipackage")
			continue
		}
		if arg == "--completion" {
			completionRequested = true
			shell := ""
			if i+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[i+1], "-") {
				shell = rawArgs[i+1]
				i++
			}
			parseArgs = append(parseArgs, "-completion="+shell)
			continue
		}
		if strings.HasPrefix(arg, "--completion=") {
			completionRequested = true
		}
		parseArgs = append(parseArgs, arg)
	}
	if err := fs.Parse(parseArgs); err != nil {
		if err == flag.ErrHelp {
			helpRequested = true
		} else {
			fmt.Fprintln(os.Stderr, err)
			printBuildToolUsage(os.Stderr)
			os.Exit(1)
		}
	}
	args := fs.Args()

	if helpRequested && !helpTargets {
		startDir, err := os.Getwd()
		if err != nil {
			fmt.Printf("Error resolving working directory: %v\n", err)
			os.Exit(1)
		}
		if err := printBuildToolHelp(os.Stdout, startDir, multiPackage); err != nil {
			fmt.Printf("Error discovering packages: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if completionRequested {
		if completionShell == "" {
			completionShell = detectShell()
		}
		if completionShell == "" {
			fmt.Fprintln(os.Stderr, "Usage: --completion [bash|zsh|fish]")
			os.Exit(1)
		}
		binName := os.Args[0]
		if idx := strings.LastIndex(binName, "/"); idx != -1 {
			binName = binName[idx+1:]
		}
		if idx := strings.LastIndex(binName, "\\"); idx != -1 {
			binName = binName[idx+1:]
		}
		if err := targs.PrintCompletionScript(completionShell, binName); err != nil {
			fmt.Fprintf(os.Stderr, "Unsupported shell: %s. Supported: bash, zsh, fish\n", completionShell)
			os.Exit(1)
		}
		return
	}

	startDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error resolving working directory: %v\n", err)
		os.Exit(1)
	}

	taggedDirs, err := buildtool.SelectTaggedDirs(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:     startDir,
		MultiPackage: multiPackage,
		BuildTag:     "targs",
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
			BuildTag:   "targs",
			OnlyTagged: true,
		}); err != nil {
			fmt.Printf("Error generating command wrappers: %v\n", err)
			os.Exit(1)
		}
	}

	infos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:     startDir,
		MultiPackage: multiPackage,
		BuildTag:     "targs",
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

	taggedFiles, err := buildtool.TaggedFiles(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:     startDir,
		MultiPackage: multiPackage,
		BuildTag:     "targs",
	})
	if err != nil {
		fmt.Printf("Error gathering tagged files: %v\n", err)
		os.Exit(1)
	}
	moduleFiles, err := collectModuleFiles(moduleRoot)
	if err != nil {
		fmt.Printf("Error gathering module files: %v\n", err)
		os.Exit(1)
	}
	cacheInputs := append(taggedFiles, moduleFiles...)
	cacheKey, err := computeCacheKey(modulePath, moduleRoot, "targs", buf.Bytes(), cacheInputs)
	if err != nil {
		fmt.Printf("Error computing cache key: %v\n", err)
		os.Exit(1)
	}

	tempDir := filepath.Join(moduleRoot, ".targs", "tmp")
	tempFile, cleanupTemp, err := writeBootstrapFile(tempDir, buf.Bytes(), keepBootstrap)
	if err != nil {
		fmt.Printf("Error writing bootstrap file: %v\n", err)
		os.Exit(1)
	}
	if !keepBootstrap {
		defer cleanupTemp()
	}

	cacheDir := filepath.Join(moduleRoot, ".targs", "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Printf("Error creating cache directory: %v\n", err)
		os.Exit(1)
	}
	binaryPath := filepath.Join(cacheDir, fmt.Sprintf("targs_%s", cacheKey))

	if !noCache {
		if info, err := os.Stat(binaryPath); err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			cmd := exec.Command(binaryPath, args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			if err := cmd.Run(); err != nil {
				if !keepBootstrap {
					_ = cleanupTemp()
				}
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
					os.Exit(exitErr.ExitCode())
				}
				fmt.Printf("Error running command: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	buildArgs := []string{"build", "-tags", "targs", "-o", binaryPath, tempFile}
	buildCmd := exec.Command("go", buildArgs...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		if !keepBootstrap {
			_ = cleanupTemp()
		}
		fmt.Printf("Error building command: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if !keepBootstrap {
			_ = cleanupTemp()
		}
		os.Exit(1)
	}
}

func writeBootstrapFile(tempDir string, data []byte, keep bool) (string, func() error, error) {
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", nil, err
	}
	tempFile := filepath.Join(tempDir, "targs_bootstrap.go")
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return "", nil, err
	}
	cleanup := func() error {
		if keep {
			return nil
		}
		if err := os.Remove(tempFile); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return tempFile, cleanup, nil
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

func detectShell() string {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		return ""
	}
	base := shell
	if idx := strings.LastIndex(base, "/"); idx != -1 {
		base = base[idx+1:]
	}
	if idx := strings.LastIndex(base, "\\"); idx != -1 {
		base = base[idx+1:]
	}
	switch base {
	case "bash", "zsh", "fish":
		return base
	default:
		return ""
	}
}

func printMultiPackageError(startDir string, multiErr *buildtool.MultipleTaggedDirsError) error {
	infos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:     startDir,
		MultiPackage: true,
		BuildTag:     "targs",
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
	fmt.Println("For more information, run `targs --help`.")
	return nil
}

func printBuildToolUsage(out io.Writer) {
	fmt.Fprintln(out, "targs is a build-tool runner that discovers tagged commands and executes them.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage: targs [--multipackage|-m] [--no-cache] [--keep] [args]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Flags:")
	fmt.Fprintln(out, "  --multipackage, -m    enable multipackage mode (recursive package-scoped discovery)")
	fmt.Fprintln(out, "  --no-cache            disable cached build tool binaries")
	fmt.Fprintln(out, "  --keep                keep generated bootstrap file")
	fmt.Fprintln(out, "  --completion [bash|zsh|fish]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "More info: https://github.com/toejough/targs#readme")
}

type packageSummary struct {
	Name     string
	Path     string
	Doc      string
	Commands []string
}

func printBuildToolHelp(out io.Writer, startDir string, multiPackage bool) error {
	printBuildToolUsage(out)
	fmt.Fprintln(out, "")

	currentInfos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:     startDir,
		MultiPackage: multiPackage,
		BuildTag:     "targs",
	})
	if err != nil && !errors.Is(err, buildtool.ErrNoTaggedFiles) {
		return err
	}

	allInfos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:     startDir,
		MultiPackage: true,
		BuildTag:     "targs",
	})
	if err != nil && !errors.Is(err, buildtool.ErrNoTaggedFiles) {
		return err
	}

	currentSummaries := summarizePackages(currentInfos, startDir)
	allSummaries := summarizePackages(allInfos, startDir)

	if len(currentSummaries) == 0 {
		fmt.Fprintln(out, "No tagged packages found in this directory.")
		return nil
	}

	if multiPackage {
		fmt.Fprintln(out, "Subcommands:")
		printPackageSummaries(out, currentSummaries)
		return nil
	}

	fmt.Fprintln(out, "Loaded package:")
	printPackageSummaries(out, currentSummaries)

	otherSummaries := filterOtherPackages(currentSummaries, allSummaries)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Other discovered packages:")
	printPackageSummaries(out, otherSummaries)
	if len(otherSummaries) > 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Use -m/--multipackage or run from a package directory to access these.")
	}
	return nil
}

func summarizePackages(infos []buildtool.PackageInfo, startDir string) []packageSummary {
	summaries := make([]packageSummary, 0, len(infos))
	for _, info := range infos {
		commands := append([]string{}, info.Structs...)
		commands = append(commands, info.Funcs...)
		for i, name := range commands {
			commands[i] = camelToKebab(name)
		}
		sort.Strings(commands)
		path := info.Dir
		if rel, err := filepath.Rel(startDir, info.Dir); err == nil {
			path = rel
		}
		summaries = append(summaries, packageSummary{
			Name:     info.Package,
			Path:     path,
			Doc:      info.Doc,
			Commands: commands,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})
	return summaries
}

func filterOtherPackages(loaded []packageSummary, all []packageSummary) []packageSummary {
	seen := map[string]bool{}
	for _, summary := range loaded {
		key := summary.Path + "::" + summary.Name
		seen[key] = true
	}
	var others []packageSummary
	for _, summary := range all {
		key := summary.Path + "::" + summary.Name
		if seen[key] {
			continue
		}
		others = append(others, summary)
	}
	return others
}

func printPackageSummaries(out io.Writer, summaries []packageSummary) {
	if len(summaries) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	for _, summary := range summaries {
		line := fmt.Sprintf("  %s", summary.Name)
		if summary.Doc != "" {
			line = fmt.Sprintf("%s - %s", line, summary.Doc)
		}
		fmt.Fprintln(out, line)
		fmt.Fprintf(out, "    Path: %s\n", summary.Path)
		if len(summary.Commands) > 0 {
			fmt.Fprintf(out, "    Commands: %s\n", strings.Join(summary.Commands, ", "))
		}
	}
}

func parseHelpRequest(args []string) (bool, bool) {
	helpRequested := false
	sawTarget := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--help" || arg == "-h" {
			helpRequested = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		sawTarget = true
	}
	return helpRequested, sawTarget
}

func camelToKebab(name string) string {
	var out strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out.WriteByte('-')
		}
		out.WriteRune(r)
	}
	return strings.ToLower(out.String())
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
	imports := []bootstrapImport{{Path: "targs"}}
	usedImports := map[string]bool{"targs": true}
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
	if candidate == "targs" {
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

func collectModuleFiles(moduleRoot string) ([]buildtool.TaggedFile, error) {
	var files []buildtool.TaggedFile
	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == ".targs" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, buildtool.TaggedFile{
			Path:    path,
			Content: data,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

const bootstrapTemplate = `
package main

import (
	"targs"
{{- if .BannerLit }}
	"fmt"
	"os"
{{- end }}
{{- range .Imports }}
{{- if and (ne .Path "targs") (ne .Alias "") }}
	{{ .Alias }} "{{ .Path }}"
{{- else if ne .Path "targs" }}
	"{{ .Path }}"
{{- end }}
{{- end }}
)

{{- if .UsePackageWrapper }}
{{- range .Packages }}
type {{ .TypeName }} struct {
{{- range .Commands }}
	{{ .Name }} {{ .TypeExpr }} ` + "`targs:\"subcommand\"`" + `
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

	targs.RunWithOptions(targs.RunOptions{AllowDefault: {{ .AllowDefault }}}, roots...)
{{- else }}
	cmds := []interface{}{
{{- range .Targets }}
		{{ . }},
{{- end }}
	}
	targs.RunWithOptions(targs.RunOptions{AllowDefault: {{ .AllowDefault }}}, cmds...)
{{- end }}
}
`

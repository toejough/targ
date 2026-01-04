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
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf8"

	"targ"
	"targ/buildtool"
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
	var helpFlag bool

	fs := flag.NewFlagSet("targ", flag.ContinueOnError)
	fs.BoolVar(&multiPackage, "multipackage", false, "enable multipackage mode (recursive package-scoped discovery)")
	fs.BoolVar(&multiPackage, "m", false, "alias for --multipackage")
	fs.BoolVar(&noCache, "no-cache", false, "disable cached build tool binaries")
	fs.BoolVar(&keepBootstrap, "keep", false, "keep generated bootstrap file")
	fs.StringVar(&completionShell, "completion", "", "print shell completion (bash|zsh|fish)")
	fs.BoolVar(&helpFlag, "help", false, "print help information")
	fs.BoolVar(&helpFlag, "h", false, "alias for --help")
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
		fmt.Fprintln(os.Stderr, err)
		printBuildToolUsage(os.Stderr)
		os.Exit(1)
	}
	args := fs.Args()
	if helpFlag {
		helpRequested = true
	}

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
	if helpRequested && helpTargets {
		args = append(args, "--help")
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
		if err := targ.PrintCompletionScript(completionShell, binName); err != nil {
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
		BuildTag:     "targ",
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
			BuildTag:   "targ",
			OnlyTagged: true,
		}); err != nil {
			fmt.Printf("Error generating command wrappers: %v\n", err)
			os.Exit(1)
		}
	}

	infos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:     startDir,
		MultiPackage: multiPackage,
		BuildTag:     "targ",
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

	importRoot, modulePath, moduleFound, err := findModuleRootAndPath(startDir)
	if err != nil {
		fmt.Printf("Error finding module root: %v\n", err)
		os.Exit(1)
	}
	buildRoot := importRoot
	usingFallback := false
	if !moduleFound {
		importRoot = startDir
		modulePath = "targ.local"
		dep := resolveTargDependency()
		buildRoot, err = ensureFallbackModuleRoot(startDir, modulePath, dep)
		if err != nil {
			fmt.Printf("Error preparing fallback module: %v\n", err)
			os.Exit(1)
		}
		usingFallback = true
	}

	data, err := buildBootstrapData(infos, startDir, importRoot, modulePath, multiPackage)
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
		BuildTag:     "targ",
	})
	if err != nil {
		fmt.Printf("Error gathering tagged files: %v\n", err)
		os.Exit(1)
	}
	moduleFiles, err := collectModuleFiles(importRoot)
	if err != nil {
		fmt.Printf("Error gathering module files: %v\n", err)
		os.Exit(1)
	}
	cacheInputs := append(taggedFiles, moduleFiles...)
	cacheKey, err := computeCacheKey(modulePath, importRoot, "targ", buf.Bytes(), cacheInputs)
	if err != nil {
		fmt.Printf("Error computing cache key: %v\n", err)
		os.Exit(1)
	}

	bootstrapDir := startDir
	if usingFallback {
		bootstrapDir = buildRoot
	}
	_, cleanupTemp, err := writeBootstrapFile(bootstrapDir, buf.Bytes(), keepBootstrap)
	if err != nil {
		fmt.Printf("Error writing bootstrap file: %v\n", err)
		os.Exit(1)
	}
	if !keepBootstrap {
		defer cleanupTemp()
	}

	cacheDir := filepath.Join(buildRoot, ".targ", "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Printf("Error creating cache directory: %v\n", err)
		os.Exit(1)
	}
	binaryPath := filepath.Join(cacheDir, fmt.Sprintf("targ_%s", cacheKey))

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

	buildArgs := []string{"build", "-tags", "targ", "-o", binaryPath}
	if usingFallback {
		buildArgs = append(buildArgs, "-mod=mod")
	}
	buildArgs = append(buildArgs, ".")
	buildCmd := exec.Command("go", buildArgs...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if usingFallback {
		buildCmd.Dir = buildRoot
	} else {
		buildCmd.Dir = startDir
	}
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

func writeBootstrapFile(dir string, data []byte, keep bool) (string, func() error, error) {
	temp, err := os.CreateTemp(dir, "targ_bootstrap_*.go")
	if err != nil {
		return "", nil, err
	}
	tempFile := temp.Name()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return "", nil, err
	}
	if err := temp.Close(); err != nil {
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

func findModuleRootAndPath(startDir string) (string, string, bool, error) {
	dir := startDir
	for {
		modPath := filepath.Join(dir, "go.mod")
		data, err := os.ReadFile(modPath)
		if err == nil {
			modulePath := parseModulePath(string(data))
			if modulePath == "" {
				return "", "", true, fmt.Errorf("module path not found in %s", modPath)
			}
			return dir, modulePath, true, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", "", false, nil
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

type targDependency struct {
	ModulePath string
	Version    string
	ReplaceDir string
}

func resolveTargDependency() targDependency {
	dep := targDependency{
		ModulePath: "targ",
		Version:    "v0.0.0",
	}
	if override := strings.TrimSpace(os.Getenv("TARG_MODULE_DIR")); override != "" {
		if info, err := os.Stat(override); err == nil && info.IsDir() {
			dep.ReplaceDir = override
			return dep
		}
	}
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		dep.Version = info.Main.Version
		if modCache, err := goEnv("GOMODCACHE"); err == nil && modCache != "" {
			candidate := filepath.Join(modCache, dep.ModulePath+"@"+dep.Version)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				dep.ReplaceDir = candidate
			}
		}
	}
	return dep
}

func goEnv(key string) (string, error) {
	cmd := exec.Command("go", "env", key)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func ensureFallbackModuleRoot(startDir string, modulePath string, dep targDependency) (string, error) {
	hash := sha256.Sum256([]byte(startDir))
	root := filepath.Join(startDir, ".targ", "cache", "mod", fmt.Sprintf("%x", hash[:8]))
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	if err := linkModuleRoot(startDir, root); err != nil {
		return "", err
	}
	if err := writeFallbackGoMod(root, modulePath, dep); err != nil {
		return "", err
	}
	if err := touchFile(filepath.Join(root, "go.sum")); err != nil {
		return "", err
	}
	return root, nil
}

func linkModuleRoot(startDir string, root string) error {
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == ".targ" || name == ".git" {
			continue
		}
		src := filepath.Join(startDir, name)
		dst := filepath.Join(root, name)
		info, err := os.Lstat(dst)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			_ = os.RemoveAll(dst)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Symlink(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func writeFallbackGoMod(root string, modulePath string, dep targDependency) error {
	modPath := filepath.Join(root, "go.mod")
	if dep.ModulePath == "" {
		dep.ModulePath = "targ"
	}
	if dep.Version == "" {
		dep.Version = "v0.0.0"
	}
	lines := []string{
		"module " + modulePath,
		"",
		"go 1.21",
		"",
		fmt.Sprintf("require %s %s", dep.ModulePath, dep.Version),
	}
	if dep.ReplaceDir != "" {
		lines = append(lines, "", fmt.Sprintf("replace %s => %s", dep.ModulePath, dep.ReplaceDir))
	}
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(modPath, []byte(content), 0644)
}

func touchFile(path string) error {
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		return err
	}
	return nil
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
		BuildTag:     "targ",
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
	fmt.Println("For more information, run `targ --help`.")
	return nil
}

func printBuildToolUsage(out io.Writer) {
	fmt.Fprintln(out, "targ is a build-tool runner that discovers tagged commands and executes them.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage: targ [FLAGS...] COMMAND [COMMAND_ARGS...]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Flags:")
	fmt.Fprintf(out, "    %-28s %s\n", "--multipackage, -m", "enable multipackage mode (recursive package-scoped discovery)")
	fmt.Fprintf(out, "    %-28s %s\n", "--no-cache", "disable cached build tool binaries")
	fmt.Fprintf(out, "    %-28s %s\n", "--keep", "keep generated bootstrap file")
	fmt.Fprintf(out, "    %-28s %s\n", "--completion {bash|zsh|fish}", "print completion script for specified shell. Uses the current shell if none is")
	fmt.Fprintf(out, "    %-28s %s\n", "", "specified. The output should be eval'd/sourced in the shell to enable completions.")
	fmt.Fprintf(out, "    %-28s %s\n", "", "(e.g. 'targ --completion fish | source')")
	fmt.Fprintf(out, "    %-28s %s\n", "--help", "Print help information")
}

type packageSummary struct {
	Name string
	Path string
	Doc  string
}

func printBuildToolHelp(out io.Writer, startDir string, multiPackage bool) error {
	printBuildToolUsage(out)
	fmt.Fprintln(out, "")

	currentInfos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:     startDir,
		MultiPackage: multiPackage,
		BuildTag:     "targ",
	})
	if err != nil && !errors.Is(err, buildtool.ErrNoTaggedFiles) {
		return err
	}

	allInfos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir:     startDir,
		MultiPackage: true,
		BuildTag:     "targ",
	})
	if err != nil && !errors.Is(err, buildtool.ErrNoTaggedFiles) {
		return err
	}

	currentSummaries := summarizePackages(currentInfos, startDir)
	allSummaries := summarizePackages(allInfos, startDir)

	if len(currentSummaries) == 0 {
		fmt.Fprintln(out, "No tagged packages found in this directory.")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "More info: https://github.com/toejough/targ#readme")
		return nil
	}

	if multiPackage {
		fmt.Fprintln(out, "Subcommands:")
		printPackageSummaries(out, currentSummaries)
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "More info: https://github.com/toejough/targ#readme")
		return nil
	}

	fmt.Fprintln(out, "Loaded package:")
	printPackageSummaries(out, currentSummaries)

	if len(currentInfos) > 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Commands:")
		printCommandSummaries(out, commandSummaries(currentInfos[0]))
	}

	otherSummaries := filterOtherPackages(currentSummaries, allSummaries)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Other discovered packages:")
	printPackageSummaries(out, otherSummaries)
	if len(otherSummaries) > 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Use -m/--multipackage or run from a package directory to access these.")
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "More info: https://github.com/toejough/targ#readme")
	return nil
}

func summarizePackages(infos []buildtool.PackageInfo, startDir string) []packageSummary {
	summaries := make([]packageSummary, 0, len(infos))
	for _, info := range infos {
		path := info.Dir
		if rel, err := filepath.Rel(startDir, info.Dir); err == nil {
			path = rel
		}
		summaries = append(summaries, packageSummary{
			Name: info.Package,
			Path: path,
			Doc:  info.Doc,
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
		fmt.Fprintln(out, "    (none)")
		return
	}
	for _, summary := range summaries {
		name := summary.Name
		if summary.Doc != "" {
			fmt.Fprintf(out, "    %-10s %s\n", name, summary.Doc)
		} else {
			fmt.Fprintf(out, "    %s\n", name)
		}
		fmt.Fprintf(out, "                Path: %s\n", summary.Path)
	}
}

type commandSummary struct {
	Name        string
	Description string
}

func commandSummaries(info buildtool.PackageInfo) []commandSummary {
	commands := make([]commandSummary, 0, len(info.Structs)+len(info.Funcs))
	for _, name := range info.Structs {
		desc := ""
		if info.StructDescriptions != nil {
			desc = info.StructDescriptions[name]
		}
		commands = append(commands, commandSummary{
			Name:        camelToKebab(name),
			Description: desc,
		})
	}
	for _, name := range info.Funcs {
		desc := ""
		if info.FuncDescriptions != nil {
			desc = info.FuncDescriptions[name]
		}
		commands = append(commands, commandSummary{
			Name:        camelToKebab(name),
			Description: desc,
		})
	}
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})
	return commands
}

func printCommandSummaries(out io.Writer, summaries []commandSummary) {
	if len(summaries) == 0 {
		fmt.Fprintln(out, "    (none)")
		return
	}
	for _, summary := range summaries {
		if summary.Description != "" {
			fmt.Fprintf(out, "    %-10s %s\n", summary.Name, summary.Description)
		} else {
			fmt.Fprintf(out, "    %s\n", summary.Name)
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
	imports := []bootstrapImport{{Path: "targ"}}
	usedImports := map[string]bool{"targ": true}
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
	if candidate == "targ" {
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
			if name == ".git" || name == ".targ" || name == "vendor" {
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
	"targ"
{{- if .BannerLit }}
	"fmt"
	"os"
{{- end }}
{{- range .Imports }}
{{- if and (ne .Path "targ") (ne .Alias "") }}
	{{ .Alias }} "{{ .Path }}"
{{- else if ne .Path "targ" }}
	"{{ .Path }}"
{{- end }}
{{- end }}
)

{{- if .UsePackageWrapper }}
{{- range .Packages }}
type {{ .TypeName }} struct {
{{- range .Commands }}
	{{ .Name }} {{ .TypeExpr }} ` + "`targ:\"subcommand\"`" + `
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

	targ.RunWithOptions(targ.RunOptions{AllowDefault: {{ .AllowDefault }}}, roots...)
{{- else }}
	cmds := []interface{}{
{{- range .Targets }}
		{{ . }},
{{- end }}
	}
	targ.RunWithOptions(targ.RunOptions{AllowDefault: {{ .AllowDefault }}}, cmds...)
{{- end }}
}
`

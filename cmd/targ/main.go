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
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/toejough/targ"
	"github.com/toejough/targ/buildtool"
)

type bootstrapCommand struct {
	Name      string
	TypeExpr  string
	ValueExpr string
}

type bootstrapImport struct {
	Path  string
	Alias string
}

type bootstrapFuncWrapper struct {
	TypeName     string
	Name         string
	FuncExpr     string
	UsesContext  bool
	ReturnsError bool
}

type bootstrapNode struct {
	Name     string
	TypeName string
	VarName  string
	Fields   []bootstrapField
}

type bootstrapField struct {
	Name      string
	TypeExpr  string
	TagLit    string
	ValueExpr string
	SetValue  bool
}

type bootstrapData struct {
	AllowDefault bool
	BannerLit    string
	Imports      []bootstrapImport
	RootExprs    []string
	Nodes        []bootstrapNode
	FuncWrappers []bootstrapFuncWrapper
	UsesContext  bool
}

func main() {
	var noCache bool
	var keepBootstrap bool
	var helpFlag bool
	var timeoutFlag string
	var completionShell string

	fs := flag.NewFlagSet("targ", flag.ContinueOnError)
	fs.BoolVar(&noCache, "no-cache", false, "disable cached build tool binaries")
	fs.BoolVar(&keepBootstrap, "keep", false, "keep generated bootstrap file")
	fs.BoolVar(&helpFlag, "help", false, "print help information")
	fs.BoolVar(&helpFlag, "h", false, "alias for --help")
	fs.Usage = func() {
		printBuildToolUsage(os.Stdout)
	}
	fs.SetOutput(io.Discard)
	rawArgs := os.Args[1:]
	quietBuild := len(rawArgs) > 0 && rawArgs[0] == "__complete"
	errOut := io.Writer(os.Stderr)
	if quietBuild {
		errOut = io.Discard
	}
	helpRequested, helpTargets := parseHelpRequest(rawArgs)
	// Extract leading flags before any command
	timeoutFlag, rawArgs = extractLeadingTimeout(rawArgs)
	completionShell, rawArgs = extractLeadingCompletion(rawArgs)
	completionRequested := completionShell != ""
	parseArgs := make([]string, 0, len(rawArgs))
	for _, arg := range rawArgs {
		parseArgs = append(parseArgs, arg)
	}
	if err := fs.Parse(parseArgs); err != nil {
		fmt.Fprintln(errOut, err)
		printBuildToolUsage(errOut)
		os.Exit(1)
	}
	args := fs.Args()
	if helpFlag {
		helpRequested = true
	}

	if helpRequested && !helpTargets {
		startDir, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(errOut, "Error resolving working directory: %v\n", err)
			os.Exit(1)
		}
		if err := printBuildToolHelp(os.Stdout, startDir); err != nil {
			fmt.Fprintf(errOut, "Error discovering packages: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if helpRequested && helpTargets {
		args = append(args, "--help")
	}

	// Prepend timeout flag to args for the bootstrap binary
	if timeoutFlag != "" {
		args = append([]string{"--timeout", timeoutFlag}, args...)
	}

	if completionRequested {
		if completionShell == "auto" {
			completionShell = detectShell()
		}
		if completionShell == "" || completionShell == "auto" {
			fmt.Fprintln(errOut, "Usage: --completion [bash|zsh|fish]")
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
			fmt.Fprintf(errOut, "Unsupported shell: %s. Supported: bash, zsh, fish\n", completionShell)
			os.Exit(1)
		}
		return
	}

	startDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(errOut, "Error resolving working directory: %v\n", err)
		os.Exit(1)
	}

	taggedDirs, err := buildtool.SelectTaggedDirs(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir: startDir,
		BuildTag: "targ",
	})
	if err != nil {
		fmt.Fprintf(errOut, "Error discovering commands: %v\n", err)
		os.Exit(1)
	}

	var generatedWrappers []string
	for _, dir := range taggedDirs {
		wrapper, err := buildtool.GenerateFunctionWrappers(buildtool.OSFileSystem{}, buildtool.GenerateOptions{
			Dir:        dir.Path,
			BuildTag:   "targ",
			OnlyTagged: true,
		})
		if err != nil {
			fmt.Fprintf(errOut, "Error generating command wrappers: %v\n", err)
			os.Exit(1)
		}
		if wrapper != "" {
			generatedWrappers = append(generatedWrappers, wrapper)
		}
	}
	cleanupWrappers := func() {
		for _, path := range generatedWrappers {
			_ = os.Remove(path)
		}
	}
	exit := func(code int) {
		cleanupWrappers()
		os.Exit(code)
	}

	infos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir: startDir,
		BuildTag: "targ",
	})
	if err != nil {
		fmt.Fprintf(errOut, "Error discovering commands: %v\n", err)
		exit(1)
	}

	// Determine the target directory (where the target files are)
	packageDir := startDir
	if len(infos) == 1 && infos[0].Package == "main" {
		packageDir = infos[0].Dir
	}

	// Only use a go.mod if it's in the same directory as the target files
	// This ensures targets are self-contained and don't accidentally depend
	// on a parent module that may not have targ as a dependency
	importRoot, modulePath, moduleFound, err := findModuleInDir(packageDir)
	if err != nil {
		fmt.Fprintf(errOut, "Error checking for module: %v\n", err)
		exit(1)
	}
	buildRoot := importRoot
	usingFallback := false
	if !moduleFound {
		importRoot = startDir
		modulePath = "targ.local"
		dep := resolveTargDependency()
		buildRoot, err = ensureFallbackModuleRoot(startDir, modulePath, dep)
		if err != nil {
			fmt.Fprintf(errOut, "Error preparing fallback module: %v\n", err)
			exit(1)
		}
		usingFallback = true
	}

	data, err := buildBootstrapData(infos, packageDir, importRoot, modulePath)
	if err != nil {
		fmt.Fprintf(errOut, "Error preparing bootstrap: %v\n", err)
		exit(1)
	}

	tmpl := template.Must(template.New("main").Parse(bootstrapTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		fmt.Fprintf(errOut, "Error generating code: %v\n", err)
		exit(1)
	}

	taggedFiles, err := buildtool.TaggedFiles(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir: startDir,
		BuildTag: "targ",
	})
	if err != nil {
		fmt.Fprintf(errOut, "Error gathering tagged files: %v\n", err)
		exit(1)
	}
	moduleFiles, err := collectModuleFiles(importRoot)
	if err != nil {
		fmt.Fprintf(errOut, "Error gathering module files: %v\n", err)
		exit(1)
	}
	cacheInputs := append(taggedFiles, moduleFiles...)
	cacheKey, err := computeCacheKey(modulePath, importRoot, "targ", buf.Bytes(), cacheInputs)
	if err != nil {
		fmt.Fprintf(errOut, "Error computing cache key: %v\n", err)
		exit(1)
	}

	localMain := len(infos) == 1 && infos[0].Package == "main"
	relPackageDir, err := filepath.Rel(startDir, packageDir)
	if err != nil {
		fmt.Fprintf(errOut, "Error resolving package path: %v\n", err)
		exit(1)
	}
	buildPackageDir := packageDir
	if usingFallback {
		buildPackageDir = filepath.Join(buildRoot, relPackageDir)
	}
	projCache := projectCacheDir(importRoot)
	bootstrapDir := filepath.Join(projCache, "tmp")
	if usingFallback {
		// When using fallback module, bootstrap must be inside buildRoot
		// so it can find the go.mod with the replace directive
		bootstrapDir = filepath.Join(buildRoot, "tmp")
	}
	if localMain {
		cacheRoot := projCache
		if usingFallback {
			cacheRoot = buildRoot
		}
		localMainDir, err := ensureLocalMainBuildDir(packageDir, cacheRoot)
		if err != nil {
			fmt.Fprintf(errOut, "Error preparing local main build directory: %v\n", err)
			exit(1)
		}
		buildPackageDir = localMainDir
		bootstrapDir = localMainDir
	}

	var tempFile string
	var cleanupTemp func() error
	tempFile, cleanupTemp, err = writeBootstrapFile(bootstrapDir, buf.Bytes(), keepBootstrap)
	if err != nil {
		fmt.Fprintf(errOut, "Error writing bootstrap file: %v\n", err)
		exit(1)
	}
	if !keepBootstrap {
		defer cleanupTemp()
	}

	cacheDir := filepath.Join(projCache, "bin")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Fprintf(errOut, "Error creating cache directory: %v\n", err)
		exit(1)
	}
	binaryPath := filepath.Join(cacheDir, fmt.Sprintf("targ_%s", cacheKey))

	// Get binary name for help output
	targBinName := "targ"
	if binArg := os.Args[0]; binArg != "" {
		if idx := strings.LastIndex(binArg, "/"); idx != -1 {
			targBinName = binArg[idx+1:]
		} else if idx := strings.LastIndex(binArg, "\\"); idx != -1 {
			targBinName = binArg[idx+1:]
		} else {
			targBinName = binArg
		}
	}

	if !noCache {
		if info, err := os.Stat(binaryPath); err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			cmd := exec.Command(binaryPath, args...)
			cmd.Env = append(os.Environ(), "TARG_BIN_NAME="+targBinName)
			cmd.Stdout = os.Stdout
			cmd.Stderr = errOut
			cmd.Stdin = os.Stdin
			if err := cmd.Run(); err != nil {
				if !keepBootstrap {
					_ = cleanupTemp()
				}
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
					exit(exitErr.ExitCode())
				}
				fmt.Fprintf(errOut, "Error running command: %v\n", err)
				exit(1)
			}
			cleanupWrappers()
			return
		}
	}

	buildArgs := []string{"build", "-tags", "targ", "-o", binaryPath}
	if usingFallback {
		buildArgs = append(buildArgs, "-mod=mod")
	}
	if localMain {
		buildArgs = append(buildArgs, ".")
	} else {
		buildArgs = append(buildArgs, tempFile)
	}
	buildCmd := exec.Command("go", buildArgs...)
	var buildOutput bytes.Buffer
	buildCmd.Stdout = io.Discard
	buildCmd.Stderr = &buildOutput
	if localMain {
		buildCmd.Dir = buildPackageDir
	} else if usingFallback {
		buildCmd.Dir = buildRoot
	} else {
		buildCmd.Dir = importRoot
	}
	if err := buildCmd.Run(); err != nil {
		if !keepBootstrap {
			_ = cleanupTemp()
		}
		if buildOutput.Len() > 0 {
			fmt.Fprint(errOut, buildOutput.String())
		}
		fmt.Fprintf(errOut, "Error building command: %v\n", err)
		exit(1)
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(), "TARG_BIN_NAME="+targBinName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = errOut
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if !keepBootstrap {
			_ = cleanupTemp()
		}
		exit(1)
	}
	cleanupWrappers()
}

func writeBootstrapFile(dir string, data []byte, keep bool) (string, func() error, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", nil, err
	}
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

// findModuleInDir checks for a go.mod only in the specified directory (no walking up).
// This ensures targets are self-contained and don't accidentally use a parent module.
func findModuleInDir(dir string) (string, string, bool, error) {
	modPath := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	modulePath := parseModulePath(string(data))
	if modulePath == "" {
		return "", "", true, fmt.Errorf("module path not found in %s", modPath)
	}
	return dir, modulePath, true, nil
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
		ModulePath: "github.com/toejough/targ",
	}
	info, ok := debug.ReadBuildInfo()
	if ok {
		if looksLikeModulePath(info.Main.Path) {
			dep.ModulePath = info.Main.Path
		}
		if info.Main.Version != "" && info.Main.Version != "(devel)" && !strings.Contains(info.Main.Version, "+dirty") {
			if modCache, err := goEnv("GOMODCACHE"); err == nil && modCache != "" {
				candidate := filepath.Join(modCache, dep.ModulePath+"@"+info.Main.Version)
				if statInfo, err := os.Stat(candidate); err == nil && statInfo.IsDir() {
					dep.Version = info.Main.Version
					dep.ReplaceDir = candidate
				}
			}
		} else if root, ok := buildSourceRoot(); ok {
			dep.ReplaceDir = root
		}
	}
	return dep
}

func buildSourceRoot() (string, bool) {
	_, file, _, ok := runtime.Caller(0)
	if !ok || file == "" {
		return "", false
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

func looksLikeModulePath(path string) bool {
	if path == "" {
		return false
	}
	first := strings.Split(path, "/")[0]
	return strings.Contains(first, ".")
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
	root := filepath.Join(projectCacheDir(startDir), "mod", fmt.Sprintf("%x", hash[:8]))
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
		if name == ".git" {
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

func ensureLocalMainBuildDir(packageDir string, cacheRoot string) (string, error) {
	hash := sha256.Sum256([]byte(packageDir))
	dir := filepath.Join(cacheRoot, "localmain", fmt.Sprintf("%x", hash[:8]))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" {
			continue
		}
		src := filepath.Join(packageDir, name)
		dst := filepath.Join(dir, name)
		info, err := os.Lstat(dst)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			_ = os.RemoveAll(dst)
		} else if !os.IsNotExist(err) {
			return "", err
		}
		if err := os.Symlink(src, dst); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func writeFallbackGoMod(root string, modulePath string, dep targDependency) error {
	modPath := filepath.Join(root, "go.mod")
	if dep.ModulePath == "" {
		dep.ModulePath = "github.com/toejough/targ"
	}
	lines := []string{
		"module " + modulePath,
		"",
		"go 1.21",
	}
	if dep.Version != "" {
		lines = append(lines, "", fmt.Sprintf("require %s %s", dep.ModulePath, dep.Version))
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

// targCacheDir returns the centralized cache directory for targ following XDG spec.
// Uses $XDG_CACHE_HOME/targ or ~/.cache/targ as fallback.
func targCacheDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "targ")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to temp directory if home can't be determined
		return filepath.Join(os.TempDir(), "targ-cache")
	}
	return filepath.Join(home, ".cache", "targ")
}

// projectCacheDir returns a project-specific subdirectory within the targ cache.
// Uses a hash of the project path to isolate projects.
func projectCacheDir(projectPath string) string {
	hash := sha256.Sum256([]byte(projectPath))
	return filepath.Join(targCacheDir(), fmt.Sprintf("%x", hash[:8]))
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

func printBuildToolUsage(out io.Writer) {
	fmt.Fprintln(out, "targ is a build-tool runner that discovers tagged commands and executes them.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage: targ [FLAGS...] COMMAND [COMMAND_ARGS...]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Flags:")
	fmt.Fprintf(out, "    %-28s %s\n", "--no-cache", "disable cached build tool binaries")
	fmt.Fprintf(out, "    %-28s %s\n", "--keep", "keep generated bootstrap file")
	fmt.Fprintf(out, "    %-28s %s\n", "--timeout <duration>", "set execution timeout (e.g., 10m, 1h)")
	fmt.Fprintf(out, "    %-28s %s\n", "--completion {bash|zsh|fish}", "print completion script for specified shell. Uses the current shell if none is")
	fmt.Fprintf(out, "    %-28s %s\n", "", "specified. The output should be eval'd/sourced in the shell to enable completions.")
	fmt.Fprintf(out, "    %-28s %s\n", "", "(e.g. 'targ --completion fish | source')")
	fmt.Fprintf(out, "    %-28s %s\n", "--help", "Print help information")
}

func printBuildToolHelp(out io.Writer, startDir string) error {
	printBuildToolUsage(out)
	fmt.Fprintln(out, "")

	infos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir: startDir,
		BuildTag: "targ",
	})
	if err != nil && !errors.Is(err, buildtool.ErrNoTaggedFiles) {
		return err
	}

	if len(infos) == 0 {
		fmt.Fprintln(out, "No tagged commands found in this directory.")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "More info: https://github.com/toejough/targ#readme")
		return nil
	}

	fileCommands := make(map[string][]commandSummary)
	var filePaths []string
	for _, info := range infos {
		for _, file := range info.Files {
			summaries := commandSummariesFromCommands(file.Commands)
			fileCommands[file.Path] = summaries
			filePaths = append(filePaths, file.Path)
		}
	}
	sort.Strings(filePaths)
	paths, err := namespacePaths(filePaths, startDir)
	if err != nil {
		return err
	}

	var rootCommands []commandSummary
	for _, path := range filePaths {
		if len(paths[path]) != 0 {
			continue
		}
		rootCommands = append(rootCommands, fileCommands[path]...)
	}
	if len(rootCommands) > 0 {
		sort.Slice(rootCommands, func(i, j int) bool {
			return rootCommands[i].Name < rootCommands[j].Name
		})
		fmt.Fprintln(out, "Commands:")
		printCommandSummaries(out, rootCommands)
		fmt.Fprintln(out, "")
	}

	tree := buildNamespaceTree(paths)
	if len(tree.Children) > 0 {
		names := make([]string, 0, len(tree.Children))
		for name := range tree.Children {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Fprintln(out, "Subcommands:")
		for _, name := range names {
			fmt.Fprintf(out, "    %s\n", name)
		}
		fmt.Fprintln(out, "")
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "More info: https://github.com/toejough/targ#readme")
	return nil
}

type commandSummary struct {
	Name        string
	Description string
}

func commandSummariesFromCommands(commands []buildtool.CommandInfo) []commandSummary {
	summaries := make([]commandSummary, 0, len(commands))
	for _, cmd := range commands {
		summaries = append(summaries, commandSummary{
			Name:        camelToKebab(cmd.Name),
			Description: cmd.Description,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})
	return summaries
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

// extractLeadingTimeout extracts --timeout from args before any command.
// Returns the timeout value (empty if not found) and remaining args.
func extractLeadingTimeout(args []string) (string, []string) {
	var result []string
	var timeout string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		// Stop looking for timeout once we hit a non-flag (command name)
		if !strings.HasPrefix(arg, "-") {
			result = append(result, args[i:]...)
			break
		}
		if arg == "--timeout" {
			if i+1 < len(args) {
				timeout = args[i+1]
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--timeout=") {
			timeout = strings.TrimPrefix(arg, "--timeout=")
			continue
		}
		result = append(result, arg)
	}
	return timeout, result
}

// extractLeadingCompletion extracts --completion from args before any command.
// Returns the shell value (empty if not found) and remaining args.
func extractLeadingCompletion(args []string) (string, []string) {
	var result []string
	var shell string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		// Stop looking for completion once we hit a non-flag (command name)
		if !strings.HasPrefix(arg, "-") {
			result = append(result, args[i:]...)
			break
		}
		if arg == "--completion" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				shell = args[i+1]
				i++
			} else {
				shell = "auto" // Signal that completion was requested but no shell specified
			}
			continue
		}
		if strings.HasPrefix(arg, "--completion=") {
			shell = strings.TrimPrefix(arg, "--completion=")
			if shell == "" {
				shell = "auto"
			}
			continue
		}
		result = append(result, arg)
	}
	return shell, result
}

func camelToKebab(name string) string {
	var result strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			// Insert hyphen if previous is lowercase (e.g., fooBar -> foo-bar)
			// OR if we're at the start of a new word after an acronym (e.g., APIServer -> api-server)
			if unicode.IsLower(prev) || (i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
				result.WriteRune('-')
			}
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

type namespaceNode struct {
	Name     string
	File     string
	Children map[string]*namespaceNode
	TypeName string
	VarName  string
}

func namespacePaths(files []string, root string) (map[string][]string, error) {
	if len(files) == 0 {
		return map[string][]string{}, nil
	}
	raw := make(map[string][]string, len(files))
	paths := make([][]string, 0, len(files))
	for _, file := range files {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		if len(parts) == 0 {
			parts = []string{filepath.Base(file)}
		}
		last := parts[len(parts)-1]
		parts[len(parts)-1] = strings.TrimSuffix(last, filepath.Ext(last))
		raw[file] = parts
		paths = append(paths, parts)
	}

	common := append([]string(nil), paths[0]...)
	for _, p := range paths[1:] {
		common = commonPrefix(common, p)
		if len(common) == 0 {
			break
		}
	}

	trimmed := make(map[string][]string, len(files))
	for file, parts := range raw {
		if len(common) >= len(parts) {
			trimmed[file] = nil
			continue
		}
		trimmed[file] = append([]string(nil), parts[len(common):]...)
	}
	return compressNamespacePaths(trimmed), nil
}

func commonPrefix(a []string, b []string) []string {
	max := len(a)
	if len(b) < max {
		max = len(b)
	}
	var i int
	for i = 0; i < max; i++ {
		if a[i] != b[i] {
			break
		}
	}
	return a[:i]
}

func compressNamespacePaths(paths map[string][]string) map[string][]string {
	root := &namespaceNode{Children: make(map[string]*namespaceNode)}
	out := make(map[string][]string, len(paths))

	for file, parts := range paths {
		if len(parts) == 0 {
			out[file] = nil
			continue
		}
		node := root
		for _, part := range parts {
			child := node.Children[part]
			if child == nil {
				child = &namespaceNode{Name: part, Children: make(map[string]*namespaceNode)}
				node.Children[part] = child
			}
			node = child
		}
		node.File = file
	}

	var walk func(node *namespaceNode, prefix []string)
	walk = func(node *namespaceNode, prefix []string) {
		if node != root && len(node.Children) == 1 && node.File == "" {
			for _, child := range node.Children {
				walk(child, prefix)
			}
			return
		}
		if node != root {
			prefix = append(prefix, node.Name)
		}
		if node.File != "" {
			out[node.File] = append([]string(nil), prefix...)
		}
		names := make([]string, 0, len(node.Children))
		for name := range node.Children {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			walk(node.Children[name], prefix)
		}
	}
	walk(root, nil)
	return out
}

func buildNamespaceTree(paths map[string][]string) *namespaceNode {
	root := &namespaceNode{Children: make(map[string]*namespaceNode)}
	for file, parts := range paths {
		if len(parts) == 0 {
			continue
		}
		node := root
		for _, part := range parts {
			child := node.Children[part]
			if child == nil {
				child = &namespaceNode{Name: part, Children: make(map[string]*namespaceNode)}
				node.Children[part] = child
			}
			node = child
		}
		node.File = file
	}
	return root
}

type nameGenerator struct {
	used map[string]int
}

func (g *nameGenerator) uniqueTypeName(base string) string {
	if g.used == nil {
		g.used = make(map[string]int)
	}
	if base == "" {
		base = "Node"
	}
	count := g.used[base]
	g.used[base] = count + 1
	if count == 0 {
		return base
	}
	return fmt.Sprintf("%s%d", base, count+1)
}

func segmentToIdent(segment string) string {
	var out strings.Builder
	capNext := true
	for _, r := range segment {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			capNext = true
			continue
		}
		if capNext {
			out.WriteRune(unicode.ToUpper(r))
			capNext = false
			continue
		}
		out.WriteRune(r)
	}
	ident := out.String()
	if ident == "" {
		return "Node"
	}
	if !unicode.IsLetter(rune(ident[0])) {
		return "Node" + ident
	}
	return ident
}

func subcommandTag(fieldName string, segment string) string {
	if camelToKebab(fieldName) == segment {
		return `targ:"subcommand"`
	}
	return fmt.Sprintf(`targ:"subcommand,name=%s"`, segment)
}

func assignNamespaceNames(root *namespaceNode, gen *nameGenerator) {
	var walk func(node *namespaceNode)
	walk = func(node *namespaceNode) {
		names := make([]string, 0, len(node.Children))
		for name := range node.Children {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			child := node.Children[name]
			base := segmentToIdent(child.Name)
			child.TypeName = gen.uniqueTypeName(base)
			child.VarName = lowerFirst(child.TypeName)
			walk(child)
		}
	}
	walk(root)
}

func collectNamespaceNodes(root *namespaceNode, fileCommands map[string][]bootstrapCommand, out *[]bootstrapNode) error {
	var walk func(node *namespaceNode) error
	walk = func(node *namespaceNode) error {
		names := make([]string, 0, len(node.Children))
		for name := range node.Children {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := walk(node.Children[name]); err != nil {
				return err
			}
		}

		if node == root {
			return nil
		}

		fields := make([]bootstrapField, 0, len(node.Children))
		usedNames := map[string]bool{}

		for _, name := range names {
			child := node.Children[name]
			fieldName := segmentToIdent(child.Name)
			if usedNames[fieldName] {
				return fmt.Errorf("duplicate namespace field %q under %q", fieldName, node.Name)
			}
			usedNames[fieldName] = true
			fields = append(fields, bootstrapField{
				Name:     fieldName,
				TypeExpr: "*" + child.TypeName,
				TagLit:   subcommandTag(fieldName, child.Name),
			})
		}

		if node.File != "" {
			commands := fileCommands[node.File]
			for _, cmd := range commands {
				if usedNames[cmd.Name] {
					return fmt.Errorf("duplicate command name %q under %q", cmd.Name, node.Name)
				}
				usedNames[cmd.Name] = true
				fields = append(fields, bootstrapField{
					Name:     cmd.Name,
					TypeExpr: cmd.TypeExpr,
					TagLit:   `targ:"subcommand"`,
				})
			}
		}

		*out = append(*out, bootstrapNode{
			Name:     node.Name,
			TypeName: node.TypeName,
			VarName:  node.VarName,
			Fields:   fields,
		})
		return nil
	}
	return walk(root)
}

func buildBootstrapData(
	infos []buildtool.PackageInfo,
	startDir string,
	moduleRoot string,
	modulePath string,
) (bootstrapData, error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return bootstrapData{}, err
	}
	imports := []bootstrapImport{{Path: "github.com/toejough/targ"}}
	usedImports := map[string]bool{"github.com/toejough/targ": true}
	fileCommands := make(map[string][]bootstrapCommand)
	var funcWrappers []bootstrapFuncWrapper
	usesContext := false
	wrapperNames := &nameGenerator{}

	for _, info := range infos {
		if len(info.Commands) == 0 {
			return bootstrapData{}, fmt.Errorf("no commands found in package %s", info.Package)
		}

		local := sameDir(absStart, info.Dir)
		if info.Package == "main" && !local {
			return bootstrapData{}, fmt.Errorf("cannot import package main at %s; run targ from that directory or use a non-main package", info.Dir)
		}
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

		for _, cmd := range info.Commands {
			switch cmd.Kind {
			case buildtool.CommandStruct:
				fileCommands[cmd.File] = append(fileCommands[cmd.File], bootstrapCommand{
					Name:      cmd.Name,
					TypeExpr:  "*" + prefix + cmd.Name,
					ValueExpr: "&" + prefix + cmd.Name + "{}",
				})
			case buildtool.CommandFunc:
				base := segmentToIdent(info.Package) + segmentToIdent(cmd.Name) + "Func"
				typeName := wrapperNames.uniqueTypeName(base)
				funcWrappers = append(funcWrappers, bootstrapFuncWrapper{
					TypeName:     typeName,
					Name:         cmd.Name,
					FuncExpr:     prefix + cmd.Name,
					UsesContext:  cmd.UsesContext,
					ReturnsError: cmd.ReturnsError,
				})
				if cmd.UsesContext {
					usesContext = true
				}
				fileCommands[cmd.File] = append(fileCommands[cmd.File], bootstrapCommand{
					Name:      cmd.Name,
					TypeExpr:  "*" + typeName,
					ValueExpr: "&" + typeName + "{}",
				})
			default:
				return bootstrapData{}, fmt.Errorf("unknown command kind for %s", cmd.Name)
			}
		}
	}

	filePaths := make([]string, 0, len(fileCommands))
	for path := range fileCommands {
		sort.Slice(fileCommands[path], func(i, j int) bool {
			return fileCommands[path][i].Name < fileCommands[path][j].Name
		})
		filePaths = append(filePaths, path)
	}
	sort.Strings(filePaths)

	paths, err := namespacePaths(filePaths, startDir)
	if err != nil {
		return bootstrapData{}, err
	}

	tree := buildNamespaceTree(paths)
	assignNamespaceNames(tree, &nameGenerator{})

	var nodes []bootstrapNode
	rootExprs := make([]string, 0)
	for _, path := range filePaths {
		if len(paths[path]) != 0 {
			continue
		}
		for _, cmd := range fileCommands[path] {
			rootExprs = append(rootExprs, cmd.ValueExpr)
		}
	}
	rootNames := make([]string, 0, len(tree.Children))
	for name := range tree.Children {
		rootNames = append(rootNames, name)
	}
	sort.Strings(rootNames)
	for _, name := range rootNames {
		rootExprs = append(rootExprs, tree.Children[name].VarName)
	}

	if err := collectNamespaceNodes(tree, fileCommands, &nodes); err != nil {
		return bootstrapData{}, err
	}

	allowDefault := false
	bannerLit := ""
	if len(infos) == 1 {
		bannerLit = strconv.Quote(singlePackageBanner(infos[0]))
	}
	return bootstrapData{
		AllowDefault: allowDefault,
		BannerLit:    bannerLit,
		Imports:      imports,
		RootExprs:    rootExprs,
		Nodes:        nodes,
		FuncWrappers: funcWrappers,
		UsesContext:  usesContext,
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
	if candidate == "github.com/toejough/targ" {
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
			if name == ".git" || name == "vendor" {
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
	"github.com/toejough/targ"
{{- if .UsesContext }}
	"context"
{{- end }}
{{- if .BannerLit }}
	"fmt"
	"os"
{{- end }}
{{- range .Imports }}
{{- if and (ne .Path "github.com/toejough/targ") (ne .Alias "") }}
	{{ .Alias }} "{{ .Path }}"
{{- else if ne .Path "github.com/toejough/targ" }}
	"{{ .Path }}"
{{- end }}
{{- end }}
)

{{- range .FuncWrappers }}
type {{ .TypeName }} struct{}

func (c *{{ .TypeName }}) Run({{ if .UsesContext }}ctx context.Context{{ end }}) error {
{{- if .UsesContext }}
{{- if .ReturnsError }}
	return {{ .FuncExpr }}(ctx)
{{- else }}
	{{ .FuncExpr }}(ctx)
	return nil
{{- end }}
{{- else }}
{{- if .ReturnsError }}
	return {{ .FuncExpr }}()
{{- else }}
	{{ .FuncExpr }}()
	return nil
{{- end }}
{{- end }}
}

func (c *{{ .TypeName }}) Name() string {
	return "{{ .Name }}"
}
{{- end }}

{{- range .Nodes }}
type {{ .TypeName }} struct {
{{- range .Fields }}
	{{ .Name }} {{ .TypeExpr }} ` + "`{{ .TagLit }}`" + `
{{- end }}
}
{{- end }}

func main() {
{{- if .BannerLit }}
	if len(os.Args) == 1 || (len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help")) {
		fmt.Println({{ .BannerLit }})
		fmt.Println()
	}
{{- end }}
{{- range .Nodes }}
	{{ .VarName }} := &{{ .TypeName }}{}
{{- end }}

	roots := []interface{}{
{{- range .RootExprs }}
		{{ . }},
{{- end }}
	}

	targ.RunWithOptions(targ.RunOptions{AllowDefault: {{ .AllowDefault }}}, roots...)
}
`

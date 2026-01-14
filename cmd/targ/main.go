package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
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

func main() {
	// Handle --init early, before flag parsing
	if initResult := handleInitFlag(os.Args[1:]); initResult != nil {
		if initResult.err != nil {
			fmt.Fprintln(os.Stderr, initResult.err)
			os.Exit(1)
		}
		fmt.Println(initResult.message)
		return
	}

	// Handle --alias early, before flag parsing
	if aliasResult := handleAliasFlag(os.Args[1:]); aliasResult != nil {
		if aliasResult.err != nil {
			fmt.Fprintln(os.Stderr, aliasResult.err)
			os.Exit(1)
		}
		fmt.Println(aliasResult.message)
		return
	}

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
	parseArgs := append([]string{}, rawArgs...)
	if err := fs.Parse(parseArgs); err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		printBuildToolUsage(errOut)
		os.Exit(1)
	}
	args := fs.Args()
	if helpFlag {
		helpRequested = true
	}

	// For --help without targets, we need to check if it's multi-module
	// This is deferred until after module grouping
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
			_, _ = fmt.Fprintln(errOut, "Usage: --completion [bash|zsh|fish]")
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
			_, _ = fmt.Fprintf(errOut, "Unsupported shell: %s. Supported: bash, zsh, fish\n", completionShell)
			os.Exit(1)
		}
		return
	}

	startDir, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "Error resolving working directory: %v\n", err)
		os.Exit(1)
	}

	taggedDirs, err := buildtool.SelectTaggedDirs(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir: startDir,
		BuildTag: "targ",
	})
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "Error discovering commands: %v\n", err)
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
			_, _ = fmt.Fprintf(errOut, "Error generating command wrappers: %v\n", err)
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
		_, _ = fmt.Fprintf(errOut, "Error discovering commands: %v\n", err)
		exit(1)
	}

	// Group packages by module
	moduleGroups, err := groupByModule(infos, startDir)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "Error grouping packages by module: %v\n", err)
		exit(1)
	}

	// Handle --help without targets
	if helpRequested && !helpTargets {
		if len(moduleGroups) > 1 {
			// Multi-module: build binaries and show aggregated help
			registry, err := buildMultiModuleBinaries(moduleGroups, startDir, noCache, keepBootstrap, errOut)
			if err != nil {
				_, _ = fmt.Fprintf(errOut, "Error building module binaries: %v\n", err)
				exit(1)
			}
			cleanupWrappers()
			printMultiModuleHelp(registry)
			return
		}
		// Single module: use standard help
		if err := printBuildToolHelp(os.Stdout, startDir); err != nil {
			_, _ = fmt.Fprintf(errOut, "Error discovering packages: %v\n", err)
			exit(1)
		}
		cleanupWrappers()
		return
	}

	// For multi-module case, use the new multi-binary dispatch
	if len(moduleGroups) > 1 {
		registry, err := buildMultiModuleBinaries(moduleGroups, startDir, noCache, keepBootstrap, errOut)
		if err != nil {
			_, _ = fmt.Fprintf(errOut, "Error building module binaries: %v\n", err)
			exit(1)
		}
		cleanupWrappers()
		if err := dispatchCommand(registry, args, errOut); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			_, _ = fmt.Fprintf(errOut, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Single module case
	// Collect file paths and compute collapsed namespace paths
	var filePaths []string
	for _, info := range infos {
		for _, f := range info.Files {
			filePaths = append(filePaths, f.Path)
		}
	}
	if len(filePaths) == 0 {
		// Handle completion even with no targets - suggest targ flags
		if len(args) > 0 && args[0] == "__complete" {
			printNoTargetsCompletion(args)
			return
		}
		_, _ = fmt.Fprintf(errOut, "Error: no target files found\n")
		exit(1)
	}

	collapsedPaths, err := namespacePaths(filePaths, startDir)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "Error computing namespace paths: %v\n", err)
		exit(1)
	}

	// Find module root for imports and cache key
	importRoot, modulePath, moduleFound, err := findModuleForPath(filePaths[0])
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "Error checking for module: %v\n", err)
		exit(1)
	}
	if !moduleFound {
		importRoot = startDir
		modulePath = "targ.local"
	}

	// Only use isolation when package main targ files coexist with library files
	useIsolation := needsIsolation(infos)

	data, err := buildBootstrapData(infos, startDir, importRoot, modulePath, collapsedPaths, useIsolation)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "Error preparing bootstrap: %v\n", err)
		exit(1)
	}

	tmpl := template.Must(template.New("main").Parse(bootstrapTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		_, _ = fmt.Fprintf(errOut, "Error generating code: %v\n", err)
		exit(1)
	}

	taggedFiles, err := buildtool.TaggedFiles(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir: startDir,
		BuildTag: "targ",
	})
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "Error gathering tagged files: %v\n", err)
		exit(1)
	}
	moduleFiles, err := collectModuleFiles(importRoot)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "Error gathering module files: %v\n", err)
		exit(1)
	}
	cacheInputs := append(taggedFiles, moduleFiles...)
	cacheModulePath := modulePath
	if useIsolation {
		cacheModulePath = "targ.build.local"
	}
	cacheKey, err := computeCacheKey(cacheModulePath, importRoot, "targ", buf.Bytes(), cacheInputs)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "Error computing cache key: %v\n", err)
		exit(1)
	}

	projCache := projectCacheDir(importRoot)
	bootstrapDir := filepath.Join(projCache, "tmp")

	cacheDir := filepath.Join(projCache, "bin")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		_, _ = fmt.Fprintf(errOut, "Error creating cache directory: %v\n", err)
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

	// Check if cached binary exists - if so, run it without writing bootstrap file
	if !noCache {
		if info, err := os.Stat(binaryPath); err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			cmd := exec.Command(binaryPath, args...)
			cmd.Env = append(os.Environ(), "TARG_BIN_NAME="+targBinName)
			cmd.Stdout = os.Stdout
			cmd.Stderr = errOut
			cmd.Stdin = os.Stdin
			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
					exit(exitErr.ExitCode())
				}
				_, _ = fmt.Fprintf(errOut, "Error running command: %v\n", err)
				exit(1)
			}
			cleanupWrappers()
			return
		}
	}

	// Write bootstrap file only when we need to build
	var tempFile string
	var cleanupTemp func() error
	tempFile, cleanupTemp, err = writeBootstrapFile(bootstrapDir, buf.Bytes(), keepBootstrap)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "Error writing bootstrap file: %v\n", err)
		exit(1)
	}
	if !keepBootstrap {
		defer func() { _ = cleanupTemp() }()
	}

	var buildDir string
	var cleanupBuildDir func()
	if useIsolation {
		// Create isolated build directory to avoid module conflicts
		buildDir, cleanupBuildDir, err = createIsolatedBuildDir(infos, collapsedPaths, generatedWrappers, tempFile, startDir, keepBootstrap)
		if err != nil {
			_, _ = fmt.Fprintf(errOut, "Error creating isolated build directory: %v\n", err)
			exit(1)
		}
		if !keepBootstrap {
			defer cleanupBuildDir()
		}

		// Run go mod tidy to ensure dependencies are resolved
		tidyCmd := exec.Command("go", "mod", "tidy")
		tidyCmd.Dir = buildDir
		tidyCmd.Stdout = io.Discard
		tidyCmd.Stderr = io.Discard
		_ = tidyCmd.Run() // Ignore errors - build will catch any real issues
	} else {
		// Build in module root - no isolation needed
		buildDir = importRoot
	}

	var buildArgs []string
	if useIsolation {
		buildArgs = []string{"build", "-tags", "targ", "-o", binaryPath, "."}
	} else {
		// For non-isolated build, explicitly list the bootstrap file
		buildArgs = []string{"build", "-tags", "targ", "-o", binaryPath, tempFile}
	}
	buildCmd := exec.Command("go", buildArgs...)
	var buildOutput bytes.Buffer
	buildCmd.Stdout = io.Discard
	buildCmd.Stderr = &buildOutput
	buildCmd.Dir = buildDir
	if err := buildCmd.Run(); err != nil {
		if !keepBootstrap {
			_ = cleanupTemp()
		}
		if buildOutput.Len() > 0 {
			_, _ = fmt.Fprint(errOut, buildOutput.String())
		}
		_, _ = fmt.Fprintf(errOut, "Error building command: %v\n", err)
		exit(1)
	}

	// Clean up bootstrap file before running the binary
	// so commands like reorderDeclsCheck don't see it
	if !keepBootstrap {
		_ = cleanupTemp()
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(), "TARG_BIN_NAME="+targBinName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = errOut
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		exit(1)
	}
	cleanupWrappers()
}

// unexported constants.
const (
	bootstrapTemplate = `
package main

import (
	"github.com/toejough/targ"
	"github.com/toejough/targ/sh"
{{- if .UsesContext }}
	"context"
{{- end }}
{{- if .BannerLit }}
	"fmt"
	"os"
{{- end }}
{{- range .Imports }}
{{- if and (ne .Path "github.com/toejough/targ") (ne .Path "github.com/toejough/targ/sh") (ne .Alias "") }}
	{{ .Alias }} "{{ .Path }}"
{{- else if and (ne .Path "github.com/toejough/targ") (ne .Path "github.com/toejough/targ/sh") }}
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
	sh.EnableCleanup()
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
)

type aliasResult struct {
	message string
	err     error
}

type bootstrapCommand struct {
	Name      string
	TypeExpr  string
	ValueExpr string
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

type bootstrapField struct {
	Name      string
	TypeExpr  string
	TagLit    string
	ValueExpr string
	SetValue  bool
}

type bootstrapFuncWrapper struct {
	TypeName     string
	Name         string
	FuncExpr     string
	UsesContext  bool
	ReturnsError bool
}

type bootstrapImport struct {
	Path  string
	Alias string
}

type bootstrapNode struct {
	Name     string
	TypeName string
	VarName  string
	Fields   []bootstrapField
}

// commandInfo represents a command from a module binary.
type commandInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type commandSummary struct {
	Name        string
	Description string
}

type initResult struct {
	message string
	err     error
}

// listOutput is the JSON structure returned by __list command.
type listOutput struct {
	Commands []commandInfo `json:"commands"`
}

// moduleRegistry tracks built binaries and their commands.
type moduleRegistry struct {
	BinaryPath string
	ModuleRoot string
	ModulePath string
	Commands   []commandInfo
}

// moduleTargets groups discovered packages by their module.
type moduleTargets struct {
	ModuleRoot string
	ModulePath string
	Packages   []buildtool.PackageInfo
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

type namespaceNode struct {
	Name     string
	File     string
	Children map[string]*namespaceNode
	TypeName string
	VarName  string
}

type targDependency struct {
	ModulePath string
	Version    string
	ReplaceDir string
}

// addAlias generates and appends alias code to the appropriate target file.
func addAlias(name, command, targetFile string) (string, error) {
	code, err := generateAlias(name, command)
	if err != nil {
		return "", err
	}

	// If target file specified, use it directly
	if targetFile != "" {
		if err := appendToFile(targetFile, code); err != nil {
			return "", err
		}
		return fmt.Sprintf("Added %s to %s", toExportedName(name), targetFile), nil
	}

	// Discover target files in current directory
	targetFiles, err := findTargetFiles(".")
	if err != nil {
		return "", fmt.Errorf("discovering target files: %w", err)
	}

	switch len(targetFiles) {
	case 0:
		// No target files - create targs.go
		targetFile = "targs.go"
		if _, err := createTargetsFile(targetFile); err != nil {
			return "", err
		}
		if err := appendToFile(targetFile, code); err != nil {
			return "", err
		}
		return fmt.Sprintf("Created %s and added %s", targetFile, toExportedName(name)), nil

	case 1:
		// One target file - ensure sh import and append
		targetFile = targetFiles[0]
		if err := ensureShImport(targetFile); err != nil {
			return "", fmt.Errorf("ensuring sh import: %w", err)
		}
		if err := appendToFile(targetFile, code); err != nil {
			return "", err
		}
		return fmt.Sprintf("Added %s to %s", toExportedName(name), targetFile), nil

	default:
		// Multiple target files - require explicit file
		return "", fmt.Errorf("multiple target files found (%s); specify which file: --alias %s %q <file>",
			strings.Join(targetFiles, ", "), name, command)
	}
}

// appendToFile appends content to a file.
func appendToFile(path string, content string) (err error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	_, err = f.WriteString(content)
	return err
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

func buildBootstrapData(
	infos []buildtool.PackageInfo,
	startDir string,
	moduleRoot string,
	modulePath string,
	collapsedPaths map[string][]string,
	useIsolation bool,
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

		var local bool
		var importPath string

		if useIsolation {
			// In isolated mode, "local" means empty collapsed path (at root of temp dir)
			var pkgCollapsedPath []string
			if len(info.Files) > 0 {
				pkgCollapsedPath = collapsedPaths[info.Files[0].Path]
			}
			local = len(pkgCollapsedPath) == 0
			if !local {
				importPath = "targ.build.local/" + strings.Join(pkgCollapsedPath, "/")
			}
		} else {
			// In non-isolated mode, "local" means same directory as startDir
			local = sameDir(absStart, info.Dir)
			if !local {
				rel, relErr := filepath.Rel(moduleRoot, info.Dir)
				if relErr != nil {
					return bootstrapData{}, relErr
				}
				importPath = modulePath
				if rel != "." {
					importPath = modulePath + "/" + filepath.ToSlash(rel)
				}
			}
		}

		if info.Package == "main" && !local {
			return bootstrapData{}, fmt.Errorf("cannot import package main at %s; run targ from that directory or use a non-main package", info.Dir)
		}
		importName := ""
		prefix := ""
		if !local {
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

// buildModuleBinary builds a single module's binary and queries its commands.
func buildModuleBinary(
	mt moduleTargets,
	startDir string,
	dep targDependency,
	noCache bool,
	keepBootstrap bool,
	errOut io.Writer,
) (moduleRegistry, error) {
	reg := moduleRegistry{
		ModuleRoot: mt.ModuleRoot,
		ModulePath: mt.ModulePath,
	}

	// Determine if using fallback module
	usingFallback := mt.ModulePath == "targ.local"
	buildRoot := mt.ModuleRoot
	importRoot := mt.ModuleRoot

	if usingFallback {
		var err error
		buildRoot, err = ensureFallbackModuleRoot(startDir, mt.ModulePath, dep)
		if err != nil {
			return reg, fmt.Errorf("preparing fallback module: %w", err)
		}
	}

	// Determine package directory
	packageDir := startDir
	if len(mt.Packages) == 1 && mt.Packages[0].Package == "main" {
		packageDir = mt.Packages[0].Dir
	}

	// Collect file paths and compute collapsed namespace paths
	var filePaths []string
	for _, pkg := range mt.Packages {
		for _, f := range pkg.Files {
			filePaths = append(filePaths, f.Path)
		}
	}
	collapsedPaths, err := namespacePaths(filePaths, startDir)
	if err != nil {
		return reg, fmt.Errorf("computing namespace paths: %w", err)
	}

	// Check if isolation is needed for this module
	useIsolation := needsIsolation(mt.Packages)

	// Build bootstrap data
	data, err := buildBootstrapData(mt.Packages, startDir, importRoot, mt.ModulePath, collapsedPaths, useIsolation)
	if err != nil {
		return reg, fmt.Errorf("preparing bootstrap: %w", err)
	}

	// Generate bootstrap code
	tmpl := template.Must(template.New("main").Parse(bootstrapTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return reg, fmt.Errorf("generating code: %w", err)
	}

	// Compute cache key for this module
	taggedFiles, err := collectModuleTaggedFiles(mt)
	if err != nil {
		return reg, fmt.Errorf("gathering tagged files: %w", err)
	}
	moduleFiles, err := collectModuleFiles(importRoot)
	if err != nil {
		return reg, fmt.Errorf("gathering module files: %w", err)
	}
	cacheInputs := append(taggedFiles, moduleFiles...)
	cacheKey, err := computeCacheKey(mt.ModulePath, importRoot, "targ", buf.Bytes(), cacheInputs)
	if err != nil {
		return reg, fmt.Errorf("computing cache key: %w", err)
	}

	// Determine build directories
	localMain := len(mt.Packages) == 1 && mt.Packages[0].Package == "main"
	buildPackageDir := packageDir
	bootstrapDir := filepath.Join(projectCacheDir(importRoot), "tmp")

	if usingFallback {
		relPackageDir, err := filepath.Rel(startDir, packageDir)
		if err != nil {
			return reg, fmt.Errorf("resolving package path: %w", err)
		}
		buildPackageDir = filepath.Join(buildRoot, relPackageDir)
		bootstrapDir = filepath.Join(buildRoot, "tmp")
	}

	if localMain {
		if usingFallback {
			localMainDir, err := ensureLocalMainBuildDir(packageDir, buildRoot)
			if err != nil {
				return reg, fmt.Errorf("preparing local main build directory: %w", err)
			}
			buildPackageDir = localMainDir
			bootstrapDir = localMainDir
		} else {
			bootstrapDir = packageDir
			buildPackageDir = packageDir
		}
	}

	// Write bootstrap file
	tempFile, cleanupTemp, err := writeBootstrapFile(bootstrapDir, buf.Bytes(), keepBootstrap)
	if err != nil {
		return reg, fmt.Errorf("writing bootstrap file: %w", err)
	}
	if !keepBootstrap {
		defer func() { _ = cleanupTemp() }()
	}

	// Determine binary path
	projCache := projectCacheDir(importRoot)
	cacheDir := filepath.Join(projCache, "bin")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return reg, fmt.Errorf("creating cache directory: %w", err)
	}
	binaryPath := filepath.Join(cacheDir, fmt.Sprintf("targ_%s", cacheKey))
	reg.BinaryPath = binaryPath

	// Check cache
	if !noCache {
		if info, err := os.Stat(binaryPath); err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			// Binary exists, query commands
			cmds, err := queryModuleCommands(binaryPath)
			if err != nil {
				return reg, fmt.Errorf("querying commands: %w", err)
			}
			reg.Commands = cmds
			return reg, nil
		}
	}

	// Ensure targ dependency is available (bootstrap imports it even if targets don't)
	getCmd := exec.Command("go", "get", dep.ModulePath)
	getCmd.Dir = importRoot
	getCmd.Stdout = io.Discard
	getCmd.Stderr = io.Discard
	_ = getCmd.Run() // Ignore errors - the build will fail if there's a real issue

	// Build the binary
	buildArgs := []string{"build", "-tags", "targ", "-o", binaryPath}
	if usingFallback {
		buildArgs = append(buildArgs, "-mod=mod")
	}
	if localMain {
		// List only targ-tagged files explicitly (not ".") to avoid
		// including non-tagged files with different package names
		for _, pkg := range mt.Packages {
			for _, f := range pkg.Files {
				buildArgs = append(buildArgs, filepath.Base(f.Path))
			}
		}
		// Add any generated wrapper files in the directory
		wrappers, _ := filepath.Glob(filepath.Join(buildPackageDir, "generated_targ_*.go"))
		for _, wrapper := range wrappers {
			buildArgs = append(buildArgs, filepath.Base(wrapper))
		}
		// Add bootstrap file
		buildArgs = append(buildArgs, filepath.Base(tempFile))
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
		if buildOutput.Len() > 0 {
			_, _ = fmt.Fprint(errOut, buildOutput.String())
		}
		return reg, fmt.Errorf("building command: %w", err)
	}

	// Query commands from the newly built binary
	cmds, err := queryModuleCommands(binaryPath)
	if err != nil {
		return reg, fmt.Errorf("querying commands: %w", err)
	}
	reg.Commands = cmds

	return reg, nil
}

// buildMultiModuleBinaries builds a binary for each module group and returns the registry.
func buildMultiModuleBinaries(
	moduleGroups []moduleTargets,
	startDir string,
	noCache bool,
	keepBootstrap bool,
	errOut io.Writer,
) ([]moduleRegistry, error) {
	var registry []moduleRegistry
	dep := resolveTargDependency()

	for _, mt := range moduleGroups {
		reg, err := buildModuleBinary(mt, startDir, dep, noCache, keepBootstrap, errOut)
		if err != nil {
			return nil, fmt.Errorf("building module %s: %w", mt.ModulePath, err)
		}
		registry = append(registry, reg)
	}

	return registry, nil
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
		name := entry.Name()
		// Include go.mod and go.sum for cache invalidation when dependencies change
		isModFile := name == "go.mod" || name == "go.sum"
		isGoFile := strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
		if !isModFile && !isGoFile {
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

// collectModuleTaggedFiles collects tagged files from a module's packages.
func collectModuleTaggedFiles(mt moduleTargets) ([]buildtool.TaggedFile, error) {
	var files []buildtool.TaggedFile
	for _, pkg := range mt.Packages {
		for _, f := range pkg.Files {
			data, err := os.ReadFile(f.Path)
			if err != nil {
				return nil, err
			}
			files = append(files, buildtool.TaggedFile{
				Path:    f.Path,
				Content: data,
			})
		}
	}
	return files, nil
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

// createIsolatedBuildDir creates a temporary directory with copies of tagged files.
// This isolates the build from the source module, avoiding package conflicts when
// targ-tagged files coexist with library files in the same directory.
// Files are organized using the collapsed namespace structure (same as subcommand hierarchy).
func createIsolatedBuildDir(
	infos []buildtool.PackageInfo,
	collapsedPaths map[string][]string,
	wrappers []string,
	bootstrapFile string,
	startDir string,
	keep bool,
) (string, func(), error) {
	// Create temp directory outside any Go module
	tmpDir, err := os.MkdirTemp("", "targ-build-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}

	cleanup := func() {
		if !keep {
			_ = os.RemoveAll(tmpDir)
		}
	}

	// Copy ALL targ files to isolated directory with collapsed structure
	// Strip the build tag so files compile without -tags targ
	for _, info := range infos {
		for _, f := range info.Files {
			data, err := os.ReadFile(f.Path)
			if err != nil {
				cleanup()
				return "", nil, fmt.Errorf("reading %s: %w", f.Path, err)
			}
			content := stripBuildTag(string(data))

			// Determine target directory from collapsed path
			collapsedPath := collapsedPaths[f.Path]
			targetDir := tmpDir
			if len(collapsedPath) > 0 {
				targetDir = filepath.Join(tmpDir, filepath.Join(collapsedPath...))
			}
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				cleanup()
				return "", nil, fmt.Errorf("creating dir %s: %w", targetDir, err)
			}

			dst := filepath.Join(targetDir, filepath.Base(f.Path))
			if err := os.WriteFile(dst, []byte(content), 0644); err != nil {
				cleanup()
				return "", nil, fmt.Errorf("writing %s: %w", dst, err)
			}
		}
	}

	// Copy generated wrapper files to root (they're package main for bootstrap)
	for _, wrapper := range wrappers {
		data, err := os.ReadFile(wrapper)
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("reading wrapper %s: %w", wrapper, err)
		}
		dst := filepath.Join(tmpDir, filepath.Base(wrapper))
		if err := os.WriteFile(dst, data, 0644); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("writing wrapper %s: %w", dst, err)
		}
	}

	// Copy bootstrap file to root
	bootstrapData, err := os.ReadFile(bootstrapFile)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("reading bootstrap: %w", err)
	}
	bootstrapDst := filepath.Join(tmpDir, filepath.Base(bootstrapFile))
	if err := os.WriteFile(bootstrapDst, bootstrapData, 0644); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("writing bootstrap: %w", err)
	}

	// Setup go.mod by copying from project and replacing module name
	if err := setupIsolatedGoMod(tmpDir, startDir); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("setting up go.mod: %w", err)
	}

	return tmpDir, cleanup, nil
}

// createTargetsFile creates a starter targets file with the build tag.
func createTargetsFile(filename string) (string, error) {
	// Check if file already exists
	if _, err := os.Stat(filename); err == nil {
		return "", fmt.Errorf("%s already exists", filename)
	}

	content := `//go:build targ

package main

import "github.com/toejough/targ/sh"

// Keep the compiler happy - sh is used by generated aliases
var _ = sh.Run
`

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", filename, err)
	}

	return fmt.Sprintf("Created %s", filename), nil
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

// dispatchCommand finds the right binary for a command and executes it.
func dispatchCommand(registry []moduleRegistry, args []string, errOut io.Writer) error {
	// Handle help request
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		printMultiModuleHelp(registry)
		return nil
	}

	// Handle completion
	if args[0] == "__complete" {
		return dispatchCompletion(registry, args)
	}

	// Find the command in the registry
	cmdName := args[0]
	for _, reg := range registry {
		for _, cmd := range reg.Commands {
			// Check if command matches (exact match or prefix for subcommands)
			if cmd.Name == cmdName || strings.HasPrefix(cmd.Name, cmdName+" ") {
				// Execute via the module's binary
				proc := exec.Command(reg.BinaryPath, args...)
				proc.Stdin = os.Stdin
				proc.Stdout = os.Stdout
				proc.Stderr = errOut

				// Set TARG_BIN_NAME for proper help output
				targBinName := "targ"
				if binArg := os.Args[0]; binArg != "" {
					if idx := strings.LastIndex(binArg, "/"); idx != -1 {
						targBinName = binArg[idx+1:]
					} else {
						targBinName = binArg
					}
				}
				proc.Env = append(os.Environ(), "TARG_BIN_NAME="+targBinName)

				return proc.Run()
			}
		}
	}

	// Command not found
	_, _ = fmt.Fprintf(errOut, "Unknown command: %s\n", cmdName)
	printMultiModuleHelp(registry)
	return fmt.Errorf("unknown command: %s", cmdName)
}

// dispatchCompletion handles completion requests by querying all binaries.
func dispatchCompletion(registry []moduleRegistry, args []string) error {
	if len(args) < 2 {
		return nil
	}

	// Query each binary for completions and aggregate
	seen := make(map[string]bool)
	for _, reg := range registry {
		cmd := exec.Command(reg.BinaryPath, args...)
		output, err := cmd.Output()
		if err != nil {
			continue // Skip failed completions
		}
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !seen[line] {
				seen[line] = true
				fmt.Println(line)
			}
		}
	}

	return nil
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

// ensureShImport ensures the file imports github.com/toejough/targ/sh.
func ensureShImport(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)

	// Check if already imported
	if strings.Contains(content, `"github.com/toejough/targ/sh"`) {
		return nil
	}

	// Find the import block or single import
	lines := strings.Split(content, "\n")
	var result []string
	importAdded := false

	for i, line := range lines {
		result = append(result, line)
		trimmed := strings.TrimSpace(line)

		// Handle import block: import (
		if trimmed == "import (" && !importAdded {
			// Add sh import after the opening paren
			result = append(result, `	"github.com/toejough/targ/sh"`)
			importAdded = true
			continue
		}

		// Handle single import: import "..."
		if strings.HasPrefix(trimmed, "import \"") && !importAdded {
			// Convert to import block
			result[len(result)-1] = "import ("
			result = append(result, "\t"+strings.TrimPrefix(trimmed, "import "))
			result = append(result, `	"github.com/toejough/targ/sh"`)
			result = append(result, ")")
			importAdded = true
			continue
		}

		// If no imports yet and we hit package, add import after it
		if strings.HasPrefix(trimmed, "package ") && !importAdded {
			// Look ahead - if next non-empty line isn't import, add one
			hasImport := false
			for j := i + 1; j < len(lines); j++ {
				nextTrimmed := strings.TrimSpace(lines[j])
				if nextTrimmed == "" {
					continue
				}
				if strings.HasPrefix(nextTrimmed, "import") {
					hasImport = true
				}
				break
			}
			if !hasImport {
				result = append(result, "")
				result = append(result, `import "github.com/toejough/targ/sh"`)
				importAdded = true
			}
		}
	}

	return os.WriteFile(path, []byte(strings.Join(result, "\n")), 0644)
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

// findModuleForPath walks up from the given path to find the nearest go.mod.
// Returns the module root directory, module path, whether found, and any error.
func findModuleForPath(path string) (string, string, bool, error) {
	// Start from the directory containing the path
	dir := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
	}

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
		if !os.IsNotExist(err) {
			return "", "", false, err
		}

		// Move up to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}
	return "", "", false, nil
}

// findTargetFiles finds all files with //go:build targ in the given directory.
func findTargetFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if hasTargBuildTag(path) {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

// generateAlias creates Go code for a simple shell command target.
func generateAlias(name string, command string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("alias name cannot be empty")
	}

	// Convert name to exported Go function name
	funcName := toExportedName(name)

	// Parse command into parts
	parts, err := parseShellCommand(command)
	if err != nil {
		return "", fmt.Errorf("parsing command: %w", err)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("command cannot be empty")
	}

	// Build sh.Run arguments
	var argsStr string
	for i, part := range parts {
		if i > 0 {
			argsStr += ", "
		}
		argsStr += strconv.Quote(part)
	}

	// Generate the code with leading newline for nice appending
	code := fmt.Sprintf(`
// %s runs %q.
func %s() error {
	return sh.Run(%s)
}
`, funcName, command, funcName, argsStr)

	return code, nil
}

func goEnv(key string) (string, error) {
	cmd := exec.Command("go", "env", key)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// groupByModule groups packages by their module root.
// Packages without a module are grouped under startDir with "targ.local" module path.
func groupByModule(infos []buildtool.PackageInfo, startDir string) ([]moduleTargets, error) {
	byModule := make(map[string]*moduleTargets)

	for _, info := range infos {
		if len(info.Files) == 0 {
			continue
		}

		// Find module for first file in package
		modRoot, modPath, found, err := findModuleForPath(info.Files[0].Path)
		if err != nil {
			return nil, err
		}
		if !found {
			// No module found - use startDir as pseudo-module
			modRoot = startDir
			modPath = "targ.local"
		}

		// Group by module root
		if mt, ok := byModule[modRoot]; ok {
			mt.Packages = append(mt.Packages, info)
		} else {
			byModule[modRoot] = &moduleTargets{
				ModuleRoot: modRoot,
				ModulePath: modPath,
				Packages:   []buildtool.PackageInfo{info},
			}
		}
	}

	// Convert to sorted slice for deterministic ordering
	result := make([]moduleTargets, 0, len(byModule))
	for _, mt := range byModule {
		result = append(result, *mt)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ModuleRoot < result[j].ModuleRoot
	})

	return result, nil
}

// handleAliasFlag checks for --alias and generates target code.
// Returns nil if --alias was not specified.
func handleAliasFlag(args []string) *aliasResult {
	for i, arg := range args {
		var name, command, targetFile string

		if arg == "--alias" {
			if i+2 >= len(args) {
				return &aliasResult{err: fmt.Errorf("--alias requires at least two arguments: NAME \"COMMAND\" [FILE]")}
			}
			name = args[i+1]
			command = args[i+2]
			// Optional third argument for target file
			if i+3 < len(args) && !strings.HasPrefix(args[i+3], "-") {
				targetFile = args[i+3]
			}
		} else if strings.HasPrefix(arg, "--alias=") {
			// --alias=name "command" [file] format
			name = strings.TrimPrefix(arg, "--alias=")
			if i+1 >= len(args) {
				return &aliasResult{err: fmt.Errorf("--alias requires a command argument")}
			}
			command = args[i+1]
			if i+2 < len(args) && !strings.HasPrefix(args[i+2], "-") {
				targetFile = args[i+2]
			}
		} else {
			continue
		}

		msg, err := addAlias(name, command, targetFile)
		return &aliasResult{message: msg, err: err}
	}
	return nil
}

// handleInitFlag checks for --init and creates a starter targets file.
// Returns nil if --init was not specified.
func handleInitFlag(args []string) *initResult {
	for i, arg := range args {
		if arg == "--init" {
			filename := "targs.go"
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				filename = args[i+1]
			}
			msg, err := createTargetsFile(filename)
			return &initResult{message: msg, err: err}
		}
		if strings.HasPrefix(arg, "--init=") {
			filename := strings.TrimPrefix(arg, "--init=")
			if filename == "" {
				filename = "targs.go"
			}
			msg, err := createTargetsFile(filename)
			return &initResult{message: msg, err: err}
		}
	}
	return nil
}

// hasNonTargGoFiles returns true if the directory contains Go files without the targ build tag.
func hasNonTargGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// Skip generated files
		if strings.HasPrefix(name, "generated_targ_") {
			continue
		}
		path := filepath.Join(dir, name)
		if !hasTargBuildTag(path) {
			return true
		}
	}
	return false
}

// hasTargBuildTag checks if a file has the //go:build targ tag.
func hasTargBuildTag(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	// Check for //go:build targ (with possible other tags)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "//go:build") && strings.Contains(line, "targ") {
			return true
		}
		// Stop at package declaration
		if strings.HasPrefix(line, "package ") {
			break
		}
	}
	return false
}

func linkModuleRoot(startDir string, root string) error {
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		// Skip .git and module files - we'll create our own go.mod/go.sum
		if name == ".git" || name == "go.mod" || name == "go.sum" {
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
	// Clean up stale go.mod/go.sum symlinks from before the fix
	for _, name := range []string{"go.mod", "go.sum"} {
		dst := filepath.Join(root, name)
		info, err := os.Lstat(dst)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(dst)
		}
	}
	return nil
}

func looksLikeModulePath(path string) bool {
	if path == "" {
		return false
	}
	first := strings.Split(path, "/")[0]
	return strings.Contains(first, ".")
}

func lowerFirst(name string) string {
	if name == "" {
		return "pkg"
	}
	return strings.ToLower(name[:1]) + name[1:]
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

// needsIsolation returns true if the build contains package main targ files
// that coexist with non-targ library files in the same directory.
// This is the only case where isolation is required to avoid package conflicts.
func needsIsolation(infos []buildtool.PackageInfo) bool {
	for _, info := range infos {
		// Only package main can conflict with library files
		if info.Package != "main" {
			continue
		}
		// Check if there are non-targ Go files in this directory
		if hasNonTargGoFiles(info.Dir) {
			return true
		}
	}
	return false
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

func parseModulePath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// parseShellCommand splits a shell command string into parts.
// Handles quoted strings.
func parseShellCommand(cmd string) ([]string, error) {
	var parts []string
	var current strings.Builder
	inQuote := rune(0)
	escaped := false

	for _, r := range cmd {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		if r == '"' || r == '\'' {
			inQuote = r
			continue
		}
		if r == ' ' || r == '\t' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}

	if inQuote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts, nil
}

func printBuildToolHelp(out io.Writer, startDir string) error {
	printBuildToolUsage(out)
	_, _ = fmt.Fprintln(out, "")

	infos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir: startDir,
		BuildTag: "targ",
	})
	if err != nil && !errors.Is(err, buildtool.ErrNoTaggedFiles) {
		return err
	}

	if len(infos) == 0 {
		_, _ = fmt.Fprintln(out, "No tagged commands found in this directory.")
		_, _ = fmt.Fprintln(out, "")
		_, _ = fmt.Fprintln(out, "More info: https://github.com/toejough/targ#readme")
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
		_, _ = fmt.Fprintln(out, "Commands:")
		printCommandSummaries(out, rootCommands)
		_, _ = fmt.Fprintln(out, "")
	}

	tree := buildNamespaceTree(paths)
	if len(tree.Children) > 0 {
		names := make([]string, 0, len(tree.Children))
		for name := range tree.Children {
			names = append(names, name)
		}
		sort.Strings(names)
		_, _ = fmt.Fprintln(out, "Subcommands:")
		for _, name := range names {
			_, _ = fmt.Fprintf(out, "    %s\n", name)
		}
		_, _ = fmt.Fprintln(out, "")
	}

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "More info: https://github.com/toejough/targ#readme")
	return nil
}

// printNoTargetsCompletion outputs completion suggestions when no target files exist.
// This allows users to discover flags like --init even before creating targets.
func printNoTargetsCompletion(args []string) {
	// Parse the command line from __complete args
	if len(args) < 2 {
		return
	}
	cmdLine := args[1]
	parts := strings.Fields(cmdLine)
	// Remove binary name
	if len(parts) > 0 {
		parts = parts[1:]
	}

	// Determine prefix (what user is typing)
	prefix := ""
	if len(parts) > 0 && !strings.HasSuffix(cmdLine, " ") {
		prefix = parts[len(parts)-1]
	}

	// All targ flags available at root level
	allFlags := []string{
		"--help",
		"--timeout",
		"--no-cache",
		"--keep",
		"--completion",
		"--init",
		"--alias",
	}

	for _, flag := range allFlags {
		if strings.HasPrefix(flag, prefix) {
			fmt.Println(flag)
		}
	}
}

func printBuildToolUsage(out io.Writer) {
	_, _ = fmt.Fprintln(out, "targ is a build-tool runner that discovers tagged commands and executes them.")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Usage: targ [FLAGS...] COMMAND [COMMAND_ARGS...]")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Flags:")
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "--no-cache", "disable cached build tool binaries")
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "--keep", "keep generated bootstrap file")
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "--timeout <duration>", "set execution timeout (e.g., 10m, 1h)")
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "--completion {bash|zsh|fish}", "print completion script for specified shell. Uses the current shell if none is")
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "", "specified. The output should be eval'd/sourced in the shell to enable completions.")
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "", "(e.g. 'targ --completion fish | source')")
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "--init [FILE]", "create a starter targets file (default: targs.go)")
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "--alias NAME \"CMD\" [FILE]", "add a shell command target to a targets file")
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "", "(auto-creates targs.go if no targets exist)")
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "--help", "Print help information")
}

func printCommandSummaries(out io.Writer, summaries []commandSummary) {
	if len(summaries) == 0 {
		_, _ = fmt.Fprintln(out, "    (none)")
		return
	}
	// Find max name length for alignment
	maxLen := 0
	for _, summary := range summaries {
		if len(summary.Name) > maxLen {
			maxLen = len(summary.Name)
		}
	}
	if maxLen < 10 {
		maxLen = 10
	}
	format := fmt.Sprintf("    %%-%ds %%s\n", maxLen+2)

	for _, summary := range summaries {
		if summary.Description != "" {
			_, _ = fmt.Fprintf(out, format, summary.Name, summary.Description)
		} else {
			_, _ = fmt.Fprintf(out, "    %s\n", summary.Name)
		}
	}
}

// printMultiModuleHelp prints aggregated help for all modules.
func printMultiModuleHelp(registry []moduleRegistry) {
	fmt.Println("targ is a build-tool runner that discovers tagged commands and executes them.")
	fmt.Println()
	fmt.Println("Usage: targ [FLAGS...] COMMAND [COMMAND_ARGS...]")
	fmt.Println()
	fmt.Println("Commands:")

	// Collect all commands and sort by name
	type cmdEntry struct {
		name        string
		description string
	}
	var allCmds []cmdEntry
	for _, reg := range registry {
		for _, cmd := range reg.Commands {
			allCmds = append(allCmds, cmdEntry{cmd.Name, cmd.Description})
		}
	}
	sort.Slice(allCmds, func(i, j int) bool {
		return allCmds[i].name < allCmds[j].name
	})

	// Find max command name length for alignment
	maxLen := 0
	for _, cmd := range allCmds {
		if len(cmd.name) > maxLen {
			maxLen = len(cmd.name)
		}
	}
	// Ensure minimum width and add padding
	if maxLen < 10 {
		maxLen = 10
	}
	format := fmt.Sprintf("    %%-%ds %%s\n", maxLen+2)

	for _, cmd := range allCmds {
		fmt.Printf(format, cmd.name, cmd.description)
	}

	fmt.Println()
	fmt.Println("More info: https://github.com/toejough/targ#readme")
}

// projectCacheDir returns a project-specific subdirectory within the targ cache.
// Uses a hash of the project path to isolate projects.
func projectCacheDir(projectPath string) string {
	hash := sha256.Sum256([]byte(projectPath))
	return filepath.Join(targCacheDir(), fmt.Sprintf("%x", hash[:8]))
}

// queryModuleCommands queries a module binary for its available commands.
func queryModuleCommands(binaryPath string) ([]commandInfo, error) {
	cmd := exec.Command(binaryPath, "__list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running __list: %w", err)
	}

	var result listOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parsing __list output: %w", err)
	}

	return result.Commands, nil
}

// replaceModuleName replaces the module declaration in go.mod content.
func replaceModuleName(content, newName string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			lines[i] = "module " + newName
			break
		}
	}
	return strings.Join(lines, "\n")
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

// setupIsolatedGoMod sets up the go.mod for the isolated build directory.
// It walks up from startDir to find the nearest go.mod, copies it with the
// module name replaced to targ.build.local, and copies go.sum if present.
func setupIsolatedGoMod(tmpDir, startDir string) error {
	// Walk up to find go.mod
	goModPath := ""
	current := startDir
	for {
		candidate := filepath.Join(current, "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			goModPath = candidate
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	destPath := filepath.Join(tmpDir, "go.mod")
	if goModPath != "" {
		// Copy existing go.mod and replace module name
		content, err := os.ReadFile(goModPath)
		if err != nil {
			return fmt.Errorf("reading go.mod: %w", err)
		}
		newContent := replaceModuleName(string(content), "targ.build.local")
		if err := os.WriteFile(destPath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("writing go.mod: %w", err)
		}

		// Also copy go.sum if exists
		sumPath := filepath.Join(filepath.Dir(goModPath), "go.sum")
		if sumContent, err := os.ReadFile(sumPath); err == nil {
			if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), sumContent, 0644); err != nil {
				return fmt.Errorf("writing go.sum: %w", err)
			}
		}
	} else {
		// Create minimal go.mod
		content := "module targ.build.local\n\ngo 1.21\n"
		if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing go.mod: %w", err)
		}
	}

	return nil
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

// stripBuildTag removes the //go:build targ line from Go source code.
// This allows files to compile without needing the -tags targ flag.
func stripBuildTag(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip //go:build targ lines (with possible variations)
		if strings.HasPrefix(trimmed, "//go:build") && strings.Contains(trimmed, "targ") {
			continue
		}
		// Also skip old-style // +build targ lines
		if strings.HasPrefix(trimmed, "// +build") && strings.Contains(trimmed, "targ") {
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func subcommandTag(fieldName string, segment string) string {
	if camelToKebab(fieldName) == segment {
		return `targ:"subcommand"`
	}
	return fmt.Sprintf(`targ:"subcommand,name=%s"`, segment)
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

// toExportedName converts a name like "tidy" or "run-tests" to "Tidy" or "RunTests".
func toExportedName(name string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range name {
		if r == '-' || r == '_' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func touchFile(path string) error {
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		return err
	}
	return nil
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

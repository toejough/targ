// Package main provides the targ CLI tool for running targ-based build scripts.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/toejough/targ/buildtool"
)

func main() {
	os.Exit(runMain())
}

// unexported constants.
const (
	bootstrapTemplate = `
package main

import (
	"github.com/toejough/targ"
	"github.com/toejough/targ/sh"
{{- range .BlankImports }}
	_ "{{ . }}"
{{- end }}
)

func main() {
	sh.EnableCleanup()
	targ.ExecuteRegisteredWithOptions(targ.RunOptions{
		Description: {{ printf "%q" .Description }},
	})
}
`
	commandNamePadding     = 2 // Padding after command name column
	completeCommand        = "__complete"
	defaultPackageName     = "main" // default package name for created targ files
	defaultTargModulePath  = "github.com/toejough/targ"
	filePermissionsForCode = 0o644 // standard file permissions for created source files
	helpIndentWidth        = 4     // Leading spaces in help output
	isolatedModuleName     = "targ.build.local"
	minArgsForCompletion   = 2      // Minimum args for __complete (binary + arg)
	minCommandNameWidth    = 10     // Minimum column width for command names in help output
	pkgNameMain            = "main" // package main check for targ files
	targLocalModule        = "targ.local"
)

// unexported variables.
var (
	errCreateUsage = errors.New(
		"usage: targ --create [group...] <name> [--deps ...] [--cache ...] \"<shell-command>\"",
	)
	errDuplicateTarget    = errors.New("target already exists")
	errFlagRemoved        = errors.New("flag has been removed; use --create instead")
	errInvalidPackagePath = errors.New(
		"invalid package path: must be a module path (e.g., github.com/user/repo)",
	)
	errInvalidUTF8Path        = errors.New("invalid utf-8 path in tagged file")
	errModulePathNotFound     = errors.New("module path not found")
	errNoExplicitRegistration = errors.New(
		"package does not use explicit registration (targ.Register in init)",
	)
	errPackageAlreadySynced  = errors.New("package already synced")
	errPackageMainNotAllowed = errors.New("targ files cannot use 'package main'")
	errSyncUsage             = errors.New("usage: targ --sync <package-path>")
	errUnknownCommand        = errors.New("unknown command")
	errUnknownCreateFlag     = errors.New("unknown flag")
	validTargetNameRe        = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$`)
)

type bootstrapBuilder struct {
	moduleRoot          string
	modulePath          string
	explicitRegPackages []string // import paths for packages using targ.Register()
}

func (b *bootstrapBuilder) buildResult() bootstrapData {
	return bootstrapData{
		BlankImports: b.explicitRegPackages,
		Description:  "Targ discovers and runs build targets you write in Go.",
	}
}

func (b *bootstrapBuilder) computeImportPath(dir string) string {
	rel, err := filepath.Rel(b.moduleRoot, dir)
	if err != nil || rel == "." {
		return b.modulePath
	}

	return b.modulePath + "/" + filepath.ToSlash(rel)
}

func (b *bootstrapBuilder) processPackage(info buildtool.PackageInfo) error {
	if !info.UsesExplicitRegistration {
		return fmt.Errorf("%w: %s", errNoExplicitRegistration, info.Package)
	}

	importPath := b.computeImportPath(info.Dir)
	b.explicitRegPackages = append(b.explicitRegPackages, importPath)

	return nil
}

type bootstrapData struct {
	Description  string
	BlankImports []string // import paths for explicit registration packages
}

type buildContext struct {
	usingFallback bool
	buildRoot     string
	importRoot    string
}

type commandInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type contentPatch struct {
	old string
	new string
}

type createOptions struct {
	Path     []string // Group path components (e.g., ["dev", "lint"] for "dev lint fast")
	Name     string   // Target name (e.g., "fast")
	ShellCmd string   // Shell command to execute
	Deps     []string // Dependency target names
	Cache    []string // Cache patterns
}

type groupModifications struct {
	newCode        string         // New group declarations to append
	contentPatches []contentPatch // Modifications to existing content
}

type listOutput struct {
	Commands []commandInfo `json:"commands"`
}

type moduleBootstrap struct {
	code     []byte
	cacheKey string
}

type moduleRegistry struct {
	BinaryPath string
	ModuleRoot string
	ModulePath string
	Commands   []commandInfo
}

type moduleTargets struct {
	ModuleRoot string
	ModulePath string
	Packages   []buildtool.PackageInfo
}

type namespaceNode struct {
	Name     string
	File     string
	Children map[string]*namespaceNode
	TypeName string
	VarName  string
}

// canCompress returns true if this node should be compressed (skipped in output).
func (n *namespaceNode) canCompress() bool {
	return len(n.Children) == 1 && n.File == ""
}

// collectCompressedPaths walks the tree and collects compressed paths.
// Assumes Children is always non-nil (enforced by insertPath and constructors).
func (n *namespaceNode) collectCompressedPaths(
	out map[string][]string,
	prefix []string,
	isRoot bool,
) {
	// Skip single-child intermediate nodes (compression)
	if !isRoot && n.canCompress() {
		for _, child := range n.Children {
			child.collectCompressedPaths(out, prefix, false)
		}

		return
	}

	if !isRoot {
		prefix = append(prefix, n.Name)
	}

	if n.File != "" {
		out[n.File] = append([]string(nil), prefix...)
	}

	for _, child := range n.sortedChildren() {
		child.collectCompressedPaths(out, prefix, false)
	}
}

// insertPath inserts a file path into the namespace tree.
func (n *namespaceNode) insertPath(file string, parts []string) {
	node := n
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

// sortedChildren returns children in sorted name order.
func (n *namespaceNode) sortedChildren() []*namespaceNode {
	names := make([]string, 0, len(n.Children))
	for name := range n.Children {
		names = append(names, name)
	}

	sort.Strings(names)

	children := make([]*namespaceNode, 0, len(names))
	for _, name := range names {
		if child := n.Children[name]; child != nil {
			children = append(children, child)
		}
	}

	return children
}

type osFileSystem struct{}

func (osFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(name)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", name, err)
	}

	return entries, nil
}

//nolint:gosec // build tool reads user source files by design
func (osFileSystem) ReadFile(name string) ([]byte, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", name, err)
	}

	return data, nil
}

func (osFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	err := os.WriteFile(name, data, perm)
	if err != nil {
		return fmt.Errorf("writing file %s: %w", name, err)
	}

	return nil
}

type syncOptions struct {
	PackagePath string // Module path to sync (e.g., "github.com/foo/bar")
}

type targDependency struct {
	ModulePath string
	Version    string
	ReplaceDir string
}

type targRunner struct {
	binArg        string
	args          []string
	errOut        io.Writer
	startDir      string
	noBinaryCache bool
}

func (r *targRunner) buildAndRun(
	importRoot, binaryPath, targBinName string,
	bootstrapCode []byte,
) int {
	return r.buildAndRunWithOptions(importRoot, binaryPath, targBinName, bootstrapCode, false)
}

func (r *targRunner) buildAndRunIsolated(
	isolatedDir, binaryPath, targBinName string,
	bootstrapCode []byte,
) int {
	return r.buildAndRunWithOptions(isolatedDir, binaryPath, targBinName, bootstrapCode, true)
}

func (r *targRunner) buildAndRunWithOptions(
	buildDir, binaryPath, targBinName string,
	bootstrapCode []byte,
	isolated bool,
) int {
	bootstrapDir := r.resolveBootstrapDir(buildDir, isolated)

	tempFile, cleanupTemp, err := writeBootstrapFile(bootstrapDir, bootstrapCode)
	if err != nil {
		r.logError("Error writing bootstrap file", err)
		return r.exitWithCleanup(1)
	}

	defer func() { _ = cleanupTemp() }()

	err = r.executeBuild(buildDir, binaryPath, tempFile, isolated)
	if err != nil {
		r.logError("Error building command", err)
		return r.exitWithCleanup(1)
	}

	return r.executeBuiltBinary(binaryPath, targBinName)
}

func (r *targRunner) discoverPackages() ([]buildtool.PackageInfo, error) {
	infos, err := buildtool.Discover(osFileSystem{}, buildtool.Options{
		StartDir: r.startDir,
		BuildTag: "targ",
	})
	if err != nil {
		return nil, fmt.Errorf("error discovering commands: %w", err)
	}

	// Validate no package main in targ files
	for _, info := range infos {
		if info.Package == pkgNameMain {
			return nil, fmt.Errorf(
				"%w (found in %s); use a named package instead, e.g., 'package targets' or 'package dev'",
				errPackageMainNotAllowed,
				info.Dir,
			)
		}
	}

	return infos, nil
}

func (r *targRunner) executeBuild(buildDir, binaryPath, tempFile string, isolated bool) error {
	buildArgs := []string{"build", "-tags", "targ", "-o", binaryPath}
	if isolated {
		buildArgs = append(buildArgs, "-mod=mod")
	}

	buildArgs = append(buildArgs, tempFile)

	//nolint:gosec // build tool runs go build by design
	buildCmd := exec.CommandContext(context.Background(), "go", buildArgs...)

	var buildOutput bytes.Buffer

	buildCmd.Stdout = io.Discard
	buildCmd.Stderr = &buildOutput
	buildCmd.Dir = buildDir

	err := buildCmd.Run()
	if err != nil {
		if buildOutput.Len() > 0 {
			_, _ = fmt.Fprint(r.errOut, buildOutput.String())
		}

		return fmt.Errorf("running go build: %w", err)
	}

	return nil
}

func (r *targRunner) executeBuiltBinary(binaryPath, targBinName string) int {
	//nolint:gosec // build tool runs compiled binary by design
	cmd := exec.CommandContext(context.Background(), binaryPath, r.args...)

	cmd.Env = append(os.Environ(), "TARG_BIN_NAME="+targBinName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = r.errOut
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		return r.exitWithCleanup(1)
	}

	return 0
}

func (r *targRunner) exitWithCleanup(code int) int {
	return code
}

//nolint:funlen // Validation logic is straightforward but verbose
func (r *targRunner) handleCreateFlag(args []string) (exitCode int, done bool) {
	// Parse the create arguments
	opts, err := parseCreateArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1, true
	}

	// Validate target name (kebab-case)
	if !isValidTargetName(opts.Name) {
		fmt.Fprintf(
			os.Stderr,
			"invalid target name %q: must be lowercase letters, numbers, and hyphens\n",
			opts.Name,
		)

		return 1, true
	}

	// Validate path components (all must be valid kebab-case names)
	for _, p := range opts.Path {
		if !isValidTargetName(p) {
			fmt.Fprintf(
				os.Stderr,
				"invalid group name %q: must be lowercase letters, numbers, and hyphens\n",
				p,
			)

			return 1, true
		}
	}

	// Validate dependency names
	for _, dep := range opts.Deps {
		if !isValidTargetName(dep) {
			fmt.Fprintf(
				os.Stderr,
				"invalid dependency name %q: must be lowercase letters, numbers, and hyphens\n",
				dep,
			)

			return 1, true
		}
	}

	// Get working directory (startDir may not be set yet)
	startDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting working directory: %v\n", err)
		return 1, true
	}

	// Find or create targ file
	targFile, err := findOrCreateTargFile(startDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding/creating targ file: %v\n", err)
		return 1, true
	}

	// Add the target to the file
	err = addTargetToFileWithOptions(targFile, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error adding target: %v\n", err)
		return 1, true
	}

	// Build display name
	fullPath := append(opts.Path, opts.Name) //nolint:gocritic // intentional copy
	displayName := strings.Join(fullPath, "/")
	fmt.Printf("Created target %q in %s\n", displayName, targFile)

	return 0, true
}

func (r *targRunner) handleEarlyFlags() (exitCode int, done bool) {
	for i, arg := range r.args {
		if isRemovedFlag(arg) {
			fmt.Fprintf(os.Stderr, "%s: %v\n", arg, errFlagRemoved)
			return 1, true
		}

		if isCreateFlag(arg) {
			return r.handleCreateFlag(r.args[i+1:])
		}

		if isSyncFlag(arg) {
			return r.handleSyncFlag(r.args[i+1:])
		}
	}

	return 0, false
}

func (r *targRunner) handleIsolatedModule(infos []buildtool.PackageInfo) int {
	// Create isolated build directory with copied files
	dep := resolveTargDependency()

	isolatedDir, cleanup, err := createIsolatedBuildDir(infos, r.startDir, dep)
	if err != nil {
		r.logError("Error creating isolated build directory", err)
		return r.exitWithCleanup(1)
	}

	defer cleanup()

	// Remap package infos to point to isolated directory
	isolatedInfos, _, err := remapPackageInfosToIsolated(infos, r.startDir, isolatedDir)
	if err != nil {
		r.logError("Error remapping package infos", err)
		return r.exitWithCleanup(1)
	}

	bootstrap, err := r.prepareBootstrap(
		isolatedInfos,
		isolatedDir,
		isolatedModuleName,
	)
	if err != nil {
		r.logError("", err)
		return r.exitWithCleanup(1)
	}

	// Use startDir for cache key computation to enable caching across runs
	binaryPath, err := r.setupBinaryPath(r.startDir, bootstrap.cacheKey)
	if err != nil {
		r.logError("Error creating cache directory", err)
		return r.exitWithCleanup(1)
	}

	targBinName := extractBinName(r.binArg)

	// Try cached binary first
	if !r.noBinaryCache {
		if code, ran := r.tryRunCached(binaryPath, targBinName); ran {
			return code
		}
	}

	// Build and run from isolated directory
	return r.buildAndRunIsolated(isolatedDir, binaryPath, targBinName, bootstrap.code)
}

func (r *targRunner) handleMultiModule(
	moduleGroups []moduleTargets,
	helpRequested, helpTargets bool,
) int {
	registry, err := buildMultiModuleBinaries(
		moduleGroups,
		r.startDir,
		r.noBinaryCache,
		r.errOut,
	)
	if err != nil {
		r.logError("Error building module binaries", err)
		return r.exitWithCleanup(1)
	}

	if helpRequested && !helpTargets {
		printMultiModuleHelp(registry)
		return 0
	}

	err = dispatchCommand(registry, r.args, r.errOut, r.binArg)
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}

		r.logError("Error", err)

		return 1
	}

	return 0
}

func (r *targRunner) handleNoTargets() int {
	if len(r.args) > 0 && r.args[0] == completeCommand {
		printNoTargetsCompletion(r.args)
		return 0
	}

	r.logError("Error: no target files found", nil)

	return r.exitWithCleanup(1)
}

func (r *targRunner) handleSingleModule(infos []buildtool.PackageInfo) int {
	filePaths := collectFilePaths(infos)

	if len(filePaths) == 0 {
		return r.handleNoTargets()
	}

	_, _, moduleFound, err := findModuleForPath(filePaths[0])
	if err != nil {
		r.logError("Error checking for module", err)
		return r.exitWithCleanup(1)
	}

	// Use isolated build when no module found
	if !moduleFound {
		return r.handleIsolatedModule(infos)
	}

	importRoot, modulePath, _, err := findModuleForPath(filePaths[0])
	if err != nil {
		r.logError("Error checking for module", err)
		return r.exitWithCleanup(1)
	}

	bootstrap, err := r.prepareBootstrap(infos, importRoot, modulePath)
	if err != nil {
		r.logError("", err)
		return r.exitWithCleanup(1)
	}

	binaryPath, err := r.setupBinaryPath(importRoot, bootstrap.cacheKey)
	if err != nil {
		r.logError("Error creating cache directory", err)
		return r.exitWithCleanup(1)
	}

	targBinName := extractBinName(r.binArg)

	// Try cached binary first
	if !r.noBinaryCache {
		if code, ran := r.tryRunCached(binaryPath, targBinName); ran {
			return code
		}
	}

	// Build and run
	return r.buildAndRun(importRoot, binaryPath, targBinName, bootstrap.code)
}

func (r *targRunner) handleSyncFlag(args []string) (exitCode int, done bool) {
	// Parse the sync arguments
	opts, err := parseSyncArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1, true
	}

	// Get working directory
	startDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting working directory: %v\n", err)
		return 1, true
	}

	// Find or create targ file
	targFile, err := findOrCreateTargFile(startDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding/creating targ file: %v\n", err)
		return 1, true
	}

	// Check if import already exists
	exists, err := checkImportExists(targFile, opts.PackagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error checking imports: %v\n", err)
		return 1, true
	}

	if exists {
		fmt.Fprintf(os.Stderr, "%v: %s\n", errPackageAlreadySynced, opts.PackagePath)
		return 1, true
	}

	// Fetch the package
	err = fetchPackage(opts.PackagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch package: %v\n", err)
		return 1, true
	}

	// Add the import to the targ file
	err = addImportToTargFile(targFile, opts.PackagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error adding import: %v\n", err)
		return 1, true
	}

	fmt.Printf("Synced package %q to %s\n", opts.PackagePath, targFile)

	return 0, true
}

func (r *targRunner) logError(prefix string, err error) {
	switch {
	case prefix != "" && err != nil:
		_, _ = fmt.Fprintf(r.errOut, "%s: %v\n", prefix, err)
	case prefix != "":
		_, _ = fmt.Fprintln(r.errOut, prefix)
	case err != nil:
		_, _ = fmt.Fprintf(r.errOut, "%v\n", err)
	}
}

func (r *targRunner) prepareBootstrap(
	infos []buildtool.PackageInfo,
	importRoot, modulePath string,
) (moduleBootstrap, error) {
	data, err := buildBootstrapData(infos, importRoot, modulePath)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error preparing bootstrap: %w", err)
	}

	tmpl := template.Must(template.New("main").Parse(bootstrapTemplate))

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, data)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error generating code: %w", err)
	}

	taggedFiles, err := buildtool.TaggedFiles(osFileSystem{}, buildtool.Options{
		StartDir: r.startDir,
		BuildTag: "targ",
	})
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error gathering tagged files: %w", err)
	}

	moduleFiles, err := collectModuleFiles(importRoot)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error gathering module files: %w", err)
	}

	cacheInputs := slices.Concat(taggedFiles, moduleFiles)

	cacheKey, err := computeCacheKey(modulePath, importRoot, "targ", buf.Bytes(), cacheInputs)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error computing cache key: %w", err)
	}

	return moduleBootstrap{code: buf.Bytes(), cacheKey: cacheKey}, nil
}

func (r *targRunner) resolveBootstrapDir(buildDir string, isolated bool) string {
	if isolated {
		return buildDir
	}

	projCache := projectCacheDir(buildDir)

	return filepath.Join(projCache, "tmp")
}

func (r *targRunner) run() int {
	// Handle removed flags early (--init, --alias, --move)
	if code, done := r.handleEarlyFlags(); done {
		return code
	}

	// Setup quiet mode for completion
	if len(r.args) > 0 && r.args[0] == completeCommand {
		r.errOut = io.Discard
	}

	helpRequested, helpTargets := parseHelpRequest(r.args)
	r.noBinaryCache, r.args = extractTargFlags(r.args)

	var err error

	r.startDir, err = os.Getwd()
	if err != nil {
		r.logError("Error resolving working directory", err)
		return 1
	}

	// Discover targ packages
	infos, err := r.discoverPackages()
	if err != nil {
		r.logError("", err)
		return 1
	}

	// Group packages by module
	moduleGroups, err := groupByModule(infos, r.startDir)
	if err != nil {
		r.logError("Error grouping packages by module", err)
		return r.exitWithCleanup(1)
	}

	// Handle multi-module cases
	if len(moduleGroups) > 1 {
		return r.handleMultiModule(moduleGroups, helpRequested, helpTargets)
	}

	// Single module case
	return r.handleSingleModule(infos)
}

func (r *targRunner) setupBinaryPath(importRoot, cacheKey string) (string, error) {
	projCache := projectCacheDir(importRoot)

	cacheDir := filepath.Join(projCache, "bin")

	//nolint:gosec,mnd // standard cache directory permissions
	err := os.MkdirAll(cacheDir, 0o755)
	if err != nil {
		return "", fmt.Errorf("creating cache directory: %w", err)
	}

	return filepath.Join(cacheDir, "targ_"+cacheKey), nil
}

func (r *targRunner) tryRunCached(binaryPath, targBinName string) (exitCode int, ran bool) {
	info, err := os.Stat(binaryPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return 0, false
	}

	//nolint:gosec // build tool runs cached binary by design
	cmd := exec.CommandContext(context.Background(), binaryPath, r.args...)

	cmd.Env = append(os.Environ(), "TARG_BIN_NAME="+targBinName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = r.errOut
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return r.exitWithCleanup(exitErr.ExitCode()), true
		}

		r.logError("Error running command", err)

		return r.exitWithCleanup(1), true
	}

	return 0, true
}

// addImportToTargFile adds a blank import for the given package to the targ file.
func addImportToTargFile(path, packagePath string) error {
	//nolint:gosec // build tool reads user source files by design
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	// Add the blank import
	importSpec := &ast.ImportSpec{
		Name: ast.NewIdent("_"),
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: strconv.Quote(packagePath),
		},
	}

	// Find or create import declaration
	var importDecl *ast.GenDecl

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if ok && genDecl.Tok == token.IMPORT {
			importDecl = genDecl

			break
		}
	}

	if importDecl != nil {
		// Add to existing import block
		importDecl.Specs = append(importDecl.Specs, importSpec)
	} else {
		// Create new import declaration
		importDecl = &ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: []ast.Spec{importSpec},
		}
		// Insert after package declaration
		file.Decls = append([]ast.Decl{importDecl}, file.Decls...)
	}

	// Format and write back
	var buf bytes.Buffer

	err = format.Node(&buf, fset, file)
	if err != nil {
		return fmt.Errorf("formatting file: %w", err)
	}

	err = os.WriteFile(path, buf.Bytes(), filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// addTargetToFile adds a target variable to an existing targ file.
func addTargetToFile(path, name, shellCmd string) error {
	return addTargetToFileWithOptions(path, createOptions{
		Name:     name,
		ShellCmd: shellCmd,
	})
}

// addTargetToFileWithOptions adds a target with full options to an existing targ file.
func addTargetToFileWithOptions(path string, opts createOptions) error {
	//nolint:gosec // build tool reads user source files by design
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Build the full variable name including path
	fullPath := append(opts.Path, opts.Name) //nolint:gocritic // intentional copy
	varName := pathToPascal(fullPath)

	// Check if target already exists
	if strings.Contains(string(content), fmt.Sprintf("var %s = ", varName)) {
		return fmt.Errorf("%w: %s", errDuplicateTarget, strings.Join(fullPath, "/"))
	}

	// Build the target code
	var code strings.Builder

	// Comment
	code.WriteString(fmt.Sprintf("\n// %s runs: %s\n", varName, opts.ShellCmd))

	// Start variable declaration
	escapedCmd := escapeGoString(opts.ShellCmd)
	code.WriteString(fmt.Sprintf("var %s = targ.Targ(%q)", varName, escapedCmd))

	// Add Name() - use just the target name, not the full path
	code.WriteString(fmt.Sprintf(".Name(%q)", opts.Name))

	// Add Deps() if specified
	if len(opts.Deps) > 0 {
		depVars := make([]string, len(opts.Deps))
		for i, dep := range opts.Deps {
			depVars[i] = kebabToPascal(dep)
		}

		code.WriteString(fmt.Sprintf(".Deps(%s)", strings.Join(depVars, ", ")))
	}

	// Add Cache() if specified
	if len(opts.Cache) > 0 {
		patterns := make([]string, len(opts.Cache))
		for i, p := range opts.Cache {
			patterns[i] = fmt.Sprintf("%q", p)
		}

		code.WriteString(fmt.Sprintf(".Cache(%s)", strings.Join(patterns, ", ")))
	}

	code.WriteString("\n")

	// Generate group modifications (new groups and patches to existing groups)
	groupMods := generateGroupModifications(opts.Path, varName, string(content))

	// Apply patches to existing content
	modifiedContent := string(content)
	for _, patch := range groupMods.contentPatches {
		modifiedContent = strings.Replace(modifiedContent, patch.old, patch.new, 1)
	}

	// Append new code
	newContent := modifiedContent + code.String() + groupMods.newCode

	err = os.WriteFile(path, []byte(newContent), filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// buildAndQueryBinary builds the binary and queries its commands.
func buildAndQueryBinary(
	ctx buildContext,
	_ moduleTargets,
	dep targDependency,
	binaryPath string,
	bootstrap moduleBootstrap,
	errOut io.Writer,
) ([]commandInfo, error) {
	bootstrapDir := filepath.Join(projectCacheDir(ctx.importRoot), "tmp")
	if ctx.usingFallback {
		bootstrapDir = filepath.Join(ctx.buildRoot, "tmp")
	}

	tempFile, cleanupTemp, err := writeBootstrapFile(bootstrapDir, bootstrap.code)
	if err != nil {
		return nil, fmt.Errorf("writing bootstrap file: %w", err)
	}

	defer func() { _ = cleanupTemp() }()

	ensureTargDependency(dep, ctx.importRoot)

	err = runGoBuild(ctx, binaryPath, tempFile, errOut)
	if err != nil {
		return nil, err
	}

	cmds, err := queryModuleCommands(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("querying commands: %w", err)
	}

	return cmds, nil
}

func buildBootstrapData(
	infos []buildtool.PackageInfo,
	moduleRoot string,
	modulePath string,
) (bootstrapData, error) {
	builder := newBootstrapBuilder(moduleRoot, modulePath)

	for _, info := range infos {
		err := builder.processPackage(info)
		if err != nil {
			return bootstrapData{}, err
		}
	}

	return builder.buildResult(), nil
}

// buildModuleBinary builds a single module's binary and queries its commands.
func buildModuleBinary(
	mt moduleTargets,
	startDir string,
	dep targDependency,
	noBinaryCache bool,
	errOut io.Writer,
) (moduleRegistry, error) {
	reg := moduleRegistry{
		ModuleRoot: mt.ModuleRoot,
		ModulePath: mt.ModulePath,
	}

	err := validateNoPackageMain(mt)
	if err != nil {
		return reg, err
	}

	buildCtx, err := prepareBuildContext(mt, startDir, dep)
	if err != nil {
		return reg, err
	}

	bootstrap, err := generateModuleBootstrap(mt, buildCtx.importRoot)
	if err != nil {
		return reg, err
	}

	binaryPath, err := setupBinaryPath(buildCtx.importRoot, mt.ModulePath, bootstrap)
	if err != nil {
		return reg, err
	}

	reg.BinaryPath = binaryPath

	if !noBinaryCache {
		if cmds, ok := tryCachedBinary(binaryPath); ok {
			reg.Commands = cmds
			return reg, nil
		}
	}

	cmds, err := buildAndQueryBinary(
		buildCtx,
		mt,
		dep,
		binaryPath,
		bootstrap,
		errOut,
	)
	if err != nil {
		return reg, err
	}

	reg.Commands = cmds

	return reg, nil
}

// buildMultiModuleBinaries builds a binary for each module group and returns the registry.
func buildMultiModuleBinaries(
	moduleGroups []moduleTargets,
	startDir string,
	noBinaryCache bool,
	errOut io.Writer,
) ([]moduleRegistry, error) {
	registry := make([]moduleRegistry, 0, len(moduleGroups))

	dep := resolveTargDependency()

	for _, mt := range moduleGroups {
		reg, err := buildModuleBinary(mt, startDir, dep, noBinaryCache, errOut)
		if err != nil {
			return nil, fmt.Errorf("building module %s: %w", mt.ModulePath, err)
		}

		registry = append(registry, reg)
	}

	return registry, nil
}

func buildSourceRoot() (string, bool) {
	_, file, _, ok := runtime.Caller(0)
	if !ok || file == "" {
		return "", false
	}

	dir := filepath.Dir(file)
	for {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
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

// checkImportExists checks if a blank import for the given package already exists in the file.
func checkImportExists(path, packagePath string) (bool, error) {
	//nolint:gosec // build tool reads user source files by design
	content, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("reading file: %w", err)
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, content, parser.ImportsOnly)
	if err != nil {
		return false, fmt.Errorf("parsing file: %w", err)
	}

	for _, imp := range file.Imports {
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}

		if importPath == packagePath {
			return true, nil
		}
	}

	return false, nil
}

// cleanupStaleModSymlinks removes stale go.mod/go.sum symlinks from before the fix.
func cleanupStaleModSymlinks(root string) {
	for _, name := range []string{"go.mod", "go.sum"} {
		dst := filepath.Join(root, name)
		if symlinkExists(dst) {
			_ = os.Remove(dst)
		}
	}
}

func collectFilePaths(infos []buildtool.PackageInfo) []string {
	var filePaths []string

	for _, info := range infos {
		for _, f := range info.Files {
			filePaths = append(filePaths, f.Path)
		}
	}

	return filePaths
}

func collectModuleFiles(moduleRoot string) ([]buildtool.TaggedFile, error) {
	var files []buildtool.TaggedFile

	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking directory: %w", err)
		}

		if entry.IsDir() {
			return skipIfVendorOrGit(entry.Name())
		}

		if !isIncludableModuleFile(entry.Name()) {
			return nil
		}

		//nolint:gosec // build tool reads source files by design
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", path, err)
		}

		files = append(files, buildtool.TaggedFile{
			Path:    path,
			Content: data,
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking module directory: %w", err)
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
				return nil, fmt.Errorf("reading tagged file %s: %w", f.Path, err)
			}

			files = append(files, buildtool.TaggedFile{
				Path:    f.Path,
				Content: data,
			})
		}
	}

	return files, nil
}

func commonPrefix(a, b []string) []string {
	limit := min(len(b), len(a))

	for i := range limit {
		if a[i] != b[i] {
			return a[:i]
		}
	}

	return a[:limit]
}

func compressNamespacePaths(paths map[string][]string) map[string][]string {
	root := &namespaceNode{Children: make(map[string]*namespaceNode)}
	out := make(map[string][]string, len(paths))

	for file, parts := range paths {
		if len(parts) == 0 {
			out[file] = nil
			continue
		}

		root.insertPath(file, parts)
	}

	root.collectCompressedPaths(out, nil, true)

	return out
}

func computeCacheKey(
	modulePath, moduleRoot, buildTag string,
	bootstrap []byte,
	tagged []buildtool.TaggedFile,
) (string, error) {
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
			return "", fmt.Errorf("%w: %q", errInvalidUTF8Path, file.Path)
		}

		write("file:" + file.Path)
		hasher.Write(file.Content)
		hasher.Write([]byte{0})
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// computeModuleCacheKey computes the cache key for a module build.
func computeModuleCacheKey(mt moduleTargets, importRoot string, bootstrap []byte) (string, error) {
	taggedFiles, err := collectModuleTaggedFiles(mt)
	if err != nil {
		return "", fmt.Errorf("gathering tagged files: %w", err)
	}

	moduleFiles, err := collectModuleFiles(importRoot)
	if err != nil {
		return "", fmt.Errorf("gathering module files: %w", err)
	}

	cacheInputs := slices.Concat(taggedFiles, moduleFiles)

	cacheKey, err := computeCacheKey(mt.ModulePath, importRoot, "targ", bootstrap, cacheInputs)
	if err != nil {
		return "", fmt.Errorf("computing cache key: %w", err)
	}

	return cacheKey, nil
}

// copyFileStrippingTag copies a file to destPath, removing the //go:build targ line.
func copyFileStrippingTag(srcPath, destPath string) error {
	//nolint:gosec // build tool reads source files by design
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading source file: %w", err)
	}

	content := stripBuildTag(string(data))

	err = os.WriteFile(destPath, []byte(content), filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("writing destination file: %w", err)
	}

	return nil
}

// createGroupMemberPatch creates a patch to add a new member to an existing group.
// Returns nil if the member already exists in the group.
func createGroupMemberPatch(content, groupVarName, newMember string) *contentPatch {
	// Find the group declaration: var GroupName = targ.NewGroup("name", member1, member2)
	// We need to add newMember before the closing parenthesis

	// Find the start of the group declaration
	pattern := fmt.Sprintf("var %s = targ.NewGroup(", groupVarName)

	startIdx := strings.Index(content, pattern)
	if startIdx == -1 {
		return nil
	}

	// Find the closing parenthesis for this declaration
	// We need to handle nested parentheses (though unlikely in this context)
	parenStart := startIdx + len(pattern)
	parenCount := 1
	endIdx := -1

	for i := parenStart; i < len(content) && parenCount > 0; i++ {
		switch content[i] {
		case '(':
			parenCount++
		case ')':
			parenCount--
			if parenCount == 0 {
				endIdx = i
			}
		}
	}

	if endIdx == -1 {
		return nil
	}

	// Extract the current group declaration
	oldDecl := content[startIdx : endIdx+1]

	// Check if member already exists
	if strings.Contains(oldDecl, newMember) {
		return nil
	}

	// Create the new declaration by inserting the member before the closing paren
	newDecl := content[startIdx:endIdx] + ", " + newMember + ")"

	return &contentPatch{
		old: oldDecl,
		new: newDecl,
	}
}

// createIsolatedBuildDir creates an isolated build directory with targ files.
// Files are copied (with build tags stripped) preserving collapsed namespace paths.
// Returns the tmp directory path, the module path to use for imports, and a cleanup function.
func createIsolatedBuildDir(
	infos []buildtool.PackageInfo,
	startDir string,
	dep targDependency,
) (tmpDir string, cleanup func(), err error) {
	filePaths := collectFilePaths(infos)

	paths, err := namespacePaths(filePaths, startDir)
	if err != nil {
		return "", nil, fmt.Errorf("computing namespace paths: %w", err)
	}

	tmpDir, err = os.MkdirTemp("", "targ-build-")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp directory: %w", err)
	}

	cleanup = func() {
		_ = os.RemoveAll(tmpDir)
	}

	// Copy files using collapsed namespace paths
	for _, info := range infos {
		for _, f := range info.Files {
			collapsedPath := paths[f.Path]

			var targetDir string

			if len(collapsedPath) > 0 {
				// Use all but the last element (which is the filename stem)
				dirParts := collapsedPath[:len(collapsedPath)-1]
				// Add the package name as the final directory
				dirParts = append(dirParts, info.Package)
				targetDir = filepath.Join(tmpDir, filepath.Join(dirParts...))
			} else {
				targetDir = filepath.Join(tmpDir, info.Package)
			}

			//nolint:gosec,mnd // standard directory permissions
			err := os.MkdirAll(targetDir, 0o755)
			if err != nil {
				cleanup()
				return "", nil, fmt.Errorf("creating target directory: %w", err)
			}

			destPath := filepath.Join(targetDir, filepath.Base(f.Path))

			err = copyFileStrippingTag(f.Path, destPath)
			if err != nil {
				cleanup()
				return "", nil, fmt.Errorf("copying file %s: %w", f.Path, err)
			}
		}
	}

	// Create synthetic go.mod
	err = writeIsolatedGoMod(tmpDir, dep)
	if err != nil {
		cleanup()
		return "", nil, err
	}

	return tmpDir, cleanup, nil
}

// dispatchCommand finds the right binary for a command and executes it.
func dispatchCommand(
	registry []moduleRegistry,
	args []string,
	errOut io.Writer,
	binArg string,
) error {
	if isHelpRequest(args) {
		printMultiModuleHelp(registry)
		return nil
	}

	if len(args) > 0 && args[0] == completeCommand {
		return dispatchCompletion(registry, args)
	}

	cmdName := args[0]
	if binaryPath, ok := findCommandBinary(registry, cmdName); ok {
		return runModuleBinary(binaryPath, args, errOut, binArg)
	}

	_, _ = fmt.Fprintf(errOut, "Unknown command: %s\n", cmdName)

	printMultiModuleHelp(registry)

	return fmt.Errorf("%w: %s", errUnknownCommand, cmdName)
}

// dispatchCompletion handles completion requests by querying all binaries.
func dispatchCompletion(registry []moduleRegistry, args []string) error {
	if len(args) < minArgsForCompletion {
		return nil
	}

	// Query each binary for completions and aggregate
	seen := make(map[string]bool)

	for _, reg := range registry {
		//nolint:gosec // build tool runs module binaries by design
		cmd := exec.CommandContext(context.Background(), reg.BinaryPath, args...)

		output, err := cmd.Output()
		if err != nil {
			continue // Skip failed completions
		}

		for line := range strings.SplitSeq(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !seen[line] {
				seen[line] = true
				fmt.Println(line)
			}
		}
	}

	return nil
}

func ensureFallbackModuleRoot(startDir, modulePath string, dep targDependency) (string, error) {
	hash := sha256.Sum256([]byte(startDir))

	root := filepath.Join(projectCacheDir(startDir), "mod", hex.EncodeToString(hash[:8]))

	//nolint:gosec,mnd // standard cache directory permissions
	err := os.MkdirAll(root, 0o755)
	if err != nil {
		return "", fmt.Errorf("creating fallback module directory: %w", err)
	}

	err = linkModuleRoot(startDir, root)
	if err != nil {
		return "", err
	}

	err = writeFallbackGoMod(root, modulePath, dep)
	if err != nil {
		return "", err
	}

	err = touchFile(filepath.Join(root, "go.sum"))
	if err != nil {
		return "", err
	}

	return root, nil
}

// ensureTargDependency runs go get to ensure targ dependency is available.
func ensureTargDependency(dep targDependency, importRoot string) {
	//nolint:gosec // build tool runs go get by design
	getCmd := exec.CommandContext(context.Background(), "go", "get", dep.ModulePath)
	getCmd.Dir = importRoot
	getCmd.Stdout = io.Discard
	getCmd.Stderr = io.Discard
	_ = getCmd.Run()
}

// escapeGoString escapes a string for use in a Go string literal.
func escapeGoString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")

	return s
}

func extractBinName(binArg string) string {
	if binArg == "" {
		return "targ"
	}

	if idx := strings.LastIndex(binArg, "/"); idx != -1 {
		return binArg[idx+1:]
	}

	if idx := strings.LastIndex(binArg, "\\"); idx != -1 {
		return binArg[idx+1:]
	}

	return binArg
}

// extractTargFlags extracts targ-specific flags (--no-binary-cache) from args.
// Returns the flag value and remaining args to pass to the binary.
func extractTargFlags(args []string) (noBinaryCache bool, remaining []string) {
	remaining = make([]string, 0, len(args))

	for _, arg := range args {
		switch arg {
		case "--no-binary-cache":
			noBinaryCache = true
		case "--no-cache":
			// Deprecated: use --no-binary-cache instead
			fmt.Fprintln(
				os.Stderr,
				"warning: --no-cache is deprecated, use --no-binary-cache instead",
			)

			noBinaryCache = true
		default:
			remaining = append(remaining, arg)
		}
	}

	return noBinaryCache, remaining
}

// fetchPackage runs go get to fetch a package.
func fetchPackage(packagePath string) error {
	cmd := exec.CommandContext(context.Background(), "go", "get", packagePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("go get %s: %w", packagePath, err)
	}

	return nil
}

// findCommandBinary finds the binary path for a command in the registry.
func findCommandBinary(registry []moduleRegistry, cmdName string) (string, bool) {
	for _, reg := range registry {
		for _, cmd := range reg.Commands {
			if cmd.Name == cmdName || strings.HasPrefix(cmd.Name, cmdName+" ") {
				return reg.BinaryPath, true
			}
		}
	}

	return "", false
}

// findModCacheDir finds the cached module directory for a clean version.
func findModCacheDir(modulePath, version string) (string, bool) {
	if !isCleanVersion(version) {
		return "", false
	}

	modCache, err := goEnv("GOMODCACHE")
	if err != nil || modCache == "" {
		return "", false
	}

	candidate := filepath.Join(modCache, modulePath+"@"+version)

	statInfo, err := os.Stat(candidate)
	if err == nil && statInfo.IsDir() {
		return candidate, true
	}

	return "", false
}

// findModuleForPath walks up from the given path to find the nearest go.mod.
// Returns the module root directory, module path, whether found, and any error.
func findModuleForPath(path string) (string, string, bool, error) {
	// Start from the directory containing the path
	dir := path

	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
	}

	for {
		modPath := filepath.Join(dir, "go.mod")

		//nolint:gosec // build tool reads go.mod files by design
		data, err := os.ReadFile(modPath)
		if err == nil {
			modulePath := parseModulePath(string(data))
			if modulePath == "" {
				return "", "", true, fmt.Errorf("%w: %s", errModulePathNotFound, modPath)
			}

			return dir, modulePath, true, nil
		}

		if !os.IsNotExist(err) {
			return "", "", false, fmt.Errorf("reading go.mod: %w", err)
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

// findOrCreateTargFile finds an existing targ file in the current directory or creates a new one.
func findOrCreateTargFile(startDir string) (string, error) {
	// Look for existing targ files in the current directory
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return "", fmt.Errorf("reading directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// Check if it has the targ build tag
		path := filepath.Join(startDir, name)
		if hasTargBuildTag(path) {
			return path, nil
		}
	}

	// No existing targ file found, create a new one
	targFile := filepath.Join(startDir, "targs.go")
	pkgName := filepath.Base(startDir)
	// Sanitize package name (remove invalid characters)
	pkgName = strings.ReplaceAll(pkgName, "-", "")

	pkgName = strings.ReplaceAll(pkgName, ".", "")
	if pkgName == "" {
		pkgName = defaultPackageName
	}

	content := fmt.Sprintf(`//go:build targ

package %s

import "github.com/toejough/targ"

// Ensure targ import is used
var _ = targ.Targ
`, pkgName)

	err = os.WriteFile(targFile, []byte(content), filePermissionsForCode)
	if err != nil {
		return "", fmt.Errorf("creating targ file: %w", err)
	}

	return targFile, nil
}

// generateGroupModifications creates group declarations and modifications for the path.
// For existing groups, it returns patches to add the new member.
// For new groups, it returns code to append.
func generateGroupModifications(
	path []string,
	targetVarName, existingContent string,
) groupModifications {
	var mods groupModifications
	if len(path) == 0 {
		return mods
	}

	var newCode strings.Builder

	// Build groups from innermost to outermost
	// e.g., for path ["dev", "lint"] and target "fast":
	// - DevLint group contains DevLintFast (the target)
	// - Dev group contains DevLint
	childVarName := targetVarName

	for i := len(path) - 1; i >= 0; i-- {
		groupPath := path[:i+1]
		groupVarName := pathToPascal(groupPath)
		groupName := path[i] // Use the last component as the group's name

		// Check if group already exists
		groupPattern := fmt.Sprintf("var %s = ", groupVarName)
		if strings.Contains(existingContent, groupPattern) {
			// Group exists - create a patch to add the new member
			patch := createGroupMemberPatch(existingContent, groupVarName, childVarName)
			if patch != nil {
				mods.contentPatches = append(mods.contentPatches, *patch)
			}

			childVarName = groupVarName

			continue
		}

		newCode.WriteString(fmt.Sprintf("var %s = targ.NewGroup(%q, %s)\n",
			groupVarName, groupName, childVarName))
		childVarName = groupVarName
	}

	mods.newCode = newCode.String()

	return mods
}

// generateModuleBootstrap creates bootstrap code and computes cache key.
func generateModuleBootstrap(
	mt moduleTargets,
	importRoot string,
) (moduleBootstrap, error) {
	data, err := buildBootstrapData(
		mt.Packages,
		importRoot,
		mt.ModulePath,
	)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("preparing bootstrap: %w", err)
	}

	tmpl := template.Must(template.New("main").Parse(bootstrapTemplate))

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, data)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("generating code: %w", err)
	}

	cacheKey, err := computeModuleCacheKey(mt, importRoot, buf.Bytes())
	if err != nil {
		return moduleBootstrap{}, err
	}

	return moduleBootstrap{
		code:     buf.Bytes(),
		cacheKey: cacheKey,
	}, nil
}

func goEnv(key string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "go", "env", key)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting go env %s: %w", key, err)
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
			modPath = targLocalModule
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

// hasTargBuildTag returns true if the file has the targ build tag.
func hasTargBuildTag(path string) bool {
	//nolint:gosec // build tool reads user source files by design
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	// Check for //go:build targ at the start
	lines := strings.SplitSeq(string(content), "\n")
	for line := range lines {
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

// isCleanVersion returns true if the version is suitable for cache lookup.
func isCleanVersion(version string) bool {
	return version != "" && version != "(devel)" && !strings.Contains(version, "+dirty")
}

// isCreateFlag checks if the argument is the --create flag.
func isCreateFlag(arg string) bool {
	return arg == "--create"
}

// isHelpRequest returns true if args represent a help request.
func isHelpRequest(args []string) bool {
	return len(args) == 0 || args[0] == "-h" || args[0] == "--help"
}

// isIncludableModuleFile returns true if the file should be included in module cache.
func isIncludableModuleFile(name string) bool {
	// Include go.mod and go.sum for cache invalidation when dependencies change
	if name == "go.mod" || name == "go.sum" {
		return true
	}

	// Include non-test .go files
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

// isRemovedFlag checks if the argument is a removed flag.
func isRemovedFlag(arg string) bool {
	switch {
	case arg == "--init" || strings.HasPrefix(arg, "--init="):
		return true
	case arg == "--alias" || strings.HasPrefix(arg, "--alias="):
		return true
	case arg == "--move":
		return true
	default:
		return false
	}
}

// isSyncFlag checks if the argument is the --sync flag.
func isSyncFlag(arg string) bool {
	return arg == "--sync"
}

// isValidTargetName returns true if the name is valid for a target (kebab-case).
// Must start with lowercase letter, contain only lowercase letters, numbers, and hyphens,
// and cannot end with a hyphen.
func isValidTargetName(name string) bool {
	return validTargetNameRe.MatchString(name)
}

// kebabToPascal converts kebab-case to PascalCase.
func kebabToPascal(s string) string {
	parts := strings.Split(s, "-")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}

	return strings.Join(parts, "")
}

// linkModuleEntry creates a symlink for a single directory entry if needed.
func linkModuleEntry(startDir, root string, entry os.DirEntry) error {
	name := entry.Name()
	// Skip .git and module files - we'll create our own go.mod/go.sum
	if name == ".git" || name == "go.mod" || name == "go.sum" {
		return nil
	}

	src := filepath.Join(startDir, name)
	dst := filepath.Join(root, name)

	if symlinkExists(dst) {
		return nil
	}

	// Remove non-symlink file/dir if it exists
	_ = os.RemoveAll(dst)

	err := os.Symlink(src, dst)
	if err != nil {
		return fmt.Errorf("creating symlink %s -> %s: %w", dst, src, err)
	}

	return nil
}

func linkModuleRoot(startDir, root string) error {
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return fmt.Errorf("reading start directory: %w", err)
	}

	for _, entry := range entries {
		err := linkModuleEntry(startDir, root, entry)
		if err != nil {
			return err
		}
	}

	cleanupStaleModSymlinks(root)

	return nil
}

func looksLikeModulePath(path string) bool {
	if path == "" {
		return false
	}

	first := strings.Split(path, "/")[0]

	return strings.Contains(first, ".")
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
			return nil, fmt.Errorf("getting relative path for %s: %w", file, err)
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

func newBootstrapBuilder(moduleRoot, modulePath string) *bootstrapBuilder {
	return &bootstrapBuilder{
		moduleRoot: moduleRoot,
		modulePath: modulePath,
	}
}

// parseCreateArgs parses arguments after --create into createOptions.
// Format: [path...] <name> [--deps dep1 dep2...] [--cache pattern1...] "shell-command"
// The shell command is always the last argument.
//
//nolint:cyclop // Parsing logic has necessary branches for each flag type
func parseCreateArgs(args []string) (createOptions, error) {
	var opts createOptions

	// Need at least 2 args: name and shell command
	if len(args) < 2 { //nolint:mnd // minimum: name + command
		return opts, errCreateUsage
	}

	// Shell command is always the last argument
	opts.ShellCmd = args[len(args)-1]
	remaining := args[:len(args)-1]

	// Parse remaining arguments
	var pathAndName []string

	i := 0
	for i < len(remaining) {
		arg := remaining[i]

		switch {
		case arg == "--deps":
			// Collect deps until next flag or end
			i++
			for i < len(remaining) && !strings.HasPrefix(remaining[i], "--") {
				opts.Deps = append(opts.Deps, remaining[i])
				i++
			}
		case arg == "--cache":
			// Collect cache patterns until next flag or end
			i++
			for i < len(remaining) && !strings.HasPrefix(remaining[i], "--") {
				opts.Cache = append(opts.Cache, remaining[i])
				i++
			}
		case strings.HasPrefix(arg, "--"):
			return opts, fmt.Errorf("%w: %s", errUnknownCreateFlag, arg)
		default:
			// Non-flag argument: path component or name
			pathAndName = append(pathAndName, arg)
			i++
		}
	}

	// Need at least the target name
	if len(pathAndName) < 1 {
		return opts, errCreateUsage
	}

	// Last of pathAndName is name, rest are path
	opts.Name = pathAndName[len(pathAndName)-1]
	if len(pathAndName) > 1 {
		opts.Path = pathAndName[:len(pathAndName)-1]
	}

	return opts, nil
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
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after)
		}
	}

	return ""
}

// parseSyncArgs parses arguments after --sync into syncOptions.
// Format: --sync <package-path>
func parseSyncArgs(args []string) (syncOptions, error) {
	var opts syncOptions

	if len(args) < 1 {
		return opts, errSyncUsage
	}

	opts.PackagePath = args[0]

	// Validate that it looks like a module path
	if !looksLikeModulePath(opts.PackagePath) {
		return opts, fmt.Errorf("%w: %s", errInvalidPackagePath, opts.PackagePath)
	}

	return opts, nil
}

// pathToPascal converts a path like ["dev", "lint", "fast"] to "DevLintFast".
func pathToPascal(path []string) string {
	var result strings.Builder
	for _, p := range path {
		result.WriteString(kebabToPascal(p))
	}

	return result.String()
}

// prepareBuildContext determines build roots and handles fallback module setup.
func prepareBuildContext(
	mt moduleTargets,
	startDir string,
	dep targDependency,
) (buildContext, error) {
	ctx := buildContext{
		usingFallback: mt.ModulePath == targLocalModule,
		buildRoot:     mt.ModuleRoot,
		importRoot:    mt.ModuleRoot,
	}

	if ctx.usingFallback {
		var err error

		ctx.buildRoot, err = ensureFallbackModuleRoot(startDir, mt.ModulePath, dep)
		if err != nil {
			return ctx, fmt.Errorf("preparing fallback module: %w", err)
		}
	}

	return ctx, nil
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
	if maxLen < minCommandNameWidth {
		maxLen = minCommandNameWidth
	}
	// Indent for continuation lines: leading + name width + padding + space + extra padding
	indent := strings.Repeat(" ", helpIndentWidth+maxLen+commandNamePadding+1+commandNamePadding)

	for _, cmd := range allCmds {
		lines := strings.Split(cmd.description, "\n")
		fmt.Printf("    %-*s %s\n", maxLen+commandNamePadding, cmd.name, lines[0])

		for _, line := range lines[1:] {
			fmt.Printf("%s%s\n", indent, line)
		}
	}

	fmt.Println()
	fmt.Println("More info: https://github.com/toejough/targ#readme")
}

// printNoCommandsHelp prints the help message when no commands are found.

// printNoTargetsCompletion outputs completion suggestions when no target files exist.
// This allows users to discover flags even before creating targets.
func printNoTargetsCompletion(args []string) {
	// Parse the command line from __complete args
	if len(args) < minArgsForCompletion {
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
		"--no-binary-cache",
		"--completion",
	}

	for _, flag := range allFlags {
		if strings.HasPrefix(flag, prefix) {
			fmt.Println(flag)
		}
	}
}

// printRootCommands prints commands that are at the root level (no namespace).

// printSubcommandTree prints the top-level subcommand names.

// projectCacheDir returns a project-specific subdirectory within the targ cache.
// Uses a hash of the project path to isolate projects.
func projectCacheDir(projectPath string) string {
	hash := sha256.Sum256([]byte(projectPath))
	return filepath.Join(targCacheDir(), hex.EncodeToString(hash[:8]))
}

// queryModuleCommands queries a module binary for its available commands.
func queryModuleCommands(binaryPath string) ([]commandInfo, error) {
	cmd := exec.CommandContext(context.Background(), binaryPath, "__list")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running __list: %w", err)
	}

	var result listOutput

	err = json.Unmarshal(output, &result)
	if err != nil {
		return nil, fmt.Errorf("parsing __list output: %w", err)
	}

	return result.Commands, nil
}

// remapPackageInfosToIsolated creates new package infos with paths pointing to isolated dir.
// Returns the remapped infos and a mapping from new paths to original paths.
func remapPackageInfosToIsolated(
	infos []buildtool.PackageInfo,
	startDir, isolatedDir string,
) ([]buildtool.PackageInfo, map[string]string, error) {
	filePaths := collectFilePaths(infos)

	paths, err := namespacePaths(filePaths, startDir)
	if err != nil {
		return nil, nil, fmt.Errorf("computing namespace paths: %w", err)
	}

	result := make([]buildtool.PackageInfo, 0, len(infos))
	pathMapping := make(map[string]string) // newPath -> originalPath

	for _, info := range infos {
		newInfo := buildtool.PackageInfo{
			Package: info.Package,
			Doc:     info.Doc,
		}

		// Compute new directory based on collapsed paths
		var newDir string

		if len(info.Files) > 0 {
			collapsedPath := paths[info.Files[0].Path]
			if len(collapsedPath) > 0 {
				dirParts := collapsedPath[:len(collapsedPath)-1]
				dirParts = append(dirParts, info.Package)
				newDir = filepath.Join(isolatedDir, filepath.Join(dirParts...))
			} else {
				newDir = filepath.Join(isolatedDir, info.Package)
			}
		}

		newInfo.Dir = newDir

		// Remap file paths
		newFiles := make([]buildtool.FileInfo, 0, len(info.Files))
		for _, f := range info.Files {
			newPath := filepath.Join(newDir, filepath.Base(f.Path))
			pathMapping[newPath] = f.Path // Track original path
			newFiles = append(newFiles, buildtool.FileInfo{
				Path: newPath,
				Base: f.Base,
			})
		}

		newInfo.Files = newFiles
		result = append(result, newInfo)
	}

	return result, pathMapping, nil
}

func resolveTargDependency() targDependency {
	dep := targDependency{
		ModulePath: defaultTargModulePath,
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return dep
	}

	if looksLikeModulePath(info.Main.Path) {
		dep.ModulePath = info.Main.Path
	}

	if cacheDir, ok := findModCacheDir(dep.ModulePath, info.Main.Version); ok {
		dep.Version = info.Main.Version
		dep.ReplaceDir = cacheDir
	} else if root, ok := buildSourceRoot(); ok {
		dep.ReplaceDir = root
	}

	return dep
}

// runGoBuild executes the go build command.
func runGoBuild(ctx buildContext, binaryPath, tempFile string, errOut io.Writer) error {
	buildArgs := []string{"build", "-tags", "targ", "-o", binaryPath}
	if ctx.usingFallback {
		buildArgs = append(buildArgs, "-mod=mod")
	}

	buildArgs = append(buildArgs, tempFile)

	//nolint:gosec // build tool runs go build by design
	buildCmd := exec.CommandContext(context.Background(), "go", buildArgs...)

	var buildOutput bytes.Buffer

	buildCmd.Stdout = io.Discard
	buildCmd.Stderr = &buildOutput

	if ctx.usingFallback {
		buildCmd.Dir = ctx.buildRoot
	} else {
		buildCmd.Dir = ctx.importRoot
	}

	err := buildCmd.Run()
	if err != nil {
		if buildOutput.Len() > 0 {
			_, _ = fmt.Fprint(errOut, buildOutput.String())
		}

		return fmt.Errorf("building command: %w", err)
	}

	return nil
}

func runMain() int {
	// Guard against nil os.Args (should never happen, but satisfies static analysis)
	if len(os.Args) == 0 {
		fmt.Fprintln(os.Stderr, "error: os.Args is empty")
		return 1
	}

	r := &targRunner{
		binArg: os.Args[0],
		args:   os.Args[1:],
		errOut: os.Stderr,
	}

	return r.run()
}

// runModuleBinary executes a module binary with the given args.
func runModuleBinary(binaryPath string, args []string, errOut io.Writer, binArg string) error {
	proc := exec.CommandContext(context.Background(), binaryPath, args...)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = errOut

	proc.Env = append(os.Environ(), "TARG_BIN_NAME="+extractBinName(binArg))

	err := proc.Run()
	if err != nil {
		return fmt.Errorf("running module binary: %w", err)
	}

	return nil
}

// setupBinaryPath creates cache directory and returns binary path.
func setupBinaryPath(importRoot, _ string, bootstrap moduleBootstrap) (string, error) {
	projCache := projectCacheDir(importRoot)
	cacheDir := filepath.Join(projCache, "bin")

	//nolint:gosec,mnd // standard cache directory permissions
	err := os.MkdirAll(cacheDir, 0o755)
	if err != nil {
		return "", fmt.Errorf("creating cache directory: %w", err)
	}

	return filepath.Join(cacheDir, "targ_"+bootstrap.cacheKey), nil
}

// skipIfVendorOrGit returns SkipDir for .git and vendor directories.
func skipIfVendorOrGit(name string) error {
	if name == ".git" || name == "vendor" {
		return filepath.SkipDir
	}

	return nil
}

// stripBuildTag removes the //go:build targ line from source content.
func stripBuildTag(content string) string {
	var result strings.Builder

	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//go:build") && strings.Contains(trimmed, "targ") {
			continue
		}
		// Also skip legacy +build tag
		if strings.HasPrefix(trimmed, "// +build") && strings.Contains(trimmed, "targ") {
			continue
		}

		result.WriteString(line)
		result.WriteString("\n")
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// symlinkExists returns true if dst is an existing symlink.
func symlinkExists(dst string) bool {
	info, err := os.Lstat(dst)
	if err != nil || info == nil {
		return false
	}

	return info.Mode()&os.ModeSymlink != 0
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

func touchFile(path string) error {
	err := os.WriteFile(path, []byte{}, filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("touching file %s: %w", path, err)
	}

	return nil
}

// tryCachedBinary checks if a cached binary exists and queries its commands.
func tryCachedBinary(binaryPath string) ([]commandInfo, bool) {
	info, err := os.Stat(binaryPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return nil, false
	}

	cmds, err := queryModuleCommands(binaryPath)
	if err != nil {
		return nil, false
	}

	return cmds, true
}

// validateNoPackageMain ensures no targ files use package main.
func validateNoPackageMain(mt moduleTargets) error {
	for _, pkg := range mt.Packages {
		if pkg.Package == pkgNameMain {
			return fmt.Errorf(
				"%w (found in %s); use a named package instead",
				errPackageMainNotAllowed,
				pkg.Dir,
			)
		}
	}

	return nil
}

func writeBootstrapFile(dir string, data []byte) (string, func() error, error) {
	//nolint:gosec,mnd // standard directory permissions for bootstrap
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return "", nil, fmt.Errorf("creating bootstrap directory: %w", err)
	}

	temp, err := os.CreateTemp(dir, "targ_bootstrap_*.go")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp file: %w", err)
	}

	tempFile := temp.Name()

	_, err = temp.Write(data)
	if err != nil {
		_ = temp.Close()
		return "", nil, fmt.Errorf("writing bootstrap file: %w", err)
	}

	err = temp.Close()
	if err != nil {
		return "", nil, fmt.Errorf("closing bootstrap file: %w", err)
	}

	cleanup := func() error {
		err := os.Remove(tempFile)

		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing bootstrap file: %w", err)
		}

		return nil
	}

	return tempFile, cleanup, nil
}

func writeFallbackGoMod(root, modulePath string, dep targDependency) error {
	modPath := filepath.Join(root, "go.mod")

	if dep.ModulePath == "" {
		dep.ModulePath = defaultTargModulePath
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

	err := os.WriteFile(modPath, []byte(content), filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("writing go.mod: %w", err)
	}

	return nil
}

// writeIsolatedGoMod creates a go.mod for isolated builds.
func writeIsolatedGoMod(tmpDir string, dep targDependency) error {
	modPath := filepath.Join(tmpDir, "go.mod")

	if dep.ModulePath == "" {
		dep.ModulePath = defaultTargModulePath
	}

	lines := []string{
		"module " + isolatedModuleName,
		"",
		"go 1.21",
	}

	// Always add require - use a placeholder version if not specified
	version := dep.Version
	if version == "" {
		version = "v0.0.0"
	}

	lines = append(lines, "", fmt.Sprintf("require %s %s", dep.ModulePath, version))

	if dep.ReplaceDir != "" {
		lines = append(lines, "", fmt.Sprintf("replace %s => %s", dep.ModulePath, dep.ReplaceDir))
	}

	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(modPath, []byte(content), filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("writing isolated go.mod: %w", err)
	}

	// Touch go.sum file
	sumPath := filepath.Join(tmpDir, "go.sum")

	err = os.WriteFile(sumPath, []byte{}, filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("writing go.sum: %w", err)
	}

	return nil
}

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
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode"
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
{{- if .SourceFile }}
func (c *{{ .TypeName }}) SourceFile() string {
	return {{ printf "%q" .SourceFile }}
}
{{- end }}
{{- end }}

{{- range .Nodes }}
type {{ .TypeName }} struct {
{{- range .Fields }}
	{{ .Name }} {{ .TypeExpr }} ` + "`{{ .TagLit }}`" + `
{{- end }}
}
{{- if .Description }}
func (c *{{ .TypeName }}) Description() string {
	return {{ printf "%q" .Description }}
}
{{- end }}
{{- if .SourceFile }}
func (c *{{ .TypeName }}) SourceFile() string {
	return {{ printf "%q" .SourceFile }}
}
{{- end }}
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

	opts := targ.RunOptions{
		AllowDefault: {{ .AllowDefault }},
{{- if .Description }}
		Description: {{ printf "%q" .Description }},
{{- end }}
	}
	targ.RunWithOptions(opts, roots...)
}
`
	commandNamePadding    = 2 // Padding after command name column
	completeCommand       = "__complete"
	defaultTargModulePath = "github.com/toejough/targ"
	helpIndentWidth       = 4 // Leading spaces in help output
	importExtraLines      = 3 // Extra line capacity when adding import statement
	isolatedModuleName    = "targ.build.local"
	minArgsForCompletion  = 2  // Minimum args for __complete (binary + arg)
	minCommandNameWidth   = 10 // Minimum column width for command names in help output
	minSourcePatternParts = 2  // package + pattern at minimum
	pkgNameDefault        = "pkg"
	targLocalModule       = "targ.local"
	targsGoFilename       = "targs.go"
)

// unexported variables.
var (
	errAliasNameEmpty    = errors.New("alias name cannot be empty")
	errAliasRequiresArgs = errors.New(
		"--alias requires at least two arguments: NAME \"COMMAND\" [FILE]",
	)
	errAliasRequiresCommand     = errors.New("--alias requires a command argument")
	errCommandEmpty             = errors.New("command cannot be empty")
	errDuplicateCommandName     = errors.New("duplicate command name")
	errDuplicateNamespace       = errors.New("duplicate namespace field")
	errFileExists               = errors.New("file already exists")
	errInvalidUTF8Path          = errors.New("invalid utf-8 path in tagged file")
	errModulePathNotFound       = errors.New("module path not found")
	errMoveCommandNotFound      = errors.New("command not found")
	errMoveDestConflict         = errors.New("destination conflicts with existing command")
	errMoveInvalidSourcePattern = errors.New(
		"source pattern must include package (e.g., dev.lint*)",
	)
	errMoveNoMatches         = errors.New("no commands match pattern")
	errMoveNoSourceFiles     = errors.New("no source files found in package")
	errMovePackageNotFound   = errors.New("package not found")
	errMoveRequiresArgs      = errors.New("--move requires: DEST SOURCE_PATTERN")
	errMoveStructNotFound    = errors.New("struct not found")
	errMultipleTargetFiles   = errors.New("multiple target files found")
	errNoCommandsInPackage   = errors.New("no commands found in package")
	errPackageMainNotAllowed = errors.New("targ files cannot use 'package main'")
	errUnclosedQuote         = errors.New("unclosed quote")
	errUnknownCommand        = errors.New("unknown command")
	errUnknownCommandKind    = errors.New("unknown command kind")
)

type aliasResult struct {
	message string
	err     error
}

type bootstrapBuilder struct {
	absStart     string
	moduleRoot   string
	modulePath   string
	imports      []bootstrapImport
	usedImports  map[string]bool
	fileCommands map[string][]bootstrapCommand
	funcWrappers []bootstrapFuncWrapper
	usesContext  bool
	wrapperNames *nameGenerator
	pathMapping  map[string]string // maps isolated paths to original paths
}

func (b *bootstrapBuilder) addFuncCommand(
	pkgName string,
	cmd buildtool.CommandInfo,
	prefix string,
) {
	base := segmentToIdent(pkgName) + segmentToIdent(cmd.Name) + "Func"
	typeName := b.wrapperNames.uniqueTypeName(base)

	b.funcWrappers = append(b.funcWrappers, bootstrapFuncWrapper{
		TypeName:     typeName,
		Name:         cmd.Name,
		FuncExpr:     prefix + cmd.Name,
		SourceFile:   cmd.File,
		UsesContext:  cmd.UsesContext,
		ReturnsError: cmd.ReturnsError,
	})

	if cmd.UsesContext {
		b.usesContext = true
	}

	b.fileCommands[cmd.File] = append(b.fileCommands[cmd.File], bootstrapCommand{
		Name:      cmd.Name,
		TypeExpr:  "*" + typeName,
		ValueExpr: "&" + typeName + "{}",
	})
}

func (b *bootstrapBuilder) addStructCommand(cmd buildtool.CommandInfo, prefix string) {
	b.fileCommands[cmd.File] = append(b.fileCommands[cmd.File], bootstrapCommand{
		Name:      cmd.Name,
		TypeExpr:  "*" + prefix + cmd.Name,
		ValueExpr: "&" + prefix + cmd.Name + "{}",
	})
}

func (b *bootstrapBuilder) buildResult(
	startDir string,
	infos []buildtool.PackageInfo,
) (bootstrapData, error) {
	filePaths := b.sortedFilePaths()

	paths, err := namespacePaths(filePaths, startDir)
	if err != nil {
		return bootstrapData{}, err
	}

	tree := buildNamespaceTree(paths)
	assignNamespaceNames(tree, &nameGenerator{})

	rootExprs := b.collectRootExprs(filePaths, paths, tree)

	// Build map from file path to package doc
	// Also map by directory for namespace nodes (which may have any file from the package)
	pkgDocs := make(map[string]string)

	for _, info := range infos {
		doc := strings.TrimSpace(info.Doc)
		if doc == "" {
			continue
		}

		for _, cmd := range info.Commands {
			pkgDocs[cmd.File] = doc
		}
		// Also map by directory for files not in commands (namespace nodes use one file)
		for _, f := range info.Files {
			pkgDocs[f.Path] = doc
		}
	}

	var nodes []bootstrapNode

	err = collectNamespaceNodes(tree, b.fileCommands, pkgDocs, b.pathMapping, &nodes)
	if err != nil {
		return bootstrapData{}, err
	}

	bannerLit := ""
	// Default description for targ build tool mode
	description := "Targ discovers and runs build targets you write in Go."

	if len(infos) == 1 {
		bannerLit = strconv.Quote(singlePackageBanner(infos[0]))
	}

	return bootstrapData{
		AllowDefault: false,
		BannerLit:    bannerLit,
		Description:  description,
		Imports:      b.imports,
		RootExprs:    rootExprs,
		Nodes:        nodes,
		FuncWrappers: b.funcWrappers,
		UsesContext:  b.usesContext,
	}, nil
}

func (b *bootstrapBuilder) collectRootExprs(
	filePaths []string,
	paths map[string][]string,
	tree *namespaceNode,
) []string {
	rootExprs := make([]string, 0)

	for _, path := range filePaths {
		if len(paths[path]) != 0 {
			continue
		}

		for _, cmd := range b.fileCommands[path] {
			rootExprs = append(rootExprs, cmd.ValueExpr)
		}
	}

	rootNames := make([]string, 0, len(tree.Children))
	for name := range tree.Children {
		rootNames = append(rootNames, name)
	}

	sort.Strings(rootNames)

	for _, name := range rootNames {
		if child := tree.Children[name]; child != nil {
			rootExprs = append(rootExprs, child.VarName)
		}
	}

	return rootExprs
}

func (b *bootstrapBuilder) computeImportPath(dir string) string {
	rel, err := filepath.Rel(b.moduleRoot, dir)
	if err != nil || rel == "." {
		return b.modulePath
	}

	return b.modulePath + "/" + filepath.ToSlash(rel)
}

func (b *bootstrapBuilder) processCommand(
	pkgName string,
	cmd buildtool.CommandInfo,
	prefix string,
) error {
	switch cmd.Kind {
	case buildtool.CommandStruct:
		b.addStructCommand(cmd, prefix)
	case buildtool.CommandFunc:
		b.addFuncCommand(pkgName, cmd, prefix)
	default:
		return fmt.Errorf("%w: %s", errUnknownCommandKind, cmd.Name)
	}

	return nil
}

func (b *bootstrapBuilder) processPackage(info buildtool.PackageInfo) error {
	if len(info.Commands) == 0 {
		return fmt.Errorf("%w: %s", errNoCommandsInPackage, info.Package)
	}

	local := sameDir(b.absStart, info.Dir)
	prefix := b.setupImport(info, local)

	for _, cmd := range info.Commands {
		err := b.processCommand(info.Package, cmd, prefix)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *bootstrapBuilder) setupImport(info buildtool.PackageInfo, _ bool) string {
	// Always import and prefix - bootstrap is package main, so it can't
	// access symbols from target packages without importing them
	importPath := b.computeImportPath(info.Dir)
	importName := uniqueImportName(info.Package, b.usedImports)

	b.imports = append(b.imports, bootstrapImport{
		Path:  importPath,
		Alias: importName,
	})

	return importName + "."
}

func (b *bootstrapBuilder) sortedFilePaths() []string {
	filePaths := make([]string, 0, len(b.fileCommands))

	for path := range b.fileCommands {
		sort.Slice(b.fileCommands[path], func(i, j int) bool {
			return b.fileCommands[path][i].Name < b.fileCommands[path][j].Name
		})
		filePaths = append(filePaths, path)
	}

	sort.Strings(filePaths)

	return filePaths
}

type bootstrapCommand struct {
	Name      string
	TypeExpr  string
	ValueExpr string
}

type bootstrapData struct {
	AllowDefault bool
	BannerLit    string
	Description  string
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
	SourceFile   string
	UsesContext  bool
	ReturnsError bool
}

type bootstrapImport struct {
	Path  string
	Alias string
}

type bootstrapNode struct {
	Name        string
	TypeName    string
	VarName     string
	Description string
	SourceFile  string
	Fields      []bootstrapField
}

type buildContext struct {
	usingFallback bool
	buildRoot     string
	importRoot    string
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

type moduleBootstrap struct {
	code     []byte
	cacheKey string
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

type moveResult struct {
	message string
	err     error
}

type movedCommand struct {
	info    buildtool.CommandInfo
	newName string
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

// parseShellCommand splits a shell command string into parts.
// Handles quoted strings.
type shellCommandParser struct {
	parts   []string
	current strings.Builder
	inQuote rune
	escaped bool
}

func (p *shellCommandParser) finalize() ([]string, error) {
	if p.inQuote != 0 {
		return nil, errUnclosedQuote
	}

	p.flushCurrent()

	return p.parts, nil
}

func (p *shellCommandParser) flushCurrent() {
	if p.current.Len() > 0 {
		p.parts = append(p.parts, p.current.String())
		p.current.Reset()
	}
}

func (p *shellCommandParser) handleQuotedChar(r rune) {
	if r == p.inQuote {
		p.inQuote = 0
	} else {
		p.current.WriteRune(r)
	}
}

func (p *shellCommandParser) handleUnquotedChar(r rune) {
	if r == '"' || r == '\'' {
		p.inQuote = r

		return
	}

	if r == ' ' || r == '\t' {
		p.flushCurrent()

		return
	}

	p.current.WriteRune(r)
}

func (p *shellCommandParser) processRune(r rune) {
	if p.escaped {
		p.current.WriteRune(r)
		p.escaped = false

		return
	}

	if r == '\\' {
		p.escaped = true

		return
	}

	if p.inQuote != 0 {
		p.handleQuotedChar(r)

		return
	}

	p.handleUnquotedChar(r)
}

type subcommandRef struct {
	typeName string
	file     string
}

type targDependency struct {
	ModulePath string
	Version    string
	ReplaceDir string
}

// targRunner holds state for a single targ invocation.
type targRunner struct {
	binArg            string
	args              []string
	errOut            io.Writer
	startDir          string
	generatedWrappers []string
	noCache           bool
	keepBootstrap     bool
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

	tempFile, cleanupTemp, err := writeBootstrapFile(bootstrapDir, bootstrapCode, r.keepBootstrap)
	if err != nil {
		r.logError("Error writing bootstrap file", err)
		return r.exitWithCleanup(1)
	}

	if !r.keepBootstrap {
		defer func() { _ = cleanupTemp() }()
	}

	err = r.executeBuild(buildDir, binaryPath, tempFile, isolated)
	if err != nil {
		if !r.keepBootstrap {
			_ = cleanupTemp()
		}

		r.logError("Error building command", err)

		return r.exitWithCleanup(1)
	}

	if !r.keepBootstrap {
		_ = cleanupTemp()
	}

	return r.executeBuiltBinary(binaryPath, targBinName)
}

func (r *targRunner) cleanupWrappers() {
	for _, path := range r.generatedWrappers {
		_ = os.Remove(path)
	}
}

func (r *targRunner) discoverAndGenerateWrappers() ([]buildtool.PackageInfo, error) {
	taggedDirs, err := buildtool.SelectTaggedDirs(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir: r.startDir,
		BuildTag: "targ",
	})
	if err != nil {
		return nil, fmt.Errorf("error discovering commands: %w", err)
	}

	for _, dir := range taggedDirs {
		wrapper, err := buildtool.GenerateFunctionWrappers(
			buildtool.OSFileSystem{},
			buildtool.GenerateOptions{
				Dir:        dir.Path,
				BuildTag:   "targ",
				OnlyTagged: true,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("error generating command wrappers: %w", err)
		}

		if wrapper != "" {
			r.generatedWrappers = append(r.generatedWrappers, wrapper)
		}
	}

	infos, err := buildtool.Discover(buildtool.OSFileSystem{}, buildtool.Options{
		StartDir: r.startDir,
		BuildTag: "targ",
	})
	if err != nil {
		return nil, fmt.Errorf("error discovering commands: %w", err)
	}

	// Validate no package main in targ files
	for _, info := range infos {
		if info.Package == "main" {
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

	r.cleanupWrappers()

	return 0
}

func (r *targRunner) exitWithCleanup(code int) int {
	r.cleanupWrappers()
	return code
}

func (r *targRunner) handleEarlyFlags() (exitCode int, done bool) {
	if result := handleInitFlag(r.args); result != nil {
		if result.err != nil {
			fmt.Fprintln(os.Stderr, result.err)
			return 1, true
		}

		fmt.Println(result.message)

		return 0, true
	}

	if result := handleAliasFlag(r.args); result != nil {
		if result.err != nil {
			fmt.Fprintln(os.Stderr, result.err)
			return 1, true
		}

		fmt.Println(result.message)

		return 0, true
	}

	if result := handleMoveFlag(r.args); result != nil {
		if result.err != nil {
			fmt.Fprintln(os.Stderr, result.err)
			return 1, true
		}

		fmt.Println(result.message)

		return 0, true
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

	if !r.keepBootstrap {
		defer cleanup()
	}

	// Remap package infos to point to isolated directory
	isolatedInfos, pathMapping, err := remapPackageInfosToIsolated(infos, r.startDir, isolatedDir)
	if err != nil {
		r.logError("Error remapping package infos", err)
		return r.exitWithCleanup(1)
	}

	filePaths := collectFilePaths(isolatedInfos)

	collapsedPaths, err := namespacePaths(filePaths, isolatedDir)
	if err != nil {
		r.logError("Error computing namespace paths", err)
		return r.exitWithCleanup(1)
	}

	bootstrap, err := r.prepareBootstrapWithMapping(
		isolatedInfos,
		isolatedDir,
		isolatedModuleName,
		collapsedPaths,
		pathMapping,
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
	if !r.noCache {
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
		r.noCache,
		r.keepBootstrap,
		r.errOut,
	)
	if err != nil {
		r.logError("Error building module binaries", err)
		return r.exitWithCleanup(1)
	}

	r.cleanupWrappers()

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

	collapsedPaths, err := namespacePaths(filePaths, r.startDir)
	if err != nil {
		r.logError("Error computing namespace paths", err)
		return r.exitWithCleanup(1)
	}

	importRoot, modulePath, _, err := findModuleForPath(filePaths[0])
	if err != nil {
		r.logError("Error checking for module", err)
		return r.exitWithCleanup(1)
	}

	bootstrap, err := r.prepareBootstrap(infos, importRoot, modulePath, collapsedPaths)
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
	if !r.noCache {
		if code, ran := r.tryRunCached(binaryPath, targBinName); ran {
			return code
		}
	}

	// Build and run
	return r.buildAndRun(importRoot, binaryPath, targBinName, bootstrap.code)
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
	collapsedPaths map[string][]string,
) (moduleBootstrap, error) {
	data, err := buildBootstrapData(infos, r.startDir, importRoot, modulePath, collapsedPaths)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error preparing bootstrap: %w", err)
	}

	tmpl := template.Must(template.New("main").Parse(bootstrapTemplate))

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, data)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error generating code: %w", err)
	}

	taggedFiles, err := buildtool.TaggedFiles(buildtool.OSFileSystem{}, buildtool.Options{
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

func (r *targRunner) prepareBootstrapWithMapping(
	infos []buildtool.PackageInfo,
	importRoot, modulePath string,
	collapsedPaths map[string][]string,
	pathMapping map[string]string,
) (moduleBootstrap, error) {
	data, err := buildBootstrapDataWithMapping(
		infos, r.startDir, importRoot, modulePath, collapsedPaths, pathMapping,
	)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error preparing bootstrap: %w", err)
	}

	tmpl := template.Must(template.New("main").Parse(bootstrapTemplate))

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, data)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error generating code: %w", err)
	}

	taggedFiles, err := buildtool.TaggedFiles(buildtool.OSFileSystem{}, buildtool.Options{
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
	// Handle --init and --alias early
	if code, done := r.handleEarlyFlags(); done {
		return code
	}

	// Setup quiet mode for completion
	if len(r.args) > 0 && r.args[0] == completeCommand {
		r.errOut = io.Discard
	}

	helpRequested, helpTargets := parseHelpRequest(r.args)
	r.noCache, r.keepBootstrap, r.args = extractTargFlags(r.args)

	var err error

	r.startDir, err = os.Getwd()
	if err != nil {
		r.logError("Error resolving working directory", err)
		return 1
	}

	// Discovery and wrapper generation
	infos, err := r.discoverAndGenerateWrappers()
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

	r.cleanupWrappers()

	return 0, true
}

// addAlias generates and appends alias code to the appropriate target file.
func addAlias(name, command, targetFile string) (string, error) {
	code, err := generateAlias(name, command)
	if err != nil {
		return "", err
	}

	if targetFile != "" {
		return addAliasToFile(name, targetFile, code)
	}

	return addAliasAutoDiscover(name, command, code)
}

func addAliasAutoDiscover(name, command, code string) (string, error) {
	targetFiles, err := findTargetFiles(".")
	if err != nil {
		return "", fmt.Errorf("discovering target files: %w", err)
	}

	switch len(targetFiles) {
	case 0:
		return addAliasCreateNew(name, code)
	case 1:
		return addAliasToExisting(name, targetFiles[0], code)
	default:
		return "", fmt.Errorf(
			"%w (%s); specify which file: --alias %s %q <file>",
			errMultipleTargetFiles, strings.Join(targetFiles, ", "), name, command,
		)
	}
}

func addAliasCreateNew(name, code string) (string, error) {
	targetFile := targsGoFilename

	_, err := createTargetsFile(targetFile)
	if err != nil {
		return "", err
	}

	err = appendToFile(targetFile, code)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Created %s and added %s", targetFile, toExportedName(name)), nil
}

func addAliasToExisting(name, targetFile, code string) (string, error) {
	err := ensureShImport(targetFile)
	if err != nil {
		return "", fmt.Errorf("ensuring sh import: %w", err)
	}

	err = appendToFile(targetFile, code)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Added %s to %s", toExportedName(name), targetFile), nil
}

func addAliasToFile(name, targetFile, code string) (string, error) {
	err := appendToFile(targetFile, code)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Added %s to %s", toExportedName(name), targetFile), nil
}

// addFieldToStruct adds a new field to an existing struct in a Go file.
func addFieldToStruct(filePath, structName, fieldCode string) error {
	//nolint:gosec // build tool reads source files by design
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	insertPos := findStructClosingBrace(file, structName)
	if insertPos == 0 {
		return fmt.Errorf("%w: %s", errMoveStructNotFound, structName)
	}

	// Convert position to byte offset and insert field
	offset := fset.Position(insertPos).Offset
	newContent := string(content[:offset]) + fieldCode + string(content[offset:])

	//nolint:gosec,mnd // build tool writes source files by design
	err = os.WriteFile(filePath, []byte(newContent), 0o644)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

func addMoveStructUnderParent(
	destParentPath []string,
	destLeaf string,
	exactMatch *buildtool.CommandInfo,
	subcommands []movedCommand,
	targetFile, parentFile, parentStruct string,
) (string, error) {
	newStructName := toExportedName(destLeaf)
	code := generateMoveStruct(destLeaf, exactMatch, subcommands, targetFile)

	fieldCode := fmt.Sprintf("\t%s *%s `targ:\"subcommand\"`\n", newStructName, newStructName)

	err := addFieldToStruct(parentFile, parentStruct, fieldCode)
	if err != nil {
		return "", fmt.Errorf("adding field to %s: %w", parentStruct, err)
	}

	err = appendToFile(targetFile, code)
	if err != nil {
		return "", err
	}

	dest := strings.Join(append(destParentPath, destLeaf), ".")
	movedNames := buildMovedNames(dest, exactMatch, subcommands)

	return fmt.Sprintf(
		"Added to %s (under %s):\n  %s",
		targetFile, parentStruct, strings.Join(movedNames, "\n  "),
	), nil
}

func addShImportToContent(content string) string {
	lines := strings.Split(content, "\n")

	result := make([]string, 0, len(lines)+importExtraLines)

	importAdded := false

	for i, line := range lines {
		result = append(result, line)

		if importAdded {
			continue
		}

		trimmed := strings.TrimSpace(line)
		if added := tryAddImportBlock(trimmed, &result); added {
			importAdded = true

			continue
		}

		if added := tryConvertSingleImport(trimmed, &result); added {
			importAdded = true

			continue
		}

		if added := tryAddAfterPackage(trimmed, lines, i, &result); added {
			importAdded = true
		}
	}

	return strings.Join(result, "\n")
}

// appendToFile appends content to a file.
func appendToFile(path, content string) (err error) {
	//nolint:gosec,mnd // standard file permissions for shell config
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening file for append: %w", err)
	}

	defer func() {
		cerr := f.Close()
		if cerr != nil && err == nil {
			err = fmt.Errorf("closing file: %w", cerr)
		}
	}()

	_, err = f.WriteString(content)
	if err != nil {
		return fmt.Errorf("writing to file: %w", err)
	}

	return nil
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

// buildAndQueryBinary builds the binary and queries its commands.
func buildAndQueryBinary(
	ctx buildContext,
	_ moduleTargets,
	dep targDependency,
	binaryPath string,
	bootstrap moduleBootstrap,
	keepBootstrap bool,
	errOut io.Writer,
) ([]commandInfo, error) {
	bootstrapDir := filepath.Join(projectCacheDir(ctx.importRoot), "tmp")
	if ctx.usingFallback {
		bootstrapDir = filepath.Join(ctx.buildRoot, "tmp")
	}

	tempFile, cleanupTemp, err := writeBootstrapFile(bootstrapDir, bootstrap.code, keepBootstrap)
	if err != nil {
		return nil, fmt.Errorf("writing bootstrap file: %w", err)
	}

	if !keepBootstrap {
		defer func() { _ = cleanupTemp() }()
	}

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
	startDir string,
	moduleRoot string,
	modulePath string,
	_ map[string][]string,
) (bootstrapData, error) {
	return buildBootstrapDataWithMapping(infos, startDir, moduleRoot, modulePath, nil, nil)
}

func buildBootstrapDataWithMapping(
	infos []buildtool.PackageInfo,
	startDir string,
	moduleRoot string,
	modulePath string,
	_ map[string][]string,
	pathMapping map[string]string,
) (bootstrapData, error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return bootstrapData{}, fmt.Errorf("getting absolute path: %w", err)
	}

	builder := newBootstrapBuilder(absStart, moduleRoot, modulePath)
	builder.pathMapping = pathMapping

	for _, info := range infos {
		err := builder.processPackage(info)
		if err != nil {
			return bootstrapData{}, err
		}
	}

	return builder.buildResult(startDir, infos)
}

func buildCommandFields(
	node *namespaceNode,
	commands []bootstrapCommand,
	usedNames map[string]bool,
) ([]bootstrapField, error) {
	fields := make([]bootstrapField, 0, len(commands))

	for _, cmd := range commands {
		if usedNames[cmd.Name] {
			return nil, fmt.Errorf("%w: %q under %q", errDuplicateCommandName, cmd.Name, node.Name)
		}

		usedNames[cmd.Name] = true
		fields = append(fields, bootstrapField{
			Name:     cmd.Name,
			TypeExpr: cmd.TypeExpr,
			TagLit:   `targ:"subcommand"`,
		})
	}

	return fields, nil
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

	err := validateNoPackageMain(mt)
	if err != nil {
		return reg, err
	}

	buildCtx, err := prepareBuildContext(mt, startDir, dep)
	if err != nil {
		return reg, err
	}

	bootstrap, err := generateModuleBootstrap(mt, startDir, buildCtx.importRoot)
	if err != nil {
		return reg, err
	}

	binaryPath, err := setupBinaryPath(buildCtx.importRoot, mt.ModulePath, bootstrap)
	if err != nil {
		return reg, err
	}

	reg.BinaryPath = binaryPath

	if !noCache {
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
		keepBootstrap,
		errOut,
	)
	if err != nil {
		return reg, err
	}

	reg.Commands = cmds

	return reg, nil
}

// buildMovedNames creates the list of moved command names for output.
func buildMovedNames(
	dest string,
	exactMatch *buildtool.CommandInfo,
	subcommands []movedCommand,
) []string {
	capacity := len(subcommands)
	if exactMatch != nil {
		capacity++
	}

	movedNames := make([]string, 0, capacity)

	if exactMatch != nil {
		movedNames = append(movedNames, dest+" (parent)")
	}

	for _, cmd := range subcommands {
		movedNames = append(movedNames, dest+" "+cmd.newName)
	}

	return movedNames
}

// buildMultiModuleBinaries builds a binary for each module group and returns the registry.
func buildMultiModuleBinaries(
	moduleGroups []moduleTargets,
	startDir string,
	noCache bool,
	keepBootstrap bool,
	errOut io.Writer,
) ([]moduleRegistry, error) {
	registry := make([]moduleRegistry, 0, len(moduleGroups))

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

func buildNamespaceFields(
	node *namespaceNode,
	names []string,
	fileCommands map[string][]bootstrapCommand,
) ([]bootstrapField, error) {
	fields := make([]bootstrapField, 0, len(node.Children))
	usedNames := map[string]bool{}

	for _, name := range names {
		child := node.Children[name]
		if child == nil {
			continue
		}

		fieldName := segmentToIdent(child.Name)

		if usedNames[fieldName] {
			return nil, fmt.Errorf("%w: %q under %q", errDuplicateNamespace, fieldName, node.Name)
		}

		usedNames[fieldName] = true
		fields = append(fields, bootstrapField{
			Name:     fieldName,
			TypeExpr: "*" + child.TypeName,
			TagLit:   subcommandTag(fieldName, child.Name),
		})
	}

	if node.File != "" {
		cmdFields, err := buildCommandFields(node, fileCommands[node.File], usedNames)
		if err != nil {
			return nil, err
		}

		fields = append(fields, cmdFields...)
	}

	return fields, nil
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

// checkDestConflict checks if the destination already exists.
func checkDestConflict(infos []buildtool.PackageInfo, destLeaf string) error {
	for i := range infos {
		for _, cmd := range infos[i].Commands {
			if toKebabCase(cmd.Name) == destLeaf {
				return fmt.Errorf("%w: %q already exists", errMoveDestConflict, destLeaf)
			}
		}
	}

	return nil
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

// collectFileCommands collects commands from package infos into a map by file path.

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

func collectNamespaceNodes(
	root *namespaceNode,
	fileCommands map[string][]bootstrapCommand,
	pkgDocs map[string]string,
	pathMapping map[string]string,
	out *[]bootstrapNode,
) error {
	return walkNamespaceTree(root, root, fileCommands, pkgDocs, pathMapping, out)
}

// collectPackageFilePaths extracts all file paths from module packages.
func collectPackageFilePaths(mt moduleTargets) []string {
	var filePaths []string

	for _, pkg := range mt.Packages {
		for _, f := range pkg.Files {
			filePaths = append(filePaths, f.Path)
		}
	}

	return filePaths
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

	//nolint:gosec,mnd // standard file permissions for source files
	err = os.WriteFile(destPath, []byte(content), 0o644)
	if err != nil {
		return fmt.Errorf("writing destination file: %w", err)
	}

	return nil
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

// createNestedHierarchy creates a full hierarchy of structs for nested destination.
func createNestedHierarchy(
	parentPath []string,
	leaf string,
	exactMatch *buildtool.CommandInfo,
	subcommands []movedCommand,
	targetFile string,
) (string, error) {
	var builder strings.Builder

	// Generate leaf struct first
	leafCode := generateMoveStruct(leaf, exactMatch, subcommands, targetFile)
	builder.WriteString(leafCode)

	// Generate parent structs from innermost to outermost
	childName := toExportedName(leaf)

	for i := len(parentPath) - 1; i >= 0; i-- {
		parentName := toExportedName(parentPath[i])
		builder.WriteString(fmt.Sprintf("\ntype %s struct {\n", parentName))
		builder.WriteString(fmt.Sprintf("\t%s *%s `targ:\"subcommand\"`\n", childName, childName))
		builder.WriteString("}\n")

		childName = parentName
	}

	err := appendToFile(targetFile, builder.String())
	if err != nil {
		return "", err
	}

	dest := strings.Join(append(parentPath, leaf), ".")
	movedNames := buildMovedNames(dest, exactMatch, subcommands)

	return fmt.Sprintf("Added to %s:\n  %s", targetFile, strings.Join(movedNames, "\n  ")), nil
}

// createTargetsFile creates a starter targets file with the build tag.
func createTargetsFile(filename string) (string, error) {
	// Check if file already exists
	_, err := os.Stat(filename)
	if err == nil {
		return "", fmt.Errorf("%w: %s", errFileExists, filename)
	}

	content := `//go:build targ

package targets

import "github.com/toejough/targ/sh"

// Keep the compiler happy - sh is used by generated aliases
var _ = sh.Run
`

	//nolint:gosec,mnd // standard file permissions for targs.go
	err = os.WriteFile(filename, []byte(content), 0o644)
	if err != nil {
		return "", fmt.Errorf("writing %s: %w", filename, err)
	}

	return "Created " + filename, nil
}

// discoverSourcePackage finds the package matching pkgName.
func discoverSourcePackage(
	pkgName string,
) ([]buildtool.PackageInfo, *buildtool.PackageInfo, error) {
	filesystem := buildtool.OSFileSystem{}

	infos, err := buildtool.Discover(filesystem, buildtool.Options{StartDir: "."})
	if err != nil {
		return nil, nil, fmt.Errorf("discovering commands: %w", err)
	}

	for i := range infos {
		if infos[i].Package == pkgName {
			return infos, &infos[i], nil
		}
	}

	return nil, nil, fmt.Errorf("%w: %s", errMovePackageNotFound, pkgName)
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

// ensureShImport ensures the file imports github.com/toejough/targ/sh.
func ensureShImport(path string) error {
	//nolint:gosec // build tool reads source files by design
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file for import check: %w", err)
	}

	content := string(data)

	if strings.Contains(content, `"github.com/toejough/targ/sh"`) {
		return nil
	}

	result := addShImportToContent(content)

	//nolint:gosec,mnd // standard file permissions for source files
	err = os.WriteFile(path, []byte(result), 0o644)
	if err != nil {
		return fmt.Errorf("writing file with import: %w", err)
	}

	return nil
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

func extractSubcommandFields(structType *ast.StructType, filePath string) map[string]subcommandRef {
	result := make(map[string]subcommandRef)

	for _, field := range structType.Fields.List {
		if field.Tag == nil || len(field.Names) == 0 {
			continue
		}

		tagValue := strings.Trim(field.Tag.Value, "`")
		if !strings.Contains(tagValue, "subcommand") {
			continue
		}

		fieldName := field.Names[0].Name
		subName := parseSubcommandName(tagValue, fieldName)

		result[subName] = subcommandRef{
			typeName: fieldTypeName(field.Type),
			file:     filePath,
		}
	}

	return result
}

// extractTargFlags extracts targ-specific flags (--no-cache, --keep) from args.
// Returns the flag values and remaining args to pass to the binary.
func extractTargFlags(args []string) (noCache, keepBootstrap bool, remaining []string) {
	remaining = make([]string, 0, len(args))

	for _, arg := range args {
		switch arg {
		case "--no-cache":
			noCache = true
		case "--keep":
			keepBootstrap = true
		default:
			remaining = append(remaining, arg)
		}
	}

	return noCache, keepBootstrap, remaining
}

// fieldTypeName extracts the type name from an AST expression.
func fieldTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return fieldTypeName(t.X)
	case *ast.SelectorExpr:
		return fieldTypeName(t.Sel)
	default:
		return ""
	}
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

// findMatchingCommands finds commands matching the pattern in the source package.
func findMatchingCommands(
	sourcePackage *buildtool.PackageInfo,
	parentPath []string,
	pattern, prefix, pkgName string,
) (*buildtool.CommandInfo, []movedCommand, error) {
	if len(parentPath) == 0 {
		return findTopLevelMatches(sourcePackage, pattern, prefix)
	}

	return findNestedMatches(sourcePackage, parentPath, pattern, prefix, pkgName)
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

// findNestedMatches finds matching commands within a nested parent structure.
func findNestedMatches(
	sourcePackage *buildtool.PackageInfo,
	parentPath []string,
	pattern, prefix, pkgName string,
) (*buildtool.CommandInfo, []movedCommand, error) {
	currentFile, currentStruct, err := findRootParent(sourcePackage, parentPath[0], pkgName)
	if err != nil {
		return nil, nil, err
	}

	currentFile, currentStruct, err = navigateToParent(currentFile, currentStruct, parentPath[1:])
	if err != nil {
		return nil, nil, err
	}

	return findSubcommandMatches(currentFile, currentStruct, pattern, prefix)
}

// findRootParent finds the root parent command in the package.
func findRootParent(
	sourcePackage *buildtool.PackageInfo,
	parentName, pkgName string,
) (string, string, error) {
	for _, cmd := range sourcePackage.Commands {
		if toKebabCase(cmd.Name) == parentName {
			return cmd.File, cmd.Name, nil
		}
	}

	return "", "", fmt.Errorf("%w: %s in package %s", errMoveCommandNotFound, parentName, pkgName)
}

func findRootParentCommand(
	rootParent string,
	infos []buildtool.PackageInfo,
) (*buildtool.CommandInfo, string) {
	for i := range infos {
		for j := range infos[i].Commands {
			if toKebabCase(infos[i].Commands[j].Name) == rootParent {
				return &infos[i].Commands[j], infos[i].Commands[j].File
			}
		}
	}

	return nil, ""
}

// findStructClosingBrace finds the closing brace position of a named struct.
func findStructClosingBrace(file *ast.File, structName string) token.Pos {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != structName {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			return structType.Fields.Closing
		}
	}

	return 0
}

// findStructSubcommands parses a Go file and returns subcommand info for a struct.
// Returns a map of subcommand kebab-name -> (field name, type name, uses context, file).
func findStructSubcommands(filePath, structName string) (map[string]subcommandRef, error) {
	//nolint:gosec // build tool reads source files by design
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing file %s: %w", filePath, err)
	}

	structType := findStructType(file, structName)
	if structType == nil {
		return make(map[string]subcommandRef), nil
	}

	return extractSubcommandFields(structType, filePath), nil
}

func findStructType(file *ast.File, structName string) *ast.StructType {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != structName {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if ok {
				return structType
			}
		}
	}

	return nil
}

// findSubcommandMatches finds subcommands matching the pattern.
func findSubcommandMatches(
	currentFile, currentStruct, pattern, prefix string,
) (*buildtool.CommandInfo, []movedCommand, error) {
	subs, err := findStructSubcommands(currentFile, currentStruct)
	if err != nil {
		return nil, nil, fmt.Errorf("finding subcommands of %s: %w", currentStruct, err)
	}

	var exactMatch *buildtool.CommandInfo

	var matches []movedCommand

	for subName, subRef := range subs {
		if !matchGlob(subName, pattern) {
			continue
		}

		cmdInfo := buildtool.CommandInfo{
			Name: subRef.typeName,
			Kind: buildtool.CommandStruct,
			File: subRef.file,
		}

		if subName == prefix {
			exactMatch = &cmdInfo
		} else {
			matches = append(matches, movedCommand{
				info:    cmdInfo,
				newName: stripPrefix(subName, prefix),
			})
		}
	}

	return exactMatch, matches, nil
}

// findTargetFiles finds all files with //go:build targ in the given directory.
func findTargetFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
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

// findTopLevelMatches finds matching commands at package level.
func findTopLevelMatches(
	sourcePackage *buildtool.PackageInfo,
	pattern, prefix string,
) (*buildtool.CommandInfo, []movedCommand, error) {
	var exactMatch *buildtool.CommandInfo

	var matches []movedCommand

	for i := range sourcePackage.Commands {
		cmd := &sourcePackage.Commands[i]
		cmdKebab := toKebabCase(cmd.Name)

		if !matchGlob(cmdKebab, pattern) {
			continue
		}

		if cmdKebab == prefix {
			exactMatch = cmd
		} else {
			matches = append(matches, movedCommand{
				info:    *cmd,
				newName: stripPrefix(cmdKebab, prefix),
			})
		}
	}

	return exactMatch, matches, nil
}

// generateAlias creates Go code for a simple shell command target.
func generateAlias(name, command string) (string, error) {
	if name == "" {
		return "", errAliasNameEmpty
	}

	// Convert name to exported Go function name
	funcName := toExportedName(name)

	// Parse command into parts
	parts, err := parseShellCommand(command)
	if err != nil {
		return "", fmt.Errorf("parsing command: %w", err)
	}

	if len(parts) == 0 {
		return "", errCommandEmpty
	}

	// Build sh.Run arguments
	var argsStr strings.Builder

	for i, part := range parts {
		if i > 0 {
			argsStr.WriteString(", ")
		}

		argsStr.WriteString(strconv.Quote(part))
	}

	// Generate the code with leading newline for nice appending
	code := fmt.Sprintf(`
// %s runs %q.
func %s() error {
	return sh.Run(%s)
}
`, funcName, command, funcName, argsStr.String())

	return code, nil
}

// generateModuleBootstrap creates bootstrap code and computes cache key.
func generateModuleBootstrap(
	mt moduleTargets,
	startDir, importRoot string,
) (moduleBootstrap, error) {
	filePaths := collectPackageFilePaths(mt)

	collapsedPaths, err := namespacePaths(filePaths, startDir)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("computing namespace paths: %w", err)
	}

	data, err := buildBootstrapData(
		mt.Packages,
		startDir,
		importRoot,
		mt.ModulePath,
		collapsedPaths,
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

// generateMoveStruct creates a parent struct with moved subcommands.
func generateMoveStruct(
	destName string,
	exactMatch *buildtool.CommandInfo,
	subcommands []movedCommand,
	sourceFile string,
) string {
	structName := toExportedName(destName)

	var builder strings.Builder

	// Generate struct with subcommand fields
	// Use destName in wrapper name to avoid conflicts with targ's internal wrappers
	builder.WriteString(fmt.Sprintf("\ntype %s struct {\n", structName))

	for _, cmd := range subcommands {
		wrapperName := structName + toExportedName(cmd.newName) + "Wrapper"
		builder.WriteString(fmt.Sprintf("\t%s *%s `targ:\"subcommand\"`\n",
			toExportedName(cmd.newName),
			wrapperName))
	}

	builder.WriteString("}\n")

	// Generate Run() method if there was an exact match
	if exactMatch != nil {
		if exactMatch.UsesContext {
			builder.WriteString(
				fmt.Sprintf("\nfunc (c *%s) Run(ctx context.Context) error {\n", structName),
			)
			builder.WriteString(fmt.Sprintf("\treturn %s(ctx)\n", exactMatch.Name))
		} else {
			builder.WriteString(fmt.Sprintf("\nfunc (c *%s) Run() error {\n", structName))
			builder.WriteString(fmt.Sprintf("\treturn %s()\n", exactMatch.Name))
		}

		builder.WriteString("}\n")
	}

	// Generate wrapper structs for each subcommand
	for _, cmd := range subcommands {
		wrapperName := structName + toExportedName(cmd.newName) + "Wrapper"
		builder.WriteString(fmt.Sprintf("\ntype %s struct{}\n", wrapperName))

		if cmd.info.UsesContext {
			builder.WriteString(
				fmt.Sprintf("\nfunc (c *%s) Run(ctx context.Context) error {\n", wrapperName),
			)
			builder.WriteString(fmt.Sprintf("\treturn %s(ctx)\n", cmd.info.Name))
		} else {
			builder.WriteString(fmt.Sprintf("\nfunc (c *%s) Run() error {\n", wrapperName))
			builder.WriteString(fmt.Sprintf("\treturn %s()\n", cmd.info.Name))
		}

		builder.WriteString("}\n")

		// Add Name() method for CLI name
		builder.WriteString(fmt.Sprintf("\nfunc (c *%s) Name() string {\n", wrapperName))
		builder.WriteString(fmt.Sprintf("\treturn %q\n", cmd.newName))
		builder.WriteString("}\n")

		// Add SourceFile() method
		builder.WriteString(fmt.Sprintf("\nfunc (c *%s) SourceFile() string {\n", wrapperName))
		builder.WriteString(fmt.Sprintf("\treturn %q\n", sourceFile))
		builder.WriteString("}\n")
	}

	return builder.String()
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

// handleAliasFlag checks for --alias and generates target code.
// Returns nil if --alias was not specified.
func handleAliasFlag(args []string) *aliasResult {
	for i, arg := range args {
		name, command, targetFile, ok, err := parseAliasArgs(args, i, arg)
		if !ok {
			continue
		}

		if err != nil {
			return &aliasResult{err: err}
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
			filename := targsGoFilename
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				filename = args[i+1]
			}

			msg, err := createTargetsFile(filename)

			return &initResult{message: msg, err: err}
		}

		if after, ok := strings.CutPrefix(arg, "--init="); ok {
			filename := after
			if filename == "" {
				filename = "targs.go"
			}

			msg, err := createTargetsFile(filename)

			return &initResult{message: msg, err: err}
		}
	}

	return nil
}

// handleMoveFlag checks for --move and generates hierarchy code.
// Returns nil if --move was not specified.
func handleMoveFlag(args []string) *moveResult {
	for i, arg := range args {
		if arg != "--move" {
			continue
		}

		// Need at least 2 more args: DEST SOURCE_PATTERN
		if i+2 >= len(args) {
			return &moveResult{err: errMoveRequiresArgs}
		}

		dest := args[i+1]
		sourcePattern := args[i+2]

		msg, err := moveCommands(dest, sourcePattern)

		return &moveResult{message: msg, err: err}
	}

	return nil
}

func hasImportAhead(lines []string, index int) bool {
	for j := index + 1; j < len(lines); j++ {
		nextTrimmed := strings.TrimSpace(lines[j])
		if nextTrimmed == "" {
			continue
		}

		return strings.HasPrefix(nextTrimmed, "import")
	}

	return false
}

// hasTargBuildTag checks if a file has the //go:build targ tag.
func hasTargBuildTag(path string) bool {
	//nolint:gosec // build tool reads source files by design
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	content := string(data)
	// Check for //go:build targ (with possible other tags)
	for line := range strings.SplitSeq(content, "\n") {
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

func lowerFirst(name string) string {
	if name == "" {
		return pkgNameDefault
	}

	return strings.ToLower(name[:1]) + name[1:]
}

// matchGlob matches a command name against a glob pattern.
// Only supports trailing * for simplicity.
func matchGlob(name, pattern string) bool {
	if before, ok := strings.CutSuffix(pattern, "*"); ok {
		return strings.HasPrefix(name, before)
	}

	return name == pattern
}

// moveCommands implements the --move flag logic.
// Supports nested paths: source can be pkg.parent.pattern*, dest can be parent.child.
func moveCommands(dest, sourcePattern string) (string, error) {
	sourceParts := strings.Split(sourcePattern, ".")
	if len(sourceParts) < minSourcePatternParts {
		return "", errMoveInvalidSourcePattern
	}

	pkgName := sourceParts[0]
	pattern := sourceParts[len(sourceParts)-1]
	parentPath := sourceParts[1 : len(sourceParts)-1]

	destParts := strings.Split(dest, ".")
	destLeaf := destParts[len(destParts)-1]
	destParentPath := destParts[:len(destParts)-1]

	infos, sourcePackage, err := discoverSourcePackage(pkgName)
	if err != nil {
		return "", err
	}

	if len(sourcePackage.Files) == 0 {
		return "", fmt.Errorf("%w: %s", errMoveNoSourceFiles, pkgName)
	}

	targetFile := sourcePackage.Files[0].Path
	prefix := strings.TrimSuffix(pattern, "*")

	exactMatch, matchingCommands, err := findMatchingCommands(
		sourcePackage, parentPath, pattern, prefix, pkgName,
	)
	if err != nil {
		return "", err
	}

	if exactMatch == nil && len(matchingCommands) == 0 {
		return "", errMoveNoMatches
	}

	if len(destParentPath) > 0 {
		return moveToNestedDest(
			destParentPath,
			destLeaf,
			exactMatch,
			matchingCommands,
			targetFile,
			infos,
		)
	}

	return moveToTopLevel(destLeaf, dest, exactMatch, matchingCommands, targetFile, infos)
}

// moveToNestedDest handles moving commands to a nested destination like tools.lint.
func moveToNestedDest(
	destParentPath []string,
	destLeaf string,
	exactMatch *buildtool.CommandInfo,
	subcommands []movedCommand,
	targetFile string,
	infos []buildtool.PackageInfo,
) (string, error) {
	parentCmd, parentFile := findRootParentCommand(destParentPath[0], infos)

	if parentCmd == nil {
		return createNestedHierarchy(destParentPath, destLeaf, exactMatch, subcommands, targetFile)
	}

	currentFile, currentStruct, err := navigateToImmediateParent(
		destParentPath,
		parentFile,
		parentCmd.Name,
	)
	if err != nil {
		return "", err
	}

	return addMoveStructUnderParent(
		destParentPath, destLeaf, exactMatch, subcommands,
		targetFile, currentFile, currentStruct,
	)
}

// moveToTopLevel handles moving commands to a top-level destination.
func moveToTopLevel(
	destLeaf, dest string,
	exactMatch *buildtool.CommandInfo,
	matchingCommands []movedCommand,
	targetFile string,
	infos []buildtool.PackageInfo,
) (string, error) {
	err := checkDestConflict(infos, destLeaf)
	if err != nil {
		return "", err
	}

	code := generateMoveStruct(destLeaf, exactMatch, matchingCommands, targetFile)

	err = appendToFile(targetFile, code)
	if err != nil {
		return "", err
	}

	movedNames := buildMovedNames(dest, exactMatch, matchingCommands)

	return fmt.Sprintf("Added to %s:\n  %s", targetFile, strings.Join(movedNames, "\n  ")), nil
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

func navigateToImmediateParent(
	destParentPath []string,
	startFile, startStruct string,
) (string, string, error) {
	currentFile := startFile
	currentStruct := startStruct

	for i := 1; i < len(destParentPath); i++ {
		subs, err := findStructSubcommands(currentFile, currentStruct)
		if err != nil {
			return "", "", fmt.Errorf("finding subcommands of %s: %w", currentStruct, err)
		}

		subRef, ok := subs[destParentPath[i]]
		if !ok {
			return "", "", fmt.Errorf(
				"%w: %s in %s",
				errMoveCommandNotFound,
				destParentPath[i],
				currentStruct,
			)
		}

		currentFile = subRef.file
		currentStruct = subRef.typeName
	}

	return currentFile, currentStruct, nil
}

// navigateToParent navigates through intermediate parent path segments.
func navigateToParent(currentFile, currentStruct string, path []string) (string, string, error) {
	for _, segment := range path {
		subs, err := findStructSubcommands(currentFile, currentStruct)
		if err != nil {
			return "", "", fmt.Errorf("finding subcommands of %s: %w", currentStruct, err)
		}

		subRef, ok := subs[segment]
		if !ok {
			return "", "", fmt.Errorf(
				"%w: %s in %s",
				errMoveCommandNotFound,
				segment,
				currentStruct,
			)
		}

		currentFile = subRef.file
		currentStruct = subRef.typeName
	}

	return currentFile, currentStruct, nil
}

func newBootstrapBuilder(absStart, moduleRoot, modulePath string) *bootstrapBuilder {
	return &bootstrapBuilder{
		absStart:     absStart,
		moduleRoot:   moduleRoot,
		modulePath:   modulePath,
		imports:      []bootstrapImport{{Path: "github.com/toejough/targ"}},
		usedImports:  map[string]bool{"github.com/toejough/targ": true},
		fileCommands: make(map[string][]bootstrapCommand),
		wrapperNames: &nameGenerator{},
	}
}

// parseAliasArgs extracts alias name, command, and optional file from arguments.
// Returns ok=false if this is not an alias invocation.
// Returns ok=true with error if alias is malformed.
func parseAliasArgs(
	args []string,
	i int,
	arg string,
) (name, command, targetFile string, ok bool, err error) {
	if arg == "--alias" {
		if i+2 >= len(args) {
			return "", "", "", true, errAliasRequiresArgs
		}

		name = args[i+1]
		command = args[i+2]
		// Optional third argument for target file
		if i+3 < len(args) && !strings.HasPrefix(args[i+3], "-") {
			targetFile = args[i+3]
		}

		return name, command, targetFile, true, nil
	}

	after, found := strings.CutPrefix(arg, "--alias=")
	if !found {
		return "", "", "", false, nil
	}

	// --alias=name "command" [file] format
	if i+1 >= len(args) {
		return "", "", "", true, errAliasRequiresCommand
	}

	name = after

	command = args[i+1]
	if i+2 < len(args) && !strings.HasPrefix(args[i+2], "-") {
		targetFile = args[i+2]
	}

	return name, command, targetFile, true, nil
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

func parseShellCommand(cmd string) ([]string, error) {
	p := &shellCommandParser{}

	for _, r := range cmd {
		p.processRune(r)
	}

	return p.finalize()
}

func parseSubcommandName(tagValue, fieldName string) string {
	subName := toKebabCase(fieldName)

	_, after, found := strings.Cut(tagValue, "subcommand=")
	if !found {
		return subName
	}

	if before, _, ok := strings.Cut(after, ","); ok {
		return before
	}

	if before, _, ok := strings.Cut(after, "\""); ok {
		return before
	}

	return after
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

func printBuildToolUsage(out io.Writer) {
	_, _ = fmt.Fprintln(
		out,
		"targ is a build-tool runner that discovers tagged commands and executes them.",
	)
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Usage: targ [FLAGS...] COMMAND [COMMAND_ARGS...]")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Flags:")
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "--no-cache", "disable cached build tool binaries")
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "--keep", "keep generated bootstrap file")
	_, _ = fmt.Fprintf(
		out,
		"    %-28s %s\n",
		"--timeout <duration>",
		"set execution timeout (e.g., 10m, 1h)",
	)
	_, _ = fmt.Fprintf(
		out,
		"    %-28s %s\n",
		"--completion {bash|zsh|fish}",
		"print completion script for specified shell. Uses the current shell if none is",
	)
	_, _ = fmt.Fprintf(
		out,
		"    %-28s %s\n",
		"",
		"specified. The output should be eval'd/sourced in the shell to enable completions.",
	)
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "", "(e.g. 'targ --completion fish | source')")
	_, _ = fmt.Fprintf(
		out,
		"    %-28s %s\n",
		"--init [FILE]",
		"create a starter targets file (default: targs.go)",
	)
	_, _ = fmt.Fprintf(
		out,
		"    %-28s %s\n",
		"--alias NAME \"CMD\" [FILE]",
		"add a shell command target to a targets file",
	)
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "", "(auto-creates targs.go if no targets exist)")
	_, _ = fmt.Fprintf(
		out,
		"    %-28s %s\n",
		"--move <dest_path> <source_pattern>",
		"reorganize commands into a hierarchy",
	)
	_, _ = fmt.Fprintf(
		out,
		"    %-28s %s\n",
		"",
		"(e.g. 'targ --move lint dev.lint*' groups lint* commands)",
	)
	_, _ = fmt.Fprintf(out, "    %-28s %s\n", "--help", "Print help information")
}

// Find max name length for alignment

// Indent for continuation lines: 4 leading + name width + 2 padding + 1 space + 2 extra

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
// This allows users to discover flags like --init even before creating targets.
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
		"--no-cache",
		"--keep",
		"--completion",
		"--init",
		"--alias",
		"--move",
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
			Package:  info.Package,
			Doc:      info.Doc,
			Commands: info.Commands,
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
				Path:     newPath,
				Base:     f.Base,
				Commands: f.Commands,
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

func sameDir(a, b string) bool {
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

func singlePackageBanner(info buildtool.PackageInfo) string {
	lines := []string{
		fmt.Sprintf("Loaded tasks from package %q.", info.Package),
	}

	doc := strings.TrimSpace(info.Doc)
	if doc != "" {
		lines = append(lines, doc)
	}

	lines = append(lines, "Path: "+info.Dir)

	return strings.Join(lines, "\n")
}

// skipIfVendorOrGit returns SkipDir for .git and vendor directories.
func skipIfVendorOrGit(name string) error {
	if name == ".git" || name == "vendor" {
		return filepath.SkipDir
	}

	return nil
}

func sortedChildNames(node *namespaceNode) []string {
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
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

// stripPrefix removes the pattern prefix from a command name.
// e.g., stripPrefix("lint-fast", "lint") → "fast"
//
//	stripPrefix("lint", "lint") → "lint" (exact match preserved)
func stripPrefix(name, prefix string) string {
	if name == prefix {
		return name
	}

	if after, ok := strings.CutPrefix(name, prefix+"-"); ok {
		return after
	}

	return name
}

func subcommandTag(fieldName, segment string) string {
	if camelToKebab(fieldName) == segment {
		return `targ:"subcommand"`
	}

	return fmt.Sprintf(`targ:"subcommand,name=%s"`, segment)
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

// toKebabCase converts CamelCase to kebab-case.
func toKebabCase(s string) string {
	var result strings.Builder

	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			if unicode.IsLower(prev) || (i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
				result.WriteRune('-')
			}
		}

		result.WriteRune(unicode.ToLower(r))
	}

	return result.String()
}

func touchFile(path string) error {
	//nolint:gosec,mnd // standard file permissions for go.sum
	err := os.WriteFile(path, []byte{}, 0o644)
	if err != nil {
		return fmt.Errorf("touching file %s: %w", path, err)
	}

	return nil
}

func tryAddAfterPackage(trimmed string, lines []string, index int, result *[]string) bool {
	if !strings.HasPrefix(trimmed, "package ") {
		return false
	}

	if hasImportAhead(lines, index) {
		return false
	}

	*result = append(*result, "")
	*result = append(*result, `import "github.com/toejough/targ/sh"`)

	return true
}

func tryAddImportBlock(trimmed string, result *[]string) bool {
	if trimmed != "import (" {
		return false
	}

	*result = append(*result, `	"github.com/toejough/targ/sh"`)

	return true
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

func tryConvertSingleImport(trimmed string, result *[]string) bool {
	if !strings.HasPrefix(trimmed, "import \"") {
		return false
	}

	(*result)[len(*result)-1] = "import ("
	*result = append(*result, "\t"+strings.TrimPrefix(trimmed, "import "))
	*result = append(*result, `	"github.com/toejough/targ/sh"`)
	*result = append(*result, ")")

	return true
}

func uniqueImportName(name string, used map[string]bool) string {
	candidate := name
	if candidate == "" {
		candidate = pkgNameDefault
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

// validateNoPackageMain ensures no targ files use package main.
func validateNoPackageMain(mt moduleTargets) error {
	for _, pkg := range mt.Packages {
		if pkg.Package == "main" {
			return fmt.Errorf(
				"%w (found in %s); use a named package instead",
				errPackageMainNotAllowed,
				pkg.Dir,
			)
		}
	}

	return nil
}

func walkNamespaceTree(
	node, root *namespaceNode,
	fileCommands map[string][]bootstrapCommand,
	pkgDocs map[string]string,
	pathMapping map[string]string,
	out *[]bootstrapNode,
) error {
	names := sortedChildNames(node)

	for _, name := range names {
		child := node.Children[name]
		if child == nil {
			continue
		}

		err := walkNamespaceTree(child, root, fileCommands, pkgDocs, pathMapping, out)
		if err != nil {
			return err
		}
	}

	if node == root {
		return nil
	}

	fields, err := buildNamespaceFields(node, names, fileCommands)
	if err != nil {
		return err
	}

	// Look up package doc for this namespace
	description := ""

	sourceFile := node.File
	if node.File != "" {
		description = pkgDocs[node.File]
		// Map isolated path to original path if available
		if pathMapping != nil {
			if original, ok := pathMapping[node.File]; ok {
				sourceFile = original
			}
		}
	}

	*out = append(*out, bootstrapNode{
		Name:        node.Name,
		TypeName:    node.TypeName,
		VarName:     node.VarName,
		Description: description,
		SourceFile:  sourceFile,
		Fields:      fields,
	})

	return nil
}

func writeBootstrapFile(dir string, data []byte, keep bool) (string, func() error, error) {
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
		if keep {
			return nil
		}

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

	//nolint:gosec,mnd // standard file permissions for go.mod
	err := os.WriteFile(modPath, []byte(content), 0o644)
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

	//nolint:gosec,mnd // standard file permissions for go.mod
	err := os.WriteFile(modPath, []byte(content), 0o644)
	if err != nil {
		return fmt.Errorf("writing isolated go.mod: %w", err)
	}

	// Touch go.sum file
	sumPath := filepath.Join(tmpDir, "go.sum")

	//nolint:gosec,mnd // standard file permissions for go.sum
	err = os.WriteFile(sumPath, []byte{}, 0o644)
	if err != nil {
		return fmt.Errorf("writing go.sum: %w", err)
	}

	return nil
}

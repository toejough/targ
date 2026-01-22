// Package buildtool provides utilities for discovering and generating code for targ commands.
package buildtool

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/toejough/targ/buildtool/internal/parse"
)

// Exported constants.
const (
	CommandFunc   CommandKind = "func"
	CommandStruct CommandKind = "struct"
)

// Exported variables.
var (
	ErrDuplicateCommand       = errors.New("duplicate command name")
	ErrMainFunctionNotAllowed = errors.New("tagged files must not declare main()")
	ErrMultiplePackageNames   = errors.New("multiple package names")
	ErrNoTaggedFiles          = errors.New("no tagged files found")
)

// CommandInfo describes a discovered command within a targ file.
type CommandInfo struct {
	Name         string
	Kind         CommandKind
	File         string
	Description  string
	UsesContext  bool
	ReturnsError bool
}

// CommandKind indicates whether a command is a function or struct.
type CommandKind string

// FileInfo describes a file containing targ commands.
type FileInfo struct {
	Path     string
	Base     string
	Commands []CommandInfo
}

// FileSystem abstracts file operations for testability.
type FileSystem interface {
	ReadDir(name string) ([]fs.DirEntry, error)
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

// OSFileSystem implements FileSystem using the os package.
type OSFileSystem struct{}

// ReadDir reads the named directory.
func (OSFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(name)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", name, err)
	}

	return entries, nil
}

// ReadFile reads the named file.
func (OSFileSystem) ReadFile(name string) ([]byte, error) {
	//nolint:gosec // build tool reads user source files by design
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", name, err)
	}

	return data, nil
}

// WriteFile writes data to the named file.
func (OSFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	err := os.WriteFile(name, data, perm)
	if err != nil {
		return fmt.Errorf("writing file %s: %w", name, err)
	}

	return nil
}

// Options configures the Discover function.
type Options struct {
	StartDir string
	BuildTag string
}

// PackageInfo describes a package containing targ commands.
type PackageInfo struct {
	Dir                      string
	Package                  string
	Doc                      string
	Commands                 []CommandInfo
	Files                    []FileInfo
	UsesExplicitRegistration bool // true if package has init() calling targ.Register()
}

// TaggedDir represents a directory containing tagged files.
type TaggedDir struct {
	Path  string
	Depth int
}

// TaggedFile represents a file with a targ build tag.
type TaggedFile struct {
	Path    string
	Content []byte
}

// Discover finds all packages with targ-tagged files and parses their commands.
func Discover(filesystem FileSystem, opts Options) ([]PackageInfo, error) {
	startDir := opts.StartDir
	if startDir == "" {
		startDir = "."
	}

	tag := opts.BuildTag
	if tag == "" {
		tag = defaultBuildTag
	}

	dirs, err := findTaggedDirs(filesystem, startDir, tag)
	if err != nil {
		return nil, err
	}

	infos := make([]PackageInfo, 0, len(dirs))

	for _, dir := range dirs {
		info, err := parsePackageInfo(dir)
		if err != nil {
			return nil, err
		}

		infos = append(infos, info)
	}

	return infos, nil
}

// SelectTaggedDirs returns directories containing targ-tagged files.
func SelectTaggedDirs(filesystem FileSystem, opts Options) ([]TaggedDir, error) {
	startDir := opts.StartDir
	if startDir == "" {
		startDir = "."
	}

	tag := opts.BuildTag
	if tag == "" {
		tag = defaultBuildTag
	}

	dirs, err := findTaggedDirs(filesystem, startDir, tag)
	if err != nil {
		return nil, err
	}

	paths := make([]TaggedDir, 0, len(dirs))
	for _, dir := range dirs {
		paths = append(paths, TaggedDir{Path: dir.Path, Depth: dir.Depth})
	}

	return paths, nil
}

// TaggedFiles returns all files with the specified build tag.
func TaggedFiles(filesystem FileSystem, opts Options) ([]TaggedFile, error) {
	startDir := opts.StartDir
	if startDir == "" {
		startDir = "."
	}

	tag := opts.BuildTag
	if tag == "" {
		tag = defaultBuildTag
	}

	dirs, err := findTaggedDirs(filesystem, startDir, tag)
	if err != nil {
		return nil, err
	}

	var files []TaggedFile

	for _, dir := range dirs {
		for _, file := range dir.Files {
			files = append(files, TaggedFile(file))
		}
	}

	return files, nil
}

// unexported constants.
const (
	defaultBuildTag = "targ"
	runMethodName   = "Run"
)

type dirQueueEntry struct {
	path  string
	depth int
}

// packageInfoParser holds state for parsing package info from tagged files.
type packageInfoParser struct {
	dir                      taggedDir
	fset                     *token.FileSet
	packageName              string
	packageDoc               string
	structs                  map[string]bool
	structFiles              map[string]string
	structHasSubcommands     map[string]bool
	structHasRun             map[string]bool
	structDescriptions       map[string]string
	funcs                    map[string]bool
	funcFiles                map[string]string
	funcDescriptions         map[string]string
	funcUsesContext          map[string]bool
	funcReturnsError         map[string]bool
	subcommandNames          map[string]bool
	subcommandTypes          map[string]bool
	mainFiles                []string
	structList               []string
	funcList                 []string
	usesExplicitRegistration bool
}

// buildCommands creates CommandInfo for all structs and funcs.
func (p *packageInfoParser) buildCommands() ([]CommandInfo, map[string][]CommandInfo) {
	commands := make([]CommandInfo, 0, len(p.structList)+len(p.funcList))
	fileCommands := make(map[string][]CommandInfo)

	for _, name := range p.structList {
		cmd := CommandInfo{
			Name:        name,
			Kind:        CommandStruct,
			File:        p.structFiles[name],
			Description: p.structDescriptions[name],
		}
		commands = append(commands, cmd)
		fileCommands[cmd.File] = append(fileCommands[cmd.File], cmd)
	}

	for _, name := range p.funcList {
		cmd := CommandInfo{
			Name:         name,
			Kind:         CommandFunc,
			File:         p.funcFiles[name],
			Description:  p.funcDescriptions[name],
			UsesContext:  p.funcUsesContext[name],
			ReturnsError: p.funcReturnsError[name],
		}
		commands = append(commands, cmd)
		fileCommands[cmd.File] = append(fileCommands[cmd.File], cmd)
	}

	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})

	return commands, fileCommands
}

// buildFiles creates FileInfo from the file-grouped commands.
func (p *packageInfoParser) buildFiles(fileCommands map[string][]CommandInfo) []FileInfo {
	files := make([]FileInfo, 0, len(fileCommands))

	for path, cmds := range fileCommands {
		sort.Slice(cmds, func(i, j int) bool {
			return cmds[i].Name < cmds[j].Name
		})
		files = append(files, FileInfo{
			Path:     path,
			Base:     strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
			Commands: cmds,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files
}

// buildResult constructs the final PackageInfo.
func (p *packageInfoParser) buildResult() PackageInfo {
	commands, fileCommands := p.buildCommands()
	files := p.buildFiles(fileCommands)

	return PackageInfo{
		Dir:                      p.dir.Path,
		Package:                  p.packageName,
		Doc:                      p.packageDoc,
		Commands:                 commands,
		Files:                    files,
		UsesExplicitRegistration: p.usesExplicitRegistration,
	}
}

// checkDuplicates ensures no duplicate command names exist.
func (p *packageInfoParser) checkDuplicates() error {
	seen := make(map[string]string)

	for _, name := range p.structList {
		seen[parse.CamelToKebab(name)] = name
	}

	for _, name := range p.funcList {
		cmd := parse.CamelToKebab(name)
		if other, ok := seen[cmd]; ok {
			return fmt.Errorf("%w: %q from %s and %s", ErrDuplicateCommand, cmd, other, name)
		}
	}

	return nil
}

// checkPackageName validates and records the package name.
func (p *packageInfoParser) checkPackageName(parsed *ast.File, _ string) error {
	if p.packageName == "" {
		p.packageName = parsed.Name.Name
	}

	// Capture package doc from any file that has it (first one wins)
	if p.packageDoc == "" && parsed.Doc != nil {
		p.packageDoc = strings.TrimSpace(parsed.Doc.Text())
	}

	if p.packageName != parsed.Name.Name {
		return fmt.Errorf("%w: %s", ErrMultiplePackageNames, p.dir.Path)
	}

	return nil
}

// filterCandidates builds the filtered lists of structs and funcs.
func (p *packageInfoParser) filterCandidates() {
	p.structList = filterStructs(
		p.structs,
		p.subcommandTypes,
		p.structHasRun,
		p.structHasSubcommands,
	)
	p.funcList = filterCommands(p.funcs, p.subcommandNames)
	p.removeWrappedFuncs()
}

// parseFile parses a single file and extracts declarations.
func (p *packageInfoParser) parseFile(file taggedFile) error {
	parsed, err := parser.ParseFile(p.fset, file.Path, file.Content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing file %s: %w", file.Path, err)
	}

	err = p.checkPackageName(parsed, file.Path)
	if err != nil {
		return err
	}

	ctxAliases, ctxDotImport := parse.ContextImportInfo(parsed.Imports)
	targAliases := parse.TargImportInfo(parsed.Imports)

	for _, decl := range parsed.Decls {
		err := p.processDecl(decl, file.Path, ctxAliases, ctxDotImport, targAliases)
		if err != nil {
			return err
		}
	}

	return nil
}

// parseFiles parses all files in the tagged directory.
func (p *packageInfoParser) parseFiles() error {
	for _, file := range p.dir.Files {
		err := p.parseFile(file)
		if err != nil {
			return err
		}
	}

	return nil
}

// processDecl handles a single declaration.
func (p *packageInfoParser) processDecl(
	decl ast.Decl,
	filePath string,
	ctxAliases map[string]bool,
	ctxDotImport bool,
	targAliases map[string]bool,
) error {
	switch node := decl.(type) {
	case *ast.GenDecl:
		p.processGenDecl(node, filePath)
	case *ast.FuncDecl:
		return p.processFuncDecl(node, filePath, ctxAliases, ctxDotImport, targAliases)
	}

	return nil
}

// processExportedFunc validates and records an exported function.
func (p *packageInfoParser) processExportedFunc(
	node *ast.FuncDecl,
	filePath string,
	ctxAliases map[string]bool,
	ctxDotImport bool,
) error {
	err := parse.ValidateFunctionSignature(node.Type, ctxAliases, ctxDotImport)
	if err != nil {
		return fmt.Errorf("function %s %w", node.Name.Name, err)
	}

	name := node.Name.Name
	p.funcs[name] = true
	p.funcFiles[name] = filePath
	p.funcUsesContext[name] = parse.FunctionUsesContext(node.Type, ctxAliases, ctxDotImport)
	p.funcReturnsError[name] = parse.FunctionReturnsError(node.Type)

	if desc, ok := parse.FunctionDocValue(node); ok {
		p.funcDescriptions[name] = desc
	}

	return nil
}

// processFuncDecl processes a function declaration.
func (p *packageInfoParser) processFuncDecl(
	node *ast.FuncDecl,
	filePath string,
	ctxAliases map[string]bool,
	ctxDotImport bool,
	targAliases map[string]bool,
) error {
	if node.Recv != nil {
		p.processMethod(node)
		return nil
	}

	if node.Name.Name == "main" {
		p.mainFiles = append(p.mainFiles, filePath)
		return nil
	}

	if node.Name.Name == "init" {
		if containsTargRegisterCall(node.Body, targAliases) {
			p.usesExplicitRegistration = true
		}

		return nil
	}

	if !node.Name.IsExported() {
		return nil
	}

	return p.processExportedFunc(node, filePath, ctxAliases, ctxDotImport)
}

// processGenDecl processes a general declaration (types).
func (p *packageInfoParser) processGenDecl(node *ast.GenDecl, filePath string) {
	for _, spec := range node.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok || !typeSpec.Name.IsExported() {
			continue
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}

		p.structs[typeSpec.Name.Name] = true
		p.structFiles[typeSpec.Name.Name] = filePath

		if parse.RecordSubcommandRefs(structType, p.subcommandNames, p.subcommandTypes) {
			p.structHasSubcommands[typeSpec.Name.Name] = true
		}
	}
}

// processMethod handles method declarations on structs.
func (p *packageInfoParser) processMethod(node *ast.FuncDecl) {
	if desc, ok := parse.DescriptionMethodValue(node); ok {
		if recvName := parse.ReceiverTypeName(node.Recv); recvName != "" {
			p.structDescriptions[recvName] = desc
		}
	}

	if node.Name.Name == runMethodName {
		if recvName := parse.ReceiverTypeName(node.Recv); recvName != "" {
			p.structHasRun[recvName] = true
		}
	}
}

// removeWrappedFuncs removes functions that have XCommand struct wrappers.
func (p *packageInfoParser) removeWrappedFuncs() {
	if len(p.structList) == 0 || len(p.funcList) == 0 {
		return
	}

	wrapped := make(map[string]bool)

	for _, name := range p.structList {
		if before, ok := strings.CutSuffix(name, "Command"); ok && before != "" {
			wrapped[before] = true
		}
	}

	if len(wrapped) == 0 {
		return
	}

	filtered := make([]string, 0, len(p.funcList))

	for _, name := range p.funcList {
		if !wrapped[name] {
			filtered = append(filtered, name)
		}
	}

	p.funcList = filtered
}

// validate checks that parsing produced valid results.
func (p *packageInfoParser) validate() error {
	if len(p.mainFiles) == 0 {
		return nil
	}

	sort.Strings(p.mainFiles)

	return fmt.Errorf("%w: %s", ErrMainFunctionNotAllowed, strings.Join(p.mainFiles, ", "))
}

type taggedDir struct {
	Path  string
	Depth int
	Files []taggedFile
}

type taggedFile struct {
	Path    string
	Content []byte
}

// containsTargRegisterCall checks if a function body contains a targ.Register() call.
func containsTargRegisterCall(body *ast.BlockStmt, targAliases map[string]bool) bool {
	if body == nil {
		return false
	}

	found := false

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if sel.Sel.Name != "Register" {
			return true
		}

		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		if targAliases[ident.Name] {
			found = true

			return false
		}

		return true
	})

	return found
}

func filterCommands(candidates, subcommandNames map[string]bool) []string {
	var result []string

	for name := range candidates {
		cmd := parse.CamelToKebab(name)
		if !subcommandNames[cmd] {
			result = append(result, name)
		}
	}

	sort.Strings(result)

	return result
}

func filterStructs(
	candidates, subcommandTypes, structHasRun, structHasSubcommands map[string]bool,
) []string {
	var result []string

	for name := range candidates {
		if !subcommandTypes[name] {
			if structHasRun[name] || structHasSubcommands[name] {
				result = append(result, name)
			}
		}
	}

	sort.Strings(result)

	return result
}

func findTaggedDirs(filesystem FileSystem, startDir, tag string) ([]taggedDir, error) {
	queue := []dirQueueEntry{{path: startDir, depth: 0}}

	var results []taggedDir

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		tagged, newDirs, err := processDirectory(filesystem, current, tag)
		if err != nil {
			return nil, err
		}

		queue = append(queue, newDirs...)

		if len(tagged) > 0 {
			results = append(results, taggedDir{
				Path:  current.path,
				Depth: current.depth,
				Files: tagged,
			})
		}
	}

	return results, nil
}

// newPackageInfoParser creates a new parser with initialized state.
func newPackageInfoParser(dir taggedDir) *packageInfoParser {
	return &packageInfoParser{
		dir:                  dir,
		fset:                 token.NewFileSet(),
		structs:              make(map[string]bool),
		structFiles:          make(map[string]string),
		structHasSubcommands: make(map[string]bool),
		structHasRun:         make(map[string]bool),
		structDescriptions:   make(map[string]string),
		funcs:                make(map[string]bool),
		funcFiles:            make(map[string]string),
		funcDescriptions:     make(map[string]string),
		funcUsesContext:      make(map[string]bool),
		funcReturnsError:     make(map[string]bool),
		subcommandNames:      make(map[string]bool),
		subcommandTypes:      make(map[string]bool),
	}
}

func parsePackageInfo(dir taggedDir) (PackageInfo, error) {
	p := newPackageInfoParser(dir)

	err := p.parseFiles()
	if err != nil {
		return PackageInfo{}, err
	}

	err = p.validate()
	if err != nil {
		return PackageInfo{}, err
	}

	p.filterCandidates()

	err = p.checkDuplicates()
	if err != nil {
		return PackageInfo{}, err
	}

	return p.buildResult(), nil
}

func processDirectory(
	filesystem FileSystem,
	current dirQueueEntry,
	tag string,
) ([]taggedFile, []dirQueueEntry, error) {
	entries, err := filesystem.ReadDir(current.path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading directory %s: %w", current.path, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var tagged []taggedFile

	var subdirs []dirQueueEntry

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(current.path, name)

		if entry.IsDir() {
			if !parse.ShouldSkipDir(name) {
				subdirs = append(subdirs, dirQueueEntry{path: fullPath, depth: current.depth + 1})
			}

			continue
		}

		if file, ok := tryReadTaggedFile(filesystem, fullPath, name, tag); ok {
			tagged = append(tagged, file)
		}
	}

	return tagged, subdirs, nil
}

func tryReadTaggedFile(filesystem FileSystem, fullPath, name, tag string) (taggedFile, bool) {
	if parse.ShouldSkipGoFile(name) {
		return taggedFile{}, false
	}

	content, err := filesystem.ReadFile(fullPath)
	if err != nil {
		return taggedFile{}, false
	}

	if !parse.HasBuildTag(content, tag) {
		return taggedFile{}, false
	}

	return taggedFile{
		Path:    fullPath,
		Content: content,
	}, true
}

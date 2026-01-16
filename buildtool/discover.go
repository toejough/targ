package buildtool

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Exported constants.
const (
	CommandFunc   CommandKind = "func"
	CommandStruct CommandKind = "struct"
)

// Exported variables.
var (
	ErrNoTaggedFiles = errors.New("no tagged files found")
)

type CommandInfo struct {
	Name         string
	Kind         CommandKind
	File         string
	Description  string
	UsesContext  bool
	ReturnsError bool
}

type CommandKind string

type FileInfo struct {
	Path     string
	Base     string
	Commands []CommandInfo
}

type FileSystem interface {
	ReadDir(name string) ([]fs.DirEntry, error)
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

type OSFileSystem struct{}

func (OSFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}

func (OSFileSystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (OSFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}

type Options struct {
	StartDir string
	BuildTag string
}

type PackageInfo struct {
	Dir      string
	Package  string
	Doc      string
	Commands []CommandInfo
	Files    []FileInfo
}

type TaggedDir struct {
	Path  string
	Depth int
}

type TaggedFile struct {
	Path    string
	Content []byte
}

func Discover(fs FileSystem, opts Options) ([]PackageInfo, error) {
	startDir := opts.StartDir
	if startDir == "" {
		startDir = "."
	}

	tag := opts.BuildTag
	if tag == "" {
		tag = "targ"
	}

	dirs, err := findTaggedDirs(fs, startDir, tag)
	if err != nil {
		return nil, err
	}

	var infos []PackageInfo

	for _, dir := range dirs {
		info, err := parsePackageInfo(dir)
		if err != nil {
			return nil, err
		}

		infos = append(infos, info)
	}

	return infos, nil
}

func SelectTaggedDirs(fs FileSystem, opts Options) ([]TaggedDir, error) {
	startDir := opts.StartDir
	if startDir == "" {
		startDir = "."
	}

	tag := opts.BuildTag
	if tag == "" {
		tag = "targ"
	}

	dirs, err := findTaggedDirs(fs, startDir, tag)
	if err != nil {
		return nil, err
	}

	paths := make([]TaggedDir, 0, len(dirs))
	for _, dir := range dirs {
		paths = append(paths, TaggedDir{Path: dir.Path, Depth: dir.Depth})
	}

	return paths, nil
}

func TaggedFiles(fs FileSystem, opts Options) ([]TaggedFile, error) {
	startDir := opts.StartDir
	if startDir == "" {
		startDir = "."
	}

	tag := opts.BuildTag
	if tag == "" {
		tag = "targ"
	}

	dirs, err := findTaggedDirs(fs, startDir, tag)
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

type dirQueueEntry struct {
	path  string
	depth int
}

// packageInfoParser holds state for parsing package info from tagged files.
type packageInfoParser struct {
	dir                  taggedDir
	fset                 *token.FileSet
	packageName          string
	packageDoc           string
	structs              map[string]bool
	structFiles          map[string]string
	structHasSubcommands map[string]bool
	structHasRun         map[string]bool
	structDescriptions   map[string]string
	funcs                map[string]bool
	funcFiles            map[string]string
	funcDescriptions     map[string]string
	funcUsesContext      map[string]bool
	funcReturnsError     map[string]bool
	subcommandNames      map[string]bool
	subcommandTypes      map[string]bool
	mainFiles            []string
	structList           []string
	funcList             []string
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
		Dir:      p.dir.Path,
		Package:  p.packageName,
		Doc:      p.packageDoc,
		Commands: commands,
		Files:    files,
	}
}

// checkDuplicates ensures no duplicate command names exist.
func (p *packageInfoParser) checkDuplicates() error {
	seen := make(map[string]string)

	for _, name := range p.structList {
		seen[camelToKebab(name)] = name
	}

	for _, name := range p.funcList {
		cmd := camelToKebab(name)
		if other, ok := seen[cmd]; ok {
			return fmt.Errorf(
				"duplicate command name %q from %s and %s",
				cmd,
				other,
				name,
			)
		}
	}

	return nil
}

// checkPackageName validates and records the package name.
func (p *packageInfoParser) checkPackageName(parsed *ast.File, filePath string) error {
	if p.packageName == "" {
		p.packageName = parsed.Name.Name
		if parsed.Doc != nil {
			p.packageDoc = strings.TrimSpace(parsed.Doc.Text())
		}

		return nil
	}

	if p.packageName != parsed.Name.Name {
		return fmt.Errorf("multiple package names in %s", p.dir.Path)
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
		return err
	}

	if err := p.checkPackageName(parsed, file.Path); err != nil {
		return err
	}

	ctxAliases, ctxDotImport := contextImportInfo(parsed.Imports)

	for _, decl := range parsed.Decls {
		err := p.processDecl(decl, file.Path, ctxAliases, ctxDotImport)
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
) error {
	switch node := decl.(type) {
	case *ast.GenDecl:
		p.processGenDecl(node, filePath)
	case *ast.FuncDecl:
		return p.processFuncDecl(node, filePath, ctxAliases, ctxDotImport)
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
	err := validateFunctionSignature(node.Type, ctxAliases, ctxDotImport)
	if err != nil {
		return fmt.Errorf("function %s %w", node.Name.Name, err)
	}

	name := node.Name.Name
	p.funcs[name] = true
	p.funcFiles[name] = filePath
	p.funcUsesContext[name] = functionUsesContext(node.Type, ctxAliases, ctxDotImport)
	p.funcReturnsError[name] = functionReturnsError(node.Type)

	if desc, ok := functionDocValue(node); ok {
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
) error {
	if node.Recv != nil {
		p.processMethod(node)
		return nil
	}

	if node.Name.Name == "main" {
		p.mainFiles = append(p.mainFiles, filePath)
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

		if recordSubcommandRefs(structType, p.subcommandNames, p.subcommandTypes) {
			p.structHasSubcommands[typeSpec.Name.Name] = true
		}
	}
}

// processMethod handles method declarations on structs.
func (p *packageInfoParser) processMethod(node *ast.FuncDecl) {
	if desc, ok := descriptionMethodValue(node); ok {
		if recvName := receiverTypeName(node.Recv); recvName != "" {
			p.structDescriptions[recvName] = desc
		}
	}

	if node.Name.Name == "Run" {
		if recvName := receiverTypeName(node.Recv); recvName != "" {
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

	return fmt.Errorf(
		"tagged files must not declare main(): %s",
		strings.Join(p.mainFiles, ", "),
	)
}

type reflectTag string

func (tag reflectTag) Get(key string) string {
	value := string(tag)
	for value != "" {
		i := strings.Index(value, ":")
		if i < 0 {
			break
		}

		name := strings.TrimSpace(value[:i])

		value = strings.TrimSpace(value[i+1:])
		if !strings.HasPrefix(value, "\"") {
			break
		}

		value = value[1:]

		j := strings.Index(value, "\"")
		if j < 0 {
			break
		}

		quoted := value[:j]
		value = strings.TrimSpace(value[j+1:])

		if name == key {
			return quoted
		}
	}

	return ""
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

func camelToKebab(s string) string {
	var result strings.Builder

	runes := []rune(s)
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

func contextImportInfo(imports []*ast.ImportSpec) (map[string]bool, bool) {
	aliases := map[string]bool{}
	dotImport := false

	for _, spec := range imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != "context" {
			continue
		}

		if spec.Name != nil {
			if spec.Name.Name == "." {
				dotImport = true
				continue
			}

			if spec.Name.Name == "_" {
				continue
			}

			aliases[spec.Name.Name] = true

			continue
		}

		aliases["context"] = true
	}

	return aliases, dotImport
}

func descriptionMethodValue(node *ast.FuncDecl) (string, bool) {
	if node.Name.Name != "Description" || node.Recv == nil {
		return "", false
	}

	if node.Type.Params != nil && len(node.Type.Params.List) > 0 {
		return "", false
	}

	if node.Type.Results == nil || len(node.Type.Results.List) != 1 {
		return "", false
	}

	if !isStringExpr(node.Type.Results.List[0].Type) {
		return "", false
	}

	return returnStringLiteral(node.Body)
}

func fieldTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}

	return ""
}

func filterCommands(candidates, subcommandNames map[string]bool) []string {
	var result []string

	for name := range candidates {
		cmd := camelToKebab(name)
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

func findTaggedDirs(fs FileSystem, startDir, tag string) ([]taggedDir, error) {
	queue := []dirQueueEntry{{path: startDir, depth: 0}}

	var results []taggedDir

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		tagged, newDirs, err := processDirectory(fs, current, tag)
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

func funcParamIsContext(expr ast.Expr, ctxAliases map[string]bool, ctxDotImport bool) bool {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok && t.Sel != nil && t.Sel.Name == "Context" {
			return ctxAliases[ident.Name]
		}
	case *ast.Ident:
		if ctxDotImport && t.Name == "Context" {
			return true
		}
	}

	return false
}

func functionDocValue(node *ast.FuncDecl) (string, bool) {
	if node.Doc == nil {
		return "", false
	}

	text := strings.TrimSpace(node.Doc.Text())
	if text == "" {
		return "", false
	}

	return text, true
}

func functionUsesContext(fnType *ast.FuncType, ctxAliases map[string]bool, ctxDotImport bool) bool {
	if fnType.Params == nil || len(fnType.Params.List) != 1 {
		return false
	}

	return funcParamIsContext(fnType.Params.List[0].Type, ctxAliases, ctxDotImport)
}

func hasBuildTag(content []byte, tag string) bool {
	lines := strings.SplitSeq(string(content), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "//") {
			return false
		}

		if after, ok := strings.CutPrefix(line, "//go:build"); ok {
			exprText := strings.TrimSpace(after)

			expr, err := constraint.Parse(exprText)
			if err != nil {
				return exprText == tag
			}

			return expr.Eval(func(t string) bool { return t == tag })
		}
	}

	return false
}

func isErrorExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "error"
}

func isStringExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "string"
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
	fs FileSystem,
	current dirQueueEntry,
	tag string,
) ([]taggedFile, []dirQueueEntry, error) {
	entries, err := fs.ReadDir(current.path)
	if err != nil {
		return nil, nil, err
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
			if !shouldSkipDir(name) {
				subdirs = append(subdirs, dirQueueEntry{path: fullPath, depth: current.depth + 1})
			}

			continue
		}

		if file, ok := tryReadTaggedFile(fs, fullPath, name, tag); ok {
			tagged = append(tagged, file)
		}
	}

	return tagged, subdirs, nil
}

func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}

	return fieldTypeName(recv.List[0].Type)
}

func recordSubcommandRefs(
	structType *ast.StructType,
	subcommandNames map[string]bool,
	subcommandTypes map[string]bool,
) bool {
	hasSubcommand := false

	for _, field := range structType.Fields.List {
		if field.Tag == nil {
			continue
		}

		tagValue := strings.Trim(field.Tag.Value, "`")
		tag := reflectStructTag(tagValue)

		targTag := tag.Get("targ")
		if !strings.Contains(targTag, "subcommand") {
			continue
		}

		hasSubcommand = true
		nameOverride := ""

		parts := strings.SplitSeq(targTag, ",")
		for p := range parts {
			p = strings.TrimSpace(p)
			if after, ok := strings.CutPrefix(p, "name="); ok {
				nameOverride = after
			} else if after, ok := strings.CutPrefix(p, "subcommand="); ok {
				nameOverride = after
			}
		}

		if nameOverride != "" {
			subcommandNames[nameOverride] = true
		} else if len(field.Names) > 0 {
			subcommandNames[camelToKebab(field.Names[0].Name)] = true
		}

		if typeName := fieldTypeName(field.Type); typeName != "" {
			subcommandTypes[typeName] = true
		}
	}

	return hasSubcommand
}

func reflectStructTag(tag string) reflectTag {
	return reflectTag(tag)
}

func returnStringLiteral(body *ast.BlockStmt) (string, bool) {
	if body == nil || len(body.List) != 1 {
		return "", false
	}

	ret, ok := body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return "", false
	}

	lit, ok := ret.Results[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}

	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}

	return strings.TrimSpace(value), true
}

func shouldSkipDir(name string) bool {
	return name == ".git" || name == "vendor"
}

func shouldSkipGoFile(name string) bool {
	if !strings.HasSuffix(name, ".go") {
		return true
	}

	if strings.HasSuffix(name, "_test.go") {
		return true
	}

	return strings.HasPrefix(name, "generated_targ_")
}

func tryReadTaggedFile(fs FileSystem, fullPath, name, tag string) (taggedFile, bool) {
	if shouldSkipGoFile(name) {
		return taggedFile{}, false
	}

	content, err := fs.ReadFile(fullPath)
	if err != nil {
		return taggedFile{}, false
	}

	if !hasBuildTag(content, tag) {
		return taggedFile{}, false
	}

	return taggedFile{
		Path:    fullPath,
		Content: content,
	}, true
}

func validateFunctionSignature(
	fnType *ast.FuncType,
	ctxAliases map[string]bool,
	ctxDotImport bool,
) error {
	paramCount := 0
	if fnType.Params != nil {
		paramCount = len(fnType.Params.List)
	}

	if paramCount > 1 {
		return errors.New("must be niladic or accept context")
	}

	if paramCount == 1 &&
		!funcParamIsContext(fnType.Params.List[0].Type, ctxAliases, ctxDotImport) {
		return errors.New("must accept context.Context")
	}

	if fnType.Results == nil || len(fnType.Results.List) == 0 {
		return nil
	}

	if len(fnType.Results.List) == 1 && isErrorExpr(fnType.Results.List[0].Type) {
		return nil
	}

	return errors.New("must return only error")
}

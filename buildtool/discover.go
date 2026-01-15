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

func parsePackageInfo(dir taggedDir) (PackageInfo, error) {
	fset := token.NewFileSet()
	packageName := ""
	packageDoc := ""
	structs := make(map[string]bool)
	structFiles := make(map[string]string)
	structHasSubcommands := make(map[string]bool)
	structHasRun := make(map[string]bool)
	funcs := make(map[string]bool)
	funcFiles := make(map[string]string)
	structDescriptions := make(map[string]string)
	funcDescriptions := make(map[string]string)
	funcUsesContext := make(map[string]bool)
	funcReturnsError := make(map[string]bool)
	subcommandNames := make(map[string]bool)
	subcommandTypes := make(map[string]bool)

	var mainFiles []string

	for _, file := range dir.Files {
		parsed, err := parser.ParseFile(fset, file.Path, file.Content, parser.ParseComments)
		if err != nil {
			return PackageInfo{}, err
		}

		if packageName == "" {
			packageName = parsed.Name.Name
			if parsed.Doc != nil {
				packageDoc = strings.TrimSpace(parsed.Doc.Text())
			}
		} else if packageName != parsed.Name.Name {
			return PackageInfo{}, fmt.Errorf("multiple package names in %s", dir.Path)
		}

		ctxAliases, ctxDotImport := contextImportInfo(parsed.Imports)
		for _, decl := range parsed.Decls {
			switch node := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range node.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok || !typeSpec.Name.IsExported() {
						continue
					}

					structType, ok := typeSpec.Type.(*ast.StructType)
					if !ok {
						continue
					}

					structs[typeSpec.Name.Name] = true

					structFiles[typeSpec.Name.Name] = file.Path
					if recordSubcommandRefs(structType, subcommandNames, subcommandTypes) {
						structHasSubcommands[typeSpec.Name.Name] = true
					}
				}
			case *ast.FuncDecl:
				if node.Recv != nil {
					if desc, ok := descriptionMethodValue(node); ok {
						if recvName := receiverTypeName(node.Recv); recvName != "" {
							structDescriptions[recvName] = desc
						}
					}

					if node.Name.Name == "Run" {
						if recvName := receiverTypeName(node.Recv); recvName != "" {
							structHasRun[recvName] = true
						}
					}

					continue
				}

				if node.Name.Name == "main" {
					mainFiles = append(mainFiles, file.Path)
					continue
				}

				if !node.Name.IsExported() {
					continue
				}

				err := validateFunctionSignature(node.Type, ctxAliases, ctxDotImport)
				if err != nil {
					return PackageInfo{}, fmt.Errorf("function %s %w", node.Name.Name, err)
				}

				funcs[node.Name.Name] = true
				funcFiles[node.Name.Name] = file.Path
				funcUsesContext[node.Name.Name] = functionUsesContext(node.Type, ctxAliases, ctxDotImport)

				funcReturnsError[node.Name.Name] = functionReturnsError(node.Type)
				if desc, ok := functionDocValue(node); ok {
					funcDescriptions[node.Name.Name] = desc
				}
			}
		}
	}

	if len(mainFiles) > 0 {
		sort.Strings(mainFiles)

		return PackageInfo{}, fmt.Errorf(
			"tagged files must not declare main(): %s",
			strings.Join(mainFiles, ", "),
		)
	}

	structList := filterStructs(structs, subcommandTypes, structHasRun, structHasSubcommands)
	funcList := filterCommands(funcs, subcommandNames)

	if len(structList) > 0 && len(funcList) > 0 {
		wrapped := make(map[string]bool)

		for _, name := range structList {
			if before, ok := strings.CutSuffix(name, "Command"); ok {
				base := before
				if base != "" {
					wrapped[base] = true
				}
			}
		}

		if len(wrapped) > 0 {
			filtered := make([]string, 0, len(funcList))
			for _, name := range funcList {
				if !wrapped[name] {
					filtered = append(filtered, name)
				}
			}

			funcList = filtered
		}
	}

	seen := make(map[string]string)

	for _, name := range structList {
		cmd := camelToKebab(name)
		seen[cmd] = name
	}

	for _, name := range funcList {
		cmd := camelToKebab(name)
		if other, ok := seen[cmd]; ok {
			return PackageInfo{}, fmt.Errorf(
				"duplicate command name %q from %s and %s",
				cmd,
				other,
				name,
			)
		}
	}

	commands := make([]CommandInfo, 0, len(structList)+len(funcList))
	fileCommands := make(map[string][]CommandInfo)

	for _, name := range structList {
		cmd := CommandInfo{
			Name:        name,
			Kind:        CommandStruct,
			File:        structFiles[name],
			Description: structDescriptions[name],
		}
		commands = append(commands, cmd)
		fileCommands[cmd.File] = append(fileCommands[cmd.File], cmd)
	}

	for _, name := range funcList {
		cmd := CommandInfo{
			Name:         name,
			Kind:         CommandFunc,
			File:         funcFiles[name],
			Description:  funcDescriptions[name],
			UsesContext:  funcUsesContext[name],
			ReturnsError: funcReturnsError[name],
		}
		commands = append(commands, cmd)
		fileCommands[cmd.File] = append(fileCommands[cmd.File], cmd)
	}

	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})

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

	return PackageInfo{
		Dir:      dir.Path,
		Package:  packageName,
		Doc:      packageDoc,
		Commands: commands,
		Files:    files,
	}, nil
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

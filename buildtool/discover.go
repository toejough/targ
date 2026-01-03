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
	StartDir     string
	MultiPackage bool
	BuildTag     string
}

type TaggedDir struct {
	Path  string
	Depth int
}

type TaggedFile struct {
	Path    string
	Content []byte
}

type PackageInfo struct {
	Dir     string
	Package string
	Doc     string
	Structs []string
	Funcs   []string
}

type taggedFile struct {
	Path    string
	Content []byte
}

type taggedDir struct {
	Path  string
	Depth int
	Files []taggedFile
}

var (
	ErrNoTaggedFiles = errors.New("no tagged files found")
)

type MultipleTaggedDirsError struct {
	Depth int
	Paths []string
}

func (e *MultipleTaggedDirsError) Error() string {
	return fmt.Sprintf("multiple tagged directories at depth %d: %s", e.Depth, strings.Join(e.Paths, ", "))
}

func Discover(fs FileSystem, opts Options) ([]PackageInfo, error) {
	startDir := opts.StartDir
	if startDir == "" {
		startDir = "."
	}
	tag := opts.BuildTag
	if tag == "" {
		tag = "commander"
	}

	dirs, err := findTaggedDirs(fs, startDir, tag)
	if err != nil {
		return nil, err
	}
	selected, err := selectTaggedDirs(dirs, opts.MultiPackage)
	if err != nil {
		return nil, err
	}

	var infos []PackageInfo
	seenPackages := make(map[string]string)
	for _, dir := range selected {
		info, err := parsePackageInfo(dir)
		if err != nil {
			return nil, err
		}
		if otherDir, ok := seenPackages[info.Package]; ok {
			return nil, fmt.Errorf("duplicate package name %q in %s and %s", info.Package, otherDir, info.Dir)
		}
		seenPackages[info.Package] = info.Dir
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
		tag = "commander"
	}

	dirs, err := findTaggedDirs(fs, startDir, tag)
	if err != nil {
		return nil, err
	}
	selected, err := selectTaggedDirs(dirs, opts.MultiPackage)
	if err != nil {
		return nil, err
	}

	paths := make([]TaggedDir, 0, len(selected))
	for _, dir := range selected {
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
		tag = "commander"
	}

	dirs, err := findTaggedDirs(fs, startDir, tag)
	if err != nil {
		return nil, err
	}
	selected, err := selectTaggedDirs(dirs, opts.MultiPackage)
	if err != nil {
		return nil, err
	}

	var files []TaggedFile
	for _, dir := range selected {
		for _, file := range dir.Files {
			files = append(files, TaggedFile{
				Path:    file.Path,
				Content: file.Content,
			})
		}
	}
	return files, nil
}

func selectTaggedDirs(dirs []taggedDir, multiPackage bool) ([]taggedDir, error) {
	if len(dirs) == 0 {
		return nil, ErrNoTaggedFiles
	}
	if multiPackage {
		return dirs, nil
	}

	minDepth := dirs[0].Depth
	for _, dir := range dirs[1:] {
		if dir.Depth < minDepth {
			minDepth = dir.Depth
		}
	}

	selected := []taggedDir{}
	for _, dir := range dirs {
		if dir.Depth == minDepth {
			selected = append(selected, dir)
		}
	}

	if len(selected) > 1 {
		paths := make([]string, 0, len(selected))
		for _, dir := range selected {
			paths = append(paths, dir.Path)
		}
		sort.Strings(paths)
		return nil, &MultipleTaggedDirsError{Depth: minDepth, Paths: paths}
	}

	return selected, nil
}

func findTaggedDirs(fs FileSystem, startDir string, tag string) ([]taggedDir, error) {
	type dirEntry struct {
		path  string
		depth int
	}

	queue := []dirEntry{{path: startDir, depth: 0}}
	var results []taggedDir

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		entries, err := fs.ReadDir(current.path)
		if err != nil {
			return nil, err
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})

		var tagged []taggedFile
		for _, entry := range entries {
			name := entry.Name()
			fullPath := filepath.Join(current.path, name)
			if entry.IsDir() {
				queue = append(queue, dirEntry{path: fullPath, depth: current.depth + 1})
				continue
			}

			if !strings.HasSuffix(name, ".go") {
				continue
			}

			content, err := fs.ReadFile(fullPath)
			if err != nil {
				return nil, err
			}
			if hasBuildTag(content, tag) {
				tagged = append(tagged, taggedFile{
					Path:    fullPath,
					Content: content,
				})
			}
		}

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

func hasBuildTag(content []byte, tag string) bool {
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			return false
		}
		if strings.HasPrefix(line, "//go:build") {
			exprText := strings.TrimSpace(strings.TrimPrefix(line, "//go:build"))
			expr, err := constraint.Parse(exprText)
			if err != nil {
				return exprText == tag
			}
			return expr.Eval(func(t string) bool { return t == tag })
		}
	}
	return false
}

func parsePackageInfo(dir taggedDir) (PackageInfo, error) {
	fset := token.NewFileSet()
	packageName := ""
	packageDoc := ""
	structs := make(map[string]bool)
	structHasSubcommands := make(map[string]bool)
	structHasRun := make(map[string]bool)
	funcs := make(map[string]bool)
	subcommandNames := make(map[string]bool)
	subcommandTypes := make(map[string]bool)

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
					if recordSubcommandRefs(structType, subcommandNames, subcommandTypes) {
						structHasSubcommands[typeSpec.Name.Name] = true
					}
				}
			case *ast.FuncDecl:
				if node.Recv != nil {
					if node.Name.Name == "Run" {
						if recvName := receiverTypeName(node.Recv); recvName != "" {
							structHasRun[recvName] = true
						}
					}
					continue
				}
				if !node.Name.IsExported() {
					continue
				}
				if err := validateFunctionSignature(node.Type, ctxAliases, ctxDotImport); err != nil {
					return PackageInfo{}, fmt.Errorf("function %s %v", node.Name.Name, err)
				}
				funcs[node.Name.Name] = true
			}
		}
	}

	structList := filterStructs(structs, subcommandTypes, structHasRun, structHasSubcommands)
	funcList := filterCommands(funcs, subcommandNames)

	if len(structList) > 0 && len(funcList) > 0 {
		wrapped := make(map[string]bool)
		for _, name := range structList {
			if strings.HasSuffix(name, "Command") {
				base := strings.TrimSuffix(name, "Command")
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
			return PackageInfo{}, fmt.Errorf("duplicate command name %q from %s and %s", cmd, other, name)
		}
	}

	return PackageInfo{
		Dir:     dir.Path,
		Package: packageName,
		Doc:     packageDoc,
		Structs: structList,
		Funcs:   funcList,
	}, nil
}

func validateFunctionSignature(fnType *ast.FuncType, ctxAliases map[string]bool, ctxDotImport bool) error {
	paramCount := 0
	if fnType.Params != nil {
		paramCount = len(fnType.Params.List)
	}
	if paramCount > 1 {
		return fmt.Errorf("must be niladic or accept context")
	}
	if paramCount == 1 && !funcParamIsContext(fnType.Params.List[0].Type, ctxAliases, ctxDotImport) {
		return fmt.Errorf("must accept context.Context")
	}
	if fnType.Results == nil || len(fnType.Results.List) == 0 {
		return nil
	}
	if len(fnType.Results.List) == 1 && isErrorExpr(fnType.Results.List[0].Type) {
		return nil
	}
	return fmt.Errorf("must return only error")
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

func isErrorExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "error"
}

func filterCommands(candidates map[string]bool, subcommandNames map[string]bool) []string {
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

func filterStructs(candidates map[string]bool, subcommandTypes map[string]bool, structHasRun map[string]bool, structHasSubcommands map[string]bool) []string {
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
		commanderTag := tag.Get("commander")
		if !strings.Contains(commanderTag, "subcommand") {
			continue
		}
		hasSubcommand = true
		nameOverride := ""
		parts := strings.Split(commanderTag, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, "name=") {
				nameOverride = strings.TrimPrefix(p, "name=")
			} else if strings.HasPrefix(p, "subcommand=") {
				nameOverride = strings.TrimPrefix(p, "subcommand=")
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

func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	return fieldTypeName(recv.List[0].Type)
}

func camelToKebab(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune('-')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

func reflectStructTag(tag string) reflectTag {
	return reflectTag(tag)
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

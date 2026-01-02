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
	"strings"
	"unicode"
)

type FileSystem interface {
	ReadDir(name string) ([]fs.DirEntry, error)
	ReadFile(name string) ([]byte, error)
}

type OSFileSystem struct{}

func (OSFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}

func (OSFileSystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

type Options struct {
	StartDir        string
	PackageGrouping bool
	BuildTag        string
}

type PackageInfo struct {
	Dir     string
	Package string
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
	if len(dirs) == 0 {
		return nil, ErrNoTaggedFiles
	}

	selected := dirs
	if !opts.PackageGrouping {
		minDepth := dirs[0].Depth
		for _, dir := range dirs[1:] {
			if dir.Depth < minDepth {
				minDepth = dir.Depth
			}
		}

		selected = []taggedDir{}
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
			return nil, fmt.Errorf("multiple tagged directories at depth %d: %s", minDepth, strings.Join(paths, ", "))
		}
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
	structs := make(map[string]bool)
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
		} else if packageName != parsed.Name.Name {
			return PackageInfo{}, fmt.Errorf("multiple package names in %s", dir.Path)
		}

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
					recordSubcommandRefs(structType, subcommandNames, subcommandTypes)
				}
			case *ast.FuncDecl:
				if node.Recv != nil || !node.Name.IsExported() {
					continue
				}
				if hasParams(node.Type) {
					return PackageInfo{}, fmt.Errorf("function %s must be niladic", node.Name.Name)
				}
				funcs[node.Name.Name] = true
			}
		}
	}

	structList := filterStructs(structs, subcommandTypes)
	funcList := filterCommands(funcs, subcommandNames)

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
		Structs: structList,
		Funcs:   funcList,
	}, nil
}

func hasParams(fnType *ast.FuncType) bool {
	if fnType.Params != nil && len(fnType.Params.List) > 0 {
		return true
	}
	if fnType.Results != nil && len(fnType.Results.List) > 0 {
		return true
	}
	return false
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

func filterStructs(candidates map[string]bool, subcommandTypes map[string]bool) []string {
	var result []string
	for name := range candidates {
		if !subcommandTypes[name] {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

func recordSubcommandRefs(
	structType *ast.StructType,
	subcommandNames map[string]bool,
	subcommandTypes map[string]bool,
) {
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

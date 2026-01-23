// Package discover provides internal implementation for discovering targ packages.
package discover

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/toejough/targ/internal/parse"
)

// Exported variables.
var (
	ErrMainFunctionNotAllowed = errors.New("tagged files must not declare main()")
	ErrMultiplePackageNames   = errors.New("multiple package names")
	ErrNoTaggedFiles          = errors.New("no tagged files found")
)

type FileInfo struct {
	Path string
	Base string
}

type FileSystem interface {
	ReadDir(name string) ([]fs.DirEntry, error)
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

type Options struct {
	StartDir string
	BuildTag string
}

type PackageInfo struct {
	Dir                      string
	Package                  string
	Doc                      string
	Files                    []FileInfo
	UsesExplicitRegistration bool
}

type TaggedDir struct {
	Path  string
	Depth int
}

type TaggedFile struct {
	Path    string
	Content []byte
}

// Public API.

// Discover finds all packages with targ-tagged files and parses their info.
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
)

type dirQueueEntry struct {
	path  string
	depth int
}

type packageInfoParser struct {
	dir                      taggedDir
	fset                     *token.FileSet
	packageName              string
	packageDoc               string
	mainFiles                []string
	usesExplicitRegistration bool
}

// buildFiles creates FileInfo for all tagged files.
func (p *packageInfoParser) buildFiles() []FileInfo {
	files := make([]FileInfo, 0, len(p.dir.Files))

	for _, tf := range p.dir.Files {
		files = append(files, FileInfo{
			Path: tf.Path,
			Base: strings.TrimSuffix(filepath.Base(tf.Path), filepath.Ext(tf.Path)),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files
}

// buildResult constructs the final PackageInfo.
func (p *packageInfoParser) buildResult() PackageInfo {
	return PackageInfo{
		Dir:                      p.dir.Path,
		Package:                  p.packageName,
		Doc:                      p.packageDoc,
		Files:                    p.buildFiles(),
		UsesExplicitRegistration: p.usesExplicitRegistration,
	}
}

// checkPackageName validates and records the package name.
func (p *packageInfoParser) checkPackageName(parsed *ast.File) error {
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

// parseFile parses a single file and extracts package info.
func (p *packageInfoParser) parseFile(file taggedFile) error {
	parsed, err := parser.ParseFile(p.fset, file.Path, file.Content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing file %s: %w", file.Path, err)
	}

	err = p.checkPackageName(parsed)
	if err != nil {
		return err
	}

	targAliases := parse.TargImportInfo(parsed.Imports)

	for _, decl := range parsed.Decls {
		p.processDecl(decl, file.Path, targAliases)
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
	targAliases map[string]bool,
) {
	funcDecl, ok := decl.(*ast.FuncDecl)
	if !ok {
		return
	}

	if funcDecl.Recv != nil {
		return
	}

	if funcDecl.Name.Name == "main" {
		p.mainFiles = append(p.mainFiles, filePath)
		return
	}

	if funcDecl.Name.Name == "init" {
		if containsTargRegisterCall(funcDecl.Body, targAliases) {
			p.usesExplicitRegistration = true
		}
	}
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
		dir:  dir,
		fset: token.NewFileSet(),
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

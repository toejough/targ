package buildtool

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	iofs "io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Error variables used by the wrapper generator.
var (
	ErrGeneratedWrapperExists = errors.New("generated wrapper already exists")
	ErrNoGoFilesFound         = errors.New("no Go files found")
	ErrPackageNameNotFound    = errors.New("package name not found")
)

type GenerateOptions struct {
	Dir        string
	BuildTag   string
	OnlyTagged bool
}

func GenerateFunctionWrappers(filesystem FileSystem, opts GenerateOptions) (string, error) {
	g := newWrapperGenerator(filesystem, opts)

	err := g.parseFiles()
	if err != nil {
		return "", err
	}

	err = g.validate()
	if err != nil {
		return "", err
	}

	if len(g.functionNames) == 0 {
		return "", nil
	}

	return g.generateAndWrite()
}

type functionDoc struct {
	Name         string
	Description  string
	ReturnsError bool
	UsesContext  bool
}

// wrapperGenerator holds state for generating function wrappers.
type wrapperGenerator struct {
	filesystem         FileSystem
	opts               GenerateOptions
	dir                string
	tag                string
	fset               *token.FileSet
	packageName        string
	functions          map[string]functionDoc
	typeNames          map[string]bool
	subcommandNames    map[string]bool
	parsedFiles        int
	needsContextImport bool
	functionNames      []string
}

// buildFunctionDoc creates a functionDoc from a function declaration.
func (g *wrapperGenerator) buildFunctionDoc(
	node *ast.FuncDecl,
	ctxAliases map[string]bool,
	ctxDotImport bool,
) functionDoc {
	desc := ""
	if node.Doc != nil {
		desc = strings.TrimSpace(node.Doc.Text())
	}

	usesContext := false
	if node.Type.Params != nil && len(node.Type.Params.List) == 1 {
		usesContext = funcParamIsContext(node.Type.Params.List[0].Type, ctxAliases, ctxDotImport)
	}

	return functionDoc{
		Name:         node.Name.Name,
		Description:  desc,
		ReturnsError: functionReturnsError(node.Type),
		UsesContext:  usesContext,
	}
}

// checkPackageName validates and records the package name.
func (g *wrapperGenerator) checkPackageName(name string) error {
	if g.packageName == "" {
		g.packageName = name
		return nil
	}

	if g.packageName != name {
		return fmt.Errorf("%w: %s", ErrMultiplePackageNames, g.dir)
	}

	return nil
}

// checkWrapperConflicts ensures generated names don't conflict with existing types.
func (g *wrapperGenerator) checkWrapperConflicts() error {
	for _, name := range g.functionNames {
		wrapperName := name + "Command"
		if g.typeNames[wrapperName] {
			return fmt.Errorf("%w: %s", ErrGeneratedWrapperExists, wrapperName)
		}
	}

	return nil
}

// collectFunctionNames builds the sorted list of functions to wrap.
func (g *wrapperGenerator) collectFunctionNames() {
	for name := range g.functions {
		if g.subcommandNames[camelToKebab(name)] {
			continue
		}

		g.functionNames = append(g.functionNames, name)
	}

	sort.Strings(g.functionNames)
}

// generateAndWrite generates the wrapper code and writes it to a file.
func (g *wrapperGenerator) generateAndWrite() (string, error) {
	var buf bytes.Buffer

	g.writeHeader(&buf)
	g.writeWrappers(&buf)

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return "", err
	}

	filename := filepath.Join(g.dir, fmt.Sprintf("generated_targ_%s.go", g.packageName))
	if err := g.filesystem.WriteFile(filename, formatted, iofs.FileMode(0o644)); err != nil {
		return "", err
	}

	return filename, nil
}

// parseFile parses a single Go file and extracts declarations.
func (g *wrapperGenerator) parseFile(fullPath string, content []byte) error {
	parsed, err := parser.ParseFile(g.fset, fullPath, content, parser.ParseComments)
	if err != nil {
		return err
	}

	g.parsedFiles++

	if err := g.checkPackageName(parsed.Name.Name); err != nil {
		return err
	}

	ctxAliases, ctxDotImport := contextImportInfo(parsed.Imports)

	for _, decl := range parsed.Decls {
		err := g.processDecl(decl, ctxAliases, ctxDotImport)
		if err != nil {
			return err
		}
	}

	return nil
}

// parseFiles reads and parses all Go files in the directory.
func (g *wrapperGenerator) parseFiles() error {
	entries, err := g.filesystem.ReadDir(g.dir)
	if err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		err := g.processEntry(entry)
		if err != nil {
			return err
		}
	}

	return nil
}

// processDecl handles a single declaration.
func (g *wrapperGenerator) processDecl(
	decl ast.Decl,
	ctxAliases map[string]bool,
	ctxDotImport bool,
) error {
	switch node := decl.(type) {
	case *ast.GenDecl:
		g.processGenDecl(node)
	case *ast.FuncDecl:
		return g.processFuncDecl(node, ctxAliases, ctxDotImport)
	}

	return nil
}

// processEntry processes a single directory entry.
func (g *wrapperGenerator) processEntry(entry iofs.DirEntry) error {
	if entry.IsDir() || !isGoSourceFile(entry.Name()) {
		return nil
	}

	fullPath := filepath.Join(g.dir, entry.Name())

	content, err := g.filesystem.ReadFile(fullPath)
	if err != nil {
		return err
	}

	if g.opts.OnlyTagged && !hasBuildTag(content, g.tag) {
		return nil
	}

	return g.parseFile(fullPath, content)
}

// processFuncDecl processes a function declaration.
func (g *wrapperGenerator) processFuncDecl(
	node *ast.FuncDecl,
	ctxAliases map[string]bool,
	ctxDotImport bool,
) error {
	if node.Recv != nil || !node.Name.IsExported() {
		return nil
	}

	err := validateFunctionSignature(node.Type, ctxAliases, ctxDotImport)
	if err != nil {
		return fmt.Errorf("function %s %w", node.Name.Name, err)
	}

	fn := g.buildFunctionDoc(node, ctxAliases, ctxDotImport)
	if fn.UsesContext {
		g.needsContextImport = true
	}

	g.functions[node.Name.Name] = fn

	return nil
}

// processGenDecl processes a general declaration (types, etc).
func (g *wrapperGenerator) processGenDecl(node *ast.GenDecl) {
	for _, spec := range node.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		g.typeNames[typeSpec.Name.Name] = true

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}

		recordSubcommandRefs(structType, g.subcommandNames, map[string]bool{})
	}
}

// validate checks that parsing produced valid results.
func (g *wrapperGenerator) validate() error {
	if g.parsedFiles == 0 {
		return fmt.Errorf("%w: %s", ErrNoGoFilesFound, g.dir)
	}

	if g.packageName == "" {
		return fmt.Errorf("%w: %s", ErrPackageNameNotFound, g.dir)
	}

	g.collectFunctionNames()

	return g.checkWrapperConflicts()
}

// writeDescriptionMethod writes the Description method for a wrapper.
func (g *wrapperGenerator) writeDescriptionMethod(buf *bytes.Buffer, wrapperName, desc string) {
	buf.WriteString("func (c *")
	buf.WriteString(wrapperName)
	buf.WriteString(") Description() string {\n")
	buf.WriteString("\treturn ")
	buf.WriteString(strconv.Quote(desc))
	buf.WriteString("\n}\n\n")
}

// writeHeader writes the file header (build tag, package, imports).
func (g *wrapperGenerator) writeHeader(buf *bytes.Buffer) {
	if g.opts.BuildTag != "" {
		buf.WriteString("//go:build ")
		buf.WriteString(g.opts.BuildTag)
		buf.WriteString("\n\n")
	}

	buf.WriteString("// Code generated by targ. DO NOT EDIT.\n\n")
	buf.WriteString("package ")
	buf.WriteString(g.packageName)
	buf.WriteString("\n\n")

	if g.needsContextImport {
		buf.WriteString("import \"context\"\n\n")
	}
}

// writeNameMethod writes the Name method for a wrapper.
func (g *wrapperGenerator) writeNameMethod(buf *bytes.Buffer, wrapperName, funcName string) {
	buf.WriteString("func (c *")
	buf.WriteString(wrapperName)
	buf.WriteString(") Name() string {\n")
	buf.WriteString("\treturn ")
	buf.WriteString(strconv.Quote(funcName))
	buf.WriteString("\n}\n\n")
}

// writeRunMethod writes the Run method for a wrapper.
func (g *wrapperGenerator) writeRunMethod(
	buf *bytes.Buffer,
	wrapperName, funcName string,
	fn functionDoc,
) {
	buf.WriteString("func (c *")
	buf.WriteString(wrapperName)
	buf.WriteString(") Run(")

	if fn.UsesContext {
		buf.WriteString("ctx context.Context")
	}

	buf.WriteString(")")

	if fn.ReturnsError {
		buf.WriteString(" error")
	}

	buf.WriteString(" {\n")

	if fn.ReturnsError {
		buf.WriteString("\treturn ")
	} else {
		buf.WriteString("\t")
	}

	buf.WriteString(funcName)
	buf.WriteString("(")

	if fn.UsesContext {
		buf.WriteString("ctx")
	}

	buf.WriteString(")\n}\n\n")
}

// writeTypeDecl writes the wrapper struct type declaration.
func (g *wrapperGenerator) writeTypeDecl(buf *bytes.Buffer, wrapperName string) {
	buf.WriteString("type ")
	buf.WriteString(wrapperName)
	buf.WriteString(" struct{}\n\n")
}

// writeWrapper writes a single function wrapper.
func (g *wrapperGenerator) writeWrapper(buf *bytes.Buffer, name string, fn functionDoc) {
	wrapperName := name + "Command"

	g.writeTypeDecl(buf, wrapperName)
	g.writeRunMethod(buf, wrapperName, name, fn)
	g.writeNameMethod(buf, wrapperName, name)

	if fn.Description != "" {
		g.writeDescriptionMethod(buf, wrapperName, fn.Description)
	}
}

// writeWrappers writes all function wrapper types and methods.
func (g *wrapperGenerator) writeWrappers(buf *bytes.Buffer) {
	for idx, name := range g.functionNames {
		if idx > 0 {
			buf.WriteString("\n")
		}

		g.writeWrapper(buf, name, g.functions[name])
	}
}

func functionReturnsError(fnType *ast.FuncType) bool {
	if fnType.Results == nil || len(fnType.Results.List) == 0 {
		return false
	}

	return len(fnType.Results.List) == 1 && isErrorExpr(fnType.Results.List[0].Type)
}

// isGoSourceFile checks if a filename is a non-generated Go source file.
func isGoSourceFile(name string) bool {
	if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
		return false
	}

	return !strings.HasPrefix(name, "generated_targ_")
}

// newWrapperGenerator creates a new wrapper generator with initialized state.
func newWrapperGenerator(filesystem FileSystem, opts GenerateOptions) *wrapperGenerator {
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}

	tag := opts.BuildTag
	if opts.OnlyTagged && tag == "" {
		tag = "targ"
	}

	return &wrapperGenerator{
		filesystem:      filesystem,
		opts:            opts,
		dir:             dir,
		tag:             tag,
		fset:            token.NewFileSet(),
		functions:       make(map[string]functionDoc),
		typeNames:       make(map[string]bool),
		subcommandNames: make(map[string]bool),
	}
}

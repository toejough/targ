// Package parse provides internal parsing utilities for buildtool.
package parse

import (
	"go/ast"
	"go/build/constraint"
	"go/token"
	"strconv"
	"strings"
	"unicode"
)

// Exported variables.
var (
	ErrMustAcceptContext = validationError("must accept context.Context")
	ErrMustReturnError   = validationError("must return only error")
	ErrNiladicOrContext  = validationError("must be niladic or accept context")
)

// ReflectTag parses struct field tags in the reflect format.
type ReflectTag string

// NewReflectTag creates a ReflectTag from a raw tag string.
func NewReflectTag(tag string) ReflectTag {
	return ReflectTag(tag)
}

// Get returns the value for the given key in the tag.
func (tag ReflectTag) Get(key string) string {
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

// CamelToKebab converts CamelCase to kebab-case.
func CamelToKebab(s string) string {
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

// ContextImportInfo extracts context import aliases from import specs.
func ContextImportInfo(imports []*ast.ImportSpec) (aliases map[string]bool, dotImport bool) {
	aliases = map[string]bool{}

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

// DescriptionMethodValue extracts the description from a Description() method.
func DescriptionMethodValue(node *ast.FuncDecl) (string, bool) {
	if node.Name.Name != "Description" || node.Recv == nil {
		return "", false
	}

	if node.Type.Params != nil && len(node.Type.Params.List) > 0 {
		return "", false
	}

	if node.Type.Results == nil || len(node.Type.Results.List) != 1 {
		return "", false
	}

	if !IsStringExpr(node.Type.Results.List[0].Type) {
		return "", false
	}

	return ReturnStringLiteral(node.Body)
}

// FieldTypeName extracts the type name from a field expression.
func FieldTypeName(expr ast.Expr) string {
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

// FuncParamIsContext checks if a function parameter is context.Context.
func FuncParamIsContext(expr ast.Expr, ctxAliases map[string]bool, ctxDotImport bool) bool {
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

// FunctionDocValue extracts the doc comment text from a function declaration.
func FunctionDocValue(node *ast.FuncDecl) (string, bool) {
	if node.Doc == nil {
		return "", false
	}

	text := strings.TrimSpace(node.Doc.Text())
	if text == "" {
		return "", false
	}

	return text, true
}

// FunctionReturnsError checks if a function returns only an error.
func FunctionReturnsError(fnType *ast.FuncType) bool {
	if fnType.Results == nil || len(fnType.Results.List) == 0 {
		return false
	}

	return len(fnType.Results.List) == 1 && IsErrorExpr(fnType.Results.List[0].Type)
}

// FunctionUsesContext checks if a function accepts context.Context as its only parameter.
func FunctionUsesContext(fnType *ast.FuncType, ctxAliases map[string]bool, ctxDotImport bool) bool {
	if fnType.Params == nil || len(fnType.Params.List) != 1 {
		return false
	}

	return FuncParamIsContext(fnType.Params.List[0].Type, ctxAliases, ctxDotImport)
}

// HasBuildTag checks if file content contains the specified build tag.
func HasBuildTag(content []byte, tag string) bool {
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

// IsErrorExpr checks if an expression is the error type.
func IsErrorExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "error"
}

// IsGoSourceFile checks if a filename is a non-generated Go source file.
func IsGoSourceFile(name string) bool {
	if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
		return false
	}

	return !strings.HasPrefix(name, "generated_targ_")
}

// IsStringExpr checks if an expression is the string type.
func IsStringExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "string"
}

// ReceiverTypeName extracts the type name from a method receiver.
func ReceiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}

	return FieldTypeName(recv.List[0].Type)
}

// RecordSubcommandRefs records subcommand names and types from a struct's fields.
func RecordSubcommandRefs(
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
		tag := NewReflectTag(tagValue)

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
			subcommandNames[CamelToKebab(field.Names[0].Name)] = true
		}

		if typeName := FieldTypeName(field.Type); typeName != "" {
			subcommandTypes[typeName] = true
		}
	}

	return hasSubcommand
}

// ReturnStringLiteral extracts a string literal from a single return statement.
func ReturnStringLiteral(body *ast.BlockStmt) (string, bool) {
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

// ShouldSkipDir determines if a directory should be skipped during discovery.
func ShouldSkipDir(name string) bool {
	return name == ".git" || name == "vendor"
}

// ShouldSkipGoFile determines if a Go file should be skipped during discovery.
func ShouldSkipGoFile(name string) bool {
	if !strings.HasSuffix(name, ".go") {
		return true
	}

	if strings.HasSuffix(name, "_test.go") {
		return true
	}

	return strings.HasPrefix(name, "generated_targ_")
}

// ValidateFunctionSignature validates that a function has an acceptable signature.
func ValidateFunctionSignature(
	fnType *ast.FuncType,
	ctxAliases map[string]bool,
	ctxDotImport bool,
) error {
	paramCount := 0
	if fnType.Params != nil {
		paramCount = len(fnType.Params.List)
	}

	if paramCount > 1 {
		return ErrNiladicOrContext
	}

	if paramCount == 1 &&
		!FuncParamIsContext(fnType.Params.List[0].Type, ctxAliases, ctxDotImport) {
		return ErrMustAcceptContext
	}

	if fnType.Results == nil || len(fnType.Results.List) == 0 {
		return nil
	}

	if len(fnType.Results.List) == 1 && IsErrorExpr(fnType.Results.List[0].Type) {
		return nil
	}

	return ErrMustReturnError
}

type validationError string

func (e validationError) Error() string { return string(e) }

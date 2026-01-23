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

// DescriptionMethodValue extracts the description from a Description() method.

// FuncParamIsContext checks if a function parameter is context.Context.

// FunctionDocValue extracts the doc comment text from a function declaration.

// FunctionReturnsError checks if a function returns only an error.

// FunctionUsesContext checks if a function accepts context.Context as its only parameter.

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

// IsGoSourceFile checks if a filename is a non-generated Go source file.
func IsGoSourceFile(name string) bool {
	if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
		return false
	}

	return !strings.HasPrefix(name, "generated_targ_")
}

// IsStringExpr checks if an expression is the string type.

// RecordSubcommandRefs records subcommand names and types from a struct's fields.

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
	if strings.HasPrefix(name, ".") {
		return true // Skip hidden directories
	}

	// Skip internal directories - they're implementation details pulled in via imports,
	// not target packages that need targ.Register()
	return name == "vendor" || name == "testdata" || name == "internal"
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

// TargImportInfo extracts targ import aliases from import specs.
func TargImportInfo(imports []*ast.ImportSpec) (aliases map[string]bool) {
	aliases = map[string]bool{}

	for _, spec := range imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != "github.com/toejough/targ" {
			continue
		}

		if spec.Name != nil {
			if spec.Name.Name == "_" || spec.Name.Name == "." {
				continue
			}

			aliases[spec.Name.Name] = true

			continue
		}

		aliases["targ"] = true
	}

	return aliases
}

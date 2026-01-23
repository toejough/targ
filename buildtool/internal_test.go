package buildtool

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestContainsTargRegisterCall_NilBody(t *testing.T) {
	result := containsTargRegisterCall(nil, map[string]bool{"targ": true})
	if result {
		t.Error("expected false for nil body")
	}
}

func TestContainsTargRegisterCall_NoRegisterCall(t *testing.T) {
	src := `package test
func init() {
	fmt.Println("hello")
}
`
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Find the init function body
	var body *ast.BlockStmt

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "init" {
			body = fn.Body
			break
		}
	}

	result := containsTargRegisterCall(body, map[string]bool{"targ": true})
	if result {
		t.Error("expected false when no Register call exists")
	}
}

func TestContainsTargRegisterCall_WithAliasedImport(t *testing.T) {
	src := `package test
func init() {
	t.Register(Build)
}
`
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	var body *ast.BlockStmt

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "init" {
			body = fn.Body
			break
		}
	}

	// "t" is the alias for targ
	result := containsTargRegisterCall(body, map[string]bool{"t": true})
	if !result {
		t.Error("expected true when Register call exists with alias")
	}
}

func TestContainsTargRegisterCall_WithRegisterCall(t *testing.T) {
	src := `package test
func init() {
	targ.Register(Build)
}
`
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	var body *ast.BlockStmt

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "init" {
			body = fn.Body
			break
		}
	}

	result := containsTargRegisterCall(body, map[string]bool{"targ": true})
	if !result {
		t.Error("expected true when Register call exists")
	}
}

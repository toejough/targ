package parse_test

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/toejough/targ/buildtool/internal/parse"
)

func TestCamelToKebab(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"FooBar", "foo-bar"},
		{"APIServer", "api-server"},
		{"Build", "build"},
		{"", ""},
	}

	for _, tc := range tests {
		result := parse.CamelToKebab(tc.input)
		if result != tc.expected {
			t.Errorf("CamelToKebab(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestHasBuildTag(t *testing.T) {
	t.Parallel()

	content := []byte(`//go:build targ

package main
`)
	if !parse.HasBuildTag(content, "targ") {
		t.Fatal("expected build tag to match")
	}

	if parse.HasBuildTag(content, "other") {
		t.Fatal("expected build tag not to match other")
	}
}

func TestHasBuildTag_CommentBeforeBuildTag(t *testing.T) {
	t.Parallel()

	content := []byte(`// Some comment
//go:build targ

package main
`)
	if !parse.HasBuildTag(content, "targ") {
		t.Fatal("expected match with comment before build tag")
	}
}

func TestHasBuildTag_EmptyContent(t *testing.T) {
	t.Parallel()

	if parse.HasBuildTag([]byte(""), "targ") {
		t.Fatal("expected no match for empty content")
	}
}

func TestHasBuildTag_EmptyLinesBeforeBuildTag(t *testing.T) {
	t.Parallel()

	content := []byte(`

//go:build targ

package main
`)
	if !parse.HasBuildTag(content, "targ") {
		t.Fatal("expected match with empty lines before build tag")
	}
}

func TestHasBuildTag_InvalidBuildConstraint(t *testing.T) {
	t.Parallel()

	// Invalid constraint syntax - should fall back to string match
	content := []byte(`//go:build !!!invalid

package main
`)
	// When constraint parsing fails, it falls back to exact string match
	if !parse.HasBuildTag(content, "!!!invalid") {
		t.Fatal("expected match for invalid constraint with exact string match")
	}

	if parse.HasBuildTag(content, "targ") {
		t.Fatal("expected no match for valid tag when constraint is invalid")
	}
}

func TestHasBuildTag_NonCommentLineFirst(t *testing.T) {
	t.Parallel()

	content := []byte(`package main

//go:build targ
`)
	if parse.HasBuildTag(content, "targ") {
		t.Fatal("expected no match when non-comment line comes first")
	}
}

func TestIsGoSourceFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"valid go file", "main.go", true},
		{"test file", "main_test.go", false},
		{"generated file", "generated_targ_build.go", false},
		{"non-go file", "main.txt", false},
		{"go in name", "main.golang", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parse.IsGoSourceFile(tt.filename); got != tt.want {
				t.Errorf("IsGoSourceFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestReceiverTypeName_EmptyList(t *testing.T) {
	t.Parallel()

	// Defensive code path - empty field list
	recv := &ast.FieldList{List: []*ast.Field{}}

	result := parse.ReceiverTypeName(recv)
	if result != "" {
		t.Fatalf("expected empty string for empty list, got %q", result)
	}
}

func TestReceiverTypeName_Nil(t *testing.T) {
	t.Parallel()

	// Defensive code path - nil receiver
	result := parse.ReceiverTypeName(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil receiver, got %q", result)
	}
}

func TestReflectTagGet_Found(t *testing.T) {
	t.Parallel()

	tag := parse.NewReflectTag(`json:"name" targ:"flag"`)
	if got := tag.Get("targ"); got != "flag" {
		t.Fatalf("expected 'flag', got '%s'", got)
	}
}

func TestReflectTagGet_NoColon(t *testing.T) {
	t.Parallel()

	tag := parse.NewReflectTag(`json`)
	if got := tag.Get("json"); got != "" {
		t.Fatalf("expected empty for malformed tag, got '%s'", got)
	}
}

func TestReflectTagGet_NoQuoteAfterColon(t *testing.T) {
	t.Parallel()

	tag := parse.NewReflectTag(`json:name`)
	if got := tag.Get("json"); got != "" {
		t.Fatalf("expected empty for missing quote, got '%s'", got)
	}
}

func TestReflectTagGet_NotFound(t *testing.T) {
	t.Parallel()

	tag := parse.NewReflectTag(`json:"name"`)
	if got := tag.Get("targ"); got != "" {
		t.Fatalf("expected empty, got '%s'", got)
	}
}

func TestReflectTagGet_UnclosedQuote(t *testing.T) {
	t.Parallel()

	tag := parse.NewReflectTag(`json:"name`)
	if got := tag.Get("json"); got != "" {
		t.Fatalf("expected empty for unclosed quote, got '%s'", got)
	}
}

func TestReturnStringLiteral_EmptyBody(t *testing.T) {
	t.Parallel()

	body := &ast.BlockStmt{List: []ast.Stmt{}}

	result, ok := parse.ReturnStringLiteral(body)
	if ok || result != "" {
		t.Fatalf("expected false/empty for empty body, got %q/%v", result, ok)
	}
}

func TestReturnStringLiteral_MultipleResults(t *testing.T) {
	t.Parallel()

	body := &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ReturnStmt{
				Results: []ast.Expr{
					&ast.Ident{Name: "a"},
					&ast.Ident{Name: "b"},
				},
			},
		},
	}

	result, ok := parse.ReturnStringLiteral(body)
	if ok || result != "" {
		t.Fatalf("expected false/empty for multiple results, got %q/%v", result, ok)
	}
}

func TestReturnStringLiteral_NilBody(t *testing.T) {
	t.Parallel()

	result, ok := parse.ReturnStringLiteral(nil)
	if ok || result != "" {
		t.Fatalf("expected false/empty for nil body, got %q/%v", result, ok)
	}
}

func TestReturnStringLiteral_NotBasicLit(t *testing.T) {
	t.Parallel()

	body := &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ReturnStmt{
				Results: []ast.Expr{&ast.Ident{Name: "variable"}},
			},
		},
	}

	result, ok := parse.ReturnStringLiteral(body)
	if ok || result != "" {
		t.Fatalf("expected false/empty for non-literal return, got %q/%v", result, ok)
	}
}

func TestReturnStringLiteral_NotReturnStmt(t *testing.T) {
	t.Parallel()

	body := &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ExprStmt{X: &ast.Ident{Name: "foo"}},
		},
	}

	result, ok := parse.ReturnStringLiteral(body)
	if ok || result != "" {
		t.Fatalf("expected false/empty for non-return stmt, got %q/%v", result, ok)
	}
}

func TestReturnStringLiteral_ValidString(t *testing.T) {
	t.Parallel()

	body := &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ReturnStmt{
				Results: []ast.Expr{
					&ast.BasicLit{Kind: token.STRING, Value: `"hello world"`},
				},
			},
		},
	}

	result, ok := parse.ReturnStringLiteral(body)
	if !ok || result != "hello world" {
		t.Fatalf("expected 'hello world'/true, got %q/%v", result, ok)
	}
}

func TestShouldSkipGoFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"regular go file", "main.go", false},
		{"test file", "main_test.go", true},
		{"generated targ file", "generated_targ_bootstrap.go", true},
		{"non-go file", "readme.md", true},
		{"non-go file txt", "notes.txt", true},
		{"go in name but wrong suffix", "go.mod", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parse.ShouldSkipGoFile(tt.filename)
			if got != tt.want {
				t.Errorf("ShouldSkipGoFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

package parse_test

// LEGACY_TESTS: This file contains tests being evaluated for redundancy.
// Property-based replacements are in *_properties_test.go files.
// Do not add new tests here. See docs/test-migration.md for details.

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/toejough/targ/internal/parse"
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

func TestHasBuildTag_EmptyContent(t *testing.T) {
	t.Parallel()

	if parse.HasBuildTag([]byte(""), "targ") {
		t.Fatal("expected no match for empty content")
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

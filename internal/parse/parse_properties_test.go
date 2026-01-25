package parse_test

import (
	"go/ast"
	"go/token"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/parse"
)

func TestProperty_Parsing(t *testing.T) {
	t.Parallel()

	t.Run("CamelToKebab", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		cases := []struct {
			input    string
			expected string
		}{
			{"FooBar", "foo-bar"},
			{"APIServer", "api-server"},
			{"Build", "build"},
			{"", ""},
		}

		for _, tc := range cases {
			g.Expect(parse.CamelToKebab(tc.input)).To(Equal(tc.expected))
		}
	})

	t.Run("HasBuildTag", func(t *testing.T) {
		t.Parallel()

		t.Run("EmptyContentReturnsFalse", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(parse.HasBuildTag([]byte(""), "targ")).To(BeFalse())
		})
	})

	t.Run("IsGoSourceFile", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		cases := []struct {
			filename string
			want     bool
		}{
			{"main.go", true},
			{"main_test.go", false},
			{"generated_targ_build.go", false},
			{"main.txt", false},
			{"main.golang", false},
		}

		for _, tc := range cases {
			g.Expect(parse.IsGoSourceFile(tc.filename)).
				To(Equal(tc.want), "filename: %s", tc.filename)
		}
	})

	t.Run("ShouldSkipGoFile", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		cases := []struct {
			filename string
			want     bool
		}{
			{"main.go", false},
			{"main_test.go", true},
			{"generated_targ_bootstrap.go", true},
			{"readme.md", true},
			{"notes.txt", true},
			{"go.mod", true},
		}

		for _, tc := range cases {
			g.Expect(parse.ShouldSkipGoFile(tc.filename)).
				To(Equal(tc.want), "filename: %s", tc.filename)
		}
	})

	t.Run("ReflectTag", func(t *testing.T) {
		t.Parallel()

		t.Run("GetFound", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			tag := parse.NewReflectTag(`json:"name" targ:"flag"`)
			g.Expect(tag.Get("targ")).To(Equal("flag"))
		})

		t.Run("GetNoColon", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			tag := parse.NewReflectTag(`json`)
			g.Expect(tag.Get("json")).To(BeEmpty())
		})
	})

	t.Run("ReturnStringLiteral", func(t *testing.T) {
		t.Parallel()

		t.Run("EmptyBody", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			body := &ast.BlockStmt{List: []ast.Stmt{}}
			result, ok := parse.ReturnStringLiteral(body)
			g.Expect(ok).To(BeFalse())
			g.Expect(result).To(BeEmpty())
		})

		t.Run("MultipleResults", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

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
			g.Expect(ok).To(BeFalse())
			g.Expect(result).To(BeEmpty())
		})

		t.Run("NotBasicLit", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			body := &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.ReturnStmt{
						Results: []ast.Expr{&ast.Ident{Name: "variable"}},
					},
				},
			}
			result, ok := parse.ReturnStringLiteral(body)
			g.Expect(ok).To(BeFalse())
			g.Expect(result).To(BeEmpty())
		})

		t.Run("ValidString", func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

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
			g.Expect(ok).To(BeTrue())
			g.Expect(result).To(Equal("hello world"))
		})
	})

	t.Run("CamelToKebabProperty", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			// Generate PascalCase identifiers
			input := rapid.StringMatching(`[A-Z][a-z]+([A-Z][a-z]+)*`).Draw(t, "input")

			result := parse.CamelToKebab(input)

			// Result should be lowercase
			for _, c := range result {
				if c >= 'A' && c <= 'Z' {
					g.Expect(false).To(BeTrue(), "result %q should be lowercase", result)
				}
			}
			// Result should only contain lowercase letters and hyphens
			for _, c := range result {
				valid := (c >= 'a' && c <= 'z') || c == '-'
				g.Expect(valid).To(BeTrue(), "result %q contains invalid character %c", result, c)
			}
		})
	})
}

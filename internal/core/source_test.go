package core

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

func TestProperty_ExtractPackagePath(t *testing.T) {
	t.Parallel()

	t.Run("ExtractedPathIsPrefix", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			// Generate valid function names
			// Format: github.com/user/repo[/pkg].FuncName
			domain := rapid.StringMatching(`[a-z]+\.[a-z]+`).Draw(t, "domain")
			user := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "user")
			repo := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "repo")

			// Optional nested packages
			pkgSegments := rapid.SliceOfN(
				rapid.StringMatching(`[a-z][a-z0-9_]*`),
				0, 5,
			).Draw(t, "pkgSegments")

			funcName := rapid.StringMatching(`[A-Z][a-zA-Z0-9]*`).Draw(t, "funcName")

			// Build full function name
			fullName := domain + "/" + user + "/" + repo
			if len(pkgSegments) > 0 {
				fullName += "/" + strings.Join(pkgSegments, "/")
			}
			fullName += "." + funcName

			result := extractPackagePath(fullName)

			// Result should be a prefix of input
			g.Expect(fullName).To(HavePrefix(result),
				"extracted path %q should be prefix of %q", result, fullName)
		})
	})

	t.Run("ExtractedPathHasNoDot", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			// Generate valid function names with various patterns
			domain := rapid.StringMatching(`[a-z]+\.[a-z]+`).Draw(t, "domain")
			user := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "user")
			repo := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "repo")
			pkgSegments := rapid.SliceOfN(
				rapid.StringMatching(`[a-z][a-z0-9_]*`),
				0, 3,
			).Draw(t, "pkgSegments")
			funcName := rapid.StringMatching(`[A-Z][a-zA-Z0-9]*`).Draw(t, "funcName")

			fullName := domain + "/" + user + "/" + repo
			if len(pkgSegments) > 0 {
				fullName += "/" + strings.Join(pkgSegments, "/")
			}
			fullName += "." + funcName

			result := extractPackagePath(fullName)

			// Result should not end with a dot
			if result != "" {
				g.Expect(result).ToNot(HaveSuffix("."),
					"extracted path %q should not end with dot", result)
			}
		})
	})

	t.Run("EmptyInputReturnsEmpty", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		result := extractPackagePath("")
		g.Expect(result).To(BeEmpty(), "empty input should return empty string")
	})

	t.Run("KnownExtractions", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			// Test known conversions with randomized selection
			cases := []struct {
				input    string
				expected string
			}{
				{"github.com/user/repo/pkg.Func", "github.com/user/repo/pkg"},
				{"github.com/user/repo.init", "github.com/user/repo"},
				{"github.com/user/repo.init.0", "github.com/user/repo"},
				{"github.com/user/repo.init.func1", "github.com/user/repo"},
				{"github.com/user/repo/internal/pkg.Func", "github.com/user/repo/internal/pkg"},
				{"", ""},
			}

			idx := rapid.IntRange(0, len(cases)-1).Draw(t, "caseIdx")
			tc := cases[idx]
			g.Expect(extractPackagePath(tc.input)).To(Equal(tc.expected),
				"extractPackagePath(%q)", tc.input)
		})
	})
}

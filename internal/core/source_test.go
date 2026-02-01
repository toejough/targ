// TEST-020: Source location properties - validates caller package path discovery
// traces: ARCH-005

package core_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

func TestProperty_CallerPackagePath(t *testing.T) {
	t.Parallel()

	t.Run("InvalidDepthReturnsError", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			// Very large depth should fail - no stack that deep
			_, err := core.CallerPackagePathForTest(99999)

			g.Expect(err).To(HaveOccurred(),
				"callerPackagePath with invalid depth should return error")
		})
	})
}

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

			result := core.ExtractPackagePathForTest(fullName)

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

			result := core.ExtractPackagePathForTest(fullName)

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

		result := core.ExtractPackagePathForTest("")
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
				// Edge case: package name with no dot (no function name)
				{"simplepkg", "simplepkg"},
				// Edge case: path with no dot after last slash
				{"github.com/user/repo", "github.com/user/repo"},
			}

			idx := rapid.IntRange(0, len(cases)-1).Draw(t, "caseIdx")
			tc := cases[idx]
			g.Expect(core.ExtractPackagePathForTest(tc.input)).To(Equal(tc.expected),
				"extractPackagePath(%q)", tc.input)
		})
	})
}

// TEST-015: Example helpers properties - validates example documentation helpers
// traces: ARCH-007

package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

func TestProperty_ExampleHelpers(t *testing.T) {
	t.Parallel()

	t.Run("EmptyExamplesReturnsEmptySlice", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		examples := targ.EmptyExamples()
		g.Expect(examples).To(BeEmpty())
	})

	t.Run("BuiltinExamplesReturnsNonEmpty", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		examples := targ.BuiltinExamples()
		g.Expect(examples).NotTo(BeEmpty())
	})

	t.Run("AppendBuiltinExamplesPreservesCustomFirst", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			title := rapid.String().Draw(t, "title")
			code := rapid.String().Draw(t, "code")

			custom := targ.Example{Title: title, Code: code}
			result := targ.AppendBuiltinExamples(custom)

			g.Expect(result).NotTo(BeEmpty())
			g.Expect(result[0]).To(Equal(custom))
			g.Expect(len(result)).To(BeNumerically(">", 1))
		})
	})

	t.Run("PrependBuiltinExamplesPreservesCustomLast", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			title := rapid.String().Draw(t, "title")
			code := rapid.String().Draw(t, "code")

			custom := targ.Example{Title: title, Code: code}
			result := targ.PrependBuiltinExamples(custom)

			g.Expect(result).NotTo(BeEmpty())
			g.Expect(result[len(result)-1]).To(Equal(custom))
			g.Expect(len(result)).To(BeNumerically(">", 1))
		})
	})

	t.Run("AppendAndPrependHaveSameLength", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			title := rapid.String().Draw(t, "title")
			code := rapid.String().Draw(t, "code")

			custom := targ.Example{Title: title, Code: code}
			appended := targ.AppendBuiltinExamples(custom)
			prepended := targ.PrependBuiltinExamples(custom)

			g.Expect(appended).To(HaveLen(len(prepended)))
		})
	})
}

func TestProperty_PortableExamplesCompile(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Verify portable examples compile with targ build tag
		// Path is relative to test/ directory
		err := targ.Run("go", "build", "-tags", "targ_example", "../examples/portable/...")
		g.Expect(err).
			ToNot(HaveOccurred(), "portable examples should compile with targ_example build tag")
	})
}

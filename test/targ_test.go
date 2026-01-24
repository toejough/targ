//go:build !targ

package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

func TestAppendBuiltinExamples(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate random custom example
		title := rapid.StringMatching(`[A-Z][a-z]{2,10}`).Draw(rt, "title")
		code := rapid.StringMatching(`[a-z]{5,20}`).Draw(rt, "code")
		custom := targ.Example{Title: title, Code: code}

		examples := targ.AppendBuiltinExamples(custom)

		// Custom should be first, followed by builtins
		g.Expect(examples).To(HaveLen(3))
		g.Expect(examples[0].Title).To(Equal(title))
		g.Expect(examples[0].Code).To(Equal(code))
	})
}

func TestBuiltinExamples(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	examples := targ.BuiltinExamples()
	g.Expect(examples).To(HaveLen(2))
}

func TestEmptyExamples(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	examples := targ.EmptyExamples()
	g.Expect(examples).To(BeEmpty())
}

func TestPrependBuiltinExamples(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate random custom example
		title := rapid.StringMatching(`[A-Z][a-z]{2,10}`).Draw(rt, "title")
		code := rapid.StringMatching(`[a-z]{5,20}`).Draw(rt, "code")
		custom := targ.Example{Title: title, Code: code}

		examples := targ.PrependBuiltinExamples(custom)

		// Builtins should be first, custom should be last
		g.Expect(examples).To(HaveLen(3))
		g.Expect(examples[2].Title).To(Equal(title))
		g.Expect(examples[2].Code).To(Equal(code))
	})
}

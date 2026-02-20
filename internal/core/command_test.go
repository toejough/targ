// TEST-017: Command core properties - validates command execution and examples
// traces: ARCH-011, ARCH-007

package core_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

func TestChainExample(t *testing.T) {
	t.Parallel()

	t.Run("NilNodesReturnsFallback", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		result := core.ChainExampleForTest(nil)
		g.Expect(result.Code).To(ContainSubstring("build"))
		g.Expect(result.Code).To(ContainSubstring("test"))
	})

	t.Run("NestedGroupShowsCaretSyntax", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		sub := &core.CommandNodeForTest{Name: "sub"}
		group := &core.CommandNodeForTest{
			Name:        "infra",
			Subcommands: map[string]*core.CommandNodeForTest{"sub": sub},
		}
		other := &core.CommandNodeForTest{Name: "test"}

		nodes := []*core.CommandNodeForTest{group, other}

		result := core.ChainExampleForTest(nodes)
		g.Expect(result.Code).To(ContainSubstring("^"))
		g.Expect(result.Code).To(ContainSubstring("infra"))
		g.Expect(result.Code).To(ContainSubstring("test"))
	})

	t.Run("FlatTwoSourcesShowsBothNames", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := &core.CommandNodeForTest{Name: "build", SourceFile: "build.go"}
		b := &core.CommandNodeForTest{Name: "lint", SourceFile: "lint.go"}

		nodes := []*core.CommandNodeForTest{a, b}

		result := core.ChainExampleForTest(nodes)
		g.Expect(result.Code).To(ContainSubstring("build"))
		g.Expect(result.Code).To(ContainSubstring("lint"))
	})
}

func TestProperty_ConvertExamplesPreservesShape(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		useNil := rapid.Bool().Draw(t, "useNil")

		var examples []core.Example

		if !useNil {
			count := rapid.IntRange(0, 5).Draw(t, "count")

			examples = make([]core.Example, 0, count)

			for range count {
				ex := core.Example{
					Title: rapid.String().Draw(t, "title"),
					Code:  rapid.String().Draw(t, "code"),
				}
				examples = append(examples, ex)
			}
		}

		result := core.ConvertExamplesForTest(examples)
		if examples == nil {
			g.Expect(result).To(BeNil())
			return
		}

		g.Expect(result).To(HaveLen(len(examples)))

		for i, ex := range examples {
			g.Expect(result[i].Title).To(Equal(ex.Title))
			g.Expect(result[i].Code).To(Equal(ex.Code))
		}
	})
}

func TestProperty_ResolveMoreInfoTextPrefersMoreInfoText(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		moreInfo := rapid.String().
			Filter(func(s string) bool { return s != "" }).
			Draw(t, "moreInfo")
		repoURL := rapid.String().Draw(t, "repoURL")

		opts := core.RunOptions{MoreInfoText: moreInfo, RepoURL: repoURL}
		result := core.ResolveMoreInfoTextForTest(opts)

		g.Expect(result).To(Equal(moreInfo))
	})
}

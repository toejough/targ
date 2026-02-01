// TEST-017: Command core properties - validates command execution and examples
// traces: ARCH-011, ARCH-007

package core_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

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

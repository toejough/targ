package core_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestConvertExamples_Empty(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	result := core.ConvertExamplesForTest([]core.Example{})
	g.Expect(result).To(BeEmpty())
}

func TestResolveMoreInfoText_MoreInfoTextSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	opts := core.RunOptions{MoreInfoText: "https://example.com/docs"}
	result := core.ResolveMoreInfoTextForTest(opts)
	g.Expect(result).To(Equal("https://example.com/docs"))
}

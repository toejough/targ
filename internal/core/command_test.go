package core_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
	"github.com/toejough/targ/internal/help"
)

func TestBuildPositionalParts_NilNode(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// A nil Type means no positionals
	node := &core.CommandNodeForTest{}
	parts, err := core.BuildPositionalPartsForTest(node)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(parts).To(BeEmpty())
}

func TestConvertExamples_Empty(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	result := core.ConvertExamplesForTest([]core.Example{})
	g.Expect(result).To(BeEmpty())
}

func TestConvertExamples_Nil(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	result := core.ConvertExamplesForTest(nil)
	g.Expect(result).To(BeNil())
}

func TestConvertExamples_WithExamples(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	input := []core.Example{
		{Title: "Basic", Code: "targ build"},
		{Title: "Advanced", Code: "targ build --parallel"},
	}

	result := core.ConvertExamplesForTest(input)

	g.Expect(result).To(HaveLen(2))
	g.Expect(result[0]).To(Equal(help.Example{Title: "Basic", Code: "targ build"}))
	g.Expect(result[1]).To(Equal(help.Example{Title: "Advanced", Code: "targ build --parallel"}))
}

func TestPositionalDisplayName_Default(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	item := core.PositionalHelpForTest{}
	g.Expect(core.PositionalDisplayNameForTest(item)).To(Equal("ARG"))
}

func TestPositionalDisplayName_Name(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	item := core.PositionalHelpForTest{Name: "input"}
	g.Expect(core.PositionalDisplayNameForTest(item)).To(Equal("input"))
}

func TestPositionalDisplayName_Placeholder(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	item := core.PositionalHelpForTest{Placeholder: "<file>", Name: "input"}
	g.Expect(core.PositionalDisplayNameForTest(item)).To(Equal("<file>"))
}

func TestResolveMoreInfoText_DetectRepoURLFallback(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	opts := core.RunOptions{}
	result := core.ResolveMoreInfoTextForTest(opts)
	// DetectRepoURL() returns empty or a detected URL; just verify it doesn't panic
	g.Expect(result).To(BeAssignableToTypeOf(""))
}

func TestResolveMoreInfoText_MoreInfoTextSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	opts := core.RunOptions{MoreInfoText: "https://example.com/docs"}
	result := core.ResolveMoreInfoTextForTest(opts)
	g.Expect(result).To(Equal("https://example.com/docs"))
}

func TestResolveMoreInfoText_RepoURLFallback(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	opts := core.RunOptions{RepoURL: "https://github.com/example/repo"}
	result := core.ResolveMoreInfoTextForTest(opts)
	g.Expect(result).To(Equal("https://github.com/example/repo"))
}

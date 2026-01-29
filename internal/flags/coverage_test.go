package flags_test

import (
	"testing"

	. "github.com/onsi/gomega"

	flags "github.com/toejough/targ/internal/flags"
)

func TestBooleanFlagsIncludesShortAndLong(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	m := flags.BooleanFlags()
	g.Expect(m["--help"]).To(BeTrue())
	g.Expect(m["-h"]).To(BeTrue())
	g.Expect(m["--timeout"]).To(BeFalse())
	g.Expect(m["--init"]).To(BeFalse())
}

func TestFindSkipsMultiShortFlag(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	g.Expect(flags.Find("-ab")).To(BeNil())
	g.Expect(flags.Find("-")).To(BeNil())

	def := flags.Find("--help")
	if def == nil {
		t.Fatal("expected --help flag to exist")
	}

	g.Expect(def.Long).To(Equal("help"))
}

func TestGlobalFlagsExcludeRootOnlyHiddenAndRemoved(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	global := flags.GlobalFlags()
	g.Expect(global).To(ContainElement("--help"))
	g.Expect(global).To(ContainElement("--timeout"))
	g.Expect(global).NotTo(ContainElement("--source"))
	g.Expect(global).NotTo(ContainElement("--no-cache"))
	g.Expect(global).NotTo(ContainElement("--init"))
}

func TestPlaceholderNeedsExplanation(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	g.Expect(flags.Placeholder{Name: "<n>"}.NeedsExplanation()).To(BeFalse())
	g.Expect(flags.Placeholder{Name: "<duration>", Format: "time value"}.NeedsExplanation()).
		To(BeTrue())
}

func TestPlaceholdersUsedByFlagsFiltersAndDedupes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	withFormat := flags.Placeholder{Name: "<duration>", Format: "time value"}
	noFormat := flags.Placeholder{Name: "<n>"}
	defs := []flags.Def{
		{Long: "timeout", Placeholder: &withFormat},
		{Long: "backoff", Placeholder: &withFormat},
		{Long: "times", Placeholder: &noFormat},
		{Long: "help"},
	}

	placeholders := flags.PlaceholdersUsedByFlags(defs)
	if len(placeholders) != 1 {
		t.Fatalf("expected 1 placeholder, got %d", len(placeholders))
	}

	g.Expect(placeholders[0].Name).To(Equal("<duration>"))
}

func TestRootOnlyFlagsExcludeGlobal(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	rootOnly := flags.RootOnlyFlags()
	g.Expect(rootOnly).To(ContainElement("--source"))
	g.Expect(rootOnly).To(ContainElement("--completion"))
	g.Expect(rootOnly).NotTo(ContainElement("--help"))
}

func TestVisibleFlagsExcludeHiddenAndRemoved(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	defs := flags.VisibleFlags()

	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Long)
	}

	g.Expect(names).To(ContainElement("help"))
	g.Expect(names).NotTo(ContainElement("no-cache"))
	g.Expect(names).NotTo(ContainElement("init"))
}

func TestWithValuesIncludesValueFlagsOnly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	m := flags.WithValues()
	g.Expect(m["--timeout"]).To(BeTrue())
	g.Expect(m["--source"]).To(BeTrue())
	g.Expect(m["--help"]).To(BeFalse())
}

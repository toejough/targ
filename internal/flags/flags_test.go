package flags_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/flags"
)

func TestBackoffFlagHasDurationPlaceholder(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	def := flags.Find("--backoff")
	if def == nil {
		t.Fatal("expected --backoff flag to exist")
	}

	if def.Placeholder == nil {
		t.Fatal("expected --backoff flag to have placeholder")
	}

	g.Expect(def.Placeholder.Name).To(ContainSubstring("duration"))
}

func TestFindReturnsNilForNonFlagArg(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	def := flags.Find("build")
	g.Expect(def).To(BeNil())
}

func TestFindReturnsNilForUnknownFlag(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	def := flags.Find("--nonexistent")
	g.Expect(def).To(BeNil())
}

func TestFindReturnsNilForUnknownShortFlag(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	def := flags.Find("-z")
	g.Expect(def).To(BeNil())
}

func TestFindWithLongFlagEqualsValue(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// --flag=value format should still find the flag
	def := flags.Find("--timeout=30s")
	if def == nil {
		t.Fatal("expected --timeout=30s to find timeout flag")
	}

	g.Expect(def.Long).To(Equal("timeout"))
}

func TestFindWithShortFlag(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	def := flags.Find("-h")
	if def == nil {
		t.Fatal("expected -h flag to exist")
	}

	g.Expect(def.Long).To(Equal("help"))
}

func TestFlagsWithValueHavePlaceholders(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	for _, f := range flags.All {
		if f.TakesValue && f.Removed == "" && !f.Hidden {
			g.Expect(f.Placeholder).NotTo(BeNil(),
				"flag --%s takes a value but has no placeholder", f.Long)
			g.Expect(f.Placeholder.Name).NotTo(BeEmpty(),
				"flag --%s has placeholder with empty name", f.Long)
		}
	}
}

func TestFlagsWithoutValueHaveNoPlaceholder(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	for _, f := range flags.All {
		if !f.TakesValue && f.Removed == "" {
			g.Expect(f.Placeholder).To(BeNil(),
				"flag --%s doesn't take a value but has placeholder", f.Long)
		}
	}
}

func TestGlobalFlagsReturnsNonRootFlags(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	globalFlags := flags.GlobalFlags()
	g.Expect(globalFlags).NotTo(BeEmpty())
	// --help is a global flag
	g.Expect(globalFlags).To(ContainElement("--help"))
	// --timeout is a global flag
	g.Expect(globalFlags).To(ContainElement("--timeout"))
}

func TestTimeoutFlagHasDurationPlaceholder(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	def := flags.Find("--timeout")
	if def == nil {
		t.Fatal("expected --timeout flag to exist")
	}

	if def.Placeholder == nil {
		t.Fatal("expected --timeout flag to have placeholder")
	}

	g.Expect(def.Placeholder.Name).To(Equal("<duration>"))
}

func TestWithValuesReturnsMapOfFlagsThatTakeValues(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	m := flags.WithValues()
	g.Expect(m).NotTo(BeNil())
	// timeout takes a value
	g.Expect(m["--timeout"]).To(BeTrue())
	// help does not take a value
	g.Expect(m["--help"]).To(BeFalse())
}

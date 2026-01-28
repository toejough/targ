package flags_test

import (
	"testing"

	"github.com/toejough/targ/internal/flags"
	. "github.com/onsi/gomega"
)

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

func TestTimeoutFlagHasDurationPlaceholder(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	def := flags.Find("--timeout")
	g.Expect(def).NotTo(BeNil())
	g.Expect(def.Placeholder).NotTo(BeNil())
	g.Expect(def.Placeholder.Name).To(Equal("<duration>"))
}

func TestBackoffFlagHasDurationPlaceholder(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	def := flags.Find("--backoff")
	g.Expect(def).NotTo(BeNil())
	g.Expect(def.Placeholder).NotTo(BeNil())
	g.Expect(def.Placeholder.Name).To(ContainSubstring("duration"))
}

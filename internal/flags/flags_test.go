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
			g.Expect(f.Placeholder).NotTo(BeEmpty(),
				"flag --%s takes a value but has no placeholder", f.Long)
		}
	}
}

func TestFlagsWithoutValueHaveNoPlaceholder(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	for _, f := range flags.All {
		if !f.TakesValue && f.Removed == "" {
			g.Expect(f.Placeholder).To(BeEmpty(),
				"flag --%s doesn't take a value but has placeholder %q", f.Long, f.Placeholder)
		}
	}
}

func TestTimeoutFlagHasDurationPlaceholder(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	def := flags.Find("--timeout")
	g.Expect(def).NotTo(BeNil())
	g.Expect(def.Placeholder).To(Equal("<duration>"))
}

func TestBackoffFlagHasDurationPlaceholder(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	def := flags.Find("--backoff")
	g.Expect(def).NotTo(BeNil())
	g.Expect(def.Placeholder).To(ContainSubstring("duration"))
}

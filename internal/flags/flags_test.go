// TEST-008: Flag registry properties - validates flag definitions and lookup
// traces: ARCH-001

package flags_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	flags "github.com/toejough/targ/internal/flags"
)

func TestAllFlagsHaveExplicitMode(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	for _, f := range flags.All() {
		// Every flag must have been consciously classified.
		// FlagModeAll (0) is valid for help/completion.
		// FlagModeTargOnly (1) is valid for everything else.
		// We verify by checking that only "help" and "completion" use FlagModeAll.
		if f.Mode == flags.FlagModeAll {
			g.Expect(f.Long).To(BeElementOf("help", "completion"),
				"only help and completion should be FlagModeAll, got: "+f.Long)
		}
	}
}

func TestProperty_FindUnknownShortReturnsNil(t *testing.T) {
	t.Parallel()

	defs := flags.All()

	known := make(map[string]bool, len(defs))

	for _, def := range defs {
		if def.Short != "" {
			known[def.Short] = true
		}
	}

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		short := rapid.StringMatching(`[a-zA-Z]`).Filter(func(s string) bool {
			return !known[s]
		}).Draw(t, "short")

		g.Expect(flags.Find("-" + short)).To(BeNil())
	})
}

// TEST-008: Flag registry properties - validates flag definitions and lookup
// traces: ARCH-001

package flags_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	flags "github.com/toejough/targ/internal/flags"
)

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

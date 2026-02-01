// TEST-025: Flag coverage properties - validates flag lookup with non-single shorts
// traces: ARCH-001

package flags_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	flags "github.com/toejough/targ/internal/flags"
)

func TestProperty_FindRejectsNonSingleShort(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		length := rapid.IntRange(0, 5).Filter(func(n int) bool { return n != 1 }).Draw(t, "length")
		arg := "-" + strings.Repeat("a", length)
		g.Expect(flags.Find(arg)).To(BeNil())
	})
}

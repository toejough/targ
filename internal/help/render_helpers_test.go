// TEST-023: Render helpers properties - validates ANSI stripping and rendering utilities
// traces: ARCH-007

package help_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/help"
)

func TestProperty_StripANSI_RemovesEscapeBytes(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		input := rapid.String().Draw(t, "input")
		result := help.StripANSI(input)

		g.Expect(strings.Contains(result, "\x1b")).To(BeFalse())
	})
}

func TestProperty_StripANSI_RemovesWellFormedSequences(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		plain := rapid.StringMatching("[^\\x1b]*").Draw(t, "plain")
		prefix := rapid.StringMatching("[^\\x1b]*").Draw(t, "prefix")
		suffix := rapid.StringMatching("[^\\x1b]*").Draw(t, "suffix")

		input := prefix + "\x1b[1m" + plain + "\x1b[0m" + suffix
		result := help.StripANSI(input)

		g.Expect(result).To(Equal(prefix + plain + suffix))
		g.Expect(strings.Contains(result, "\x1b")).To(BeFalse())
	})
}

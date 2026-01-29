package flags_test

import (
	"testing"

	. "github.com/onsi/gomega"

	flags "github.com/toejough/targ/internal/flags"
)

func TestFindReturnsNilForUnknownShortFlag(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	def := flags.Find("-z")
	g.Expect(def).To(BeNil())
}

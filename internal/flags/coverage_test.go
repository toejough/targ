package flags_test

import (
	"testing"

	. "github.com/onsi/gomega"

	flags "github.com/toejough/targ/internal/flags"
)

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

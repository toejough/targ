package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ"
)

func TestGroup_PanicsOnEmptyName(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	g.Expect(func() {
		targ.Group("")
	}).To(Panic())
}

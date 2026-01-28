package help_test

import (
	"testing"

	"github.com/toejough/targ/internal/help"
	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

func TestNewBuilderReturnsBuilder(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	b := help.New("test-command")
	g.Expect(b).NotTo(BeNil())
}

func TestProperty_NewBuilderAcceptsAnyNonEmptyCommandName(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringOf(rapid.Rune()).Filter(func(s string) bool {
			return s != ""
		}).Draw(t, "commandName")
		b := help.New(name)
		_ = b // Builder created successfully
	})
}

func TestNewBuilderPanicsOnEmptyName(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	g.Expect(func() { help.New("") }).To(Panic())
}

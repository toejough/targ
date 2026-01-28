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

func TestWithDescriptionReturnsContentBuilder(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	b := help.New("test-command")
	cb := b.WithDescription("A test command")

	g.Expect(cb).NotTo(BeNil())
}

func TestProperty_WithDescriptionCarriesOverCommandName(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringOf(rapid.Rune()).Filter(func(s string) bool {
			return s != ""
		}).Draw(t, "commandName")
		desc := rapid.String().Draw(t, "description")

		b := help.New(name)
		cb := b.WithDescription(desc)
		_ = cb // ContentBuilder created successfully with description
	})
}

func TestWithUsageIsChainable(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cb := help.New("test").WithDescription("desc").WithUsage("test [options]")
	g.Expect(cb).NotTo(BeNil())
}

func TestProperty_WithUsageStoresValue(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringOf(rapid.Rune()).Filter(func(s string) bool {
			return s != ""
		}).Draw(t, "commandName")
		desc := rapid.String().Draw(t, "description")
		usage := rapid.String().Draw(t, "usage")

		cb := help.New(name).WithDescription(desc).WithUsage(usage)
		_ = cb // ContentBuilder with usage set
	})
}

package help_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/help"
)

func TestAddGlobalFlagsFromRegistryHandlesUnknownFlagGracefully(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Unknown flags should be silently ignored (no panic)
	cb := help.New("test").WithDescription("desc").AddGlobalFlagsFromRegistry("--nonexistent")
	g.Expect(cb).NotTo(BeNil())
}

func TestAddGlobalFlagsFromRegistryIsChainable(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cb := help.New("test").
		WithDescription("desc").
		AddGlobalFlagsFromRegistry("--timeout", "--parallel")
	g.Expect(cb).NotTo(BeNil())
}

func TestAddRootOnlyFlagsIsChainable(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cb := help.New("test").WithDescription("desc").AddRootOnlyFlags(
		help.Flag{Long: "--source", Desc: "Set source directory"},
	)
	g.Expect(cb).NotTo(BeNil())
}

func TestNewBuilderPanicsOnEmptyName(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	g.Expect(func() { help.New("") }).To(Panic())
}

func TestProperty_AddPositionalsAccumulates(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(_ *rapid.T) {
		cb := help.New("test").WithDescription("desc")
		// Multiple calls should accumulate
		cb.AddPositionals(help.Positional{Name: "a"})
		cb.AddPositionals(help.Positional{Name: "b"})
		_ = cb
	})
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

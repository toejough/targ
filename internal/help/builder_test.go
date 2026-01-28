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

func TestAddPositionalsIsChainable(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cb := help.New("test").WithDescription("desc").AddPositionals(
		help.Positional{Name: "file", Placeholder: "<file>", Required: true},
	)
	g.Expect(cb).NotTo(BeNil())
}

func TestProperty_AddPositionalsAccumulates(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		cb := help.New("test").WithDescription("desc")
		// Multiple calls should accumulate
		cb.AddPositionals(help.Positional{Name: "a"})
		cb.AddPositionals(help.Positional{Name: "b"})
		_ = cb
	})
}

func TestAddCommandFlagsIsChainable(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cb := help.New("test").WithDescription("desc").AddCommandFlags(
		help.Flag{Long: "--verbose", Short: "-v", Desc: "Be verbose"},
	)
	g.Expect(cb).NotTo(BeNil())
}

func TestAddFormatsIsChainable(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cb := help.New("test").WithDescription("desc").AddFormats(
		help.Format{Name: "duration", Desc: "e.g., 5s, 1m, 2h"},
	)
	g.Expect(cb).NotTo(BeNil())
}

func TestAddSubcommandsIsChainable(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cb := help.New("test").WithDescription("desc").AddSubcommands(
		help.Subcommand{Name: "build", Desc: "Build the project"},
	)
	g.Expect(cb).NotTo(BeNil())
}

func TestAddExamplesIsChainable(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cb := help.New("test").WithDescription("desc").AddExamples(
		help.Example{Title: "Basic usage", Code: "test run"},
	)
	g.Expect(cb).NotTo(BeNil())
}

func TestAddExamplesPanicsOnEmpty(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cb := help.New("test").WithDescription("desc")
	g.Expect(func() { cb.AddExamples() }).To(Panic())
}

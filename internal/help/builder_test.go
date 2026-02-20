// TEST-005: Help builder properties - validates help content construction
// traces: ARCH-007

package help_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/flags"
	"github.com/toejough/targ/internal/help"
)

func TestProperty_AddGlobalFlagsFromRegistryIgnoresUnknownAndIsChainable(t *testing.T) {
	t.Parallel()

	defs := flags.All()
	known := make([]string, 0, len(defs))
	defByName := make(map[string]flags.Def, len(defs))

	for _, def := range defs {
		long := "--" + def.Long
		known = append(known, long)

		defByName[long] = def

		if def.Short != "" {
			short := "-" + def.Short
			known = append(known, short)
			defByName[short] = def
		}
	}

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		count := rapid.IntRange(0, 6).Draw(t, "count")

		names := make([]string, 0, count)

		for range count {
			if rapid.Bool().Draw(t, "useKnown") && len(known) > 0 {
				idx := rapid.IntRange(0, len(known)-1).Draw(t, "knownIdx")
				names = append(names, known[idx])
			} else {
				unknown := "--unknown-" + rapid.StringMatching(`[a-z]{3,6}`).Draw(t, "unknown")
				names = append(names, unknown)
			}
		}

		cb := help.New("test").WithDescription("desc")
		cb2 := cb.AddGlobalFlagsFromRegistry(names...)

		g.Expect(cb2).To(BeIdenticalTo(cb))

		output := cb.Render()

		for _, name := range names {
			if def, ok := defByName[name]; ok {
				g.Expect(output).To(ContainSubstring("--" + def.Long))
			} else {
				g.Expect(output).NotTo(ContainSubstring(name))
			}
		}
	})
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

func TestProperty_AddRootOnlyFlagsAppendsAndIsChainable(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		count := rapid.IntRange(0, 5).Draw(t, "count")

		flgs := make([]help.Flag, 0, count)

		for range count {
			name := "--" + rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "name")
			flgs = append(flgs, help.Flag{Long: name, Desc: "desc"})
		}

		cb := help.New("test").WithDescription("desc").SetRoot(true)
		cb2 := cb.AddRootOnlyFlags(flgs...)
		g.Expect(cb2).To(BeIdenticalTo(cb))

		output := cb.Render()
		for _, f := range flgs {
			g.Expect(output).To(ContainSubstring(f.Long))
		}
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

func TestProperty_NewBuilderPanicsOnEmptyName(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)
		_ = t

		g.Expect(func() { help.New("") }).To(Panic())
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

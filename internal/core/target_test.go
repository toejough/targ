// TEST-016: Target core properties - validates target builder internals
// traces: ARCH-002

package core_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

func TestProperty_DefaultIsNotRenamed(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Create target without calling Name()
		target := core.Targ(func() {})

		// Verify nameOverridden flag is false
		g.Expect(target.IsRenamed()).To(BeFalse(),
			"targets without Name() should have IsRenamed() false")
	})
}

func TestProperty_DefaultSourceIsEmpty(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Create target without setting sourcePkg
		target := core.Targ(func() {})

		// Verify GetSource() returns empty string
		g.Expect(target.GetSource()).To(BeEmpty(),
			"new targets should have empty GetSource()")
	})
}

func TestProperty_DepsOnlyTargetIsNotRenamed(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Create deps-only target
		target := core.Targ()

		// Verify nameOverridden flag is false
		g.Expect(target.IsRenamed()).To(BeFalse(),
			"deps-only targets should have IsRenamed() false")
	})
}

func TestProperty_GetSourceReturnsSetValue(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate random package path
		pkgPath := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkgPath")

		// Create target and set sourcePkg
		target := core.Targ(func() {})
		target.SetSourceForTest(pkgPath)

		// Verify GetSource() returns the same value
		g.Expect(target.GetSource()).To(Equal(pkgPath),
			"GetSource() should return the set sourcePkg value")
	})
}

func TestProperty_NameAfterRegistrationIsRenamed(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate non-empty target name and package path
		name := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name")
		pkgPath := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkgPath")

		// Create target and set sourcePkg (simulating registered remote target)
		target := core.Targ(func() {})
		target.SetSourceForTest(pkgPath)

		// Now call Name() (simulating consumer renaming)
		target.Name(name)

		// Verify name is set
		g.Expect(target.GetName()).To(Equal(name),
			"Name() should set the target name")

		// Verify nameOverridden flag is set
		g.Expect(target.IsRenamed()).To(BeTrue(),
			"calling Name() after registration should set IsRenamed() to true")
	})
}

func TestProperty_NameBeforeRegistrationIsNotRenamed(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate non-empty target name to avoid empty string edge case
		name := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name")

		// Create target without sourcePkg (simulating package author defining target)
		target := core.Targ(func() {}).Name(name)

		// Verify name is set
		g.Expect(target.GetName()).To(Equal(name),
			"Name() should set the target name")

		// Verify nameOverridden flag is false (not registered yet)
		g.Expect(target.IsRenamed()).To(BeFalse(),
			"calling Name() before registration should not set IsRenamed() to true")
	})
}

func TestProperty_ShellCommandTargetIsNotRenamed(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate random shell command
		cmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "cmd")

		// Create shell command target
		target := core.Targ(cmd)

		// Verify nameOverridden flag is false
		g.Expect(target.IsRenamed()).To(BeFalse(),
			"shell command targets should have IsRenamed() false")
	})
}

func TestProperty_DepGroupChaining(t *testing.T) {
	t.Parallel()

	t.Run("SingleSerialGroup", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {})
		b := core.Targ(func() {})
		main := core.Targ(func() {}).Deps(a, b)

		groups := main.GetDepGroups()
		g.Expect(groups).To(HaveLen(1))
		g.Expect(groups[0].Targets).To(Equal([]*core.Target{a, b}))
		g.Expect(groups[0].Mode).To(Equal(core.DepModeSerial))
		g.Expect(main.GetDepMode()).To(Equal(core.DepModeSerial))
	})

	t.Run("SingleParallelGroup", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {})
		b := core.Targ(func() {})
		main := core.Targ(func() {}).Deps(a, b, core.DepModeParallel)

		groups := main.GetDepGroups()
		g.Expect(groups).To(HaveLen(1))
		g.Expect(groups[0].Targets).To(Equal([]*core.Target{a, b}))
		g.Expect(groups[0].Mode).To(Equal(core.DepModeParallel))
		g.Expect(main.GetDepMode()).To(Equal(core.DepModeParallel))
	})

	t.Run("CoalescesSameMode", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {})
		b := core.Targ(func() {})
		main := core.Targ(func() {}).Deps(a).Deps(b)

		groups := main.GetDepGroups()
		g.Expect(groups).To(HaveLen(1))
		g.Expect(groups[0].Targets).To(Equal([]*core.Target{a, b}))
		g.Expect(groups[0].Mode).To(Equal(core.DepModeSerial))
	})

	t.Run("CoalescesSameModeParallel", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {})
		b := core.Targ(func() {})
		main := core.Targ(func() {}).Deps(a, core.DepModeParallel).Deps(b, core.DepModeParallel)

		groups := main.GetDepGroups()
		g.Expect(groups).To(HaveLen(1))
		g.Expect(groups[0].Targets).To(Equal([]*core.Target{a, b}))
		g.Expect(groups[0].Mode).To(Equal(core.DepModeParallel))
	})

	t.Run("MixedModeCreatesMultipleGroups", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {})
		b := core.Targ(func() {})
		c := core.Targ(func() {})
		d := core.Targ(func() {})
		main := core.Targ(func() {}).
			Deps(a).
			Deps(b, c, core.DepModeParallel).
			Deps(d)

		groups := main.GetDepGroups()
		g.Expect(groups).To(HaveLen(3))
		g.Expect(groups[0].Targets).To(Equal([]*core.Target{a}))
		g.Expect(groups[0].Mode).To(Equal(core.DepModeSerial))
		g.Expect(groups[1].Targets).To(Equal([]*core.Target{b, c}))
		g.Expect(groups[1].Mode).To(Equal(core.DepModeParallel))
		g.Expect(groups[2].Targets).To(Equal([]*core.Target{d}))
		g.Expect(groups[2].Mode).To(Equal(core.DepModeSerial))
		g.Expect(main.GetDepMode()).To(Equal(core.DepModeMixed))
	})

	t.Run("GetDepsFlattensAllGroups", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {})
		b := core.Targ(func() {})
		c := core.Targ(func() {})
		main := core.Targ(func() {}).
			Deps(a).
			Deps(b, core.DepModeParallel).
			Deps(c)

		g.Expect(main.GetDeps()).To(Equal([]*core.Target{a, b, c}))
	})

	t.Run("NoDepsReturnsEmptyGroups", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		main := core.Targ(func() {})

		g.Expect(main.GetDepGroups()).To(BeEmpty())
		g.Expect(main.GetDeps()).To(BeEmpty())
		g.Expect(main.GetDepMode()).To(Equal(core.DepModeSerial))
	})
}

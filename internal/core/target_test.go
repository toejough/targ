package core

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

func TestProperty_DefaultIsNotRenamed(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Create target without calling Name()
		target := Targ(func() {})

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
		target := Targ(func() {})

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
		target := Targ()

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
		target := Targ(func() {})
		target.sourcePkg = pkgPath

		// Verify GetSource() returns the same value
		g.Expect(target.GetSource()).To(Equal(pkgPath),
			"GetSource() should return the set sourcePkg value")
	})
}

func TestProperty_NameBeforeRegistrationIsNotRenamed(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate random target name
		name := rapid.String().Draw(t, "name")

		// Create target without sourcePkg (simulating package author defining target)
		target := Targ(func() {}).Name(name)

		// Verify name is set
		g.Expect(target.GetName()).To(Equal(name),
			"Name() should set the target name")

		// Verify nameOverridden flag is false (not registered yet)
		g.Expect(target.IsRenamed()).To(BeFalse(),
			"calling Name() before registration should not set IsRenamed() to true")
	})
}

func TestProperty_NameAfterRegistrationIsRenamed(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate random target name and package path
		name := rapid.String().Draw(t, "name")
		pkgPath := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkgPath")

		// Create target and set sourcePkg (simulating registered remote target)
		target := Targ(func() {})
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

func TestProperty_ShellCommandTargetIsNotRenamed(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate random shell command
		cmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "cmd")

		// Create shell command target
		target := Targ(cmd)

		// Verify nameOverridden flag is false
		g.Expect(target.IsRenamed()).To(BeFalse(),
			"shell command targets should have IsRenamed() false")
	})
}

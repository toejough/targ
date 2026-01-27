package core_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

func TestDeregisterFrom_EmptyPathReturnsError(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset deregistrations before test
		core.ResetDeregistrations()

		// Empty string should always error
		err := core.DeregisterFrom("")

		g.Expect(err).To(HaveOccurred(),
			"DeregisterFrom with empty string should return error")
	})
}

func TestDeregisterFrom_IdempotentForSamePackage(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset deregistrations before test
		core.ResetDeregistrations()

		// Generate a valid package path
		pkgPath := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkgPath")

		// Call twice with same path
		err1 := core.DeregisterFrom(pkgPath)
		err2 := core.DeregisterFrom(pkgPath)

		g.Expect(err1).ToNot(HaveOccurred(),
			"first DeregisterFrom should not error")
		g.Expect(err2).ToNot(HaveOccurred(),
			"second DeregisterFrom should not error")

		// Verify no duplicate in queue
		deregistrations := core.GetDeregistrations()
		count := 0

		for _, path := range deregistrations {
			if path == pkgPath {
				count++
			}
		}

		g.Expect(count).To(Equal(1),
			"package path should only appear once in deregistrations queue")
	})
}

func TestDeregisterFrom_MultipleDifferentPackages(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset deregistrations before test
		core.ResetDeregistrations()

		// Generate multiple distinct package paths
		pkgPaths := rapid.SliceOfNDistinct(
			rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`),
			2, 5, // Between 2 and 5 distinct paths
			func(s string) string { return s },
		).Draw(t, "pkgPaths")

		// Queue all paths
		for _, pkgPath := range pkgPaths {
			err := core.DeregisterFrom(pkgPath)
			g.Expect(err).ToNot(HaveOccurred(),
				"DeregisterFrom should not error for valid paths")
		}

		// Verify all were queued
		deregistrations := core.GetDeregistrations()
		g.Expect(deregistrations).To(HaveLen(len(pkgPaths)),
			"all package paths should be in deregistrations queue")

		for _, pkgPath := range pkgPaths {
			g.Expect(deregistrations).To(ContainElement(pkgPath),
				"each package path should be in deregistrations queue")
		}
	})
}

func TestDeregisterFrom_ValidPathQueuesSuccessfully(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset deregistrations before test
		core.ResetDeregistrations()

		// Generate a valid package path (non-empty string)
		pkgPath := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkgPath")

		// Should queue successfully and return nil
		err := core.DeregisterFrom(pkgPath)

		g.Expect(err).ToNot(HaveOccurred(),
			"DeregisterFrom with valid path should not return error")

		// Verify it was queued
		deregistrations := core.GetDeregistrations()
		g.Expect(deregistrations).To(ContainElement(pkgPath),
			"package path should be in deregistrations queue")
	})
}

func TestRegisterTarget_ExplicitSourceNotOverwritten(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset registry before test
		core.SetRegistry(nil)
		t.Cleanup(func() { core.SetRegistry(nil) })

		// Generate explicit source package path
		explicitSource := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "explicitSource")

		// Create target with explicit source set
		target := core.Targ(func() {}).Name("test-target")
		target.SetSourceForTest(explicitSource)

		// Register the target
		core.RegisterTarget(target)

		// Verify explicit source was preserved
		registry := core.GetRegistry()
		g.Expect(registry).To(HaveLen(1),
			"registry should contain one item")

		registeredTarget, ok := registry[0].(*core.Target)
		g.Expect(ok).To(BeTrue(),
			"registry item should be a *Target")

		g.Expect(registeredTarget.GetSource()).To(Equal(explicitSource),
			"explicit sourcePkg should not be overwritten")
	})
}

func TestRegisterTarget_LocalTargetsGetLocalSource(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset registry before test
		core.SetRegistry(nil)
		t.Cleanup(func() { core.SetRegistry(nil) })

		// Create target without explicit source
		target := core.Targ(func() {}).Name("test-target")

		// Register from this test package
		core.RegisterTarget(target)

		// Verify source was set to test package
		registry := core.GetRegistry()
		g.Expect(registry).To(HaveLen(1),
			"registry should contain one item")

		registeredTarget, ok := registry[0].(*core.Target)
		g.Expect(ok).To(BeTrue(),
			"registry item should be a *Target")

		source := registeredTarget.GetSource()
		g.Expect(source).ToNot(BeEmpty(),
			"sourcePkg should be set by RegisterTarget")

		g.Expect(source).To(Equal("github.com/toejough/targ/internal/core_test"),
			"sourcePkg should be set to calling package")
	})
}

func TestRegisterTarget_NonTargetItemsHandledGracefully(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset registry before test
		core.SetRegistry(nil)
		t.Cleanup(func() { core.SetRegistry(nil) })

		// Create various types to register
		target := core.Targ(func() {}).Name("test-target")
		groupName := "test-group"
		anotherTarget := core.Targ(func() {}).Name("another-target")

		// Register mixed types (should not panic)
		core.RegisterTarget(target, groupName, anotherTarget)

		// Verify all items were registered
		registry := core.GetRegistry()
		g.Expect(registry).To(HaveLen(3),
			"registry should contain all three items")

		// Verify targets have source set
		t1, ok := registry[0].(*core.Target)
		g.Expect(ok).To(BeTrue(),
			"first item should be a *Target")
		g.Expect(t1.GetSource()).ToNot(BeEmpty(),
			"first target should have source set")

		// Group name should be preserved as-is
		groupStr, ok := registry[1].(string)
		g.Expect(ok).To(BeTrue(),
			"second item should be a string")
		g.Expect(groupStr).To(Equal(groupName),
			"group name should be preserved")

		// Second target should have source set
		t2, ok := registry[2].(*core.Target)
		g.Expect(ok).To(BeTrue(),
			"third item should be a *Target")
		g.Expect(t2.GetSource()).ToNot(BeEmpty(),
			"second target should have source set")
	})
}

func TestRegisterTarget_RegisteredTargetsHaveSource(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset registry before test
		core.SetRegistry(nil)
		t.Cleanup(func() { core.SetRegistry(nil) })

		// Generate 1-5 targets to register
		numTargets := rapid.IntRange(1, 5).Draw(t, "numTargets")
		targets := make([]*core.Target, numTargets)

		for i := range numTargets {
			targets[i] = core.Targ(func() {}).Name(fmt.Sprintf("target-%d", i))
		}

		// Convert to []any for RegisterTarget
		items := make([]any, numTargets)
		for i, target := range targets {
			items[i] = target
		}

		// Register all targets
		core.RegisterTarget(items...)

		// Verify all registered targets have non-empty source
		registry := core.GetRegistry()
		g.Expect(registry).To(HaveLen(numTargets),
			"registry should contain all targets")

		for i, item := range registry {
			target, ok := item.(*core.Target)
			g.Expect(ok).To(BeTrue(),
				fmt.Sprintf("registry item %d should be a *Target", i))

			g.Expect(target.GetSource()).ToNot(BeEmpty(),
				fmt.Sprintf("target %d should have non-empty source", i))
		}
	})
}

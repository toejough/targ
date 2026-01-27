package core_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

//nolint:paralleltest // Cannot run in parallel - modifies global registryResolved state
func TestDeregisterFrom_EmptyPathReturnsError(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset deregistrations before test
		core.ResetDeregistrations()
		core.ResetResolved()

		// Empty string should always error
		err := core.DeregisterFrom("")

		g.Expect(err).To(HaveOccurred(),
			"DeregisterFrom with empty string should return error")
	})
}

//nolint:paralleltest // Cannot run in parallel - modifies global deregistrations/registryResolved state
func TestDeregisterFrom_IdempotentForSamePackage(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset deregistrations before test
		core.ResetDeregistrations()
		core.ResetResolved()

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

//nolint:paralleltest // Cannot run in parallel - modifies global deregistrations/registryResolved state
func TestDeregisterFrom_MultipleDifferentPackages(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset deregistrations before test
		core.ResetDeregistrations()
		core.ResetResolved()

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

//nolint:paralleltest // Cannot run in parallel - modifies global deregistrations/registryResolved state
func TestDeregisterFrom_ValidPathQueuesSuccessfully(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset deregistrations before test
		core.ResetDeregistrations()
		core.ResetResolved()

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

// TestProperty_ExecuteRegisteredResolution_ConflictPreventsExecution verifies that
// conflicting targets prevent execution and cause error exit.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_ExecuteRegisteredResolution_ConflictPreventsExecution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate target name
		name := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name")

		// Generate two different package paths
		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)
		pkg1 := pkgGen.Draw(t, "pkg1")
		pkg2 := pkgGen.Filter(func(s string) bool { return s != pkg1 }).
			Draw(t, "pkg2")

		executionCount := 0

		// Create registry with conflict: same name from two packages
		reg := []any{
			func() *core.Target {
				tgt := core.Targ(func() { executionCount++ }).Name(name)
				tgt.SetSourceForTest(pkg1)

				return tgt
			}(),
			func() *core.Target {
				tgt := core.Targ(func() { executionCount++ }).Name(name)
				tgt.SetSourceForTest(pkg2)

				return tgt
			}(),
		}

		// Set up registry with conflict - no deregistrations
		core.SetRegistry(reg)
		core.ResetDeregistrations()
		core.ResetResolved()
		t.Cleanup(func() {
			core.SetRegistry(nil)
			core.ResetDeregistrations()
			core.ResetResolved()
		})

		// Execute with resolution - args specify the conflicting target
		env := core.NewExecuteEnv([]string{"targ", name})
		err := core.ExecuteWithResolution(env, core.RunOptions{AllowDefault: true})

		g.Expect(err).To(HaveOccurred(),
			"registry resolution should fail when conflicts exist")
		g.Expect(executionCount).To(Equal(0),
			"no targets should execute when conflict exists")
		g.Expect(env.ExitCode()).To(Equal(1),
			"should exit with code 1 on conflict")
	})
}

// TestProperty_ExecuteRegisteredResolution_DeregistrationErrorPreventsExecution verifies that
// bad deregistration (unknown package) prevents execution and causes error exit.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_ExecuteRegisteredResolution_DeregistrationErrorPreventsExecution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate two different package paths
		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)
		existingPkg := pkgGen.Draw(t, "existingPkg")
		unknownPkg := pkgGen.Filter(func(s string) bool { return s != existingPkg }).
			Draw(t, "unknownPkg")

		executionCount := 0

		// Create registry with one target
		reg := []any{
			func() *core.Target {
				tgt := core.Targ(func() { executionCount++ }).Name("test-target")
				tgt.SetSourceForTest(existingPkg)

				return tgt
			}(),
		}

		// Set up registry and deregister unknown package
		core.SetRegistry(reg)
		core.ResetDeregistrations()
		core.ResetResolved()

		err := core.DeregisterFrom(unknownPkg)
		g.Expect(err).ToNot(HaveOccurred(), "queueing deregistration should succeed")

		t.Cleanup(func() {
			core.SetRegistry(nil)
			core.ResetDeregistrations()
			core.ResetResolved()
		})

		// Execute with resolution - args just use default since only one target
		env := core.NewExecuteEnv([]string{"targ"})
		err = core.ExecuteWithResolution(env, core.RunOptions{AllowDefault: true})

		g.Expect(err).To(HaveOccurred(),
			"registry resolution should fail when deregistration errors")
		g.Expect(executionCount).To(Equal(0),
			"no targets should execute when deregistration fails")
		g.Expect(env.ExitCode()).To(Equal(1),
			"should exit with code 1 on deregistration error")
	})
}

// TestProperty_ExecuteRegisteredResolution_ExistingBehaviorUnchanged verifies that
// registry resolution works as before when there are no deregistrations and no conflicts.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_ExecuteRegisteredResolution_ExistingBehaviorUnchanged(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate unique target names
		numTargets := rapid.IntRange(1, 5).Draw(t, "numTargets")
		names := make(map[string]bool)
		reg := make([]any, 0, numTargets)

		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)

		executionCount := 0

		for i := range numTargets {
			// Use simple sequential names
			name := fmt.Sprintf("test%d", i)
			names[name] = true

			// Generate random package
			pkg := pkgGen.Draw(t, "pkg")

			// Create target that increments counter
			target := core.Targ(func() { executionCount++ }).Name(name)
			target.SetSourceForTest(pkg)
			reg = append(reg, target)
		}

		// Set up clean registry - no deregistrations
		core.SetRegistry(reg)
		core.ResetDeregistrations()
		core.ResetResolved()
		t.Cleanup(func() {
			core.SetRegistry(nil)
			core.ResetDeregistrations()
			core.ResetResolved()
		})

		// Execute first target via ExecuteWithResolution
		// With multiple targets, need to specify name in args
		firstTarget, ok := reg[0].(*core.Target)
		g.Expect(ok).To(BeTrue(), "first item should be a *Target")

		args := []string{"targ"}
		if numTargets > 1 {
			args = append(args, firstTarget.GetName())
		}

		env := core.NewExecuteEnv(args)
		err := core.ExecuteWithResolution(env, core.RunOptions{AllowDefault: true})

		g.Expect(err).ToNot(HaveOccurred(),
			"registry resolution should succeed with clean registry")
		g.Expect(executionCount).To(Equal(1),
			"target should execute normally when no conflicts/deregistrations")
	})
}

//nolint:paralleltest // Cannot run in parallel - modifies global registry state
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

//nolint:paralleltest // Cannot run in parallel - modifies global registry state
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

//nolint:paralleltest // Cannot run in parallel - modifies global registry state
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

//nolint:paralleltest // Cannot run in parallel - modifies global registry state
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

// TestDeregisterFromAfterResolutionErrors verifies that DeregisterFrom
// returns an error after resolveRegistry has run.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestDeregisterFromAfterResolutionErrors(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset state before test
		core.SetRegistry(nil)
		core.ResetDeregistrations()
		core.ResetResolved()
		t.Cleanup(func() {
			core.SetRegistry(nil)
			core.ResetDeregistrations()
			core.ResetResolved()
		})

		// Generate a package path
		pkgPath := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkgPath")

		// Trigger resolution (even with empty registry)
		env := core.NewExecuteEnv([]string{"targ"})
		_ = core.ExecuteWithResolution(env, core.RunOptions{AllowDefault: true})

		// Now DeregisterFrom should error
		err := core.DeregisterFrom(pkgPath)

		g.Expect(err).To(HaveOccurred(),
			"DeregisterFrom should error after resolveRegistry has run")
	})
}

// TestDeregisterFromBeforeResolutionSucceeds verifies that DeregisterFrom
// works normally before resolution.
//
//nolint:paralleltest // Cannot run in parallel - checks global registryResolved state
func TestDeregisterFromBeforeResolutionSucceeds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset deregistrations before test
		core.ResetDeregistrations()
		core.ResetResolved()

		// Generate a valid package path
		pkgPath := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkgPath")

		// Should succeed before resolution
		err := core.DeregisterFrom(pkgPath)

		g.Expect(err).ToNot(HaveOccurred(),
			"DeregisterFrom should succeed before resolution")

		// Verify it was queued
		deregistrations := core.GetDeregistrations()
		g.Expect(deregistrations).To(ContainElement(pkgPath),
			"package path should be in deregistrations queue")
	})
}

//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestRegisterTargetWithSkip_Skip0ResolvesCorePackage(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset registry before test
		core.SetRegistry(nil)
		t.Cleanup(func() { core.SetRegistry(nil) })

		// Create target without explicit source
		target := core.Targ(func() {}).Name("test-target")

		// Register with skip=0 (should resolve to core package)
		core.RegisterTargetWithSkip(0, target)

		// Verify source was set to core package
		registry := core.GetRegistry()
		g.Expect(registry).To(HaveLen(1),
			"registry should contain one item")

		registeredTarget, ok := registry[0].(*core.Target)
		g.Expect(ok).To(BeTrue(),
			"registry item should be a *Target")

		source := registeredTarget.GetSource()
		g.Expect(source).ToNot(BeEmpty(),
			"sourcePkg should be set by RegisterTargetWithSkip")

		g.Expect(source).To(Equal("github.com/toejough/targ/internal/core"),
			"with skip=0, sourcePkg should be core package itself")
	})
}

//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestRegisterTargetWithSkip_Skip1ResolvesCaller(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset registry before test
		core.SetRegistry(nil)
		t.Cleanup(func() { core.SetRegistry(nil) })

		// Create target without explicit source
		target := core.Targ(func() {}).Name("test-target")

		// Register with skip=1 (should resolve to this test package)
		core.RegisterTargetWithSkip(1, target)

		// Verify source was set to test package
		registry := core.GetRegistry()
		g.Expect(registry).To(HaveLen(1),
			"registry should contain one item")

		registeredTarget, ok := registry[0].(*core.Target)
		g.Expect(ok).To(BeTrue(),
			"registry item should be a *Target")

		source := registeredTarget.GetSource()
		g.Expect(source).ToNot(BeEmpty(),
			"sourcePkg should be set by RegisterTargetWithSkip")

		g.Expect(source).To(Equal("github.com/toejough/targ/internal/core_test"),
			"with skip=1, sourcePkg should be the direct caller's package")
	})
}

// TestErrorMessageMentionsInit verifies that the error message
// from DeregisterFrom after resolution contains "init()".
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestErrorMessageMentionsInit(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset state before test
		core.SetRegistry(nil)
		core.ResetDeregistrations()
		core.ResetResolved()
		t.Cleanup(func() {
			core.SetRegistry(nil)
			core.ResetDeregistrations()
			core.ResetResolved()
		})

		// Generate a package path
		pkgPath := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkgPath")

		// Trigger resolution
		env := core.NewExecuteEnv([]string{"targ"})
		_ = core.ExecuteWithResolution(env, core.RunOptions{AllowDefault: true})

		// Get the error
		err := core.DeregisterFrom(pkgPath)

		g.Expect(err).To(HaveOccurred(),
			"DeregisterFrom should error after resolution")
		g.Expect(err.Error()).To(ContainSubstring("init()"),
			"error message should mention init()")
	})
}

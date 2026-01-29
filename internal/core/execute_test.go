package core_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

// TestDeregisterFromAfterResolutionErrors verifies that DeregisterFrom
// returns an error after resolveRegistry has run.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_DeregisterFromAfterResolutionErrors(t *testing.T) {
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

// TestProperty_DeregisterThenReregister verifies that deregistering a package
// then re-registering individual targets from it preserves the re-registered targets.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_DeregisterThenReregister(t *testing.T) {
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

		// Generate package path
		remotePkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "remotePkg")

		// Generate unique target names
		name1 := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name1")
		name2 := rapid.StringMatching(`[a-z][a-z0-9-]*`).
			Filter(func(s string) bool { return s != name1 }).
			Draw(t, "name2")

		// Simulate remote package init() registering targets
		remoteLint := core.Targ(func() {}).Name(name1)
		remoteLint.SetSourceForTest(remotePkg)

		remoteTest := core.Targ(func() {}).Name(name2)
		remoteTest.SetSourceForTest(remotePkg)

		core.RegisterTarget(remoteLint, remoteTest)

		// Verify both targets are in registry with remote source
		reg := core.GetRegistry()
		g.Expect(reg).To(HaveLen(2), "should have two targets after remote registration")

		// Simulate consumer init() deregistering remote package
		err := core.DeregisterFrom(remotePkg)
		g.Expect(err).ToNot(HaveOccurred(), "DeregisterFrom should succeed")

		// Simulate consumer init() re-registering specific targets
		// These are the SAME Go pointers from remote package
		core.RegisterTarget(remoteLint, remoteTest.Name("unit-test"))

		// Verify registry has 4 items total (2 original + 2 re-registered)
		reg = core.GetRegistry()
		g.Expect(reg).To(HaveLen(4),
			"registry should have original targets plus re-registered ones")

		// Resolve registry - this applies deregistrations
		resolved, _, err := core.ResolveRegistryForTest()
		g.Expect(err).ToNot(HaveOccurred(), "resolveRegistry should succeed")

		// Should preserve the re-registered targets, not remove them
		g.Expect(resolved).To(HaveLen(2),
			"re-registered targets should be preserved after deregistration")

		// Verify the preserved targets have the expected names
		names := make([]string, 0, 2)

		for _, item := range resolved {
			if tgt, ok := item.(*core.Target); ok {
				names = append(names, tgt.GetName())
			}
		}

		g.Expect(names).To(ConsistOf(name1, "unit-test"),
			"should preserve re-registered targets with their names")
	})
}

// TestProperty_DeregisterWithoutReregister verifies that deregistering without
// re-registering still removes all targets from that package (existing behavior).
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_DeregisterWithoutReregister(t *testing.T) {
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

		// Generate package path
		remotePkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "remotePkg")

		// Register multiple targets from remote package
		numTargets := rapid.IntRange(1, 5).Draw(t, "numTargets")
		for i := range numTargets {
			tgt := core.Targ(func() {}).Name(fmt.Sprintf("target-%d", i))
			tgt.SetSourceForTest(remotePkg)
			core.RegisterTarget(tgt)
		}

		// Verify targets are in registry
		reg := core.GetRegistry()
		g.Expect(reg).To(HaveLen(numTargets),
			"should have all targets in registry")

		// Deregister the package WITHOUT re-registering anything
		err := core.DeregisterFrom(remotePkg)
		g.Expect(err).ToNot(HaveOccurred(), "DeregisterFrom should succeed")

		// Resolve registry
		resolved, _, err := core.ResolveRegistryForTest()
		g.Expect(err).ToNot(HaveOccurred(), "resolveRegistry should succeed")

		// All targets should be removed
		g.Expect(resolved).To(BeEmpty(),
			"all targets from deregistered package should be removed")
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

// TestProperty_LocalTargetsHaveSourcePkgCleared verifies that targets from the main module
// have their sourcePkg cleared during resolution.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_LocalTargetsHaveSourcePkgCleared(t *testing.T) {
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
			core.SetMainModuleForTest(nil)
		})

		// Generate main module path
		mainModule := rapid.StringMatching(`github\.com/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "mainModule")

		// Create a local target (from main module)
		localTarget := core.Targ(func() {}).Name("local-target")
		localTarget.SetSourceForTest(mainModule)

		// Set up registry with one local target
		core.SetRegistry([]any{localTarget})

		// Inject main module provider
		core.SetMainModuleForTest(func() (string, bool) {
			return mainModule, true
		})

		// Verify sourcePkg BEFORE resolution
		g.Expect(localTarget.GetSource()).To(Equal(mainModule),
			"local target should have mainModule as sourcePkg before resolution")

		// Call resolveRegistry to clear local sourcePkg
		resolved, _, err := core.ResolveRegistryForTest()
		g.Expect(err).ToNot(HaveOccurred(), "resolveRegistry should not error")
		g.Expect(resolved).To(HaveLen(1), "should have one item in resolved registry")

		// Verify the target's sourcePkg was cleared AFTER resolution
		g.Expect(localTarget.GetSource()).To(BeEmpty(),
			"local target should have empty sourcePkg after resolution")

		// Verify resolved item is the same target with empty sourcePkg
		resolvedTarget, ok := resolved[0].(*core.Target)
		g.Expect(ok).To(BeTrue(), "resolved item should be *core.Target")
		g.Expect(resolvedTarget.GetSource()).To(BeEmpty(),
			"resolved target should have empty sourcePkg")
	})
}

// TestProperty_MixedLocalAndRemoteTargetsHandled verifies that in a registry with both
// local and remote targets, only local ones get sourcePkg cleared.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_MixedLocalAndRemoteTargetsHandled(t *testing.T) {
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
			core.SetMainModuleForTest(nil)
		})

		// Generate main module and external module paths (must be different)
		mainModule := rapid.StringMatching(`github\.com/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "mainModule")
		externalModule := rapid.StringMatching(`github\.com/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Filter(func(s string) bool { return s != mainModule }).
			Draw(t, "externalModule")

		// Create mixed targets
		localTarget := core.Targ(func() {}).Name("local-target")
		localTarget.SetSourceForTest(mainModule)

		remoteTarget := core.Targ(func() {}).Name("remote-target")
		remoteTarget.SetSourceForTest(externalModule)

		// Set up registry with both
		core.SetRegistry([]any{localTarget, remoteTarget})

		// Inject main module provider
		core.SetMainModuleForTest(func() (string, bool) {
			return mainModule, true
		})

		// Call resolveRegistry
		resolved, _, err := core.ResolveRegistryForTest()
		g.Expect(err).ToNot(HaveOccurred(), "resolveRegistry should not error")
		g.Expect(resolved).To(HaveLen(2), "should have two items in resolved registry")

		// Verify local target has empty sourcePkg
		g.Expect(localTarget.GetSource()).To(BeEmpty(),
			"local target should have empty sourcePkg")

		// Verify remote target kept its sourcePkg
		g.Expect(remoteTarget.GetSource()).To(Equal(externalModule),
			"remote target should retain sourcePkg")
	})
}

// TestProperty_RegisterTargetWithSkip_PreservesExplicitGroupSource verifies that
// RegisterTargetWithSkip does not overwrite explicitly set group sourcePkg.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_RegisterTargetWithSkip_PreservesExplicitGroupSource(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset registry before test
		core.SetRegistry(nil)
		t.Cleanup(func() { core.SetRegistry(nil) })

		// Generate explicit source package path
		explicitSource := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "explicitSource")

		// Create a group and set explicit source
		target := core.Targ(func() {}).Name("test-target")
		group := core.Group("test-group", target)
		group.SetSourceForTest(explicitSource)

		// Register the group
		core.RegisterTarget(group)

		// Verify explicit source was preserved
		registry := core.GetRegistry()
		g.Expect(registry).To(HaveLen(1),
			"registry should contain one item")

		registeredGroup, ok := registry[0].(*core.TargetGroup)
		g.Expect(ok).To(BeTrue(),
			"registry item should be a *TargetGroup")

		g.Expect(registeredGroup.GetSource()).To(Equal(explicitSource),
			"explicit group sourcePkg should not be overwritten")
	})
}

// TestProperty_RegisterTargetWithSkip_SetsSourceOnGroups verifies that
// RegisterTargetWithSkip sets sourcePkg on *TargetGroup items.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_RegisterTargetWithSkip_SetsSourceOnGroups(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Reset registry before test
		core.SetRegistry(nil)
		t.Cleanup(func() { core.SetRegistry(nil) })

		// Create a group with a target
		target := core.Targ(func() {}).Name("test-target")
		group := core.Group("test-group", target)

		// Verify group has no source before registration
		g.Expect(group.GetSource()).To(BeEmpty(),
			"group should have empty source before registration")

		// Register the group (calls RegisterTargetWithSkip with skip=2)
		core.RegisterTarget(group)

		// Verify group has source set after registration
		registry := core.GetRegistry()
		g.Expect(registry).To(HaveLen(1),
			"registry should contain one item")

		registeredGroup, ok := registry[0].(*core.TargetGroup)
		g.Expect(ok).To(BeTrue(),
			"registry item should be a *TargetGroup")

		g.Expect(registeredGroup.GetSource()).ToNot(BeEmpty(),
			"group sourcePkg should be set by RegisterTarget")

		g.Expect(registeredGroup.GetSource()).
			To(Equal("github.com/toejough/targ/internal/core_test"),
				"group sourcePkg should be set to calling package")
	})
}

// TestProperty_RemoteTargetsKeepSourcePkg verifies that targets from external modules
// retain their sourcePkg after resolution.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_RemoteTargetsKeepSourcePkg(t *testing.T) {
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
			core.SetMainModuleForTest(nil)
		})

		// Generate main module and external module paths (must be different)
		mainModule := rapid.StringMatching(`github\.com/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "mainModule")
		externalModule := rapid.StringMatching(`github\.com/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Filter(func(s string) bool { return s != mainModule }).
			Draw(t, "externalModule")

		// Create a remote target (from external module)
		remoteTarget := core.Targ(func() {}).Name("remote-target")
		remoteTarget.SetSourceForTest(externalModule)

		// Set up registry with one remote target
		core.SetRegistry([]any{remoteTarget})

		// Inject main module provider
		core.SetMainModuleForTest(func() (string, bool) {
			return mainModule, true
		})

		// Verify sourcePkg BEFORE resolution
		g.Expect(remoteTarget.GetSource()).To(Equal(externalModule),
			"remote target should have externalModule as sourcePkg before resolution")

		// Call resolveRegistry
		resolved, _, err := core.ResolveRegistryForTest()
		g.Expect(err).ToNot(HaveOccurred(), "resolveRegistry should not error")
		g.Expect(resolved).To(HaveLen(1), "should have one item in resolved registry")

		// Verify the target's sourcePkg was PRESERVED AFTER resolution
		g.Expect(remoteTarget.GetSource()).To(Equal(externalModule),
			"remote target should retain sourcePkg after resolution")

		// Verify resolved item is the same target with preserved sourcePkg
		resolvedTarget, ok := resolved[0].(*core.Target)
		g.Expect(ok).To(BeTrue(), "resolved item should be *core.Target")
		g.Expect(resolvedTarget.GetSource()).To(Equal(externalModule),
			"resolved target should retain sourcePkg")
	})
}

// TestProperty_ResolveRegistryReturnsDeregisteredPackages verifies that
// resolveRegistry returns the list of deregistered package paths.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_ResolveRegistryReturnsDeregisteredPackages(t *testing.T) {
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

		// Generate distinct package paths
		numPkgs := rapid.IntRange(1, 3).Draw(t, "numPkgs")
		pkgs := make([]string, 0, numPkgs)

		for i := range numPkgs {
			pkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
				Draw(t, fmt.Sprintf("pkg%d", i))
			pkgs = append(pkgs, pkg)
		}

		// Register targets from each package and deregister
		for i, pkg := range pkgs {
			tgt := core.Targ(func() {}).Name(fmt.Sprintf("tgt-%d", i))
			tgt.SetSourceForTest(pkg)
			core.RegisterTarget(tgt)

			err := core.DeregisterFrom(pkg)
			g.Expect(err).ToNot(HaveOccurred())
		}

		// Resolve registry
		_, deregisteredPkgs, err := core.ResolveRegistryForTest()
		g.Expect(err).ToNot(HaveOccurred(), "resolveRegistry should succeed")

		// Property: deregistered packages list matches what was deregistered
		g.Expect(deregisteredPkgs).To(ConsistOf(pkgs),
			"should return all deregistered package paths")
	})
}

// TestProperty_ResolveRegistryReturnsEmptyDeregisteredWhenNone verifies that
// resolveRegistry returns empty slice when no deregistrations were made.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_ResolveRegistryReturnsEmptyDeregisteredWhenNone(t *testing.T) {
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

		// Register targets without deregistering
		numTargets := rapid.IntRange(0, 3).Draw(t, "numTargets")
		for i := range numTargets {
			tgt := core.Targ(func() {}).Name(fmt.Sprintf("tgt-%d", i))
			core.RegisterTarget(tgt)
		}

		// Resolve registry
		_, deregisteredPkgs, err := core.ResolveRegistryForTest()
		g.Expect(err).ToNot(HaveOccurred(), "resolveRegistry should succeed")

		// Property: no deregistered packages when none were deregistered
		g.Expect(deregisteredPkgs).To(BeEmpty(),
			"should return empty deregistered packages when none were deregistered")
	})
}

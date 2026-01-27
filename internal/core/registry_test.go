package core

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

// TestProperty_DeregisteredPackageFullyRemoved verifies that all targets from a
// deregistered package are removed from the registry.
func TestProperty_DeregisteredPackageFullyRemoved(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate package paths
		deregPkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "deregPkg")

		// Generate registry with targets from deregistered package
		numTargets := rapid.IntRange(1, 10).Draw(t, "numTargets")
		registry := make([]any, numTargets)
		for i := range numTargets {
			target := Targ(func() {})
			target.sourcePkg = deregPkg
			registry[i] = target
		}

		// Apply deregistration
		result, err := applyDeregistrations(registry, []string{deregPkg})

		// Should succeed
		g.Expect(err).To(BeNil(), "deregistering package with targets should succeed")

		// Result should be empty - all targets removed
		g.Expect(result).To(BeEmpty(),
			"all targets from deregistered package should be removed")
	})
}

// TestProperty_OtherPackagesUntouched verifies that targets from non-deregistered
// packages are preserved exactly.
func TestProperty_OtherPackagesUntouched(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate two different package paths
		deregPkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "deregPkg")
		otherPkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Filter(func(s string) bool { return s != deregPkg }).
			Draw(t, "otherPkg")

		// Generate registry with mixed packages
		numDeregTargets := rapid.IntRange(1, 5).Draw(t, "numDeregTargets")
		numOtherTargets := rapid.IntRange(1, 5).Draw(t, "numOtherTargets")

		registry := make([]any, 0, numDeregTargets+numOtherTargets)
		expectedOther := make([]*Target, 0, numOtherTargets)

		// Add targets from deregistered package
		for range numDeregTargets {
			target := Targ(func() {})
			target.sourcePkg = deregPkg
			registry = append(registry, target)
		}

		// Add targets from other package
		for range numOtherTargets {
			target := Targ(func() {})
			target.sourcePkg = otherPkg
			registry = append(registry, target)
			expectedOther = append(expectedOther, target)
		}

		// Apply deregistration
		result, err := applyDeregistrations(registry, []string{deregPkg})

		// Should succeed
		g.Expect(err).To(BeNil(), "deregistering package should succeed")

		// Result should contain only targets from other package
		g.Expect(result).To(HaveLen(numOtherTargets),
			"should preserve all targets from non-deregistered packages")

		// Verify the exact targets are preserved
		for i, item := range result {
			target, ok := item.(*Target)
			g.Expect(ok).To(BeTrue(), "result should contain Target pointers")
			g.Expect(target).To(BeIdenticalTo(expectedOther[i]),
				"should preserve exact target instances")
			g.Expect(target.sourcePkg).To(Equal(otherPkg),
				"preserved targets should have correct source package")
		}
	})
}

// TestProperty_UnknownPackageErrors verifies that deregistering a package with no
// targets in the registry returns an error.
func TestProperty_UnknownPackageErrors(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate two different package paths
		existingPkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "existingPkg")
		unknownPkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Filter(func(s string) bool { return s != existingPkg }).
			Draw(t, "unknownPkg")

		// Create registry with targets from existing package
		numTargets := rapid.IntRange(1, 5).Draw(t, "numTargets")
		registry := make([]any, numTargets)
		for i := range numTargets {
			target := Targ(func() {})
			target.sourcePkg = existingPkg
			registry[i] = target
		}

		// Try to deregister unknown package
		_, err := applyDeregistrations(registry, []string{unknownPkg})

		// Should return DeregistrationError
		g.Expect(err).ToNot(BeNil(), "deregistering unknown package should error")

		var deregErr *DeregistrationError
		g.Expect(err).To(BeAssignableToTypeOf(deregErr),
			"error should be *DeregistrationError")

		deregErr, ok := err.(*DeregistrationError)
		g.Expect(ok).To(BeTrue(), "error should be *DeregistrationError")
		g.Expect(deregErr.PackagePath).To(Equal(unknownPkg),
			"error should contain the unknown package path")
	})
}

// TestProperty_EmptyDeregistrationsNoOp verifies that passing an empty deregistration
// list returns the registry unchanged.
func TestProperty_EmptyDeregistrationsNoOp(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate random package path
		pkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkg")

		// Generate registry with targets
		numTargets := rapid.IntRange(0, 10).Draw(t, "numTargets")
		registry := make([]any, numTargets)
		for i := range numTargets {
			target := Targ(func() {})
			target.sourcePkg = pkg
			registry[i] = target
		}

		// Apply empty deregistrations
		result, err := applyDeregistrations(registry, []string{})

		// Should succeed
		g.Expect(err).To(BeNil(), "empty deregistrations should succeed")

		// Result should be identical to input
		g.Expect(result).To(HaveLen(len(registry)),
			"empty deregistrations should preserve all items")

		for i, item := range result {
			g.Expect(item).To(BeIdenticalTo(registry[i]),
				"empty deregistrations should preserve exact instances")
		}
	})
}

// TestProperty_MultiplePackagesDeregistered verifies that multiple packages can be
// deregistered in a single call.
func TestProperty_MultiplePackagesDeregistered(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate three different package paths
		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)
		pkg1 := pkgGen.Draw(t, "pkg1")
		pkg2 := pkgGen.Filter(func(s string) bool { return s != pkg1 }).
			Draw(t, "pkg2")
		pkg3 := pkgGen.Filter(func(s string) bool { return s != pkg1 && s != pkg2 }).
			Draw(t, "pkg3")

		// Create registry with targets from all three packages
		numPerPkg := rapid.IntRange(1, 3).Draw(t, "numPerPkg")
		registry := make([]any, 0, numPerPkg*3)
		expectedRemaining := make([]*Target, 0, numPerPkg)

		// Add targets from pkg1 (will deregister)
		for range numPerPkg {
			target := Targ(func() {})
			target.sourcePkg = pkg1
			registry = append(registry, target)
		}

		// Add targets from pkg2 (will deregister)
		for range numPerPkg {
			target := Targ(func() {})
			target.sourcePkg = pkg2
			registry = append(registry, target)
		}

		// Add targets from pkg3 (will keep)
		for range numPerPkg {
			target := Targ(func() {})
			target.sourcePkg = pkg3
			registry = append(registry, target)
			expectedRemaining = append(expectedRemaining, target)
		}

		// Deregister pkg1 and pkg2
		result, err := applyDeregistrations(registry, []string{pkg1, pkg2})

		// Should succeed
		g.Expect(err).To(BeNil(), "deregistering multiple packages should succeed")

		// Result should contain only pkg3 targets
		g.Expect(result).To(HaveLen(numPerPkg),
			"should remove all targets from deregistered packages")

		// Verify remaining targets are from pkg3
		for i, item := range result {
			target, ok := item.(*Target)
			g.Expect(ok).To(BeTrue(), "result should contain Target pointers")
			g.Expect(target).To(BeIdenticalTo(expectedRemaining[i]),
				"should preserve exact target instances")
			g.Expect(target.sourcePkg).To(Equal(pkg3),
				"remaining targets should be from non-deregistered package")
		}
	})
}

// TestProperty_NonTargetItemsPreserved verifies that non-Target items in the registry
// are preserved (groups, markers, etc.).
func TestProperty_NonTargetItemsPreserved(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate package path
		pkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkg")

		// Create registry with mixed item types
		numTargets := rapid.IntRange(1, 5).Draw(t, "numTargets")
		numOther := rapid.IntRange(1, 5).Draw(t, "numOther")

		registry := make([]any, 0, numTargets+numOther)

		// Add targets
		for range numTargets {
			target := Targ(func() {})
			target.sourcePkg = pkg
			registry = append(registry, target)
		}

		// Add non-Target items (strings as stand-ins for group markers)
		expectedOther := make([]string, 0, numOther)
		for range numOther {
			marker := rapid.String().Draw(t, "marker")
			registry = append(registry, marker)
			expectedOther = append(expectedOther, marker)
		}

		// Deregister the package
		result, err := applyDeregistrations(registry, []string{pkg})

		// Should succeed
		g.Expect(err).To(BeNil(), "deregistration should succeed")

		// Result should contain only non-Target items
		g.Expect(result).To(HaveLen(numOther),
			"should preserve all non-Target items")

		// Verify non-Target items are preserved
		for i, item := range result {
			str, ok := item.(string)
			g.Expect(ok).To(BeTrue(), "non-Target items should be preserved")
			g.Expect(str).To(Equal(expectedOther[i]),
				"should preserve exact non-Target values")
		}
	})
}

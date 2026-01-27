//nolint:testpackage // Testing unexported applyDeregistrations function
package core

import (
	"errors"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

// TestProperty_CleanRegistryPassesResolution verifies that a registry with no
// deregistrations and no conflicts resolves successfully.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_CleanRegistryPassesResolution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate unique target names
		numTargets := rapid.IntRange(1, 10).Draw(t, "numTargets")
		names := make(map[string]bool)
		reg := make([]any, 0, numTargets)

		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)

		for range numTargets {
			// Generate unique name
			name := rapid.StringMatching(`[a-z][a-z0-9-]*`).
				Filter(func(s string) bool { return !names[s] }).
				Draw(t, "name")
			names[name] = true

			// Generate random package
			pkg := pkgGen.Draw(t, "pkg")

			// Create target
			target := Targ(func() {}).Name(name)
			target.sourcePkg = pkg
			reg = append(reg, target)
		}

		// Set up globals - no deregistrations
		SetRegistry(reg)
		ResetDeregistrations()
		ResetResolved()

		// Resolve registry
		result, err := resolveRegistry()

		// Should succeed
		g.Expect(err).ToNot(HaveOccurred(),
			"resolving clean registry should succeed")

		// Should return all targets unchanged
		g.Expect(result).To(HaveLen(numTargets),
			"should preserve all targets when no deregistrations/conflicts")

		// Verify targets are unchanged
		for i, item := range result {
			g.Expect(item).To(BeIdenticalTo(reg[i]),
				"clean registry should preserve exact target instances")
		}

		// Cleanup
		SetRegistry(nil)
		ResetDeregistrations()
		ResetResolved()
	})
}

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
		result, err := applyDeregistrations(registry, []Deregistration{
			{PackagePath: deregPkg, RegistryLen: len(registry)},
		})

		// Should succeed
		g.Expect(err).ToNot(HaveOccurred(), "deregistering package with targets should succeed")

		// Result should be empty - all targets removed
		g.Expect(result).To(BeEmpty(),
			"all targets from deregistered package should be removed")
	})
}

// TestProperty_DeregistrationBeforeConflictCheck verifies that deregistering one side
// of a conflict resolves it, confirming deregistration happens before conflict detection.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_DeregistrationBeforeConflictCheck(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate target name
		name := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name")

		// Generate two different package paths
		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)
		pkg1 := pkgGen.Draw(t, "pkg1")
		pkg2 := pkgGen.Filter(func(s string) bool { return s != pkg1 }).
			Draw(t, "pkg2")

		// Create registry with conflict: same name from two packages
		reg := []any{
			func() *Target {
				tgt := Targ(func() {}).Name(name)
				tgt.sourcePkg = pkg1

				return tgt
			}(),
			func() *Target {
				tgt := Targ(func() {}).Name(name)
				tgt.sourcePkg = pkg2

				return tgt
			}(),
		}

		// Set up globals - deregister pkg1 to resolve conflict
		SetRegistry(reg)
		ResetDeregistrations()
		ResetResolved()

		err := DeregisterFrom(pkg1)
		g.Expect(err).ToNot(HaveOccurred(), "queueing deregistration should succeed")

		// Resolve registry
		result, err := resolveRegistry()

		// Should succeed - conflict was resolved by deregistration
		g.Expect(err).ToNot(HaveOccurred(),
			"resolving registry should succeed after deregistering one side of conflict")

		// Should contain only pkg2's target
		g.Expect(result).To(HaveLen(1), "should have one target remaining")
		target, ok := result[0].(*Target)
		g.Expect(ok).To(BeTrue(), "result should contain Target pointer")
		g.Expect(target.sourcePkg).To(Equal(pkg2),
			"remaining target should be from non-deregistered package")

		// Deregistration queue should be cleared
		g.Expect(GetDeregistrations()).To(BeEmpty(),
			"deregistration queue should be cleared after resolution")

		// Cleanup
		SetRegistry(nil)
		ResetDeregistrations()
		ResetResolved()
	})
}

// TestProperty_DeregistrationErrorMessage verifies the error message format.
func TestProperty_DeregistrationErrorMessage(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate package path
		pkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkg")

		// Create error
		err := &DeregistrationError{PackagePath: pkg}

		// Verify error message format
		expectedMsg := `targ: DeregisterFrom("` + pkg + `"): no targets registered from this package`
		g.Expect(err.Error()).To(Equal(expectedMsg),
			"error message should match expected format")
	})
}

// TestProperty_DeregistrationErrorStopsResolution verifies that a bad deregistration
// (package not found) returns error and prevents conflict check from running.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_DeregistrationErrorStopsResolution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate two different package paths
		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)
		existingPkg := pkgGen.Draw(t, "existingPkg")
		unknownPkg := pkgGen.Filter(func(s string) bool { return s != existingPkg }).
			Draw(t, "unknownPkg")

		// Create registry with one target
		reg := []any{
			func() *Target {
				tgt := Targ(func() {})
				tgt.sourcePkg = existingPkg

				return tgt
			}(),
		}

		// Set up globals - try to deregister unknown package
		SetRegistry(reg)
		ResetDeregistrations()
		ResetResolved()

		err := DeregisterFrom(unknownPkg)
		g.Expect(err).ToNot(HaveOccurred(), "queueing deregistration should succeed")

		// Resolve registry
		_, err = resolveRegistry()

		// Should return DeregistrationError
		g.Expect(err).To(HaveOccurred(),
			"resolving registry should fail for unknown package deregistration")

		var deregErr *DeregistrationError
		g.Expect(err).To(BeAssignableToTypeOf(deregErr),
			"error should be *DeregistrationError")

		deregErr = &DeregistrationError{}
		ok := errors.As(err, &deregErr)
		g.Expect(ok).To(BeTrue(), "error should be *DeregistrationError")
		g.Expect(deregErr.PackagePath).To(Equal(unknownPkg),
			"error should contain the unknown package path")

		// Deregistration queue should still be cleared even on error
		g.Expect(GetDeregistrations()).To(BeEmpty(),
			"deregistration queue should be cleared even after error")

		// Cleanup
		SetRegistry(nil)
		ResetDeregistrations()
		ResetResolved()
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
		result, err := applyDeregistrations(registry, []Deregistration{})

		// Should succeed
		g.Expect(err).ToNot(HaveOccurred(), "empty deregistrations should succeed")

		// Result should be identical to input
		g.Expect(result).To(HaveLen(len(registry)),
			"empty deregistrations should preserve all items")

		for i, item := range result {
			g.Expect(item).To(BeIdenticalTo(registry[i]),
				"empty deregistrations should preserve exact instances")
		}
	})
}

// TestProperty_ErrorMessageContainsName verifies that ConflictError.Error() includes
// the conflicting target name.
func TestProperty_ErrorMessageContainsName(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate target name
		name := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name")

		// Generate two different package paths
		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)
		pkg1 := pkgGen.Draw(t, "pkg1")
		pkg2 := pkgGen.Filter(func(s string) bool { return s != pkg1 }).
			Draw(t, "pkg2")

		// Create registry with conflict
		registry := []any{
			func() *Target {
				t := Targ(func() {}).Name(name)
				t.sourcePkg = pkg1

				return t
			}(),
			func() *Target {
				t := Targ(func() {}).Name(name)
				t.sourcePkg = pkg2

				return t
			}(),
		}

		// Detect conflicts
		err := detectConflicts(registry)
		g.Expect(err).To(HaveOccurred(), "should return error")

		// Verify error message contains the name
		errMsg := err.Error()
		g.Expect(errMsg).To(ContainSubstring(name),
			"error message should contain the conflicting target name")
	})
}

// TestProperty_ErrorMessageContainsSources verifies that ConflictError.Error() includes
// both source package paths.
func TestProperty_ErrorMessageContainsSources(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate target name
		name := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name")

		// Generate two different package paths
		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)
		pkg1 := pkgGen.Draw(t, "pkg1")
		pkg2 := pkgGen.Filter(func(s string) bool { return s != pkg1 }).
			Draw(t, "pkg2")

		// Create registry with conflict
		registry := []any{
			func() *Target {
				t := Targ(func() {}).Name(name)
				t.sourcePkg = pkg1

				return t
			}(),
			func() *Target {
				t := Targ(func() {}).Name(name)
				t.sourcePkg = pkg2

				return t
			}(),
		}

		// Detect conflicts
		err := detectConflicts(registry)
		g.Expect(err).To(HaveOccurred(), "should return error")

		// Verify error message contains both sources
		errMsg := err.Error()
		g.Expect(errMsg).To(ContainSubstring(pkg1),
			"error message should contain first package path")
		g.Expect(errMsg).To(ContainSubstring(pkg2),
			"error message should contain second package path")
	})
}

// TestProperty_ErrorMessageSuggestsFix verifies that ConflictError.Error() mentions
// DeregisterFrom as a solution.
func TestProperty_ErrorMessageSuggestsFix(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate target name
		name := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name")

		// Generate two different package paths
		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)
		pkg1 := pkgGen.Draw(t, "pkg1")
		pkg2 := pkgGen.Filter(func(s string) bool { return s != pkg1 }).
			Draw(t, "pkg2")

		// Create registry with conflict
		registry := []any{
			func() *Target {
				t := Targ(func() {}).Name(name)
				t.sourcePkg = pkg1

				return t
			}(),
			func() *Target {
				t := Targ(func() {}).Name(name)
				t.sourcePkg = pkg2

				return t
			}(),
		}

		// Detect conflicts
		err := detectConflicts(registry)
		g.Expect(err).To(HaveOccurred(), "should return error")

		// Verify error message suggests DeregisterFrom
		errMsg := err.Error()
		g.Expect(errMsg).To(ContainSubstring("DeregisterFrom"),
			"error message should suggest using DeregisterFrom")
	})
}

// TestProperty_MultipleConflictsAllReported verifies that all conflicts are collected
// and reported, not just the first one found.
func TestProperty_MultipleConflictsAllReported(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate two different target names
		nameGen := rapid.StringMatching(`[a-z][a-z0-9-]*`)
		name1 := nameGen.Draw(t, "name1")
		name2 := nameGen.Filter(func(s string) bool { return s != name1 }).
			Draw(t, "name2")

		// Generate three different package paths
		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)
		pkgA := pkgGen.Draw(t, "pkgA")
		pkgB := pkgGen.Filter(func(s string) bool { return s != pkgA }).
			Draw(t, "pkgB")
		pkgC := pkgGen.Filter(func(s string) bool { return s != pkgA && s != pkgB }).
			Draw(t, "pkgC")

		// Create registry with multiple conflicts:
		// - name1 from pkgA and pkgB (conflict)
		// - name2 from pkgB and pkgC (conflict)
		registry := []any{
			func() *Target {
				t := Targ(func() {}).Name(name1)
				t.sourcePkg = pkgA

				return t
			}(),
			func() *Target {
				t := Targ(func() {}).Name(name1)
				t.sourcePkg = pkgB

				return t
			}(),
			func() *Target {
				t := Targ(func() {}).Name(name2)
				t.sourcePkg = pkgB

				return t
			}(),
			func() *Target {
				t := Targ(func() {}).Name(name2)
				t.sourcePkg = pkgC

				return t
			}(),
		}

		// Detect conflicts
		err := detectConflicts(registry)
		g.Expect(err).To(HaveOccurred(), "should return error for conflicts")

		var conflictErr *ConflictError

		ok := errors.As(err, &conflictErr)
		g.Expect(ok).To(BeTrue(), "error should be *ConflictError")

		// Should report both conflicts
		g.Expect(conflictErr.Conflicts).To(HaveLen(2),
			"should report all conflicts, not just the first")

		// Verify both conflict names are present
		conflictNames := make([]string, len(conflictErr.Conflicts))
		for i, c := range conflictErr.Conflicts {
			conflictNames[i] = c.Name
		}

		g.Expect(conflictNames).To(ConsistOf(name1, name2),
			"should report conflicts for both names")
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
		result, err := applyDeregistrations(registry, []Deregistration{
			{PackagePath: pkg1, RegistryLen: len(registry)},
			{PackagePath: pkg2, RegistryLen: len(registry)},
		})

		// Should succeed
		g.Expect(err).ToNot(HaveOccurred(), "deregistering multiple packages should succeed")

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
		result, err := applyDeregistrations(registry, []Deregistration{
			{PackagePath: pkg, RegistryLen: len(registry)},
		})

		// Should succeed
		g.Expect(err).ToNot(HaveOccurred(), "deregistration should succeed")

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
		result, err := applyDeregistrations(registry, []Deregistration{
			{PackagePath: deregPkg, RegistryLen: len(registry)},
		})

		// Should succeed
		g.Expect(err).ToNot(HaveOccurred(), "deregistering package should succeed")

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

// TestProperty_QueueClearedAfterResolution verifies that the deregistration queue
// is cleared after resolveRegistry completes, whether it succeeds or fails.
//
//nolint:paralleltest // Cannot run in parallel - modifies global registry state
func TestProperty_QueueClearedAfterResolution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate package path
		pkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkg")

		// Create registry with one target
		reg := []any{
			func() *Target {
				tgt := Targ(func() {})
				tgt.sourcePkg = pkg

				return tgt
			}(),
		}

		// Set up globals - deregister the package
		SetRegistry(reg)
		ResetDeregistrations()
		ResetResolved()

		err := DeregisterFrom(pkg)
		g.Expect(err).ToNot(HaveOccurred(), "queueing deregistration should succeed")

		// Verify queue has entry before resolution
		g.Expect(GetDeregistrations()).To(HaveLen(1),
			"deregistration queue should have entry before resolution")

		// Resolve registry
		_, err = resolveRegistry()
		g.Expect(err).ToNot(HaveOccurred(), "resolution should succeed")

		// Queue should be cleared
		g.Expect(GetDeregistrations()).To(BeEmpty(),
			"deregistration queue should be cleared after successful resolution")

		// Cleanup
		SetRegistry(nil)
		ResetDeregistrations()
		ResetResolved()
	})
}

// TestProperty_SameNameDifferentSourceConflicts verifies that the same name from
// different packages returns a ConflictError.
func TestProperty_SameNameDifferentSourceConflicts(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate target name
		name := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name")

		// Generate two different package paths
		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)
		pkg1 := pkgGen.Draw(t, "pkg1")
		pkg2 := pkgGen.Filter(func(s string) bool { return s != pkg1 }).
			Draw(t, "pkg2")

		// Create registry with same name from different packages
		registry := []any{
			func() *Target {
				t := Targ(func() {}).Name(name)
				t.sourcePkg = pkg1

				return t
			}(),
			func() *Target {
				t := Targ(func() {}).Name(name)
				t.sourcePkg = pkg2

				return t
			}(),
		}

		// Detect conflicts
		err := detectConflicts(registry)

		// Should return ConflictError
		g.Expect(err).To(HaveOccurred(),
			"same name from different packages should conflict")

		var conflictErr *ConflictError
		g.Expect(err).To(BeAssignableToTypeOf(conflictErr),
			"error should be *ConflictError")

		conflictErr = &ConflictError{}
		ok := errors.As(err, &conflictErr)
		g.Expect(ok).To(BeTrue(), "error should be *ConflictError")
		g.Expect(conflictErr.Conflicts).To(HaveLen(1),
			"should report exactly one conflict")
		g.Expect(conflictErr.Conflicts[0].Name).To(Equal(name),
			"conflict should contain the target name")
		g.Expect(conflictErr.Conflicts[0].Sources).To(ConsistOf(pkg1, pkg2),
			"conflict should contain both package paths")
	})
}

// TestProperty_SameNameSameSourceNoConflict verifies that the same name from the same
// package (idempotent registration) is not a conflict.
func TestProperty_SameNameSameSourceNoConflict(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate target name and package
		name := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name")
		pkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkg")

		// Generate multiple targets with same name from same package
		numTargets := rapid.IntRange(2, 5).Draw(t, "numTargets")
		registry := make([]any, numTargets)

		for i := range numTargets {
			target := Targ(func() {}).Name(name)
			target.sourcePkg = pkg
			registry[i] = target
		}

		// Detect conflicts
		err := detectConflicts(registry)

		// Should not error - same source is idempotent
		g.Expect(err).ToNot(HaveOccurred(),
			"same name from same package should not conflict (idempotent)")
	})
}

// TestProperty_UniqueNamesNoConflict verifies that a registry with all unique names
// across packages never returns a conflict error.
func TestProperty_UniqueNamesNoConflict(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate unique target names
		numTargets := rapid.IntRange(1, 10).Draw(t, "numTargets")
		names := make(map[string]bool)
		registry := make([]any, 0, numTargets)

		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)

		for range numTargets {
			// Generate unique name
			name := rapid.StringMatching(`[a-z][a-z0-9-]*`).
				Filter(func(s string) bool { return !names[s] }).
				Draw(t, "name")
			names[name] = true

			// Generate random package
			pkg := pkgGen.Draw(t, "pkg")

			// Create target
			target := Targ(func() {}).Name(name)
			target.sourcePkg = pkg
			registry = append(registry, target)
		}

		// Detect conflicts
		err := detectConflicts(registry)

		// Should not error
		g.Expect(err).ToNot(HaveOccurred(),
			"registry with all unique names should have no conflicts")
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
		_, err := applyDeregistrations(registry, []Deregistration{
			{PackagePath: unknownPkg, RegistryLen: len(registry)},
		})

		// Should return DeregistrationError
		g.Expect(err).To(HaveOccurred(), "deregistering unknown package should error")

		var deregErr *DeregistrationError
		g.Expect(err).To(BeAssignableToTypeOf(deregErr),
			"error should be *DeregistrationError")

		deregErr = &DeregistrationError{}
		ok := errors.As(err, &deregErr)
		g.Expect(ok).To(BeTrue(), "error should be *DeregistrationError")
		g.Expect(deregErr.PackagePath).To(Equal(unknownPkg),
			"error should contain the unknown package path")
	})
}

// TestProperty_ApplyDeregistrations_RemovesGroupsFromDeregisteredPackages verifies that
// groups from deregistered packages are removed just like targets.
func TestProperty_ApplyDeregistrations_RemovesGroupsFromDeregisteredPackages(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate package path to deregister
		deregPkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "deregPkg")

		// Generate groups from deregistered package
		numGroups := rapid.IntRange(1, 10).Draw(t, "numGroups")

		registry := make([]any, numGroups)
		for i := range numGroups {
			target := Targ(func() {})
			group := Group("test-group", target)
			group.SetSourceForTest(deregPkg)
			registry[i] = group
		}

		// Apply deregistration
		result, err := applyDeregistrations(registry, []Deregistration{
			{PackagePath: deregPkg, RegistryLen: len(registry)},
		})

		// Should succeed
		g.Expect(err).ToNot(HaveOccurred(), "deregistering package with groups should succeed")

		// Result should be empty - all groups removed
		g.Expect(result).To(BeEmpty(),
			"all groups from deregistered package should be removed")
	})
}

// TestProperty_ApplyDeregistrations_PreservesGroupsFromOtherPackages verifies that
// groups from non-deregistered packages are preserved.
func TestProperty_ApplyDeregistrations_PreservesGroupsFromOtherPackages(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate two different package paths
		deregPkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "deregPkg")
		otherPkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Filter(func(s string) bool { return s != deregPkg }).
			Draw(t, "otherPkg")

		// Generate groups from both packages
		numDeregGroups := rapid.IntRange(1, 5).Draw(t, "numDeregGroups")
		numOtherGroups := rapid.IntRange(1, 5).Draw(t, "numOtherGroups")

		registry := make([]any, 0, numDeregGroups+numOtherGroups)
		expectedOther := make([]*TargetGroup, 0, numOtherGroups)

		// Add groups from deregistered package
		for i := range numDeregGroups {
			target := Targ(func() {})
			group := Group(fmt.Sprintf("dereg-group-%d", i), target)
			group.SetSourceForTest(deregPkg)
			registry = append(registry, group)
		}

		// Add groups from other package
		for i := range numOtherGroups {
			target := Targ(func() {})
			group := Group(fmt.Sprintf("other-group-%d", i), target)
			group.SetSourceForTest(otherPkg)
			registry = append(registry, group)
			expectedOther = append(expectedOther, group)
		}

		// Apply deregistration
		result, err := applyDeregistrations(registry, []Deregistration{
			{PackagePath: deregPkg, RegistryLen: len(registry)},
		})

		// Should succeed
		g.Expect(err).ToNot(HaveOccurred(), "deregistering package should succeed")

		// Result should contain only groups from other package
		g.Expect(result).To(HaveLen(numOtherGroups),
			"should preserve all groups from non-deregistered packages")

		// Verify the exact groups are preserved
		for i, item := range result {
			group, ok := item.(*TargetGroup)
			g.Expect(ok).To(BeTrue(), "result should contain TargetGroup pointers")
			g.Expect(group).To(BeIdenticalTo(expectedOther[i]),
				"should preserve exact group instances")
			g.Expect(group.GetSource()).To(Equal(otherPkg),
				"preserved groups should have correct source package")
		}
	})
}

// TestProperty_DetectConflicts_CatchesGroupNameConflicts verifies that groups
// with the same name from different packages are detected as conflicts.
func TestProperty_DetectConflicts_CatchesGroupNameConflicts(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate group name
		name := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name")

		// Generate two different package paths
		pkgGen := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`)
		pkg1 := pkgGen.Draw(t, "pkg1")
		pkg2 := pkgGen.Filter(func(s string) bool { return s != pkg1 }).
			Draw(t, "pkg2")

		// Create registry with same group name from different packages
		target := Targ(func() {})
		group1 := Group(name, target)
		group1.SetSourceForTest(pkg1)

		group2 := Group(name, target)
		group2.SetSourceForTest(pkg2)

		registry := []any{group1, group2}

		// Detect conflicts
		err := detectConflicts(registry)

		// Should return ConflictError
		g.Expect(err).To(HaveOccurred(),
			"same group name from different packages should conflict")

		var conflictErr *ConflictError

		ok := errors.As(err, &conflictErr)
		g.Expect(ok).To(BeTrue(), "error should be *ConflictError")
		g.Expect(conflictErr.Conflicts).To(HaveLen(1),
			"should report exactly one conflict")
		g.Expect(conflictErr.Conflicts[0].Name).To(Equal(name),
			"conflict should contain the group name")
		g.Expect(conflictErr.Conflicts[0].Sources).To(ConsistOf(pkg1, pkg2),
			"conflict should contain both package paths")
	})
}

// TestProperty_DetectConflicts_AllowsSameGroupFromSamePackage verifies that
// groups with the same name from the same package are not considered conflicts.
func TestProperty_DetectConflicts_AllowsSameGroupFromSamePackage(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate group name and package
		name := rapid.StringMatching(`[a-z][a-z0-9-]*`).Draw(t, "name")
		pkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "pkg")

		// Generate multiple groups with same name from same package
		numGroups := rapid.IntRange(2, 5).Draw(t, "numGroups")
		registry := make([]any, numGroups)

		for i := range numGroups {
			target := Targ(func() {})
			group := Group(name, target)
			group.SetSourceForTest(pkg)
			registry[i] = group
		}

		// Detect conflicts
		err := detectConflicts(registry)

		// Should not error - same source is idempotent
		g.Expect(err).ToNot(HaveOccurred(),
			"same group name from same package should not conflict (idempotent)")
	})
}

// TestProperty_ClearLocalTargetSources_ClearsGroupsFromMainModule verifies that
// groups from the main module have their sourcePkg cleared.
//
//nolint:paralleltest // Cannot run in parallel - modifies global state via mainModuleProvider
func TestProperty_ClearLocalTargetSources_ClearsGroupsFromMainModule(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate main module path
		mainModule := rapid.StringMatching(`github\.com/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "mainModule")

		// Create a local group (from main module)
		target := Targ(func() {})
		localGroup := Group("local-group", target)
		localGroup.SetSourceForTest(mainModule)

		// Set up registry with one local group
		registry := []any{localGroup}

		// Inject main module provider
		SetMainModuleForTest(func() (string, bool) {
			return mainModule, true
		})
		t.Cleanup(func() { SetMainModuleForTest(nil) })

		// Verify sourcePkg BEFORE clearing
		g.Expect(localGroup.GetSource()).To(Equal(mainModule),
			"local group should have mainModule as sourcePkg before clearing")

		// Call clearLocalTargetSources
		clearLocalTargetSources(registry)

		// Verify the group's sourcePkg was cleared
		g.Expect(localGroup.GetSource()).To(BeEmpty(),
			"local group should have empty sourcePkg after clearing")
	})
}

// TestProperty_ClearLocalTargetSources_PreservesRemoteGroups verifies that
// groups from external modules retain their sourcePkg.
//
//nolint:paralleltest // Cannot run in parallel - modifies global state via mainModuleProvider
func TestProperty_ClearLocalTargetSources_PreservesRemoteGroups(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate main module and external module paths (must be different)
		mainModule := rapid.StringMatching(`github\.com/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "mainModule")
		externalModule := rapid.StringMatching(`github\.com/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Filter(func(s string) bool { return s != mainModule }).
			Draw(t, "externalModule")

		// Create a remote group (from external module)
		target := Targ(func() {})
		remoteGroup := Group("remote-group", target)
		remoteGroup.SetSourceForTest(externalModule)

		// Set up registry with one remote group
		registry := []any{remoteGroup}

		// Inject main module provider
		SetMainModuleForTest(func() (string, bool) {
			return mainModule, true
		})
		t.Cleanup(func() { SetMainModuleForTest(nil) })

		// Verify sourcePkg BEFORE clearing
		g.Expect(remoteGroup.GetSource()).To(Equal(externalModule),
			"remote group should have externalModule as sourcePkg before clearing")

		// Call clearLocalTargetSources
		clearLocalTargetSources(registry)

		// Verify the group's sourcePkg was PRESERVED
		g.Expect(remoteGroup.GetSource()).To(Equal(externalModule),
			"remote group should retain sourcePkg after clearing")
	})
}

// TestProperty_ClearLocalTargetSources_MixedTargetsAndGroups verifies that
// both targets and groups from the main module are cleared.
//
//nolint:paralleltest // Cannot run in parallel - modifies global state via mainModuleProvider
func TestProperty_ClearLocalTargetSources_MixedTargetsAndGroups(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate main module and external module paths (must be different)
		mainModule := rapid.StringMatching(`github\.com/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Draw(t, "mainModule")
		externalModule := rapid.StringMatching(`github\.com/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*`).
			Filter(func(s string) bool { return s != mainModule }).
			Draw(t, "externalModule")

		// Create mixed local and remote items
		localTarget := Targ(func() {}).Name("local-target")
		localTarget.SetSourceForTest(mainModule)

		remoteTarget := Targ(func() {}).Name("remote-target")
		remoteTarget.SetSourceForTest(externalModule)

		target := Targ(func() {})
		localGroup := Group("local-group", target)
		localGroup.SetSourceForTest(mainModule)

		remoteGroup := Group("remote-group", target)
		remoteGroup.SetSourceForTest(externalModule)

		// Set up registry with all items
		registry := []any{localTarget, remoteTarget, localGroup, remoteGroup}

		// Inject main module provider
		SetMainModuleForTest(func() (string, bool) {
			return mainModule, true
		})
		t.Cleanup(func() { SetMainModuleForTest(nil) })

		// Call clearLocalTargetSources
		clearLocalTargetSources(registry)

		// Verify local items have empty sourcePkg
		g.Expect(localTarget.GetSource()).To(BeEmpty(),
			"local target should have empty sourcePkg")
		g.Expect(localGroup.GetSource()).To(BeEmpty(),
			"local group should have empty sourcePkg")

		// Verify remote items kept their sourcePkg
		g.Expect(remoteTarget.GetSource()).To(Equal(externalModule),
			"remote target should retain sourcePkg")
		g.Expect(remoteGroup.GetSource()).To(Equal(externalModule),
			"remote group should retain sourcePkg")
	})
}

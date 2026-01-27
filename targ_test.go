package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
	"github.com/toejough/targ/internal/core"
)

// TestDeregisterFromDelegatesToInternal verifies that the public API
// delegates to the internal implementation with the same behavior.
func TestDeregisterFromDelegatesToInternal(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate a random package path
		packagePath := rapid.String().Draw(t, "packagePath")

		// Reset state before each property check
		core.ResetDeregistrations()

		// Call the public API
		pubErr := targ.DeregisterFrom(packagePath)

		// Reset state again for internal call
		core.ResetDeregistrations()

		// Call the internal implementation
		internalErr := core.DeregisterFrom(packagePath)

		// Both should have the same error state
		if pubErr != nil {
			g.Expect(internalErr).To(HaveOccurred(), "internal should error when public errors")
			g.Expect(pubErr.Error()).To(Equal(internalErr.Error()), "error messages should match")
		} else {
			g.Expect(internalErr).
				NotTo(HaveOccurred(), "internal should succeed when public succeeds")
		}

		// Reset for queue comparison
		core.ResetDeregistrations()

		// Call public API again
		_ = targ.DeregisterFrom(packagePath)
		pubQueue := core.GetDeregistrations()

		// Reset and call internal
		core.ResetDeregistrations()
		_ = core.DeregisterFrom(packagePath)
		internalQueue := core.GetDeregistrations()

		// Queues should be identical
		g.Expect(pubQueue).To(Equal(internalQueue), "deregistration queues should match")
	})
}

// TestDeregisterFromEmptyPackagePath verifies that empty package path is rejected.
func TestDeregisterFromEmptyPackagePath(t *testing.T) {
	g := NewWithT(t)

	err := targ.DeregisterFrom("")
	g.Expect(err).To(MatchError(ContainSubstring("package path")))
}

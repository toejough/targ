package core_test

import (
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

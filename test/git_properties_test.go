package targ_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ"
)

func TestProperty_CheckCleanWorkTree(t *testing.T) {
	t.Parallel()

	// Pins the wrapper to the real git invocation without depending on the
	// checkout's cleanliness: a dead context cannot start the subprocess, so the
	// failure surfaces as the core implementation's wrapped `git diff` error.
	t.Run("CancelledContextReportsTheGitFailure", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := targ.CheckCleanWorkTree(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("git diff"))
		g.Expect(err).To(MatchError(context.Canceled))
	})
}

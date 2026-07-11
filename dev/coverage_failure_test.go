//go:build targ

package dev

import (
	"context"
	"errors"
	"os"
	"testing"

	. "github.com/onsi/gomega"
)

func TestCheckCoverageForFail_CorruptProfileNamesCause(t *testing.T) {
	// t.Chdir forbids t.Parallel.
	g := NewWithT(t)

	t.Chdir(t.TempDir())
	writeErr := os.WriteFile("coverage.out", []byte("garbage not a profile\n"), 0o600)
	g.Expect(writeErr).NotTo(HaveOccurred())

	err := checkCoverageForFail(context.Background(), CoverageCheckArgs{})

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("go tool cover -func=coverage.out"),
		"the failed command must be named")
	g.Expect(err.Error()).To(ContainSubstring("bad mode line"),
		"the tool's own diagnostic must be included")
}

func TestCommandFailure_EmptyOutputOmitsBlock(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	err := commandFailure("deadcode -test ./...", "   \n", errors.New("exit status 1"))

	g.Expect(err.Error()).To(Equal("deadcode -test ./...: exit status 1"))
}

func TestCommandFailure_IncludesCommandAndOutput(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	err := commandFailure("go tool cover -func=coverage.out", "cover: bad mode line: junk\n",
		errors.New("exit status 2"))

	g.Expect(err.Error()).To(ContainSubstring("go tool cover -func=coverage.out"))
	g.Expect(err.Error()).To(ContainSubstring("exit status 2"))
	g.Expect(err.Error()).To(ContainSubstring("cover: bad mode line: junk"))
}

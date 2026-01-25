package targ_test

import (
	"context"
	"os"
	"sync/atomic"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ"
)

func TestTarget_CacheDisabledSetsFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := targ.Targ(func() {}).Cache(targ.Disabled)

	// GetConfig returns (watch, cache, watchDisabled, cacheDisabled)
	_, cache, _, cacheDisabled := target.GetConfig()

	g.Expect(cacheDisabled).To(BeTrue())
	g.Expect(cache).To(BeNil()) // Patterns cleared when disabled
}

func TestTarget_CacheHitSkipsExecution(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Create temp dir and file for cache testing
	tmpDir := t.TempDir()
	inputFile := tmpDir + "/input.txt"
	cacheDir := tmpDir + "/cache"

	err := os.WriteFile(inputFile, []byte("content"), 0o644)
	g.Expect(err).ToNot(HaveOccurred())

	execCount := 0
	target := targ.Targ(func() { execCount++ }).
		Cache(inputFile).
		CacheDir(cacheDir)

	// First run - cache miss, should execute
	err = target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(execCount).To(Equal(1))

	// Second run - cache hit, should skip
	err = target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(execCount).To(Equal(1))
}

func TestTarget_RunWithWatchRerunsOnFileChange(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Create temp dir and file for watch testing
	tmpDir := t.TempDir()
	inputFile := tmpDir + "/input.txt"

	err := os.WriteFile(inputFile, []byte("content"), 0o644)
	g.Expect(err).ToNot(HaveOccurred())

	var execCount atomic.Int32

	target := targ.Targ(func() { execCount.Add(1) }).
		Watch(inputFile)

	// Run in a goroutine with a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- target.Run(ctx)
	}()

	// Wait for initial run
	g.Eventually(execCount.Load).Should(Equal(int32(1)))

	// Modify the file to trigger re-run
	err = os.WriteFile(inputFile, []byte("content2"), 0o644)
	g.Expect(err).ToNot(HaveOccurred())

	// Wait for re-run
	g.Eventually(execCount.Load, "2s").Should(Equal(int32(2)))

	// Cancel and verify it stops
	cancel()

	err = <-done
	g.Expect(err).To(HaveOccurred()) // Should error on context cancellation
}

func TestTarget_WatchBuilderReturnsSameTarget(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	original := targ.Targ(func() {})
	afterWatch := original.Watch("*.go")

	g.Expect(afterWatch).To(BeIdenticalTo(original))
}

func TestTarget_WatchDisabledSetsFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := targ.Targ(func() {}).Watch(targ.Disabled)

	// GetConfig returns (watch, cache, watchDisabled, cacheDisabled)
	watch, _, watchDisabled, _ := target.GetConfig()

	g.Expect(watchDisabled).To(BeTrue())
	g.Expect(watch).To(BeNil()) // Patterns cleared when disabled
}

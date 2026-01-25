package targ_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

// Property: Backoff increases delay between retries
func TestProperty_Backoff_IncreasesDelayBetweenRetries(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	delays := make([]time.Duration, 0)
	lastRun := time.Now()

	execCount := 0
	target := targ.Targ(func() error {
		now := time.Now()
		delays = append(delays, now.Sub(lastRun))
		lastRun = now
		execCount++

		return errors.New("fail")
	}).Times(3).Retry().Backoff(20*time.Millisecond, 2.0)

	err := target.Run(context.Background())
	g.Expect(err).To(HaveOccurred())
	g.Expect(execCount).To(Equal(3))

	// First run has no delay, second ~20ms, third ~40ms
	// Just verify delays exist and are increasing
	g.Expect(delays).To(HaveLen(3))
	g.Expect(delays[2]).To(BeNumerically(">", delays[1]))
}

// Property: String targets execute shell commands via Run().
func TestProperty_ShellCommand_ExecutesViaRun(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate a simple command that will succeed or fail
		shouldFail := rapid.Bool().Draw(rt, "shouldFail")

		var cmd string
		if shouldFail {
			cmd = "exit 1"
		} else {
			cmd = "true"
		}

		target := targ.Targ(cmd)

		err := target.Run(context.Background())
		if shouldFail {
			g.Expect(err).To(HaveOccurred())
		} else {
			g.Expect(err).NotTo(HaveOccurred())
		}
	})
}

// Property: While condition stops execution when false
func TestProperty_While_StopsWhenConditionFalse(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		stopAt := rapid.IntRange(1, 5).Draw(rt, "stopAt")

		execCount := 0
		target := targ.Targ(func() { execCount++ }).
			Times(10).
			While(func() bool { return execCount < stopAt })

		err := target.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(execCount).To(Equal(stopAt))
	})
}

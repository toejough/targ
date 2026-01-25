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

func TestProperty_Execution(t *testing.T) {
	t.Parallel()

	t.Run("MultipleTargetsRunSequentially", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		order := make([]string, 0)
		a := targ.Targ(func() { order = append(order, "a") }).Name("a")
		b := targ.Targ(func() { order = append(order, "b") }).Name("b")

		_, err := targ.Execute([]string{"app", "a", "b"}, a, b)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(order).To(Equal([]string{"a", "b"}))
	})

	t.Run("BackoffIncreasesDelayBetweenRetries", func(t *testing.T) {
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
		g.Expect(delays).To(HaveLen(3))
		g.Expect(delays[2]).To(BeNumerically(">", delays[1]))
	})

	t.Run("ShellCommandExecutesViaRun", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			shouldFail := rapid.Bool().Draw(t, "shouldFail")

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
	})

	t.Run("WhileStopsWhenConditionFalse", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			stopAt := rapid.IntRange(1, 5).Draw(t, "stopAt")

			execCount := 0
			target := targ.Targ(func() { execCount++ }).
				Times(10).
				While(func() bool { return execCount < stopAt })

			err := target.Run(context.Background())
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(execCount).To(Equal(stopAt))
		})
	})
}

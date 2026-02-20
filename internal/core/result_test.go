package core_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestResult(t *testing.T) {
	t.Parallel()

	t.Run("StringValues", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.Pass.String()).To(Equal("PASS"))
		g.Expect(core.Fail.String()).To(Equal("FAIL"))
		g.Expect(core.Cancelled.String()).To(Equal("CANCELLED"))
		g.Expect(core.Errored.String()).To(Equal("ERRORED"))
	})

	t.Run("ClassifyNilIsPass", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyResult(nil, false)).To(Equal(core.Pass))
	})

	t.Run("ClassifyContextCanceledNotFirstIsCancelled", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyResult(context.Canceled, false)).To(Equal(core.Cancelled))
	})

	t.Run("ClassifyContextCanceledFirstIsFail", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyResult(context.Canceled, true)).To(Equal(core.Fail))
	})

	t.Run("ClassifyDeadlineExceededIsErrored", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyResult(context.DeadlineExceeded, false)).To(Equal(core.Errored))
	})

	t.Run("ClassifyOtherErrorIsFail", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyResult(errors.New("boom"), false)).To(Equal(core.Fail))
		g.Expect(core.ClassifyResult(errors.New("boom"), true)).To(Equal(core.Fail))
	})

	t.Run("FormatSummaryNonZeroOnly", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		results := []core.TargetResult{
			{Name: "build", Status: core.Pass},
			{Name: "test", Status: core.Fail},
			{Name: "lint", Status: core.Cancelled},
		}

		g.Expect(core.FormatSummary(results)).To(Equal("PASS:1 FAIL:1 CANCELLED:1"))
	})

	t.Run("FormatSummaryAllPass", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		results := []core.TargetResult{
			{Name: "build", Status: core.Pass},
			{Name: "test", Status: core.Pass},
		}

		g.Expect(core.FormatSummary(results)).To(Equal("PASS:2"))
	})
}

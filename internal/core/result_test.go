package core_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestFormatDetailedSummary(t *testing.T) {
	t.Parallel()

	t.Run("ShowsPerTargetStatus", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		results := []core.TargetResult{
			{Name: "check-coverage", Status: core.Pass, Duration: 1200 * time.Millisecond},
			{
				Name: "lint-full", Status: core.Fail,
				Duration: 800 * time.Millisecond,
				Err:      errors.New("unused variable 'foo' in bar.go:42"),
			},
			{
				Name: "check-nils", Status: core.Fail,
				Duration: 300 * time.Millisecond,
				Err:      errors.New("nil dereference in baz.go:17"),
			},
			{Name: "reorder-decls", Status: core.Pass, Duration: 100 * time.Millisecond},
		}

		summary := core.FormatDetailedSummary(results)

		g.Expect(summary).To(ContainSubstring("PASS"))
		g.Expect(summary).To(ContainSubstring("FAIL"))
		g.Expect(summary).To(ContainSubstring("check-coverage"))
		g.Expect(summary).To(ContainSubstring("lint-full"))
		g.Expect(summary).To(ContainSubstring("unused variable 'foo' in bar.go:42"))
		g.Expect(summary).To(ContainSubstring("nil dereference in baz.go:17"))
		g.Expect(summary).To(ContainSubstring("check-nils"))
		g.Expect(summary).To(ContainSubstring("reorder-decls"))
		// Should end with the counts summary
		g.Expect(summary).To(ContainSubstring("PASS:2 FAIL:2"))
	})

	t.Run("HeaderNote", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		results := []core.TargetResult{
			{Name: "a", Status: core.Fail, Err: errors.New("err")},
		}

		summary := core.FormatDetailedSummary(results)
		g.Expect(summary).To(ContainSubstring("See full output above"))
	})

	t.Run("AllPassNoSnippets", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		results := []core.TargetResult{
			{Name: "a", Status: core.Pass, Duration: 100 * time.Millisecond},
			{Name: "b", Status: core.Pass, Duration: 200 * time.Millisecond},
		}

		summary := core.FormatDetailedSummary(results)
		g.Expect(summary).To(ContainSubstring("PASS:2"))
		g.Expect(summary).NotTo(ContainSubstring("FAIL"))
	})

	t.Run("TruncatesLongSnippet", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		longMsg := strings.Repeat("x", 200)
		results := []core.TargetResult{
			{Name: "a", Status: core.Fail, Err: errors.New(longMsg)},
		}

		summary := core.FormatDetailedSummary(results)
		// Should not contain the full 200-char message
		g.Expect(len(summary)).To(BeNumerically("<", 250))
	})
}

func TestMultiError(t *testing.T) {
	t.Parallel()

	t.Run("ErrorMessageContainsAllFailures", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		results := []core.TargetResult{
			{Name: "lint", Status: core.Fail, Err: errors.New("unused variable 'foo'")},
			{Name: "test", Status: core.Pass},
			{Name: "coverage", Status: core.Fail, Err: errors.New("coverage below 80%")},
		}

		me := core.NewMultiError(results)
		msg := me.Error()

		g.Expect(msg).To(ContainSubstring("lint"))
		g.Expect(msg).To(ContainSubstring("unused variable 'foo'"))
		g.Expect(msg).To(ContainSubstring("coverage"))
		g.Expect(msg).To(ContainSubstring("coverage below 80%"))
	})

	t.Run("ErrorMessageUsesFirstLineOfMultiLineError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		results := []core.TargetResult{
			{
				Name:   "lint",
				Status: core.Fail,
				Err:    errors.New("first line\nsecond line\nthird line"),
			},
		}

		me := core.NewMultiError(results)
		msg := me.Error()

		g.Expect(msg).To(ContainSubstring("first line"))
		g.Expect(msg).NotTo(ContainSubstring("second line"))
	})

	t.Run("ErrorsAsMultiError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		results := []core.TargetResult{
			{Name: "lint", Status: core.Fail, Err: errors.New("fail")},
		}

		me := core.NewMultiError(results)

		var target *core.MultiError

		g.Expect(errors.As(me, &target)).To(BeTrue())
		g.Expect(target.Results()).To(HaveLen(1))
	})

	t.Run("ResultsReturnsAllResults", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		results := []core.TargetResult{
			{Name: "a", Status: core.Pass},
			{Name: "b", Status: core.Fail, Err: errors.New("boom")},
			{Name: "c", Status: core.Errored, Err: errors.New("timeout")},
		}

		me := core.NewMultiError(results)
		g.Expect(me.Results()).To(Equal(results))
	})
}

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

	t.Run("ReportedErrorPreservesMessage", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		inner := errors.New("something failed")
		re := core.NewReportedErrorForTest(inner)

		g.Expect(re.Error()).To(Equal("something failed"))
		g.Expect(errors.Unwrap(re)).To(Equal(inner))
	})

	t.Run("ClassifyCollectAllNilIsPass", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyCollectAllResultForTest(nil)).To(Equal(core.Pass))
	})

	t.Run("ClassifyCollectAllDeadlineExceededIsErrored", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyCollectAllResultForTest(context.DeadlineExceeded)).
			To(Equal(core.Errored))
	})

	t.Run("ClassifyCollectAllOtherErrorIsFail", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyCollectAllResultForTest(errors.New("boom"))).To(Equal(core.Fail))
	})

	t.Run("FirstLineWithNewline", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.FirstLineForTest("hello\nworld")).To(Equal("hello"))
	})

	t.Run("FirstLineWithoutNewline", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.FirstLineForTest("no newline here")).To(Equal("no newline here"))
	})

	t.Run("FormatSummaryWithErrored", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		results := []core.TargetResult{
			{Name: "a", Status: core.Errored},
			{Name: "b", Status: core.Pass},
		}

		g.Expect(core.FormatSummary(results)).To(Equal("PASS:1 ERRORED:1"))
	})
}

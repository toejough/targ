package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Result represents the outcome of a parallel target execution.
type Result int

// Result values.
const (
	Pass Result = iota
	Fail
	Cancelled
	Errored
)

// String returns the string representation of the result.
func (r Result) String() string {
	switch r {
	case Pass:
		return "PASS"
	case Fail:
		return "FAIL"
	case Cancelled:
		return "CANCELLED"
	case Errored:
		return "ERRORED"
	default:
		return "UNKNOWN"
	}
}

// MultiError wraps multiple target failures from a collect-all-errors parallel run.
type MultiError struct {
	results []TargetResult
}

// NewMultiError creates a MultiError from a set of target results.
func NewMultiError(results []TargetResult) *MultiError {
	return &MultiError{results: results}
}

// Error returns a summary of all failures.
func (e *MultiError) Error() string {
	var parts []string

	for _, r := range e.results {
		if r.Err != nil {
			parts = append(parts, fmt.Sprintf("%s: %s", r.Name, firstLine(r.Err.Error())))
		}
	}

	return "multiple targets failed:\n  " + strings.Join(parts, "\n  ")
}

// Results returns the full list of target results.
func (e *MultiError) Results() []TargetResult {
	return e.results
}

// TargetResult holds the outcome of a single target in a parallel group.
type TargetResult struct {
	Name     string
	Status   Result
	Duration time.Duration
	Err      error
}

// ClassifyResult determines the Result from an error and whether this was the first failure.
func ClassifyResult(err error, isFirstFailure bool) Result {
	if err == nil {
		return Pass
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return Errored
	}

	if errors.Is(err, context.Canceled) && !isFirstFailure {
		return Cancelled
	}

	return Fail
}

// FormatDetailedSummary formats results as a detailed per-target summary
// showing status, duration, and error snippet for failures.
func FormatDetailedSummary(results []TargetResult) string {
	var b strings.Builder

	b.WriteString("[See full output above for details]\n\n")

	// Find max name length for alignment
	maxNameLen := 0
	for _, r := range results {
		if n := len(r.Name); n > maxNameLen {
			maxNameLen = n
		}
	}

	for _, r := range results {
		snippet := ""
		if r.Err != nil {
			snippet = firstLine(r.Err.Error())
			if len(snippet) > maxSnippetLen {
				snippet = snippet[:maxSnippetLen] + "..."
			}
		}

		durStr := r.Duration.Round(time.Millisecond).String()

		if snippet != "" {
			fmt.Fprintf(
				&b,
				"  %-9s %-*s  (%s)  %s\n",
				r.Status,
				maxNameLen,
				r.Name,
				durStr,
				snippet,
			)
		} else {
			fmt.Fprintf(&b, "  %-9s %-*s  (%s)\n", r.Status, maxNameLen, r.Name, durStr)
		}
	}

	b.WriteString("\n")
	b.WriteString(FormatSummary(results))

	return b.String()
}

// FormatSummary formats results as a summary line showing only non-zero counts.
// Order: PASS, FAIL, CANCELLED, ERRORED.
func FormatSummary(results []TargetResult) string {
	counts := map[Result]int{}
	for _, r := range results {
		counts[r.Status]++
	}

	var parts []string

	for _, status := range []Result{Pass, Fail, Cancelled, Errored} {
		if c := counts[status]; c > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", status, c))
		}
	}

	return strings.Join(parts, " ")
}

// unexported constants.
const (
	maxSnippetLen = 100
)

// reportedError wraps an error that has already been printed with a target prefix.
// Callers should check for this to avoid double-printing.
type reportedError struct {
	err error
}

func (e reportedError) Error() string { return e.err.Error() }

func (e reportedError) Unwrap() error { return e.err }

func firstLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}

	return s
}

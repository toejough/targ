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

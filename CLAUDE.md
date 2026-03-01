# targ Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-02-19

## Active Technologies

- Go 1.25.5 + gomega (assertions), rapid (property-based testing), lipgloss (ANSI styling, existing) (001-parallel-output)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for Go 1.25.5

## Code Style

Go 1.25.5: Follow standard conventions

- **No new package-level mutable state.** Use dependency injection (context, struct fields, function parameters). If a plan proposes a global variable, flag it and use DI instead. Existing globals are tech debt — don't add to them.
- **Check complexity before adding logic to existing functions.** Lint enforces cyclomatic and cognitive complexity limits. When adding cases to a switch or branches to a function, extract a helper if the function is already near the limit. Don't wait for lint to tell you — anticipate it.
- **Declaration ordering matters.** `go-reorder` enforces ordering: types, then vars, then funcs — alphabetical within each group. When adding new functions or test functions, insert them in alphabetical order, not "near related code." Run `targ reorder-decls` to auto-fix.
- **Fix all instances, not just the first.** When fixing a bug pattern (e.g., old API usage), grep the entire codebase for all occurrences before committing. One instance means there may be more.

## Recent Changes

- 001-parallel-output: Added Go 1.25.5 + gomega (assertions), rapid (property-based testing), lipgloss (ANSI styling, existing)

<!-- MANUAL ADDITIONS START -->

## Testing Rules

- **No flaky tests.** Tests must not depend on wall-clock timing, system load, or scheduling. If a test needs to verify timeout/cancellation behavior, assert on error types (e.g., `context.DeadlineExceeded`) rather than elapsed time.
- **No IO mocking.** Do not mock filesystem, network, or other IO in unit tests. If a test's sole purpose is verifying IO behavior, tag it as an integration test with `//go:build integration`.
- **No pre-existing failures accepted.** If `check-full` fails, it must be fixed before declaring done — even if the failure predates the current change. Every run must be green.
- **Always run `check-full` before declaring done.** Use `targ check-full`. This reports ALL failures at once (lint, coverage, ordering, dead code, nil checks). Do NOT use `check-for-fail` (stops at first error, causes whack-a-mole). Do NOT use bare `go test` as final validation — it misses lint, coverage thresholds, and declaration ordering.
- **Coverage is per-function (80% threshold).** Every exported function must have ≥80% test coverage individually. Adding a new exported function without tests will fail `check-full` even if package-level coverage is fine. Write tests for every new function as part of the TDD cycle, not as a cleanup step.
- **TDD red step is mandatory.** Write the test, run it, confirm it FAILS with the expected error (compilation error or assertion failure). Do NOT write test + implementation together. The red step proves the test actually tests something.

<!-- MANUAL ADDITIONS END -->

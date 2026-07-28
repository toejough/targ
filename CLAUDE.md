# targ Development Guidelines

## Project Structure

Public API at the repo root (`targ.go`); implementation under `internal/`; CLI
binary at `cmd/targ/`; build targets in `dev/`; examples in `examples/`.

## Specs

`openspec/specs/` is the spec of record, organized by bounded context. Legacy
documentation lives in `old-docs/` and is authoritative only where verified
correct against the code — when you find a claim there wrong, delete it and write
the corrected form as an OpenSpec spec rather than editing it in place. A document
that merely *omits* something is not wrong and is not converted. `README.md`'s
Specifications section is the canonical statement of the migration standard.

If you change that standard, change `openspec/config.yaml`'s `context:` block in
the same commit. It restates the standard in full — deliberately, because it is
injected into AI agent prompts where a cross-reference to README would be useless —
so it is the one copy that cannot be replaced by a pointer, and the one that drifts
silently if you forget it.

## Commands

Build, test and lint run through targ, never `go test` directly:
`targ test`, `targ check-full`, `targ reorder-decls`.

## Code Style

Go 1.25.5: Follow standard conventions

- **No new package-level mutable state.** Use dependency injection (context, struct fields, function parameters). If a plan proposes a global variable, flag it and use DI instead. Existing globals are tech debt — don't add to them.
- **Check complexity before adding logic to existing functions.** Lint enforces cyclomatic and cognitive complexity limits. When adding cases to a switch or branches to a function, extract a helper if the function is already near the limit. Don't wait for lint to tell you — anticipate it.
- **Declaration ordering matters.** `go-reorder` enforces ordering: types, then vars, then funcs — alphabetical within each group. When adding new functions or test functions, insert them in alphabetical order, not "near related code." Run `targ reorder-decls` to auto-fix.
- **Fix all instances, not just the first.** When fixing a bug pattern (e.g., old API usage), grep the entire codebase for all occurrences before committing. One instance means there may be more.

## Testing Rules

- **No flaky tests.** Tests must not depend on wall-clock timing, system load, or scheduling. If a test needs to verify timeout/cancellation behavior, assert on error types (e.g., `context.DeadlineExceeded`) rather than elapsed time.
- **No IO mocking.** Do not mock filesystem, network, or other IO in unit tests. If a test's sole purpose is verifying IO behavior, tag it as an integration test with `//go:build integration` and name it `TestIntegration...` — `targ test-integration` selects tagged tests by that prefix, and `check-full` runs it. A tagged test named anything else never executes.
- **No pre-existing failures accepted.** If `check-full` fails, it must be fixed before declaring done — even if the failure predates the current change. Every run must be green.
- **Always run `check-full` before declaring done.** Use `targ check-full`. This reports ALL failures at once (lint, coverage, ordering, dead code, nil checks, integration tests). Do NOT use `check-for-fail` (stops at first error, causes whack-a-mole). Do NOT use bare `go test` as final validation — it misses lint, coverage thresholds, and declaration ordering. One gotcha when a run does fail: a stale `golangci-lint` result cache produces phantom `lint-full` failures citing paths that no longer exist, so run `golangci-lint cache clean` and re-run before treating one as real.
- **Coverage is per-function (80% threshold).** Every exported function must have ≥80% test coverage individually. Adding a new exported function without tests will fail `check-full` even if package-level coverage is fine. Write tests for every new function as part of the TDD cycle, not as a cleanup step.
- **TDD red step is mandatory.** Write the test, run it, confirm it FAILS with the expected error (compilation error or assertion failure). Do NOT write test + implementation together. The red step proves the test actually tests something.

You are reviewing recent development sessions on the targ project to extract permanent lessons. Your job is to identify patterns — corrections the user made, failures encountered, and behaviors that should be prevented or enforced going forward — then propose concrete, permanent fixes (CLAUDE.md updates, hook rules, skill updates, linter configs).

Do NOT propose vague improvements. Every proposal must be a specific file edit, config change, or rule.

---

## Evidence: User Corrections (last ~3 weeks)

These are things the user explicitly corrected or reinforced. Each one is a signal that the LLM's default behavior is wrong:

1. **"No flaky tests — assert on error types, not timing"** — The LLM wrote timeout and backoff tests that asserted on wall-clock elapsed time (`Expect(elapsed).To(BeNumerically("~", 500*time.Millisecond, 100*time.Millisecond))`). These failed under load. Had to be rewritten to assert on error types (e.g., `context.DeadlineExceeded`) and retry counts instead. Commit `875a8b1`.

2. **"Parallel tests must not share mutable state"** — Multiple test files shared global registries and package-level `printOutput` writer. Parallel tests would stomp each other's state. Required injecting `RegistryState` (commit `ec46061`) and threading `io.Writer` through context via `ExecInfo` instead of a global (commit `9c6c213`). The fix was never "remove `t.Parallel()`" — it was always "fix the shared state."

3. **"Use `check-for-fail` before declaring done"** — The LLM would run individual package tests and declare success, but `check-for-fail` (which runs lint + coverage + dead code + nil checks + redundant test detection all together) would catch additional issues. The full pipeline is the source of truth, not individual test passes.

4. **"Don't play whack-a-mole with failures"** — `check-for-fail` stops at the first error. The LLM would fix one issue, re-run, find the next, fix it, re-run, etc. The `CollectAllErrors` option and `check-full` target were created (commit `a783724`) specifically to address this: run everything, collect all failures, fix them in one pass.

5. **"Lint fixes cascade — anticipate the full set"** — After adding new code, lint violations arrive in clusters: nlreturn, cyclomatic complexity, cognitive complexity, declaration ordering, godoclint. Commit `ae5a8bd` shows a typical cleanup commit fixing lint + ordering + coverage together. The LLM should expect multiple lint issues after any code addition and batch-fix them.

6. **"Use `targ` build tooling, not raw `go test`"** — The project has its own build system (`go run -tags targ ./cmd/targ check-for-fail`). The LLM sometimes bypassed it with `go test ./...` directly, missing the lint/coverage/ordering checks that `check-for-fail` runs.

7. **"Coverage thresholds are enforced per-function"** — `check-for-fail` enforces 80% coverage per function, not just per package. Adding a new exported function without tests will fail even if package-level coverage is fine. The LLM has repeatedly been surprised by this.

8. **"No pre-existing failures accepted"** — If `check-for-fail` catches something, it must be fixed — even if it wasn't introduced in the current session. The LLM has tried to excuse failures as "pre-existing." The rule: every run must be green.

9. **"Plans must be followed task-by-task with TDD"** — Implementation plans (like `docs/plans/2026-02-21-help-source-attribution.md`) specify exact TDD steps: write failing test, verify it fails, write implementation, verify it passes, run full suite. The LLM sometimes skips the "verify it fails" step or writes tests and implementation together.

## Evidence: Test & Build Failures

These are failure modes encountered during implementation:

1. **Cyclomatic/cognitive complexity violations** — Adding logic to existing functions (`parseTargetLike`, `applyTagPart`) pushed them over lint thresholds. Required extracting helper functions (`resolveTargetSource`). Commit `ae5a8bd`, also `18aad05` which was a full pass reducing complexity.

2. **Coverage threshold failures** — Multiple new features added exported methods without enough test coverage. The `check-for-fail` pipeline catches these per-function. `check-full` with `CollectAllErrors` reports all of them at once instead of stopping at the first.

3. **Declaration ordering violations** — Go-reorder checks require specific ordering (types, then vars, then funcs, alphabetically within). New functions added out of order trigger failures. The LLM often inserts code where it "makes sense logically" rather than where the ordering rules require.

4. **Global state in tests** — The core package had `printOutput` as a package-level variable and a global registry. Tests using `t.Parallel()` would race. Required full DI refactor (commits `ec46061`, `9c6c213`).

5. **String/deps-only targets had no source attribution** — `targ.Targ("cmd")` and `targ.Targ()` didn't capture where they were defined, so `--help` showed `(unknown)`. Fix required `runtime.Caller(1)` in `Targ()` (commit `a8a0a5e`). This was a gap the LLM didn't anticipate when the source attribution feature was first designed — it only handled function targets.

6. **`--create` generated invalid code** — Code generation used the old variable-based API instead of `targ.Register()` in `init()`. Both the runner and dev targets had this bug — it was fixed twice (commits `9b11754`, `765497e`). The LLM fixed it in one location and missed the second.

## Evidence: Existing Rules Already Captured

The project has CLAUDE.md (project + global) with testing rules, critical warnings, and build system instructions. Key rules:

- "No flaky tests" (CLAUDE.md)
- "No IO mocking" (CLAUDE.md)
- "No pre-existing failures" (CLAUDE.md)
- "Always run `check-for-fail`" (CLAUDE.md)
- "Parallel tests must not share mutable state" (global CLAUDE.md)
- "Don't play whack-a-mole" (global CLAUDE.md)
- "Use build tool commands" (global CLAUDE.md)

The question is: **are these rules actually being followed, or are they just documented and ignored?**

---

## Your Task

Work through this in three phases:

### Phase 1: Pattern Analysis

For each piece of evidence above, answer:
- Is this already captured in CLAUDE.md or global CLAUDE.md? If so, why wasn't it followed?
- Is this a one-off mistake or a recurring pattern?
- What is the root cause? (Wrong default behavior? Missing enforcement? Unclear instruction?)

### Phase 2: Permanent Fixes

For each recurring pattern or root cause, propose ONE of these fix types (prefer enforcement over documentation):

| Fix Type | When to Use | Example |
|----------|-------------|---------|
| **Hook rule** | Deterministic enforcement needed | Block declaring "done" without `check-for-fail` |
| **CLAUDE.md update** | Behavioral guidance for LLM | "Run `check-full` instead of `check-for-fail` to see all errors" |
| **Linter/tool config** | Catch code-level patterns | go-reorder integration |
| **Skill update** | Process gap in a skill | Enforce TDD red step verification |
| **Code pattern** | Structural prevention | Template for lint-safe function extraction |

For each proposal, specify: the exact file to edit, the exact content to add/change, and which evidence item(s) it addresses.

### Phase 3: Verify Coverage

Walk through all 9 user corrections and 6 failure modes above. For each one, confirm which Phase 2 proposal addresses it. Flag any that have no permanent fix proposed.

Present findings at each phase. Wait for user review before proceeding.

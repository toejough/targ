# Coverage-Gate Diagnostics (engram#682) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A failing `check-coverage-for-fail` (and its same-pattern siblings) prints WHY it failed — the underlying tool's diagnostic and the command that produced it — instead of a bare "exit status 2" (toejough/engram#682; the defect lives here in targ).

**Architecture:** `internal.OutputContext` already returns the failed command's combined output (stdout+stderr) alongside the bare error; the gate functions discard it. Fix at the call sites in `dev/targets.go`: one small error-composer helper folds the command and its captured output into the returned error; bare `return err` sites gain context wraps. No behavior change on success paths; no changes to `internal/sh`.

**Tech Stack:** Go 1.25.5, gomega tests in the `dev` package (build tag `targ`), targ's own check-full gate.

## Global Constraints

- This is the TARG repo. Its rules (CLAUDE.md, verbatim): **TDD red step is mandatory** (run the test, confirm it FAILS before implementing); **no IO mocking** (real `go tool cover` in tests is fine); **no flaky tests**; **always run `targ check-full` before declaring done — every run must be green, no pre-existing-failure exceptions**; **declaration ordering** — insert new funcs alphabetically, `targ reorder-decls` to fix; **check complexity before adding logic** — extract helpers rather than fattening near-limit functions; **fix all instances of a bug pattern, not just the first**.
- Tagged dev tests do not run under plain `go test ./...` — run them explicitly during TDD: `go test -tags targ -run '<TestName>' ./dev/`. New test files copy `dev/ansi_lint_test.go`'s header (`//go:build targ`, `package dev`). Final validation is `targ check-full`, never bare `go test`.
- Tests that `t.Chdir` must NOT use `t.Parallel()` (Go forbids the combination); pure-helper tests DO use `t.Parallel()`.
- Every commit: conventional-commit subject **measured ≤72 chars**, trailer `AI-Used: [claude]` (never Co-Authored-By).
- Work on branch `engram682-coverage-diagnostics`; review before any push; ff-only merge to main. Task 3's engram commit follows the same discipline on an ENGRAM branch named `682-targ-bump`: reviewed in the final whole-branch pass, then ff-only merged and pushed.
- **Gate ordering:** `check-full` includes `check-uncommitted` (clean-tree check), so it can only run green POST-commit. Per task: run the task's own tests pre-commit, COMMIT, then run `targ check-full` as the post-commit gate; any failure → fix and `git commit --amend` (branch is unpushed) → re-run until green.
- Executors re-locate edits by verbatim code anchor, not line numbers.
- **Measured preconditions** (run 2026-07-11 against this tree; re-verify at execution):
  - `internal.OutputContext` (internal/sh/context.go) returns `(buf.String(), err)` with err UNWRAPPED on non-zero exit — the bare "exit status 2".
  - The local `output()` helper (dev/targets.go, anchor `func output(ctx context.Context`) passes stderr through to `os.Stderr` and returns bare err — its callers' diagnostics reach the terminal but the error itself is context-free.
  - Pattern instances in dev/targets.go: `checkCoverageForFail` (anchor `out, err := targ.OutputContext(ctx, "go", "tool", "cover"` — discards `out` on error: THE engram#682 defect); `deadcode` (anchor `out, err := targ.OutputContext(ctx, "deadcode"` — same discard); `checkCoverage` (anchor `out, err := output(ctx, "go", "tool", "cover"` — bare err); `deleteDeadcode` (anchor `out, err := output(ctx, "deadcode"` — bare err); two bare ParseFloat returns (anchors: the `strconv.ParseFloat(percentString, 64)` blocks in `checkCoverage` and `checkCoverageForFail`); `checkCoverage`'s unguarded `lc := linesAndCoverage[0]` (panics on an all-skipped profile) and its missing `percentString == ""` skip (blank line → `ParseFloat("")` → bare error today).
  - Failure fixture verified live: a `coverage.out` containing exactly `garbage not a profile\n` PASSES `mergeCoverageBlocks` (mode line kept verbatim, unparseable blocks skipped) and then `go tool cover -func=coverage.out` exits 2 with stderr `cover: bad mode line: garbage not a profile`. This is the deterministic RED fixture.
  - `coverage.out` is git-ignored in both targ and engram.
  - engram pins targ at v0.0.0-20260402141037-105fb05f62e1 == current main HEAD (105fb05), so engram's gates run exactly this code; the fix reaches engram only via a pushed targ commit + go.mod bump (Task 3).
  - **Targ's own `check-full` baseline is RED on the pristine branch** (measured 2026-07-11, re-verified by Gate A): pre-existing lint findings at `internal/core/command.go:337`, `internal/flags/flags.go:158`, `internal/runner/runner.go:2467`, `:3336` (goconst/govet/modernize classes; blame dates Jan–Feb 2026). The no-exceptions rule (targ CLAUDE.md + Joe's global) requires clearing them: Task 0 below. The live `targ lint-full` run at execution time governs the exact list and count — the locations above are the planning-time snapshot.
  - Empty-profile fixture branch PINNED (measured by Gate A against the real pre-fix code): `go tool cover -func` on `mode: set\n` exits 0 with only a `total:` line; the local `output()` helper trims the trailing newline, so pre-fix `checkCoverage` PANICS (`index out of range [0] with length 0`) — not a bare ParseFloat error. Post-fix, the `len(linesAndCoverage) == 0` guard fires. Task 2's test asserts exactly that path.

---

### Task 0: Clear the pre-existing lint debt blocking green gates

**Files:**
- Modify: `internal/core/command.go`, `internal/flags/flags.go`, `internal/runner/runner.go` (per the live `targ lint-full` enumeration)

**Interfaces:** none — lint-only fixes (named constants for goconst, the govet and modernize findings as the linters direct). No behavior change; no lint suppressions (fix the code, never add nolint overrides — surface to Joe if any finding resists a clean fix).

- [ ] **Step 1:** `targ lint-full` → record the full finding list (expected: the snapshot locations above; the live run governs the exact list and count — Gate A's probe saw a revive cascade appear and clear during fixing).
- [ ] **Step 2:** Fix each finding minimally. RED analogue: the lint findings ARE the failing checks; GREEN = `targ lint-full` clean. Probe-measured tips: put the flags.go goconst const in its own commented declaration (inserting it above `type FlagMode` displaces its doc comment and trips revive `exported`); run `targ reorder-decls` BEFORE committing (both goconst fixes trigger reorder churn — saves an amend cycle).
- [ ] **Step 3:** Commit — subject: `chore(lint): clear pre-existing lint debt blocking green gates` (62 bytes, measured) + trailer. Then `targ check-full` post-commit → every check green (this baseline makes Tasks 1–2's gates achievable). Fix+amend until green.

---

### Task 1: `checkCoverageForFail` prints the cover diagnostic (the engram#682 fix)

**Files:**
- Modify: `dev/targets.go` (checkCoverageForFail + new helper)
- Create: `dev/coverage_failure_test.go` (build tag `targ`)

**Interfaces:**
- Produces: `func commandFailure(command, out string, err error) error` — unexported, used by Task 2's sites too.

- [ ] **Step 1: Write the failing tests** in `dev/coverage_failure_test.go` (header copied from `dev/ansi_lint_test.go` — build tag + package; imports needed beyond it: `context`, `errors`, `os`; gomega `NewWithT`; alphabetical insertion — `targ reorder-decls` auto-fixes test ordering too, measured):

```go
func TestCheckCoverageForFail_CorruptProfileNamesCause(t *testing.T) {
	// t.Chdir forbids t.Parallel.
	g := NewWithT(t)

	t.Chdir(t.TempDir())
	writeErr := os.WriteFile("coverage.out", []byte("garbage not a profile\n"), 0o600)
	g.Expect(writeErr).NotTo(HaveOccurred())

	err := checkCoverageForFail(context.Background(), CoverageCheckArgs{})

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("go tool cover -func=coverage.out"),
		"the failed command must be named")
	g.Expect(err.Error()).To(ContainSubstring("bad mode line"),
		"the tool's own diagnostic must be included")
}

func TestCommandFailure_IncludesCommandAndOutput(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	err := commandFailure("go tool cover -func=coverage.out", "cover: bad mode line: junk\n",
		errors.New("exit status 2"))

	g.Expect(err.Error()).To(ContainSubstring("go tool cover -func=coverage.out"))
	g.Expect(err.Error()).To(ContainSubstring("exit status 2"))
	g.Expect(err.Error()).To(ContainSubstring("cover: bad mode line: junk"))
}

func TestCommandFailure_EmptyOutputOmitsBlock(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	err := commandFailure("deadcode -test ./...", "   \n", errors.New("exit status 1"))

	g.Expect(err.Error()).To(Equal("deadcode -test ./...: exit status 1"))
}
```

- [ ] **Step 2: RED stage 1.** `go test -tags targ -run 'TestCheckCoverageForFail_CorruptProfileNamesCause|TestCommandFailure' ./dev/` → expect `FAIL [build failed]` (`commandFailure` undefined) — a compile error blocks the whole package, so this is the only observable RED in this run (a valid RED per targ CLAUDE.md). Record it.

- [ ] **Step 3a: Add the helper ONLY** (alphabetical position, near other `c` funcs):

```go
// commandFailure wraps a failed command's error with the command itself and
// its captured combined output, so gate failures print their reason instead
// of a bare exit status.
func commandFailure(command, out string, err error) error {
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return fmt.Errorf("%s: %w", command, err)
	}

	return fmt.Errorf("%s: %w\n%s", command, err, trimmed)
}
```

- [ ] **Step 3b: RED stage 2.** Re-run the same test command → the two `TestCommandFailure_*` tests now PASS; `TestCheckCoverageForFail_CorruptProfileNamesCause` FAILS on the `ContainSubstring("go tool cover")` assertion (today's error is bare `exit status 2`). Record it — this is the assertion-level RED for the ask's exact defect.

- [ ] **Step 3c: Edit `checkCoverageForFail`.** The anchor block

```go
	out, err := targ.OutputContext(ctx, "go", "tool", "cover", "-func=coverage.out")
	if err != nil {
		return err
	}
```

  becomes

```go
	out, err := targ.OutputContext(ctx, "go", "tool", "cover", "-func=coverage.out")
	if err != nil {
		return commandFailure("go tool cover -func=coverage.out", out, err)
	}
```

  Also wrap `checkCoverageForFail`'s bare ParseFloat return (anchor: the `strconv.ParseFloat(percentString, 64)` block inside it): `return fmt.Errorf("parsing coverage percent from %q: %w", line, err)`.

- [ ] **Step 4: Run GREEN.** Same `go test -tags targ ...` command → all three PASS.
- [ ] **Step 5: Pre-commit checks + commit.** `targ reorder-decls`; `git diff --stat` → exactly the two named files. Commit. Subject: `fix(dev): coverage gate failures print the cover diagnostic` (59 bytes, `wc -c`-measured). Body: names the discard-on-error defect, the OutputContext bare-err behavior, and `Fixes reported as toejough/engram#682.` Trailer `AI-Used: [claude]`.
- [ ] **Step 6: Post-commit gate.** `targ check-full` → every check green (check-uncommitted requires the clean tree, hence post-commit). Failure → fix, `git commit --amend`, re-run.

---

### Task 2: Fix-all-instances sweep — checkCoverage, deadcode, deleteDeadcode

**Files:**
- Modify: `dev/targets.go`
- Modify: `dev/coverage_failure_test.go` (one new test)

**Interfaces:** consumes `commandFailure` from Task 1.

- [ ] **Step 1: Write the failing test** — `checkCoverage`'s all-skipped-profile panic becomes an error (alphabetical insertion):

```go
func TestCheckCoverage_EmptyProfileErrorsInsteadOfPanicking(t *testing.T) {
	// t.Chdir forbids t.Parallel.
	g := NewWithT(t)

	t.Chdir(t.TempDir())
	profile := "mode: set\n"
	writeErr := os.WriteFile("coverage.out", []byte(profile), 0o600)
	g.Expect(writeErr).NotTo(HaveOccurred())

	var err error

	g.Expect(func() { err = checkCoverage(context.Background(), CoverageCheckArgs{}) }).
		NotTo(Panic())
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("no per-function coverage lines"))
}
```

  Fixture branch PINNED (Gate A measured it against the real pre-fix code — see Global Constraints): the tool exits 0 with only a `total:` line, `output()` trims the trailing newline, and pre-fix `checkCoverage` PANICS with `index out of range [0] with length 0`; post-fix the `len(linesAndCoverage) == 0` guard fires. The test above asserts exactly that path — no fixture adjustment needed.

  SCOPE LABEL: this guard is **opportunistic hardening, disclosed** — an unguarded-index defect class, distinct from the discard-on-error pattern the fix-all-instances rule covers. It rides along because a panic is the worst possible "failure reason" a gate can print; it is not ask-mandated work.

- [ ] **Step 2: RED.** `go test -tags targ -run TestCheckCoverage_EmptyProfile ./dev/` → panics (`index out of range [0] with length 0`, measured) today. Record.
- [ ] **Step 3: Implement the sweep.**
  - `checkCoverage` (anchor `out, err := output(ctx, "go", "tool", "cover", "-func=coverage.out")`): `if err != nil { return fmt.Errorf("go tool cover -func=coverage.out: %w", err) }` (stderr already streamed by `output()`; no captured output to attach).
  - `checkCoverage` loop: add the missing `if percentString == "" { continue }` skip (mirrors checkCoverageForFail) BEFORE ParseFloat; wrap its ParseFloat return like Task 1's.
  - `checkCoverage` (anchor `lc := linesAndCoverage[0]`): guard first — `if len(linesAndCoverage) == 0 { return errors.New("no per-function coverage lines found in coverage.out") }`.
  - `deadcode` (anchor `out, err := targ.OutputContext(ctx, "deadcode", "-test", "./...")`): `return commandFailure("deadcode -test ./...", out, err)`.
  - `deleteDeadcode` (anchor `out, err := output(ctx, "deadcode", "-test", "./...")`): `return fmt.Errorf("deadcode -test ./...: %w", err)`.
- [ ] **Step 4: GREEN** (same test run), then `targ reorder-decls`, diff-scope check (two files), COMMIT. Subject: `fix(dev): wrap remaining bare errors in coverage and deadcode gates` (67 bytes, measured). Trailer `AI-Used: [claude]`.
- [ ] **Step 5: Post-commit gate.** `targ check-full` → green; failure → fix, `git commit --amend`, re-run.

---

### Task 3: Deliver to engram — pin bump + consumer verification

**Files (ENGRAM repo, /Users/joe/repos/personal/engram):**
- Modify: `go.mod`, `go.sum`

**Preconditions:** Tasks 1–2 merged to targ main (ff-only, post-review) and PUSHED to github (the module proxy/VCS fetch needs the public commit).

- [ ] **Step 0: Branch (engram).** `cd /Users/joe/repos/personal/engram && git checkout -b 682-targ-bump` (from main).
- [ ] **Step 1: Resolve the commit hash mechanically and bump.** `TARG_HEAD=$(git -C /Users/joe/repos/personal/targ rev-parse origin/main)` — run AFTER the targ merge+push, and verify it equals the local merge result (`git -C /Users/joe/repos/personal/targ rev-parse main`); then in engram: `go get github.com/toejough/targ@"$TARG_HEAD"` and `go mod tidy`.
- [ ] **Step 2: Consumer gate.** `targ check-full` in engram → all 8 PASS (the bumped dev module compiles and gates run green).
- [ ] **Step 3: Delivery + error-path evidence** (AMENDED post-final-review: the original corrupt-coverage.out smoke is unreachable from the consumer — `CheckCoverageForFail.Deps(TestForFail)` regenerates coverage.out before the check, measured live by the final reviewer, and targ has no skip-deps mode — DepMode is Serial/Parallel/Mixed only). The honest evidence chain:
  a. **Delivery proof:** `go list -m github.com/toejough/targ` in engram → the bumped pseudo-version; verify its commit contains the fix: `git -C /Users/joe/repos/personal/targ merge-base --is-ancestor 964cbeb <bumped-commit>` → exit 0.
  b. **Consumer-runs-it proof:** engram `targ check-full` → all 8 PASS (Step 2 — the bumped dev module is what compiles and executes the gate).
  c. **Error-path proof:** targ's own test suite at the pinned commit — `TestCheckCoverageForFail_CorruptProfileNamesCause` drives the real function + real `go tool cover` against the garbage fixture and asserts the named command + `bad mode line`; re-verified independently by the Task-1 reviewer and the final whole-branch reviewer, and enforced by targ's check-full at every future commit.
  The #682 closing comment cites this chain, not a consumer-side failure demo.
- [ ] **Step 4: Commit (engram).** Subject: `chore(deps): bump targ - coverage gate prints failure reasons` (61 bytes, measured; plain hyphen — an em-dash is 3 bytes to git). Body references #682 + the targ commits. Trailer `AI-Used: [claude]`. Diff-scope: go.mod + go.sum only. The `682-targ-bump` branch is reviewed in the final whole-branch pass, then ff-only merged to engram main and pushed — same review-before-push discipline as targ's branch.

---

## Controller close-out

- Trap gate: **N/A for this change** — dev-tooling error formatting touches no recall/vault/binary behavior the trap gate measures; the gates here are targ's check-full (both repos) plus the Task-3 failure-path smoke. (Gate A reviewers: challenge this if you disagree.)
- Final whole-branch review over the targ branch before its merge/push; engram's one-commit bump reviewed in the same pass.
- Close engram#682 with: the targ commits, the engram bump commit, and the Task-3 smoke evidence (before: bare `exit status 2`; after: named command + `bad mode line` diagnostic).

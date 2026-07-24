# Issue #32: `--dep-mode` for Deps-Only Targets Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make deps-only targets (`targ.Targ().Name("all").Deps(...)`) honor the `--dep-mode` CLI override exactly as function-bearing targets do.

**Architecture:** `runDeps` has three callers; `runNodeDeps` (`internal/core/command.go:1945`) is the only one that applies the `--dep-mode` flatten (with the CollectAllErrors union rule from #26). Deps-only targets currently take `executeDepsOnlyTarget` (`command.go:889`), which calls `node.Target.runDeps(ctx)` raw — bypassing `opts.Overrides`. The fix replaces that raw call (and its `len(depGroups) > 0` guard) by delegating to `runTargetWithOverrides(ctx, node, nodeInstance(node), opts)` — the shared override-aware runner, which routes through `runNodeDeps` and returns before function execution when `node.Func` is invalid (`command.go:2027-2030`; `nodeInstance` yields a safe zero value for type-less nodes) — while **keeping the `opts.HelpOnly` early-return**, which the shared runner cannot see; the function-bearing path short-circuits HelpOnly upstream, and `TestProperty_Execution/DepsOnlyTargetHelpShowsDeps` pins that deps must not run under `--help`. *(Amended after Gate B: the original plan called `runNodeDeps` directly; the design-fit review showed delegation removes a hand-rolled duplicate of `runTargetWithOverrides`'s first three lines.)*

**Tech Stack:** Go 1.25.5, gomega assertions, `targ` build system for all verification (never bare `go test`).

## Global Constraints

- **No flaky tests**: no wall-clock timing, no load-dependent scheduling assumptions (repo Testing Rules; commit d3ff091 precedent).
- **No `errors.As` through `targ.Execute()`**: the run_env.go CLI dispatch collapses errors to bare `ExitError{Code: 1}`; assert on counters/order logs/`result.Output` instead (vault note 411).
- **Parallel subtests own their data**: no shared mutable state across subtests (repo CLAUDE.md).
- **Declaration ordering**: go-reorder governs top-level decls; new subtests sit adjacent to the sibling dep-mode subtests inside `TestProperty_Overrides` (subtest order is not lint-governed — existing siblings are non-alphabetical).
- **`targ check-full` full-green is sequenced AFTER the final commit** — its `check-uncommitted` leg fails by design on a dirty tree (vault note 395). Pre-commit, expect every leg green EXCEPT `check-uncommitted`.
- **TDD red step is mandatory**: run the new test against unfixed code and observe the failure before implementing.

## Design notes

- **Determinism trade (stated honestly):** post-fix, `--dep-mode serial` execution order is deterministic by construction (serial declaration order), so the checked-in test satisfies the no-flaky rule. Under a *regression* (deps run parallel), a 3-dep order assertion fails with probability 5/6 per run rather than 1 — perfect two-sided determinism is impossible because in serial mode an earlier dep can never block on a later dep's signal without deadlocking. This is the same trade the existing `DepModeOverrideFlattensMixedGroups` test (`test/overrides_properties_test.go:530-579`) makes for the function-bearing path; three deps are used (matching that test's shape) to keep accidental-pass probability low. The RED step verifies the failure empirically.
- **On the issue's named style precedent:** the issue says "same style as `TestProperty_Overrides/DepModeSerialPreservesCollectAllErrors`". That test pins CollectAllErrors survival via atomic *counters*, which cannot express an ordering assertion — so "style" is followed at the file/convention level (public `targ` API, `targ.Execute` with `--dep-mode` before the command name, no `errors.As`, `t.Parallel()` with subtest-local state), while the ordering mechanics follow the file's own order-log precedent, `DepModeOverrideFlattensMixedGroups`. This divergence is deliberate, not an oversight.
- **Property-rigor note:** the generalized flatten property (any group shape → one group, mode overridden, CollectAllErrors unioned) is already pinned at the unit layer by `TestRunNodeDepsFlattenPreservesCollectAll` (`internal/core/command_internal_test.go:123-160`). This fix routes the deps-only path through that already-pinned layer; the new test pins the routing end-to-end in the established example style. No new rapid property is warranted (YAGNI).
- **Scope:** exactly what the issue asks — the routing fix plus one ordering regression test, and the one-line spec sync below. No README wording changes (see disposition list).
- **Scope disclosure (Task 2):** the issue's "Affected files" lists only `internal/core/command.go` and `test/overrides_properties_test.go`. Task 2's `docs/specs/tests.md` line is an explicit add-on beyond that list, mandated by the repo's spec-sync convention (see commit `183566b docs(specs): MultiError is not parallel-only in IMPL-8 and T-6`) and the workflow's Document step — disclosed here rather than silently bundled.

## Doc-surface disposition list (enumeration grep: `dep-mode|deps-only|DepMode` over README.md, docs/, specs/)

| File | Disposition | Reason |
| --- | --- | --- |
| `README.md:352-356` (deps-only targets), `:712` (flag row), `:716` (flatten + CollectAllErrors note) | keep | The flatten note describes `--dep-mode` unconditionally; the fix makes it true for deps-only targets too (issue: "no doc change needed"). |
| `docs/specs/tests.md:49` (T-3 property list) | update | Add one property line for the new deps-only override test; `TestProperty_Overrides` is already in T-3's **Tests:** roster, so no other edit. |
| `docs/specs/implementation.md:122` (`--dep-mode` caveat) | keep | Describes the override unconditionally — true post-fix. |
| `docs/specs/implementation.md:32,33,40,121` (IMPL descriptions naming `DepMode`/`--dep-mode`) | keep | Builder API, key types, and flag-registry descriptions unchanged by the fix. |
| `docs/specs/requirements.md:21,161,175` | keep | Runtime-override and deps-only descriptions are mode-agnostic — true post-fix. |
| `docs/specs/architecture.md:21` | keep | `RuntimeOverrides` extraction description unaffected. |
| `docs/specs/architecture.md:7` (Target data model mentions `DepMode`) | keep | Data model unchanged. |
| `docs/specs/tests.md:34` (T-3 Given/When/Then) | keep | Already states overrides incl. `--dep-mode` "are applied" — true post-fix (more true, in fact). |
| `README.md:207,340,347,385,536,540,545` (Deps builder row + examples) | keep | Builder API and examples unchanged; deps-only examples remain accurate. |
| `docs/plans/2026-07-23-coverage-leg-timeout.md`, `docs/plans/2026-02-13-dep-group-chaining*.md`, other `docs/plans/*` | N/A | Historical planning records; the repo keeps them as-written (not living docs). |
| `docs/archive/*` | N/A | Archived. |
| `specs/001-parallel-output/` | N/A | No dep-mode references (grep: zero hits). |

---

### Task 1: Regression test + fix — route `executeDepsOnlyTarget` through `runNodeDeps`

**Files:**
- Modify: `internal/core/command.go:889-909` (`executeDepsOnlyTarget`)
- Test: `test/overrides_properties_test.go` (new subtest inside `TestProperty_Overrides`, adjacent to `DepModeSerialPreservesCollectAllErrors` at line 581)

**Interfaces:**
- Consumes: `runNodeDeps(ctx context.Context, node *commandNode, overrides RuntimeOverrides) error` (`command.go:1945`, unchanged); `opts.Overrides` field of `RunOptions`.
- Produces: no signature changes anywhere; `executeDepsOnlyTarget` keeps its exact signature `(ctx context.Context, args []string, node *commandNode, opts RunOptions) ([]string, error)`.

- [ ] **Step 1: Write the failing test**

Insert into `test/overrides_properties_test.go` directly ABOVE the `t.Run("DepModeSerialPreservesCollectAllErrors", ...)` subtest (line 581), inside `TestProperty_Overrides`:

```go
	t.Run("DepModeSerialAppliesToDepsOnlyTarget", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		order := make([]string, 0, 3)

		var mu sync.Mutex

		record := func(name string) {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		}

		depA := targ.Targ(func() { record("a") }).Name("a")
		depB := targ.Targ(func() { record("b") }).Name("b")
		depC := targ.Targ(func() { record("c") }).Name("c")
		all := targ.Targ().Name("all").Deps(depA, depB, depC, targ.DepModeParallel)

		_, err := targ.Execute(
			[]string{"app", "--dep-mode", "serial", "all"},
			all, depA, depB, depC, dummy(),
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(order).To(Equal([]string{"a", "b", "c"}),
			"--dep-mode serial on a deps-only target must run deps serially in declaration order")
	})
```

Notes for the implementer (verified against `b7a3f6f`): `sync` is imported at `test/overrides_properties_test.go:7`; `dummy` is a local closure inside `TestProperty_Overrides` at line 23 (`func() *targ.Target { return targ.Targ(func() {}).Name("_dummy") }`) — copy its usage from the sibling subtest at line 581-607. The mutex-guarded order-log pattern precedent is `DepModeOverrideFlattensMixedGroups` at line 530-579. Do not share `order`/`mu` with any other subtest.

- [ ] **Step 2: Run the test to verify it fails (RED)**

Run: `targ test 2>&1 | grep -A 5 "DepModeSerialAppliesToDepsOnlyTarget"` (or run the full `targ test` and inspect).
Expected: FAIL — order is a permutation other than `["a", "b", "c"]` because the override is ignored and deps run in parallel. The pre-fix failure is probabilistic (~5/6 per run; parallel scheduling can accidentally produce declaration order ~1/6 of the time): if a run passes, re-run until the order-mismatch failure is observed — RED is satisfied by that observed failure. Post-fix the test is deterministic by construction (serial declaration order), so no flakiness survives into the checked-in suite.

- [ ] **Step 3: Apply the minimal fix (GREEN)**

In `internal/core/command.go`, replace the body of `executeDepsOnlyTarget` (lines 889-909):

```go
// executeDepsOnlyTarget handles targets that have no function but run dependencies.
func executeDepsOnlyTarget(
	ctx context.Context,
	args []string,
	node *commandNode,
	opts RunOptions,
) ([]string, error) {
	// Skip execution in help-only mode
	if opts.HelpOnly {
		return args, nil
	}

	// Delegate to the shared override-aware runner; with no function to
	// execute it runs only the dependencies (--dep-mode applies there)
	err := runTargetWithOverrides(ctx, node, nodeInstance(node), opts)
	if err != nil {
		return nil, err
	}

	return args, nil
}
```

The `len(node.Target.depGroups) > 0` guard is deleted deliberately: `runNodeDeps` (called first inside `runTargetWithOverrides`) performs its own `node.Target == nil || len(node.Target.depGroups) == 0` guard (`command.go:1946`). The `opts.HelpOnly` early-return MUST remain (see Architecture).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `targ test`
Expected: PASS — the new subtest and the whole unit suite green, including `DepsOnlyTargetHelpShowsDeps` (pins the retained HelpOnly guard), `DepsOnlyTargetRunsDependencies`, `DepsOnlyTargetErrorPropagates`, `DepsOnlyTargetNoDepsSucceeds` (`test/execution_properties_test.go:1308-1368`), and the two `DepMode*PreservesCollectAllErrors` siblings.

- [ ] **Step 5: Refactor check + Gate B**

The change is a body swap; the deduplication IS the change (both CLI dep-running paths now share `runNodeDeps`), and nothing new is worth extracting (YAGNI). Then dispatch Gate B: a fresh-context design-fit reviewer over the full diff, charged with DRY/SRP/YAGNI and "does the result read as written-from-the-start". The unit is done only when Gate B closes with all findings resolved.

- [ ] **Step 6: Pre-commit checks, then commit**

Run: `targ check` (runs all checks & auto-fixes — formatting, declaration reordering) then `targ check-full`.
Expected pre-commit: every leg PASS except `check-uncommitted` (dirty tree by design — vault note 395).

```bash
git add internal/core/command.go test/overrides_properties_test.go
git commit -m "$(cat <<'EOF'
fix(core): honor --dep-mode override for deps-only targets

executeDepsOnlyTarget called Target.runDeps directly, bypassing the
override layer, so --dep-mode parsed successfully and then silently
did nothing for deps-only targets. Delegate to runTargetWithOverrides,
the shared override-aware runner, which routes deps through runNodeDeps
(guarding empty depGroups itself and preserving CollectAllErrors
through the flatten) and no-ops the absent function; keep the HelpOnly
early-return, which the shared runner cannot see.

Closes #32

AI-Used: [claude]
EOF
)"
```

(Exact commit text passes Gate D before the commit runs; trailer per user convention.)

### Task 2: Spec sync — T-3 property list

**Files:**
- Modify: `docs/specs/tests.md:49` (T-3 property list)

**Interfaces:**
- Consumes: nothing from Task 1 besides the merged test name.
- Produces: nothing consumed later.

- [ ] **Step 1: Add the property line**

After line 49 (verified against `b7a3f6f`: line 49 reads `- Property: --dep-mode flatten preserves CollectAllErrors (serial and parallel)`), insert:

```markdown
- Property: --dep-mode applies to deps-only targets (serial declaration order)
```

T-3's **Tests:** roster (line 51) already lists `TestProperty_Overrides`; no other spec edit (see disposition list).

- [ ] **Step 2: Gate C over the touched doc, then commit**

```bash
git add docs/specs/tests.md
git commit -m "$(cat <<'EOF'
docs(specs): deps-only --dep-mode property in T-3

AI-Used: [claude]
EOF
)"
```

### Task 3: Full-green verification + close the issue

- [ ] **Step 1: Post-commit full suite**

Run: `targ check-full`
Expected: ALL legs PASS including `check-uncommitted` (tree now clean). No pre-existing-failure exemptions — any failure gets fixed before proceeding.

- [ ] **Step 2: Close issue #32**

```bash
gh issue close 32 --repo toejough/targ --comment "Fixed in <commit-sha>: executeDepsOnlyTarget now routes through runNodeDeps(ctx, node, opts.Overrides), so --dep-mode serial|parallel flattens dep groups (CollectAllErrors preserved via the union rule) for deps-only targets exactly as for function-bearing ones. HelpOnly early-return retained. Regression test: TestProperty_Overrides/DepModeSerialAppliesToDepsOnlyTarget (3 parallel-wired deps, serial declaration-order assertion). Spec sync: T-3 property list."
```

(Comment text passes Gate D first; substitute the real SHA.)

- [ ] **Step 3: Delete planning artifacts?**

This plan file is a `docs/plans/` record — the repo keeps those (see disposition list); do NOT delete. No temp/scratch artifacts are expected; remove any that appear.

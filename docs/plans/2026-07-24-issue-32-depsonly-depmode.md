# Issue #32: `--dep-mode` for Deps-Only Targets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make deps-only targets (`targ.Targ().Name("all").Deps(...)`) honor the `--dep-mode` CLI override exactly as function-bearing targets do.

**Architecture:** `runDeps` has three callers; `runNodeDeps` (`internal/core/command.go:1945`) is the only one that applies the `--dep-mode` flatten (with the CollectAllErrors union rule from #26). Deps-only targets currently take `executeDepsOnlyTarget` (`command.go:889`), which calls `node.Target.runDeps(ctx)` raw — bypassing `opts.Overrides`. The fix replaces that raw call (and its `len(depGroups) > 0` guard, which `runNodeDeps` performs itself) with `runNodeDeps(ctx, node, opts.Overrides)`, while **keeping the `opts.HelpOnly` early-return** — `runNodeDeps` takes `RuntimeOverrides`, not `RunOptions`, so it cannot see HelpOnly; the function-bearing path short-circuits HelpOnly upstream, and `TestProperty_Execution/DepsOnlyTargetHelpShowsDeps` pins that deps must not run under `--help`.

**Tech Stack:** Go 1.25.5, gomega assertions, `targ` build system for all verification (never bare `go test`).

## Global Constraints

- **No flaky tests**: no wall-clock timing, no load-dependent scheduling assumptions (repo Testing Rules; commit d3ff091 precedent).
- **No `errors.As` through `targ.Execute()`**: the run_env.go CLI dispatch collapses errors to bare `ExitError{Code: 1}`; assert on counters/order logs/`result.Output` instead (vault note 411).
- **Parallel subtests own their data**: no shared mutable state across subtests (repo CLAUDE.md).
- **Declaration ordering**: go-reorder governs top-level decls; new subtests sit adjacent to the sibling dep-mode subtests inside `TestProperty_Overrides` (subtest order is not lint-governed — existing siblings are non-alphabetical).
- **`targ check-full` full-green is sequenced AFTER the final commit** — its `check-uncommitted` leg fails by design on a dirty tree (vault note 395). Pre-commit, expect every leg green EXCEPT `check-uncommitted`.
- **TDD red step is mandatory**: run the new test against unfixed code and observe the failure before implementing.

## Design notes

- **Determinism trade (stated honestly):** post-fix, `--dep-mode serial` execution order is deterministic by construction (serial declaration order), so the checked-in test satisfies the no-flaky rule. Under a *regression* (deps run parallel), a 3-dep order assertion fails with probability 5/6 per run rather than 1 — perfect two-sided determinism is impossible because in serial mode an earlier dep can never block on a later dep's signal without deadlocking. This is the same trade the existing `DepModeSerialExecution` test (`test/overrides_properties_test.go:534-577`) makes for the function-bearing path; three deps are used (matching that test's shape) to keep accidental-pass probability low. The RED step verifies the failure empirically.
- **Property-rigor note:** the generalized flatten property (any group shape → one group, mode overridden, CollectAllErrors unioned) is already pinned at the unit layer by `TestRunNodeDepsFlattenPreservesCollectAll` (`internal/core/command_internal_test.go:123-160`). This fix routes the deps-only path through that already-pinned layer; the new test pins the routing end-to-end in the established example style. No new rapid property is warranted (YAGNI).
- **Scope:** exactly what the issue asks — the routing fix plus one ordering regression test, and the one-line spec sync below. No README wording changes (see disposition list).

## Doc-surface disposition list (enumeration grep: `dep-mode|deps-only|DepMode` over README.md, docs/, specs/)

| File | Disposition | Reason |
| --- | --- | --- |
| `README.md:352-356` (deps-only targets), `:712` (flag row), `:716` (flatten + CollectAllErrors note) | keep | The flatten note describes `--dep-mode` unconditionally; the fix makes it true for deps-only targets too (issue: "no doc change needed"). |
| `docs/specs/tests.md:49` (T-3 property list) | update | Add one property line for the new deps-only override test; `TestProperty_Overrides` is already in T-3's **Tests:** roster, so no other edit. |
| `docs/specs/implementation.md:122` (`--dep-mode` caveat) | keep | Describes the override unconditionally — true post-fix. |
| `docs/specs/requirements.md:21,161,175` | keep | Runtime-override and deps-only descriptions are mode-agnostic — true post-fix. |
| `docs/specs/architecture.md:21` | keep | `RuntimeOverrides` extraction description unaffected. |
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

		record := func(name string) func() {
			return func() {
				mu.Lock()
				order = append(order, name)
				mu.Unlock()
			}
		}

		depA := targ.Targ(record("a")).Name("a")
		depB := targ.Targ(record("b")).Name("b")
		depC := targ.Targ(record("c")).Name("c")
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

Notes for the implementer: `sync` is already imported in this file (the mutex-guarded order-log pattern exists at line 534-577); `dummy()` is the file's existing helper for a filler target — copy its usage from the sibling subtest at line 581-607. Do not share `order`/`mu` with any other subtest.

- [ ] **Step 2: Run the test to verify it fails (RED)**

Run: `targ test 2>&1 | grep -A 5 "DepModeSerialAppliesToDepsOnlyTarget"` (or run the full `targ test` and inspect).
Expected: FAIL — order is a permutation other than `["a", "b", "c"]` (deps still run parallel; the override is ignored on the deps-only path). Under a regression the assertion passes by accident with probability ~1/6 per run; if it happens to PASS, re-run to observe the failure — the red step is satisfied by an observed deterministic-mismatch failure, and flakiness of the failing state is expected pre-fix (it is exactly the bug).

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

	// Run dependencies through the override layer so --dep-mode applies
	err := runNodeDeps(ctx, node, opts.Overrides)
	if err != nil {
		return nil, err
	}

	return args, nil
}
```

The `len(node.Target.depGroups) > 0` guard is deleted deliberately: `runNodeDeps` performs its own `node.Target == nil || len(node.Target.depGroups) == 0` guard (`command.go:1946`). The `opts.HelpOnly` early-return MUST remain (see Architecture).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `targ test`
Expected: PASS — the new subtest and the whole unit suite green, including `DepsOnlyTargetHelpShowsDeps` (pins the retained HelpOnly guard), `DepsOnlyTargetRunsDependencies`, `DepsOnlyTargetErrorPropagates`, `DepsOnlyTargetNoDepsSucceeds` (`test/execution_properties_test.go:1308-1368`), and the two `DepMode*PreservesCollectAllErrors` siblings.

- [ ] **Step 5: Refactor check + Gate B**

The change is a 9-line body swap; confirm no further refactor is warranted (DRY: the two dep-running paths now share `runNodeDeps` — that IS the deduplication; YAGNI: nothing else). Run Gate B (design-fit reviewer, fresh context) on the diff before declaring the unit done.

- [ ] **Step 6: Pre-commit checks, then commit**

Run: `targ check` (fix pass) then `targ check-full`.
Expected pre-commit: every leg PASS except `check-uncommitted` (dirty tree by design — vault note 395).

```bash
git add internal/core/command.go test/overrides_properties_test.go
git commit -m "fix(core): honor --dep-mode override for deps-only targets

executeDepsOnlyTarget called Target.runDeps directly, bypassing the
runNodeDeps override layer, so --dep-mode parsed successfully and then
silently did nothing for deps-only targets. Route it through
runNodeDeps (which guards empty depGroups itself and preserves
CollectAllErrors through the flatten); keep the HelpOnly early-return,
which runNodeDeps cannot see.

Closes #32

AI-Used: [claude]"
```

(Exact commit text passes Gate D before the commit runs; trailer per user convention.)

### Task 2: Spec sync — T-3 property list

**Files:**
- Modify: `docs/specs/tests.md:49` (T-3 property list)

**Interfaces:**
- Consumes: nothing from Task 1 besides the merged test name.
- Produces: nothing consumed later.

- [ ] **Step 1: Add the property line**

After line 49 (`- Property: --dep-mode flatten preserves CollectAllErrors (serial and parallel)`), insert:

```markdown
- Property: --dep-mode applies to deps-only targets (serial declaration order)
```

T-3's **Tests:** roster already lists `TestProperty_Overrides`; no other spec edit (see disposition list).

- [ ] **Step 2: Gate C over the touched doc, then commit**

```bash
git add docs/specs/tests.md
git commit -m "docs(specs): deps-only --dep-mode property in T-3

AI-Used: [claude]"
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

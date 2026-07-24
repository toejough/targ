# Issue #26 Option B: Bounded Parallel Dep Fan-Out + CollectAllErrors Fixes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cap the concurrency of parallel dependency groups at `min(n, max(2, GOMAXPROCS/2))` so `check-full`'s eight-leg fan-out no longer stacks unbounded load, and make `CollectAllErrors` survive the `--dep-mode` flatten and work for serial groups.

**Architecture:** A pure `parallelCap(n, procs)` helper computes the bound; both parallel group runners (`runGroupParallelAll`, `runGroupParallel` in `internal/core/target.go`) take an explicit `capN` parameter and enforce it with a channel semaphore (no new dependencies — `golang.org/x/sync` is depguard-blocked). A new `runGroupSerialAll` runner gives serial groups real collect-all semantics, dispatched from `runDeps`. The `--dep-mode` flatten in `runNodeDeps` (`internal/core/command.go`) preserves `collectAll` when any original group had it.

**Tech Stack:** Go 1.25.5 (builtin `min`/`max`), gomega assertions, existing channel idioms. No new module dependencies.

## Global Constraints

- Go 1.25.5; `targ` build system for all checks (`targ check-full` is the done-gate; never bare `go test` as final validation).
- No new package-level mutable state — cap is computed per `runDeps` call and passed as a parameter.
- Channel-based semaphore, NOT `golang.org/x/sync/semaphore` (depguard allowlist in `dev/golangci-lint.toml` does not include it; no blanket lint-config edits).
- Declaration ordering (go-reorder): funcs alphabetical within their group. `parallelCap` slots between `classifyCollectAllResult` (target.go:752) and `parallelShellEnv` (:766); `runGroupSerialAll` slots between `runGroupSerial` (:1017) and `runShellCommand` (:1031). Test funcs alphabetical within test files.
- Per-function coverage ≥80% — every new function (`parallelCap`, `runGroupSerialAll`, and the modified runners) needs test coverage individually; lowest function gates `check-full`.
- Tests: `t.Parallel()` in every parent and subtest (paralleltest lint); no shared mutable state across parallel subtests; NO timing/load-dependent assertions (upper-bound concurrency assertions and pre-canceled contexts only — no sleeps as synchronization).
- TDD red step mandatory: run each new test before implementing; confirm the expected failure.
- Commit trailer: `AI-Used: [claude]` (NOT Co-Authored-By).
- Per-task commits are user-confirmed (Joe, 2026-07-24, Gate A resolution): each task lands as its own atomic conventional commit; no hold-for-go-ahead gate.
- Cyclomatic headroom: cyclop max is 10 (default; no override block). `runDeps` goes from ~7 to ~8 with the new dispatch case — do NOT inline cap computation branches or semaphore logic into `runDeps`; keep them in the helpers. Enforcement is mechanical, not on trust: cyclop runs inside `targ check-full` (Task 8's gate), and Task 6 Step 3 already names the `anyGroupCollectsAll` extraction as the fallback if `runNodeDeps` tips over.
- File:line anchors in this plan were measured at plan time (HEAD 65b1d1c). If an anchor misses at execution time, re-locate with `grep -n "func <name>" <file>` and adjust — do not guess offsets.

## Design Decisions (settled during orientation; do not relitigate in-task)

1. **Cap formula: `min(n, max(2, GOMAXPROCS/2))` — user-decided.** History: Joe initially picked "max(1, numCPU/2), no knob"; the plan's first draft self-substituted a floor of 2 (the four subtests at `test/execution_properties_test.go:298,543,631,674` gate exactly 2 deps and require both to start concurrently, so a cap of 1 breaks the "parallel means concurrent" contract); Gate A ask-alignment correctly flagged that override as a decision belonging to the user. Escalated 2026-07-24: Joe answered "just use max procs", then confirmed the exact formula `min(n, max(2, GOMAXPROCS/2))` — the halving cap sourced from `runtime.GOMAXPROCS(0)` (respects container CPU quotas, unlike `NumCPU`), floor 2 preserving the concurrency contract. No knob (YAGNI, per Joe's pick).
2. **Full CollectAllErrors fix** (Joe's pick): flatten preserves `collectAll` (union across original groups), AND serial groups honor `collectAll` via new `runGroupSerialAll`. This also fixes the pre-existing silent gap where `Deps(a, b, CollectAllErrors)` without `DepModeParallel` was accepted but ignored (dispatch at target.go:502 only honored it for parallel).
3. **Queued-target skip in fail-fast runner:** after acquiring the semaphore, `runGroupParallel` checks `ctx.Err()`; a canceled context short-circuits to a `Cancelled` result without firing `OnStart` or `Run`. Rationale: with a cap, a queued dep could otherwise START fresh work after a sibling already failed — a regression of fail-fast semantics. The guarantee is best-effort (same as today's mid-flight cancellation), tested deterministically via a pre-canceled context.
4. **`runGroupSerialAll` output:** quiet on success (like `runGroupSerial`); on failure prints per-target `Error:` lines as they occur and a `FormatDetailedSummary` block at the end, returns `reportedError{NewMultiError(results)}` (mirrors `runGroupParallelAll`'s failure reporting without the parallel printer machinery, which serial output doesn't need).
5. **Semaphore acquired inside the goroutine** (spawn all, gate execution): keeps the result-collection loop unchanged (all N goroutines send exactly one result) and means `OnStart`/"starting..." only fires when a target actually begins.
6. **Cap clamp:** runners treat `capN < 1` as 1 (defensive; `parallelCap` returns 0 only for n=0, where no goroutines spawn anyway).
7. **Out of scope (pre-existing, unchanged):** `runNodeDeps`'s flatten permanently mutates the registered `*Target`'s `depGroups` (observable on a second execution in-process; command.go:1961). Not this issue.

## Doc-Surface Enumeration (grep run 2026-07-23)

Greps: `grep -rn --include="*.md" -iE "dep-mode|collectallerrors|collect-all|collect all|fail-fast|fail fast|fan-out|fanout|unbounded|semaphore|concurrenc" README.md CLAUDE.md docs/` plus `grep -rn "CollectAllErrors" --include="*.go" targ.go internal/` for GoDoc comment surfaces (Gate A caught that the `*.md`-only grep is structurally blind to doc comments), plus the #28 plan's disposition table as prior art.

| Surface | Disposition | Reason |
|---|---|---|
| `README.md` Dependencies section (~:334-352) | update | Add one sentence documenting the bounded-concurrency default for parallel groups |
| `README.md:380-383` (CollectAllErrors example) | update | "first failure cancels" prose is parallel-specific; note collect-all now also works for serial groups |
| `README.md:710` (`--dep-mode` flag-table row) | keep | Row text ("Override dependency mode") still accurate |
| `README.md:714` (`--dep-mode` note) | update | Remove "ignores any `targ.CollectAllErrors` … reverts to fail-fast" — the flatten now preserves it; keep the flatten + conflict-detection-bypass facts |
| `docs/specs/implementation.md:40` (IMPL-5 purpose) | keep | Flag enumeration unchanged |
| `docs/specs/implementation.md:122` (flags caveat) | update | Rewrite caveat: flatten preserves `CollectAllErrors`; drop "(behavioral fix tracked in #26)" |
| `docs/specs/requirements.md:49` (REQ-7) | update | "Parallel mode with `CollectAllErrors`" → collect-all applies in both modes |
| `docs/specs/requirements.md:56` (REQ-8) | keep | Output prefixing unaffected |
| `docs/specs/requirements.md:175` (DES-5) | update | Add serial collect-all + the concurrency cap to the dependency execution model |
| `docs/specs/architecture.md:7` (ARCH-1 builder) | keep | Builder/DepOption description accurate as-is |
| `docs/specs/architecture.md:21` (ARCH-3 execute) | keep | Override extraction unchanged |
| `docs/specs/architecture.md:42` (ARCH-6 results) | update | "`MultiError` collects all failures from parallel `CollectAllErrors` mode" → drop "parallel" |
| `docs/specs/tests.md:34` (T-3) | update | Add property lines for the cap, serial collect-all, and flatten-preserves-collectAll tests |
| `docs/specs/use-cases.md` (UC-5 parallel execution) | keep | High-level use-case prose; traces to REQ-7/DES-5 which ARE updated; UC-5's own text stays accurate |
| `targ.go:17-19` (`CollectAllErrors` re-export GoDoc) | update | Says "parallel deps" — false once serial groups honor it; Task 5 Step 4 rewrites it (pkg.go.dev/IDE surface, invisible to the `*.md` grep) |
| `internal/core/target.go:50-52` (`CollectAllErrors` GoDoc) | update | Same text as targ.go re-export; Task 5 Step 4 rewrites it |
| `docs/archive/*` (architecture.md, design.md, requirements.md, issues.md) | N/A | Archive convention: no retro-edits |
| `docs/plans/*` (incl. 2026-07-23-coverage-leg-timeout.md, 2026-07-23-issue-28-dep-mode-flag-docs.md, 2026-02-13-dep-group-chaining*.md) | N/A | Historical session records; #28's caveat wording anticipated "if #26 later changes the flatten/drop, the wording gets revisited then" — that revisit is the two `update` rows above, not edits to the plan docs |
| `CLAUDE.md` (repo) | keep | No dep-semantics content |
| GitHub issue #26 body | N/A | Closed with an explanatory comment (Task 9); body not edited |

---

### Task 1: `parallelCap` helper

**Files:**
- Modify: `internal/core/target.go` (insert func between `classifyCollectAllResult` ending ~:764 and `parallelShellEnv` at :766)
- Test: `internal/core/target_internal_test.go` (new file — verified absent at plan time; whitebox `package core`, needed to reach unexported funcs, mirroring the existing `command_internal_test.go` convention)

**Interfaces:**
- Produces: `func parallelCap(n, procs int) int` — later tasks call it from `runDeps`.

- [ ] **Step 1: Write the failing test**

Create `internal/core/target_internal_test.go`:

```go
package core

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestParallelCap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		n     int
		procs int
		want  int
	}{
		{name: "TenProcsEightTargets", n: 8, procs: 10, want: 5},
		{name: "FourProcsFloorsAtTwo", n: 8, procs: 4, want: 2},
		{name: "TwoProcsFloorsAtTwo", n: 8, procs: 2, want: 2},
		{name: "OneProcFloorsAtTwo", n: 8, procs: 1, want: 2},
		{name: "SingleTargetCapsAtOne", n: 1, procs: 10, want: 1},
		{name: "ZeroTargetsCapsAtZero", n: 0, procs: 10, want: 0},
		{name: "ManyProcsCapsAtN", n: 8, procs: 32, want: 8},
		{name: "OddProcsIntegerDivision", n: 3, procs: 7, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(parallelCap(tt.n, tt.procs)).To(Equal(tt.want))
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ -run TestParallelCap`
Expected: FAIL — compile error `undefined: parallelCap`

- [ ] **Step 3: Write minimal implementation**

In `internal/core/target.go`, after `classifyCollectAllResult` and before `parallelShellEnv` (alphabetical position):

```go
// parallelCap bounds how many targets in a parallel dep group run
// concurrently: min(n, max(2, procs/2)). The floor of 2 keeps parallel
// groups observably concurrent even at GOMAXPROCS=1.
func parallelCap(n, procs int) int {
	return min(n, max(2, procs/2))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/ -run TestParallelCap`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/target.go internal/core/target_internal_test.go
git commit -m "feat(core): add parallelCap helper for bounded dep fan-out

AI-Used: [claude]"
```

---

### Task 2: Bound `runGroupParallelAll` with a semaphore

**Files:**
- Modify: `internal/core/target.go:905` (`runGroupParallelAll` signature + body), `:497-516` (`runDeps` call site), imports (add `"runtime"`)
- Test: `internal/core/target_internal_test.go`

**Interfaces:**
- Consumes: `parallelCap` from Task 1.
- Produces: `func runGroupParallelAll(ctx context.Context, targets []*Target, capN int) error` — Task 5's `runDeps` shape relies on this signature.

- [ ] **Step 1: Write the failing test**

Add to `internal/core/target_internal_test.go` (alphabetical: after `TestParallelCap`):

```go
func TestRunGroupParallelAllBoundsConcurrency(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var (
		buf     bytes.Buffer
		mu      sync.Mutex
		cur     atomic.Int32
		maxSeen atomic.Int32
		ran     atomic.Int32
	)

	mk := func(name string) *Target {
		return Targ(func() {
			c := cur.Add(1)

			mu.Lock()
			if c > maxSeen.Load() {
				maxSeen.Store(c)
			}
			mu.Unlock()

			ran.Add(1)
			cur.Add(-1)
		}).Name(name)
	}

	targets := []*Target{mk("t1"), mk("t2"), mk("t3"), mk("t4"), mk("t5")}
	ctx := WithExecInfo(context.Background(), ExecInfo{Output: &buf})

	err := runGroupParallelAll(ctx, targets, 1)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ran.Load()).To(Equal(int32(5)), "all targets must run")
	g.Expect(maxSeen.Load()).To(Equal(int32(1)), "cap 1 must serialize execution")
}
```

(File imports grow to: `bytes`, `context`, `sync`, `sync/atomic`, `testing`, gomega.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ -run TestRunGroupParallelAllBoundsConcurrency`
Expected: FAIL — compile error: too many arguments in call to `runGroupParallelAll`

- [ ] **Step 3: Implement**

In `internal/core/target.go`:

a) Change the signature and add the semaphore (acquire inside the goroutine, before `OnStart`):

```go
func runGroupParallelAll(ctx context.Context, targets []*Target, capN int) error {
	out := outputFromContext(ctx)
	// ... existing maxNameLen / printer / resultCh setup unchanged ...

	sem := make(chan struct{}, max(1, capN))

	for i, dep := range targets {
		name := dep.GetName()
		results[i].Name = name

		go func(idx int, d *Target, targetName string) {
			sem <- struct{}{}
			defer func() { <-sem }()

			// ... existing body: WithExecInfo, OnStart, Run, send result — unchanged ...
		}(i, dep, name)
	}
	// ... rest unchanged ...
}
```

b) Update `runDeps` (target.go:497) to pass the cap (`"runtime"` is already in target.go's imports — verified at plan time, along with `"time"`, `"errors"`, `"fmt"`; no import edits needed anywhere in this plan):

```go
case group.mode == DepModeParallel && group.collectAll:
	err = runGroupParallelAll(ctx, group.targets, parallelCap(len(group.targets), runtime.GOMAXPROCS(0)))
```

(The plain-parallel case gets the same treatment in Task 3; until then it compiles unchanged.)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/core/ -run 'TestParallelCap|TestRunGroupParallelAll' && go test ./test/ -run 'TestProperty_Execution'`
Expected: PASS (existing collect-all and gate subtests unaffected — cap ≥ 2 on this machine)

- [ ] **Step 5: Commit**

```bash
git add internal/core/target.go internal/core/target_internal_test.go
git commit -m "feat(core): bound runGroupParallelAll concurrency with semaphore

AI-Used: [claude]"
```

---

### Task 3: Bound `runGroupParallel` + skip queued targets on canceled context

**Files:**
- Modify: `internal/core/target.go:784` (`runGroupParallel`), `runDeps` plain-parallel call site (:505)
- Test: `internal/core/target_internal_test.go`

**Interfaces:**
- Produces: `func runGroupParallel(ctx context.Context, targets []*Target, capN int) error`.

- [ ] **Step 1: Write the failing tests**

Add (alphabetical: `TestRunGroupParallelBoundsConcurrency` after `TestRunGroupParallelAllBoundsConcurrency`, then `TestRunGroupParallelSkipsOnCanceledContext`):

```go
func TestRunGroupParallelBoundsConcurrency(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var (
		buf     bytes.Buffer
		mu      sync.Mutex
		cur     atomic.Int32
		maxSeen atomic.Int32
		ran     atomic.Int32
	)

	mk := func(name string) *Target {
		return Targ(func() {
			c := cur.Add(1)

			mu.Lock()
			if c > maxSeen.Load() {
				maxSeen.Store(c)
			}
			mu.Unlock()

			ran.Add(1)
			cur.Add(-1)
		}).Name(name)
	}

	targets := []*Target{mk("t1"), mk("t2"), mk("t3"), mk("t4")}
	ctx := WithExecInfo(context.Background(), ExecInfo{Output: &buf})

	err := runGroupParallel(ctx, targets, 1)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ran.Load()).To(Equal(int32(4)), "all targets must run")
	g.Expect(maxSeen.Load()).To(Equal(int32(1)), "cap 1 must serialize execution")
}

func TestRunGroupParallelSkipsOnCanceledContext(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var (
		buf bytes.Buffer
		ran atomic.Int32
	)

	mk := func(name string) *Target {
		return Targ(func() {
			ran.Add(1)
		}).Name(name)
	}

	targets := []*Target{mk("t1"), mk("t2"), mk("t3")}

	ctx, cancel := context.WithCancel(
		WithExecInfo(context.Background(), ExecInfo{Output: &buf}),
	)
	cancel() // canceled before the group starts

	err := runGroupParallel(ctx, targets, 1)

	g.Expect(err).To(HaveOccurred())
	g.Expect(ran.Load()).To(Equal(int32(0)), "no target may start on a canceled context")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/core/ -run 'TestRunGroupParallelBounds|TestRunGroupParallelSkips'`
Expected: FAIL — compile error: too many arguments in call to `runGroupParallel`

- [ ] **Step 3: Implement**

In `runGroupParallel` (target.go:784):

```go
func runGroupParallel(ctx context.Context, targets []*Target, capN int) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// ... existing setup unchanged ...

	sem := make(chan struct{}, max(1, capN))

	for i, dep := range targets {
		name := dep.GetName()
		results[i].Name = name

		go func(idx int, d *Target, targetName string) {
			sem <- struct{}{}
			defer func() { <-sem }()

			// Skip queued targets once the group is canceled (fail-fast):
			// starting fresh work after a sibling failed would defeat the mode.
			if ctx.Err() != nil {
				resultCh <- targetResult{index: idx, err: ctx.Err()}
				return
			}

			// ... existing body: WithExecInfo, OnStart, Run, send result — unchanged ...
		}(i, dep, name)
	}
	// ... rest unchanged ...
}
```

And `runDeps` (:505):

```go
case group.mode == DepModeParallel:
	err = runGroupParallel(ctx, group.targets, parallelCap(len(group.targets), runtime.GOMAXPROCS(0)))
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/core/ && go test ./test/`
Expected: PASS (incl. `FailFastReportsCancelledTargets` — skipped targets classify as Cancelled via the same `context.Canceled` path)

- [ ] **Step 5: Commit**

```bash
git add internal/core/target.go internal/core/target_internal_test.go
git commit -m "feat(core): bound runGroupParallel concurrency; skip queued deps after cancel

AI-Used: [claude]"
```

---

### Task 4: End-to-end cap property test (public API)

**Files:**
- Test: `test/execution_properties_test.go` (new subtest inside `TestProperty_Execution`, near the other parallel-dep subtests)

**Interfaces:**
- Consumes: public `targ.Targ(...).Deps(..., targ.DepModeParallel)` + `main.Run` only.

- [ ] **Step 1: Write the test** (green immediately — it locks the invariant at the public boundary; the red phases for this behavior were Tasks 2–3)

```go
	t.Run("ParallelDepsRespectConcurrencyCap", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var (
			mu      sync.Mutex
			cur     atomic.Int32
			maxSeen atomic.Int32
		)

		mkDep := func(name string) *targ.Target {
			return targ.Targ(func() {
				c := cur.Add(1)

				mu.Lock()
				if c > maxSeen.Load() {
					maxSeen.Store(c)
				}
				mu.Unlock()

				cur.Add(-1)
			}).Name(name)
		}

		deps := []any{
			mkDep("c1"), mkDep("c2"), mkDep("c3"),
			mkDep("c4"), mkDep("c5"), mkDep("c6"),
			targ.DepModeParallel,
		}
		main := targ.Targ(func() {}).Name("main").Deps(deps...)

		err := main.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())

		bound := max(2, runtime.GOMAXPROCS(0)/2)
		g.Expect(int(maxSeen.Load())).To(BeNumerically("<=", bound),
			"parallel dep concurrency must not exceed max(2, GOMAXPROCS/2)")
	})
```

(Add `"runtime"` to the test file's imports if absent. NOTE: upper-bound assertion only — asserting `maxSeen == bound` would be scheduling-dependent and flaky.)

- [ ] **Step 2: Run it**

Run: `go test ./test/ -run 'TestProperty_Execution/ParallelDepsRespectConcurrencyCap'`
Expected: PASS

- [ ] **Step 3: Mutation sanity check (manual red)**

Temporarily change `parallelCap` to `return n` and re-run the test on a machine where 6 > max(2, GOMAXPROCS/2) would... — NOT verifiable on a GOMAXPROCS=10 machine (bound=5 < 6 targets, but without sleeps overlap of all 6 is not guaranteed, so the test may still pass). Skip the mutation check; the deterministic bound proof lives in the whitebox cap=1 tests (Tasks 2–3). This subtest is a public-API regression tripwire, not the primary proof.

- [ ] **Step 4: Commit**

```bash
git add test/execution_properties_test.go
git commit -m "test(execution): lock parallel dep concurrency bound at public API

AI-Used: [claude]"
```

---

### Task 5: `runGroupSerialAll` + serial collect-all dispatch

**Files:**
- Modify: `internal/core/target.go` — new func between `runGroupSerial` (:1017) and `runShellCommand` (:1031); `runDeps` gains a `case group.collectAll:` arm; `CollectAllErrors` doc comment (:50-52)
- Modify: `targ.go:17-19` (`CollectAllErrors` re-export doc comment)
- Test: `test/execution_properties_test.go` (new subtests in the collect-all block, ~:1445-1610)

**Interfaces:**
- Consumes: `classifyCollectAllResult(err error) Result` (target.go:752), `NewMultiError(results []TargetResult) *MultiError` (result.go:44), `FormatDetailedSummary` (result.go:93), `reportedError` (result.go:164), `outputFromContext` (exec_info.go:34).
- Produces: `func runGroupSerialAll(ctx context.Context, targets []*Target) error`.

- [ ] **Step 1: Write the failing tests**

In the collect-all block of `TestProperty_Execution` (alphabetical placement within the block's subtests is not enforced — follow the block's existing grouping style):

```go
	t.Run("SerialCollectAllErrorsRunsAllInOrder", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var mu sync.Mutex

		var order []string

		record := func(name string) {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		}

		dep1 := targ.Targ(func() error {
			record("dep1")
			return errors.New("dep1 failed")
		}).Name("dep1")

		dep2 := targ.Targ(func() error {
			record("dep2")
			return errors.New("dep2 failed")
		}).Name("dep2")

		dep3 := targ.Targ(func() { record("dep3") }).Name("dep3")

		main := targ.Targ(func() {}).Name("main").
			Deps(dep1, dep2, dep3, targ.CollectAllErrors)

		err := main.Run(context.Background())
		g.Expect(err).To(HaveOccurred())

		var me *targ.MultiError
		g.Expect(errors.As(err, &me)).To(BeTrue())
		g.Expect(me.Results()).To(HaveLen(3))
		g.Expect(order).To(Equal([]string{"dep1", "dep2", "dep3"}),
			"serial collect-all must run every dep, in declaration order")
	})

	t.Run("SerialCollectAllErrorsAllPassingSucceeds", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		dep1 := targ.Targ(func() {}).Name("dep1")
		dep2 := targ.Targ(func() {}).Name("dep2")

		main := targ.Targ(func() {}).Name("main").
			Deps(dep1, dep2, targ.CollectAllErrors)

		err := main.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
	})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./test/ -run 'TestProperty_Execution/SerialCollectAll'`
Expected: FAIL — `SerialCollectAllErrorsRunsAllInOrder` gets dep1's plain error (not a `*MultiError`) and `order == ["dep1"]` (fail-fast serial ran only the first dep)

- [ ] **Step 3: Implement**

a) New runner in `internal/core/target.go` (between `runGroupSerial` and `runShellCommand`):

```go
// runGroupSerialAll runs targets sequentially, continuing past failures and
// collecting every result (CollectAllErrors semantics for serial groups).
// Quiet on success; on failure prints per-target errors plus a detailed
// summary and returns a MultiError.
func runGroupSerialAll(ctx context.Context, targets []*Target) error {
	out := outputFromContext(ctx)
	results := make([]TargetResult, len(targets))
	hasFailure := false

	for i, dep := range targets {
		results[i].Name = dep.GetName()

		start := time.Now()
		err := dep.Run(ctx)

		results[i].Duration = time.Since(start)
		results[i].Err = err
		results[i].Status = classifyCollectAllResult(err)

		if results[i].Status != Pass {
			hasFailure = true
		}

		if err != nil && !errors.Is(err, context.Canceled) {
			_, _ = fmt.Fprintf(out, "Error: %v\n", err)
		}
	}

	if !hasFailure {
		return nil
	}

	summary := FormatDetailedSummary(results)
	if summary != "" {
		_, _ = fmt.Fprintln(out, "\n"+summary)
	}

	return reportedError{err: NewMultiError(results)}
}
```

b) Dispatch in `runDeps` (target.go:497) — full switch after this task:

```go
		switch {
		case group.mode == DepModeParallel && group.collectAll:
			err = runGroupParallelAll(ctx, group.targets, parallelCap(len(group.targets), runtime.GOMAXPROCS(0)))
		case group.mode == DepModeParallel:
			err = runGroupParallel(ctx, group.targets, parallelCap(len(group.targets), runtime.GOMAXPROCS(0)))
		case group.collectAll:
			err = runGroupSerialAll(ctx, group.targets)
		default:
			err = runGroupSerial(ctx, group.targets)
		}
```

- [ ] **Step 4: Update the `CollectAllErrors` GoDoc comments** (highest-visibility doc surface for this behavior — pkg.go.dev / IDE hover / `go doc`). Measured at plan time: `grep -rn "parallel deps to run all targets" targ.go internal/` → exactly 2 matches (`targ.go:17`, `internal/core/target.go:50`) — these are the only copies. In BOTH, replace:

```go
	// CollectAllErrors causes parallel deps to run all targets to completion
	// and collect all errors, rather than cancelling on first failure.
```

with:

```go
	// CollectAllErrors causes deps to run all targets to completion and
	// collect all errors, rather than stopping on the first failure.
	// Applies to both parallel and serial dependency groups.
```

- [ ] **Step 5: Run tests**

Run: `go test ./test/ && go test ./internal/core/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/core/target.go targ.go test/execution_properties_test.go
git commit -m "feat(core): honor CollectAllErrors for serial dep groups

AI-Used: [claude]"
```

---

### Task 6: `--dep-mode` flatten preserves `CollectAllErrors`

**Files:**
- Modify: `internal/core/command.go:1944-1966` (`runNodeDeps`)
- Test: `test/overrides_properties_test.go` (new subtests in `TestProperty_Overrides`, after `DepModeOverrideFlattensMixedGroups` ~:529)

**Interfaces:**
- Consumes: `depGroup{targets, mode, collectAll}` (target.go:691-694), Tasks 3/5 runners.

- [ ] **Step 1: Write the failing tests**

```go
	t.Run("DepModeSerialPreservesCollectAllErrors", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var ran atomic.Int32

		dep1 := targ.Targ(func() error {
			ran.Add(1)
			return errors.New("dep1 failed")
		}).Name("dep1")

		dep2 := targ.Targ(func() error {
			ran.Add(1)
			return errors.New("dep2 failed")
		}).Name("dep2")

		main := targ.Targ(func() {}).Name("main").
			Deps(dep1, dep2, targ.DepModeParallel, targ.CollectAllErrors)

		_, err := targ.Execute(
			[]string{"app", "--dep-mode", "serial", "main"},
			main, dep1, dep2, dummy(),
		)
		g.Expect(err).To(HaveOccurred())

		var me *targ.MultiError
		g.Expect(errors.As(err, &me)).To(BeTrue(),
			"--dep-mode serial must preserve CollectAllErrors")
		g.Expect(me.Results()).To(HaveLen(2))
		g.Expect(ran.Load()).To(Equal(int32(2)), "both deps must run despite the first failing")
	})

	t.Run("DepModeParallelPreservesCollectAllErrors", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		dep1 := targ.Targ(func() error {
			return errors.New("dep1 failed")
		}).Name("dep1")

		dep2 := targ.Targ(func() error {
			return errors.New("dep2 failed")
		}).Name("dep2")

		main := targ.Targ(func() {}).Name("main").
			Deps(dep1, dep2, targ.CollectAllErrors)

		_, err := targ.Execute(
			[]string{"app", "--dep-mode", "parallel", "main"},
			main, dep1, dep2, dummy(),
		)
		g.Expect(err).To(HaveOccurred())

		var me *targ.MultiError
		g.Expect(errors.As(err, &me)).To(BeTrue(),
			"--dep-mode parallel must preserve CollectAllErrors")
		g.Expect(me.Results()).To(HaveLen(2))
	})
```

(Add `errors` / `sync/atomic` to the test file's imports if absent.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./test/ -run 'TestProperty_Overrides/DepMode.*PreservesCollectAll'`
Expected: FAIL — `errors.As(err, &me)` is false (flatten dropped `collectAll`; fail-fast returned a plain error)

- [ ] **Step 3: Implement**

In `runNodeDeps` (`internal/core/command.go:1952-1963`):

```go
	// Apply --dep-mode override: flatten all groups into one.
	// CollectAllErrors survives the flatten — if any original group
	// collected all errors, the flattened group does too.
	if overrides.DepMode != "" {
		var mode DepMode
		if overrides.DepMode == depModeParallelStr {
			mode = DepModeParallel
		}

		collectAll := false

		for _, g := range target.depGroups {
			if g.collectAll {
				collectAll = true
				break
			}
		}

		allDeps := target.GetDeps()
		if len(allDeps) > 0 {
			target.depGroups = []depGroup{{targets: allDeps, mode: mode, collectAll: collectAll}}
		}
	}
```

(If this pushes `runNodeDeps` over a complexity limit, extract the loop as `func anyGroupCollectsAll(groups []depGroup) bool` — alphabetical placement among command.go's unexported funcs.)

- [ ] **Step 4: Run tests**

Run: `go test ./test/ && go test ./internal/core/`
Expected: PASS (incl. all existing `DepMode*` subtests)

- [ ] **Step 5: Commit**

```bash
git add internal/core/command.go test/overrides_properties_test.go
git commit -m "fix(core): preserve CollectAllErrors through --dep-mode flatten

AI-Used: [claude]"
```

---

### Task 7: Documentation scrub

**Files:**
- Modify: `README.md` (~:346 Dependencies, :380, :714), `docs/specs/requirements.md` (:49, :175), `docs/specs/architecture.md` (:42), `docs/specs/implementation.md` (:122), `docs/specs/tests.md` (T-3 property list)

**Interfaces:** none (prose edits). Every edit below is the complete replacement text.

- [ ] **Step 1: README Dependencies section** — after the mixed-groups example (~:347, the chained `.Deps()` code block), add:

```markdown
Parallel groups run at most `min(n, max(2, GOMAXPROCS/2))` targets concurrently (where `n` is the group's target count); excess targets queue until a slot frees up.
```

- [ ] **Step 2: README :380** — replace:

> By default, the first failure cancels remaining targets. Use `targ.CollectAllErrors` to run all and report all failures:

with:

> By default, the first failure cancels remaining targets. Use `targ.CollectAllErrors` to run all and report all failures (works with serial groups too — every dep runs, all failures are reported):

- [ ] **Step 3: README :714** — replace the note with:

> Note: `--dep-mode` bypasses the conflict detection above and silently overrides compile-time dependency modes: it flattens all of a target's [dependency groups](#dependencies) into a single group. `targ.CollectAllErrors` survives the flatten — if any group collects all errors, the flattened group does too.

- [ ] **Step 4: specs/requirements.md REQ-7 (:49)** — replace the middle sentence:

> Parallel mode with `CollectAllErrors` collects all failures.

with:

> `CollectAllErrors` collects all failures, in both parallel and serial modes.

- [ ] **Step 5: specs/requirements.md DES-5 (:175)** — replace the final sentence of the paragraph:

> In parallel mode, fail-fast cancels remaining targets (default) or `CollectAllErrors` runs all and reports all failures.

with:

> In parallel mode, fail-fast cancels remaining targets (default) or `CollectAllErrors` runs all and reports all failures; serial groups honor `CollectAllErrors` by continuing past failures. Parallel groups are bounded to `min(n, max(2, GOMAXPROCS/2))` concurrent targets.

- [ ] **Step 6: specs/architecture.md ARCH-6 (:42)** — replace:

> `MultiError` collects all failures from parallel CollectAllErrors mode.

with:

> `MultiError` collects all failures from CollectAllErrors mode (parallel or serial).

- [ ] **Step 7: specs/implementation.md :122** — replace the caveat with:

> **Caveat:** `--dep-mode` (`serial`|`parallel`) overrides how a target's dependency groups execute; the override flattens all dependency groups into a single group. The `CollectAllErrors` option survives the flatten (any group having it sets it on the flattened group) — e.g. `--dep-mode serial` on `check-full` runs the legs one at a time and still reports all failures.

- [ ] **Step 8: specs/tests.md T-3** — append to the property list:

```markdown
- Property: parallel dep concurrency never exceeds min(n, max(2, GOMAXPROCS/2))
- Property: serial CollectAllErrors runs every dep in order and reports all failures
- Property: --dep-mode flatten preserves CollectAllErrors (serial and parallel)
```

and append the new test names to the **Tests:** line if T-3 lists them (follow the section's existing style).

- [ ] **Step 9: Verify doc claims against behavior**

These are POST-EDIT checks; the pre-edit baselines were measured at plan time (HEAD 65b1d1c) so the executor can confirm each count flipped:

| Command | Baseline (measured) | Expected after Task 7 |
|---|---|---|
| `grep -c "ignores any" README.md` | 1 | 0 |
| `grep -c "tracked in #26" docs/specs/implementation.md` | 1 | 0 |
| `grep -rn "parallel deps to run all targets" targ.go internal/` | 2 matches (targ.go:17, internal/core/target.go:50) | no matches (Task 5 Step 4 rewrote both) |

- [ ] **Step 10: Commit**

```bash
git add README.md docs/specs/requirements.md docs/specs/architecture.md docs/specs/implementation.md docs/specs/tests.md
git commit -m "docs: bounded dep fan-out + CollectAllErrors survives --dep-mode and serial groups

AI-Used: [claude]"
```

---

### Task 8: Full verification (real binary, full gate)

- [ ] **Step 1:** `cd /Users/joe/repos/personal/targ && pwd` — confirm cwd (vault note 359: verify cwd before every build-runner invocation).
- [ ] **Step 2:** Run `targ --no-binary-cache check-full` (note 371: bypass the bootstrap binary cache so the gate binary embeds this change). Expected: all eight legs report, exit 0. This run itself exercises the capped parallel collect-all path for real.
- [ ] **Step 3:** Real-binary probe of the fixed flatten: `targ --no-binary-cache check-nils --dep-mode serial` (a target with a serial dep chain; confirms flatten path executes). Then confirm collect-all survival on a failing case only if one exists naturally — do NOT commit synthetic failures; the automated tests in Task 6 are the failing-path proof.
- [ ] **Step 4:** Observe cap behavior in Step 2's output: at most 5 legs (GOMAXPROCS=10 machine → cap 5) show `starting...` before the first completions. If all 8 start simultaneously, the cap is not wired — stop and debug.

---

### Task 9: Complete

- [ ] **Step 1:** Close issue #26 with a comment naming what landed (Option B semaphore with `min(n, max(2, GOMAXPROCS/2))`, the fail-fast queued-skip, serial collect-all, flatten preservation) and what was explicitly not done (Option A ordering; the `runNodeDeps` lasting-mutation wart, noted as pre-existing).
- [ ] **Step 2:** Confirm working tree clean (`git status`), all commits present with `AI-Used: [claude]` trailer.
- [ ] **Step 3:** Delete no docs — the plan file stays as a session record (repo convention).

# Issue #40 — default-root dispatch, bounded CLI fan-out, and the undocumented default target

Implementation plan. Branch `issue-40-default-dispatch`, worktree
`.claude/worktrees/issue-40`, based on `2473fce`.

Issue #40 supersedes #24 and #34.

## Baseline (measured, not assumed)

`targ check-full` in this worktree at `2473fce`: **8/8 PASS**.

The first run reported `lint-full` FAIL with 51 findings (wsl_v5 38, prealloc 4, whitespace 3,
testpackage 2, wrapcheck 2, unparam 1, tparallel 1). `golangci-lint cache clean` followed by
`targ lint-full` returned `0 issues`, and a full `targ check-full` re-run returned 8/8. That first
result was the stale-result-cache phantom CLAUDE.md documents, not a real failure. **Anyone
re-running the baseline in a fresh worktree should clean the lint cache first.**

## Verified corrections to issue #40's text

Everything in #40 was checked against the code and against real binaries built from this worktree.
It is accurate except for the following. These are recorded so the next reader does not inherit
them, exactly as #40 did for #24 and #34.

1. **`-p` does NOT swallow the error.** #40 says the `-p` form "reports a failed target with **no
   error text at all**" and that "'unknown command' never reaches the user", and repeats it in the
   #24 corrections list. Measured: `targ -p marker` prints

   ```
   [marker] starting...
   [marker] Error: unknown command: marker
   [marker] FAIL (0s)

   FAIL:1
   ```

   `run_env.go:384-386` prints the error explicitly, prefixed with the target name. The acceptance
   criterion built on this claim ("prints the underlying error, not a bare `FAIL:1`") is already
   satisfied in its literal form today. What is actually wrong is that the error printed is the
   *spurious* `unknown command`, because Defect A stops the target from ever running. Fixing
   Defect A makes `-p <name>` print the target's real error. **No separate error-plumbing work is
   implied by that criterion**, and none is planned.

2. **Defect A also bites a GROUP root.** #40's examples only show a plain-target root. Measured on
   a module whose sole root is `targ.Group("grp", s1…s8)`:

   ```
   $ repro40grp grp s1   → Unknown command: grp
   $ repro40grp grp      → Unknown command: grp
   ```

   The strip must match the sole root's own name whatever kind of node it is. (Note the error text
   differs from the plain-target case — `Unknown command: grp` with no `Error:` prefix — because
   the group's own dispatch emits it.)

3. **`executeMultiRootParallel` is at `:493`, not `:492`.** `:492` is its `//nolint` line. Every
   other line number in #40 is exact: `:241`, `:281`, `:324`, `:433`, `:559`, `:1098`, `:1111`,
   `target.go:794`, `command.go:1793`/`:1800`, `generators.go:76`/`:82-87`,
   `execution_properties_test.go:586`/`:666`/`:709`/`:1266-1279`.

4. **`hasDefault` is read in three places, not one.** `run_env.go:732` (`handleGlobalHelp`),
   `:753` (`handleNoArgs`), `:1111` (main dispatch). Only `:1111` reaches the four functions #40
   tables. The other two matter to this plan anyway — see Defect C, where the help fix has to
   cover **both** help paths, and Out of scope, where `handleNoArgs`' execution path is
   deliberately untouched.

5. **`printCommandHelp` has a fourth call site** #40 does not mention: `export_test.go:16` aliases
   it as `PrintCommandHelpForTest`, used at `command_test.go:128` and `:144`. A signature change
   breaks them; the design below avoids a signature change entirely.

### Confirmed by independent enumeration

- `grep -n "sem\b\|parallelCap\|GOMAXPROCS" internal/core/run_env.go` → no output. Confirmed.
- `//nolint:cyclop,funlen` on `run_env.go:280` and `:492`. Confirmed.
- **No fifth uncapped fan-out site exists.** Every `go func` in the repo was enumerated: the two in
  `run_env.go` (uncapped), the two in `target.go` `runGroupParallel`/`runGroupParallelAll`
  (capped), and five single-goroutine-per-subprocess sites in `internal/sh` and `dev/` that are not
  fan-outs. Callers of `executeWithParents`, `findMatchingRoot`, and each of the four functions
  were enumerated repo-wide.
- `ParallelFlagRunsTargetsInParallel` registers two roots and therefore exercises
  `executeMultiRootParallel`; only `ParallelFlagWithSingleRootGroup` is on `executeDefaultParallel`.
  #40's correction of #34 on this point is right.

## Measured defect evidence

Fixtures are real modules with a `replace` directive at this worktree, built with `go build` and
run as real binaries. `GOMAXPROCS=10` on this machine, so `parallelCap(8, 10) = min(8, max(2, 5))
= 5`.

| path | command | peak concurrent starts (before) | cap |
|---|---|---|---|
| `executeMultiRootParallel` (8 roots) | `-p t1 … t8` | **8** | 5 |
| `executeDefaultParallel` (1 root group, 8 subs) | `-p s1 … s8` | **8** | 5 |

Peak was measured with an atomic in-flight counter inside each target — no wall-clock timing.

## Design

### Settled during orientation — do not relitigate in-task

- **Fix child-side, in `internal/core`, not in the `internal/runner` wrapper.** #40's reasoning
  holds: `executeDefault` is reached directly by `targ.Execute`/`targ.Run` with no wrapper
  involved.
- **Keep the top-level cap and per-dep-group caps independent.** Per-group budgets, matching #26's
  design. No shared global budget.
- **Do not fold `target.go`'s `runGroupParallel`/`runGroupParallelAll` into the new CLI helper.**
  They are a third near-duplicate of the same fan-out shape, but they operate on `[]*Target` with
  `onStart`/`onStop` hooks one layer down (inside a single target's `Run`), not over CLI dispatch
  args. Unifying all three is a larger refactor nobody asked for. They stay as the reference
  pattern the CLI helper mirrors.
- **Do not change the two distinct unknown-command output formats.** The default path prints
  `Error: unknown command: bogus` with no usage block; the multi-root path prints
  `Unknown command: bogus` plus usage. Both satisfy "still fails, naming bogus". Unifying them is
  a separate, user-visible change.

### Defect A — strip the sole root's own name

One shared helper, used by both default-mode entry points. Two conformant copies of the same
strip rule is exactly how the next regression gets in; one site is the point.

```go
// stripDefaultRootName drops a leading arg that names the sole default root
// itself, so `targ <name>` reaches the same code path as bare `targ`. The
// multi-root path already matches-then-strips; this is the default path's
// equivalent, and the single site both default-mode entry points share.
func (e *runExecutor) stripDefaultRootName(args []string) ([]string, bool) {
	if len(args) > 0 && e.findMatchingRoot(args[0]) != nil {
		return args[1:], true
	}

	return args, false
}
```

`executeDefault`'s loop becomes:

```go
	remaining := e.rest

	for len(remaining) > 0 {
		args, stripped := e.stripDefaultRootName(remaining)

		next, err := e.roots[0].executeWithParents(
			e.ctx,
			args,
			nil,
			map[string]bool{},
			false,
			e.opts,
		)
		if err != nil {
			var re reportedError
			if !errors.As(err, &re) {
				e.env.Printf("Error: %v\n", err)
			}

			return ExitError{Code: 1}
		}

		if !stripped && len(next) == len(args) {
			e.env.Printf("Unknown command: %s\n", args[0])
			return ExitError{Code: 1}
		}

		remaining = next
	}
```

The `!stripped` guard is load-bearing and not cosmetic. With `targ marker`, the strip leaves
`args` empty, `executeWithParents` returns `next` empty, and the pre-existing
`len(next) == len(args)` progress check would be `0 == 0` — true — and then index `args[0]` and
panic. Stripping *is* progress, so the check only applies when nothing was stripped. Termination
still holds in the stripped case because `next` is at most `args`, which is already one shorter
than `remaining`.

Falling through when the name does not match preserves `targ bogus` → `Error: unknown command:
bogus`.

### Defect B — one bounded fan-out for both parallel paths

The two loops differ only in how they turn CLI args into work: the default path maps every
non-flag arg onto `roots[0]`, the multi-root path resolves each arg to a root (with glob
expansion). Everything after that — printer setup, goroutines, result collection, classification,
summary — is identical. So the resolution stays per-path and the fan-out is shared.

```go
// parallelUnit is one CLI target scheduled by runUnitsParallel.
type parallelUnit struct {
	node     *commandNode
	name     string
	args     []string
	explicit bool
}
```

`explicit` is the sixth argument of `executeWithParents`; the default path passes `false`, the
multi-root path `true`. It lives on the unit because the default path's value now depends on
whether the arg named the root itself.

`executeDefaultParallel` becomes a resolver plus a call:

```go
func (e *runExecutor) executeDefaultParallel() error {
	var units []parallelUnit

	for _, arg := range e.rest {
		if strings.HasPrefix(arg, "-") {
			continue
		}

		args, stripped := e.stripDefaultRootName([]string{arg})
		units = append(units, parallelUnit{node: e.roots[0], name: arg, args: args, explicit: !stripped})
	}

	return e.runUnitsParallel(units)
}
```

`executeMultiRootParallel` likewise, with its resolution extracted to `resolveMultiRootUnits`
(unchanged logic: glob expansion, `findMatchingRoot`, unknown-name usage error).

The shared runner carries the semaphore and the cancellation skip-check, transplanted from
`runGroupParallel`:

```go
	sem := make(chan struct{}, max(1, parallelCap(len(units), runtime.GOMAXPROCS(0))))

	for i, u := range units {
		results[i].Name = u.name

		go func(idx int, unit parallelUnit) {
			sem <- struct{}{}
			defer func() { <-sem }()

			// Skip queued units once the run is canceled (fail-fast):
			// starting fresh work after a sibling failed would defeat the mode.
			if ctx.Err() != nil {
				resultCh <- unitResult{index: idx, err: ctx.Err()}

				return
			}

			resultCh <- e.runUnit(ctx, idx, unit, maxNameLen, printer)
		}(i, u)
	}
```

Acquire happens before `runUnit`, so a queued unit does not print `starting...` before it runs.

`runUnitsParallel` further delegates to `runUnit` (execute one unit with parallel-aware output
wiring), `collectUnitResults` (drain, cancel on first error, return its index), and
`reportUnitResults` (classify, print errors and stop lines, print summary).

**Measured lint outcome, not predicted:** with this decomposition, `golangci-lint run ./...`
returns `0 issues` **with both `//nolint:cyclop,funlen` directives removed and none added**. The
`//nolint` count in `run_env.go` goes 6 → 4 (the remaining four — `wrapcheck` on `Getwd`,
`containedctx` on the `ctx` field, `unparam` on `handleList`, `cyclop` on `runWithEnvInternal` —
are pre-existing and untouched). This was verified by deleting a speculative
`//nolint:funlen` from `runUnitsParallel` and re-running lint: still `0 issues`.

### Defect C — help advertises both working forms

`printCommandHelp` receives one node and `RunOptions`; it cannot know it is rendering the sole
root. #40 says "the caller has to supply the signal" and leaves the threading to the plan.

**A parameter does not work.** There are two help paths for a default root, and only one of them
runs through a caller that knows `hasDefault`:

| invocation | path | reaches `printCommandHelp` via |
|---|---|---|
| `targ <name> --help` | `handleSpecialCommands` → `handleGlobalHelp` (`:732` knows `hasDefault`) | direct call |
| `targ --help` | `extractHelpFlag` empties args → `handleNoArgs` → `roots[0].execute` → `executeWithParents` | the generic `HelpOnly` interception at `command.go:157-161`, which knows nothing |

A first prototype threaded `isDefaultRoot bool` through `printCommandHelp` and passed `true` only
from `handleGlobalHelp`. It rendered correctly for `targ <name> --help` and **had no effect at
all** for plain `targ --help` — the more common invocation. Recorded because it looks right on
paper.

The signal therefore rides on the node, set once where `hasDefault` is computed:

```go
	exec.hasDefault = len(exec.roots) == 1 && opts.AllowDefault
	if exec.hasDefault {
		exec.roots[0].IsDefaultRoot = true
	}
```

`parseTargets` builds fresh nodes on every run, so this is per-execution state, not a global.
All four `printCommandHelp` call sites — including `PrintCommandHelpForTest` — then get the right
answer with no signature change.

`commandNode` gains:

```go
	// IsDefaultRoot marks the sole registered root when AllowDefault is set:
	// it runs bare (`targ`) as well as by name (`targ <name>`), and its help
	// advertises both forms.
	IsDefaultRoot bool
```

`printCommandHelp` brackets the name and forwards the fact:

```go
	// A default root runs bare as well as by name, so its name is optional.
	if node.IsDefaultRoot {
		usageParts[0] = "[" + usageParts[0] + "]"
	}
```

`help.TargetHelpOpts` gains `DefaultRoot bool`, and `WriteTargetHelp` prepends the bare example
when generating (a caller-supplied `opts.Examples` list still wins untouched):

```go
// withBareInvocation prepends the bare-invocation example for a default root
// and relabels the generated named form, so help advertises both working
// invocations rather than only the named one.
func withBareInvocation(generated []Example, binaryName string) []Example {
	for i := range generated {
		if generated[i].Title == basicUsageTitle {
			generated[i].Title = "By name"
		}
	}

	return append([]Example{{Title: basicUsageTitle, Code: binaryName}}, generated...)
}
```

`GenerateTargetExamples` keeps its signature; the `"Basic usage"` literal becomes a
`basicUsageTitle` constant shared by both sites.

**Measured rendering** (fixture with one target `marker`, binary `bin`):

```
Usage:
  bin [targ flags...] [marker]

Examples:
  Basic usage:
    bin
  By name:
    bin marker
```

Both `targ --help` and `targ marker --help` now produce this. A two-target module's
`bin marker --help` is byte-identical to before (`bin [targ flags...] marker`, single
`Basic usage: bin marker`), verified by running both.

## Prototype verification (run before this plan was written)

The whole design was applied in a throwaway worktree (`.claude/worktrees/issue-40-proto`, detached
at `2473fce`) and exercised. Nothing below is a prediction.

| check | result |
|---|---|
| `go build ./...` | OK |
| `go test ./...` | all packages ok, including `test` (`ParallelFlagRunsTargetsInParallel` and `ParallelFlagWithSingleRootGroup` green, untouched) |
| `golangci-lint run ./...` | `0 issues`, both `cyclop,funlen` nolints deleted, none added |
| `repro40` (1 plain target) bare / `marker` / `-p marker` / `bogus` | `MARKER-ONE` / `MARKER-ONE` / `PASS:1` / `Error: unknown command: bogus` |
| `repro40grp grp s1`, `repro40grp grp` | run (peak 1) / run group default (peak 0) |
| `repro40cap -p t1…t8` peak | **8 → 5** |
| `repro40grp -p s1…s8` peak | **8 → 5** |
| `repro40two` (2 roots): `marker`, `-p marker other`, `bogus`, `marker --help` | unchanged in every case |

The prototype worktree is deleted before this plan's commit; execution re-derives the code through
TDD rather than transplanting the diff.

## Doc-surface enumeration (run at plan-write time)

Two invariants change: (1) a module registering exactly one target gets a default that runs bare,
by name, and under `-p`; (2) CLI-level `--parallel`/`-p` fan-out is now bounded by
`parallelCap(n, GOMAXPROCS)` — the same formula dep groups have had since #26.

Commands run (re-runnable):

```bash
grep -rniI --exclude-dir=.git -E "default target|default root|default command|defaults to" .
grep -rniI --exclude-dir=.git -E "single target|one target|lone target|only target|sole target" .
grep -rniI --exclude-dir=.git -E "bare targ|no arguments|no args\b|without arguments" .
grep -rniI --exclude-dir=.git -E "default-target|defaulttarget|AllowDefault|hasDefault" .
grep -rniI --exclude-dir=.git -E "all at once|simultaneous|concurrently|no limit|unbounded" .
grep -rniI --exclude-dir=.git -E "fan-out|fanout|GOMAXPROCS|parallelCap|concurrency cap" .
grep -rniI --exclude-dir=.git -E -- "--parallel" README.md CLAUDE.md docs/ specs/ projects/ .claude/
grep -rn "Usage:" . --include="*.go" --include="*.md" --include="*.golden"
grep -n "^## \|^### " README.md
```

Per-file disposition. Rows marked N/A had hits that are false positives or live in archives.

| file | line(s) | what it says | disposition | reason |
|---|---|---|---|---|
| `README.md` | 319 §Command Names | kebab-case derivation, `.Name()` override | **update** | Natural home for the default-target rule: bare, by name, and `-p <name>` all run the sole target |
| `README.md` | 474-493 §Example Help Output | `$ targ build --help` → `Usage: build [flags]` | **update** | Add a single-target example showing the bracketed usage line and both invocation examples. Note the existing block is *already* stale independent of this issue (real output is `Usage:\n  <bin> [targ flags...] build`); do not copy its format |
| `README.md` | 708 `--parallel`/`-p` row | "Run multiple targets concurrently" | **update** | Add the bound, mirroring what :350 does for dep groups |
| `README.md` | 350 | dep-group cap sentence | keep | Accurate and correctly scoped; the §Runtime Flags update carries the cross-reference |
| `README.md` | 186 §Execution Control | builder-method table | keep | Zero hits; landmark only in #40's text |
| `README.md` | 293 | `// name defaults to "golangci-lint"` | N/A | Different sense of "defaults" (auto-derived name) |
| `docs/specs/use-cases.md` | 5-13 UC-1 | "Developer runs `targ <command>`…" | **update** | Silent on the optional-command case, and `targ <command>` is precisely what is broken for it today |
| `docs/specs/requirements.md` | 173-178 DES-5 | "Parallel groups are bounded to `min(n, max(2, GOMAXPROCS/2))`" | **update** | Scoped to `.Deps()` only; add the CLI `-p` fan-out using the identical formula |
| `docs/specs/requirements.md` | 19-24 REQ-3 | runtime-flag enumeration | **update** | Add the default-dispatch behavioural rule (bare / by name / `-p <name>`) |
| `docs/specs/architecture.md` | 19-24 ARCH-3 | "Command Resolution and Execution" | **update** | No ARCH item names the `hasDefault` mechanism at all; ARCH-3 owns command resolution and is the right home |
| `docs/specs/tests.md` | 32-53 T-3 | execution-behaviour property list | **update** | Add two property bullets: `targ <name>` succeeds with exactly one registered target; top-level `-p` fan-out never exceeds `parallelCap` |
| `docs/specs/state.toml` | `[layers.*]`, `[tree.*]` | traced adoption state | **update** | Add a `history` re-entry note; extend existing IDs rather than mint new ones (matches how #26 and #32 re-entered) |
| `docs/specs/implementation.md` | 36-42 IMPL-5, 117-124 IMPL-15 | purpose blurbs enumerating flags | keep | Enumeration only; no claim to correct |
| `internal/core/types.go` | `AllowDefault bool` | **no doc comment at all** | **update** | Controls the whole invariant, is part of the public `targ.RunOptions` alias, and its sibling `BinaryMode` is documented |
| `internal/core/run_env.go` | 239 | "executeDefault executes commands against a single default root." | **update** | The entirety of the in-code documentation of the invariant; must say the name is optional |
| `internal/core/target.go` | 767-772 `parallelCap` | "bounds how many targets in a parallel **dep group** run concurrently" | **update** | Once the CLI path calls it, the comment's dep-group scoping is wrong |
| `internal/core/override.go` | 27 `Parallel bool` | "Run multiple targets concurrently (--parallel or -p)" | **update** | Public GoDoc via the `targ.RuntimeOverrides` alias; add the bound |
| `targ.go` | 152-249 (`Execute`, `ExecuteRegistered`, `ExecuteWithOptions`, `Main`, `Register`) | none mention the default target | **update** | The pkg.go.dev-facing gap; one sentence on `Register`/`Main` is enough |
| `internal/flags/flags.go` | 71-76 `--parallel` `Desc` | "Run multiple targets concurrently" | keep | This string is rendered inside every `--help`; adding the formula bloats the flag table. The bound belongs in README §Runtime Flags, which the row already implies |
| `specs/001-parallel-output/contracts/api.md` | 119 | "still executes targets concurrently" | **propose, do not fold** | Closed spec-kit artifact for a shipped, different feature (output prefixing). Listed for Joe's yes/no; not in this cycle's scope |
| `docs/archive/**` | many | old usage-line templates, `--parallel` rows, REQ-039 | N/A | Archive convention: no retro-edits |
| `docs/plans/2026-07-23-issue-26-dep-fanout-cap.md` | 33, 918 | names this exact CLI fan-out as deferred out-of-scope for #26 | N/A (cite, don't edit) | Load-bearing prior art: proves the gap was known and deferred |
| `docs/plans/**` (others) | various | historical records | N/A | Historical |
| `specs/001-parallel-output/**` (others) | various | printer buffering, thread-safety | N/A | Different concern; "unbounded" refers to line buffers |
| `projects/portable-targets/**`, `session-learning-prompt.md`, `.claude/skills/**`, `.claude/commands/**` | various | unrelated senses of "default"/"one target"/"at once" | N/A | False positives |
| `internal/runner/testdata/golden/*.golden` | all | `--create`/`--sync`/`--to-func`/`--to-string` meta-command help | N/A | Static literals in `runner.go`; unreachable from `commandNode` rendering |
| `internal/runner/runner.go` | 3906 `printMultiModuleHelp` | `Usage: targ [FLAGS...] COMMAND …` | N/A | Only fires when `len(moduleGroups) > 1`; a single-target module always takes `handleSingleModule` |

**Zero-hit surfaces, so absence is distinguishable from not-searched:** no C4 or architecture
diagram directory exists; no `CHANGELOG.md`; no `.traced/config.toml` (traced state lives solely
in `docs/specs/state.toml`); `internal/core/doc.go` has no package comment; `examples/*` contain
no single-target `Main()` example and no `--parallel` prose.

### Tests that pin rendered help

No test anywhere does an exact-match assertion on a full default-root `Usage:` line. The closest
are substring assertions that reach the changed path:

- `test/validation_properties_test.go:12-65` (`TestProperty_UsageLine`, 3 subtests) — single
  target, `--help`, asserts on positional-arg tokens. Reaches the path; survived the prototype.
- `test/hierarchy_properties_test.go:416-431` (`HelpInDefaultModeShowsTargetHelp`) — same path,
  description substring. Survived the prototype.
- `internal/core/binary_mode_propagation_test.go` — `AllowDefault: true` with flag-visibility
  assertions. Survived the prototype.

That gap is itself a finding: the usage line for a default root is currently untested for exact
shape. Unit 4 closes it.

## Units

Each unit is RED → GREEN → REFACTOR, with a design-fit review after the refactor phase. Tests go
in `test/` (blackbox `package targ_test`) unless noted. New subtests are inserted in the existing
alphabetical position within their `t.Run` block.

### Unit 1 — Defect A, serial default path

**RED.** In `test/execution_properties_test.go`, add subtests asserting, via `targ.Execute` with a
single registered target:

- `["app", "marker"]` succeeds and the target ran (`Execute` returns no error; the target's
  side-effect counter is 1)
- `["app", "bogus"]` fails and `result.Output` names `bogus`
- a single root **group**: `["app", "grp", "s1"]` runs `s1`

Assert on `result.Output` content, never `errors.As` — `run_env.go`'s dispatch layer collapses
every propagated error into a bare `ExitError{Code: 1}` across ~16 call sites, so `errors.As`
after `targ.Execute()` is structurally impossible to pass. This is pre-existing, deliberate
convention.

Run the tests and confirm they fail with `unknown command: marker` / `unknown command: grp`.

**GREEN.** Add `stripDefaultRootName` and rewire `executeDefault` as designed above.

**REFACTOR + gate B.**

### Unit 2 — Defect A, parallel default path

**RED.** Add a subtest: single registered target, `["app", "--parallel", "marker"]` succeeds and
the target ran. Confirm it fails first with `[marker] Error: unknown command: marker`.

Also assert the corrected claim from Correction 1: with a single target whose function returns a
distinctive error, `["app", "--parallel", "marker"]` puts **that** error text in `result.Output`,
not `unknown command`.

**GREEN.** Route `executeDefaultParallel` through `stripDefaultRootName` per unit.

**REFACTOR + gate B.**

### Unit 3 — Defect B, bounded fan-out on both paths

**RED.** One upper-bound test per path — the multi-root path has none today.

Shape, mirroring `ParallelDepsRespectConcurrencyCap` (`:586`) and honouring the no-flaky-tests
rule: each target increments an atomic in-flight counter, CAS-updates a peak, then blocks on a
gate channel that a watcher closes once the peak reaches the expected cap; assert
`peak <= max(2, GOMAXPROCS/2)`. No wall-clock assertions. Each subtest gets its own fixtures and
its own counters — no shared mutable state across parallel subtests.

Use `n = 2*cap + 2` targets so the pre-fix violation margin is wide.

**This test's RED must be observed, not assumed.** An upper-bound assertion is observational: it
can pass by scheduling luck against unfixed code. Run it against the unfixed tree and confirm it
actually fails; if it does not fail on the first run, re-run until RED is observed, and if RED is
not reliably observable, widen `n` until it is. Record the observed pre-fix peak in the commit
message. A cap test that cannot be made to fail without the semaphore is not testing the
semaphore.

The measured pre-fix peaks (8 vs a cap of 5, both paths, real binaries) say the margin is real.

**GREEN.** Introduce `parallelUnit`, `resolveMultiRootUnits`, `runUnitsParallel`, `unitResult`,
`runUnit`, `collectUnitResults`, `reportUnitResults`; rewrite both parallel entry points as
resolver + call. Add the `runtime` import. **Delete both `//nolint:cyclop,funlen` directives and
add none** — lint returns `0 issues` without them.

**REFACTOR + gate B.** Gate B specifically checks that the result reads as one fan-out written
that way from the start, not two loops with a shared tail bolted on.

### Unit 4 — Defect C, help

**RED.** Two kinds of test:

- `internal/core` (whitebox, `package core_test`, via `PrintCommandHelpForTest`): a node with
  `IsDefaultRoot` true renders `[name]` in the usage line; false renders `name`. This closes the
  exact-shape gap the enumeration found.
- `test/` (blackbox): with one registered target, `["app", "--help"]` output contains both the
  bracketed usage line and both examples; **and** `["app", "marker", "--help"]` output contains
  the same. Both paths, because only one of them was fixed by the first design attempt.
- with two registered targets, `["app", "marker", "--help"]` output does **not** contain
  `[marker]`.

**GREEN.** `commandNode.IsDefaultRoot`; set it in `runWithEnvInternal`; bracket in
`printCommandHelp`; `TargetHelpOpts.DefaultRoot`; `withBareInvocation` + `basicUsageTitle` in
`internal/help`.

**REFACTOR + gate B.**

### Unit 5 — the queued-unit cancellation skip

The skip-check ships as part of Unit 3 (it is one of the two statements transplanted from
`runGroupParallel`, and shipping the semaphore without it would let a cancelled run start fresh
work).

Its *test* is separated here because its determinism is unproven. Investigate whether a
deterministic assertion exists: with cap `c` and `n > c` units where unit 0 fails immediately, the
queued units should return `ctx.Err()` and classify `CANCELLED` without running their bodies. If a
deterministic discriminator can be built (assert that fewer than `n` bodies executed *and* that at
least one result classified `CANCELLED`), write it. **If it cannot be made deterministic, do not
ship a non-discriminating test** — say so in the unit's report and record it as a known gap rather
than adding a test that passes whether or not the guard exists. No existing test asserts the
`CANCELLED` classification for this path on either the dep-group or CLI side.

### Unit 6 — docs

Apply every **update** row from the disposition table. Regenerate the README §Example Help Output
addition from real binary output rather than hand-writing it.

## Verification

- `targ check-full` must report **8/8**. Sequence it **after** the commit, or expect PASS on all
  legs *except* `check-uncommitted` — that leg fails by design on a dirty tree, so a pre-commit
  "expect green" gate is unsatisfiable as written.
- If `lint-full` fails, run `golangci-lint cache clean` and re-run before treating it as real.
- Re-run the four real-binary fixtures and confirm the peak-concurrency table's "after" column.
- Per-function coverage threshold is 80%. Every new function (`stripDefaultRootName`,
  `resolveMultiRootUnits`, `runUnitsParallel`, `runUnit`, `collectUnitResults`,
  `reportUnitResults`, `withBareInvocation`) needs coverage from the units above, not as cleanup.
  Read the uncovered lines rather than accepting the percentage — an uncovered error branch in a
  function whose siblings propagate is a defect, not a gap.
- `targ reorder-decls` for declaration ordering.

### Watch item

`test/execution_properties_test.go:1584` (`CollectAllErrorsPreservesDeclarationOrder`) enforces a
cross-goroutine ordering assumption with `time.Sleep(50ms)` inside `dep1` while `dep2` fails
instantly. It is the one sleep-based ordering-critical assertion in the parallel-execution tests.
This cycle adds concurrency-heavy tests to the same gate path, which is exactly the load change
that has starved it before. If it fails during `check-full` while passing solo, that is the cause
— fix by causality if it comes to that, never by lengthening the sleep.

## Out of scope

- `handleNoArgs`' **execution** path (`:753`). It is a second `hasDefault`-gated branch, but it
  only runs when there are no args at all, so it passes `nil` and has nothing to strip. Its help
  path *is* in scope (Unit 4). Deliberately excluded, not overlooked.
- `handleGlobalHelp`'s multi-root branch.
- `target.go`'s `runGroupParallel`/`runGroupParallelAll`.
- `<binary> <sub> --help` on a single-root-group module rendering the *group's* help instead of
  the subcommand's. Verified pre-existing at `2473fce` (baseline binary reproduces it), unchanged
  by this work.
- Unifying the two unknown-command output formats.
- `specs/001-parallel-output/contracts/api.md:119` — proposed above for Joe's call.

## Acceptance (from #40, with Correction 1 applied)

- [ ] A module registering exactly one target runs via `targ`, `targ <name>`, and `targ -p <name>`
- [ ] `targ <bogus>` on that module still fails, naming `bogus`
- [ ] `targ -p <name>` on a failing target prints that target's own error (today it prints the
      spurious `unknown command`; the error was never swallowed)
- [ ] Every invocation form `targ --help` advertises for a default root is executable, and both
      working forms are advertised
- [ ] Both parallel paths bound concurrent starts to `parallelCap(n, GOMAXPROCS)`, with a
      queued-unit skip on cancellation; one upper-bound test per path
- [ ] `ParallelFlagRunsTargetsInParallel` and `ParallelFlagWithSingleRootGroup` stay green,
      untouched
- [ ] `targ check-full` 8/8
- [ ] Default-target behaviour documented in the README and carried into `docs/specs/`

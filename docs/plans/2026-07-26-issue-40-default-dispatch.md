# Issue #40 — collapse the two CLI dispatch paths into one

Implementation plan, revision 2. Branch `issue-40-default-dispatch`, worktree
`.claude/worktrees/issue-40`, based on `2473fce`.

Issue #40 supersedes #24 and #34.

## What breaks for users today

A module that registers exactly one target:

```
$ targ            → MARKER-ONE          (works)
$ targ marker     → Error: unknown command: marker
$ targ -p marker  → [marker] Error: unknown command: marker
                    FAIL:1
```

`targ --help` prints `targ [targ flags...] marker` in its Usage line and `targ marker` under
Examples. Both are the invocation that fails. Adding a second registered target makes both work —
the trigger is the target *count*.

Separately, `targ -p a b c d e f g h` starts all eight at once regardless of core count, while a
dependency group of eight has been capped at `min(n, max(2, GOMAXPROCS/2))` since #26.

## Why this revision supersedes revision 1

Revision 1 (committed `f9cb6e1`) planned to *fix* the default-mode functions in place: teach
`executeDefault` to strip, teach both parallel loops to bound their fan-out. Gate A returned 19
findings across four angles, two of them Critical and both verified independently:

- The prescribed `explicit: !stripped` made `targ -p bogus` **silently succeed** (exit 0, wrong
  target runs) where it correctly fails today. Cause: `explicit=true` makes
  `parse.go`'s `trySubcommandOrUnknown` suppress `errUnknownCommand` and return the arg as
  leftover. Revision 1's prototype checked five positive cases and never passed a bad name.
- Revision 1's headline "measured: `0 issues`, no `//nolint` needed" was measured with the wrong
  linter. This repo has **no root `.golangci` config**, so a bare `golangci-lint run ./...` falls
  back to golangci-lint's small built-in default set. The project gate is
  `golangci-lint run -c dev/golangci-lint.toml` (`default = 'all'`), which is what `targ lint-full`
  invokes. Under the real config the same code produced `cyclop`/`lll`/`varnamelen` findings.

Shown those, Joe redirected: *"Simplify. I think this whole set of errors increasingly shows that
the default command is subtly complicated. Let's just remove the errors by removing the feature."*

Removing the feature outright was measured first: forcing `hasDefault = false` at `2473fce` breaks
**102 subtests** — not only dispatch tests but `TestFloat64FlagParsing`,
`TestProperty_EnvVarBehavior`, `TestProperty_CommandHelp`, `TestParallelOutputShellCommand`,
positional parsing — because default mode is the substrate the suite uses to exercise a single
target without naming a command. It also breaks the public `targ.RunOptions.AllowDefault` and
`targ.Main`'s dedicated-binary story (README Stage 3). Presented with that, Joe chose the
alternative that serves the same goal: **collapse the two dispatch implementations into one.**

That is the plan below. It deletes more of the subtle code than revision 1 did, and every defect
#40 reports falls out of the collapse rather than being patched.

## Baseline (measured)

`targ check-full` in this worktree at `2473fce`: **8/8 PASS**.

The first run reported `lint-full` FAIL with 51 findings; `golangci-lint cache clean` + re-run gave
`0 issues` and a full 8/8. That was the stale-result-cache phantom CLAUDE.md documents. **Clean the
lint cache before trusting a baseline in a fresh worktree.**

## Verified corrections to issue #40's text

Checked against the code and against real binaries. #40 is accurate except:

1. **`-p` does NOT swallow the error.** #40 says the `-p` form "reports a failed target with **no
   error text at all**". Measured: `targ -p marker` prints
   `[marker] Error: unknown command: marker` — `run_env.go:384-386` prints it explicitly. The
   error was never swallowed. What is wrong is that the error is the *spurious* `unknown command`,
   because the target never runs. Acceptance bullet 3 is therefore **not** satisfied today and is
   **not** dropped: it becomes true only once dispatch is fixed, and Unit 2 verifies it by
   asserting a target's own distinctive error reaches `result.Output` under `-p`.

2. **Defect A also bites a GROUP root.** #40's examples show only a plain-target root. Measured:
   `repro40grp grp s1` and `repro40grp grp` both give `Unknown command: grp`.

3. **`executeMultiRootParallel` is at `:493`, not `:492`** — `:492` is its `//nolint` line. Every
   other line number in #40 is exact.

4. **`hasDefault` is read in three *functions*** — `handleGlobalHelp` (`:723` and `:732`),
   `handleNoArgs` (`:753`), `runWithEnvInternal` (`:1111`) — four textual occurrences. Only the
   `runWithEnvInternal` site reaches the four functions #40 tables.

5. **`printCommandHelp` has a fourth call site** #40 does not mention: `export_test.go:16` aliases
   it as `PrintCommandHelpForTest` (`command_test.go:128`, `:144`).

## The collapse

### Settled — do not relitigate in-task

- Fix child-side in `internal/core`, not the `internal/runner` wrapper (#40's reasoning holds:
  `targ.Execute`/`targ.Run` reach this code with no wrapper involved).
- Top-level and per-dep-group caps stay independent (per-group budgets, matching #26).
- `target.go`'s `runGroupParallel`/`runGroupParallelAll` are **not** folded in. They are a third
  instance of the fan-out shape but operate on `[]*Target` with `onStart`/`onStop` hooks one layer
  down. They remain the reference pattern this code mirrors.
- The default-target *feature* stays. Only its duplicate *implementation* goes. (Considered
  removing the feature; measured at 102 broken subtests plus a public API break; Joe chose the
  collapse instead.)

### Serial — delete `executeDefault`

`runWithEnvInternal` prepends the sole root's own name, then always calls `executeMultiRoot`:

```go
	if exec.hasDefault && !exec.opts.Overrides.Parallel &&
		!strings.EqualFold(exec.rest[0], exec.roots[0].Name) {
		exec.rest = append([]string{exec.roots[0].Name}, exec.rest...)
	}

	return exec.executeMultiRoot()
```

`exec.rest` is non-empty here — the zero-arg case went to `handleNoArgs` earlier, and
`handleSpecialCommands` already indexes `e.rest[0]`.

`executeMultiRoot` stops hardcoding `true` for `executeWithParents`' `explicit` parameter and
passes `!e.hasDefault` instead.

**That parameter is the entire semantic difference between the two old paths**, and getting it
wrong is what broke revision 1. In `parse.go`'s `trySubcommandOrUnknown` (`:139-155`), an arg
matching no positional and no subcommand is an `errUnknownCommand` when `explicit` is false, but
is returned as leftover `remaining` when it is true — so the caller can try it as the next root.
That leftover behaviour is *chaining*, which only makes sense with more than one root. With
`explicit=true` a single-root module would run its target and only then complain about a bad
trailing arg; with `explicit=false` it reports the error without running anything, matching
today's behaviour exactly.

### Parallel — delete `executeDefaultParallel`

One resolver and one bounded fan-out, shared by both modes.

```go
// parallelUnit is one CLI target scheduled by runUnitsParallel. It resolves from
// an explicit root/glob match, or - in default mode - a subcommand name to run
// against the sole root.
type parallelUnit struct {
	node          *commandNode
	name          string
	args          []string
	explicit      bool
	checkProgress bool
}
```

`resolveUnits` maps each non-flag arg to a unit — glob expansion, then explicit root match, then
(default mode only) a subcommand of the sole root, else an unknown-command usage error:

```go
		if matched := e.findMatchingRoot(arg); matched != nil {
			units = append(units, parallelUnit{node: matched, name: arg, explicit: true})
			continue
		}

		if e.hasDefault {
			units = append(units, parallelUnit{
				node:          e.roots[0],
				name:          arg,
				args:          []string{arg},
				explicit:      false,
				checkProgress: true,
			})

			continue
		}

		e.env.Printf("Unknown command: %s\n", arg)
		printUsage(e.env.Stdout(), e.roots, e.opts)

		return nil, ExitError{Code: 1}
```

The serial prepend is gated on `!Overrides.Parallel` because parallel mode resolves each arg to
its own unit rather than to one chained arg list.

`startUnits` carries the semaphore and the cancellation skip, transplanted from `runGroupParallel`:

```go
	capN := parallelCap(len(units), runtime.GOMAXPROCS(0))
	sem := make(chan struct{}, max(1, capN))

	for i, unit := range units {
		results[i].Name = unit.name

		go func(idx int, unit parallelUnit) {
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				resultCh <- unitResultMsg{index: idx, err: ctx.Err()}
				return
			}
			...
			next, err := unit.node.executeWithParents(
				tctx, unit.args, nil, map[string]bool{}, unit.explicit, e.opts)
			// A default-mode unit that consumed nothing named no real subcommand;
			// the serial path reports that, so the parallel path must too.
			if err == nil && unit.checkProgress &&
				len(unit.args) > 0 && len(next) == len(unit.args) {
				err = fmt.Errorf("%w: %s", errUnknownCommand, unit.args[0])
			}
```

Acquire precedes the `starting...` print, so a queued unit does not announce itself before it
runs. `runUnitsParallel` splits into `startUnits` / `collectUnitResults` / `printUnitResults` to
stay under the complexity limits **without any `//nolint`**.

### Help (#40 Defect C) — advertise both working forms

The collapse makes `targ <name>` work, so the existing help text stops lying. It still never
mentions that the *bare* form works. `printCommandHelp` gets one node and `RunOptions`; it cannot
tell it is rendering the sole root.

**Threading `isDefaultRoot` as a parameter does not work** — there are two help paths and only one
passes through a caller that knows `hasDefault`:

| invocation | reaches `printCommandHelp` via |
|---|---|
| `targ <name> --help` | `handleGlobalHelp` (`:732`), which knows `hasDefault` |
| `targ --help` | `extractHelpFlag` empties args → `handleNoArgs` → `roots[0].execute` → the generic `HelpOnly` interception at `command.go:157-161`, which knows nothing |

A prototype threading the parameter rendered correctly for `targ <name> --help` and had **no
effect at all** on plain `targ --help`, the more common invocation. Recorded because it looks
right on paper.

So the signal rides on the node, set once where `hasDefault` is computed
(`exec.roots[0].IsDefaultRoot = true`). `parseTargets` builds fresh nodes each run, so this is
per-execution state, not a global — confirmed by a test reusing one `*targ.Target` across two
`Execute` calls, sole-root then not. All four `printCommandHelp` call sites, including
`PrintCommandHelpForTest`, then get the right answer with no signature change.

`printCommandHelp` brackets the name (`usageParts[0] = "[" + usageParts[0] + "]"` — **before** the
`append([]string{opts.BinaryName, "[targ flags...]"}, ...)`, or it brackets the binary name
instead) and forwards `DefaultRoot: node.IsDefaultRoot` into `help.TargetHelpOpts`.
`WriteTargetHelp` prepends the bare example when generating; a caller-supplied `opts.Examples`
list still wins untouched.

**Budget for this:** adding the branch pushes the *existing* `WriteTargetHelp` over `cyclop` under
the real config. Extract the examples selection into a helper (`targetHelpExamples`). Adding a
`//nolint` instead is not an option.

Rendered result (fixture with one target `marker`, binary `bin`):

```
Usage:
  bin [targ flags...] [marker]

Examples:
  Basic usage:
    bin
  By name:
    bin marker
```

## Intended behaviour changes

Beyond fixing #40's three defects, unifying the paths changes four things. All are consequences of
having one implementation instead of two, all were measured, and all are intended.

| case | before | after |
|---|---|---|
| `targ grp s1`, `targ grp` (sole group root) | `Unknown command: grp` | runs; naming the sole root works like any other root — this **is** Defect A for group roots |
| `targ -p <bogus>` (sole group root) | **silently PASSES**, exit 0, nothing runs | `Error: unknown command: bogus`, `FAIL:1`, exit 1 |
| `targ <bogus>` (sole group root) | `Unknown command: bogus`, no usage block | same line and exit code, **plus** a usage block |
| `targ -p <name>` on a failing target | prints the spurious `unknown command` | prints the target's own error |

The third is a pure output addition on an already-failing path, and matches what multi-root mode
has always printed. Plain-`Func` sole roots are unaffected (they raise a real error on the first
pass). Two-target modules are **byte-identical** before and after in every case tested, including
help.

## Prototype verification (run before this plan was committed)

Applied in a throwaway worktree (`.claude/worktrees/issue-40-proto2`, detached at `2473fce`).
Nothing below is a prediction. `GOMAXPROCS=10`; `parallelCap(8, 10) = 5`.

| check | command | result |
|---|---|---|
| build | `go build ./...` | clean |
| tests | `go test ./...` | all packages ok |
| race | `go test -race ./internal/core/... ./test/...` | clean |
| **lint (real config)** | `golangci-lint cache clean && golangci-lint run -c dev/golangci-lint.toml ./...` | **`0 issues`** |
| nolint count | `grep -c nolint internal/core/run_env.go` | **6 → 4**, none added |
| size | `git diff --stat` | **235 insertions, 307 deletions — net −72 lines** |
| `targ check-full` | in the dirty worktree | 7/8; only `check-uncommitted` fails, by design |
| `repro40` | bare / `marker` / `-p marker` | `MARKER-ONE` / `MARKER-ONE` / `PASS:1` |
| `repro40` | `bogus` / `-p bogus` | `Error: unknown command: bogus` exit 1, target does **not** run / `FAIL:1` exit 1 |
| `repro40grp` | `-p bogus` | `FAIL:1` exit 1 (baseline wrongly passed) |
| `repro40cap` | `-p t1…t8` | peak **8 → 5** |
| `repro40grp` | `-p s1…s8` | peak **8 → 5** |
| `repro40two` | every case incl. `marker --help` | byte-identical to baseline |
| **kill-switch** | delete `sem` + acquire/release, re-measure ×5 each | peak returns to **8, 8, 8, 8, 8** on both fixtures; restored, back to 5 |

The kill-switch matters: it proves the semaphore is the sole thing bounding concurrency and that
no runtime or stdlib limit silently backstops it, so a cap test has real teeth.

The prototype worktree is deleted before this plan's commit. Execution re-derives the code through
TDD rather than transplanting the diff.

## Doc-surface enumeration

Two invariants change: (1) a module registering exactly one target gets a default that runs bare,
by name, and under `-p`; (2) CLI `--parallel`/`-p` fan-out is bounded by
`parallelCap(n, GOMAXPROCS)`.

Commands run at plan-write time. **Unit 6 re-runs them** and confirms every hit is dispositioned —
they are a verification step, not just documentation:

```bash
grep -rniI --exclude-dir=.git -E "default target|default root|default command|defaults to" .
grep -rniI --exclude-dir=.git -E "single target|one target|lone target|only target|sole target" .
grep -rniI --exclude-dir=.git -E "bare targ|no arguments|no args\b|without arguments" .
grep -rniI --exclude-dir=.git -E "default-target|defaulttarget|AllowDefault|hasDefault" .
grep -rniI --exclude-dir=.git -E "all at once|simultaneous|concurrently|no limit|unbounded" .
grep -rniI --exclude-dir=.git -E "fan-out|fanout|GOMAXPROCS|parallelCap|concurrency cap" .
grep -rniI --exclude-dir=.git -E -- "--parallel" README.md CLAUDE.md docs/ specs/ projects/ .claude/
grep -rn "Usage:" . --include="*.go" --include="*.md" --include="*.golden"
grep -rnE "go func\(|go [a-z][A-Za-z]*\(" . --include="*.go"   # method-call goroutines too
```

The last grep's second alternation exists because `internal/core/printer.go:23` launches a
goroutine as `go p.run()` — a bare `go func(` search misses that form. It is one printer goroutine
per parallel call across all four call sites, not a fan-out; the "no fifth uncapped fan-out site"
conclusion holds, now by a search that could have found one.

| file | line(s) | disposition | reason |
|---|---|---|---|
| `README.md` | 319 §Command Names | **update** | Home for the default-target rule: bare, by name, and `-p <name>` all run the sole target |
| `README.md` | 474-493 §Example Help Output | **update** | Add a single-target example. The existing block is *already* stale independent of this issue (it shows `Usage: build [flags]`; real output is `Usage:\n  <bin> [targ flags...] build`) — regenerate from a real binary, do not copy its format |
| `README.md` | 708 `--parallel` row | **update** | Add the bound, mirroring what :350 does for dep groups |
| `README.md` | 350 | keep | Correctly scoped to dep groups; the :708 update carries the cross-reference |
| `README.md` | 186 §Execution Control | keep | Zero hits; a landmark in #40's text only |
| `docs/specs/use-cases.md` | 5-13 UC-1 | **update** | Silent on the optional-command case |
| `docs/specs/requirements.md` | 173-178 DES-5 | **update** | Scoped to `.Deps()`; add the CLI `-p` fan-out using the identical formula |
| `docs/specs/requirements.md` | 19-24 REQ-3 | **update** | Add the default-dispatch rule |
| `docs/specs/architecture.md` | 19-24 ARCH-3 | **update** | No ARCH item names the `hasDefault` mechanism; ARCH-3 owns command resolution |
| `docs/specs/tests.md` | 32-53 T-3 | **update** | Two property bullets: `targ <name>` succeeds with one registered target; top-level `-p` fan-out never exceeds `parallelCap` |
| `docs/specs/state.toml` | — | **no edit** | Extending existing IDs (REQ-3/DES-5/ARCH-3/T-3/UC-1) is exactly what #26 (`fdd0984`) and #32 (`ac2542e`) did, and **neither touched `state.toml`**. Every `history` note in that file coincides with *minting* a new ID. Revision 1 instructed a history note here, citing a #26/#32 precedent that does not exist — dropped |
| `internal/core/types.go` | `AllowDefault` | **update** | No doc comment at all, on the field controlling the invariant, exported via `targ.RunOptions`; sibling `BinaryMode` is documented |
| `internal/core/target.go` | 767-772 `parallelCap` | **update** | Comment says "in a parallel **dep group**"; the CLI path now calls it too |
| `internal/core/override.go` | 27 `Parallel bool` | **update** | Public GoDoc via `targ.RuntimeOverrides`; add the bound |
| `targ.go` | 152-249 | **update** | `Execute`/`ExecuteRegistered`/`Main`/`Register` never mention the default target — the pkg.go.dev-facing gap |
| `internal/flags/flags.go` | 71-76 `--parallel` `Desc` | keep | Rendered inside every `--help`; the formula would bloat the flag table. The bound belongs in README §Runtime Flags |
| `internal/core/execute_test.go` | 288-320 | **verify, no edit expected** | Runs `ExecuteWithResolution` with `AllowDefault: true` on a single-target registry — real overlap with the rewired dispatch. Revision 1 missed it. Confirm it still passes and say why |
| `test/completion_properties_test.go` | 352, 892 | **verify, no edit expected** | `HandlesSingleTargetMode` / `SingleRootCompletionWithRemainingArgs` exercise `__complete` in single-root mode; `targ.Execute` always sets `AllowDefault: true`. Real exposure to the dispatch change |
| `test/validation_properties_test.go` | 12-65 | **verify** | `TestProperty_UsageLine`, single target, `--help`; reaches the bracketed usage line |
| `test/hierarchy_properties_test.go` | 416-431 | **verify** | `HelpInDefaultModeShowsTargetHelp`, same path |
| `internal/core/binary_mode_propagation_test.go` | 31, 72, 104, 128 | **verify** | `AllowDefault: true` help rendering |
| `internal/help/render.go`, `render_test.go` | `"Usage:"` emission | **verify, no edit expected** | The literal string Defect C is about. Assertions are substring/ordering, not exact-match |
| `internal/runner/runner_help_test.go` | `validateHelpOutput` | **verify** | Builds and runs real binaries via `handleSingleModule`, through the modified core/help code |
| `internal/runner/testdata/golden/*.golden` | all | N/A | `--create`/`--sync`/`--to-func`/`--to-string` meta-command help; static literals in `runner.go`, unreachable from `commandNode` rendering |
| `internal/runner/runner.go` | 3906 | N/A | `printMultiModuleHelp` only fires when `len(moduleGroups) > 1` |
| `docs/archive/**` | many | N/A | Not by an "archive convention" — no such rule is written anywhere in this repo (revision 1 asserted one; it exists only in a self-citing chain of three plans). The empirical fact: `git log ac98256..HEAD -- docs/archive/` returns **zero commits** since the 2026-03-08 archiving. Nothing has ever been retro-edited |
| `docs/plans/2026-07-23-issue-26-dep-fanout-cap.md` | 33, 918 | N/A — cite, don't edit | Names this exact CLI fan-out as deferred out-of-scope for #26. Load-bearing prior art |
| `specs/001-parallel-output/contracts/api.md` | 119 | **propose, do not fold** | "still executes targets concurrently", now incomplete. Grounds for deferring: `specs/001-parallel-output/` has had **exactly one commit touch it, ever**. (Revision 1 called it "closed"; `spec.md:5` self-reports `Status: Draft`, so that label was wrong even though the disposition is right.) For Joe's yes/no |
| false positives, verified by inspection | `internal/core/{command,execute,registry,result,override,target}.go`, `internal/help/builder.go`, `test/{constraints,hierarchy_fuzz,overrides,shell}_properties_test.go`, `projects/portable-targets/**`, `session-learning-prompt.md`, `.claude/skills/**`, `.claude/commands/**`, `specs/001-parallel-output/**` (other files), `docs/plans/**` (others), `README.md:293`, `CLAUDE.md:40`, `docs/specs/tests.md:9` | N/A | Each is a different sense of "default" / "one target" / "at once" / "concurrent", or historical record. Enumerated rather than summarised, so a reviewer can tell a classified hit from an unrun grep |

**Zero-hit surfaces** (searched, genuinely absent — so absence is distinguishable from
not-searched): no C4 or architecture-diagram directory; no `CHANGELOG.md`; no `.traced/config.toml`
(traced state lives solely in `docs/specs/state.toml`); no package comment in
`internal/core/doc.go`; no single-target `Main()` example and no `--parallel` prose under
`examples/`.

## Units

Each unit is RED → GREEN → REFACTOR, with a design-fit review after the refactor phase. Blackbox
tests go in `test/` (`package targ_test`); new subtests are inserted in their block's existing
alphabetical position.

Assert on `result.Output`, never `errors.As`, after `targ.Execute` — this dispatch layer collapses
every propagated error into a bare `ExitError{Code: 1}` across ~16 call sites, so `errors.As` is
structurally impossible to pass. Pre-existing, deliberate convention.

### Unit 1 — serial collapse

**RED.** Single registered target: `["app","marker"]` runs it; `["app","bogus"]` fails naming
`bogus` **and the target does not run**; sole group root `["app","grp","s1"]` runs `s1`. Confirm
each fails first with `unknown command`.

**GREEN.** Prepend in `runWithEnvInternal`; `explicit: !e.hasDefault` in `executeMultiRoot`;
delete `executeDefault`.

**REFACTOR + gate B.**

### Unit 2 — parallel collapse

**RED.** `["app","--parallel","marker"]` runs the target. `["app","--parallel","bogus"]` **fails**
— on both a sole plain root and a sole group root; the group case is the pre-existing silent-pass
bug. A single target whose function returns a distinctive error puts **that** text in
`result.Output` under `-p`, not `unknown command`.

**GREEN.** `parallelUnit`, `resolveUnits`, `runUnitsParallel` + `startUnits` /
`collectUnitResults` / `printUnitResults`; delete `executeDefaultParallel`; add the `runtime`
import; **delete both `//nolint:cyclop,funlen` directives (`run_env.go:280` and `:492`) and add
none.**

**REFACTOR + gate B.** Gate B checks the result reads as one fan-out written that way from the
start.

### Unit 3 — the concurrency cap

**RED.** One upper-bound test per mode (default and multi-root); the multi-root path has none
today. Each target increments an atomic in-flight counter and CAS-updates a peak, then blocks on a
gate a watcher closes once the peak reaches the expected cap; assert
`peak <= max(2, GOMAXPROCS/2)`. No wall-clock assertions. Each subtest gets its own fixtures and
counters. Use `n = 2*cap + 2` so the pre-fix margin is wide.

**Observe the RED, do not assume it.** An upper-bound assertion can pass by scheduling luck. Run
it against the uncapped tree and confirm it fails; re-run until RED is observed; widen `n` if it
is not reliably observable. The kill-switch measurement (peak returns to 8/8 on five runs of each
fixture with the semaphore deleted) says the margin is real.

**GREEN.** The semaphore and `ctx.Err()` skip ship with Unit 2; this unit adds their tests.

**REFACTOR + gate B.**

### Unit 4 — help advertises both forms

**RED.**
- whitebox (`internal/core`, via `PrintCommandHelpForTest`): `IsDefaultRoot` true renders
  `[name]`, false renders `name`. Closes the exact-shape gap — no existing test asserts the full
  default-root usage line.
- blackbox: one registered target, **both** `["app","--help"]` and `["app","marker","--help"]`
  contain the bracketed usage line and both examples.
- two registered targets: `["app","marker","--help"]` does **not** contain `[marker]`.
- `IsDefaultRoot` does not leak: reuse one `*targ.Target` across two `Execute` calls, sole-root
  then not.

**GREEN.** `commandNode.IsDefaultRoot`; set it in `runWithEnvInternal`; bracket in
`printCommandHelp`; `TargetHelpOpts.DefaultRoot`; bare-example prepend in `WriteTargetHelp`;
extract `targetHelpExamples` to keep `WriteTargetHelp` under `cyclop`.

**REFACTOR + gate B.**

### Unit 5 — queued-unit cancellation skip

The skip ships with Unit 2 (omitting it would let a cancelled run start fresh work). Its *test*
is separated because determinism is unproven. Investigate whether a deterministic discriminator
exists: with cap `c` and `n > c` units where unit 0 fails immediately, assert fewer than `n`
bodies ran **and** at least one result classified `CANCELLED`. **If it cannot be made
deterministic, do not ship a non-discriminating test** — record it as a known gap in the unit's
report. No existing test asserts `CANCELLED` on either the dep-group or CLI side.

### Unit 6 — docs

Re-run the enumeration greps first and confirm every hit is dispositioned. Then, by file:

1. `README.md` — :319 default-target rule; :708 the cap bound; :474-493 regenerate the help
   example from a real binary's output.
2. `docs/specs/use-cases.md` UC-1; `requirements.md` REQ-3 and DES-5; `architecture.md` ARCH-3;
   `tests.md` T-3 (two bullets). No `state.toml` edit.
3. Doc comments: `types.go` `AllowDefault`; `override.go` `Parallel`; `target.go` `parallelCap`;
   `targ.go` `Execute`/`ExecuteRegistered`/`Main`/`Register`.
4. Run the **verify, no edit expected** rows and record what was checked.

## Verification

- `targ check-full` must report **8/8**. Sequence it **after** the commit, or expect PASS on every
  leg *except* `check-uncommitted`, which fails by design on a dirty tree.
- **Lint with the project's config**: `targ lint-full`, or
  `golangci-lint run -c dev/golangci-lint.toml ./...`. A bare `golangci-lint run ./...` uses a
  weaker default set and is not a valid measurement — this repo has no root config.
- If `lint-full` fails, `golangci-lint cache clean` and re-run before treating it as real.
- Re-run the four fixtures and confirm the before/after table.
- Per-function coverage is 80%. Every new function needs coverage from the Units' own tests, not
  as cleanup — measured on the prototype, the design snippets alone leave `stripDefaultRootName`,
  `resolveUnits` and the help helper under the bar until the Unit tests are written. Read the
  uncovered lines rather than accepting the percentage.
- `targ reorder-decls` — both touched files needed reordering in the prototype.

### Watch item

`test/execution_properties_test.go:1584` (`CollectAllErrorsPreservesDeclarationOrder`) enforces a
cross-goroutine ordering assumption with `time.Sleep(50ms)` while `dep2` fails instantly. It is
the one sleep-based ordering-critical assertion in the parallel tests, and this cycle adds
concurrency-heavy tests to the same gate path — the load change that has starved it before. If it
fails under `check-full` while passing solo, that is the cause. Fix by causality, never by
lengthening the sleep.

## Out of scope

- `handleNoArgs`' execution path. It runs only when there are no args, passes `nil`, and has
  nothing to strip. Its *help* path is in scope (Unit 4). Deliberately excluded.
- `target.go`'s `runGroupParallel`/`runGroupParallelAll`.
- **`<binary> <sub> --help` on a sole group root renders the group's help, not the subcommand's.**
  Verified pre-existing at `2473fce`; the collapse does not repair it. Raised with Joe during
  planning; not resolved there because the redirect superseded the question. **To be filed as a
  separate issue at close** — not silently dropped.
- `specs/001-parallel-output/contracts/api.md:119` — proposed above for Joe's call.
- Removing the default-target feature (measured: 102 broken subtests plus a public API break; Joe
  chose the collapse instead).

## Acceptance

- [ ] A module registering exactly one target runs via `targ`, `targ <name>`, and `targ -p <name>`
- [ ] `targ <bogus>` on that module still fails, naming `bogus`, without running the target
- [ ] `targ -p <bogus>` fails on both a sole plain root and a sole group root
- [ ] `targ -p <name>` on a failing target prints that target's own error
- [ ] Every invocation form `targ --help` advertises for a default root is executable, and both
      working forms are advertised
- [ ] Both parallel modes bound concurrent starts to `parallelCap(n, GOMAXPROCS)`, with a
      queued-unit skip on cancellation; one upper-bound test per mode
- [ ] `ParallelFlagRunsTargetsInParallel` and `ParallelFlagWithSingleRootGroup` stay green,
      untouched
- [ ] `executeDefault` and `executeDefaultParallel` no longer exist; both `//nolint:cyclop,funlen`
      directives are gone and no new ones added
- [ ] `targ check-full` 8/8
- [ ] Default-target behaviour documented in the README and carried into `docs/specs/`

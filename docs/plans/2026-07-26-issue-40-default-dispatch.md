# Issue #40 — collapse the two CLI dispatch paths into one

Implementation plan, revision 3. Branch `issue-40-default-dispatch`, worktree
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
Examples — both are the invocation that fails — and it never mentions that the bare form works,
which is the only form that does. Nothing in the README documents the default-target case at all.
Adding a second registered target makes the named forms work; the trigger is the target *count*.

Separately, `targ -p a b c d e f g h` starts all eight at once regardless of core count, while a
dependency group of eight has been capped at `min(n, max(2, GOMAXPROCS/2))` since #26.

## Global constraints

- Go 1.25.5. Use `targ` for all build/test/check operations, never bare `go test` as final
  validation.
- **Lint is measured with the project config**: `targ lint-full`, or
  `golangci-lint run -c dev/golangci-lint.toml ./...`. There is **no root `.golangci` config**, so
  a bare `golangci-lint run ./...` silently falls back to a weak built-in linter set and is not a
  valid measurement.
- No new `//nolint` directives. The count in `run_env.go` must go **down** (6 → 4), not up. A
  tripped limit means refactoring, never suppression.
- No new package-level mutable state.
- `errors.As` after `targ.Execute` is structurally impossible to pass — this dispatch layer
  collapses every propagated error into a bare `ExitError{Code: 1}` across ~16 call sites.
  Assert on `result.Output`.
- No timing-dependent assertions. Parallel subtests get their own fixtures; never share mutable
  state across them.
- Declaration order is alphabetical within type/var/func groups — `targ reorder-decls` fixes it.

## Design decisions (settled; do not relitigate in-task)

1. Fix child-side in `internal/core`, not the `internal/runner` wrapper. `targ.Execute`/`targ.Run`
   reach this code with no wrapper involved.
2. Top-level and per-dep-group caps stay independent — per-group budgets, matching #26. No shared
   global budget.
3. `target.go`'s `runGroupParallel`/`runGroupParallelAll` are **not** folded in. They are a third
   instance of the fan-out shape, but operate on `[]*Target` with `onStart`/`onStop` hooks one
   layer down. They remain the reference pattern this code mirrors.
4. The default-target *feature* stays; only its duplicate *implementation* goes. Removing the
   feature was measured first — 102 broken subtests plus a public API break on
   `targ.RunOptions.AllowDefault` and `targ.Main`'s dedicated-binary story — and Joe chose the
   collapse instead.
5. The collapse trades duplicated code for call-graph indirection: two ~90-line near-identical
   goroutine loops become one pipeline reached through six names (`parallelUnit`, `resolveUnits`,
   `runUnitsParallel`, `startUnits`, `collectUnitResults`, `printUnitResults`). Fewer duplicated
   lines and zero suppressions, but a reader tracing parallel dispatch now hops between functions
   instead of reading one top to bottom. The decomposition is forced by the complexity gate
   applying to the now-unduplicated pipeline; accepted deliberately.

## Why this revision supersedes revisions 1 and 2

**Revision 1** (`f9cb6e1`) planned to *fix* the default-mode functions in place. Gate A round 1
returned 19 findings, two Critical, both verified independently:

- The prescribed `explicit: !stripped` made `targ -p bogus` **silently succeed** (exit 0, wrong
  target runs) where it correctly fails today. `explicit=true` makes `parse.go`'s
  `trySubcommandOrUnknown` suppress `errUnknownCommand` and return the arg as leftover. Rev 1's
  prototype checked five positive cases and never passed a bad name.
- Rev 1's headline "measured `0 issues`, no `//nolint` needed" used a bare
  `golangci-lint run ./...` — the wrong linter set, per Global constraints above. Under the real
  config the same code produced `cyclop`/`lll`/`varnamelen` findings.

Shown those, Joe redirected: *"Simplify. I think this whole set of errors increasingly shows that
the default command is subtly complicated. Let's just remove the errors by removing the feature."*
Removing it outright was measured (102 subtests, public API break) and Joe then chose the collapse.

**Revision 2** (`7beccbc`) wrote that collapse. Gate A round 2 returned 23 findings across four
angles. The design survived — an executing reviewer implemented it independently and reached green
build, tests, race, lint, and 7/8 `check-full` — but the plan's *prose* carried several false
claims, three of which were reviewer assertions from round 1 that revision 2 restated without
re-verifying (a `handleSingleModule` rationale that does not exist in the cited file, an
`execute_test.go` "overlap" with code the test never reaches, and a `CLAUDE.md:40` grep hit that
the plan's own listed commands do not produce). Revision 3 corrects all of them and adds the
behavioural consequences the round-2 executing reviewer found by probing cases neither previous
revision enumerated.

## Baseline (measured)

`targ check-full` in this worktree at `2473fce`: **8/8 PASS**.

The first run reported `lint-full` FAIL with 51 findings; `golangci-lint cache clean` + re-run gave
`0 issues` and a full 8/8. That was the stale-result-cache phantom CLAUDE.md documents. **Clean the
lint cache before trusting a baseline in a fresh worktree.**

## Verified corrections to issue #40's text

Checked against the code and against real binaries. #40 is accurate except:

1. **`-p` does NOT swallow the error.** #40 says the `-p` form "reports a failed target with **no
   error text at all**". Measured: `targ -p marker` prints
   `[marker] Error: unknown command: marker` — `run_env.go:384-386` prints it explicitly. What is
   wrong is that the error is the *spurious* `unknown command`, because the target never runs.
   Acceptance bullet 3 is therefore **not** satisfied today and is **not** dropped: it becomes
   true only once dispatch is fixed, and Unit 2 verifies it.

2. **Defect A also bites a GROUP root.** #40's examples show only a plain-target root. Measured:
   `repro40grp grp s1` and `repro40grp grp` both give `Unknown command: grp`.

3. **`executeMultiRootParallel` is at `:493`, not `:492`** — `:492` is its `//nolint` line. Every
   other line number in #40 is exact.

4. **`hasDefault` is read in three *functions*** — `handleGlobalHelp` (`:723`, `:732`),
   `handleNoArgs` (`:753`), `runWithEnvInternal` (`:1098`, `:1111`) — four textual occurrences.

5. **`printCommandHelp` has a fourth call site** #40 does not mention: `export_test.go:16` aliases
   it as `PrintCommandHelpForTest` (`command_test.go:128`, `:144`).

## The collapse

### Serial — delete `executeDefault`

`runWithEnvInternal` prepends the sole root's own name, then always calls `executeMultiRoot`:

```go
	// A default root is addressable by name like any other root, so hand the
	// one dispatch path an arg list that already names it.
	if exec.hasDefault && !exec.opts.Overrides.Parallel &&
		!strings.EqualFold(exec.rest[0], exec.roots[0].Name) {
		exec.rest = append([]string{exec.roots[0].Name}, exec.rest...)
	}

	return exec.executeMultiRoot()
```

`exec.rest` is non-empty here — the zero-arg case went to `handleNoArgs` earlier, and
`handleSpecialCommands` already indexes `e.rest[0]`.

`executeMultiRoot` stops hardcoding `true` for `executeWithParents`' `explicit` parameter and
passes `!e.hasDefault`.

**That parameter is the entire semantic difference between the two old paths**, and getting it
wrong is what broke revision 1. In `trySubcommandOrUnknown` (`parse.go:139-155`), an arg matching
no positional and no subcommand is an `errUnknownCommand` when `explicit` is false, but is
returned as leftover `remaining` when it is true, so the caller can try it as the next root. That
leftover behaviour is *chaining*, which only makes sense with more than one root. With
`explicit=true` a single-root module would run its target and only then complain about a bad
trailing arg; with `explicit=false` it reports the error without running anything.

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
func (e *runExecutor) resolveUnits() ([]parallelUnit, error) {
	units := make([]parallelUnit, 0, len(e.rest))

	for _, arg := range e.rest {
		if strings.HasPrefix(arg, "-") {
			continue
		}

		if isGlobPattern(arg) {
			matches := e.findMatchingRootsGlob(arg)
			if len(matches) == 0 {
				e.env.Printf("No targets match pattern: %s\n", arg)
				return nil, ExitError{Code: 1}
			}

			for _, matched := range matches {
				units = append(units, parallelUnit{node: matched, name: matched.Name, explicit: true})
			}

			continue
		}

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
	}

	return units, nil
}
```

The serial prepend is gated on `!Overrides.Parallel` because parallel mode resolves each arg to
its own unit rather than to one chained arg list.

`startUnits` carries the semaphore and the cancellation skip, transplanted from `runGroupParallel`:

```go
func (e *runExecutor) startUnits(
	ctx context.Context,
	units []parallelUnit,
	results []TargetResult,
	resultCh chan<- unitResultMsg,
	printer *Printer,
	maxNameLen int,
) {
	sem := make(chan struct{}, max(1, parallelCap(len(units), runtime.GOMAXPROCS(0))))

	for i, unit := range units {
		results[i].Name = unit.name

		go func(idx int, unit parallelUnit) {
			sem <- struct{}{}
			defer func() { <-sem }()

			// Skip queued units once the run is canceled (fail-fast):
			// starting fresh work after a sibling failed would defeat the mode.
			if ctx.Err() != nil {
				resultCh <- unitResultMsg{index: idx, err: ctx.Err()}
				return
			}

			tctx := WithExecInfo(ctx, ExecInfo{
				Parallel:   true,
				Name:       unit.name,
				MaxNameLen: maxNameLen,
				Printer:    printer,
				Output:     e.opts.Stdout,
			})

			Print(tctx, "starting...\n")

			start := time.Now()

			next, err := unit.node.executeWithParents(
				tctx, unit.args, nil, map[string]bool{}, unit.explicit, e.opts)
			// A default-mode unit that consumed nothing named no real subcommand.
			if err == nil && unit.checkProgress &&
				len(unit.args) > 0 && len(next) == len(unit.args) {
				err = fmt.Errorf("%w: %s", errUnknownCommand, unit.args[0])
			}

			resultCh <- unitResultMsg{index: idx, err: err, duration: time.Since(start)}
		}(i, unit)
	}
}
```

Acquire precedes the `starting...` print, so a queued unit does not announce itself before it
runs. `runUnitsParallel` sets up the printer and result slice, calls `startUnits`, then
`collectUnitResults(resultCh, results) (int, error)` — **error last, or `staticcheck` ST1008
fails** — and `printUnitResults`. Splitting this way keeps every function under the complexity
limits with no `//nolint`.

### Help (#40 Defect C) — advertise both working forms

The collapse makes `targ <name>` work, so existing help stops lying. It still never mentions the
bare form. `printCommandHelp` gets one node and `RunOptions`; it cannot tell it is the sole root.

**Threading `isDefaultRoot` as a parameter does not work** — there are two help paths and only one
passes through a caller that knows `hasDefault`:

| invocation | reaches `printCommandHelp` via |
|---|---|
| `targ <name> --help` | `handleGlobalHelp` (`:732`), which knows `hasDefault` |
| `targ --help` | `extractHelpFlag` empties args → `handleNoArgs` → `roots[0].execute` → the generic `HelpOnly` interception at `command.go:157-161`, which knows nothing |

A prototype threading the parameter rendered correctly for `targ <name> --help` and had **no
effect at all** on plain `targ --help`. Recorded because it looks right on paper.

So the signal rides on the node, set once where `hasDefault` is computed
(`exec.roots[0].IsDefaultRoot = true`). `parseTargets` builds fresh nodes each run, so this is
per-execution state, not a global — confirmed by a test reusing one `*targ.Target` across two
`Execute` calls, sole-root then not. All four call sites, including `PrintCommandHelpForTest`,
then get the right answer with no signature change.

`printCommandHelp` brackets the name — `usageParts[0] = "[" + usageParts[0] + "]"`, **before** the
`append([]string{opts.BinaryName, "[targ flags...]"}, ...)` or it brackets the binary name — and
forwards `DefaultRoot: node.IsDefaultRoot` into `help.TargetHelpOpts`. `WriteTargetHelp` prepends
the bare example when generating; a caller-supplied `opts.Examples` list still wins untouched.

**Budget for this:** adding the branch pushes the *existing* `WriteTargetHelp` to cyclop **11,
max 10** — measured by inlining the extraction back and re-linting. Extract the examples selection
into `targetHelpExamples`. A `//nolint` is not an option.

Rendered result (one target `marker`, binary `bin`):

```
Usage:
  bin [targ flags...] [marker]

Examples:
  Basic usage:
    bin
  By name:
    bin marker
```

## Consequences of the collapse

Unifying the paths changes user-visible behaviour beyond #40's three defects. Every row was
measured against real binaries. Rows 1–2 *are* Defect A applied to cases #40's examples omit; the
rest are new.

| # | case | before | after | note |
|---|---|---|---|---|
| 1 | `targ grp s1`, `targ grp` (sole group root) | `Unknown command: grp` | runs | Defect A for group roots |
| 2 | `targ -p <name>` on a failing target | spurious `unknown command` | the target's own error | Defect A under `-p` |
| 3 | **`targ <name> <positional>…` where a positional value equals the target's own name** | first arg is data | **first arg is consumed as the command name** | see below |
| 4 | `targ <name>` where the target has a required positional | binds the name as the positional value | `Error: missing required positional` | see below |
| 5 | `targ -p <bogus>` (sole group root) | **silently PASSES**, exit 0, nothing ran | `Error: unknown command: bogus`, exit 1 | fixes a pre-existing bug |
| 6 | `targ <bogus>` (sole group root) | error, no usage block | same error, **plus** a usage block | matches multi-root output |
| 7 | `targ <name> <bogus>` | `unknown command: <name>` | `unknown command: <bogus>` | names the actually-offending token |
| 8 | `targ -p 'glob*'` (sole root) | `unknown command: glob*` | expands and runs | globs now work in default mode |
| 9 | shell-command sole root, `targ <name>` | runs the command **and** reports `Unknown command` | runs, exit 0 | was worse than Defect A |

**Rows 3 and 4 are the irreducible cost of what #40 asks for**, and are the reason this section
exists. Measured on a fixture whose sole target `greet` takes a required positional plus a
variadic:

```
$ greet world extra1 extra2     before: NAME=greet RESTLEN=3    after: NAME=world RESTLEN=2
$ greet greet                   before: NAME=greet RESTLEN=1    after: NAME=greet RESTLEN=0
$ greet                         before: NAME=greet              after: Error: missing required positional: Name
$ world                         before: NAME=world              after: NAME=world       (identical)
$ -l world                      before: LOUD=true               after: LOUD=true        (identical)
```

Making `targ <name>` run the target named `<name>` *necessarily* means the first arg matching that
name is read as the name. There is no implementation that satisfies #40's first acceptance bullet
and also keeps a name-shaped first positional as data. The change resolves the ambiguity the same
way a **two-target** module already does, so single-target modules become consistent with every
other module rather than special. Ordinary positional and flag invocations are untouched — which
is why all 18 existing bare-invocation tests stay green.

This must be documented in the README's new default-target section, not just here, and Unit 1 pins
it with a test.

## Prototype verification

The design was implemented and exercised twice: once by the author in a throwaway worktree, once
independently by a Gate A round-2 executing reviewer working only from this plan's text.
`GOMAXPROCS=10`; `parallelCap(8, 10) = 5`. Nothing below is a prediction.

| check | command | result |
|---|---|---|
| build | `go build ./...` | clean |
| tests | `go test ./...` | all packages ok, incl. both pinned `ParallelFlag*` subtests |
| race | `go test -race ./internal/core/... ./test/...` | clean |
| lint (real config) | `golangci-lint cache clean && golangci-lint run -c dev/golangci-lint.toml ./...` | **`0 issues`** |
| nolint | `grep -c nolint internal/core/run_env.go` | **6 → 4**, none added |
| size | `git diff --stat` | 235 insertions, 307 deletions — **net −72 lines** |
| cyclop budget | inline `targetHelpExamples`, re-lint | **11, max 10** — extraction is required, exactly as claimed |
| `targ check-full` | dirty worktree | 7/8; only `check-uncommitted` fails, by design |
| fixtures | see the consequences table | every row reproduced |
| cap | `repro40cap -p t1…t8`, `repro40grp -p s1…s8` | peak **8 → 5**, 5 runs each |
| **kill-switch** | delete `sem` + acquire/release, re-measure ×5 each | peak returns to **8, 8, 8, 8, 8**; restored → 5 |

The kill-switch proves the semaphore is the sole thing bounding concurrency and no runtime or
stdlib limit silently backstops it, so a cap test has real teeth.

Two caveats on the measurements, both from the round-2 executing reviewer:

- "byte-identical" for the two-target fixture holds for structured output, not literally every
  byte: that fixture's targets use Go's builtin `println`, which bypasses the prefix machinery, so
  its line interleaving varies run to run on **both** binaries. Pre-existing fixture artifact, not
  a regression.
- The prototypes' coverage figures do **not** reproduce against the prescribed decomposition —
  every touched and new function already clears 80% from the pre-existing suite alone. Unit tests
  are still required for the *behaviours*, but not to reach the coverage bar.

The prototype worktrees are deleted before this plan's commit. Execution re-derives the code
through TDD rather than transplanting a diff.

## Doc-surface enumeration

Two invariants change: (1) a module registering exactly one target gets a default that runs bare,
by name, and under `-p`, with a name-shaped first positional now read as the name; (2) CLI
`--parallel`/`-p` fan-out is bounded by `parallelCap(n, GOMAXPROCS)`. A third surface exists
because two functions are **deleted** and are named in historical docs.

Commands, with hit counts measured at plan-write time against `2473fce`. **Unit 6 re-runs them**
and confirms every hit is dispositioned:

```bash
grep -rniI --exclude-dir=.git -E "default target|default root|default command|defaults to" .
grep -rniI --exclude-dir=.git -E "single target|one target|lone target|only target|sole target" .
grep -rniI --exclude-dir=.git -E "bare targ|no arguments|no args\b|without arguments" .
grep -rniI --exclude-dir=.git -E "default-target|defaulttarget|AllowDefault|hasDefault" .
grep -rniI --exclude-dir=.git -E "all at once|simultaneous|concurrently|no limit|unbounded" .
grep -rniI --exclude-dir=.git -E "fan-out|fanout|GOMAXPROCS|parallelCap|concurrency cap" .
grep -rniI --exclude-dir=.git -E -- "--parallel" README.md CLAUDE.md docs/ specs/ projects/ .claude/
grep -rn "Usage:" . --include="*.go" --include="*.md" --include="*.golden"
grep -rn "executeDefaultParallel\|executeDefault\b" --include="*.md" .   # the deleted names
grep -rnE "go [a-zA-Z_][a-zA-Z0-9_.]*\(" --include="*.go" .              # goroutines, dotted receivers included
```

The goroutine pattern allows a dotted receiver because `internal/core/printer.go:23` launches
`go p.run()`. Revision 2 used `go [a-z][A-Za-z]*\(`, which **cannot** match that form — the
character class stops at the dot — so it missed the one example it was added for. Re-run
corrected: 28 hits, of which the only production goroutine launches are `run_env.go` ×2 (the
uncapped fan-outs this plan fixes), `target.go` ×2 (`runGroupParallel*`, already capped),
`printer.go` ×1 (one printer goroutine per parallel call, across all four call sites — not a
fan-out), `internal/sh` ×3 and `dev/targets.go` ×1 (one goroutine per subprocess/pipe). The hits
in `dev/thin_api_test.go` are string literals in a linter's own test data. **No fifth uncapped
fan-out site exists** — now established by a search that could have found one.

| file | line(s) | disposition | reason |
|---|---|---|---|
| `README.md` | 319 §Command Names | **update** | Home for the default-target rule: bare, by name, `-p <name>`, **and** the name-vs-positional precedence from consequences rows 3–4 |
| `README.md` | 474-493 §Example Help Output | **update** | Add a single-target example, regenerated from a real binary. The existing block is *already* stale independent of this issue (shows `Usage: build [flags]`; real output is `Usage:\n  <bin> [targ flags...] build`) — do not copy its format |
| `README.md` | 708 `--parallel` row | **update** | Add the bound, mirroring what :350 does for dep groups |
| `README.md` | 350 | keep | Correctly scoped to dep groups; the :708 update carries the cross-reference |
| `README.md` | 186 §Execution Control | keep | Zero hits; a landmark in #40's text only |
| `docs/specs/use-cases.md` | 5-13 UC-1 | **update** | Silent on the optional-command case |
| `docs/specs/requirements.md` | 173-178 DES-5 | **update** | Scoped to `.Deps()`; add the CLI `-p` fan-out using the identical formula |
| `docs/specs/requirements.md` | 19-24 REQ-3 | **update** | Add the default-dispatch rule and the name-vs-positional precedence |
| `docs/specs/architecture.md` | 19-24 ARCH-3 | **update** | No ARCH item names the `hasDefault` mechanism; ARCH-3 owns command resolution |
| `docs/specs/tests.md` | 32-53 T-3 | **update** | Property bullets: `targ <name>` succeeds with one registered target; a name-shaped first positional binds as the name; top-level `-p` fan-out never exceeds `parallelCap` |
| `docs/specs/state.toml` | L1A/L2A/L2B/L3A `history` | **update** | Precedent is **mixed**, not one-way. #26 (`fdd0984`) and #32 (`ac2542e`) extended IDs without touching it — but `c2d462d` ("extended ARCH-11, ARCH-12"), `453f915` ("updated ARCH-8") and `354b978` ("updated IMPL-13, IMPL-18, IMPL-19") each added a `history` re-entry note for an extension with **no new ID**. Revision 2 asserted the one-way rule and was wrong. Given mixed precedent, record the re-entry — it is cheap and matches the more recent commits |
| `internal/core/types.go` | `AllowDefault` | **update** | No doc comment at all, on the field controlling the invariant, exported via `targ.RunOptions`; sibling `BinaryMode` is documented |
| `internal/core/target.go` | 767-772 `parallelCap` | **update** | Comment says "in a parallel **dep group**"; the CLI path now calls it too |
| `internal/core/override.go` | 27 `Parallel bool` | **update** | Public GoDoc via `targ.RuntimeOverrides`; add the bound |
| `targ.go` | 152-249 | **update** | `Execute`/`ExecuteRegistered`/`Main`/`Register` never mention the default target — the pkg.go.dev-facing gap |
| `internal/flags/flags.go` | 71-76 `--parallel` `Desc` | keep | Rendered inside every `--help`; the formula would bloat the flag table |
| `specs/001-parallel-output/tasks.md` | 56, 75 | N/A | Names `executeDefaultParallel` as a completed task. Historical record; `docs/plans/2026-07-23-issue-26-dep-fanout-cap.md:919` states the repo convention — "Delete no docs — the plan file stays as a session record" |
| `docs/plans/2026-02-19-parallel-output.md` | 1004, 1013, 1014, 1222, 1224 | N/A | Names `executeDefaultParallel` and cites `run_env.go:282-328`, which this plan makes stale. Historical record, same convention |
| `docs/plans/2026-02-19-parallel-output-design.md` | 199 | N/A | Same |
| `test/completion_properties_test.go` | 352, 892 | **verify** | `HandlesSingleTargetMode` / `SingleRootCompletionWithRemainingArgs` exercise `__complete` in single-root mode; `targ.Execute` always sets `AllowDefault: true` |
| `test/validation_properties_test.go` | 12-65 | **verify** | `TestProperty_UsageLine`, single target, `--help`; reaches the bracketed usage line |
| `test/hierarchy_properties_test.go` | 416-431 | **verify** | `HelpInDefaultModeShowsTargetHelp`, same path |
| `internal/core/binary_mode_propagation_test.go` | 31, 72, 104, 128 | **verify** | `AllowDefault: true` help rendering |
| `internal/help/render.go`, `render_test.go` | `"Usage:"` emission | **verify** | The literal string Defect C is about. Assertions are substring/ordering, not exact-match |
| `internal/runner/runner_help_test.go` | — | N/A | Exercises only `PrintCreateHelp`/`PrintSyncHelp`/`PrintToFuncHelp`/`PrintToStringHelp` into a `strings.Builder`; `handleSingleModule` appears **zero** times in the file. Revision 2 marked this "verify" on an inherited rationale that does not hold — corrected to N/A, matching its `.golden` sibling row |
| `internal/core/execute_test.go` | 288-320 | N/A | Revision 2 called this "real overlap with the rewired dispatch". It is not: the test deregisters a package so `resolveRegistry()` errors and `ExecuteWithResolution` returns **before** `RunWithEnv`; its own assertion (`executionCount == 0`) proves dispatch is never reached |
| `internal/runner/testdata/golden/*.golden` | all | N/A | Meta-command help; static literals in `runner.go`, unreachable from `commandNode` rendering |
| `internal/runner/runner.go` | 3906 | N/A | `printMultiModuleHelp` only fires when `len(moduleGroups) > 1` |
| `docs/archive/**` | many | N/A | Not by a written "archive convention" — no such rule exists in this repo; revision 2 asserted one that appears only in a self-citing chain of three plans. The empirical fact: `git log ac98256..HEAD -- docs/archive/` returns **zero** commits since the 2026-03-08 archiving |
| `docs/plans/2026-07-23-issue-26-dep-fanout-cap.md` | 33, 918 | N/A — cite, don't edit | Names this exact CLI fan-out as deferred out-of-scope for #26. Load-bearing prior art |
| `specs/001-parallel-output/contracts/api.md` | 119 | **propose, do not fold** | "still executes targets concurrently", now incomplete. Grounds for deferring: that directory has had **exactly one commit touch it, ever**. (Revision 2 called it "closed"; `spec.md:5` says `Status: Draft`, so that label was wrong even though the disposition is right.) For Joe's yes/no |
| false positives, verified by inspection | `internal/core/{command,execute,registry,result,override,target}.go`, `internal/help/builder.go`, `test/{constraints,hierarchy_fuzz,overrides,shell}_properties_test.go`, `projects/portable-targets/**`, `session-learning-prompt.md`, `.claude/skills/**`, `.claude/commands/**`, `specs/001-parallel-output/**` (other files), `docs/plans/**` (others), `README.md:293`, `docs/specs/tests.md:9` | N/A | Each is a different sense of "default" / "one target" / "at once" / "concurrent", or historical record |

Revision 2 also listed `CLAUDE.md:40` as a verified false positive. It is not reachable from any
command above — `CLAUDE.md:40` reads "reports ALL failures at once", and no listed pattern matches
it. Removed rather than re-justified.

**Zero-hit surfaces** (searched, genuinely absent, so absence is distinguishable from
not-searched): no C4 or architecture-diagram directory; no `CHANGELOG.md`; no `.traced/` directory
at all; no package comment in `internal/core/doc.go`; `examples/` exists but contains zero
`targ.Main(` calls and zero occurrences of "parallel".

## Units

Each unit is RED → GREEN → REFACTOR, with a design-fit review after the refactor phase. Blackbox
tests go in `test/` (`package targ_test`); new subtests are inserted in their block's existing
alphabetical position. Run tests with `targ test`, never bare `go test`, as final validation.

### Unit 1 — serial collapse

**RED 1.** Add subtests to `test/execution_properties_test.go` asserting, via `targ.Execute` with
one registered target: `["app","marker"]` runs it; `["app","bogus"]` fails naming `bogus` **and
the target does not run**; a sole group root `["app","grp","s1"]` runs `s1`; and — pinning
consequences row 3 — a sole target with a required positional invoked as
`["app","greet","world","extra"]` binds `Name == "world"`, not `"greet"`.

Run: `targ test`
Expected: **FAIL**, with `unknown command: marker` / `unknown command: grp` in the captured output.
HALT and re-read the design if any of these unexpectedly passes.

**GREEN.** Add the prepend to `runWithEnvInternal`; change `executeMultiRoot` to pass
`explicit: !e.hasDefault`; delete `executeDefault`.

Run: `targ test` → Expected: PASS.

**REFACTOR + gate B.**

### Unit 2 — parallel collapse

**RED 1.** Subtests: `["app","--parallel","marker"]` runs the target;
`["app","--parallel","bogus"]` **fails** on both a sole plain root and a sole group root (the
group case is the pre-existing silent-pass bug); a single target whose function returns a
distinctive error puts **that** text in `result.Output` under `-p`, not `unknown command`.

Run: `targ test` → Expected: **FAIL** — the group `-p bogus` case fails by *passing* when it
should error, so assert on the error text, not merely on a non-nil error.

**GREEN.** Add `parallelUnit`, `resolveUnits`, `runUnitsParallel`, `startUnits`,
`collectUnitResults` (returning `(int, error)`), `printUnitResults`; delete
`executeDefaultParallel`; add the `runtime` import; **delete both `//nolint:cyclop,funlen`
directives (`run_env.go:280`, `:492`) and add none.**

Run: `targ test` → PASS. Then `golangci-lint run -c dev/golangci-lint.toml ./...` → `0 issues`,
and `grep -c nolint internal/core/run_env.go` → `4`. HALT if either differs.

**REFACTOR + gate B**, checking the result reads as one fan-out written that way from the start.

### Unit 3 — the concurrency cap

**RED 1.** One upper-bound test per mode (default and multi-root); the multi-root path has none
today. Each target increments an atomic in-flight counter and CAS-updates a peak, then blocks on a
gate a watcher closes once the peak reaches the expected cap; assert
`peak <= max(2, GOMAXPROCS/2)`. No wall-clock assertions. Each subtest gets its own fixtures and
counters. Use `n = 2*cap + 2`.

**RED 2 — observe it, do not assume it.** An upper-bound assertion can pass by scheduling luck.
Run against the uncapped tree: `targ test`. Expected: **FAIL**. Re-run until RED is observed; if
it is not reliably observable, widen `n` until it is, and record the observed pre-fix peak in the
commit message. The kill-switch measurement (peak returns to 8 on five runs of each fixture with
the semaphore deleted) says the margin is real.

**GREEN.** The semaphore and `ctx.Err()` skip ship with Unit 2; this unit adds their tests.

**REFACTOR + gate B.**

### Unit 4 — help advertises both forms

**RED 1.** Whitebox (`internal/core`, via `PrintCommandHelpForTest`): `IsDefaultRoot` true renders
`[name]`, false renders `name`. This closes a real gap — no existing test asserts the full
default-root usage line.
**RED 2.** Blackbox: with one registered target, **both** `["app","--help"]` and
`["app","marker","--help"]` contain the bracketed usage line and both examples. With two
registered targets, `["app","marker","--help"]` does **not** contain `[marker]`.
**RED 3.** `IsDefaultRoot` does not leak: reuse one `*targ.Target` across two `Execute` calls,
sole-root then not.

Run: `targ test` → Expected: FAIL on all three.

**GREEN.** `commandNode.IsDefaultRoot`; set it in `runWithEnvInternal`; bracket in
`printCommandHelp`; `TargetHelpOpts.DefaultRoot`; bare-example prepend in `WriteTargetHelp`;
extract `targetHelpExamples`.

Run: `targ test` → PASS; `golangci-lint run -c dev/golangci-lint.toml ./...` → `0 issues`. If
`WriteTargetHelp` reports cyclop 11, the extraction was not applied.

**REFACTOR + gate B.**

### Unit 5 — queued-unit cancellation skip

The skip ships with Unit 2. Its *test* is separated because determinism is unproven. Investigate
whether a deterministic discriminator exists: with cap `c` and `n > c` units where unit 0 fails
immediately, assert fewer than `n` bodies ran **and** at least one result classified `CANCELLED`.
**If it cannot be made deterministic, do not ship a non-discriminating test** — record it as a
known gap in the unit's report. No existing test asserts `CANCELLED` on either the dep-group or
CLI side.

### Unit 6 — docs

1. Re-run all ten enumeration greps; confirm every hit is dispositioned by the table above.
2. `README.md` — :319 the default-target rule **and** the name-vs-positional precedence; :708 the
   cap bound; :474-493 regenerate the help example from a real binary's output.
3. `docs/specs/` — `use-cases.md` UC-1; `requirements.md` REQ-3 and DES-5; `architecture.md`
   ARCH-3; `tests.md` T-3; `state.toml` `history` re-entry on L1A/L2A/L2B/L3A.
4. Doc comments — `types.go` `AllowDefault`; `override.go` `Parallel`; `target.go` `parallelCap`;
   `targ.go` `Execute`/`ExecuteRegistered`/`Main`/`Register`.
5. Run the **verify** rows and record what was checked.

## Verification

- `targ check-full` must report **8/8**. Sequence it **after** the commit, or expect PASS on every
  leg *except* `check-uncommitted`, which fails by design on a dirty tree.
- Lint with the project config (see Global constraints). If `lint-full` fails, run
  `golangci-lint cache clean` and re-run before treating it as real.
- **Re-measure the two headline numbers against the TDD'd code, not the discarded prototype**:
  `git diff --stat` (expect roughly −70 net lines in `run_env.go`) and
  `grep -c nolint internal/core/run_env.go` (expect `4`). The prototype's figures do not carry
  over automatically.
- Re-run every row of the consequences table against real binaries.
- Per-function coverage is 80%. Measured on the prototype, every touched and new function already
  clears it from the pre-existing suite — so coverage is not the constraint; the Units' tests
  exist to pin *behaviour*. Read uncovered lines rather than accepting a percentage.
- `targ reorder-decls`.

### Watch item

`test/execution_properties_test.go:1584` (`CollectAllErrorsPreservesDeclarationOrder`) enforces a
cross-goroutine ordering assumption with `time.Sleep(50ms)` while `dep2` fails instantly. It is
the one sleep-based ordering-critical assertion in the parallel tests, and this cycle adds
concurrency-heavy tests to the same gate path — the load change that has starved it before. If it
fails under `check-full` while passing solo, that is the cause. Fix by causality, never by
lengthening the sleep.

## Out of scope

- `handleNoArgs`' execution path — runs only when there are no args, passes `nil`, nothing to
  strip. Its *help* path is in scope (Unit 4).
- `target.go`'s `runGroupParallel`/`runGroupParallelAll`.
- **Shell-command targets still execute before an unknown trailing arg is reported.**
  `executeShellCommand`'s `checkUnknownFlags` only rejects `-`-prefixed args, so
  `targ <shell-target> bogus` runs the command and *then* errors — before and after this change.
  Consequence: the acceptance bullet "fails without running the target" holds for `Func` targets,
  not universally. Pre-existing and unrelated to the collapse.
- **`<binary> <sub> --help` on a sole group root renders the group's help, not the subcommand's.**
  Verified pre-existing at `2473fce`; the collapse does not repair it. Raised with Joe during
  planning and **not answered** — the redirect superseded the question. **For Joe's yes/no at
  close: fix now, or file as a separate issue.** Not to be filed unilaterally.
- `specs/001-parallel-output/contracts/api.md:119` — proposed above for Joe's yes/no.
- Removing the default-target feature (measured at 102 broken subtests plus a public API break;
  Joe chose the collapse).

## Acceptance

- [ ] A module registering exactly one target runs via `targ`, `targ <name>`, and `targ -p <name>`
- [ ] `targ <bogus>` on that module still fails, naming `bogus`, without running the target
      (`Func` targets; see Out of scope for shell-command targets)
- [ ] `targ -p <bogus>` fails on both a sole plain root and a sole group root
- [ ] `targ -p <name>` on a failing target prints that target's own error
- [ ] A name-shaped first positional binds as the command name, and this is documented in the
      README and pinned by a test
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

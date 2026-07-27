# Issue #42 design — reject an unresolvable argument before anything runs

## The defect

`internal/core/command.go:1027` — `executeShellCommand` takes the `explicit` parameter as `_ bool, // explicit - not used for shell commands` and ignores it. `command.go:164` hands the same value to `executeFunctionWithParents`, which honors it. That asymmetry is the bug.

The switch it ignores is `internal/core/parse.go:150-154`, inside `trySubcommandOrUnknown`: when `ctx.explicit` is false an unmatched arg is `errUnknownCommand`; when true it is returned as a chainable leftover. `explicit` is set at `run_env.go:318` as `!e.hasDefault`.

## Measured behavior at commit 55c6e48

| invocation | shell target | function target |
|---|---|---|
| 1 target, `targ bogus` | runs dep + command, then errors | rejects, nothing runs |
| 1 target, `targ gen bogus` | runs dep + command, then errors | rejects, nothing runs |
| 2 targets, `targ bogus` | rejects, nothing runs | rejects, nothing runs |
| 2 targets, `targ gen bogus` | runs `gen`, then errors | runs `gen`, then errors — identical |

Issue #42 claims `targ <shell-target> bogus` misbehaves on any module, single- or multi-target. The multi-target row refutes that — a function target does the same thing there, so that case is targ's chaining design, not a shell defect. The genuine shell/function divergence is the single-target rows only.

`targ -p bogus` on a sole shell target has the same defect via a third path. `resolveUnits` (`run_env.go:549-593`) pre-resolves every arg EXCEPT in default mode, where an unmatched arg becomes a unit with `explicit: false` and the verdict is deferred to a post-hoc check at `run_env.go:726-731` (`len(next) == len(unit.args)`). `explicit:false` saves a function target there; it does not save a shell target.

## Scope decision

The repo owner chose the broader scope: parity fix PLUS chain pre-validation, and the parallel path included. The author recommended the narrower parity-only fix (shell targets honoring `explicit`), and the owner chose the broader scope; the narrower option is not to be relitigated.

The initial estimate was wrong in the cautious direction: chain pre-validation was first priced as requiring a from-scratch dispatch-loop restructure, until `opts.HelpOnly` turned out to already provide a resolve-without-execute seam at every target kind.

## Design — approach chosen

Add `ResolveOnly bool // Internal: resolve args to targets without running them` to `RunOptions` in `internal/core/types.go`, beside the existing `HelpOnly` and `HasDefault`, which already carry `// Internal:` markers on the same exported struct.

Two alternatives were considered and rejected:

1. **A `mode` enum replacing `HelpOnly`/`ResolveOnly`**: rejected because `RunOptions` is a public type alias (`targ.go:81`) so removing `HelpOnly` is an API break, paid for a purely internal invariant.
2. **A standalone `resolveChain()` walking roots/globs/subcommands independently**: rejected because it re-implements the resolution rules owned by `parseCommandArgs` and `parseShellCommandArgs`, creating a second source of truth that will drift.

## Components

1. `internal/core/types.go` — add `ResolveOnly bool // Internal:` beside `HelpOnly`.
2. `internal/core/command.go:896`, `:944`, `:1037` — each existing `if opts.HelpOnly` early return becomes `if opts.HelpOnly || opts.ResolveOnly`. The help PRINTING at `command.go:157-161` stays `HelpOnly`-only — that is the whole difference between the two modes.
3. `internal/core/command.go` `executeFunctionWithParents` — in resolve mode, parse into a fresh `reflect.New(node.Type).Elem()`, never the shared `node.Value`. The reason: `nodeInstance` (`command.go:1408-1418`) returns the SHARED `node.Value` when the node has an addressable value, and slice binding appends (`parse.go:221`, `parse.go:872`), so a resolve pass followed by an execute pass would double a variadic positional or a repeated flag. Nothing double-parses today; this design introduces the first double-parse, so the guard is required, not optional.
4. `internal/core/command.go` `executeShellCommand` — accept `explicit` instead of `_`, and mirror `parse.go:150-154`: if `!explicit && len(parsed.remaining) > 0`, return `errUnknownCommand` naming `parsed.remaining[0]`. Placed after `checkUnknownFlags` (`:1047`) and BEFORE `runNodeDeps` (`:1052`).
5. `internal/core/run_env.go` `executeRoots` (`:279-331`) — extract the loop body so the chain can be walked twice: a resolve pass, then an execute pass. Note that `executeGlobPattern` (`:251-273`) passes `e.opts` down, so resolve mode propagates into glob-expanded targets, and it passes `nil` args so it contributes no leftovers.
6. `internal/core/run_env.go` `resolveUnits` (`:549-593`) — resolve default-mode units upfront and reject there; DELETE the post-hoc check at `:726-731`, which becomes dead once resolution moves ahead of execution.

## Resulting behavior

| invocation | pre-pass outcome | result |
|---|---|---|
| 1 shell target, `targ bogus` | `gen` resolves with `explicit=false`; shell path now errors | `Error: unknown command: bogus`, nothing ran |
| 2 targets, `targ gen bogus` | `gen` resolves, leftover `bogus` matches no root | `Unknown command: bogus` + usage block, nothing ran |
| 2 targets, `targ gen other` | both resolve, chain empties | both run, unchanged |
| `targ -p deploy bogus` | `bogus` unit consumes nothing | rejected before any goroutine starts |

## Error handling

No new message text. A leftover in COMMAND position reuses the dispatcher's existing `Unknown command: <tok>` plus usage block (`run_env.go:307-310`). A leftover in ARGUMENT position propagates `errUnknownCommand` and surfaces as `Error: unknown command: <tok>` (`run_env.go:322-325`) — which is exactly what a function target prints today, so shell and function targets converge on identical output.

## Breaking change

On a multi-target module, `targ gen bogus` currently runs `gen` and then errors. After this change it runs nothing. That is a deliberate, owner-approved behavior change, and it also changes FUNCTION-target behavior, not only shell. It contradicts the current wording of `docs/specs/architecture.md:21`, which is why that spec line is updated as part of the work.

## Testing strategy

- RED first — the repo requires a real failing test before implementation, per `CLAUDE.md`.
- Cover: all rows of both tables above; a variadic-positional target that must receive its args ONCE, not doubled (component 3's guard); glob, caret `^`, deps-only, and group targets through the resolve pass; both parallel modes.
- CLI-level assertions read `result.Output`, never `errors.As` — the dispatch layer collapses every error into a bare `ExitError{Code: 1}`, so `errors.As` after `targ.Execute()` is structurally impossible.
- Execute the kill-switch procedure when writing it: disable the pre-pass and confirm the test actually fails. A step that is written but never run proves nothing.
- `targ check-full` is the gate. Baseline measured green 9/9 at `55c6e48`.

## Doc-surface disposition

| file | lines | disposition | reason |
|---|---|---|---|
| `docs/specs/architecture.md` | 21 (ARCH-3) | update | Owns both the `explicit=!hasDefault` sentence and the `resolveUnits` sentence; both change |
| `docs/specs/requirements.md` | 21 (REQ-3) | update | Owns target execution and default-target dispatch; the new "a rejected invocation runs nothing" invariant belongs here |
| `docs/specs/tests.md` | 32 (T-3) | update | T-3 is Execution behavior; the new tests list there |
| `docs/specs/implementation.md` | 36-42 (IMPL-5) | keep | Files list already covers command.go, run_env.go, parse.go; Key functions names only exported entry points and no new exported function is added |
| `docs/specs/use-cases.md` | UC-1, UC-5 | keep | UC level states user goals; rejecting a bad invocation is not a new goal |
| `README.md` | 334-338 (Default Target) | update | User-visible guarantee change; the Default Target section states the sole-target dispatch contract |
| `docs/archive/**` | many | N/A | Frozen — verified independently: the only commit touching it since 2026-03-01 is the archiving commit `ac98256` (2026-03-08) |
| `docs/plans/2026-*` | many | N/A | Historical cycle records, not living docs |
| `specs/001-parallel-output/contracts/api.md` | 115-120 | N/A | Backward-compatibility list covers output prefixing, not argument resolution |
| `projects/portable-targets/**` | — | N/A | A different project's docs |
| `CLAUDE.md` | — | N/A | Development process rules; no dispatch semantics |

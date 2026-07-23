# targ#22 — pass the binary name via argv[0]; delete TARG_BIN_NAME

Issue: https://github.com/toejough/targ/issues/22 · Approved design: Joe picked "option 1" from
the 2026-07-22 investigation briefing (argv[0], delete the env var entirely) over read-then-unset,
sh-helper filtering, and namespacing.

## Ask trace

The issue asks that targ stop leaking `TARG_BIN_NAME` into child process trees, where any
`targ.Main()`-based binary (engram, c4, traced) reads it with priority over its own `argv[0]`
and displays/embeds "targ" instead of its own name. The 2026-07-22 investigation (4-angle,
hermetic repro) established:

- The var exists solely to carry the user-typed wrapper name across targ's two-stage re-exec
  (wrapper → cached bootstrap binary whose own argv[0] is a cache-hash path). Introduced by
  b66b853 (2026-01-08) for exactly that.
- All three stamp sites spawn only targ-built bootstrap binaries; the leak is **transitive**
  (targets shell out with nil `cmd.Env`, so grandchildren inherit everything).
- It is functional, not cosmetic: `BinaryName()` feeds generated shell-completion scripts, so a
  leaked name makes a consumer emit completions that invoke `targ`.

Fix: pass the display name as the child's `argv[0]` (Go supports `Args[0] != Path`) and delete
the env var — writer and reader side — so the leak class ceases to exist.

## Current anchors (all read 2026-07-22 against HEAD 0436d45; line numbers current)

Writer (internal/runner/runner.go) — three stamp sites, identical stanza shape:

- `executeBuiltBinary(binaryPath, targBinName string)` — :1417
  `cmd := exec.CommandContext(context.Background(), binaryPath, r.args...)` then :1419
  `cmd.Env = append(os.Environ(), "TARG_BIN_NAME="+targBinName)`
- `tryRunCached(binaryPath, targBinName string)` — :1928/:1930, same shape.
- `runModuleBinary(binaryPath string, args []string, errOut io.Writer, binArg string)` —
  :3957/:3962, stamp value `extractBinName(binArg)`.

Reader:

- `osRunEnv.BinaryName()` — internal/core/execute.go:134-152: env branch (:135-137) → empty-args
  fallback `"targ"` → basename of `os.Args[0]` (handles `/` and `\`).
- `ExecuteEnv.BinaryName()` — internal/core/run_env.go:35-47: env-map branch → `e.args[0]`
  (raw, no basename) → `"app"`. Doc comment (:34-35) names TARG_BIN_NAME.

Pinned test: test/completion_properties_test.go:727-745, subtest `BinaryNameFromEnvironment`
(unique repo-wide): args `["app", "--completion"]`, env map `TARG_BIN_NAME: "mycli"`, asserts
completion output contains `mycli`.

Repo-wide `TARG_BIN_NAME` count (grep, non-test + test): exactly 8 hits across runner.go (3),
execute.go (1), run_env.go (2: branch + comment), completion_properties_test.go (1), plus the
run_env.go doc-comment line — all enumerated above. After this change: **0 hits**.

Unstamped bootstrap spawns (today): dispatchCompletion (:2865) and the `__list` probe (:3781)
spawn bootstrap binaries without the var. T2 carries a check item for whether their child code
paths consume `BinaryName()`; they join the new constructor only if yes (no silent scope growth).

## Design

New exported constructor in internal/runner (name collision-checked: zero hits today):

```go
// ChildCommand builds the exec.Cmd for a targ-built bootstrap binary, presenting
// displayName as the child's argv[0] so its help/completion output shows the name
// the user actually typed rather than the cache path. Env is left nil (inherit).
func ChildCommand(ctx context.Context, binaryPath, displayName string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Args = append([]string{displayName}, args...)

	return cmd
}
```

- The three stamp sites construct via `ChildCommand(...)`, keep their own IO wiring
  (Stdout/Stderr/Stdin), and **drop the `cmd.Env` line entirely** (nil = plain inheritance).
- `osRunEnv.BinaryName()` loses the env branch; basename fallback is the whole behavior.
- `ExecuteEnv.BinaryName()` loses the env-map branch; doc comment rewritten.
- Nested targ-under-targ: each wrapper names its own child via argv[0]; no cross-tree channel
  exists anymore (previously env last-entry-wins).
- Deliberate contract removal: the undocumented env override
  (`TARG_BIN_NAME=x anybinary` renaming the binary) dies with no deprecation — no repo, doc,
  or consumer evidence of intentional use (investigated 2026-07-22).

## Acceptance criteria

1. A `targ.Main()`-based binary run anywhere inside a targ process tree displays its own name
   (reader-side: env var ignored even if present; writer-side: var never set).
2. `TARG_BIN_NAME` appears in no child or grandchild environment spawned by targ (hermetic
   smoke: `env -u TARG_BIN_NAME <freshly built wrapper> --source <probe> env-probe` shows
   child env empty and an engram grandchild printing `Usage: engram`).
3. targ's own help still shows the wrapper-invoked name, not the cache-hash name, across the
   fresh-build, cached, and multi-module paths (argv[0] carries it now).
4. Completion scripts embed the args-derived name.
5. Repo-wide grep `TARG_BIN_NAME` → 0 hits; full suite green; `targ check-full` green
   post-commit.

## Tasks (TDD; RED must fail before GREEN; Gate B after each task)

Tests live blackbox where the subject is exported. Conventions: gomega `g := NewWithT(t)`,
`t.Parallel()`, alphabetical declaration order (`targ reorder-decls`), complexity budgets
(cyclomatic ≤10 / cognitive ≤30 / ≤60 lines), no new package-level mutable state.

- **T1 — reader ignores the env var.** RED: rewrite the pinned subtest as
  `BinaryNameFromArgsNotEnvironment`: args `["mycli", "--completion"]`, env map still carries
  `TARG_BIN_NAME: "fromenv"`; assert output contains `mycli` AND does not contain `fromenv`.
  Run it — fails today (env wins; output contains `fromenv`). GREEN: delete the env branch in
  `osRunEnv.BinaryName()` (execute.go:135-137) and the env-map branch + doc-comment line in
  `ExecuteEnv.BinaryName()` (run_env.go:37-40). REFACTOR + Gate B.
- **T2 — writer passes the name via argv[0].** RED: new blackbox test (runner_test package)
  `TestChildCommand`: `runner.ChildCommand(ctx, "/cache/bin/targ_abc", "targ", "build", "-v")`
  → `Path == "/cache/bin/targ_abc"`, `Args == ["targ", "build", "-v"]`, `Env == nil`. Compile
  error = valid RED (ChildCommand undefined). GREEN: add `ChildCommand`; route
  `executeBuiltBinary`, `tryRunCached`, `runModuleBinary` through it; delete all three
  `TARG_BIN_NAME` stamp lines. Check item (resolve during GREEN, include only if consumed):
  grep whether the `__complete` (dispatchCompletion :2865) and `__list` (:3781) child code
  paths reach `BinaryName()`; if yes, route them through `ChildCommand` too; if no, leave and
  note it in the report. REFACTOR + Gate B.
- **T3 — smoke + full validation (orchestrator).** `grep -rn TARG_BIN_NAME --include="*.go"`
  → 0 hits. Build the wrapper fresh (`go build -o <scratch>/targ ./cmd/targ`) and run the
  hermetic probe from the investigation (`env -u TARG_BIN_NAME <scratch>/targ --source
  <envprobe> env-probe`) — expect `CHILD TARG_BIN_NAME=""`, grandchild unset, engram line
  `engram [flags...]`; also `<scratch>/targ --help` path shows `targ` (argv[0] name). Full
  suite; `targ reorder-decls`; commit; `targ check-full` green post-commit (check-uncommitted
  needs the committed tree).
- **T4 — document, close, capture.** Doc grep (README/docs/examples) had zero TARG_BIN_NAME
  hits on 2026-07-22 — re-verify, expect nothing to update (Gate C likely N/A). Close #22 with
  the AC→evidence chain (citations verified against the tree per the standing evidence-prose
  rule). Delivery note: consumers (engram, c4, traced) pick the fix up via module bump — no
  targ-side action; flag to Joe that engram/c4 bumps can happen whenever convenient. Gate D on
  commit + close prose. Closing /learn + lessons audit.

## Risks / notes

- `exec.CommandContext` resolves `cmd.Path` from its argument before we override `cmd.Args`;
  overriding `Args[0]` post-construction is the documented Go pattern ("Args holds command line
  arguments, including the command as Args[0]").
- Coverage gate: `ChildCommand` is a new exported function in internal/runner → needs ≥80%
  per-function coverage; `TestChildCommand` provides it directly.
- The `BinaryNameFromEnvironment` rewrite is the regression pin: env map still populated,
  args-derived name must win.
- Consumers on older targ keep the old behavior until they bump — acceptable; the leak is
  wrapper-side, so a bumped targ stops polluting even trees containing old-consumer binaries
  (their reader branch just never sees the var).

## /please step tracking

1. ✅ Capture (open) — sweep current from #23 close (same session)
2. ✅ Orient — 4-angle investigation (wf_91044066-a14) + briefing; Joe approved option 1 and
   filed-now disposition; incidental dispatch bug filed as #24
3. ☐ Plan — this doc; Gate A pending
4. ☐ Execute (T1–T3, TDD, Gate B per task)
5. ☐ Document (T4a; Gate C — likely N/A)
6. ☐ Complete (close #22, commit; Gate D)
7. ☐ Capture (close) — lessons audit + closing /learn

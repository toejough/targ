# targ#22 — pass the binary name via argv[0]; delete TARG_BIN_NAME

Issue: https://github.com/toejough/targ/issues/22 · Approved design: Joe picked "option 1" from
the 2026-07-22 investigation briefing (argv[0], delete the env var entirely) over read-then-unset,
sh-helper filtering, and namespacing. **Revision 2** — Gate A round-1 findings addressed (fixture
inlined, coverage gap closed, gosec nolint added, grep gate rescoped, check item pre-resolved,
April-comment reconciliation added).

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

**Reconciling the April "not a bug" comment on #22:** the 2026-04-15 comment attributed the
symptom to a sandbox-set `TARG_BIN_NAME` overriding argv[0] in that one environment. The
2026-07-22 hermetic repro (`env -u TARG_BIN_NAME <wrapper> --source <probe> …`) proves targ's
own spawn code generates the var unconditionally with no sandbox involvement — the sandbox
observation was itself downstream of the leak (that session was launched under `targ claude`).
That supersession is what licenses fixing an issue whose thread contains the reporter's own
dismissal; T4's closing comment must cite it explicitly.

Fix: pass the display name as the child's `argv[0]` (Go supports `Args[0] != Path`) and delete
the env var — writer and reader side — so the leak class ceases to exist.

## Current anchors (verified against HEAD b4fb056 on the pristine tree, 2026-07-22)

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
- `ExecuteEnv.BinaryName()` — internal/core/run_env.go:34-47: env-map branch → `e.args[0]`
  (raw, no basename) → `"app"`. Constructor `NewExecuteEnv(args []string)` at :25.

Pinned test: test/completion_properties_test.go:727-745, subtest `BinaryNameFromEnvironment`
(unique repo-wide): args `["app", "--completion"]`, env map `TARG_BIN_NAME: "mycli"`, asserts
completion output contains `mycli`.

Repo-wide `TARG_BIN_NAME` grep, measured: exactly 8 hits — runner.go 3 (:1419, :1930, :3962),
execute.go 1 (:135), run_env.go 3 (:35 doc comment, :37 branch comment, :38 branch code),
completion_properties_test.go 1 (:738). **After this change: zero hits in non-test code; the
only remaining hits live in test/completion_properties_test.go's rewritten subtest, which
deliberately keeps the key as the regression pin.**

Unstamped bootstrap spawns (today): dispatchCompletion (:2865) and the `__list` query in
`queryModuleCommands` (:3781). **Check item pre-resolved (Gate A round 1, live-traced): they
stay unrouted.** `RunWithEnv` (internal/core/run_env.go:164, call at :167) calls
`env.BinaryName()` unconditionally, but the child-side `__complete`/`__list` handlers —
`doCompletion` (internal/core/completion.go:679) and `doListTo` (internal/core/run_env.go:931,
re-measured on the pristine tree) — never receive
or use the name, so routing those spawns would be silent scope growth for zero behavior. The T2
implementer notes this disposition in the report; no code change there.

## Design

New exported constructor in internal/runner/runner.go (name collision-checked: zero hits today;
alphabetical slot lands after `CheckImportExistsWithFileOps`):

```go
// ChildCommand builds the exec.Cmd for a targ-built bootstrap binary, presenting
// displayName as the child's argv[0] so its help/completion output shows the name
// the user actually typed rather than the cache path. Env is left nil (inherit).
func ChildCommand(ctx context.Context, binaryPath, displayName string, args ...string) *exec.Cmd {
	//nolint:gosec // G204: build tool runs targ-built bootstrap binaries by design
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Args = append([]string{displayName}, args...)

	return cmd
}
```

(The nolint is load-bearing: the three deleted stamp sites each carry a `//nolint:gosec` that
dies with them; without it here, `targ lint-full` fails G204 — measured in Gate A round 1.)

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
   smoke, fixture inlined in T3).
3. targ's own help shows the wrapper-invoked name, not the cache-hash name. Evidence split:
   fresh-build and cached paths verified end-to-end by the T3 smoke; the multi-module path
   (`runModuleBinary`) is evidenced by unit suite + inspection only — it routes through the
   same `ChildCommand` constructor that `TestChildCommand` pins (state it exactly this way in
   the close-out; do not overclaim E2E coverage there).
4. Completion scripts embed the args-derived name.
5. `grep -rn TARG_BIN_NAME --include="*.go"` returns hits ONLY in
   test/completion_properties_test.go (the deliberate regression pin); zero hits elsewhere.
   Full suite green; `targ check-full` green post-commit.

## Tasks (TDD; RED must fail before GREEN; Gate B after each task)

Tests live blackbox where the subject is exported. Conventions: gomega `g := NewWithT(t)`,
`t.Parallel()`, alphabetical declaration order (`targ reorder-decls`), complexity budgets
(cyclomatic ≤10 / cognitive ≤30 / ≤60 lines), no new package-level mutable state.

- **T1 — reader ignores the env var.** RED: rewrite the pinned subtest as
  `BinaryNameFromArgsNotEnvironment`: args `["mycli", "--completion"]`, env map still carries
  `TARG_BIN_NAME: "fromenv"`; assert output contains `mycli` AND does not contain `fromenv`.
  Run it — fails today (env wins: the completion script is built around `fromenv`; measured in
  Gate A round 1). GREEN: delete the env branch in `osRunEnv.BinaryName()` (execute.go:135-137)
  and the env-map branch + both comment lines in `ExecuteEnv.BinaryName()` (run_env.go:35,
  :37-40). **Coverage guard (measured: without this, check-coverage fails —
  `run_env.go:36: BinaryName 66.7%`):** in the same GREEN, add `TestExecuteEnvBinaryName` to
  internal/core/execute_test.go with two subtests:
  `core.NewExecuteEnv([]string{"mycli"}).BinaryName()` == `"mycli"` and
  `core.NewExecuteEnv(nil).BinaryName()` == `"app"`. (These pass before AND after — they are
  coverage-restoring regression tests, not the RED; the rewritten completion subtest is the
  RED.) REFACTOR + Gate B.
- **T2 — writer passes the name via argv[0].** RED: new file
  internal/runner/child_command_test.go, package runner_test, test `TestChildCommand`:
  `runner.ChildCommand(ctx, "/cache/bin/targ_abc", "targ", "build", "-v")` →
  `Path == "/cache/bin/targ_abc"`, `Args == ["targ", "build", "-v"]`, `Env == nil`. Compile
  error = valid RED (ChildCommand undefined). GREEN: add `ChildCommand` (design block above,
  nolint included); route `executeBuiltBinary`, `tryRunCached`, `runModuleBinary` through it;
  delete all three `TARG_BIN_NAME` stamp lines. dispatchCompletion/`queryModuleCommands` stay
  unrouted per the pre-resolved check item (note it in the report). REFACTOR + Gate B.
- **T3 — smoke + full validation (orchestrator).** Grep gate per AC5 (hits only in the pinned
  test file). Write the probe fixture below to a fresh temp dir (e.g.
  `<scratchpad>/envprobe/targets.go` — it needs its own dir; two registered targets because a
  single-target `--source` dir cannot dispatch by name, tracked as #24). Build the wrapper
  fresh (`go build -o <scratchpad>/targ ./cmd/targ`) and run
  `env -u TARG_BIN_NAME <scratchpad>/targ --source <scratchpad>/envprobe env-probe` — expect
  `CHILD TARG_BIN_NAME=""`, `GRANDCHILD-UNSET`, `ENGRAM USAGE LINE: engram [flags...]`; also
  `<scratchpad>/targ --source <scratchpad>/envprobe --help` shows `targ` (argv[0] name), and a
  copy of the wrapper named `mytool` shows `mytool` (argv[0] channel proof; measured working in
  Gate A round 1). The engram line requires `/Users/joe/go/bin/engram`; if absent, drop probe
  step 3 from the fixture and rely on AC1's reader-side test instead. **STOP condition: if any
  smoke line diverges from expected, STOP before committing** — the argv[0] fix is not taking
  effect (first suspect: a stale cached bootstrap binary; retry with `--no-binary-cache`), and
  if that doesn't explain it, surface to Joe rather than debugging forward. Then: full suite;
  `targ reorder-decls`; commit; `targ check-full` green post-commit (check-uncommitted needs
  the committed tree).

  Probe fixture (verified working 2026-07-22, including under the fixed tree in Gate A round 1):

  ```go
  //go:build targ

  // Package dev holds a probe target for TARG_BIN_NAME leak verification.
  package dev

  import (
  	"fmt"
  	"os"
  	"os/exec"
  	"strings"

  	"github.com/toejough/targ"
  )

  // EnvProbe reports TARG_BIN_NAME visibility in child, grandchild, and engram.
  var EnvProbe = targ.Targ(envProbe).Description("Probe TARG_BIN_NAME env leak")

  // Noop exists so the built binary keeps subcommand dispatch (two targets, see targ#24).
  var Noop = targ.Targ(noop).Description("Do nothing")

  func init() {
  	targ.Register(EnvProbe, Noop)
  }

  func noop() {}

  func envProbe() error {
  	fmt.Printf("CHILD TARG_BIN_NAME=%q\n", os.Getenv("TARG_BIN_NAME"))

  	out, err := exec.Command("sh", "-c",
  		"env | grep '^TARG_BIN_NAME=' || echo GRANDCHILD-UNSET").CombinedOutput()
  	if err != nil {
  		return fmt.Errorf("grandchild env probe: %w", err)
  	}
  	fmt.Printf("GRANDCHILD: %s", out)

  	help, err := exec.Command("/Users/joe/go/bin/engram", "--help").CombinedOutput()
  	if err != nil {
  		return fmt.Errorf("engram --help: %w (output: %s)", err, help)
  	}
  	lines := strings.SplitN(string(help), "\n", 3)
  	if len(lines) >= 2 {
  		fmt.Printf("ENGRAM USAGE LINE: %s\n", strings.TrimSpace(lines[1]))
  	}

  	return nil
  }
  ```

- **T4 — document, close, capture.** Doc grep (README/docs/examples/.claude, literal and
  concept-variant) re-verified zero hits in Gate A round 1 — re-confirm, expect nothing to
  update (Gate C likely N/A). Close #22 with the AC→evidence chain (citations verified against
  the tree per the standing evidence-prose rule), **including the April-comment supersession
  paragraph from the Ask trace** so the close doesn't read as ignoring the reporter's own prior
  dismissal. Delivery note: consumers (engram, c4, traced) pick the fix up via module bump — no
  targ-side action; flag to Joe that consumer bumps can happen whenever convenient. Gate D on
  commit + close prose. Closing /learn + lessons audit.

## Risks / notes

- `exec.CommandContext` resolves `cmd.Path` from its argument before we override `cmd.Args`;
  overriding `Args[0]` post-construction is the documented Go pattern ("Args holds command line
  arguments, including the command as Args[0]").
- Coverage gate reality (corrected in rev 2): internal/runner/runner.go is **exempt** from the
  per-function coverage check (isEntryPointCoverageLine, dev/targets.go:1589-1593 — runner
  orchestration is integration-tested), so the gate never sees `ChildCommand`;
  `TestChildCommand` is required by the repo's TDD rule, not the gate. `ExecuteEnv.BinaryName`
  (internal/core/run_env.go) IS gated — hence T1's coverage guard.
- The `BinaryNameFromArgsNotEnvironment` rewrite is the regression pin: env map still
  populated, args-derived name must win; its TARG_BIN_NAME references are the only ones that
  survive repo-wide.
- Consumers on older targ keep the old behavior until they bump — acceptable; the leak is
  wrapper-side, so a bumped targ stops polluting even trees containing old-consumer binaries
  (their reader branch just never sees the var).
- Gate A process note: round 1 ran the code angle's live probe concurrently with the
  anchor-checking angles, which briefly made anchors appear shifted (+10 lines) and the tree
  dirty to them; round-1 anchor "corrections" from those angles were re-verified against the
  pristine tree before this revision (the :3781 `__list` anchor stands).

## /please step tracking

1. ✅ Capture (open) — sweep current from #23 close (same session)
2. ✅ Orient — 4-angle investigation (wf_91044066-a14) + briefing; Joe approved option 1 and
   filed-now disposition; incidental dispatch bug filed as #24
3. ✅ Plan — rev 2; Gate A round 1: 4 angles, 1 Critical + 5 Important + 6 Minor, all addressed;
   ACK round: clarity + docs ACK, ask + code one convergent Minor (two stale anchors from the
   round-1 mid-fix tree, corrected above and self-verified). Gate A closed
4. ✅ Execute — T1 (reader) + T2 (writer) via workflow, Gate B PASS round 1 each; T3: scope
   exactly the six planned files, AC5 grep gate holds (2 pin hits only), hermetic smoke exact
   on cached AND fresh-build paths incl. renamed-wrapper argv[0] proof (targ-new/mytool),
   commit 2bc60d2, check-full PASS:8 post-commit. Carried item: T2's gate observed a
   pre-existing wall-clock flake (test/execution_properties_test.go
   DefaultParallelStillCancelsOnFirstError, 1s budget) — outside plan scope, flagged to Joe
5. ✅ Document — doc-surface grep re-verified zero hits outside this plan doc; Gate C N/A
6. ☐ Complete (close #22 with evidence chain + April supersession; Gate D)
7. ☐ Capture (close) — lessons audit + closing /learn

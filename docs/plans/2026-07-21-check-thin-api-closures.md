# check-thin-api: walk closures, linear thin-body grammar, composite-literal returns

**Issue:** [targ#23](https://github.com/toejough/targ/issues/23) — "check-thin-api: walk closure bodies; accept composite-literal returns"
**Date:** 2026-07-21 · **Branch:** main · **Status:** plan (Gate A pending)

## Ask trace

Joe's ask (2026-07-21): analyze issue #23, confirm the fix preserves/strengthens check-thin-api's
intent (closures must not become a place to hide business logic that belongs in testable
`internal/` functions), then — after discussion — implement it. Discussion settled three points,
all approved by Joe:

1. **Grammar unification** — named functions adopt the same linear thin-body grammar as closures
   (not a separate, stricter 1-statement rule).
2. **Close the whole loophole** — call arguments and composite-literal field values are validated
   too, not just FuncLits ("everything in a checked file is inspected" becomes literally true).
3. **`go` statement allowance** — a `go` launching a plain qualified call is deliberately allowed
   (the spawned logic lives in internal/; the edge only launches it).

Issue AC amendment (recorded per the resolution rule): the issue's "all existing non-closure
acceptance behavior unchanged" is superseded by approved point 2 — argument/field validation
deliberately tightens some previously-accepted shapes (e.g. `return pkg.F(forbiddenExpr)`), and
unification deliberately relaxes others (multi-statement linear named funcs, local calls). Also:
no test suite for the thinness functions exists today (repo-wide grep by the test-map scout), so
"the current test suite still passes" is satisfied trivially; this plan creates the suite.

## Why this strengthens the intent (settled at orientation)

Today closures are a zero-inspection channel: `analyzeThinness` iterates only top-level
`file.Decls` and never descends into `*ast.FuncLit` (dev/targets.go:171), so arbitrary logic in a
closure keeps the gate green. Walking them under a strict linear grammar converts the gate's
weakest point into an enforced boundary: **assignment of primitives is allowed; composition and
branching (beyond the single error-guard shape) fail with a named position**. The
composite-literal change removes the laundering incentive (identity constructors whose arguments
were never inspected) in favor of a transparent form that *is* inspected.

## Baseline evidence (measured at plan time)

| Claim | Evidence | Verified how |
| --- | --- | --- |
| Working tree clean, branch main | `git status --short` empty | baseline scout ran it, 2026-07-21 |
| `targ check-thin-api` green | "All 2 public API files are thin wrappers." exit 0 | baseline scout ran it |
| `targ check-full` fully green | "PASS:8 / All checks passed!" exit 0, all 8 checks listed | baseline scout ran it (full run, ~80s) |
| dev-tagged tests green | `go test -tags targ ./dev/` → `ok … 0.446s` | orchestrator ran it, 2026-07-21 |
| Checked surface = `targ.go`, `cmd/targ/main.go` only | walk skip rules: `.git/vendor/internal/testdata` SkipDir; `_test.go`, `internal/`, `examples/`, `generated_`, build-tag/generated-marker files skipped; dev/targets.go excluded by its `//go:build targ` line | code-map scout applied the verbatim skip rules to the tree |
| No existing tests cover any thinness function | repo-wide grep for the seven function names in `_test.go` files: definitions only | test-map scout |
| dev/ is exempt from lint + coverage gates but subject to reorder-decls-check | golangci has no `run.build-tags` (dev/ excluded from analysis); `targ test` runs without `-tags targ` (dev/ never in coverage.out); reorder walks all non-generated .go files | code-map + test-map scouts, empirically confirmed |
| engram consumes this checker via the targ module | `/Users/joe/repos/personal/engram/dev/targs.go:13` — `_ "github.com/toejough/targ/dev"` | orchestrator grep, 2026-07-21 |

## The grammar (normative spec)

One recursive checker applied to **every function body in checked files** — named functions
(FuncDecl) and function literals (FuncLit) alike, wherever the FuncLit appears: struct-literal
field value, call argument, var initializer, return result, nested in any allowed expression.

### Statement grammar (`checkLinearBody`)

Every statement in a body must be one of:

| # | Statement | Constraints |
| --- | --- | --- |
| S1 | AssignStmt (`:=` or `=`) | every LHS is an Ident (incl. `_`) or a SelectorExpr (field set on local/captured value, incl. tuple form); every RHS is an allowed expression |
| S2 | DeclStmt (`var`/`const`) | every spec value is an allowed expression |
| S3 | ExprStmt | the expression is an allowed CallExpr (E3) |
| S4 | GoStmt | the call's Fun passes the call-head rule (E3a–c — a FuncLit head, i.e. an inline goroutine, is NOT allowed); args are allowed expressions |
| S5 | IfStmt (error-guard only) | no Init clause; Cond is `X != nil` (BinaryExpr NEQ, exactly one operand the `nil` ident, the other an Ident or SelectorExpr); no Else; Body is exactly one ReturnStmt (S6-validated) |
| S6 | ReturnStmt | only as the body's final statement (or as an S5 guard body); **every** result is an allowed expression |

Anything else — `for`/`range`/`switch`/`select`, non-guard `if`, `defer`, channel send,
IncDecStmt, labeled/branch statements, nested FuncDecls — is a violation naming the statement
kind and position. Empty bodies and nil bodies (interface methods) are thin.

### Expression grammar (`checkLinearExpr`)

Allowed (recursive):

| # | Expression | Constraints / rationale |
| --- | --- | --- |
| E1 | Ident, BasicLit | variables, `nil`/`true`/`false`, literals |
| E2 | SelectorExpr | X recursed (chains like `out.Embeddings` allowed — field access is data) |
| E3 | CallExpr | Fun is (a) SelectorExpr with Ident X — qualified `pkg.F(...)`/`recv.M(...)`; (b) bare Ident — local call **or** builtin conversion (`int(fd)`, `fsPrimitives()`); (c) IndexExpr/IndexListExpr over (a)/(b) — generic instantiation. Every argument recursed; Ellipsis allowed. |
| E4 | CompositeLit | every element recursed (KeyValueExpr: key and value both) |
| E5 | FuncLit | body checked by the statement grammar (the core of change 1) |
| E6 | UnaryExpr | Op in `& + - ^ !` only (address-of, sign, complement, not); operand recursed. Receive (`<-`) is NOT allowed. |
| E7 | BinaryExpr | both operands recursed, any operator (needed: `os.O_APPEND\|os.O_CREATE`, `core.CallerSkipPublicAPI+1`) |
| E8 | ParenExpr, Ellipsis | recursed / pass-through |
| E9 | TypeAssertExpr | operand recursed, type is a type expression (`session.(*hugot.Session)`) |
| E10 | Type expressions | ArrayType, ChanType, MapType, StarExpr, FuncType, InterfaceType, StructType — signatures/types carry no statements; needed for `make(chan os.Signal, n)` and pointer types |

Explicitly forbidden as value expressions: IndexExpr/SliceExpr (`m[k]`, `s[i:j]` — state access
beyond simple data), receive ops, IIFE (`func(){...}()` — FuncLit call head), everything not
listed. Violations name the node kind.

### Rationale for the corpus-driven rules (each validated against real code)

- **E3b (bare-Ident calls)**: subsumes builtin conversions (`int(fd)` in engram's must-pass lock
  closures is syntactically identical to a local call) AND local calls (`fsPrimitives()` wiring in
  engram's main; `return Match(patterns...)` inside targ.go's own `Checksum`/`Watch` closures —
  **targ's own checked surface requires this rule**). Sound because every package-level function
  in a non-internal file is itself checked by this same gate — logic cannot hide behind a local
  call. This deliberately relaxes the current `return localFunc()` rejection ("calls local
  function, not external package"); recorded as intended.
- **E7**: required by targ.go's `Register` (`core.CallerSkipPublicAPI+1`) and engram's flag-OR
  args.
- **E9**: required by engram's `hugotRuntime.NewPipeline` (`session.(*hugot.Session)`).
- **S5 tightening**: current `isSimpleErrorWrapper` accepts any `a != b` cond and never inspects
  the if-body; the new rule demands nil-comparison + lone return. This is what makes the issue's
  must-FAIL `WriteFileExcl` case fail (compound cond `closeErr != nil && err == nil`, assignment
  body).
- **Multi-result returns fully validated**: closes the current hole where
  `return pkg.F(x), anything` leaves trailing results uninspected.

### Top-level spec rules

- `checkValueSpecThinness` var path: values validated with `checkLinearExpr` (so
  `var X = func(...) { linear }` and `var X = pkg.T{...}` pass; today both fail wholesale).
  Const path unchanged (selectors + basic literals) — the issue doesn't ask and consts can't hold
  FuncLits' runtime behavior meaningfully.
- Type-spec rules unchanged (aliases, interfaces, empty structs).
- File-selection rules unchanged — `isPublicAPIEntryPoint` needs no mirror update.

### Violation reporting

One violation per offending statement/expression: File, Line (of the offending node), Name (the
enclosing top-level decl via `funcDeclName`, with `(closure)` appended when inside a FuncLit),
Reason (specific: "for statement not allowed in thin body", "call argument is not thin: …").
`checkThinAPI` sorts violations by file then line before printing (replaces today's
nondeterministic map-iteration order; needed for stable tests, flagged as a small in-scope
output-quality fix).

## Tasks (TDD; RED must fail before GREEN; Gate B after each refactor)

Implementation lives in `dev/targets.go` (helpers inserted at exact alphabetical positions in the
unexported-funcs run; `targ reorder-decls` auto-fixes). Tests in new `dev/thin_api_test.go`:
`//go:build targ`, `package dev` (whitebox, per dev/ precedent), gomega dot-import,
`g := NewWithT(t)`, `t.Parallel()` (no Chdir needed — tests call `analyzeThinness(path)` on
fixture files written to `t.TempDir()`). Test command (verified working at baseline):
`go test -tags targ -run '<TestName>' ./dev/`.

- **T1 — expression grammar.** RED: table-driven tests for E1–E10 + forbidden kinds (each case an
  inline source string → temp .go file → `analyzeThinness`, asserting on violation
  presence/reason; expression cases exercised through minimal single-return functions). GREEN:
  `checkLinearExpr` + call-head/type-expr helpers (small per-kind helpers; default complexity
  budgets: cyclomatic ≤10, cognitive ≤30, ≤60 lines — lint doesn't gate dev/ but CLAUDE.md does).
  REFACTOR + Gate B.
- **T2 — statement grammar.** RED: tests for S1–S6 + forbidden statements + guard tightening
  (compound cond fails, non-return body fails, else fails, mid-body return fails). GREEN:
  `checkLinearBody`/`checkLinearStmt` + error-guard helper, FuncLit recursion, violation model
  with `(closure)` naming. REFACTOR + Gate B.
- **T3 — integration.** RED: corpus fixture tests — engram worktree shapes (RunCommand,
  StartSignalPulses, OpenDebugFile, lock conversions, hugot NewPipeline with FuncLit-in-return,
  main() wiring) must pass; `WriteFileExcl` must fail exactly at its statement 5; targ.go shapes
  (Checksum/Watch closures, Register binary-op arg, const/var blocks) must pass; composite-literal
  returns pass with embedded-FuncLit violations still caught; var-init FuncLits both ways. GREEN:
  `checkFuncThinness` delegates to the linear checker; `checkValueSpecThinness` var path uses
  `checkLinearExpr`; delete `checkReturnThinness` + `isSimpleErrorWrapper`; sorted output.
  REFACTOR + Gate B.
- **T4 — property test (rapid).** RED: generator composes bodies from allowed-statement templates
  (must pass) and injects one forbidden statement at a random position (must fail, reported line
  matches the injection). Modest scope: one property, template-based.
- **T5 — smoke + full validation.** `targ check-thin-api` on the repo (expect "All 2 public API
  files are thin wrappers."); `go test -tags targ ./dev/` green; `targ reorder-decls`; commit;
  `targ check-full` green (check-uncommitted requires the committed tree — order per the
  gate-dependency lesson).
- **T6 — document, close, capture.** Doc dispositions below (no prose docs need updating; the
  gate's one-line description stays accurate). Close #23 with the evidence chain (each AC → test
  name/output). Closing `/learn` + lessons audit.

## Doc-surface disposition (non-waivable grep, run 2026-07-21)

Greps: `check-thin-api|thin-api|thin wrapper|thin functions|checkThinAPI|CheckThinAPI` repo-wide;
`closure|FuncLit|function literal` over README/CLAUDE/docs/specs/.claude/examples.

| File | Disposition | Reason |
| --- | --- | --- |
| dev/targets.go | N/A — source | the gate being changed; its line-410 diagnostic string is updated by the implementation itself |
| projects/portable-targets/tasks.md | keep | generic "thin wrapper pattern" reference; delegation stays valid (grammar broadens acceptance) |
| cmd/targ/main.go | N/A — source | doc comment "thin wrapper" describes the binary, not the gate grammar |
| internal/core/run_env.go | N/A — source | "thin wrapper" homonym in a nolint justification |
| .claude/commands/speckit.checklist.md, speckit.analyze.md | N/A | "closure" matched inside "disclosure" |
| README.md, CLAUDE.md, docs/**, specs/** | N/A — no hits | nothing describes check-thin-api's grammar today; the plan doc (this file) is the record |

## Delivery notes

- engram picks this up by bumping its targ module pin (`_ "github.com/toejough/targ/dev"` blank
  import); its `WriteFileExcl` closure will then fail as intended (tracked engram-side as
  engram#703's decomposition work). No copies to sync.
- Deliberately stricter, no opt-out flag (issue AC; matches the no-grandfathering standard).

## /please step tracking

1. ✅ Capture (open) — sweep ran (background), vocab OK
2. ✅ Orient — recall + issue + code verified; design discussed; 3 points approved
3. ⏳ Plan — this document; commit + Gate A (4 angles) next
4. ☐ Execute (T1–T5, TDD, Gate B per refactor)
5. ☐ Document (T6a; Gate C — subject may be absent: no prose docs to touch)
6. ☐ Complete (close #23, commit; Gate D)
7. ☐ Capture (close) — lessons audit + closing /learn

(Task-list tooling is absent in this environment — TaskCreate/TaskUpdate not present; this
section is the tracking mechanism.)

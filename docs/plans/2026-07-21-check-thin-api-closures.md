# check-thin-api: walk closures, linear thin-body grammar, composite-literal returns

**Issue:** [targ#23](https://github.com/toejough/targ/issues/23) — "check-thin-api: walk closure bodies; accept composite-literal returns"
**Date:** 2026-07-21 · **Branch:** main · **Status:** plan revision 2 (Gate A round 1 findings addressed)

## Ask trace

Joe's ask (2026-07-21): analyze issue #23, confirm the fix preserves/strengthens check-thin-api's
intent (closures must not become a place to hide business logic that belongs in testable
`internal/` functions), then — after discussion — implement it. Discussion settled three points,
all approved by Joe:

1. **Grammar unification** — named functions adopt the same linear thin-body grammar as closures
   (not a separate, stricter 1-statement rule).
2. **Close the whole loophole** — call arguments and composite-literal field values are validated
   too, not just FuncLits ("everything in a checked file is inspected" becomes literally true).
3. **`go` statement allowance** — a `go` launching a plain **qualified** call is deliberately
   allowed (the spawned logic lives in internal/; the edge only launches it).

**AC amendment (approved, intentional):** the issue's original AC said "all existing non-closure
acceptance behavior unchanged." Approved point 2 deliberately goes further: call-argument and
composite-literal-field validation now applies everywhere, so some previously-accepted shapes
(e.g. `return pkg.F(m[k])`) now FAIL, and unification deliberately relaxes others
(multi-statement linear named functions; local calls — see E3b). Tests must cover both
directions of the delta. Separately: no test suite for the thinness functions exists today
(verified by the test-map scout), so "the current test suite still passes" is satisfied
trivially; this plan creates the suite.

**Scope note (Gate A ask-alignment finding, resolved):** an earlier revision added deterministic
sorting of `checkThinAPI`'s cross-file output. That is a fifth behavior change Joe never
approved — struck. Tests don't need it: per-file violations from the walker arrive in source
order (deterministic), and T1/T2 tests don't go through `checkThinAPI` at all. The pre-existing
nondeterministic cross-file print order stays as-is; flagged to Joe in the close-out report as an
optional one-line follow-up, not part of this plan. **Close-out disposition (2026-07-22): Joe
approved handling it immediately — landed in bf1dad6 (`sortedViolationFiles`; per-file source
order preserved).**

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
| Checked surface = `targ.go`, `cmd/targ/main.go` only | walk skip rules: `.git/vendor/internal/testdata` SkipDir; `_test.go`, `internal/`, `examples/`, `generated_`, build-tag/generated-marker files skipped; dev/targets.go excluded by its `//go:build targ` line | code-map scout applied the verbatim skip rules to the tree; Gate A code reviewer independently re-verified |
| New grammar keeps both checked files green | every declaration in targ.go (all 26 funcs incl. Checksum/Watch closures, Register's `core.CallerSkipPublicAPI+1`, const/var/type blocks) and cmd/targ/main.go hand-walked against S1–S6/E1–E10: all accepted | Gate A code-alignment reviewer, statement-by-statement |
| Corpus verdicts correct | three must-PASS shapes, fsPrimitives composite return, four lock closures, hugot NewPipeline all pass; WriteFileExcl fails exactly at statement 5 | Gate A code-alignment reviewer, statement-by-statement |
| No existing tests cover any thinness function | repo-wide grep for the seven function names in `_test.go` files: definitions only | test-map scout |
| dev/ exempt from lint + coverage gates, subject to reorder-decls-check | golangci has no `run.build-tags`; `targ test` runs without `-tags targ`; reorder walks all non-generated .go files | code-map + test-map scouts, empirically confirmed |
| `checkReturnThinness`/`isSimpleErrorWrapper` safe to delete | referenced only inside `checkFuncThinness`; no `_test.go` references | Gate A code-alignment reviewer |
| engram consumes this checker via the targ module | `/Users/joe/repos/personal/engram/dev/targs.go:13` — `_ "github.com/toejough/targ/dev"` | orchestrator grep; Gate A code reviewer re-confirmed |

## The grammar (normative spec)

One recursive checker applied to **every function body in checked files** — named functions
(FuncDecl) and function literals (FuncLit) alike, wherever the FuncLit appears: struct-literal
field value, call argument, var initializer, return result, nested in any allowed expression.

### Statement grammar (`checkLinearBody`)

Every statement in a body must be one of:

| # | Statement | Constraints |
| --- | --- | --- |
| S1 | AssignStmt (`:=` or `=`) | LHS: each element is an Ident (incl. `_`) or a SelectorExpr (field of a local/captured value); tuples of these are allowed (`a, b := …`, `cmd.Dir, cmd.Stdout, cmd.Stderr = …`). RHS: each value individually satisfies the expression grammar. |
| S2 | DeclStmt (`var`/`const`) | every spec value is an allowed expression |
| S3 | ExprStmt | the expression is an allowed CallExpr (E3, any head) |
| S4 | GoStmt | the call's Fun must be a **qualified** head only — E3a, or a generic instantiation (E3c) over E3a. Bare-Ident heads (E3b) and FuncLit heads (inline goroutines) are NOT allowed. Args are allowed expressions. This is the approved point-3 wording ("plain qualified call") taken literally; relax only if a real corpus demands it. |
| S5 | IfStmt (error-guard only) | no Init clause; Cond is exactly `X != nil` (BinaryExpr NEQ, one operand the `nil` ident, the other an Ident or SelectorExpr — compound conditions like `a != nil && b == nil` fail); no Else; Body is exactly one ReturnStmt (its results validated per S6's expression rule) |
| S6 | ReturnStmt | allowed in exactly two syntactic positions: (i) the **last** statement of a function/closure body, or (ii) the **sole** statement of an S5 guard body. A ReturnStmt anywhere else is a violation ("mid-body return"). Every result is an allowed expression. |

Anything else — `for`/`range`/`switch`/`select`, non-guard `if`, `defer`, channel send,
IncDecStmt, labeled/branch statements, nested FuncDecls — is a violation naming the statement
kind and position. Empty bodies and nil bodies (interface methods) are thin.

### Expression grammar (`checkLinearExpr`)

Allowed (recursive):

| # | Expression | Constraints / rationale |
| --- | --- | --- |
| E1 | Ident, BasicLit | variables, `nil`/`true`/`false`, literals |
| E2 | SelectorExpr | X recursed (chains like `out.Embeddings` allowed — field access is data) |
| E3 | CallExpr | Fun is one of: **(a)** SelectorExpr with Ident X — qualified `pkg.F(...)`/`recv.M(...)`; **(b)** bare Ident — local call or builtin conversion (`fsPrimitives()`, `int(fd)`); **(c)** IndexExpr/IndexListExpr *wrapping* an (a) or (b) head — generic instantiation (`pkg.F[T](x)`); (c) is a wrapper over (a)/(b), never an independent head. Every argument recursed; Ellipsis allowed. Exception: `make`'s first argument is a type expression (E10). |
| E4 | CompositeLit | the literal's Type is a type expression (E10); every element recursed; KeyValueExpr: key and value both recursed |
| E5 | FuncLit | body checked by the statement grammar (the core of change 1) |
| E6 | UnaryExpr | Op in `& + - ^ !` only; operand recursed. Receive (`<-`) is NOT allowed. |
| E7 | BinaryExpr | both operands recursed, any operator (needed: `os.O_APPEND\|os.O_CREATE`, `core.CallerSkipPublicAPI+1`) |
| E8 | ParenExpr, Ellipsis | recursed / pass-through |
| E9 | TypeAssertExpr | operand recursed; asserted Type is a type expression (E10) — `session.(*hugot.Session)` |
| E10 | Type expressions — **type positions only** | Ident, SelectorExpr, ArrayType, ChanType, MapType, StarExpr, FuncType, InterfaceType, StructType, accepted **only** where Go requires a type: `make`'s first argument, TypeAssertExpr.Type, CompositeLit.Type, generic index arguments. In **value** position these nodes are forbidden — in particular `*p` (value dereference via StarExpr) fails; no corpus shape needs it, and admitting it would be accidental, not decided (Gate A code finding, resolved). |

Explicitly forbidden as value expressions: IndexExpr/SliceExpr (`m[k]`, `s[i:j]` — state access
beyond simple data), value dereference (`*p`), receive ops, IIFE (`func(){...}()` — FuncLit call
head), everything not listed. Violations name the node kind.

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

One violation per offending statement/expression, in the existing `thinViolation` struct and the
existing print format (file header, then `  <line>: <name> - <reason>`):

- **File/Line**: position of the offending node.
- **Name**: for FuncDecl scopes, `funcDeclName(fn)`; inside a FuncLit, append ` (closure)`. For
  FuncLits under a top-level `var`, the enclosing name is `"var " + nameListString(vs.Names)`
  (the existing dev/targets.go:668 convention) with ` (closure)` appended — `funcDeclName` takes
  a FuncDecl and cannot name this path (Gate A code finding, resolved).
- **Reason**: specific — e.g. `for statement not allowed in thin body`,
  `call argument is not thin: index expression`, `if is not the error-guard shape: compound
  condition`.

Within-file violations arrive in source order from the walker (deterministic). Cross-file print
order (map iteration) is unchanged — see Scope note. Tests assert on `thinViolation` struct
fields (whitebox), never on printed text.

## Tasks (TDD; RED must fail before GREEN; Gate B after each refactor)

Implementation lives in `dev/targets.go` (helpers inserted at exact alphabetical positions in the
unexported-funcs run; `targ reorder-decls` auto-fixes). **All T1–T4 tests go in one new file,
`dev/thin_api_test.go`**: `//go:build targ`, `package dev` (whitebox, per dev/ precedent), gomega
dot-import, `g := NewWithT(t)`, `t.Parallel()` (no Chdir needed). Test-function naming:
`TestCheckLinearExpr_*` (T1), `TestCheckLinearBody_*` (T2), `TestAnalyzeThinness_*` (T3),
`TestProperty_*` (T4). Test command (verified working at baseline):
`go test -tags targ -run '<TestName>' ./dev/`.

**Test-target layering (Gate A code finding, resolved):** T1/T2 tests call the new helpers
*directly* — they do not go through `analyzeThinness`, which still dispatches to the old
`checkFuncThinness` path until T3 wires the delegation. T1 parses expressions with
`parser.ParseExpr(src)`; T2 parses `"package p\nfunc f() { … }"` with `parser.ParseFile` and
extracts `Body.List`. Only T3 uses temp-dir fixture files through `analyzeThinness`.

- **T1 — expression grammar.** RED: table-driven `TestCheckLinearExpr_*` cases for E1–E10 and
  each forbidden kind, shaped like:

  ```go
  cases := []struct{ name, src string; wantReason string }{ // wantReason "" = allowed
      {"qualified call", `pkg.F(x)`, ""},
      {"builtin conversion", `int(fd)`, ""},
      {"flag or", `os.O_APPEND | os.O_CREATE`, ""},
      {"funclit walked", `func() { x := 1 }`, ""}, // body delegated to checkLinearBody
      {"index expr", `m[k]`, "index expression"},
      {"iife", `func() {}()`, "function literal call"},
      {"receive", `<-ch`, "unary operator"},
  }
  // per case: expr, err := parser.ParseExpr(c.src); reason := checkLinearExpr(expr); assert
  ```

  Expected RED: compile error (`checkLinearExpr` undefined) — a valid RED per dev/ precedent.
  GREEN: `checkLinearExpr` + call-head/type-position helpers (small per-kind helpers; default
  complexity budgets: cyclomatic ≤10, cognitive ≤30, ≤60 lines — lint doesn't gate dev/ but
  CLAUDE.md does). REFACTOR + Gate B.
- **T2 — statement grammar.** RED: `TestCheckLinearBody_*` cases for S1–S6, forbidden statements,
  and guard tightening — concretely, at minimum: the RunCommand 3-statement shape (pass);
  `if err != nil { return err }` guard (pass); `if closeErr != nil && err == nil { err = closeErr }`
  (fail: compound condition); `if err != nil { log(); return err }` (fail: guard body not a lone
  return); `if a != b { return x }` (fail: not a nil comparison); guard with Else (fail); a
  ReturnStmt that is neither the body's last statement nor an S5 guard body (fail: mid-body
  return); `for`/`switch`/`defer`/IncDec (fail, reason names the kind); `go pkg.F(x)` (pass) vs
  `go localFunc()` and `go func(){}()` (fail). Fixture pattern: parse
  `"package p\nfunc f() { …body… }"`, extract `Body.List`, call `checkLinearBody`. Expected RED:
  compile error (`checkLinearBody` undefined). GREEN: `checkLinearBody`/`checkLinearStmt` +
  error-guard helper, FuncLit recursion, violation model with closure naming. REFACTOR + Gate B.
- **T3 — integration.** RED: `TestAnalyzeThinness_*` fixtures — inline source strings (hermetic
  copies of the corpus shapes, cited to their source lines) written to `t.TempDir()` .go files,
  through `analyzeThinness(path)`, asserting on the returned `[]thinViolation` structs. Helper:
  `analyzeSrc(t, src string) []thinViolation` (write temp file, run, return). Fixtures and
  expectations:
  - engram corpus (from the 700-internal-purity worktree, cited lines): RunCommand (main.go:40-49),
    StartSignalPulses (:129-140), OpenDebugFile (:124-128), the four lock closures with
    `int`/`uint32`/`uintptr` conversions (:99-111), `fsPrimitives` composite-literal return
    (:56-91 minus WriteFileExcl), hugot `NewPipeline` (hugot.go:28-49: wrapper + type-assert arg +
    FuncLit-in-return + captured-receiver call), engram `main()` wiring (nested qualified calls,
    local calls as field values, `hugotRuntime{}` local composite, `...` spread of a call result)
    — all zero violations.
  - `WriteFileExcl` (main.go:69-89): exactly one violation; its Line equals the fixture line of
    statement 5 (the compound-condition `if`), Name ends in `(closure)`, Reason names the guard
    shape.
  - targ.go shapes: `Checksum`/`Watch` closures (bare local-call returns), `Register`
    (`core.CallerSkipPublicAPI+1` arg), the const/var blocks — zero violations.
  - composite-literal return with an embedded FuncLit containing a `for` — exactly the FuncLit
    violation; `var X = func() { … }` both ways (linear → pass; with a loop → fail, Name
    `var X (closure)`); multi-result `return pkg.F(), m[k]` fails on the second result.
  GREEN: `checkFuncThinness` delegates to the linear checker; `checkValueSpecThinness` var path
  uses `checkLinearExpr`; delete `checkReturnThinness` + `isSimpleErrorWrapper`. REFACTOR + Gate B.
- **T4 — property test (rapid).** RED: `TestProperty_ThinGrammar` — generator composes bodies
  from allowed-statement templates (must yield zero violations) and injects one forbidden
  statement at a random position (must yield ≥1 violation whose Line matches the injection).
  Modest scope: one property, template-based.
- **T5 — smoke + full validation.** `targ check-thin-api` on the repo (expect "All 2 public API
  files are thin wrappers."); `go test -tags targ ./dev/` green; `targ reorder-decls`; commit;
  `targ check-full` green (check-uncommitted requires the committed tree — order per the
  gate-dependency lesson).
- **T6 — document, close, capture.** Doc dispositions below (no prose docs need updating; the
  gate's one-line description stays accurate). Close #23 with the evidence chain (each AC → test
  name/output). In the close-out report to Joe: flag the optional cross-file output-sort
  follow-up (see Scope note) for a yes/no. Closing `/learn` + lessons audit.

## Doc-surface disposition (non-waivable grep, run 2026-07-21; Gate A docs angle PASSed it)

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
3. ✅ Plan — revision 2; Gate A closed (round 1: docs PASS, ask/code/clarity findings fixed;
   round 2: all three angles ACK, no counters)
4. ✅ Execute — T1–T4 via workflow (Gate B: T1 PASS r1, T2 FAIL r1→fix→PASS r2, T3 PASS r1,
   T4 PASS r1; four Minor findings on record, none blocking); T5: scope-contained diff
   (2 files), dev suite green, check-thin-api green, commit 4476ed2, check-full PASS:8
5. ✅ Document — dispositions re-verified post-implementation (gate description "Check public
   API is thin wrappers" still accurate; no prose docs touched); Gate C N/A (subject absent)
6. ✅ Complete — Joe dispositioned all four residual Minors as fix-now and pulled the
   sort-order follow-up in-scope; both landed in bf1dad6 (impl + Gate B PASS r1, check-full
   PASS:8 post-commit). Gate D on close-out prose: FAIL r1 (citation errors, fixed +
   crystallized as vault note 343) → PASS r2; #23 closed with the AC→evidence chain
7. ☐ Capture (close) — lessons audit + closing /learn

(Task-list tooling is absent in this environment — TaskCreate/TaskUpdate not present; this
section is the tracking mechanism.)

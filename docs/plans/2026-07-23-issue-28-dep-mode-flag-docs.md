# Issue #28: Document `--dep-mode` in Spec Flag Enumerations — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--dep-mode` to every spec enumeration of built-in runtime flags and document the flatten/CollectAllErrors caveat where the flag's semantics are described — doc-only, no behavior changes. (A README runtime-flags row is proposed separately as an OPTIONAL follow-up pending Joe's yes/no — see the Scope note and Optional Follow-up F; it is NOT part of this cycle's commit.)

**Architecture:** Five text edits across four spec files (`docs/specs/implementation.md` ×2, `docs/specs/architecture.md`, `docs/specs/tests.md`, `docs/specs/requirements.md`), verified by grep counts and `targ check-full`. The TDD analogue for doc edits: baseline grep proves absence (RED), edit, grep proves presence with expected counts (GREEN).

**Tech Stack:** Markdown, grep, `targ check-full`, `gh`.

## Global Constraints

- Doc-only: no changes under `internal/`, `cmd/`, or any `.go` file. `git diff --stat` at the end must show ONLY the four spec files above (plus this plan file's own revisions). `README.md` is NOT touched in this cycle.
- `targ check-full` must be green before commit (CLAUDE.md: no pre-existing-failure exceptions).
- Commit trailer is `AI-Used: [claude]` — NOT Co-Authored-By (Joe's global CLAUDE.md).
- Behavior facts the caveat text must match (verified against main, this session, and independently re-verified by the Gate A code-alignment reviewer): `--dep-mode` accepts `serial`|`parallel` (`internal/core/override.go:521-549`; constants at `internal/core/target.go:687-688`); applying it flattens ALL dep groups into a single group and the rebuilt group carries only targets+mode, dropping `CollectAllErrors` (`internal/core/command.go:1944-1966`); `check-full` uses `targ.DepModeParallel` + `targ.CollectAllErrors` today (`dev/targets.go:37`), so `--dep-mode serial` on `check-full` reverts it to fail-fast on first error. Do not fix the behavior — #26 owns that; if #26 later changes the flatten/drop, the wording gets revisited then.

## Scope note

The issue names five spec sites. Since it was filed, commit `fbc5179` already added the bare flag to the IMPL-15 registry list (site 2), so that site only needs the caveat sentence.

**Proposed scope addition — NOT executed in this cycle (needs Joe's yes/no):** the README "Runtime Flags" table (`README.md:700-709`) enumerates the same runtime override flags and omits `--dep-mode`. Rationale for proposing it: a user-facing table omitting a documented runtime flag misleads by omission (vault note 383 records the sibling pattern failing Gate C in issue #27's cycle). Rationale for gating rather than folding it in: the user's ask was "implement 28", the issue's acceptance names exactly five spec sites, and vault note 345 (a prior targ Gate A lesson) says additions beyond the approved ask are recorded as explicitly-optional follow-ups for the user's yes/no, never folded into the committed action list. Note 383's trigger ("the change promotes/renames/adds a user-facing element") does not strictly fire here: this change edits internal specs about an already-public flag, so the README's pre-existing omission is not made worse by it. The proposal lives in **Optional Follow-up F** at the end of this plan.

## Doc-surface enumeration grep — disposition list

Grep run against the working tree (2026-07-23): `grep -rniE 'dep-mode|depmode|dependency mode|collectallerrors' --include='*.md' .` (One row below — `use-cases.md:10` — was added by manual read of the file, not produced by that grep; provenance noted inline.)

| Surface | Disposition | Reason |
| --- | --- | --- |
| `docs/specs/implementation.md:40` (IMPL-5 Purpose, runtime-overrides list) | **update** | Issue site 1 — flag missing from list |
| `docs/specs/implementation.md:121` (IMPL-15 flag registry) | **update** | Issue site 2 — flag already listed (fbc5179); caveat still missing |
| `docs/specs/architecture.md:21` (`RuntimeOverrides` extraction list) | **update** | Issue site 3 — flag missing |
| `docs/specs/tests.md:34` (runtime-overrides acceptance list) | **update** | Issue site 4 — flag missing |
| `docs/specs/requirements.md:21` (REQ-3 built-in runtime flags) | **update** | Issue site 5 — flag missing |
| `README.md:700-709` (Runtime Flags table) + footnote paragraph at `README.md:711` | **proposed update — deferred** | Optional Follow-up F, pending Joe's yes/no (see Scope note) |
| `README.md:207,340,347,380,383,538,543` (compile-time `.Deps()`/`DepModeParallel`/`CollectAllErrors` API examples) | keep | Compile-time builder-API prose and code examples; accurate as-is, same reasoning as `docs/specs/architecture.md:7` |
| `docs/specs/architecture.md:7` (Target struct, `DepMode` types) | keep | Describes the compile-time `.Deps()` API, not the CLI flag; accurate as-is |
| `docs/specs/architecture.md:42` (`MultiError`/CollectAllErrors results) | keep | Result-classification semantics; unaffected by this change |
| `docs/specs/requirements.md:49,175` (CollectAllErrors requirement, `.Deps()` API) | keep | Compile-time API semantics; accurate as-is |
| `docs/specs/tests.md:14` (`TestDepModeString`) | keep | Test-name listing; accurate |
| `docs/specs/use-cases.md:48`; also `:10` (added by manual read, not the grep) | keep | Describes target-level parallelism and the builder API, not the runtime-flag enumeration |
| `docs/archive/architecture.md`, `docs/archive/design.md`, `docs/archive/requirements.md` | keep | Archived historical specs; archive convention is no retro-edits — and archive/design.md:398,526 + archive/architecture.md:331 already document `--dep-mode` |
| `docs/plans/2026-02-13-dep-group-chaining.md`, `docs/plans/2026-02-13-dep-group-chaining-design.md`, `docs/plans/2026-02-14-help-system-overhaul.md`, `docs/plans/2026-02-14-help-system-overhaul-design.md`, `docs/plans/2026-02-19-parallel-output.md`, `docs/plans/2026-07-11-coverage-gate-diagnostics.md`, `docs/plans/2026-07-23-coverage-leg-timeout.md` | keep | Point-in-time plan/design records; the 2026-02-13 design doc documents the override, and 2026-07-23-coverage-leg-timeout.md already records the flatten caveat |
| `session-learning-prompt.md` | keep | Session artifact about check-full history; no flag enumeration |
| `projects/portable-targets/architecture.md:194` | keep | Struct-field listing in a project design doc, not a CLI flag enumeration |
| `docs/specs/state.toml` | N/A | No new spec items added (existing IMPL/REQ/T text edits only), so the layer-item lists are unchanged |

Newly-misleading-by-omission check applied to every "keep" (vault note 383): each keep either describes the compile-time API (unchanged by this work), is an archived/point-in-time record, or already documents the flag/caveat. The one surface where the question is live — the README table — is the deferred proposal above, explicitly awaiting Joe's decision rather than silently kept or silently folded in.

## Baseline (RED) — measured against the working tree before edits

```
grep -c -- '--dep-mode' docs/specs/implementation.md   → 1
grep -c -- '--dep-mode' docs/specs/architecture.md     → 0
grep -c -- '--dep-mode' docs/specs/tests.md            → 0
grep -c -- '--dep-mode' docs/specs/requirements.md     → 0
grep -c -- '--dep-mode' README.md                      → 0   (README stays 0 in this cycle)
```

Note on counting semantics: `grep -c` counts matching LINES, not occurrences. Expected post-edit counts below account for that (verified by demonstration and independently by the Gate A code-alignment review).

---

### Task 1: Add `--dep-mode` to the four bare flag enumerations

**Files:**
- Modify: `docs/specs/implementation.md:40`
- Modify: `docs/specs/architecture.md:21`
- Modify: `docs/specs/tests.md:34`
- Modify: `docs/specs/requirements.md:21`

**Interfaces:** none (prose edits). Placement rule: append `` `--dep-mode` `` as the last element of each existing list, preserving each list's internal order and comma/backtick style.

- [ ] **Step 1: Verify baseline (RED)**

Run: `grep -c -- '--dep-mode' docs/specs/architecture.md docs/specs/tests.md docs/specs/requirements.md`
Expected: `0` for all three files, confirming absence.

- [ ] **Step 2: Edit `docs/specs/implementation.md` (IMPL-5 Purpose line)**

Replace (old):

```
target execution with runtime overrides (`--times`, `--timeout`, `--watch`, `--cache`, `--parallel`).
```

With (new):

```
target execution with runtime overrides (`--times`, `--timeout`, `--watch`, `--cache`, `--parallel`, `--dep-mode`).
```

- [ ] **Step 3: Edit `docs/specs/architecture.md` (ARCH extraction list)**

Replace (old):

```
`RuntimeOverrides` extract `--times`, `--timeout`, `--watch`, `--cache`, `--parallel`, `--retry`, `--backoff` from args before target-specific parsing.
```

With (new):

```
`RuntimeOverrides` extract `--times`, `--timeout`, `--watch`, `--cache`, `--parallel`, `--retry`, `--backoff`, `--dep-mode` from args before target-specific parsing.
```

- [ ] **Step 4: Edit `docs/specs/tests.md` (acceptance enumeration)**

Replace (old):

```
runtime overrides (`--times`, `--timeout`, `--parallel`, `--retry`, `--backoff`, `--watch`, `--cache`) are applied
```

With (new):

```
runtime overrides (`--times`, `--timeout`, `--parallel`, `--retry`, `--backoff`, `--watch`, `--cache`, `--dep-mode`) are applied
```

- [ ] **Step 5: Edit `docs/specs/requirements.md` (REQ-3 list)**

Replace (old):

```
Built-in runtime flags (`--times`, `--timeout`, `--watch`, `--cache`, `--parallel`, `--retry`, `--backoff`) modify execution behavior.
```

With (new):

```
Built-in runtime flags (`--times`, `--timeout`, `--watch`, `--cache`, `--parallel`, `--retry`, `--backoff`, `--dep-mode`) modify execution behavior.
```

- [ ] **Step 6: Verify presence (GREEN)**

Run: `grep -c -- '--dep-mode' docs/specs/implementation.md docs/specs/architecture.md docs/specs/tests.md docs/specs/requirements.md`
Expected: `implementation.md:2`, `architecture.md:1`, `tests.md:1`, `requirements.md:1`.

### Task 2: Add the flatten/CollectAllErrors caveat to the IMPL-15 registry entry

**Files:**
- Modify: `docs/specs/implementation.md:121` (IMPL-15 section)

**Interfaces:** none. The caveat is a NEW `**Caveat:**` line inserted directly after the `**Purpose:**` line, keeping the section's labeled-line structure (Purpose / Caveat / Key types / Traces to) instead of growing the Purpose paragraph into a run-on. Spec prose uses the bare type name `CollectAllErrors`, matching existing spec style (`docs/specs/architecture.md:7`: "`DepOption` (CollectAllErrors)").

- [ ] **Step 1: Insert the Caveat line**

Replace (old):

```
plus the deprecated `--no-cache` alias). Flag mode classification (targ-only vs. binary mode). Placeholder definitions.
```

With (new):

```
plus the deprecated `--no-cache` alias). Flag mode classification (targ-only vs. binary mode). Placeholder definitions.
**Caveat:** `--dep-mode` (`serial`|`parallel`) overrides how a target's dependency groups execute; the override flattens all dependency groups into a single group and drops the `CollectAllErrors` option — e.g. `--dep-mode serial` on `check-full` reverts it to fail-fast on the first error (behavioral fix tracked in #26).
```

- [ ] **Step 2: Verify (GREEN)**

Run: `grep -n 'CollectAllErrors' docs/specs/implementation.md`
Expected: exactly one matching line — the new `**Caveat:**` line in IMPL-15.
Run: `grep -c -- '--dep-mode' docs/specs/implementation.md`
Expected: `3` (`grep -c` counts lines: the IMPL-5 line from Task 1, the IMPL-15 registry list line, and the new Caveat line — the Caveat line contains `--dep-mode` twice but counts once).

### Task 3: Full verification, scope check, commit, close issue

**Files:** none new; commit the four spec files plus this plan file's revisions.

- [ ] **Step 1: Run the full check suite**

Run: `targ check-full`
Expected: green (exit 0). Doc edits must not affect it; any failure blocks the commit regardless of cause.

- [ ] **Step 2: Scope-containment check**

Run: `git diff --stat`
Expected: exactly `docs/specs/implementation.md`, `docs/specs/architecture.md`, `docs/specs/tests.md`, `docs/specs/requirements.md` (and possibly this plan file if it has uncommitted revisions). `README.md` present → STOP: the optional follow-up was executed without approval. Any other file → stop and investigate before committing.

- [ ] **Step 3: Commit (gate D applies to the message)**

```bash
git add docs/specs/implementation.md docs/specs/architecture.md docs/specs/tests.md docs/specs/requirements.md
git commit -m "docs(specs): add --dep-mode to runtime-flag enumerations with CollectAllErrors caveat

Adds --dep-mode to the four spec flag lists that omitted it (IMPL-5,
ARCH extraction list, tests acceptance, REQ-3) and documents the
flatten/CollectAllErrors caveat as an IMPL-15 Caveat line.
Doc-only; the flatten behavior itself is tracked in #26.

Closes #28

AI-Used: [claude]"
```

- [ ] **Step 4: Close the issue** (auto-closes via `Closes #28` on push; if not pushed, close explicitly)

```bash
gh issue close 28 --comment "Documented across the five spec sites; caveat recorded as an IMPL-15 Caveat line. Behavior fix remains #26."
```

---

## Optional Follow-up F: README Runtime Flags row — NEEDS JOE'S YES/NO, not executed in this cycle

If Joe approves, a follow-up commit makes these two edits to `README.md`:

**F.1 — table row.** README table style for required values is an uppercase placeholder (`--times N`, `--timeout DURATION`), so the row uses `MODE` rather than the optional-arg bracket style of `--completion [bash\|zsh\|fish]`. README prose uses the package-qualified name `targ.CollectAllErrors`, matching `README.md:380,383`.

Replace (old):

```
| `--while CMD`         | Run while shell command succeeds             |
```

With (new):

```
| `--while CMD`         | Run while shell command succeeds             |
| `--dep-mode MODE`     | Override dependency mode (`serial` or `parallel`) |
```

**F.2 — caveat note.**

Replace (old):

```
Runtime flags conflict with compile-time config by default (see [No Surprises](#no-surprises)). Use `targ.Disabled` to allow CLI override.
```

With (new):

```
Runtime flags conflict with compile-time config by default (see [No Surprises](#no-surprises)). Use `targ.Disabled` to allow CLI override.

Note: `--dep-mode` flattens all of a target's [dependency groups](#dependencies) into a single group and drops `targ.CollectAllErrors` — e.g. `--dep-mode serial` on `check-full` reverts it to fail-fast on the first error.
```

**F verification.** `grep -c -- '--dep-mode' README.md` → `2`. No issue-number reference in the README text (README cites no issues anywhere; the #26 pointer lives in the spec, an internal doc — deliberate per-audience asymmetry).

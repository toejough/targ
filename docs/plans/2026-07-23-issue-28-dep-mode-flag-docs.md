# Issue #28: Document `--dep-mode` in Spec Flag Enumerations — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--dep-mode` to every spec enumeration of built-in runtime flags, document the flatten/CollectAllErrors caveat where the flag's semantics are described, and bring the README runtime-flags table in line — doc-only, no behavior changes.

**Architecture:** Six text edits across five files (`docs/specs/implementation.md`, `docs/specs/architecture.md`, `docs/specs/tests.md`, `docs/specs/requirements.md`, `README.md`), verified by grep counts and `targ check-full`. The TDD analogue for doc edits: baseline grep proves absence (RED), edit, grep proves presence with expected counts (GREEN).

**Tech Stack:** Markdown, grep, `targ check-full`, `gh`.

## Global Constraints

- Doc-only: no changes under `internal/`, `cmd/`, or any `.go` file. `git diff --stat` at the end must show ONLY the five doc files above plus this plan file.
- `targ check-full` must be green before commit (CLAUDE.md: no pre-existing-failure exceptions).
- Commit trailer is `AI-Used: [claude]` — NOT Co-Authored-By (Joe's global CLAUDE.md).
- Behavior facts the caveat text must match (verified against main, this session): `--dep-mode` accepts `serial`|`parallel` (`internal/core/override.go:521-549`); applying it flattens ALL dep groups into a single group and the rebuilt group carries only targets+mode, dropping `CollectAllErrors` (`internal/core/command.go:1944-1966`); therefore `--dep-mode serial` on `check-full` reverts it to fail-fast on first error. Do not fix the behavior — #26 owns that; if #26 later changes the flatten/drop, the wording gets revisited then.

## Scope note (deviation from the issue's literal site list, recorded)

The issue names five spec sites. Since it was filed, commit `fbc5179` already added the bare flag to the IMPL-15 registry list (site 2), so that site only needs the caveat sentence. This plan adds ONE surface beyond the issue's list: the README "Runtime Flags" table (`README.md:700-709`), which enumerates exactly the same runtime override flags — once the specs list `--dep-mode` as a runtime flag, a user-facing table omitting it misleads by omission (vault note 383: the same registry-omission pattern failed Gate C in issue #27's cycle).

## Doc-surface enumeration grep — disposition list

Grep run against the working tree (2026-07-23): `grep -rniE 'dep-mode|depmode|dependency mode|collectallerrors' --include='*.md' .`

| Surface | Disposition | Reason |
| --- | --- | --- |
| `docs/specs/implementation.md:40` (IMPL-5 Purpose, runtime-overrides list) | **update** | Issue site 1 — flag missing from list |
| `docs/specs/implementation.md:121` (IMPL-15 flag registry) | **update** | Issue site 2 — flag already listed (fbc5179); caveat sentence still missing |
| `docs/specs/architecture.md:21` (`RuntimeOverrides` extraction list) | **update** | Issue site 3 — flag missing |
| `docs/specs/tests.md:34` (runtime-overrides acceptance list) | **update** | Issue site 4 — flag missing |
| `docs/specs/requirements.md:21` (REQ-3 built-in runtime flags) | **update** | Issue site 5 — flag missing |
| `README.md:700-709` (Runtime Flags table) | **update** | Scope addition (see Scope note) — table becomes newly misleading by omission |
| `docs/specs/architecture.md:7` (Target struct, `DepMode` types) | keep | Describes the compile-time `.Deps()` API, not the CLI flag; accurate as-is |
| `docs/specs/architecture.md:42` (`MultiError`/CollectAllErrors results) | keep | Result-classification semantics; unaffected by this change |
| `docs/specs/requirements.md:49,175` (CollectAllErrors requirement, `.Deps()` API) | keep | Compile-time API semantics; accurate as-is |
| `docs/specs/tests.md:14` (`TestDepModeString`) | keep | Test-name listing; accurate |
| `docs/specs/use-cases.md:10,48` (`.Deps`/`--parallel` interactions) | keep | Describes target-level parallelism and the builder API, not the runtime-flag enumeration |
| `docs/archive/*` (architecture.md, design.md, requirements.md) | keep | Archived historical specs; archive convention is no retro-edits — and archive/design.md:398,526 + archive/architecture.md:331 already document `--dep-mode` |
| `docs/plans/*` (dep-group-chaining 2026-02-13, help-system 2026-02-14, coverage-leg-timeout 2026-07-23, parallel-output 2026-02-19, coverage-gate-diagnostics 2026-07-11) | keep | Point-in-time plan/design records; the 2026-02-13 design doc documents the override, and 2026-07-23-coverage-leg-timeout.md already records the flatten caveat |
| `session-learning-prompt.md` | keep | Session artifact about check-full history; no flag enumeration |
| `projects/portable-targets/architecture.md:194` | keep | Struct-field listing in a project design doc, not a CLI flag enumeration |
| `docs/specs/state.toml` | N/A | No new spec items added (existing IMPL/REQ/T text edits only), so the layer-item lists are unchanged |

Newly-misleading-by-omission check applied to every "keep" (vault note 383): each keep either describes the compile-time API (unchanged by this work), is an archived/point-in-time record, or already documents the flag/caveat. The one surface that flipped to misleading under that test — the README table — is dispositioned **update** above.

## Baseline (RED) — measured against the working tree before edits

```
grep -c -- '--dep-mode' docs/specs/implementation.md   → 1
grep -c -- '--dep-mode' docs/specs/architecture.md     → 0
grep -c -- '--dep-mode' docs/specs/tests.md            → 0
grep -c -- '--dep-mode' docs/specs/requirements.md     → 0
grep -c -- '--dep-mode' README.md                      → 0
```

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
- Modify: `docs/specs/implementation.md:121` (IMPL-15 **Purpose:** paragraph)

**Interfaces:** none. The caveat appends to the existing Purpose paragraph on the same line (IMPL sections are `**Purpose:** / **Key types:** / **Traces to:**` lines; no new section labels).

- [ ] **Step 1: Edit the IMPL-15 Purpose paragraph**

Replace (old):

```
plus the deprecated `--no-cache` alias). Flag mode classification (targ-only vs. binary mode). Placeholder definitions.
```

With (new):

```
plus the deprecated `--no-cache` alias). Flag mode classification (targ-only vs. binary mode). Placeholder definitions. `--dep-mode` (`serial`|`parallel`) overrides how a target's dependency groups execute; the override flattens all dep groups into a single group and drops the `CollectAllErrors` option — e.g. `--dep-mode serial` on `check-full` reverts it to fail-fast on the first error (behavioral fix tracked in #26).
```

- [ ] **Step 2: Verify (GREEN)**

Run: `grep -n 'CollectAllErrors' docs/specs/implementation.md`
Expected: exactly one matching line, inside the IMPL-15 Purpose paragraph.
Run: `grep -c -- '--dep-mode' docs/specs/implementation.md`
Expected: `2` (the IMPL-5 line from Task 1, and the IMPL-15 line — list mention and caveat share one line).

### Task 3: Add `--dep-mode` to the README Runtime Flags table with the caveat note

**Files:**
- Modify: `README.md:700-711` (Runtime Flags table + footnote paragraph)

**Interfaces:** none. Table style: escape the pipe in the value placeholder as `serial\|parallel` (matches `--completion [bash\|zsh\|fish]` at README.md:690).

- [ ] **Step 1: Add the table row**

Replace (old):

```
| `--while CMD`         | Run while shell command succeeds             |
```

With (new):

```
| `--while CMD`         | Run while shell command succeeds             |
| `--dep-mode [serial\|parallel]` | Override how dependency groups execute |
```

- [ ] **Step 2: Add the caveat note after the conflict paragraph**

Replace (old):

```
Runtime flags conflict with compile-time config by default (see [No Surprises](#no-surprises)). Use `targ.Disabled` to allow CLI override.
```

With (new):

```
Runtime flags conflict with compile-time config by default (see [No Surprises](#no-surprises)). Use `targ.Disabled` to allow CLI override.

Note: `--dep-mode` flattens all of a target's dependency groups into a single group and drops `targ.CollectAllErrors` — e.g. `--dep-mode serial` on `check-full` reverts it to fail-fast on the first error.
```

- [ ] **Step 3: Verify (GREEN)**

Run: `grep -c -- '--dep-mode' README.md`
Expected: `2` (table row + note).

### Task 4: Full verification, scope check, commit, close issue

**Files:** none new; commit everything above plus this plan file.

- [ ] **Step 1: Run the full check suite**

Run: `targ check-full`
Expected: green (exit 0). Doc edits must not affect it; any failure blocks the commit regardless of cause.

- [ ] **Step 2: Scope-containment check**

Run: `git diff --stat`
Expected: exactly `README.md`, `docs/specs/implementation.md`, `docs/specs/architecture.md`, `docs/specs/tests.md`, `docs/specs/requirements.md` (plus the already-committed plan file showing no further changes). Any other file → stop and investigate before committing.

- [ ] **Step 3: Commit (gate D applies to the message)**

```bash
git add README.md docs/specs/implementation.md docs/specs/architecture.md docs/specs/tests.md docs/specs/requirements.md
git commit -m "docs(specs): add --dep-mode to runtime-flag enumerations with CollectAllErrors caveat

Adds --dep-mode to the four spec flag lists that omitted it (IMPL-5,
ARCH extraction list, tests acceptance, REQ-3), documents the
flatten/CollectAllErrors caveat in the IMPL-15 registry entry, and
adds the flag to the README Runtime Flags table with the same caveat.
Doc-only; the flatten behavior itself is tracked in #26.

Closes #28

AI-Used: [claude]"
```

- [ ] **Step 4: Close the issue** (auto-closes via `Closes #28` on push; if not pushed, close explicitly)

```bash
gh issue close 28 --comment "Documented across the five spec sites plus the README Runtime Flags table; caveat recorded in IMPL-15 and README. Behavior fix remains #26."
```

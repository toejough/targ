# OpenSpec Adoption Implementation Plan — revision 2

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Retire three abandoned spec frameworks, consolidate every surviving document into a single shrinking `old-docs/` directory, and stand up OpenSpec as the destination the documentation migrates *into* — replacing old content with OpenSpec specs wherever an audit proves the old content wrong, organized by domain-driven bounded context.

**Architecture:** This cycle does **not** reach the final end state; it establishes the migration that gets there. `old-docs/` holds every surviving document and is the source of truth *only where it is provably correct*. Where the audit finds a claim wrong, that content is deleted from `old-docs/` and its corrected form is written as an OpenSpec spec under the bounded context it belongs to. `old-docs/` therefore shrinks monotonically: files that empty are deleted, and when the directory empties it is deleted along with every reference to it — at which point the repo holds only `README.md`, `CLAUDE.md`, the `.claude/` harness, and `openspec/`. A ratio gate (below) triggers a final conversion push before the tail drags on.

**Tech Stack:** OpenSpec 1.6.0 (`@fission-ai/openspec`, at `/opt/homebrew/bin/openspec`); Go 1.25.5; `targ` as build/test/lint runner.

## Global Constraints

- **The migration standard (this is the deliverable, not just the mechanics).** Documented in `README.md` and in `openspec/config.yaml`'s `context:`:
  1. `old-docs/` is legacy documentation. It is authoritative *only* for claims verified correct against the code.
  2. When a claim in `old-docs/` is found incorrect, the fix is **not** to edit it in place. Delete the incorrect content from `old-docs/` and write its corrected form as an OpenSpec spec.
  3. OpenSpec specs are organized by **domain / bounded context** — one capability directory per bounded context, derived from the code's own seams, not from the old documents' chapter structure.
  4. When `old-docs/` content falls below **20%** of total spec content, convert the remainder in one final push.
  5. A file that empties is deleted. When `old-docs/` empties, delete the directory and every reference to it.
- **No bulk port, no blanket backfill.** OpenSpec's `docs/existing-projects.md`: *"You do not document your whole codebase to start"*; *"Resist the urge to back-fill everything."* A spec is written here only where the audit **proves an old claim wrong** — provable incorrectness is the trigger, never "this area lacks a spec". Accurate old content stays in `old-docs/` until rule 4 fires.
- **User-ratified scope** (asked and answered 2026-07-28):
  - Delete outright: `docs/plans/`, `specs/001-parallel-output/`, `projects/portable-targets/`, `.specify/` + `.claude/commands/speckit.*`, `.claude/skills/specification-layers/`.
  - Everything else surviving — including `state.toml`, `updates.jsonl`, `session-learning-prompt.md` — is moved to `old-docs/` and audited on the same terms as the rest.
  - Legacy docs live in **`old-docs/`** (this supersedes an earlier `openspec/archive/` answer).
- **Author's dissent, recorded and resolved:** the author first recommended bulk backfill, which OpenSpec's own documentation refutes; the user then designed this incorrectness-triggered migration instead. It is implemented wholeheartedly. No task revisits it.
- **Counts are re-derived, never copied from this plan.** A baseline taken before this plan was committed is already off by one in `docs/`, `docs/plans/`, and the total, because the plan file is itself tracked under `docs/plans/`. Every task that acts on a file set re-derives it mechanically against the live tree at execution time.
- **Verified environment facts** (probed 2026-07-28 in throwaway repos, and independently re-verified by a second agent):
  - `openspec init --tools claude .` writes exactly `openspec/config.yaml`, empty `openspec/{specs,changes,changes/archive}`, 6 `.claude/commands/opsx/*.md`, 6 `.claude/skills/openspec-*/SKILL.md`.
  - **`openspec validate` is NOT a valid pre-init failure check** — with no `openspec/` root it exits **0**, silently treating cwd as an implicit root and reporting zero items. Use **`openspec doctor`**, which exits 1 with "No OpenSpec root found from the current directory."
  - An unmanaged sibling directory of markdown is inert to `validate`/`doctor`/`list`.
  - Empty directories are not committed by git — `openspec/specs/` and `openspec/changes/archive/` need `.gitkeep`.
  - `gh` is authenticated and `origin` is `git@github.com:toejough/targ.git`; 7 issues open.
  - No `.go` file, `go:generate`, `go:embed`, or `dev/` target references any path this plan moves or deletes — verified by grep. A build break here would be surprising and must be root-caused, not worked around.
- **Commits:** Conventional Commits, and every commit ends with the trailer `AI-Used: [claude]` — never `Co-Authored-By`.
- **No pre-existing failures accepted.** `targ check-full` must be green before this cycle is done.

---

## File Structure

**Created:** `openspec/config.yaml`; `openspec/specs/.gitkeep`; `openspec/changes/archive/.gitkeep`; `openspec/specs/<bounded-context>/spec.md` (one per context where the audit found incorrectness); `old-docs/README.md`; `.claude/commands/opsx/*.md` (6) and `.claude/skills/openspec-*/SKILL.md` (6), both written by `openspec init`.

**Moved into `old-docs/`:** `docs/specs/*` (6), `docs/archive/*` (6), `state.toml`, `updates.jsonl`, `session-learning-prompt.md`.

**Modified:** `CLAUDE.md` (stale generated header only — see Task 6's exact non-contiguous ranges); `README.md` (gains the migration standard).

**Deleted:** `docs/plans/`, `specs/`, `projects/`, `.specify/`, `.claude/commands/speckit.*`, `.claude/skills/specification-layers/`, and any `old-docs/` file the audit empties.

---

### Task 1: Retire the three abandoned frameworks

Doing this first shrinks the surface every later task has to reason about, and it is the one fully-ratified, dependency-free deletion.

**Files:** Delete `docs/plans/`, `specs/`, `projects/`, `.specify/`, `.claude/commands/speckit.*.md`, `.claude/skills/specification-layers/`

**Interfaces:** Consumes nothing. Produces a tree where `docs/` holds only `specs/` and `archive/`.

- [ ] **Step 1: Write the failing check**

```bash
cd /Users/joe/repos/personal/targ
git ls-files -- specs projects .specify '.claude/commands/speckit.*' '.claude/skills/specification-layers' | wc -l
```

- [ ] **Step 2: Run it to verify it fails**

Expected: a non-zero count (37 at the time of writing: 9 + 5 + 12 + 9 + 2). Re-derive rather than trusting that number.

- [ ] **Step 3: Delete, re-deriving the set from the live tree**

```bash
cd /Users/joe/repos/personal/targ
git rm -r --quiet specs projects .specify .claude/skills/specification-layers
git ls-files -z -- '.claude/commands/speckit.*' | xargs -0 git rm --quiet
git rm -r --quiet docs/plans
```

`docs/plans/` includes this plan file. That is intended — the plan is already committed to git history, which retains it.

- [ ] **Step 4: Run the check to verify it passes**

```bash
cd /Users/joe/repos/personal/targ
git ls-files -- specs projects .specify '.claude/commands/speckit.*' '.claude/skills/specification-layers' docs/plans | wc -l
```

Expected: `0`.

- [ ] **Step 5: Commit**

```bash
cd /Users/joe/repos/personal/targ
git commit -q -F - <<'EOF'
chore: retire spec-kit, specification-layers, and the plan archive

Three spec frameworks accumulated in this repo and only one was ever
live. Removes spec-kit's scaffolding, templates and nine slash commands;
the specification-layers skill; the abandoned 001-parallel-output and
portable-targets doc sets; and the dated plan archive. All retained in
git history.

AI-Used: [claude]
EOF
```

---

### Task 2: Initialize OpenSpec and write the migration standard into its context

**Files:** Create `openspec/config.yaml` (via `openspec init`, then edited), `openspec/specs/.gitkeep`, `openspec/changes/archive/.gitkeep`. Tool-created: `.claude/commands/opsx/*.md`, `.claude/skills/openspec-*/SKILL.md`.

**Interfaces:** Consumes nothing. Produces the `openspec/` root and the `context:` string that every later `/opsx:` invocation receives.

- [ ] **Step 1: Write the failing check**

```bash
cd /Users/joe/repos/personal/targ
openspec doctor; echo "exit=$?"
```

Use `doctor`, not `validate` — `validate` exits 0 with no root and would give a false green.

- [ ] **Step 2: Run it to verify it fails**

Expected: `exit=1`, message "No OpenSpec root found from the current directory."

- [ ] **Step 3: Initialize and configure**

```bash
cd /Users/joe/repos/personal/targ
openspec init --tools claude .
mkdir -p openspec/specs openspec/changes/archive
touch openspec/specs/.gitkeep openspec/changes/archive/.gitkeep
```

Replace the commented-out stub in `openspec/config.yaml` with:

```yaml
schema: spec-driven

context: |
  targ: a Go build system and CLI-from-functions library (Go 1.25.5). Public API at
  the repo root (targ.go), implementation under internal/, CLI binary at cmd/targ,
  build targets in dev/, examples in examples/. Design principle: "No Surprises" —
  conflicts error rather than resolving silently.

  Build, test and lint run through targ itself, never `go test` directly:
  `targ test`, `targ check-full` (lint, per-function 80% coverage, declaration
  ordering, dead code, integration tests), `targ reorder-decls`. check-full must be
  green before any change is done; pre-existing failures are not accepted.

  Conventions: no new package-level mutable state (use dependency injection);
  declarations ordered types-then-vars-then-funcs, alphabetical within each group;
  no flaky tests (assert on error types, never elapsed time); no IO mocking (tag
  genuine IO tests `//go:build integration`, name them `TestIntegration...`);
  property-based tests via rapid, assertions via gomega.

  DOCUMENTATION MIGRATION — in progress. This repo is moving from legacy docs to
  OpenSpec specs, incrementally:
    - old-docs/ holds the legacy documentation. It is the source of truth ONLY for
      claims verified correct against the code. Anything there may be wrong; check
      the source before relying on it.
    - openspec/specs/ is the destination and is authoritative wherever it exists.
    - When a claim in old-docs/ is found INCORRECT, do not edit it in place: delete
      the incorrect content from old-docs/ and write its corrected form as an
      OpenSpec spec. Proven incorrectness is the only trigger for writing a spec
      about code you are not otherwise changing.
    - Specs are organized by domain / bounded context — one capability directory per
      bounded context, derived from the code's own seams.
    - When old-docs/ content falls below 20% of total spec content, convert the
      remainder in one final push. A file that empties is deleted; when old-docs/
      empties, delete the directory and every reference to it.
  Note old-docs/ is unrelated to openspec/changes/archive/, which is OpenSpec's own
  store of completed changes.
```

- [ ] **Step 4: Run the check to verify it passes**

```bash
cd /Users/joe/repos/personal/targ
openspec doctor; echo "exit=$?"
openspec validate --all --strict; echo "exit=$?"
```

Expected: `doctor` reports "OpenSpec root: ok", `exit=0`; `validate` reports "No items found to validate", `exit=0` — an empty spec tree is intended at this point.

- [ ] **Step 5: Commit**

```bash
cd /Users/joe/repos/personal/targ
git add openspec .claude/commands/opsx .claude/skills
git commit -q -F - <<'EOF'
feat(openspec): initialize OpenSpec with targ's context and migration standard

openspec/specs/ starts empty by design. The context: block carries targ's
stack and conventions plus the old-docs -> openspec migration rule, so
every /opsx: invocation receives it.

AI-Used: [claude]
EOF
```

---

### Task 3: Consolidate every surviving document into `old-docs/`

**Files:** Create `old-docs/README.md`. Move `docs/specs/*` (6), `docs/archive/*` (6), `state.toml`, `updates.jsonl`, `session-learning-prompt.md` into `old-docs/`.

**Interfaces:** Consumes Task 1 (the deletions are done, so what remains is exactly what moves). Produces the `old-docs/` tree that Task 4 audits.

- [ ] **Step 1: Write the failing check**

```bash
cd /Users/joe/repos/personal/targ
test -d old-docs; echo "exit=$?"
```

- [ ] **Step 2: Run it to verify it fails**

Expected: `exit=1`.

- [ ] **Step 3: Move, preserving history**

Two basename collisions exist across the sources — `architecture.md` and `requirements.md` each appear in both `docs/specs/` and `docs/archive/` (verified: these two are the *only* collisions). Keep the two provenances in subdirectories so neither clobbers the other and the audit can tell them apart:

```bash
cd /Users/joe/repos/personal/targ
mkdir -p old-docs/traced old-docs/first-gen
git mv docs/specs/use-cases.md      old-docs/traced/
git mv docs/specs/requirements.md   old-docs/traced/
git mv docs/specs/architecture.md   old-docs/traced/
git mv docs/specs/tests.md          old-docs/traced/
git mv docs/specs/implementation.md old-docs/traced/
git mv docs/specs/state.toml        old-docs/traced/
git mv docs/archive/architecture.md                  old-docs/first-gen/
git mv docs/archive/design.md                        old-docs/first-gen/
git mv docs/archive/requirements.md                  old-docs/first-gen/
git mv docs/archive/tasks.md                         old-docs/first-gen/
git mv docs/archive/issues.md                        old-docs/first-gen/
git mv docs/archive/research-gomod-tree-traversal.md old-docs/first-gen/
git mv session-learning-prompt.md old-docs/
git mv state.toml                 old-docs/
git mv updates.jsonl              old-docs/
rmdir docs/specs docs/archive docs 2>/dev/null || true
```

Write `old-docs/README.md`:

```markdown
# old-docs

Legacy documentation, migrating into `openspec/specs/`. **Authoritative only where
verified correct against the code** — anything here may be stale; check the source
before relying on it.

- `traced/` — a four-layer traced record (use cases → requirements/design →
  architecture → tests → implementation; `state.toml` holds the item rosters and
  layer topology). The most recently maintained of the two sets.
- `first-gen/` — the original architecture, design, requirements, task and issue
  docs, plus a research note on `go.mod` tree traversal. Older; predates much of
  the code it describes.
- `session-learning-prompt.md`, `state.toml`, `updates.jsonl` — process artifacts
  from earlier workflow runs.

## How this directory shrinks

When a claim here is found incorrect, it is **not** edited in place. The incorrect
content is deleted from this directory and its corrected form is written as an
OpenSpec spec under the bounded context it belongs to. A file that empties is
deleted; when this directory empties, it is deleted along with every reference to
it. When what remains here falls below 20% of total spec content, the remainder is
converted in one push.

Unrelated to `openspec/changes/archive/`, which is OpenSpec's own store of
completed changes.
```

- [ ] **Step 4: Re-derive broken pointers, then run the check**

Moved content keeps its claims verbatim; only its *pointers* are re-derived. Sweep for paths the move or Task 1 invalidated:

```bash
cd /Users/joe/repos/personal/targ
grep -rnE 'docs/(specs|plans|archive)|specs/001|projects/portable|\.specify' old-docs/ || echo "no stale path refs"
test -d old-docs && test ! -d docs; echo "exit=$?"
```

Expected: `exit=0`, and either "no stale path refs" or a short list, each then fixed. One hit is known: `old-docs/session-learning-prompt.md:27` cites `docs/plans/2026-02-21-help-source-attribution.md`, deleted in Task 1 — replace the path with a note naming the file as available in git history, and leave the surrounding lesson intact.

- [ ] **Step 5: Commit**

```bash
cd /Users/joe/repos/personal/targ
git add -A old-docs docs session-learning-prompt.md state.toml updates.jsonl
git commit -q -F - <<'EOF'
docs: consolidate surviving documentation into old-docs/

One shrinking directory, split by provenance: traced/ (the four-layer
record) and first-gen/ (the original docs), plus the loose process
artifacts. Claims move verbatim; only pointers broken by the move or by
the framework deletions are re-derived.

AI-Used: [claude]
EOF
```

---

### Task 4: Audit `old-docs/`, and replace what is wrong with OpenSpec specs

This is the cycle's substantive work. It is the only task that writes to `openspec/specs/`, and it may write there **only** for content the audit proves incorrect.

**Files:** Modify/delete files under `old-docs/`. Create `openspec/specs/<bounded-context>/spec.md`.

**Interfaces:** Consumes Task 2's `openspec/` root and Task 3's `old-docs/` tree. Produces the initial spec set and a shrunken `old-docs/`.

- [ ] **Step 1: Derive the bounded contexts from the code, before auditing anything**

Specs are organized by the code's own seams, not by the old documents' chapters. Dispatch one agent to read `targ.go`, `cmd/targ/`, and each `internal/*` package and propose the bounded-context set, returning for each: a kebab-case id, a one-line responsibility, and the packages/files that constitute it. Require that every `internal/` package land in exactly one context.

Record the result in the plan's execution notes. Every spec written in Step 4 uses an id from this set.

- [ ] **Step 2: Write the failing check — the audit, with a recorded verdict**

The "test" for a document is a decision procedure with a measurable outcome. Dispatch parallel auditors, one per `old-docs/` file, each returning a **markdown table with exactly these columns** so five agents produce comparable output:

| Item | Claim (quoted) | Code location (`file:line`) or NOT FOUND | Verdict | True behavior |
|---|---|---|---|---|

`Verdict` is exactly one of `CORRECT`, `INCORRECT`, `ORPHANED` (names code that no longer exists). For `INCORRECT`/`ORPHANED`, `True behavior` quotes the actual code or test at `file:line`; prose only where the behavior is genuinely diffuse. Below the table: `CORRECT: n / INCORRECT: n / ORPHANED: n`, and the bounded-context id each incorrect item belongs to.

For files with no numbered items (`state.toml`, `updates.jsonl`, `session-learning-prompt.md`, `README`-ish prose), the unit of audit is the individual claim or record, not a numbered id.

Omission check, per file: an omission cannot be found by grepping the file for its own terms. Derive the expected set from the *code*: run `git log --oneline --since=<the file's last-edit date>`, read each commit's diff summary, and list any exported API, behavior, or subsystem added since that the file does not mention. Report those as `INCORRECT` (incomplete) with the commit that introduced them.

- [ ] **Step 3: Run the audit and confirm it is live**

Expected: a verdict table per file with at least one `INCORRECT` or `ORPHANED`, or an explicit all-`CORRECT` verdict carrying per-item evidence. A known-stale seed proves the audit is not rubber-stamping: `old-docs/traced/state.toml`'s `[cursor].next_action` reads "Re-entry #3 complete" while four `[tree.*]` history strings describe a Re-entry #4. An audit reporting zero findings *and* missing this is not trustworthy — re-run it.

- [ ] **Step 4: Replace incorrectness with specs**

For each bounded context that has ≥1 `INCORRECT`/`ORPHANED` item, write `openspec/specs/<context>/spec.md` in OpenSpec's **main-spec** format — not the delta format:

```markdown
# <Context> Specification

## Purpose

<≥50 characters. What this bounded context is responsible for, and where its
behavior lives in the code.>

## Requirements

### Requirement: <name>
<Statement. MUST contain SHALL or MUST.>

#### Scenario: <name>
- **WHEN** <condition>
- **THEN** <expected outcome>
```

Format constraints, enforced by `openspec validate`: `## Purpose` and `## Requirements` are both required; every requirement contains SHALL or MUST; every requirement has ≥1 scenario; scenarios use **exactly four** hashes — three hashes or bullets fail silently. Do not use `## ADDED Requirements` (that is the delta format, for `openspec/changes/`).

Each requirement states the **corrected** behavior, verified at `file:line`. Then delete the superseded content from its `old-docs/` file. Delete any file that empties.

- [ ] **Step 5: Verify, measure the ratio, and commit**

```bash
cd /Users/joe/repos/personal/targ
openspec validate --all --strict; echo "exit=$?"
OLD=$(find old-docs -name '*.md' -o -name '*.toml' -o -name '*.jsonl' 2>/dev/null | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
NEW=$(find openspec/specs -name 'spec.md' 2>/dev/null | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
echo "old-docs=$OLD openspec=$NEW ratio=$(awk -v o="${OLD:-0}" -v n="${NEW:-0}" 'BEGIN{t=o+n; print (t?o/t:1)}')"
```

Expected: `exit=0` and every written spec listed as passing. Record the ratio — if it is already below `0.20`, the final conversion push (Global Constraint 4) is due now; report that to the user rather than starting it unasked.

```bash
cd /Users/joe/repos/personal/targ
git add -A old-docs openspec
git commit -q -F - <<'EOF'
docs(openspec): replace audited-incorrect legacy content with specs

Audits every old-docs file against the code. Content verified correct
stays put; content proven wrong is deleted there and rewritten as an
OpenSpec spec under its bounded context. Emptied files removed.

AI-Used: [claude]
EOF
```

---

### Task 5: De-stale CLAUDE.md

**Files:** Modify `CLAUDE.md`.

**Interfaces:** Consumes Tasks 1–3 (the generator is gone; `openspec/` and `old-docs/` exist to name).

- [ ] **Step 1: Write the failing check**

```bash
cd /Users/joe/repos/personal/targ
grep -nE 'src/|tests/|Last updated: 2026-02-19|001-parallel-output|Add commands for Go|Auto-generated from all feature plans' CLAUDE.md
```

- [ ] **Step 2: Run it to verify it fails**

Expected: exactly 6 hits, at lines 3, 7, 12, 13, 18, 31. Each is false — targ has no `src/` or `tests/` directory (it has `internal/`, `cmd/`, `dev/`, `examples/`), `## Commands` is an empty stub, and `001-parallel-output` names a directory Task 1 deleted. The generator that wrote this header (`.specify/scripts/bash/update-agent-context.sh`) is also gone, so nothing re-imposes it.

- [ ] **Step 3: Replace only the generated ranges**

**The stale material is non-contiguous.** Delete lines **1–18** (title through the empty `## Commands` stub) and lines **29–32** (`## Recent Changes` and its bullet). **Lines 20–27 — the `## Code Style` block — are hand-written project rules and MUST be preserved verbatim**, exactly as the Testing Rules block is. Deleting a contiguous 1–32 would destroy four real engineering rules; do not do that.

Also drop the now-purposeless `<!-- MANUAL ADDITIONS START -->` / `<!-- MANUAL ADDITIONS END -->` markers, since no generator remains to protect the block from.

The replacement header, which goes above the preserved `## Code Style` block:

```markdown
# targ Development Guidelines

## Project Structure

Public API at the repo root (`targ.go`); implementation under `internal/`; CLI
binary at `cmd/targ/`; build targets in `dev/`; examples in `examples/`.

## Specs

`openspec/specs/` is the spec of record, organized by bounded context. Legacy
documentation lives in `old-docs/` and is authoritative only where verified
correct against the code — when you find a claim there wrong, delete it and write
the corrected form as an OpenSpec spec rather than editing it in place. See
`README.md` for the full migration standard.

## Commands

Build, test and lint run through targ, never `go test` directly:
`targ test`, `targ check-full`, `targ reorder-decls`.
```

- [ ] **Step 4: Run the check to verify it passes**

```bash
cd /Users/joe/repos/personal/targ
grep -nE 'src/|tests/|Last updated: 2026-02-19|001-parallel-output|Add commands for Go|Auto-generated from all feature plans' CLAUDE.md || echo "clean"
grep -c 'No new package-level mutable state' CLAUDE.md
grep -c 'No flaky tests' CLAUDE.md
```

Expected: `clean`, then `1` and `1` — proving both hand-written blocks survived.

- [ ] **Step 5: Commit**

```bash
cd /Users/joe/repos/personal/targ
git add CLAUDE.md
git commit -q -F - <<'EOF'
docs: replace CLAUDE.md's stale generated header with the real layout

The spec-kit header claimed a src//tests/ layout targ has never had, an
empty Commands stub and a changelog for a deleted directory. The
hand-written Code Style and Testing Rules blocks are preserved verbatim.

AI-Used: [claude]
EOF
```

---

### Task 6: Document the migration standard in README.md

**Files:** Modify `README.md` (861 lines; currently a pure user manual with no development section).

**Interfaces:** Consumes Tasks 2–4. Produces the user-facing statement of the standard.

- [ ] **Step 1: Write the failing check**

```bash
cd /Users/joe/repos/personal/targ
grep -nE 'old-docs|openspec' README.md || echo "absent"
```

- [ ] **Step 2: Run it to verify it fails**

Expected: `absent`. README today describes only how to *use* targ; it says nothing about how the project's specs are organized, so a contributor has no way to learn the standard from it.

- [ ] **Step 3: Append a Development section**

Add at the end of README.md, as a new top-level section:

```markdown
## Specifications

targ's behavior is specified in `openspec/specs/`, organized by domain — one
capability directory per bounded context, derived from the code's own seams.
Specs are managed with [OpenSpec](https://github.com/Fission-AI/OpenSpec):
`/opsx:propose` → `/opsx:apply` → `/opsx:archive`, and validated with
`openspec validate --all --strict`.

`old-docs/` holds the project's legacy documentation while it migrates. It is
authoritative **only** where its claims have been verified against the code. The
migration rule is:

1. Find a claim in `old-docs/` that is wrong.
2. Delete the wrong content from `old-docs/` — do not edit it in place.
3. Write its corrected form as an OpenSpec spec under the bounded context it
   belongs to.
4. Delete any `old-docs/` file that empties.

Proven incorrectness is the only trigger for writing a spec about code you are not
otherwise changing — specs written speculatively go stale. When `old-docs/` content
falls below 20% of total spec content, the remainder is converted in one push; when
the directory empties it is deleted, along with every reference to it.
```

- [ ] **Step 4: Run the check to verify it passes**

```bash
cd /Users/joe/repos/personal/targ
grep -nE 'old-docs|openspec' README.md | head
```

Expected: several hits in the new section.

- [ ] **Step 5: Commit**

```bash
cd /Users/joe/repos/personal/targ
git add README.md
git commit -q -F - <<'EOF'
docs: state the openspec migration standard in the README

README described only how to use targ. Adds the specification section:
where specs live, how they are organized, and the rule that governs
old-docs shrinking into openspec/specs.

AI-Used: [claude]
EOF
```

---

### Task 7: Verify the cycle

**Files:** none modified.

- [ ] **Step 1: Run the OpenSpec gate**

```bash
cd /Users/joe/repos/personal/targ
openspec doctor
openspec validate --all --strict; echo "exit=$?"
openspec list --specs
```

Expected: root ok; `exit=0`; every spec Task 4 wrote listed.

- [ ] **Step 2: Run the repo gate**

```bash
cd /Users/joe/repos/personal/targ
targ check-full
```

Expected: green. Any failure is either pre-existing (still must be fixed) or caused by a moved file — root-cause it, do not suppress it.

- [ ] **Step 3: Verify the end state from a clean clone**

```bash
cd "$(mktemp -d)" && git clone -q /Users/joe/repos/personal/targ t && cd t
find openspec -type d | sort
git ls-files | grep -vE '\.go$' | grep -vE 'testdata|\.golden$|override\.sum$' | sort
```

Expected directories: `openspec`, `openspec/changes`, `openspec/changes/archive`, `openspec/specs` (plus a directory per written spec). If `openspec/specs` or `openspec/changes/archive` is absent, the `.gitkeep` files were not committed — fix and re-verify.

Expected file listing: `.gitignore`; `.claude/commands/opsx/*` (6); `.claude/skills/openspec-*/SKILL.md` (6); `CLAUDE.md`; `README.md`; `assets/targ.png`; `dev/golangci-{fast,fmt,lint,todos}.toml`; `go.mod`; `go.sum`; `openspec/config.yaml`; `openspec/specs/.gitkeep`; `openspec/changes/archive/.gitkeep`; the written specs; `old-docs/**` (whatever the audit left). **No** `docs/`, `specs/`, `projects/`, `.specify/`, `speckit`, or `specification-layers` entry may remain.

- [ ] **Step 4: Report the ratio and what remains**

State the `old-docs`-to-spec ratio from Task 4 Step 5, the list of specs written, and the `old-docs/` files still standing. This is a migration in progress, not a finished state — say so in the voice the evidence supports.

---

## Self-Review

**Ask coverage.** "examine all the existing specs/docs" → Task 4's audit covers every surviving file, and Task 1 disposes of the rest. "delete anything entirely stale/unused" → Task 1 (the ratified frameworks) and Task 4 Step 4 (content the audit proves wrong). "update anything only partially stale to match reality" → Task 4's replace-with-spec mechanism, Task 5 (CLAUDE.md), Task 6 (README). "don't port; move the docs to an archive directory" → Task 3. "set up the openspec config to point at those docs as context" → Task 2 Step 3. "use DDD domains/bounded contexts" → Task 4 Step 1 and every spec path. "<20% triggers a final conversion push" → Global Constraint 4, measured in Task 4 Step 5. "document this as the standard in the README and openspec config" → Tasks 6 and 2.

**"Agent files."** No `AGENTS.md` exists and OpenSpec 1.6.0 does not generate one; the ask's "claude/agent files" is read as `CLAUDE.md` plus the `.claude/` command and skill surface. Stated here rather than left implicit.

**End state.** This cycle deliberately does **not** reach "only README/CLAUDE/agent files and openspec docs." `old-docs/` survives by design and shrinks under the standard until it empties. Task 7 Step 4 reports how far along that is instead of claiming completion.

**Gate A findings folded in.** Non-contiguous CLAUDE.md ranges with the Code Style block explicitly protected (was: a contiguous delete that destroyed it); `openspec doctor` as the pre-init failure check (was: `validate`, which exits 0 with no root); `AI-Used: [claude]` on every commit; no cross-task verdict handoff (the audit and its consequences are one task); no dangling task reference; counts re-derived rather than copied.

**Known gap, stated rather than hidden.** The audit is an LLM reading code — verification, not proof. Every spec it produces cites `file:line`, and `old-docs/README.md` and the `context:` block both say the legacy material is authoritative only where verified, so nothing downstream treats an unaudited claim as settled.

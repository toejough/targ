# OpenSpec Adoption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Retire three abandoned spec frameworks from the targ repo, verify-and-correct the one live spec tree, move the surviving documentation into `openspec/archive/` as background context, and initialize OpenSpec with a `config.yaml` `context:` block that points every future agent at that archive — leaving only `README.md`, `CLAUDE.md`, the `.claude/` harness, and `openspec/` as the repo's non-code surface.

**Architecture:** OpenSpec is adopted *empty*, per its own brownfield doctrine. `openspec/specs/` starts with no capability specs and accretes one `/opsx:propose → apply → archive` cycle at a time. Existing documentation is neither ported nor back-filled: it is triaged (delete what is dead, correct what is merely drifted), then relocated verbatim under `openspec/archive/` in two tiers — `specs/` for the accuracy-verified traced tree, `historical/` for superseded first-generation docs. The single supported lever for surfacing that archive to AI agents is the free-text `context:` string in `openspec/config.yaml`, which OpenSpec injects into every artifact prompt as an `<context>` block.

**Tech Stack:** OpenSpec 1.6.0 (`@fission-ai/openspec`, installed at `/opt/homebrew/bin/openspec`); Go 1.25.5; `targ` as the build/test/lint runner.

## Global Constraints

- **Do not port and do not back-fill.** OpenSpec's `docs/existing-projects.md` states: *"You do not document your whole codebase to start"* and *"Resist the urge to back-fill everything. Writing specs for code you aren't changing feels productive and usually isn't. Those specs go stale, because nothing forces them to track reality."* No task in this plan may create a file under `openspec/specs/`.
- **Archived docs are moved verbatim.** Content correction happens in Task 1 and Task 2, *before* the move. Task 4 moves bytes and re-derives *pointers only* — never re-words claims.
- **User-ratified scope decisions** (asked and answered 2026-07-28):
  - `openspec/specs/` starts **empty**; no backfill this cycle or as a follow-up.
  - Delete: `docs/plans/`, `specs/001-parallel-output/`, `projects/portable-targets/`, `.specify/` + `.claude/commands/speckit.*`, `.claude/skills/specification-layers/`.
  - Archive location: **`openspec/archive/`**.
- **Author's dissent, recorded and resolved:** the author initially recommended back-filling specs from code, on the precedent of engram and pi-vim-mode. OpenSpec's own documentation refutes that recommendation; the user's "start empty" choice is the doctrinally correct one and is implemented wholeheartedly. No task revisits it.
- **Measured baseline (2026-07-28, `git ls-files`):** 213 tracked files total. `docs/` 37 (`plans/` 25, `specs/` 6, `archive/` 6); `specs/` 9; `projects/` 5; `.specify/` 12; `.claude/commands/speckit.*` 9; `.claude/skills/specification-layers/` 2; root strays 3.
- **Verified environment facts** (probed 2026-07-28 in a throwaway git repo, not this one):
  - `openspec init --tools claude .` writes exactly `openspec/config.yaml`, the empty dirs `openspec/{specs,changes,changes/archive}`, plus 6 `.claude/commands/opsx/*.md` and 6 `.claude/skills/openspec-*/SKILL.md`.
  - An unmanaged `openspec/archive/` containing markdown is **inert**: `openspec validate --all --strict` exits 0 with "No items found to validate"; `openspec list --specs/--changes` unaffected; `openspec doctor` reports "OpenSpec root: ok".
  - Empty directories are not committed by git, so `openspec/specs/` and `openspec/changes/archive/` require `.gitkeep` files.
- **No pre-existing failures accepted.** `targ check-full` must be green before this cycle is declared done (repo CLAUDE.md rule).

---

## File Structure

**Created:**
- `openspec/config.yaml` — schema selection + the `context:` block (the only OpenSpec-supported pointer to external docs)
- `openspec/specs/.gitkeep`, `openspec/changes/archive/.gitkeep` — make the empty accretion points survive a clone
- `openspec/archive/README.md` — states what the archive is, its two tiers, and that it is background, not the spec of record
- `openspec/archive/specs/` — relocated `docs/specs/` (5 md + `state.toml`), accuracy-verified in Task 1
- `openspec/archive/historical/` — relocated superseded docs, kept verbatim
- `.claude/commands/opsx/*.md` (6), `.claude/skills/openspec-*/SKILL.md` (6) — written by `openspec init`

**Modified:**
- `CLAUDE.md` — remove the spec-kit generated header (stale: claims `src/`/`tests/` layout, "Last updated: 2026-02-19", a `001-parallel-output` changelog, and an empty `## Commands` stub); keep the hand-maintained rules
- `docs/specs/*.md` — corrected in place in Task 1, before relocation

**Deleted:**
- `docs/plans/` (25), `specs/` (9), `projects/` (5), `.specify/` (12), `.claude/commands/speckit.*` (9), `.claude/skills/specification-layers/` (2), `state.toml`, `updates.jsonl`
- Whichever of `docs/archive/tasks.md`, `docs/archive/issues.md` Task 2 finds fully resolved

---

### Task 1: Verify and correct the live traced spec tree

`docs/specs/` is the repo's only live specification record — 6 UC, 18 REQ, 7 DES, 16 ARCH, 25 T, 20 IMPL, last edited 2026-07-28. It is the tier of the archive that must be *true*, because `config.yaml`'s `context:` will point future agents at it. This task is the "update anything only partially stale to match reality" half of the ask.

**Files:**
- Modify: `docs/specs/use-cases.md`, `docs/specs/requirements.md`, `docs/specs/architecture.md`, `docs/specs/tests.md`, `docs/specs/implementation.md`, `docs/specs/state.toml`

**Interfaces:**
- Consumes: nothing (first task)
- Produces: a corrected `docs/specs/` tree that Task 4 relocates verbatim, and an audit report listing every claim checked and every correction made

- [ ] **Step 1: Write the failing check — an accuracy audit with a recorded verdict**

The "test" for a doc is a decision procedure with a measurable outcome. Dispatch **five parallel auditors**, one per spec file, each charged to verify every claim in its file against the code at `HEAD` and return a structured verdict:

```
For each numbered item (UC-n / REQ-n / DES-n / ARCH-n / T-n / IMPL-n) in <file>:
  - Quote the claim.
  - Name the code/test that satisfies it (file:line) or state NOT FOUND.
  - Verdict: ACCURATE | DRIFTED (claim is stale, state the true behavior) | ORPHANED (names code that no longer exists)
Also: does the file omit anything that shipped? Derive the expected set from the
code (git log since the file's last edit), do not grep for the file's own terms —
an omission cannot be found by searching for what is not written.
Return: a table of every item + a count of ACCURATE / DRIFTED / ORPHANED.
```

- [ ] **Step 2: Run it and confirm it finds something**

Expected: at least one DRIFTED or ORPHANED item, OR an explicit all-ACCURATE verdict with per-item evidence. A known-stale seed to confirm the audit is live: `docs/specs/state.toml` `[cursor].next_action` reads `"Re-entry #3 complete"` while four `[tree.*]` history strings describe a Re-entry #4 — the cursor is one re-entry behind. An audit that reports zero findings *and* misses this is not trustworthy; re-run it.

- [ ] **Step 3: Apply the corrections**

Fix every DRIFTED and ORPHANED item to state the true behavior. Add entries for anything that shipped and is missing. Correct `state.toml`'s `[cursor].next_action` to reflect the true latest re-entry. Do **not** restructure, renumber, or re-word accurate items.

- [ ] **Step 4: Re-run the audit to verify**

Expected: zero DRIFTED, zero ORPHANED, and the omission check clean.

- [ ] **Step 5: Commit**

```bash
git add docs/specs/
git commit -m "docs(specs): correct the spec tree against the code before archiving"
```

---

### Task 2: Triage the two disposable first-generation docs against reality

`docs/archive/tasks.md` (1106 lines, a mostly-`pending` task ledger) and `docs/archive/issues.md` (228 lines, 21 `ISSUE-0xx` entries under Active/Completed/Blocked) are candidates for outright deletion — but only if nothing in them is live. The repo now tracks work in GitHub issues. Deleting a still-open bug would be a real loss; keeping a fully-resolved ledger is dead weight.

**Files:**
- Delete or keep: `docs/archive/tasks.md`, `docs/archive/issues.md`

**Interfaces:**
- Consumes: nothing
- Produces: a delete/keep verdict per file, with evidence, consumed by Task 4 (which relocates whatever survives) and Task 5 (which deletes the rest)

- [ ] **Step 1: Write the failing check — enumerate every open item**

```bash
cd /Users/joe/repos/personal/targ
grep -nE '^\s*[-*]?\s*(ISSUE-[0-9]+|TASK-[0-9]+)' docs/archive/issues.md docs/archive/tasks.md | head -60
gh issue list --state open --limit 100 --json number,title
```

- [ ] **Step 2: Run it and record the counts**

Expected: a concrete list of `Active`/`Blocked`/`pending` entries and the current open GitHub issues. Write both counts down — they are the evidence for the verdict.

- [ ] **Step 3: Resolve each open entry to one of three states**

For every `Active`/`Blocked`/`pending` entry, determine by reading the code at `HEAD`:
- **shipped** — the behavior exists; the entry is dead
- **tracked** — a corresponding open GitHub issue exists; the entry is redundant
- **live and untracked** — neither; this entry is real work nobody is holding

- [ ] **Step 4: Apply the verdict**

- If a file has **zero** live-and-untracked entries → delete it in Task 5.
- If it has **any** → keep the file, relocate it to `openspec/archive/historical/` in Task 4, and report the live entries to the user at close-out. Do **not** silently file new issues for them; deferral needs an explicit check-in.

- [ ] **Step 5: Commit the verdict record**

No commit if both files are deleted (Task 5 handles it). If either survives, no content change is made here either — this task produces only a verdict. Record it in the close-out report.

---

### Task 3: Initialize OpenSpec and write the context block

**Files:**
- Create: `openspec/config.yaml` (via `openspec init`, then edited), `openspec/specs/.gitkeep`, `openspec/changes/archive/.gitkeep`
- Create (by the tool): `.claude/commands/opsx/*.md` (6), `.claude/skills/openspec-*/SKILL.md` (6)

**Interfaces:**
- Consumes: nothing
- Produces: `openspec/` root that Task 4 populates under `archive/`, and the `context:` string that is this cycle's primary deliverable

- [ ] **Step 1: Write the failing check**

```bash
cd /Users/joe/repos/personal/targ
openspec validate --all --strict; echo "exit=$?"
```

- [ ] **Step 2: Run it to verify it fails**

Expected: FAIL — no `openspec/` root exists yet. (Confirm the error names a missing OpenSpec root, not something else.)

- [ ] **Step 3: Initialize and configure**

```bash
cd /Users/joe/repos/personal/targ
openspec init --tools claude .
mkdir -p openspec/specs openspec/changes/archive
touch openspec/specs/.gitkeep openspec/changes/archive/.gitkeep
```

Then replace the commented-out stub in `openspec/config.yaml` with a real `context:` block. It must (a) describe targ, (b) name `openspec/archive/` and both its tiers explicitly, (c) state that the archive is background — *not* the spec of record, (d) name the build commands, (e) disambiguate `openspec/archive/` from OpenSpec's own `openspec/changes/archive/`. Write it as:

```yaml
schema: spec-driven

context: |
  targ: a Go build system and CLI-from-functions library (Go 1.25.5). Public API at
  the repo root (targ.go), implementation under internal/, CLI binary at cmd/targ,
  build targets in dev/. Design principle: "No Surprises" — conflicts error rather
  than resolving silently.

  Build, test, and lint run through targ itself, never through go test directly:
  `targ test`, `targ check-full` (the full gate: lint, per-function 80% coverage,
  declaration ordering, dead code, integration tests). `targ check-full` must be
  green before any change is done; pre-existing failures are not accepted.

  Conventions: no new package-level mutable state (use dependency injection);
  declarations ordered types-then-vars-then-funcs, alphabetical within each group
  (`targ reorder-decls`); no flaky tests (assert on error types, never elapsed
  time); no IO mocking (tag genuine IO tests `//go:build integration` and name them
  `TestIntegration...`); property-based tests via rapid, assertions via gomega.

  Background documentation lives in openspec/archive/ and is NOT the spec of record
  — openspec/specs/ is, and it starts empty and accretes from archived changes.
  Treat the archive as source material for exploration when proposing a change:
    - openspec/archive/specs/ — a traced four-layer record (use cases → requirements
      and design → architecture → tests → implementation), verified against the code
      on 2026-07-28. The most reliable description of current behavior.
    - openspec/archive/historical/ — superseded first-generation design, architecture
      and requirements docs, kept verbatim as history. Older than the code; verify any
      claim from here against the source before relying on it.
  Note openspec/archive/ (legacy documentation) is distinct from
  openspec/changes/archive/ (OpenSpec's own completed-change store).
```

- [ ] **Step 4: Run the check to verify it passes**

```bash
cd /Users/joe/repos/personal/targ
openspec validate --all --strict; echo "exit=$?"
openspec doctor
```

Expected: `exit=0` with "No items found to validate" (correct — `specs/` is intentionally empty), and `doctor` reporting `OpenSpec root: ok`.

- [ ] **Step 5: Commit**

```bash
git add openspec .claude/commands/opsx .claude/skills/openspec-*
git commit -m "feat(openspec): initialize OpenSpec with targ's project context"
```

---

### Task 4: Relocate the surviving documentation into `openspec/archive/`

**Files:**
- Create: `openspec/archive/README.md`
- Move: `docs/specs/*` → `openspec/archive/specs/`; `docs/archive/{architecture,design,requirements,research-gomod-tree-traversal}.md` + any survivor from Task 2 + `session-learning-prompt.md` → `openspec/archive/historical/`

**Interfaces:**
- Consumes: Task 1's corrected `docs/specs/`; Task 2's delete/keep verdict; Task 3's `openspec/` root
- Produces: the archive tree that `config.yaml`'s `context:` block already names

- [ ] **Step 1: Write the failing check**

```bash
cd /Users/joe/repos/personal/targ
test -d openspec/archive/specs && test -d openspec/archive/historical; echo "exit=$?"
```

- [ ] **Step 2: Run it to verify it fails**

Expected: `exit=1` — neither directory exists.

- [ ] **Step 3: Move with `git mv` (preserves history), then write the archive README**

```bash
cd /Users/joe/repos/personal/targ
mkdir -p openspec/archive/specs openspec/archive/historical
git mv docs/specs/use-cases.md      openspec/archive/specs/
git mv docs/specs/requirements.md   openspec/archive/specs/
git mv docs/specs/architecture.md   openspec/archive/specs/
git mv docs/specs/tests.md          openspec/archive/specs/
git mv docs/specs/implementation.md openspec/archive/specs/
git mv docs/specs/state.toml        openspec/archive/specs/
git mv docs/archive/architecture.md                openspec/archive/historical/
git mv docs/archive/design.md                      openspec/archive/historical/
git mv docs/archive/requirements.md                openspec/archive/historical/
git mv docs/archive/research-gomod-tree-traversal.md openspec/archive/historical/
git mv session-learning-prompt.md                  openspec/archive/historical/
# plus, only if Task 2 kept them:
# git mv docs/archive/tasks.md  openspec/archive/historical/
# git mv docs/archive/issues.md openspec/archive/historical/
```

Name collision check before moving: `docs/specs/architecture.md` and `docs/archive/architecture.md` share a basename but land in *different* tiers (`specs/` vs `historical/`), so no clobber. Likewise `requirements.md`. Verify with `ls openspec/archive/specs openspec/archive/historical` after the moves — six files and five (or seven) files respectively.

Then write `openspec/archive/README.md`:

```markdown
# Archive

Background documentation for targ, kept as source material for exploration when
proposing a change. **This is not the spec of record.** `openspec/specs/` is —
it starts empty and accretes as changes are archived.

## `specs/` — traced four-layer record

A use-cases → requirements/design → architecture → tests → implementation tree
(`state.toml` holds the item rosters and layer topology). Every claim was verified
against the code on 2026-07-28. This is the most reliable description of targ's
current behavior outside the source itself.

## `historical/` — superseded first-generation docs

The original architecture, design, and requirements docs, plus a research note on
`go.mod` tree traversal and a one-off session-learning prompt. Kept verbatim as
history. These predate the code they describe — verify any claim here against the
source before relying on it.

Not to be confused with `openspec/changes/archive/`, which is OpenSpec's own store
of completed changes.
```

- [ ] **Step 4: Re-derive cross-references, then run the check**

"Keep verbatim" is only safe for content whose references are self-contained. Sweep the moved files for pointers that the move broke — relative paths, `docs/` prefixes, and locality words (`above`, `below`, `this directory`) — and re-derive each against its new location. Claims stay verbatim; only pointers change.

```bash
cd /Users/joe/repos/personal/targ
grep -rnE 'docs/(specs|plans|archive)|specs/001|projects/portable|\.specify' openspec/archive/ || echo "no stale path refs"
test -d openspec/archive/specs && test -d openspec/archive/historical; echo "exit=$?"
openspec validate --all --strict; echo "exit=$?"
```

Expected: "no stale path refs" (or a short list, each then fixed), `exit=0` twice.

Known hit to resolve: `session-learning-prompt.md:27` cites `docs/plans/2026-02-21-help-source-attribution.md`, which Task 5 deletes. Replace the path with a note that the plan is available in git history, naming the file. Do not delete the surrounding lesson.

- [ ] **Step 5: Commit**

```bash
git add -A openspec/archive docs session-learning-prompt.md
git commit -m "docs: relocate surviving documentation into openspec/archive"
```

---

### Task 5: Delete the retired frameworks

**Files:**
- Delete: `docs/` (whatever remains), `specs/`, `projects/`, `.specify/`, `.claude/commands/speckit.*.md`, `.claude/skills/specification-layers/`, `state.toml`, `updates.jsonl`, and this plan file

**Interfaces:**
- Consumes: Task 4 (everything worth keeping has already moved out)
- Produces: the end state the ask specifies

- [ ] **Step 1: Write the failing check — the end-state assertion**

```bash
cd /Users/joe/repos/personal/targ
git ls-files | grep -vE '\.go$' | grep -vE 'testdata|\.golden$|override\.sum$' | sort
```

- [ ] **Step 2: Run it to verify it fails**

Expected: still lists `docs/plans/*`, `specs/*`, `projects/*`, `.specify/*`, `.claude/commands/speckit.*`, `.claude/skills/specification-layers/*`, `state.toml`, `updates.jsonl`.

- [ ] **Step 3: Re-derive the deletion set against the live tree, then delete**

Do not delete from the counts in this plan — the cycle's own moves have changed them. Re-derive mechanically at execution time:

```bash
cd /Users/joe/repos/personal/targ
git rm -r --quiet docs specs projects .specify .claude/skills/specification-layers
git rm --quiet .claude/commands/speckit.analyze.md .claude/commands/speckit.checklist.md \
  .claude/commands/speckit.clarify.md .claude/commands/speckit.constitution.md \
  .claude/commands/speckit.implement.md .claude/commands/speckit.plan.md \
  .claude/commands/speckit.specify.md .claude/commands/speckit.tasks.md \
  .claude/commands/speckit.taskstoissues.md
git rm --quiet state.toml updates.jsonl
```

`git rm -r docs` also removes this plan file — that is intended. The plan is committed in Task 0 (below) so git history retains it.

- [ ] **Step 4: Run the end-state check to verify it passes**

```bash
cd /Users/joe/repos/personal/targ
git ls-files | grep -vE '\.go$' | grep -vE 'testdata|\.golden$|override\.sum$' | sort
```

Expected exactly: `.gitignore`, `.claude/commands/opsx/*` (6), `.claude/skills/openspec-*/SKILL.md` (6), `CLAUDE.md`, `README.md`, `assets/targ.png`, `dev/golangci-{fast,fmt,lint,todos}.toml`, `go.mod`, `go.sum`, `openspec/config.yaml`, `openspec/specs/.gitkeep`, `openspec/changes/archive/.gitkeep`, `openspec/archive/README.md`, `openspec/archive/specs/*` (6), `openspec/archive/historical/*` (5 or 7).

No `docs/`, `specs/`, `projects/`, `.specify/`, `speckit`, or `specification-layers` entry may remain.

- [ ] **Step 5: Commit**

```bash
git commit -m "chore: retire spec-kit, specification-layers, and the plan archive"
```

---

### Task 6: De-stale CLAUDE.md

**Files:**
- Modify: `CLAUDE.md:1-32` (the spec-kit generated header)

**Interfaces:**
- Consumes: Task 3's `openspec/` (CLAUDE.md should name it), Task 5's deletions (the generator is gone)
- Produces: a CLAUDE.md whose every line is true

- [ ] **Step 1: Write the failing check**

```bash
cd /Users/joe/repos/personal/targ
grep -nE 'src/|tests/|Last updated: 2026-02-19|001-parallel-output|Add commands for Go|Auto-generated from all feature plans' CLAUDE.md
```

- [ ] **Step 2: Run it to verify it fails**

Expected: 6+ hits. Every one is false — targ has no `src/` or `tests/` directory (`ls` confirms `internal/`, `cmd/`, `dev/`), `## Commands` is an empty stub reading `# Add commands for Go 1.25.5`, and `001-parallel-output` names a spec-kit feature dir deleted in Task 5. The generator that wrote this header (`.specify/scripts/bash/update-agent-context.sh`) is also gone, so nothing will re-impose it.

- [ ] **Step 3: Replace the generated header**

Delete lines 1–32 (through `## Recent Changes` and its bullet) and the now-pointless `<!-- MANUAL ADDITIONS START/END -->` markers, since there is no longer a generator to protect the block from. Replace the header with an accurate one:

```markdown
# targ Development Guidelines

## Project Structure

Public API at the repo root (`targ.go`); implementation under `internal/`; CLI
binary at `cmd/targ/`; build targets in `dev/`; examples in `examples/`.

## Specs

`openspec/specs/` is the spec of record; it starts empty and accretes as changes
are archived (`/opsx:propose` → `/opsx:apply` → `/opsx:archive`). Background
documentation is in `openspec/archive/` — read it as source material, not as
current truth. See `openspec/archive/README.md`.

## Commands

Build, test, and lint run through targ, never `go test` directly:
`targ test`, `targ check-full`, `targ reorder-decls`.
```

Keep the existing Code Style and Testing Rules blocks verbatim — they are hand-maintained and accurate.

- [ ] **Step 4: Run the check to verify it passes**

```bash
cd /Users/joe/repos/personal/targ
grep -nE 'src/|tests/|Last updated: 2026-02-19|001-parallel-output|Add commands for Go|Auto-generated from all feature plans' CLAUDE.md || echo "clean"
```

Expected: `clean`.

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: replace CLAUDE.md's stale generated header with the real layout"
```

---

### Task 7: Verify the whole cycle

**Files:** none modified

**Interfaces:**
- Consumes: every prior task
- Produces: the evidence that closes the cycle

- [ ] **Step 1: Run the OpenSpec gate**

```bash
cd /Users/joe/repos/personal/targ
openspec validate --all --strict; echo "exit=$?"
openspec doctor
openspec list --specs
```

Expected: `exit=0`; `OpenSpec root: ok`; "No specs found." — an empty spec tree is the intended end state, not a failure.

- [ ] **Step 2: Run the repo gate**

```bash
cd /Users/joe/repos/personal/targ
targ check-full
```

Expected: green. This is a doc-only change, so any failure is either pre-existing (must still be fixed — the repo accepts no pre-existing failures) or caused by a deleted file the build referenced. Note the earlier grep proved no `.go` file references any deleted path, so a build break would be surprising and must be root-caused, not worked around.

- [ ] **Step 3: Verify from a clean clone that the empty dirs survived**

```bash
cd "$(mktemp -d)" && git clone -q /Users/joe/repos/personal/targ t && cd t
find openspec -type d | sort
```

Expected: `openspec`, `openspec/archive`, `openspec/archive/historical`, `openspec/archive/specs`, `openspec/changes`, `openspec/changes/archive`, `openspec/specs`. If `openspec/specs` or `openspec/changes/archive` is missing, the `.gitkeep` files were not committed — fix and re-verify.

- [ ] **Step 4: Repair the vault pointers this cycle broke**

Two notes cite paths that no longer exist:
- `374.2026-07-23.targ-gomod-research-doc-is-prior-work-for-cache-replace-changes` cites `docs/archive/research-gomod-tree-traversal.md` → now `openspec/archive/historical/research-gomod-tree-traversal.md`
- `401.2026-07-23.targ-dep-execution-prior-art-and-code-map` cites `docs/plans/2026-02-13-dep-group-chaining.md`, `-design.md` (deleted; available in git history) and `docs/archive/architecture.md` → now `openspec/archive/historical/architecture.md`

Amend both with `engram amend --target <note> --subject/--predicate/--object` so the pointers resolve.

- [ ] **Step 5: No commit** — this task only verifies.

---

## Self-Review

**Spec coverage:** "examine all existing specs/docs" → the inventory in Task 1/2 preamble plus the measured baseline. "delete anything entirely stale/unused" → Tasks 2 and 5. "update anything only partially stale to match reality" → Tasks 1 and 6. "don't port" → Global Constraints, enforced by "no task may create a file under `openspec/specs/`". "move all the updated docs to an archive directory" → Task 4. "set up the openspec config to point at those docs as context for future specs" → Task 3 Step 3. "only README/claude/agent files and the openspec docs left" → Task 5 Step 4's exact expected listing.

**Placeholders:** none — every command is written out, and every count in this plan was measured against the tree on 2026-07-28 rather than recalled.

**Consistency:** the archive tiers are named `openspec/archive/specs/` and `openspec/archive/historical/` in Task 3's context block, Task 4's moves, Task 4's README, Task 5's expected listing, and Task 6's CLAUDE.md text — identically in all five.

**Known gap, stated rather than hidden:** the accuracy of `openspec/archive/specs/` rests on Task 1's audit, which is an LLM reading code. It is verification, not proof. The `context:` block therefore dates the claim ("verified against the code on 2026-07-28") instead of asserting it timelessly.

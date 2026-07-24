# Issue #33: Track Golden Fixtures Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `targ check-full` pass in a fresh clone by tracking the four golden fixtures that `.gitignore`'s blanket `testdata/` currently hides.

**Architecture:** `.gitignore:11` is a bare `testdata/` (comment above it: "Rapid/property test artifacts"), which matches every `testdata/` directory at any depth and therefore also hides `internal/runner/testdata/golden/*.golden` — the four fixtures `TestGoldenFile_HelpOutput` (`internal/runner/runner_help_test.go:16-80`) reads. Narrow the pattern to the artifacts it was written for, and stage the fixtures.

**Tech Stack:** git ignore semantics, Go test fixtures, `targ` build system for verification.

## Global Constraints

- **The replacement pattern is `**/testdata/rapid/`, NOT the issue's `testdata/rapid/`.** A pattern containing a middle slash is anchored to the `.gitignore`'s own directory, so `testdata/rapid/` only matches `<root>/testdata/rapid/` and would silently stop ignoring all five nested rapid dirs. Verified empirically in a scratch repo (`git check-ignore -q` per representative path): under `testdata/rapid/`, `internal/core/testdata/rapid/b.fail` and `test/testdata/rapid/c.fail` both report **visible**; under `**/testdata/rapid/`, both report **IGNORED** and `internal/runner/testdata/golden/d.golden` stays **visible**. This deviation from the issue text is deliberate — see Design notes.
- **Scope review is set equality, not subset** (vault 360, 150): stage explicit paths; the staged set must equal exactly the four `.golden` files plus `.gitignore`. Verify with `git diff --cached --name-only` before committing — never `git add -A`/`.`.
- **Verify optional consumers positively** (vault 420): the golden test reads fixtures from disk; "the command exited 0" is not evidence. The fresh-clone probe must assert the four files EXIST in the clone and that `TestGoldenFile_HelpOutput`'s four subtests actually RAN and compared.
- **Full-suite green gates sequence AFTER the commit** (vault 395): `targ check-full`'s `check-uncommitted` leg fails by design on a dirty tree. Pre-commit, expect every leg green EXCEPT `check-uncommitted`.
- **TDD RED is mandatory** and here means: observe the fresh-clone failure BEFORE the fix, from a clone of the current `HEAD`.
- Commit trailer is `AI-Used: [claude]`.

## Design notes

- **Why the issue's pattern is wrong (deviation record).** Issue #33 prescribes narrowing `testdata/` → `testdata/rapid/` and dropping `test/testdata/`. The second half is right; the first half is a defect. Root-anchoring means the nested rapid dirs (`internal/core/`, `internal/discover/`, `internal/help/`, `internal/parse/`, `internal/runner/`, `test/`) would all become visible, exposing hundreds of `.fail` artifacts to `git status` and to any future `git add`. Implementing `**/testdata/rapid/` delivers the issue's stated Expected (fixtures tracked, rapid artifacts still ignored) rather than its literal prescribed text.
- **Blast radius is exactly four files.** With the corrected pattern applied and `git status --porcelain` run against the real tree, the only newly-visible path is `internal/runner/testdata/` — and inside it, `rapid/` remains ignored, leaving exactly `golden/{create,sync,to-func,to-string}.golden`. Every other `testdata/` directory in the repo contains nothing but `rapid/` (verified by `find`), so nothing else can be swept in.
- **Dropping `test/testdata/` (line 9) is safe.** `test/testdata/` contains only `rapid/`, which `**/testdata/rapid/` still ignores; the line is redundant under both the old and new patterns.
- **Already-tracked `.fail` files stay tracked.** `internal/core/testdata/rapid/*.fail` and `test/testdata/rapid/*.fail` are already in the index (ignore rules never untrack). This predates the issue, is unchanged by this fix, and is out of scope.
- **No new guard test (YAGNI).** A checked-in test asserting "the fixtures are tracked" would re-implement `git ls-files` in Go to defend a one-line config; the fresh-clone probe is the honest verification and is recorded here as the RED/GREEN evidence. If this class of breakage recurs, that is the moment to add a gate — not now.

## Doc-surface enumeration grep

Ran over the repo for `gitignore|testdata|golden|fixture` (`*.md` in `README.md`, `CLAUDE.md`, `docs/`, `specs/`), plus `testdata` in `*.yml|*.yaml|*.toml|*.json` and in `dev/*.go`:

| File | Disposition | Reason |
| --- | --- | --- |
| `docs/specs/tests.md:200,203,207` (T-? golden-file help output; `TestGoldenFile_HelpOutput` in the Tests roster) | keep | Describes what the golden test asserts; the fix changes fixture *tracking*, not the property or the test's behavior. Still accurate. |
| `dev/targets.go:849` (`name == "testdata"` → `filepath.SkipDir`) | keep | A source-walker skip for `.go` discovery, unrelated to git ignore rules; unaffected by tracking fixtures. |
| `README.md`, `CLAUDE.md` | N/A | Zero hits for gitignore/testdata/golden/fixture — no contributor-facing ignore convention is documented anywhere to update. |
| `docs/plans/*`, `docs/archive/*` | N/A | Historical records kept as written (grep for `gitignore`/`golden fixture` returned no plan-doc hits regardless). |
| `*.yml`/`*.yaml`/`*.toml`/`*.json` | N/A | Zero `testdata` hits — no CI or tooling config encodes the ignore convention. |

---

### Task 1: RED — prove a fresh clone fails today

**Files:**
- Create (scratch, deleted at close): `/private/tmp/claude-501/-Users-joe-repos-personal-targ/1ad2fbb5-120e-4fca-9525-fd94fcf75014/scratchpad/i33/`

**Interfaces:**
- Consumes: nothing.
- Produces: the RED evidence quoted into the Task 3 report.

- [ ] **Step 1: Clone the current HEAD and prove the fixtures are absent**

```bash
SCRATCH=/private/tmp/claude-501/-Users-joe-repos-personal-targ/1ad2fbb5-120e-4fca-9525-fd94fcf75014/scratchpad/i33
rm -rf "$SCRATCH" && mkdir -p "$SCRATCH"
git clone -q /Users/joe/repos/personal/targ "$SCRATCH/clone-red"
ls -la "$SCRATCH/clone-red/internal/runner/testdata/golden/" 2>&1
```

Expected: `No such file or directory` — the fixtures are untracked, so the clone has no `golden/` dir. This is the defect, stated positively.

- [ ] **Step 2: Run the golden test in the clone and capture the failure**

```bash
cd "$SCRATCH/clone-red" && go test ./internal/runner -run TestGoldenFile_HelpOutput -v 2>&1 | tail -30
```

Expected: FAIL — each of the four subtests fails at `failed to read golden file testdata/golden/<name>.golden`. Record the exact failure text; it is the RED evidence.

- [ ] **Step 3: Confirm the same test passes in the working tree (isolates the cause to tracking, not the test)**

```bash
cd /Users/joe/repos/personal/targ && go test ./internal/runner -run TestGoldenFile_HelpOutput 2>&1 | tail -5
```

Expected: PASS — proving the fixtures exist locally and only their absence from git causes the clone failure.

### Task 2: GREEN — narrow the ignore and track the fixtures

**Files:**
- Modify: `/Users/joe/repos/personal/targ/.gitignore` (lines 9-11)
- Add (newly tracked): `internal/runner/testdata/golden/create.golden`, `sync.golden`, `to-func.golden`, `to-string.golden`

**Interfaces:**
- Consumes: Task 1's confirmation that the clone lacks the fixtures.
- Produces: a commit whose tracked set the Task 3 probe re-clones.

- [ ] **Step 1: Edit `.gitignore`**

Current lines 9-11:

```
test/testdata/
# Rapid/property test artifacts
testdata/
```

Replace with:

```
# Rapid/property test artifacts (kept out of git; golden fixtures are tracked)
**/testdata/rapid/
```

That is: delete the redundant `test/testdata/` line, replace bare `testdata/` with `**/testdata/rapid/`, and extend the existing comment to say why the narrowing exists. Net: 3 lines → 2.

- [ ] **Step 2: Verify the pattern's anchoring and blast radius before staging**

```bash
cd /Users/joe/repos/personal/targ
for p in internal/core/testdata/rapid/x.fail test/testdata/rapid/x.fail internal/runner/testdata/rapid/x.fail; do
  git check-ignore -q "$p" && echo "IGNORED  $p" || echo "VISIBLE  $p"
done
git status --porcelain
```

Expected: all three rapid paths print `IGNORED`; `git status --porcelain` shows exactly `?? internal/runner/testdata/` plus the modified `.gitignore` (` M .gitignore`) and nothing else.

- [ ] **Step 3: Stage explicit paths only, then verify the staged set is exactly right**

```bash
git add .gitignore \
  internal/runner/testdata/golden/create.golden \
  internal/runner/testdata/golden/sync.golden \
  internal/runner/testdata/golden/to-func.golden \
  internal/runner/testdata/golden/to-string.golden
git diff --cached --name-only
```

Expected — exactly these five lines, no more (set equality, vault 360):

```
.gitignore
internal/runner/testdata/golden/create.golden
internal/runner/testdata/golden/sync.golden
internal/runner/testdata/golden/to-func.golden
internal/runner/testdata/golden/to-string.golden
```

If any other path appears, STOP and unstage it — do not proceed.

- [ ] **Step 4: Pre-commit checks**

```bash
targ check-full 2>&1 | tail -15
```

Expected: every leg PASS except `check-uncommitted` (dirty tree by design pre-commit — vault 395).

- [ ] **Step 5: Commit**

```bash
git commit -m "$(cat <<'EOF'
fix(repo): track golden fixtures hidden by the blanket testdata ignore

.gitignore's bare `testdata/` matched every testdata directory at any
depth, so internal/runner/testdata/golden/*.golden — the fixtures
TestGoldenFile_HelpOutput reads — were never committed, and the
coverage leg of check-full failed in every fresh clone and worktree.
The four fixtures were also one `git clean -fdx` from unrecoverable
loss.

Narrow the pattern to `**/testdata/rapid/`, the artifacts the comment
was written for. The `**/` prefix is required: a middle-slash pattern
is anchored to the .gitignore's own directory, so a bare
`testdata/rapid/` would have stopped ignoring all five nested rapid
directories. Drop the now-redundant `test/testdata/` line — that
directory contains only rapid artifacts.

Closes #33

AI-Used: [claude]
EOF
)"
```

- [ ] **Step 6: Refactor check + Gate B**

There is no code to restructure — the change is two config lines plus four data files. Confirm the result reads as originally intended (the comment now explains the narrowing; no dead or redundant patterns remain), then dispatch Gate B: a fresh-context design-fit reviewer over the full diff, charged with DRY/SRP/YAGNI and "does the result read as written-from-the-start". The unit is done only when Gate B closes.

### Task 3: Verify — fresh clone is green

- [ ] **Step 1: Re-clone the fixed HEAD and assert the fixtures are present**

```bash
SCRATCH=/private/tmp/claude-501/-Users-joe-repos-personal-targ/1ad2fbb5-120e-4fca-9525-fd94fcf75014/scratchpad/i33
git clone -q /Users/joe/repos/personal/targ "$SCRATCH/clone-green"
ls "$SCRATCH/clone-green/internal/runner/testdata/golden/"
```

Expected: all four `.golden` files listed — a positive assertion, not an absence of error (vault 420).

- [ ] **Step 2: Prove the golden subtests ran AND compared**

```bash
cd "$SCRATCH/clone-green" && go test ./internal/runner -run TestGoldenFile_HelpOutput -v 2>&1 | grep -E '^(=== RUN|--- (PASS|FAIL)|ok|FAIL)' 
```

Expected: four `--- PASS` subtest lines (create, sync, to-func, to-string) and a final `ok`. Four passing subtests — not merely `ok` — is the positive evidence that each fixture was read and compared.

- [ ] **Step 3: Prove rapid artifacts did NOT leak into the clone**

```bash
find "$SCRATCH/clone-green" -path '*testdata/rapid*' -name '*.fail' | wc -l
git -C "$SCRATCH/clone-green" ls-files | grep -c 'testdata/rapid/'
```

Expected: the first count is the number of already-tracked `.fail` files (pre-existing, unchanged by this fix); the second must equal it. Record both numbers. No NEW rapid artifacts became tracked — compare against `git -C /Users/joe/repos/personal/targ ls-files | grep -c 'testdata/rapid/'` at the pre-fix commit.

- [ ] **Step 4: Full suite on the clean committed tree**

```bash
cd /Users/joe/repos/personal/targ && targ check-full 2>&1 | tail -12
```

Expected: ALL legs PASS including `check-uncommitted`. No pre-existing-failure exemptions.

- [ ] **Step 5: Clean up scratch clones**

```bash
rm -rf "$SCRATCH"
```

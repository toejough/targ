# Issues #35 + #37: Git-Detection Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop `DetectRepoURL` from reporting the wrong repository's URL when a config cannot be read (#35), and make it work inside a `git worktree` (#37).

**Architecture:** `internal/core/git.go` holds a five-deep chain that all returns bare `string`: `DetectRepoURL` → `DetectRepoURLWithDeps` → `DetectRepoURLFromDirWithOpen` (walks up looking for `<dir>/.git/config`) → `ParseGitConfigOriginURLWithOpen` → `ParseGitConfigContent`. The internal chain gains `error` returns so a failed *read* stops the walk instead of silently ascending into a parent repo, and the walk learns to resolve a worktree's `.git` **file** (a `gitdir:` pointer) to the common dir where `config` actually lives. `DetectRepoURL()` keeps its bare-string signature and swallows at the boundary.

**Tech Stack:** Go 1.25.5, gomega, `t.TempDir()` real-filesystem fixtures, `targ` build system.

## Task order and dependencies

Tasks are strictly sequential — each consumes the previous task's signatures:

| Task | Delivers | Closes | Depends on |
| --- | --- | --- | --- |
| 1 | error-returning chain; a broken read stops the walk | #35 (Observed) | — |
| 2 | unreadable-config (non-`ErrNotExist` open) also stops the walk | #35 (folded sibling) | 1 |
| 3 | worktree `gitdir:` pointer resolution | #37 (Expected) | 1 |
| 4 | detection tests independent of the checkout | #37 (folded "Related") | 1, 3 |
| 5 | docs + whole-suite and real-worktree verification | both | 1-4 |

**Closure mechanics:** every task commit uses `Refs #N`, never `Closes #N`. Both issues are closed by manual comment at Task 5, after all their work has landed. This matches the repo's convention (`git log --grep` confirms `Closes #` appears only on a commit that is the last one an issue needs — `d53605f`, `bca9850` — while `Refs #` marks partial work: `aa21f71`, `bf1dad6`). It matters here because both issues are closed by more than one commit: #35 by Tasks 1-2 and #37 by Tasks 3-4, so an auto-close keyword on any single commit would close the issue before the rest of its approved work exists.

## Global Constraints

- **Blast radius is closed and verified**: the five chain functions have callers only in `internal/core/git.go`, `internal/core/git_test.go`, and `internal/core/command.go:1859,1909` (both call bare `DetectRepoURL()`, whose signature does not change). `internal/core` is internal and `targ.go` re-exports none of it, so `check-thin-api` is unaffected — confirmed by a reviewer running the gate against the applied diff.
- **Five existing test call sites break on Task 1's signature change, not three.** All of these need the two-value form: `ParseGitConfigContentExtractsOriginURL` (`git_test.go:152`), `ParseGitConfigContentHandlesMissingOrigin` (`:166`), `DetectRepoURLWithDepsHandlesGetwdError` (`:197`), `DetectRepoURLFromDirWithOpenHandlesOpenError` (`:210`), `ParseGitConfigOriginURLWithOpenHandlesOpenError` (`:222`). Missing the first two yields `assignment mismatch` compile errors.
- **New tests use real `t.TempDir()` fixtures, not injected fake openers.** CLAUDE.md: *"No IO mocking. Do not mock filesystem, network, or other IO in unit tests."* The existing `failingOpen`/`dummyOpen` closures predate that rule; they get signature and sentinel updates only. Repo precedent for `t.TempDir()`: 23 call sites across 6 files.
- **`git_test.go` is `package core_test` (blackbox).** Every call from a test is `core.Fn(...)` — that prefix is required, not optional.
- **Per-function coverage ≥80%** (`check-coverage-for-fail`). Every new error branch needs a test, and `DetectRepoURL()` must stay covered — see Task 4's wiring assertion.
- **No flaky tests**: no wall-clock, no load dependence, no shared mutable state. Each subtest gets its own `t.TempDir()`.
- **Full-suite green gates sequence AFTER the commit** (vault 395): `check-uncommitted` fails by design on a dirty tree.
- **A stale `golangci-lint` cache produces phantom `lint-full` failures** citing deleted paths (CLAUDE.md). If `lint-full` fails, run `golangci-lint cache clean` and re-run BEFORE treating it as a real finding.
- Commit trailer is `AI-Used: [claude]`.

## Design notes

- **Why the err route fixes a bug rather than silencing a linter (user's decision, recorded).** Joe chose the error-returning chain over a local-handling variant. Every error is swallowed at `DetectRepoURL()`, the only production entry point, so no user ever sees one. The value is that an error gives the walk-up a way to **stop**. Today a failed read returns `""`, `DetectRepoURLFromDirWithOpen` ascends to the parent directory, and a *different repository's* config can be parsed and returned as yours. A bare `string` cannot express "stop, do not ascend". That is the real #35 defect, and only the err route fixes it.
- **Both adjacent items were put to the user and both were folded in (decisions recorded).** Gate A's ask-alignment reviewer correctly caught that an earlier draft gated one adjacent item while silently folding another. Both were escalated:
  - *Test environment-coupling* (#37 "Related", phrased there as "worth deciding") → **fold** (Task 4). Verified beforehand: once Task 3 lands, a worktree of this repo has a `github.com` origin, so the existing assertion would pass untouched and #37 is closable without Task 4 — the fold buys the local-path-clone case, which is what forced #33's acceptance run to use a GitHub-origin clone.
  - *Unreadable-config fallthrough* (named in neither issue) → **fold** (Task 2), against the author's recommendation to defer. Recorded as the user's call; implemented in full.
- **The realistic scan failure is `bufio.ErrTooLong`, not disk failure.** `bufio.Scanner`'s default token cap is 64KB, so a config line longer than that aborts the scan. Empirically confirmed by a Gate A reviewer: a 70KB line makes `Scan()` return false after one line, `Err()` returns `bufio.ErrTooLong`, the error survives a `fmt.Errorf("%w")` wrap, and scanning does **not** resume on later lines.
- **Error semantics — exactly which condition stops the walk:**

  | Condition | Result | Meaning |
  | --- | --- | --- |
  | open fails with `fs.ErrNotExist` | `("", nil)` | no config here — **keep walking** (the walk-up depends on this) |
  | open fails with `syscall.ENOTDIR` | `("", nil)` | `<dir>/.git` is a FILE, so the path traversal failed — **keep walking** so the worktree pointer branch can run (see below) |
  | open fails otherwise (e.g. permission) | `("", err)` | a config exists but is unreadable — **stop** (Task 2) |
  | scan fails mid-file | `("", err)` | this IS the config and the read broke — **stop** (Task 1) |
  | clean EOF, no origin section | `("", nil)` | keep walking |

- **Tasks 2 and 3 are NOT independent — they jointly define the open-error classification, and the interaction is a real defect if missed.** Both modify `ParseGitConfigOriginURLWithOpen`'s error handling, and the conflict exists in the final combined state whichever lands first. In a worktree `<dir>/.git` is a file, so the direct probe `open(<dir>/.git/config)` fails with **`ENOTDIR`, not `ENOENT`** — empirically confirmed: `errors.Is(err, fs.ErrNotExist)` is **false** and `errors.Is(err, syscall.ENOTDIR)` is **true**. Without the `ENOTDIR` branch, Task 2 classifies that as a hard stop, `DetectRepoURLFromDirWithOpen` returns an error immediately, and **Task 3's pointer-resolution branch is never reached** — silently defeating the entire #37 fix. A Gate A reviewer applied all four tasks verbatim in a real worktree and reproduced exactly this: both `DetectsRepoURLFromInsideAGitWorktree` and `DetectRepoURLDelegatesToTheInjectableForm` fail with `not a directory`. Task 2's fixture therefore includes a worktree-shaped case so the interaction is caught within Task 2, before Task 3 exists.

- **Worktree resolution mechanics (verified against a live worktree, twice).** In a linked worktree, `.git` is a file containing `gitdir: <abs path>`; that gitdir holds `HEAD`, `index`, `refs`, `commondir` — **no `config`**. `commondir` contains `../..` (relative), resolving to the main `.git`, where `config` lives. A Gate A reviewer independently created a real worktree and confirmed the fixture in Task 3 reproduces this exact depth relationship, with no daylight between fixture and reality.
- **Why `.git/config` is tried before the pointer file.** Go's `os.Open` on a *directory* succeeds (reads fail with EISDIR), so probing `.git`-as-a-file first would need EISDIR handling for the common case. Trying `<dir>/.git/config` first keeps the normal-repo path untouched and reaches the pointer logic only when it misses.
- **Complexity headroom is measured, not assumed.** A Gate A reviewer measured the post-change functions: `DetectRepoURLFromDirWithOpen` cyclomatic 8 / cognitive 15; `gitDirConfigPath` cyclomatic 7 / cognitive 6 — against configured limits of cyclop 10 and gocognit/gocyclo 30. No extraction is needed; do not pre-emptively split.

## Doc-surface enumeration grep

Ran `worktree|DetectRepoURL|More info|repo ?URL|scanner|gitdir` over `*.md` (`README.md`, `CLAUDE.md`, `docs/`, `specs/`) and `DetectRepoURL|ParseGitConfig|NormalizeGitURL|worktree|\.git/config|gitdir|More info` over `*.go` (including test files and doc comments):

| File | Disposition | Reason |
| --- | --- | --- |
| `CLAUDE.md:40` (worktree caveat added in the #33 cycle) | **update** | States `check-full` cannot pass in a worktree because `DetectRepoURL` reads `.git` as a path. Task 3 makes that false. Task 5 rewrites the clause; the `golangci-lint` cache clause stays (still true). |
| `internal/core/git.go:42,50` (doc comments on the changed functions) | **update** | Rewritten in Tasks 1-3 to state each function's error contract and the worktree resolution — part of the code change, not a separate doc task. |
| `docs/specs/implementation.md:88-89` (git integration purpose; key-functions list) | keep | Names `DetectRepoURL`/`ParseGitConfigContent`; both still exist with unchanged public behavior. Internal signatures are not spec'd at this level. |
| `docs/specs/architecture.md:91` (`DetectRepoURL()` parses `.git/config`) | keep | Still accurate — it does parse `.git/config`; the fix only changes *where it finds it* in a worktree. Considered adding "and worktrees" and rejected: the sentence describes the parse, not the search domain, and specs here stay behavioural-abstract. |
| `docs/specs/tests.md:123,129` (git detection given/when/then; `TestProperty_CleanWorkTree` roster) | keep | Roster names `TestProperty_CleanWorkTree`, not `TestProperty_GitDetection`; "Repo URL detection parses `.git/config`" remains true. |
| `docs/specs/tests.md:163` (repo URL shown when no more-info text) | keep | Help behaviour unchanged. |
| `docs/specs/requirements.md:105`, `README.md:412` | N/A | Both concern `CheckCleanWorkTree()`, a different function in the same file, untouched. |
| `internal/core/types.go:52,53,56` (`RepoURL` / `MoreInfoText` doc comments, "detect it from `.git/config`") | keep | Still true — detection still reads `.git/config`; only its search path widens. |
| `internal/runner/runner.go:3812,3861` (hardcoded `More info: …#readme`) | keep | Fallback text printed when no target files exist; never reaches the detection chain. |
| `internal/help/render.go:288`, `internal/help/generators.go:157,239` (render the "More info" section) | keep | Pure consumers — `renderMoreInfo` prints whatever string it is handed (`render.go:284-292`); unaffected by where that string came from. |
| `test/hierarchy_properties_test.go:395` (asserts output contains `More info:`) | keep | Asserts the section renders, not its content; passes regardless of detection outcome. |
| `specs/001-parallel-output/research.md:35` (Scanner 64KB cap) | keep | Historical research note about PrefixWriter — unrelated, though it independently corroborates the cap this plan relies on. |
| `docs/plans/*` (3 files mentioning worktree/DetectRepoURL) | N/A | Historical planning records, kept as written per repo convention. |

---

### Task 1: #35 — error-returning chain that stops the walk

**Files:**
- Modify: `internal/core/git.go` (all five chain functions)
- Test: `internal/core/git_test.go` (`TestProperty_GitDetection`)

**Interfaces:**
- Consumes: nothing.
- Produces the signatures Tasks 2-4 build on:
  - `ParseGitConfigContent(r io.Reader) (string, error)`
  - `ParseGitConfigOriginURLWithOpen(path string, open FileOpener) (string, error)`
  - `DetectRepoURLFromDirWithOpen(dir string, open FileOpener) (string, error)`
  - `DetectRepoURLWithDeps(getwd func() (string, error), open FileOpener) (string, error)`
  - `DetectRepoURL() string` — **unchanged**
  - `OSOpen() FileOpener` — new exported accessor (see Step 3)

- [ ] **Step 1: Write the failing tests**

Add to `TestProperty_GitDetection` in `internal/core/git_test.go`. (`strings.NewReader` is not IO mocking — `ParseGitConfigContent` is a pure function over an `io.Reader`.)

```go
	t.Run("ParseGitConfigContentReportsScanError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// A line longer than bufio.Scanner's 64KB default token cap aborts the
		// scan with bufio.ErrTooLong — the realistic read failure for a config.
		huge := "[remote \"origin\"]\n\turl = " + strings.Repeat("x", 70*1024) + "\n"

		url, err := core.ParseGitConfigContent(strings.NewReader(huge))
		g.Expect(err).To(HaveOccurred(), "an aborted scan must not look like a clean parse")
		g.Expect(url).To(BeEmpty())
	})

	t.Run("WalkUpStopsOnScanErrorInsteadOfUsingParentRepo", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// parent/ is a repo with a valid origin; parent/child/ is a repo whose
		// config cannot be scanned. Detection from child must NOT report the
		// parent's URL.
		parent := t.TempDir()
		g.Expect(os.MkdirAll(filepath.Join(parent, ".git"), 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(parent, ".git", "config"),
			[]byte("[remote \"origin\"]\n\turl = git@github.com:someone/parent.git\n"), 0o600)).To(Succeed())

		child := filepath.Join(parent, "child")
		g.Expect(os.MkdirAll(filepath.Join(child, ".git"), 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(child, ".git", "config"),
			[]byte("[remote \"origin\"]\n\turl = "+strings.Repeat("x", 70*1024)+"\n"), 0o600)).To(Succeed())

		url, err := core.DetectRepoURLFromDirWithOpen(child, core.OSOpen())
		g.Expect(err).To(HaveOccurred(), "a broken config must stop the walk, not fall through to the parent")
		g.Expect(url).NotTo(ContainSubstring("parent"), "must never report a different repository's URL")
	})
```

- [ ] **Step 2: Run the tests to verify they fail (RED)**

Run: `go test ./internal/core -run 'TestProperty_GitDetection' 2>&1 | tail -20`

Expected, in two stages:
1. **Compile errors first** — `core.OSOpen` undefined, plus `assignment mismatch` at all five existing call sites listed in Global Constraints. Add the `OSOpen` accessor and convert all five sites to the two-value form before expecting a behavioural failure.
2. **Then the real RED**: `WalkUpStopsOnScanErrorInsteadOfUsingParentRepo` fails on its **first** assertion — `Expect(err).To(HaveOccurred())` — because the pre-fix code returns `nil`. Gomega's `NewWithT` calls `Fatalf`, so the subtest aborts there and the "must never report a different repository's URL" line is never reached or printed. That first-assertion failure IS the #35 defect (no error, walk continued). Record its exact text; do not expect to see the parent-URL message.

- [ ] **Step 3: Implement the error chain (GREEN)**

In `internal/core/git.go`. Export the opener accessor as a function (matching `FileOpener`'s function type), and give it a doc comment — `revive` fails an exported symbol without one:

```go
// OSOpen returns the default FileOpener, which reads real files from disk.
func OSOpen() FileOpener {
	return osOpen
}
```

```go
// ParseGitConfigContent extracts the origin remote URL from git config content.
// It returns an error only when the content could not be read to the end; a
// config with no origin section is ("", nil).
func ParseGitConfigContent(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	inOrigin := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Check for [remote "origin"] section
		if strings.HasPrefix(line, "[remote \"origin\"]") {
			inOrigin = true
			continue
		}

		// New section starts
		if strings.HasPrefix(line, "[") {
			inOrigin = false
			continue
		}

		// Look for url = ... in origin section
		if inOrigin && strings.HasPrefix(line, "url") {
			const keyValueParts = 2

			parts := strings.SplitN(line, "=", keyValueParts)
			if len(parts) == keyValueParts {
				return NormalizeGitURL(strings.TrimSpace(parts[1])), nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading git config: %w", err)
	}

	return "", nil
}
```

```go
// ParseGitConfigOriginURLWithOpen reads a git config file using injected opener.
// A file that is simply absent yields ("", nil) — the caller treats that as
// "no config here" and keeps walking. Only a config that exists but cannot be
// read returns an error.
func ParseGitConfigOriginURLWithOpen(path string, open FileOpener) (string, error) {
	f, err := open(path)
	if err != nil {
		return "", nil
	}

	defer func() { _ = f.Close() }()

	return ParseGitConfigContent(f)
}
```

```go
// DetectRepoURLFromDirWithOpen walks up from dir looking for a git config using
// the injected opener. A config that cannot be read stops the walk and returns
// an error, so a broken config never falls through to a parent repository's URL.
func DetectRepoURLFromDirWithOpen(dir string, open FileOpener) (string, error) {
	for {
		url, err := ParseGitConfigOriginURLWithOpen(filepath.Join(dir, ".git", "config"), open)
		if err != nil {
			return "", err
		}

		if url != "" {
			return url, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}

		dir = parent
	}
}
```

```go
// DetectRepoURLWithDeps is a testable version that accepts injected dependencies.
func DetectRepoURLWithDeps(getwd func() (string, error), open FileOpener) (string, error) {
	dir, err := getwd()
	if err != nil {
		return "", fmt.Errorf("resolving working directory: %w", err)
	}

	return DetectRepoURLFromDirWithOpen(dir, open)
}

// DetectRepoURL attempts to find the repository URL by parsing the repo's git
// config, walking up from the current directory. Detection is best-effort — it
// feeds optional help text — so any failure yields an empty string.
func DetectRepoURL() string {
	url, _ := DetectRepoURLWithDeps(os.Getwd, osOpen)

	return url
}
```

Do **not** add a `//nolint:errcheck` on that assignment: `errcheck` does not fire on an explicit blank identifier, and the repo's autofix (`issues.fix = true`) strips the dead directive anyway.

Then convert all five existing call sites to the two-value form. For the two `HandlesOpenError` subtests, assert `err` is **nil** — an absent config is "keep walking", not a failure — and change their fakes to return a not-exist sentinel so they keep meaning that after Task 2:

```go
		failingOpen := func(_ string) (io.ReadCloser, error) {
			return nil, fs.ErrNotExist
		}
```

- [ ] **Step 4: Run the tests to verify they pass (GREEN)**

Run: `go test ./internal/core -run 'TestProperty_GitDetection' -v 2>&1 | grep -E '^(=== RUN|--- |ok|FAIL)'`
Expected: every subtest PASS, including both new ones.

- [ ] **Step 5: Refactor check + Gate B**

Confirm the chain reads as one design: every function's doc comment states its error contract, and the "absent config is not an error" rule is stated once (on `ParseGitConfigOriginURLWithOpen`) rather than repeated. Then dispatch Gate B — a fresh-context design-fit reviewer over the diff, charged with DRY/SRP/YAGNI and "does the result read as written-from-the-start". Done only when Gate B closes.

- [ ] **Step 6: Pre-commit checks, then commit**

Run `targ check` then `targ check-full`. Expected: every leg PASS except `check-uncommitted`. If `lint-full` fails, `golangci-lint cache clean` and re-run before believing it.

```bash
git add internal/core/git.go internal/core/git_test.go
git commit -m "$(cat <<'EOF'
fix(core): stop repo-URL detection falling through to a parent repo

ParseGitConfigContent never checked scanner.Err(), so a config that
could not be read to the end looked identical to a config with no
origin section. DetectRepoURLFromDirWithOpen then walked up to the
parent directory and could parse a DIFFERENT repository's config,
reporting its URL as this repo's.

Give the internal chain error returns so a failed read stops the walk.
An absent config still yields ("", nil) — the walk-up depends on that —
but a config that opens and then fails to scan propagates. DetectRepoURL
keeps its bare-string signature and swallows at the boundary, since it
feeds optional help text.

The realistic trigger is bufio.ErrTooLong: Scanner's default 64KB token
cap aborts on a longer line, which the regression test reproduces with a
real config file.

Refs #35

AI-Used: [claude]
EOF
)"
```

### Task 2: #35 (folded sibling) — an unreadable config also stops the walk

Folded on the user's explicit decision; see Design notes.

**Files:**
- Modify: `internal/core/git.go` (`ParseGitConfigOriginURLWithOpen`)
- Test: `internal/core/git_test.go`

**Interfaces:**
- Consumes: Task 1's signatures.
- Produces: no signature change; the open-error branch is refined.

- [ ] **Step 1: Write the failing test**

```go
	t.Run("WalkUpStopsOnUnreadableConfigInsteadOfUsingParentRepo", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		if os.Geteuid() == 0 {
			t.Skip("root bypasses file permission bits, so an unreadable config cannot be simulated")
		}

		parent := t.TempDir()
		g.Expect(os.MkdirAll(filepath.Join(parent, ".git"), 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(parent, ".git", "config"),
			[]byte("[remote \"origin\"]\n\turl = git@github.com:someone/parent.git\n"), 0o600)).To(Succeed())

		child := filepath.Join(parent, "child")
		g.Expect(os.MkdirAll(filepath.Join(child, ".git"), 0o755)).To(Succeed())
		unreadable := filepath.Join(child, ".git", "config")
		g.Expect(os.WriteFile(unreadable, []byte("[remote \"origin\"]\n\turl = x\n"), 0o600)).To(Succeed())
		g.Expect(os.Chmod(unreadable, 0o000)).To(Succeed())

		url, err := core.DetectRepoURLFromDirWithOpen(child, core.OSOpen())
		g.Expect(err).To(HaveOccurred(), "an unreadable config must stop the walk, not fall through to the parent")
		g.Expect(url).To(BeEmpty())
	})
```

The root skip is deliberate: `t.TempDir()` cleanup and mode bits are honoured for normal users, but root reads regardless of mode, which would make the assertion environment-dependent — exactly the coupling Task 4 removes elsewhere.

Add this second subtest in the same step. It pins the Task 2/3 interaction *within Task 2*, so the regression cannot hide until Task 3 exists:

```go
	t.Run("AGitPointerFileIsNotTreatedAsAnUnreadableConfig", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// A worktree's .git is a FILE, so <dir>/.git/config fails with ENOTDIR,
		// not ENOENT. That must read as "no config here, keep walking" — if it
		// stops the walk, the worktree pointer branch can never run.
		dir := t.TempDir()
		g.Expect(os.WriteFile(filepath.Join(dir, ".git"),
			[]byte("gitdir: /nonexistent/worktrees/wt1\n"), 0o600)).To(Succeed())

		url, err := core.ParseGitConfigOriginURLWithOpen(
			filepath.Join(dir, ".git", "config"), core.OSOpen())
		g.Expect(err).NotTo(HaveOccurred(), "ENOTDIR means no config here, not an unreadable config")
		g.Expect(url).To(BeEmpty())
	})
```

- [ ] **Step 2: Run the test to verify it fails (RED)**

Run: `go test ./internal/core -run 'TestProperty_GitDetection/WalkUpStopsOnUnreadableConfig' -v 2>&1 | tail -15`
Expected: FAIL on `Expect(err).To(HaveOccurred())` — after Task 1, every open failure still yields `("", nil)`, so the walk ascends and returns the parent's URL with no error.

- [ ] **Step 3: Distinguish absent from unreadable (GREEN)**

```go
// ParseGitConfigOriginURLWithOpen reads a git config file using injected opener.
// A file that is simply absent yields ("", nil) — the caller treats that as
// "no config here" and keeps walking. A config that exists but cannot be opened
// or read returns an error, so an unreadable repo never falls through to its
// parent.
func ParseGitConfigOriginURLWithOpen(path string, open FileOpener) (string, error) {
	f, err := open(path)
	if err != nil {
		// Absent, or the path ran through a .git that is a file rather than a
		// directory (a worktree pointer) — either way there is no config here,
		// so the caller keeps walking. Anything else means a config exists and
		// could not be read.
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return "", nil
		}

		return "", fmt.Errorf("opening git config: %w", err)
	}

	defer func() { _ = f.Close() }()

	return ParseGitConfigContent(f)
}
```

`osOpen` already wraps with `fmt.Errorf("opening %s: %w", path, err)` (`git.go:151`), so `errors.Is` matches through the wrap — verified for both sentinels. Add `errors`, `io/fs`, and `syscall` to the imports (`syscall` is permitted by `depguard`'s `$gostd` allowlist).

The `ENOTDIR` branch is not optional and not a Task 3 concern: without it this task breaks the worktree fix. See the Design note on Task 2/3 interaction.

- [ ] **Step 4: Run the tests to verify they pass (GREEN)**

Run: `go test ./internal/core -run 'TestProperty_GitDetection' -v 2>&1 | grep -E '^(=== RUN|--- |ok|FAIL)'`
Expected: all subtests PASS. The two `HandlesOpenError` subtests must still pass — Task 1 changed their fakes to `fs.ErrNotExist`, which now takes the keep-walking branch by name rather than by accident.

- [ ] **Step 5: Refactor check + Gate B**, then commit:

```bash
git add internal/core/git.go internal/core/git_test.go
git commit -m "$(cat <<'EOF'
fix(core): an unreadable git config stops detection instead of ascending

Sibling of the scan-error fallthrough: when .git/config exists but
cannot be opened — permissions, most plausibly — the open error was
indistinguishable from "no config here", so the walk ascended and could
report a parent repository's URL.

Treat only fs.ErrNotExist as "keep walking"; any other open failure
stops the walk. osOpen already wraps with %w, so errors.Is matches
through it.

Refs #35

AI-Used: [claude]
EOF
)"
```

### Task 3: #37 — resolve a worktree's `.git` pointer file

**Files:**
- Modify: `internal/core/git.go` (`DetectRepoURLFromDirWithOpen` + new unexported helper)
- Test: `internal/core/git_test.go`

**Interfaces:**
- Consumes: Task 1's signatures, and Task 2's `ENOTDIR` branch — **without it this task cannot work**, because the direct probe's `ENOTDIR` failure stops the walk before the pointer branch below is reached. Tasks 2 and 3 are not independent; see the Design note.
- Produces: no signature changes.

- [ ] **Step 1: Write the failing test**

```go
	t.Run("DetectsRepoURLFromInsideAGitWorktree", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Real linked-worktree layout: the worktree's .git is a FILE holding a
		// gitdir: pointer; that gitdir has no config of its own, only a
		// commondir pointing at the main .git, where config lives.
		root := t.TempDir()
		mainGit := filepath.Join(root, "main", ".git")
		wtGit := filepath.Join(mainGit, "worktrees", "wt1")
		g.Expect(os.MkdirAll(wtGit, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(mainGit, "config"),
			[]byte("[remote \"origin\"]\n\turl = git@github.com:toejough/targ.git\n"), 0o600)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(wtGit, "commondir"), []byte("../..\n"), 0o600)).To(Succeed())

		wt := filepath.Join(root, "wt1")
		g.Expect(os.MkdirAll(wt, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(wt, ".git"),
			[]byte("gitdir: "+wtGit+"\n"), 0o600)).To(Succeed())

		url, err := core.DetectRepoURLFromDirWithOpen(wt, core.OSOpen())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(url).To(Equal("https://github.com/toejough/targ"),
			"a worktree must resolve its gitdir pointer to the common dir's config")
	})
```

The worktree root sits OUTSIDE `main/` deliberately: nested, the walk-up would find `main/.git/config` on its own and the test would pass without the pointer logic ever running. The expected value is the normalized URL — a Gate A reviewer confirmed `NormalizeGitURL("git@github.com:toejough/targ.git")` → `https://github.com/toejough/targ`.

- [ ] **Step 2: Run the test to verify it fails (RED)**

Run: `go test ./internal/core -run 'TestProperty_GitDetection/DetectsRepoURLFromInsideAGitWorktree' -v 2>&1 | tail -15`
Expected: FAIL — `url` is empty (the walk finds no `<dir>/.git/config` at any level and returns `("", nil)`), so the `Equal` assertion fails. That is the #37 defect reproduced.

- [ ] **Step 3: Implement pointer resolution (GREEN)**

```go
// gitDirConfigPath resolves a worktree's .git pointer file to the config path in
// the repository's common dir. A linked worktree's .git is a file holding
// "gitdir: <path>"; that directory has no config of its own, only a commondir
// file naming the main .git (usually "../.."). Returns "" when dir is not a
// worktree.
func gitDirConfigPath(dir string, open FileOpener) string {
	pointer, err := open(filepath.Join(dir, ".git"))
	if err != nil {
		return ""
	}

	defer func() { _ = pointer.Close() }()

	data, err := io.ReadAll(pointer)
	if err != nil {
		return "" // .git is a directory, not a pointer file
	}

	gitDir, found := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
	if !found {
		return ""
	}

	common, err := open(filepath.Join(gitDir, "commondir"))
	if err != nil {
		return ""
	}

	defer func() { _ = common.Close() }()

	rel, err := io.ReadAll(common)
	if err != nil {
		return ""
	}

	commonDir := strings.TrimSpace(string(rel))
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(gitDir, commonDir)
	}

	return filepath.Join(commonDir, "config")
}
```

In `DetectRepoURLFromDirWithOpen`, after the direct probe yields no URL and no error, try the pointer path before ascending:

```go
		if path := gitDirConfigPath(dir, open); path != "" {
			url, err = ParseGitConfigOriginURLWithOpen(path, open)
			if err != nil {
				return "", err
			}

			if url != "" {
				return url, nil
			}
		}
```

- [ ] **Step 4: Run the tests to verify they pass (GREEN)**

Run: `go test ./internal/core -run 'TestProperty_GitDetection' -v 2>&1 | grep -E '^(=== RUN|--- |ok|FAIL)'`
Expected: all subtests PASS.

- [ ] **Step 5: Real-worktree verification (the fixture is not the claim)**

```bash
git worktree add /tmp/targ-wt-verify HEAD
cd /tmp/targ-wt-verify && go test ./internal/core -run TestProperty_GitDetection -v 2>&1 | tail -5
cd /Users/joe/repos/personal/targ && git worktree remove /tmp/targ-wt-verify
```
Expected: PASS in a real worktree. A fixture that passes while a real worktree fails would mean the fixture is wrong.

- [ ] **Step 6: Refactor check + Gate B**, then commit:

```bash
git add internal/core/git.go internal/core/git_test.go
git commit -m "$(cat <<'EOF'
fix(core): detect the repo URL from inside a git worktree

DetectRepoURLFromDirWithOpen only ever looked for <dir>/.git/config,
which exists when .git is a directory. In a linked worktree .git is a
file holding a "gitdir:" pointer, and that directory has no config of
its own — only a commondir naming the main .git, where config lives. So
detection returned "" in every worktree, dropping the "More info:" line
from targ --help.

Resolve the pointer: read .git as a file, follow gitdir, then follow
commondir (relative or absolute) to the config. The direct
<dir>/.git/config probe still runs first, so the ordinary-repo path is
untouched.

Refs #37

AI-Used: [claude]
EOF
)"
```

### Task 4: detection tests independent of the checkout

Folded on the user's explicit decision; see Design notes.

**Files:**
- Modify: `internal/core/git_test.go` (`DetectRepoURLReturnsRepoFromGitConfig`, `git_test.go:127-136`)

**Interfaces:**
- Consumes: Tasks 1 and 3.
- Produces: nothing consumed later.

- [ ] **Step 1: Replace the environment-coupled assertion**

The current subtest calls the real `core.DetectRepoURL()` and asserts the result contains `github.com`, coupling it to the checkout's remote — so it fails in any clone made from a local path. Replace with a hermetic fixture plus a wiring assertion that keeps `DetectRepoURL()` covered:

```go
	t.Run("DetectRepoURLReturnsRepoFromGitConfig", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		repo := t.TempDir()
		g.Expect(os.MkdirAll(filepath.Join(repo, ".git"), 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(repo, ".git", "config"),
			[]byte("[remote \"origin\"]\n\turl = git@github.com:toejough/targ.git\n"), 0o600)).To(Succeed())

		url, err := core.DetectRepoURLFromDirWithOpen(repo, core.OSOpen())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(url).To(Equal("https://github.com/toejough/targ"))
	})

	t.Run("DetectRepoURLDelegatesToTheInjectableForm", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Pins the wiring of the zero-argument entry point without asserting
		// anything about the checkout it happens to run in.
		want, err := core.DetectRepoURLWithDeps(os.Getwd, core.OSOpen())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(core.DetectRepoURL()).To(Equal(want))
	})
```

- [ ] **Step 2: Verify the change and the coverage it protects**

```bash
go test ./internal/core -run 'TestProperty_GitDetection' -v 2>&1 | grep -E '^(--- |ok|FAIL)'
go test ./internal/core -coverprofile=/tmp/cov.out >/dev/null 2>&1 && go tool cover -func=/tmp/cov.out | grep -E 'DetectRepoURL|ParseGitConfig|gitDirConfigPath|OSOpen'
```
Expected: all subtests PASS, and every changed function ≥80% — `DetectRepoURL` in particular must not be 0%.

- [ ] **Step 3: Prove the environment coupling is gone**

```bash
SC=/private/tmp/claude-501/-Users-joe-repos-personal-targ/1ad2fbb5-120e-4fca-9525-fd94fcf75014/scratchpad/i3537
rm -rf "$SC" && mkdir -p "$SC" && git clone -q . "$SC/local-clone"
cd "$SC/local-clone" && go test ./internal/core -run TestProperty_GitDetection 2>&1 | tail -3
cd /Users/joe/repos/personal/targ && rm -rf "$SC"
```
Expected: PASS in a clone whose origin is a local filesystem path — the exact configuration that fails today.

- [ ] **Step 4: Commit**

```bash
git add internal/core/git_test.go
git commit -m "$(cat <<'EOF'
test(core): make repo-URL detection tests independent of the checkout

DetectRepoURLReturnsRepoFromGitConfig called the real DetectRepoURL and
asserted the result contained "github.com", so it passed only in a
checkout whose origin is the GitHub remote — it failed in any clone made
from a local filesystem path, taking the internal/core package and the
coverage leg down with it.

Assert against a t.TempDir() fixture repo instead, and keep the
zero-argument entry point covered by pinning that it delegates to
DetectRepoURLWithDeps rather than by asserting anything about the
surrounding checkout.

Refs #37

AI-Used: [claude]
EOF
)"
```

### Task 5: docs + full verification

**Files:**
- Modify: `CLAUDE.md:40`

**Interfaces:**
- Consumes: Tasks 1-4 (the caveat is only false once Task 3 lands).

- [ ] **Step 1: Update the CLAUDE.md caveat**

Replace the now-false worktree clause, keeping the surrounding sentence and the still-true `golangci-lint` clause:

```markdown
- **Always run `check-full` before declaring done.** Use `targ check-full`. This reports ALL failures at once (lint, coverage, ordering, dead code, nil checks). Do NOT use `check-for-fail` (stops at first error, causes whack-a-mole). Do NOT use bare `go test` as final validation — it misses lint, coverage thresholds, and declaration ordering. A stale `golangci-lint` result cache can produce phantom `lint-full` failures citing paths that no longer exist — `golangci-lint cache clean` clears them.
```

- [ ] **Step 2: Full suite on the clean tree**

Run `targ check-full`. Expected: ALL 8 legs PASS.

- [ ] **Step 3: Prove the worktree gate is fixed — the payoff claim**

```bash
golangci-lint cache clean
git worktree add /tmp/targ-wt-gate HEAD
cd /tmp/targ-wt-gate && targ check-full 2>&1 | tail -12
cd /Users/joe/repos/personal/targ && git worktree remove /tmp/targ-wt-gate
```
Expected: **8/8 in a worktree** — the condition #37 says is impossible today. Quote the leg summary; this is the evidence for the close comments.

- [ ] **Step 4: Commit the doc change**

```bash
git add CLAUDE.md
git commit -m "$(cat <<'EOF'
docs: check-full now passes in a git worktree

The worktree caveat added during the #33 cycle is obsolete: #37 taught
DetectRepoURL to resolve a worktree's gitdir pointer, so the coverage
leg no longer fails there. The golangci-lint cache caveat stays.

AI-Used: [claude]
EOF
)"
```

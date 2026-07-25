# Issues #35 + #37: Git-Detection Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop `DetectRepoURL` from reporting the wrong repository's URL when a config read fails (#35), and make it work in a `git worktree` (#37).

**Architecture:** `internal/core/git.go` holds a five-deep chain that all returns bare `string`: `DetectRepoURL` → `DetectRepoURLWithDeps` → `DetectRepoURLFromDirWithOpen` (walks up looking for `<dir>/.git/config`) → `ParseGitConfigOriginURLWithOpen` → `ParseGitConfigContent`. Two changes: the internal chain gains `error` returns so a failed *read* can stop the walk instead of silently ascending into a parent repo, and the walk learns to resolve a worktree's `.git` **file** (a `gitdir:` pointer) to the common dir where `config` actually lives. `DetectRepoURL()` keeps its bare-string signature and swallows at the boundary.

**Tech Stack:** Go 1.25.5, gomega, `t.TempDir()` real-filesystem fixtures, `targ` build system.

## Global Constraints

- **Blast radius is closed and verified**: `DetectRepoURL` and its four helpers have callers only in `internal/core/git.go`, `internal/core/git_test.go`, and `internal/core/command.go:1859,1909` (enumerated with `grep -rn '\bFn(' --include='*.go' .` per function). `internal/core` is internal; `targ.go` re-exports none of these, so `check-thin-api` is unaffected.
- **New tests use real `t.TempDir()` fixtures, not injected fake openers.** CLAUDE.md: *"No IO mocking. Do not mock filesystem, network, or other IO in unit tests."* The existing `failingOpen`/`dummyOpen` closures in `git_test.go` predate that rule; they get signature updates only (rewriting them is out of scope). Repo precedent for `t.TempDir()`: 23 call sites across 6 files.
- **Per-function coverage ≥80%** (`check-coverage-for-fail`). Every new error branch needs a test, and `DetectRepoURL()` itself must stay covered — see Task 3's wiring assertion.
- **No flaky tests**: no wall-clock, no load dependence, no shared mutable state across parallel subtests. Each subtest gets its own `t.TempDir()`.
- **Full-suite green gates sequence AFTER the commit** (vault 395): `check-uncommitted` fails by design on a dirty tree.
- **A stale `golangci-lint` cache produces phantom `lint-full` failures** citing deleted paths (CLAUDE.md). If `lint-full` fails, run `golangci-lint cache clean` and re-run BEFORE treating it as a real finding.
- Commit trailer is `AI-Used: [claude]`.

## Design notes

- **Why the err route fixes a bug rather than silencing a linter (user's decision, recorded).** Joe chose the error-returning chain over a local-handling variant. Worth stating precisely: every error is swallowed at `DetectRepoURL()`, the only production entry point, so no user ever sees one. The value is that an error gives the walk-up a way to **stop**. Today a mid-scan failure returns `""`, `DetectRepoURLFromDirWithOpen` ascends to the parent directory, and a *different repository's* config can be parsed and returned as yours. A bare `string` cannot express "stop, do not ascend". That is the real #35 defect, and only the err route fixes it.
- **The realistic scan failure is `bufio.ErrTooLong`, not disk failure.** `bufio.Scanner`'s default token cap is 64KB, so a config line longer than that aborts the scan. This is the trigger the RED test uses, because it is reproducible with a real file and is the case a user could actually hit.
- **Error semantics — exactly which condition stops the walk:**
  - open fails → `("", nil)`. Means "no readable config here, try the parent." The walk-up **depends** on this; it is not an error.
  - scan fails → `("", err)`. Means "this IS a config and the read broke." Stops the walk.
  - clean EOF, no origin → `("", nil)`. Keep walking.
- **NOT folded in, proposed as a gated follow-up:** the same wrong-repo fallthrough occurs when `.git/config` exists but is **unreadable** (permission denied) — open fails, the walk ascends, wrong URL. Fixing it means distinguishing `fs.ErrNotExist` (keep walking) from other open errors (stop). It is a genuine sibling of #35 but appears in neither issue. Per the propose-don't-fold rule (vault 393/345), it is recorded here as an explicitly optional item awaiting Joe's yes/no — **not** implemented by this plan.
- **Worktree resolution mechanics (verified against a live worktree).** In a linked worktree, `.git` is a file containing `gitdir: <abs path>`; that gitdir holds `HEAD`, `index`, `refs`, `commondir` — **no `config`**. `commondir` contains `../..` (relative), resolving to the main `.git`, where `config` lives. Verified: `cat .git` → `gitdir: /Users/joe/repos/personal/targ/.git/worktrees/pre-rebuild`; `cat <gitdir>/commondir` → `../..`; the joined path's `config` exists.
- **Why `.git/config` is tried before the pointer file.** Go's `os.Open` on a *directory* succeeds (reads fail with EISDIR), so probing `.git`-as-a-file first would need EISDIR handling for the common case. Trying `<dir>/.git/config` first keeps the normal-repo path untouched and reaches the pointer logic only when it misses.

## Doc-surface enumeration grep

Ran over the repo for `worktree|DetectRepoURL|More info|repo ?URL|scanner` (`*.md` in `README.md`, `CLAUDE.md`, `docs/specs/`, `specs/`) plus `worktree|\.git/config` in non-test `*.go`:

| File | Disposition | Reason |
| --- | --- | --- |
| `CLAUDE.md:40` (worktree caveat added in the #33 cycle) | **update** | It states `check-full` cannot pass in a worktree because `DetectRepoURL` reads `.git` as a path. This fix makes that false. Task 4 rewrites the clause; the `golangci-lint` cache half of the sentence stays (still true). |
| `docs/specs/implementation.md:88-89` (IMPL git integration; key functions list) | keep | Describes purpose and names `DetectRepoURL`/`ParseGitConfigContent`. Both still exist with the same public behavior; internal signatures are not spec'd at this level. |
| `docs/specs/architecture.md:91` (`DetectRepoURL()` parses `.git/config` for origin) | keep | Still accurate — it does parse `.git/config`; the fix only adds *where it finds it* in a worktree. Wording is not contradicted. |
| `docs/specs/tests.md:123,129` (T-? git detection given/when/then; `TestProperty_CleanWorkTree` roster) | keep | The roster names `TestProperty_CleanWorkTree`, not `TestProperty_GitDetection`; the given/when/then ("Repo URL detection parses `.git/config`") remains true. |
| `docs/specs/tests.md:163` (repo URL shown when no more-info text) | keep | Help behavior unchanged. |
| `docs/specs/requirements.md:105`, `README.md:412` | N/A | Both are about `CheckCleanWorkTree()`, a different function in the same file, untouched. |
| `specs/001-parallel-output/research.md:35` (Scanner 64KB cap) | keep | A historical research note about PrefixWriter, unrelated to git config parsing — though it independently corroborates the 64KB cap this plan relies on. |
| `internal/core/types.go:53` (`If empty, targ attempts to detect it from .git/config.`) | keep | Doc comment on `RunOptions.RepoURL`; still true. |
| `internal/core/git.go:42,50` (doc comments on the changed functions) | **update** | Task 1/2 rewrite these to describe the error returns and worktree resolution — part of the code change, not a separate doc task. |
| `docs/plans/*` (3 files mentioning worktree/DetectRepoURL) | N/A | Historical planning records, kept as written per repo convention. |

---

### Task 1: #35 — error-returning chain that stops the walk

**Files:**
- Modify: `internal/core/git.go` (`ParseGitConfigContent`, `ParseGitConfigOriginURLWithOpen`, `DetectRepoURLFromDirWithOpen`, `DetectRepoURLWithDeps`, `DetectRepoURL`)
- Test: `internal/core/git_test.go` (`TestProperty_GitDetection`)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces the signatures Task 2 builds on:
  - `ParseGitConfigContent(r io.Reader) (string, error)`
  - `ParseGitConfigOriginURLWithOpen(path string, open FileOpener) (string, error)`
  - `DetectRepoURLFromDirWithOpen(dir string, open FileOpener) (string, error)`
  - `DetectRepoURLWithDeps(getwd func() (string, error), open FileOpener) (string, error)`
  - `DetectRepoURL() string` — **unchanged signature**

- [ ] **Step 1: Write the failing tests**

Add to `TestProperty_GitDetection` in `internal/core/git_test.go`. Note `strings.NewReader` is not IO mocking — `ParseGitConfigContent` is a pure function over an `io.Reader`.

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

Implementer note: `osOpen` is currently unexported (`git.go:150`). The blackbox test package needs it — export a thin accessor `OSOpen() FileOpener` returning `osOpen`, or export `osOpen` as `OSOpen`. Pick one, keep it a one-liner, and use it consistently in both tasks' tests.

- [ ] **Step 2: Run the tests to verify they fail (RED)**

Run: `go test ./internal/core -run 'TestProperty_GitDetection' 2>&1 | tail -20`
Expected: compilation failure first (`ParseGitConfigContent` returns 1 value, `core.OSOpen` undefined). After adding only the accessor and the two-value signature stubs, expected: `WalkUpStopsOnScanErrorInsteadOfUsingParentRepo` FAILS by reporting the parent's URL — that is the #35 defect reproduced. Record the exact failure text.

- [ ] **Step 3: Implement the error chain (GREEN)**

In `internal/core/git.go`:

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

`ParseGitConfigOriginURLWithOpen` — an open failure means "no readable config here", which the walk-up relies on, so it is reported as `("", nil)`; only a scan failure propagates:

```go
// ParseGitConfigOriginURLWithOpen reads a git config file using injected opener.
// A file that cannot be opened yields ("", nil) — the caller treats that as
// "no config here". Only a config that opens but cannot be read returns an error.
func ParseGitConfigOriginURLWithOpen(path string, open FileOpener) (string, error) {
	f, err := open(path)
	if err != nil {
		return "", nil
	}

	defer func() { _ = f.Close() }()

	return ParseGitConfigContent(f)
}
```

`DetectRepoURLFromDirWithOpen` — propagate the scan error instead of ascending:

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

`DetectRepoURLWithDeps` and the public entry point:

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
// config. It walks up from the current directory. Detection is best-effort — it
// feeds optional help text — so failures yield an empty string.
func DetectRepoURL() string {
	url, _ := DetectRepoURLWithDeps(os.Getwd, osOpen) //nolint:errcheck // best-effort: help text degrades to no link

	return url
}
```

Update the three existing error-path subtests to the two-value form (`url, err := ...`; assert `err` per the semantics above — `HandlesOpenError` cases expect `err` to be nil, since an unopenable config is "keep walking", not a failure).

- [ ] **Step 4: Run the tests to verify they pass (GREEN)**

Run: `go test ./internal/core -run 'TestProperty_GitDetection' -v 2>&1 | grep -E '^(--- |ok|FAIL)'`
Expected: all subtests PASS, including both new ones.

- [ ] **Step 5: Refactor check + Gate B**

Confirm the result reads as one coherent chain: every function's doc comment states its error contract, and the "open failure is not an error" rule is stated exactly once (on `ParseGitConfigOriginURLWithOpen`) rather than repeated. Then dispatch Gate B: a fresh-context design-fit reviewer over the diff, charged with DRY/SRP/YAGNI and "does the result read as written-from-the-start". The unit is done only when Gate B closes.

- [ ] **Step 6: Pre-commit checks, then commit**

Run `targ check` then `targ check-full`. Expected: every leg PASS except `check-uncommitted`. If `lint-full` fails, run `golangci-lint cache clean` and re-run before believing it.

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
An unopenable config still yields ("", nil) — the walk-up depends on
that — but a config that opens and then fails to scan propagates.
DetectRepoURL keeps its bare-string signature and swallows at the
boundary, since it feeds optional help text.

The realistic trigger is bufio.ErrTooLong: Scanner's default 64KB token
cap aborts on a longer line, which the regression test reproduces with
a real config file.

Closes #35

AI-Used: [claude]
EOF
)"
```

### Task 2: #37 — resolve a worktree's `.git` pointer file

**Files:**
- Modify: `internal/core/git.go` (`DetectRepoURLFromDirWithOpen` + a new unexported helper)
- Test: `internal/core/git_test.go`

**Interfaces:**
- Consumes: Task 1's `(string, error)` signatures.
- Produces: no signature changes; `DetectRepoURLFromDirWithOpen` gains worktree support internally.

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

Note the expected value is the *normalized* URL — `NormalizeGitURL` converts `git@github.com:toejough/targ.git` to `https://github.com/toejough/targ`. Confirm against `NormalizeGitURL`'s existing tests (`git_test.go:174,182`) before asserting.

The worktree root is placed OUTSIDE `main/` deliberately: if it were nested inside, the walk-up would find `main/.git/config` on its own and the test would pass without the pointer logic ever running.

- [ ] **Step 2: Run the test to verify it fails (RED)**

Run: `go test ./internal/core -run 'TestProperty_GitDetection/DetectsRepoURLFromInsideAGitWorktree' -v 2>&1 | tail -15`
Expected: FAIL — `url` is empty, because the walk finds no `<dir>/.git/config` at any level and returns `("", nil)`. That is the #37 defect reproduced.

- [ ] **Step 3: Implement pointer resolution (GREEN)**

Add an unexported helper and call it from the walk, after the direct-config attempt misses:

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

Then in `DetectRepoURLFromDirWithOpen`, after the direct attempt returns no URL and no error, try the pointer path before ascending:

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

Watch the cyclomatic/cognitive complexity limits on `DetectRepoURLFromDirWithOpen` (CLAUDE.md: anticipate, don't wait for lint). If the loop body trips a limit, extract the per-directory probe into an unexported `configURLForDir(dir string, open FileOpener) (string, error)` helper and keep the loop to walk-and-ascend.

- [ ] **Step 4: Run the tests to verify they pass (GREEN)**

Run: `go test ./internal/core -run 'TestProperty_GitDetection' -v 2>&1 | grep -E '^(--- |ok|FAIL)'`
Expected: all subtests PASS.

- [ ] **Step 5: Real-worktree verification (not just the fixture)**

```bash
git worktree add /tmp/targ-wt-verify HEAD
cd /tmp/targ-wt-verify && go test ./internal/core -run TestProperty_GitDetection -v 2>&1 | tail -5
cd /Users/joe/repos/personal/targ && git worktree remove /tmp/targ-wt-verify
```
Expected: PASS in the worktree. This is the positive check that the fixture matches reality — a fixture that passes while a real worktree fails would mean the fixture is wrong.

- [ ] **Step 6: Refactor check + Gate B**

Confirm the pointer helper reads as part of the file (naming, doc-comment density, error handling style consistent with its neighbours), then dispatch Gate B design-fit over the diff. Done only when Gate B closes.

- [ ] **Step 7: Commit**

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

Closes #37

AI-Used: [claude]
EOF
)"
```

### Task 3: replace the environment-coupled detection test

**Files:**
- Modify: `internal/core/git_test.go` (`DetectRepoURLReturnsRepoFromGitConfig`, `git_test.go:127-136`)

**Interfaces:**
- Consumes: Tasks 1 and 2.
- Produces: nothing consumed later.

- [ ] **Step 1: Replace the assertion**

The current subtest calls the real `core.DetectRepoURL()` and asserts the result contains `github.com` — coupling it to the checkout's remote, so it fails in any clone made from a local path (and, before Task 2, in any worktree). Replace it with a hermetic fixture test plus a wiring assertion that keeps `DetectRepoURL()` covered:

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

- [ ] **Step 2: Verify both the test change and the coverage it protects**

```bash
go test ./internal/core -run 'TestProperty_GitDetection' -v 2>&1 | grep -E '^(--- |ok|FAIL)'
go test ./internal/core -coverprofile=/tmp/cov.out >/dev/null 2>&1 && go tool cover -func=/tmp/cov.out | grep -E 'DetectRepoURL|ParseGitConfig|gitDirConfigPath'
```
Expected: all subtests PASS, and every changed function reports ≥80% coverage — `DetectRepoURL` in particular must not be 0%.

- [ ] **Step 3: Prove the environment coupling is gone**

```bash
SC=/private/tmp/claude-501/-Users-joe-repos-personal-targ/1ad2fbb5-120e-4fca-9525-fd94fcf75014/scratchpad/i3537
rm -rf "$SC" && mkdir -p "$SC" && git clone -q . "$SC/local-clone"
cd "$SC/local-clone" && go test ./internal/core -run TestProperty_GitDetection 2>&1 | tail -3
```
Expected: PASS in a clone whose origin is a local filesystem path — the exact configuration that fails today. Then `cd` back and `rm -rf "$SC"`.

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

AI-Used: [claude]
EOF
)"
```

### Task 4: docs + full verification

**Files:**
- Modify: `CLAUDE.md:40` (the worktree caveat)

- [ ] **Step 1: Update the CLAUDE.md caveat**

The clause "in a `git worktree` the coverage leg fails regardless of your change, because `DetectRepoURL` reads `.git/config` as a path and a worktree's `.git` is a file (filed separately)" is false once Tasks 2 and 3 land. Replace that clause — keep the surrounding sentence and the `golangci-lint` cache clause, which remain true:

```markdown
- **Always run `check-full` before declaring done.** Use `targ check-full`. This reports ALL failures at once (lint, coverage, ordering, dead code, nil checks). Do NOT use `check-for-fail` (stops at first error, causes whack-a-mole). Do NOT use bare `go test` as final validation — it misses lint, coverage thresholds, and declaration ordering. A stale `golangci-lint` result cache can produce phantom `lint-full` failures citing paths that no longer exist — `golangci-lint cache clean` clears them.
```

- [ ] **Step 2: Full suite on the clean tree**

Run `targ check-full`. Expected: ALL 8 legs PASS.

- [ ] **Step 3: Prove the worktree gate is actually fixed — the payoff claim**

```bash
git worktree add /tmp/targ-wt-gate HEAD
cd /tmp/targ-wt-gate && targ check-full 2>&1 | tail -12
cd /Users/joe/repos/personal/targ && git worktree remove /tmp/targ-wt-gate
```
Expected: **8/8 in a worktree** — the condition #37 says is impossible today. Quote the leg summary; this is the evidence for both close comments. Run `golangci-lint cache clean` first so a stale cache cannot confound it.

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

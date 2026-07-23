# Replace-Directive Cache Key (targ#27, proposal b) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the bootstrap binary cache key include the contents of local filesystem `replace` target directories from the consumer's `go.mod`, so editing a replaced dependency invalidates the cached gate binary (fixes toejough/targ#27, fix direction b).

**Architecture:** `computeModuleCacheKey` (`internal/runner/runner.go:2662`) is the single assembly point for cache inputs; today it hashes the consumer's tagged files plus `collectModuleFiles(importRoot)` (non-test `.go` + `go.mod` + `go.sum` under the consumer module root only). The fix adds one input source: parse the consumer's `go.mod` for filesystem replace directives (`Replace.New.Version == ""` per the go.mod spec means a directory replacement), resolve relative paths against the module root, and run the same `collectModuleFiles` walk over each target directory. Binary reuse is keyed solely on this cache key (`computeModuleCacheKey` → `setupBinaryPath` → `tryCachedBinary`), so key-level tests capture the complete stale-cache mechanism — no subprocess builds needed in tests.

**Tech Stack:** Go 1.25.5; `golang.org/x/mod/modfile` (already in module graph as indirect; becomes direct); gomega assertions; rapid property tests.

## Global Constraints

- Use `targ` commands for all verification: `targ test` while iterating, `targ check-full` before every commit (NOT bare `go test`, NOT `check-for-fail`).
- TDD red step is mandatory: run each new test and confirm the expected failure before implementing.
- Blackbox tests: `package runner_test`, dot-import gomega, `NewGomegaWithT(t)`, `t.Parallel()` on every test and subtest; no shared mutable state between parallel subtests.
- No IO mocking; tests use real files under `t.TempDir()`.
- No wall-clock/timing assertions; no subprocess spawning in these tests (cache-key computation is pure file hashing — keeps package timeout headroom).
- Per-function coverage ≥80%: every new function needs its branches exercised.
- Declaration ordering: funcs alphabetical within groups; run `targ reorder-decls` after adding declarations rather than hand-placing.
- No new package-level mutable state.
- Commit trailer: `AI-Used: [claude]` (never Co-Authored-By).
- Commit only at the points this plan marks — `targ deadcode` (part of check-full) fails on an unwired helper, so Task 1 does NOT commit; the first commit lands after Task 2 wires the helper.

## Doc-surface disposition (enumeration grep result)

Grep run over `README.md`, `CLAUDE.md`, `docs/`, `examples/` for `no-binary-cache`, `no-cache`, `binary cache`, `cache key`, `cache is invalidated`, `invalidate`, plus a `docs/specs/` sweep for `cache`:

| File:line | Disposition | Reason |
| --- | --- | --- |
| `README.md:687` (flag table row for `--no-cache`) | **update** | `--no-cache` is a deprecated alias (runner.go:488); table should document `--no-binary-cache` |
| `README.md:782` ("invalidated when source files or `go.mod`/`go.sum` change") | **update** | The invariant this change extends: add local filesystem replace targets |
| `README.md:785` (`targ --no-cache <command>`) | **update** | Same deprecated alias; use `--no-binary-cache` |
| `docs/archive/research-gomod-tree-traversal.md` | **N/A** | Archived historical snapshot (records Problem 5/9a277d4 as-of writing); archives are not updated |
| `docs/archive/architecture.md`, `docs/archive/design.md` | **N/A** | Archived |
| `docs/specs/implementation.md:121` (flag registry naming `--no-cache`) | **keep** | Registry of flags that exist; `--no-cache` is still parsed (deprecated) — no invariant change |
| `docs/specs/requirements.md:91` ("compiles it (cached in `~/.cache/targ/`)") | **keep** | States caching exists; does not state invalidation inputs |
| `docs/specs/architecture.md:63`, `docs/specs/implementation.md:112` (checksum/cache invalidation) | **keep** | Different subsystem: target-level `.Cache()` result caching, not the bootstrap binary cache |
| `CLAUDE.md` | **keep** | No cache-invariant statements |

---

### Task 1: `filesystemReplaceDirs` + `collectReplaceDirFiles` helpers

**Files:**
- Modify: `internal/runner/runner.go` (two new unexported funcs; new import `golang.org/x/mod/modfile`)
- Modify: `internal/runner/export_test.go` (test-only export wrappers)
- Create: `internal/runner/cache_key_test.go`
- Modify: `go.mod`/`go.sum` (`golang.org/x/mod` becomes direct — via `targ tidy`)

**Interfaces:**
- Consumes: `collectModuleFiles(moduleRoot string) ([]discover.TaggedFile, error)` (runner.go:2515), `goModFile` constant, `discover.TaggedFile{Path string, Content []byte}`.
- Produces: `collectReplaceDirFiles(moduleRoot string) ([]discover.TaggedFile, error)` — Task 2 wires this into `computeModuleCacheKey`. Test-only: `runner.ExportCollectReplaceDirFiles` (same signature).

- [ ] **Step 1: Write the failing tests**

Create `internal/runner/cache_key_test.go`:

```go
package runner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/runner"
)

// writeTestFile writes content to dir/name, creating it if needed.
func writeTestFile(g *GomegaWithT, dir, name, content string) {
	g.THelper()
	g.Expect(os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)).To(Succeed())
}

func TestReplaceDirFiles_CollectsFilesystemReplaceTargets(t *testing.T) {
	t.Parallel()

	t.Run("AbsoluteReplaceTargetFilesIncluded", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		dep := t.TempDir()
		writeTestFile(g, dep, "go.mod", "module example.com/dep\n\ngo 1.25\n")
		writeTestFile(g, dep, "dep.go", "package dep\n")
		writeTestFile(g, dep, "dep_test.go", "package dep\n")

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+dep+"\n")

		files, err := runner.ExportCollectReplaceDirFiles(consumer)
		g.Expect(err).NotTo(HaveOccurred())

		paths := make([]string, 0, len(files))
		for _, f := range files {
			paths = append(paths, f.Path)
		}
		g.Expect(paths).To(ContainElement(filepath.Join(dep, "dep.go")))
		g.Expect(paths).To(ContainElement(filepath.Join(dep, "go.mod")))
		g.Expect(paths).NotTo(ContainElement(filepath.Join(dep, "dep_test.go")),
			"same walk as collectModuleFiles: test files excluded")
	})

	t.Run("RelativeReplaceTargetResolvedAgainstModuleRoot", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		root := t.TempDir()
		consumer := filepath.Join(root, "consumer")
		dep := filepath.Join(root, "dep")
		g.Expect(os.MkdirAll(consumer, 0o750)).To(Succeed())
		g.Expect(os.MkdirAll(dep, 0o750)).To(Succeed())
		writeTestFile(g, dep, "dep.go", "package dep\n")
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => ../dep\n")

		files, err := runner.ExportCollectReplaceDirFiles(consumer)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(files).To(HaveLen(1))
		g.Expect(files[0].Path).To(Equal(filepath.Join(dep, "dep.go")))
	})

	t.Run("ModuleVersionReplaceIgnored", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\n"+
				"require example.com/dep v1.0.0\n\n"+
				"replace example.com/dep => example.com/fork v1.0.1\n")

		files, err := runner.ExportCollectReplaceDirFiles(consumer)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(files).To(BeEmpty(), "module-version replaces are fingerprinted by go.sum already")
	})

	t.Run("MissingGoModYieldsNoFiles", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		files, err := runner.ExportCollectReplaceDirFiles(t.TempDir())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(files).To(BeEmpty())
	})

	t.Run("MissingReplaceTargetYieldsMarkerNotError", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		consumer := t.TempDir()
		gone := filepath.Join(consumer, "does-not-exist")
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+gone+"\n")

		files, err := runner.ExportCollectReplaceDirFiles(consumer)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(files).To(HaveLen(1))
		g.Expect(files[0].Path).To(HavePrefix("replace-missing:"),
			"existence flips must still change the cache key")
	})

	t.Run("UnparseableGoModErrors", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod", "module \x00 not a go.mod")

		_, err := runner.ExportCollectReplaceDirFiles(consumer)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("go.mod"))
	})

	_ = strings.TrimSpace // placeholder guard removed in Step 3 if unused
}
```

Note for the implementer: `g.THelper()` does not exist on `GomegaWithT` in some gomega versions — if it fails to compile, take `t *testing.T` as the first parameter of `writeTestFile` and call `t.Helper()` instead, passing `g` alongside. Match whichever pattern compiles; do not add a dependency. Remove the `_ = strings.TrimSpace` line and the `strings` import if unused after Step 3.

Add to `internal/runner/export_test.go` (alphabetical among the Export funcs; imports gain `github.com/toejough/targ/internal/discover`):

```go
// ExportCollectReplaceDirFiles wraps collectReplaceDirFiles for testing.
func ExportCollectReplaceDirFiles(moduleRoot string) ([]discover.TaggedFile, error) {
	return collectReplaceDirFiles(moduleRoot)
}
```

- [ ] **Step 2: Run tests to verify they fail (RED)**

Run: `targ test`
Expected: compile FAILURE — `undefined: collectReplaceDirFiles` (referenced by the new export wrapper). This is the red step; confirm the error names the missing function before continuing.

- [ ] **Step 3: Write the minimal implementation**

In `internal/runner/runner.go`, add two funcs (placement handled by `targ reorder-decls` in Step 5) and the import `"golang.org/x/mod/modfile"`:

```go
// collectReplaceDirFiles collects cache-key inputs from the filesystem
// replace-directive targets in the module's go.mod. Local replace targets are
// not fingerprinted by go.sum, so their sources must be hashed directly or the
// cached binary goes stale when they change (issue #27).
func collectReplaceDirFiles(moduleRoot string) ([]discover.TaggedFile, error) {
	dirs, err := filesystemReplaceDirs(moduleRoot)
	if err != nil {
		return nil, err
	}

	var files []discover.TaggedFile

	for _, dir := range dirs {
		if _, statErr := os.Stat(dir); statErr != nil {
			// Hash the absence so the key still changes when the dir appears.
			files = append(files, discover.TaggedFile{Path: "replace-missing:" + dir})
			continue
		}

		dirFiles, err := collectModuleFiles(dir)
		if err != nil {
			return nil, fmt.Errorf("collecting replace target files: %w", err)
		}

		files = append(files, dirFiles...)
	}

	return files, nil
}

// filesystemReplaceDirs returns the target directories of filesystem replace
// directives in the module's go.mod, resolved against moduleRoot. Per the
// go.mod spec, a replacement with no version is a directory path.
func filesystemReplaceDirs(moduleRoot string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(moduleRoot, goModFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading go.mod: %w", err)
	}

	parsed, err := modfile.ParseLax(goModFile, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing go.mod: %w", err)
	}

	var dirs []string

	for _, rep := range parsed.Replace {
		if rep.New.Version != "" {
			continue
		}

		dir := rep.New.Path
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(moduleRoot, dir)
		}

		dirs = append(dirs, dir)
	}

	return dirs, nil
}
```

If `goModFile`/`goSumFile` constants differ from expectations, read their definitions near `isIncludableModuleFile` (runner.go:3545) and reuse them verbatim. Then run `targ tidy` to promote `golang.org/x/mod` to a direct dependency.

- [ ] **Step 4: Run tests to verify they pass (GREEN)**

Run: `targ test`
Expected: PASS, including all six new subtests.

- [ ] **Step 5: Refactor + ordering**

Run: `targ reorder-decls` then `targ test` again (expected: PASS). Review the two new funcs for DRY/SRP/YAGNI — they should read as if written with `collectModuleFiles` originally. **Gate B fires here** (design-fit reviewer on the diff). Do NOT commit yet — the helper is unwired and `targ deadcode` would fail; the commit lands at the end of Task 2.

### Task 2: Wire replace-target files into `computeModuleCacheKey`

**Files:**
- Modify: `internal/runner/runner.go:2661-2688` (`computeModuleCacheKey`)
- Modify: `internal/runner/export_test.go` (one more wrapper)
- Modify: `internal/runner/cache_key_test.go` (add wiring tests)

**Interfaces:**
- Consumes: `collectReplaceDirFiles` from Task 1; `computeModuleCacheKey(mt moduleTargets, importRoot string, bootstrap []byte) (string, error)`; `moduleTargets{ModulePath string, ...}` (zero Packages is valid — `collectModuleTaggedFiles` returns empty); `targLocalModule` constant.
- Produces: test-only `runner.ExportModuleCacheKey(modulePath, importRoot string, bootstrap []byte) (string, error)`.

- [ ] **Step 1: Write the failing test (the issue #27 repro at key level)**

Add to `internal/runner/export_test.go`:

```go
// ExportModuleCacheKey computes a module cache key for testing.
func ExportModuleCacheKey(modulePath, importRoot string, bootstrap []byte) (string, error) {
	return computeModuleCacheKey(moduleTargets{ModulePath: modulePath}, importRoot, bootstrap)
}
```

Add to `internal/runner/cache_key_test.go`:

```go
func TestModuleCacheKey_ReplaceTargetEdits(t *testing.T) {
	t.Parallel()

	t.Run("EditingReplaceTargetChangesKey", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		dep := t.TempDir()
		writeTestFile(g, dep, "go.mod", "module example.com/dep\n\ngo 1.25\n")
		writeTestFile(g, dep, "dep.go", "package dep\n\nconst Timeout = 30\n")

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+dep+"\n")

		before, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("bootstrap"))
		g.Expect(err).NotTo(HaveOccurred())

		writeTestFile(g, dep, "dep.go", "package dep\n\nconst Timeout = 600\n")

		after, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("bootstrap"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(after).NotTo(Equal(before),
			"issue #27: editing a local replace target must invalidate the cached binary")
	})

	t.Run("UnchangedInputsKeepKeyStable", func(t *testing.T) {
		t.Parallel()
		g := NewGomegaWithT(t)

		dep := t.TempDir()
		writeTestFile(g, dep, "go.mod", "module example.com/dep\n\ngo 1.25\n")
		writeTestFile(g, dep, "dep.go", "package dep\n")

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+dep+"\n")

		first, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("bootstrap"))
		g.Expect(err).NotTo(HaveOccurred())

		second, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("bootstrap"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(second).To(Equal(first), "no edits, no invalidation — the cache must still hit")
	})
}
```

- [ ] **Step 2: Run test to verify it fails (RED — the bug reproduced)**

Run: `targ test`
Expected: `EditingReplaceTargetChangesKey` FAILS with the two keys EQUAL (today's stale-cache bug, reproduced); `UnchangedInputsKeepKeyStable` passes. Both outcomes matter: the pair proves the fixture distinguishes the bug from the fix.

- [ ] **Step 3: Write the minimal wiring**

In `computeModuleCacheKey`, extend the real-module branch:

```go
	// Only walk the module directory for real modules. Pseudo-modules (targ.local)
	// have no go.mod and their "root" may be a large directory like ~/ that should
	// not be walked. Tagged files alone suffice for cache invalidation.
	if mt.ModulePath != targLocalModule {
		moduleFiles, err := collectModuleFiles(importRoot)
		if err != nil {
			return "", fmt.Errorf("gathering module files: %w", err)
		}

		replaceFiles, err := collectReplaceDirFiles(importRoot)
		if err != nil {
			return "", fmt.Errorf("gathering replace target files: %w", err)
		}

		cacheInputs = slices.Concat(taggedFiles, moduleFiles, replaceFiles)
	}
```

- [ ] **Step 4: Run tests to verify they pass (GREEN)**

Run: `targ test`
Expected: PASS — all cache_key_test.go tests green, no other test regressions.

- [ ] **Step 5: Refactor check + full verification**

Review `computeModuleCacheKey` complexity (it gains one call — should stay under lint limits; if lint flags it, extract the real-module branch into a `collectCacheInputs` helper). Run `targ reorder-decls`, then `targ check-full`.
Expected: fully green (lint, coverage ≥80% on `collectReplaceDirFiles`, `filesystemReplaceDirs`, `computeModuleCacheKey`, ordering, deadcode, nils). **Gate B fires here** on the combined Task 1+2 diff.

- [ ] **Step 6: Commit**

```bash
git add internal/runner/runner.go internal/runner/export_test.go internal/runner/cache_key_test.go go.mod go.sum
git commit -m "fix(runner): include local replace targets in bootstrap cache key

Editing a go.mod filesystem replace directive's target directory now
invalidates the cached bootstrap binary. Previously the key hashed only
the consumer module's files, and go.sum carries no entry for local
replace targets, so the stale gate binary kept running silently.

Fixes #27

AI-Used: [claude]"
```

### Task 3: Property test — replace-target sensitivity and determinism

**Files:**
- Modify: `internal/runner/cache_key_test.go`

**Interfaces:**
- Consumes: `runner.ExportModuleCacheKey` from Task 2; `pgregory.net/rapid` (already a dependency — match idiom in `internal/runner/runner_properties_test.go`).
- Produces: nothing downstream.

- [ ] **Step 1: Write the property test**

Add to `internal/runner/cache_key_test.go` (imports gain `pgregory.net/rapid`):

```go
func TestProperty_ReplaceTargetCacheKey(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewGomegaWithT(rt)

		dep := t.TempDir()
		writeTestFile(g, dep, "go.mod", "module example.com/dep\n\ngo 1.25\n")

		name := rapid.StringMatching(`[a-z][a-z0-9]{0,8}\.go`).Draw(rt, "name")
		content1 := "package dep\n// " + rapid.StringMatching(`[ -~]{0,40}`).Draw(rt, "content1") + "\n"
		content2 := "package dep\n// " + rapid.StringMatching(`[ -~]{0,40}`).Draw(rt, "content2") + "\n"
		if content1 == content2 {
			content2 += "//\n"
		}
		writeTestFile(g, dep, name, content1)

		consumer := t.TempDir()
		writeTestFile(g, consumer, "go.mod",
			"module example.com/consumer\n\ngo 1.25\n\nreplace example.com/dep => "+dep+"\n")

		key1, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("b"))
		g.Expect(err).NotTo(HaveOccurred())

		key1again, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("b"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(key1again).To(Equal(key1), "determinism: same tree, same key")

		writeTestFile(g, dep, name, content2)

		key2, err := runner.ExportModuleCacheKey("example.com/consumer", consumer, []byte("b"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(key2).NotTo(Equal(key1), "sensitivity: any content edit changes the key")
	})
}
```

Implementer note: `t.TempDir()` inside `rapid.Check` uses the outer `t` deliberately — rapid's `rt` has no `TempDir`. If the file-count across rapid iterations becomes a problem, use `rt.Draw`-scoped subdirectories under one `t.TempDir()`. If `NewGomegaWithT(rt)` does not accept a `*rapid.T`, follow the existing pattern in `runner_properties_test.go` for assertions inside rapid checks (it may use plain `if` + `rt.Fatalf`); match the repo, do not fight the types.

- [ ] **Step 2: Run test to verify it exercises the fixed path (expected PASS)**

Run: `targ test`
Expected: PASS. This is a characterization property landing after the fix; the red evidence for the behavior was Task 2 Step 2. To confirm the property has teeth, temporarily revert the Task 2 wiring (`git stash push internal/runner/runner.go` is NOT safe here — instead comment out the `replaceFiles` concat locally), run `targ test`, watch this property FAIL, then restore. Do not commit the temporary edit.

- [ ] **Step 3: Full verification**

Run: `targ reorder-decls && targ check-full`
Expected: fully green.

- [ ] **Step 4: Commit**

```bash
git add internal/runner/cache_key_test.go
git commit -m "test(runner): property-test replace-target cache key sensitivity

AI-Used: [claude]"
```

### Task 4: Documentation scrub (per disposition table)

**Files:**
- Modify: `README.md:687`, `README.md:780-787`

**Interfaces:** none (prose only).

- [ ] **Step 1: Apply the three README updates**

Line 687 flag-table row — change:

```markdown
| `--no-cache`                | Force rebuild of the build tool binary       |
```

to:

```markdown
| `--no-binary-cache`         | Force rebuild of the build tool binary       |
```

Lines 780-787 Cache Management section — change:

```markdown
## Cache Management

Targ caches compiled binaries in `~/.cache/targ/`. The cache is invalidated when source files or `go.mod`/`go.sum` change.

```bash
targ --no-cache <command>   # force rebuild
rm -rf ~/.cache/targ/       # clear all cached binaries
```
```

to:

```markdown
## Cache Management

Targ caches compiled binaries in `~/.cache/targ/`. The cache is invalidated when source files or `go.mod`/`go.sum` change — including the sources of any local filesystem `replace` targets named in `go.mod`.

```bash
targ --no-binary-cache <command>   # force rebuild
rm -rf ~/.cache/targ/              # clear all cached binaries
```
```

Before editing, re-grep `--no-cache` in README.md — if other rows/sections reference it beyond lines 687/785, update them identically (fix all instances, not just the first).

- [ ] **Step 2: Verify + commit**

Run: `targ check-full`
Expected: green. **Gate C fires here** (relevance + clarity reviewers over the README diff).

```bash
git add README.md
git commit -m "docs(readme): cache invalidation covers local replace targets; --no-binary-cache

AI-Used: [claude]"
```

---

## Self-review notes

- **Spec coverage:** issue #27's fix-direction (b) = parse filesystem replaces + hash target dirs with the same walk (Tasks 1-2); its stipulated fails-under-today test = Task 2 Step 1-2; workaround docs = Task 4. Scope decisions pinned: all filesystem replaces hashed (not just targ's — any replaced dep affects build output); one level only (no transitive replace-chasing — matches the issue's "same walk as collectModuleFiles"); missing target dir hashes a marker instead of erroring; unparseable go.mod errors (such a module cannot build anyway); go.work out of scope (rejected in docs/archive/research-gomod-tree-traversal.md §5.2).
- **Known cost (accepted):** with a filesystem replace active, every cache-key computation walks and hashes the replaced tree. Dev-mode only; correctness over cache-hit speed there.
- **Type consistency:** `collectReplaceDirFiles(moduleRoot string) ([]discover.TaggedFile, error)` and `ExportModuleCacheKey(modulePath, importRoot string, bootstrap []byte) (string, error)` are used with identical signatures across Tasks 1-3.

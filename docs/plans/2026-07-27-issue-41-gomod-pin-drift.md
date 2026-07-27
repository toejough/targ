# Issue #41 — targ must never change the version a consuming repo's go.mod pins

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

Implementation plan, revision 1. Branch `issue-41-gomod-pin-drift`, primary checkout
`/Users/joe/repos/personal/targ`, based on `ed359e7`.

**Goal:** Stop `targ` from silently rewriting a consuming repository's `go.mod`/`go.sum` to the
latest targ commit as a side effect of any invocation on the multi-module build path.

**Architecture:** `ensureTargDependency` currently runs `go get <module>` with no version, in the
real consumer's module root, with both streams discarded and the error dropped. Read the version
the consumer's own `go.mod` already requires and pin the get to it; that turns the call from a
silent upgrade into a convergent no-op that still repairs a missing `go.sum` entry. Only genuine
first-time integration — no require line at all — adds one, and that case announces itself on
stderr. The exec construction (and its single `gosec` suppression) is shared with the one other
`go get` call site, `fetchPackage`, without changing that site's behavior.

**Tech Stack:** Go 1.25.5; `golang.org/x/mod/modfile` (already a direct require at `go.mod:15`,
already imported at `internal/runner/runner.go:32`). Zero new dependencies.

## What breaks for users today

A repo whose targ files sit inside its own module is unaffected. The bug needs three conditions,
all verified in the issue with a reproduction and a negative control:

1. `targ` discovers **more than one module group** (`runner.go:1908`). A second group appears when
   a `//go:build targ` file that calls `targ.Register` in `init()` lives **outside** any module on
   the ancestor walk-up. On this machine that is one stray file, `/Users/joe/targs.go`.
2. The consuming repo contributes its **own** group — its `go.mod` plus at least one targ package.
3. The binary cache **misses** for that group (cold cache, changed source, or `--no-binary-cache`).

Then *any* invocation fires it, including `targ --help` and `targ __complete`:

```
before: require github.com/toejough/targ v0.0.0-20260724192707-d0eae3bdf0de
after:  require github.com/toejough/targ v0.0.0-20260726010428-2473fce8c5c5
```

No output, no warning, no error. The bump lands in whoever's next commit. Three engram commits
carry a mis-attributed pin bump onto a **docs-only** targ commit; a fourth was uncommitted in that
working tree when the issue was filed. This directly violates the README's own "No Surprises"
design principle.

Since targ publishes no semver tags (`go list -m -versions github.com/toejough/targ` returns
nothing), an unversioned `go get` always resolves to the current tip of `origin/main` — so **every
pushed targ commit, including docs-only ones, re-arms the write for every exposed consumer.**

## Global constraints

- Go 1.25.5. Use `targ` for all build/test/check operations. Never bare `go test` as final
  validation.
- **Verify cwd before every build-runner invocation.** This work happens in the primary checkout
  `/Users/joe/repos/personal/targ` on branch `issue-41-gomod-pin-drift`. The harness resets shell
  cwd between calls. A targ run in the wrong tree is the failure this issue is about.
- **Probe with `--no-binary-cache`.** targ's bootstrap binary cache does not invalidate on a
  local-replace source change, so a probe without the flag can measure a stale binary.
- Lint is measured with the project config: `targ lint-full`. There is no root `.golangci` config,
  so a bare `golangci-lint run ./...` silently falls back to a weak linter set and is not a valid
  measurement.
- **`//nolint` count must go down, not up.** Today there are two `//nolint:gosec` directives on
  `go get` execs (`runner.go:3063` and `runner.go:3255`). After Task 4 there is one.
- **Commit trailer `AI-Used: [claude]`; `Refs #41` only — never a closing keyword**, and never one
  beside an issue number even when *discussing* one. A quoted `Closes #37` in a docs commit body
  auto-closed that issue at push time. Issue #41 is closed deliberately at the end, not by a commit
  body.
- No new package-level mutable state.
- No timing-dependent assertions. Parallel subtests get their own fixtures.
- Declaration order is alphabetical within type/var/func groups. `targ reorder-decls` fixes it.
- Coverage is per-function at 80%, but `internal/runner/runner.go` is **excluded by name**
  (`isEntryPointCoverageLine`, `dev/targets.go:1589-1593`). **No coverage gate will push back on
  this change.** Every test here is written on purpose or it does not exist.

## Design decisions (settled; do not relitigate in-task)

1. **The consuming module's own `go.mod` require line is the only source of truth for the pin.**
   Never `dep.Version`. `resolveTargDependency` (`runner.go:4093`) fills that from the *running
   binary's* `debug.ReadBuildInfo().Main.Version`. Measured at one instant: running binary Jul 23,
   engram's actual pin Jul 24, what the bug wrote Jul 26. Pinning to `dep.Version` would have
   silently **downgraded** engram — a silent rewrite in the other direction. It is also `""`
   whenever targ was built from a devel or dirty checkout (`isCleanVersion`, `runner.go:3726`).

2. **First-time bootstrap announces, then writes** (Joe's call). When no require line exists, print
   to stderr and run the unversioned get. The alternative — hard-fail telling the user to run
   `go get` themselves — was offered and declined. Rationale for the write being *necessary at
   all*: a real consumer's bootstrap build runs under Go's default `-mod=readonly`
   (`-mod` is set only under `if isolated` / `if ctx.usingFallback`, neither true for a real
   consumer), so the build cannot add the require line itself; with the require dropped it fails
   `no required module provides package ...` and leaves `go.mod` unchanged.

3. **The pinned get is not dead weight.** With the require present but targ's `h1:` line removed
   from `go.sum`, the build fails `missing go.sum entry`; `go get M@<the existing pin>` restores
   `go.sum` byte-identically and the build succeeds. `go get M@<pin>` is *convergent*, not
   "no write": it repairs, and never upgrades. Skipping the get entirely when a require line
   exists would give that repair up.

4. **`fetchPackage`'s `--sync` upgrade semantics are preserved exactly.** Joe chose to converge the
   two `go get` sites onto a shared helper. **Recorded dissent:** I flagged that converging risks
   altering `--sync`, whose unversioned upgrade *is the point* (`README.md:824`: "Re-running
   `--sync` updates the module version (like `go get -u`)"). Joe chose to fold it in. The
   convergence is therefore limited to **command construction only** — `goGetCmd(dir, arg)`
   returns a configured `*exec.Cmd`; each caller keeps its own streams, its own error handling,
   and its own arg. `fetchPackage` keeps `dir == ""` (process CWD, today's exact behavior) and
   keeps streaming to `os.Stdout`/`os.Stderr`. The issue notes the missing `cmd.Dir` as a wart;
   **fixing it is out of scope** — it would change `--sync` behavior nobody asked to change.

5. **Integration tests get a real leg** (Joe's call). `CLAUDE.md:38` requires IO-purpose tests to
   carry `//go:build integration`, but the repo has zero tagged files and `dev/` passes `-tags`
   exactly once (`-tags=mutation`, `dev/targets.go:1851`). A tagged test with no leg never runs.
   Task 3 adds `TestIntegration` (`targ test-integration`) and wires it into `CheckFull` and
   `CheckForFail`, so the guard actually executes.

6. **Integration tests are named `TestIntegration*` and the leg selects them with `-run`.** This
   keeps the leg cheap and repo-wide at once: `-tags=integration -run 'TestIntegration' ./...`
   compiles every package but executes only integration tests, so the leg neither duplicates the
   unit suite nor needs a hand-maintained package list.

7. **The integration leg writes no coverage profile.** `CheckCoverageForFail` reads `coverage.out`
   and runs in the same parallel dep group; a second `-coverprofile` writer would race it.

8. **The integration test is hermetic and serial.** A `file://` GOPROXY serves two versions of a
   stand-in module, with `GOMODCACHE`, `GOPROXY`, `GOSUMDB`, `GONOSUMDB`, `GOFLAGS` and `GOPATH`
   redirected into `t.TempDir()`. No network, no pollution of the real module cache. It uses
   `t.Setenv`, which forbids `t.Parallel` — so these tests are serial by construction, which is
   correct for process-global env.

## Doc-surface enumeration grep

Two invariants change: (A) *whether targ may write a consuming repo's `go.mod`*, and (B) *the targ
command set*. Grep run against the working tree at `ed359e7`:

```bash
grep -rniE "go get|go\.mod|go\.sum|\bpin(ned|s)?\b|module version|go get -u|bootstrap" --include='*.md' .
grep -rniE "check-full|targ test|integration|-tags" README.md CLAUDE.md docs/archive/*.md docs/specs/*.md
grep -rn -- "-tags" dev/*.go
grep -rn "go:build integration" --include='*.go' .
```

**Invariant A — targ writing to a consumer's `go.mod`:**

| File:line | Disposition | Reason |
| --- | --- | --- |
| `README.md:708` (Multi-Module Targets) | **update** | The only prose describing the affected path. Must state that targ never changes the version your `go.mod` pins, and name the one announced exception. |
| `README.md:30` (`go get github.com/toejough/targ`) | keep | User-run install instruction; after the fix this is exactly what a first-time consumer is told to run. |
| `README.md:824` (`--sync` updates the module version, like `go get -u`) | keep | Describes `fetchPackage`, whose upgrade is deliberate and preserved (decision 4). This line **is** the contract Task 4 must not break. |
| `README.md:195`, `:514`, `:828` (`go.mod` in cache globs / cache invalidation) | N/A | Binary-cache keying, not module writes. |
| `README.md:731` (`--keep` bootstrap file) | N/A | Different "bootstrap" — the generated `.go` file. |
| `docs/archive/architecture.md:709` (`Update module version (go get -u style)`) | keep | Archived; describes `--sync`, same preserved contract as `README.md:824`. |
| `docs/archive/architecture.md:486-487` (go.mod discovery) | N/A | Discovery, not writes. |
| `docs/archive/research-gomod-tree-traversal.md` (`:113`, `:410`, `:446` cite `EnsureFallbackModuleRoot`) | keep | Historical research doc, already stale against HEAD (cites lines 423-451 for a function now at 437). `docs/archive/` is not maintained against HEAD; deleting a function does not obligate rewriting an archived research record. |
| `projects/portable-targets/{architecture,design,requirements,tasks}.md` (many `go get -u` / `go.mod` hits) | N/A | A different project's spec docs vendored under `projects/`; all hits concern a hypothetical shared-targets module, not targ's bootstrap. |
| `docs/specs/*.md` | N/A | Grep returned no claim about module writes. |

**Invariant B — the targ command set:**

| File:line | Disposition | Reason |
| --- | --- | --- |
| `CLAUDE.md:38` (IO tests get `//go:build integration`) | **update** | The rule currently dead-ends — no leg ran tagged tests. Must name `targ test-integration` and the `TestIntegration*` naming convention. |
| `CLAUDE.md:40` (Always run `check-full`; parenthetical enumerates legs) | **update** | The enumeration must include integration tests. |
| `README.md:72-73`, `:156` (`targ test --package=./...`) | N/A | Documents targ's *example* target for users, not this repo's dev legs. |
| `README.md:637` (`targ build -t integration -t linux`) | N/A | targ's `-t` build-tag passthrough example; unrelated to dev legs. |
| `README.md:725-733` (Build Tool Flags table) | N/A | targ's own CLI flags, not dev targets. |
| `docs/archive/architecture.md:372` (`targ.Group("test", testUnit, testIntegration)`) | N/A | Illustrative example in an archived doc. |
| `docs/specs/tests.md:279`, `:326`, `:329` | N/A | "integration" used descriptively about coverage strategy, not the command set. |
| `internal/runner/runner_properties_test.go:457` | N/A | The string `"//go:build integration"` inside a fixture; not a real tagged file. |

## File structure

| File | Responsibility | Change |
| --- | --- | --- |
| `internal/runner/runner.go` | The runner. Holds `pinnedModuleVersion`, `goGetCmd`, `ensureTargDependency`, `fetchPackage`. | Modify |
| `internal/runner/runner_gomod_test.go` | Unit tests for `pinnedModuleVersion` — plain filesystem, no toolchain. | Create |
| `internal/runner/runner_gomod_integration_test.go` | `//go:build integration`. Hermetic `file://`-proxy proof that `ensureTargDependency` never moves the pin. | Create |
| `dev/targets.go` | Build-system target registry. | Modify (add `TestIntegration`) |
| `README.md` | User-facing docs. | Modify (`:708` section) |
| `CLAUDE.md` | Repo working agreements. | Modify (`:38`, `:40`) |

---

## Task 1: `pinnedModuleVersion` — read the consumer's own pin

**Files:**
- Modify: `internal/runner/runner.go` (insert between `parseSingleValueArg`, ends `:3854`, and the
  doc comment of `prepareBuildContext`, `:3856`)
- Modify: `internal/runner/export_test.go` (add the `ExportPinnedModuleVersion` shim)
- Test: `internal/runner/runner_gomod_test.go` (create)

**Test-package convention — follow it, do not invent one.** `internal/runner` has 6 test files in
`package runner_test` and exactly one, `export_test.go`, in `package runner`. `export_test.go`
contains **zero** `TestXxx` functions: it is purely a shim exposing `ExportXxx` wrappers over
unexported internals. Every real test in this package, without exception, lives in
`package runner_test` and reaches internals through that shim. New tests follow that pattern —
a `TestXxx` in `package runner` would compile fine but would be the first of its kind here.

**Interfaces:**
- Consumes: nothing from earlier tasks. `goModFile` is an existing package constant;
  `golang.org/x/mod/modfile` is already imported at `runner.go:32`.
- Produces: `func pinnedModuleVersion(moduleRoot, modulePath string) string` — returns the version
  `modulePath` is required at in `moduleRoot/go.mod`, or `""` when not required / unreadable /
  unparseable. Task 2 calls it.

**Why alphabetical placement matters:** `pa` < `pi` < `pr`. Put it anywhere else and
`targ reorder-decls-check` fails `check-full`.

- [ ] **Step 1: Add the export shim**

In `internal/runner/export_test.go`, between `ExportModuleCacheKey` (`:98`) and
`ExportPrepareBootstrap` (`:104`) — `…M` < `…Pi` < `…Pr`:

```go
// ExportPinnedModuleVersion wraps pinnedModuleVersion for testing.
func ExportPinnedModuleVersion(moduleRoot, modulePath string) string {
	return pinnedModuleVersion(moduleRoot, modulePath)
}
```

- [ ] **Step 2: Write the failing test**

Create `internal/runner/runner_gomod_test.go` in `package runner_test`, reaching the unexported
function through the shim:

```go
package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/toejough/targ/internal/runner"
)

func TestPinnedModuleVersion(t *testing.T) {
	t.Parallel()

	const target = "github.com/toejough/targ"

	cases := []struct {
		name    string
		goMod   string // "" means: do not create a go.mod at all
		want    string
	}{
		{
			name: "require inside a block",
			goMod: `module example.com/consumer

go 1.25.5

require (
	github.com/toejough/targ v0.0.0-20260724192707-d0eae3bdf0de
	golang.org/x/mod v0.32.0
)
`,
			want: "v0.0.0-20260724192707-d0eae3bdf0de",
		},
		{
			name: "require on its own line",
			goMod: `module example.com/consumer

go 1.25.5

require github.com/toejough/targ v1.2.3
`,
			want: "v1.2.3",
		},
		{
			name: "module absent from go.mod",
			goMod: `module example.com/consumer

go 1.25.5

require golang.org/x/mod v0.32.0
`,
			want: "",
		},
		{
			name:  "go.mod absent",
			goMod: "",
			want:  "",
		},
		{
			name: "go.mod unparseable",
			goMod: `module example.com/consumer
require ( github.com/toejough/targ
`,
			want: "",
		},
		{
			name: "module present only as a replace target",
			goMod: `module example.com/consumer

go 1.25.5

require golang.org/x/mod v0.32.0

replace github.com/toejough/targ => ../targ
`,
			want: "",
		},
		{
			name: "require carries an indirect comment",
			goMod: `module example.com/consumer

go 1.25.5

require github.com/toejough/targ v0.9.0 // indirect
`,
			want: "v0.9.0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Each subtest gets its own directory. Never share a fixture dir.
			dir := t.TempDir()
			if tc.goMod != "" {
				path := filepath.Join(dir, "go.mod")
				if err := os.WriteFile(path, []byte(tc.goMod), 0o600); err != nil {
					t.Fatalf("writing fixture go.mod: %v", err)
				}
			}

			got := runner.ExportPinnedModuleVersion(dir, target)
			if got != tc.want {
				t.Errorf("ExportPinnedModuleVersion(%q, %q) = %q, want %q", dir, target, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 3: Run the test and verify it fails**

```bash
cd /Users/joe/repos/personal/targ
go test ./internal/runner/ -run TestPinnedModuleVersion
```

Expected: **compilation failure**, `undefined: pinnedModuleVersion`, reported against
`export_test.go` — the shim from Step 1 references a function that does not exist yet. That is the
RED step. If it compiles, the function already exists — stop and re-read the tree.

- [ ] **Step 4: Write the minimal implementation**

Insert into `internal/runner/runner.go` immediately after `parseSingleValueArg` (which ends at
`:3854`) and before the `// prepareBuildContext determines build roots...` comment at `:3856`:

```go
// pinnedModuleVersion returns the version modulePath is required at in the go.mod
// under moduleRoot. It returns "" when the module is not required there (genuine
// first-time integration) or when go.mod cannot be read or parsed.
func pinnedModuleVersion(moduleRoot, modulePath string) string {
	//nolint:gosec // build tool reads the consuming module's go.mod by design
	data, err := os.ReadFile(filepath.Join(moduleRoot, goModFile))
	if err != nil {
		return ""
	}

	parsed, err := modfile.Parse(goModFile, data, nil)
	if err != nil {
		return ""
	}

	for _, req := range parsed.Require {
		if req.Mod.Path == modulePath {
			return req.Mod.Version
		}
	}

	return ""
}
```

- [ ] **Step 5: Run the test and verify it passes**

```bash
cd /Users/joe/repos/personal/targ
go test ./internal/runner/ -run TestPinnedModuleVersion -v
```

Expected: PASS, 7 subtests.

- [ ] **Step 6: Prove the test discriminates**

A passing table test proves nothing unless a wrong implementation fails it. Temporarily change the
`for` loop body to `return ""` (i.e. never find a require). Re-run.

Expected: FAIL on all four cases that expect a non-empty version. Restore the loop, re-run, confirm
PASS. Do not skip this — the coverage gate excludes this file, so this probe is the only thing
standing between a real test and a decorative one.

- [ ] **Step 7: Verify declaration ordering**

```bash
cd /Users/joe/repos/personal/targ
targ reorder-decls-check
```

Expected: pass. If it reports a move, the insertion point was wrong — fix the placement rather than
accepting the auto-move, so the diff stays minimal.

- [ ] **Step 8: Commit**

```bash
cd /Users/joe/repos/personal/targ
git add internal/runner/runner.go internal/runner/export_test.go internal/runner/runner_gomod_test.go
git commit -m "feat(runner): read the version a consumer's go.mod pins targ at

Refs #41

AI-Used: [claude]"
```

---

## Task 2: pin the bootstrap `go get`, announce first-time adds, stop discarding errors

**Files:**
- Modify: `internal/runner/runner.go:3061-3069` (`ensureTargDependency`)
- Modify: `internal/runner/runner.go:2257` (the single call site, inside `buildAndQueryBinary`)

**Interfaces:**
- Consumes: `pinnedModuleVersion(moduleRoot, modulePath string) string` from Task 1.
- Produces: `func ensureTargDependency(dep TargDependency, importRoot string, errOut io.Writer)` —
  arity change from 2 to 3. Task 4 rewrites its exec construction.

**Why there is no unit test in this task:** the deliverable is an exec against the module proxy.
Testing it for real is Task 3, which needs the `//go:build integration` leg to exist. Writing a
decorative test here that never runs `go get` is false-green trap #1 from the issue — it passes
identically against the broken code, because it never calls the broken function. The compiler is
the only check this task earns; Task 3 supplies the behavioral proof.

- [ ] **Step 1: Change the call site**

`internal/runner/runner.go:2257`, inside `buildAndQueryBinary`, which already has an
`errOut io.Writer` parameter:

```go
-	ensureTargDependency(dep, ctx.importRoot)
+	ensureTargDependency(dep, ctx.importRoot, errOut)
```

- [ ] **Step 2: Rewrite `ensureTargDependency`**

Replace `internal/runner/runner.go:3061-3069` in full:

```go
// ensureTargDependency makes the targ module resolvable for the build in importRoot
// without ever changing which version that module pins.
//
// When importRoot's go.mod already requires the module, the go get is pinned to that
// exact version: it is a no-op on a consistent module and at most repairs a missing
// go.sum entry. Only genuine first-time integration -- no require line at all -- adds
// one, and that is announced, because the normal build path runs under -mod=readonly
// and cannot add it itself. Failures are reported rather than discarded.
func ensureTargDependency(dep TargDependency, importRoot string, errOut io.Writer) {
	arg := dep.ModulePath

	pinned := pinnedModuleVersion(importRoot, dep.ModulePath)
	if pinned != "" {
		arg += "@" + pinned
	} else {
		fmt.Fprintf(errOut, "targ: %s is not required by %s; adding it with 'go get %s'\n",
			dep.ModulePath, filepath.Join(importRoot, goModFile), dep.ModulePath)
	}

	var output bytes.Buffer

	//nolint:gosec // build tool runs go get by design
	getCmd := exec.CommandContext(context.Background(), "go", "get", arg)
	getCmd.Dir = importRoot
	getCmd.Stdout = &output
	getCmd.Stderr = &output

	err := getCmd.Run()
	if err != nil {
		fmt.Fprintf(errOut, "targ: go get %s failed: %v\n%s", arg, err, output.String())
	}
}
```

- [ ] **Step 3: Verify the arity change proves there is exactly one caller**

```bash
cd /Users/joe/repos/personal/targ
go build ./... && go vet ./internal/runner/ && go test ./internal/runner/ -count=1
```

Expected: all three exit 0. A clean build after an arity change **is** the proof that
`grep -rn "ensureTargDependency"`'s three hits (comment, definition, one call) are the whole story.
If anything fails to compile, a second caller exists — stop and re-scope.

- [ ] **Step 4: Confirm `bytes` is imported**

```bash
cd /Users/joe/repos/personal/targ
grep -n '"bytes"' internal/runner/runner.go
```

Expected: a hit. If absent, `go build` in Step 3 already failed — add it to the import block in
sorted position and re-run Step 3.

- [ ] **Step 5: Commit**

```bash
cd /Users/joe/repos/personal/targ
git add internal/runner/runner.go
git commit -m "fix(runner): pin the bootstrap go get to the consumer's own require line

targ ran 'go get github.com/toejough/targ' with no version in the consuming
repo's module root, discarding both streams and the error. With no semver tags
published, that resolves to the tip of origin/main, so any invocation on the
multi-module path silently moved the consumer's pin -- including targ --help and
targ __complete.

Read the version the consumer's go.mod already requires and pin the get to it.
That keeps the call convergent: a no-op on a consistent module, a go.sum repair
on an inconsistent one, never an upgrade. Genuine first-time integration still
adds the require line, because the build runs under -mod=readonly and cannot,
but it now says so on stderr. Failures are reported instead of dropped.

Refs #41

AI-Used: [claude]"
```

---

## Task 3: the integration leg, and the test that proves the pin holds

**Files:**
- Create: `internal/runner/runner_gomod_integration_test.go`
- Modify: `internal/runner/export_test.go` (add the `ExportEnsureTargDependency` shim)
- Modify: `dev/targets.go` (`init()` deps + `targ.Register` list, and the exported `var` block)

**Same test-package convention as Task 1:** the test lives in `package runner_test` and reaches
`ensureTargDependency` through an `ExportXxx` shim. `TargDependency` is already exported
(`runner.go:130`), so only the function needs wrapping.

**Interfaces:**
- Consumes: `ensureTargDependency(dep TargDependency, importRoot string, errOut io.Writer)` from
  Task 2; `TargDependency` (existing struct, field `ModulePath string`).
- Produces: `TestIntegration*` naming convention; `dev.TestIntegration` target (`targ
  test-integration`).

**The four false-green traps this test must dodge** (from the issue):

1. Testing only the pure decision. Task 1 already does that; it is not a regression test.
2. Using a single-module fixture — that is the negative control, it cannot drift either way.
3. Reusing a fixture directory across cases — the binary cache is keyed on `sha256(importRoot)`.
   Every case gets a fresh `t.TempDir()`.
4. Pinning the fixture to the version the proxy would resolve anyway — then the unversioned get is
   a no-op and the *broken* code passes. The fixture pins `v1.0.0` while the proxy also serves
   `v1.1.0`, so an unversioned get visibly moves.

This test calls `ensureTargDependency` directly with a **stand-in module path**, not
`github.com/toejough/targ`. That is deliberate: it isolates the invariant under test (does the get
move the pin?) from targ's own publication state, and it is what makes the `file://` proxy possible
with no network.

- [ ] **Step 1: Add the export shim**

In `internal/runner/export_test.go`, between `ExportCollectSortedCommands` (`:45`) and
`ExportFindCommandBinary` (`:78`) — `…C` < `…E` < `…F`:

```go
// ExportEnsureTargDependency wraps ensureTargDependency for testing.
func ExportEnsureTargDependency(dep TargDependency, importRoot string, errOut io.Writer) {
	ensureTargDependency(dep, importRoot, errOut)
}
```

`io` is already imported in `export_test.go`.

Note this shim lives in a file with no build tag, so it compiles into every build of the test
package — that is fine and is why `ensureTargDependency` must already have its Task 2 signature
before this task starts.

- [ ] **Step 2: Write the failing test**

Create `internal/runner/runner_gomod_integration_test.go`:

```go
//go:build integration

package runner_test

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toejough/targ/internal/runner"
)

// standInModule is a throwaway module path served only by the local file:// proxy.
// Using a stand-in rather than github.com/toejough/targ keeps the test hermetic and
// independent of what targ has published.
const standInModule = "example.com/standin"

// TestIntegrationEnsureTargDependencyKeepsThePin is the regression test for #41.
// It must FAIL against the pre-fix unversioned `go get`.
func TestIntegrationEnsureTargDependencyKeepsThePin(t *testing.T) {
	newStandInProxy(t)

	t.Run("existing require line is left byte-identical", func(t *testing.T) {
		root := newConsumerModule(t, "v1.0.0")

		before, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err != nil {
			t.Fatalf("reading go.mod: %v", err)
		}

		var errOut bytes.Buffer

		runner.ExportEnsureTargDependency(runner.TargDependency{ModulePath: standInModule}, root, &errOut)

		after, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err != nil {
			t.Fatalf("reading go.mod: %v", err)
		}

		if !bytes.Equal(before, after) {
			t.Errorf("go.mod changed.\nbefore:\n%s\nafter:\n%s", before, after)
		}

		if errOut.Len() != 0 {
			t.Errorf("expected silence on a consistent module, got: %s", errOut.String())
		}
	})

	t.Run("absent require line is announced and added", func(t *testing.T) {
		root := newConsumerModule(t, "")

		var errOut bytes.Buffer

		runner.ExportEnsureTargDependency(runner.TargDependency{ModulePath: standInModule}, root, &errOut)

		if !strings.Contains(errOut.String(), "is not required by") {
			t.Errorf("expected an announcement on stderr, got: %q", errOut.String())
		}

		after, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err != nil {
			t.Fatalf("reading go.mod: %v", err)
		}

		if !strings.Contains(string(after), standInModule) {
			t.Errorf("expected %s to be added to go.mod, got:\n%s", standInModule, after)
		}
	})
}

// newConsumerModule creates a fresh module rooted at a new temp dir. When pin is
// non-empty the module requires standInModule at that version; when empty it
// requires nothing. Returns the module root.
//
// A fresh directory per call is required: targ's binary cache is keyed on
// sha256(importRoot), so a reused path can silently skip the code under test.
func newConsumerModule(t *testing.T, pin string) string {
	t.Helper()

	root := t.TempDir()

	goMod := "module example.com/consumer\n\ngo 1.25.5\n"
	if pin != "" {
		goMod += "\nrequire " + standInModule + " " + pin + "\n"
	}

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatalf("writing consumer go.mod: %v", err)
	}

	if pin != "" {
		// Populate go.sum so the fixture is consistent before the call under test.
		cmd := exec.Command("go", "mod", "download", standInModule)
		cmd.Dir = root

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("seeding go.sum: %v\n%s", err, out)
		}
	}

	return root
}

// newStandInProxy builds a file:// module proxy serving standInModule at v1.0.0 and
// v1.1.0, and redirects every module-resolution env var at it. The redirection is
// process-global, so callers need no handle -- creating the proxy IS the setup.
//
// It uses t.Setenv, which forbids t.Parallel -- correct, because the Go module env is
// process-global. Integration tests here are serial by construction.
func newStandInProxy(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	modDir := filepath.Join(dir, "proxy", standInModule, "@v")

	if err := os.MkdirAll(modDir, 0o750); err != nil {
		t.Fatalf("creating proxy dir: %v", err)
	}

	for _, v := range []string{"v1.0.0", "v1.1.0"} {
		writeProxyVersion(t, modDir, v)
	}

	if err := os.WriteFile(filepath.Join(modDir, "list"), []byte("v1.0.0\nv1.1.0\n"), 0o600); err != nil {
		t.Fatalf("writing proxy list: %v", err)
	}

	t.Setenv("GOPROXY", "file://"+filepath.Join(dir, "proxy"))
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GONOSUMDB", "*")
	t.Setenv("GONOSUMCHECK", "1")
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOPRIVATE", standInModule)
	t.Setenv("GOMODCACHE", filepath.Join(dir, "modcache"))
}

// writeProxyVersion writes the .info, .mod and .zip a file:// proxy needs for one
// version of standInModule.
func writeProxyVersion(t *testing.T, modDir, version string) {
	t.Helper()

	info := `{"Version":"` + version + `","Time":"2026-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(modDir, version+".info"), []byte(info), 0o600); err != nil {
		t.Fatalf("writing %s.info: %v", version, err)
	}

	mod := "module " + standInModule + "\n\ngo 1.25.5\n"
	if err := os.WriteFile(filepath.Join(modDir, version+".mod"), []byte(mod), 0o600); err != nil {
		t.Fatalf("writing %s.mod: %v", version, err)
	}

	writeModuleZip(t, filepath.Join(modDir, version+".zip"), version, mod)
}
```

`writeModuleZip` is the one piece with a format contract worth stating precisely: a module zip
contains paths prefixed `<module>@<version>/`. Add it in alphabetical position:

```go
// writeModuleZip writes a minimal module zip at path. Entries must be prefixed
// "<module>@<version>/" or the go toolchain rejects the archive.
func writeModuleZip(t *testing.T, path, version, goMod string) {
	t.Helper()

	f, err := os.Create(path) //nolint:gosec // test fixture path from t.TempDir
	if err != nil {
		t.Fatalf("creating module zip: %v", err)
	}

	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	prefix := standInModule + "@" + version + "/"

	files := map[string]string{
		prefix + "go.mod":     goMod,
		prefix + "standin.go": "package standin\n",
	}

	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("creating zip entry %s: %v", name, err)
		}

		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("writing zip entry %s: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("closing module zip: %v", err)
	}
}
```

`"archive/zip"` is already in the import block shown in Step 1 for this reason.

- [ ] **Step 3: Run it and verify it fails against the CURRENT code**

This is the step that makes the whole task worth doing. Stash Task 2's fix and run the test against
the broken implementation:

```bash
cd /Users/joe/repos/personal/targ
git stash push -u -m "issue-41-red-probe" -- internal/runner/runner.go
git stash list --format='%H %gs' | head -3   # capture YOUR entry's SHA before anything else
go test -tags=integration -run 'TestIntegration' ./internal/runner/ -count=1 -v
```

Expected: **FAIL** on `existing require line is left byte-identical`, showing the pin moved
`v1.0.0` → `v1.1.0`. If it PASSES, the test does not exercise the bug — fix the fixture (most
likely trap 4: the pin already equals what the proxy resolves) before going further.

Restore with `git stash apply <the SHA you captured>`, then drop that entry by re-finding it by
tag. Never bare `git stash pop` — the stash stack is shared with other worktrees and other
sessions.

- [ ] **Step 4: Run it against the FIXED code**

```bash
cd /Users/joe/repos/personal/targ
go test -tags=integration -run 'TestIntegration' ./internal/runner/ -count=1 -v
```

Expected: PASS, both subtests.

- [ ] **Step 5: Add the `TestIntegration` target to `dev/targets.go`**

Three edits. Two are order-gated; derive their slots by comparing strings, not by name
relatedness — `TestForFail` < `TestIntegration` < `Tidy`, because the shared prefix `Test` is
followed by `F`(70) < `I`(73), and `Te` < `Ti`. Confirm before editing:

```bash
cd /Users/joe/repos/personal/targ
python3 -c "print(sorted(['Test','TestForFail','TestIntegration','Tidy']))"
```

In the exported `var` block, between **`TestForFail`** and **`Tidy`**:

```go
	TestIntegration      = targ.Targ(testIntegration).Description("Run integration tests")
```

In `targ.Register(...)`, after `TestForFail`:

```go
		TestIntegration,
```

That list is **not** order-gated — `go-reorder` reorders top-level declarations via the AST, not
arguments inside a call expression in `init()`'s body. (The existing list is already not strictly
sorted: `CheckUncommitted` follows `CheckThinAPI`, and `Watch, Status` is inverted at the end.)
Match the surrounding style; nothing will fail if you don't.

And the function, between **`testForFail`** and **`tidy`** — same byte-order reasoning as the var
block:

```go
func testIntegration(ctx context.Context) error {
	targ.Print(ctx, "Running integration tests...\n")

	// No -coverprofile: CheckCoverageForFail reads coverage.out in the same parallel
	// dep group, and a second writer would race it.
	//
	// -run TestIntegration keeps the leg repo-wide without duplicating the unit suite:
	// every package compiles, only integration tests execute.
	return targ.RunContext(ctx,
		"go",
		"test",
		"-tags=integration",
		"-buildvcs=false",
		"-timeout=10m",
		"-count=1",
		"-run", "TestIntegration",
		"./...",
	)
}
```

- [ ] **Step 6: Wire it into the check legs**

In `dev/targets.go`'s `init()`, add `TestIntegration` to both aggregate gates. A tagged test with no
leg never runs; a leg no gate calls is the same failure one level up.

```go
-	CheckForFail.Deps(CheckCoverageForFail, CheckUncommitted, ReorderDeclsCheck, LintFast, LintForFail, Deadcode, CheckThinAPI, CheckNilsForFail, targ.DepModeParallel)
-	CheckFull.Deps(CheckCoverageForFail, CheckUncommitted, ReorderDeclsCheck, LintFast, LintFull, Deadcode, CheckThinAPI, CheckNilsForFail, targ.DepModeParallel, targ.CollectAllErrors)
+	CheckForFail.Deps(CheckCoverageForFail, CheckUncommitted, ReorderDeclsCheck, LintFast, LintForFail, Deadcode, CheckThinAPI, CheckNilsForFail, TestIntegration, targ.DepModeParallel)
+	CheckFull.Deps(CheckCoverageForFail, CheckUncommitted, ReorderDeclsCheck, LintFast, LintFull, Deadcode, CheckThinAPI, CheckNilsForFail, TestIntegration, targ.DepModeParallel, targ.CollectAllErrors)
```

- [ ] **Step 7: Verify declaration ordering in this task, not a later one**

```bash
cd /Users/joe/repos/personal/targ
targ reorder-decls-check
```

Expected: pass. This task edits two order-gated lists, so it checks them itself — deferring the
check to Task 7 would surface the failure against an already-committed file. If it reports a move,
the slot was wrong; fix the placement rather than accepting the auto-move.

- [ ] **Step 8: Run the new leg through targ itself**

```bash
cd /Users/joe/repos/personal/targ
targ --no-binary-cache test-integration
```

Expected: the leg appears in `targ --help`, runs, and passes. `--no-binary-cache` is required —
`dev/targets.go` just changed, and a stale bootstrap binary would run the old target list.

- [ ] **Step 9: Confirm the leg did not dirty the tree**

```bash
cd /Users/joe/repos/personal/targ
git status --porcelain
```

Expected: only the files this task edits. If `go.mod`/`go.sum` appear, the test escaped its temp
env — that is this very bug, reproduced by its own test. Fix the env redirection before continuing.

- [ ] **Step 10: Commit**

```bash
cd /Users/joe/repos/personal/targ
git add internal/runner/runner_gomod_integration_test.go internal/runner/export_test.go dev/targets.go
git commit -m "test(runner): guard the pin against a hermetic two-version proxy

Adds the repo's first //go:build integration file, plus the leg that runs it.
A tagged test with no leg never executes, so 'targ test-integration' lands with
it and both CheckFull and CheckForFail depend on it.

The test serves a stand-in module at v1.0.0 and v1.1.0 from a file:// proxy and
pins the fixture at v1.0.0, so an unversioned get visibly drifts. Verified RED
against the pre-fix runner.

Refs #41

AI-Used: [claude]"
```

---

## Task 4: converge the two `go get` call sites onto one constructor

**Files:**
- Modify: `internal/runner/runner.go` — add `goGetCmd`, rewrite the exec construction in
  `ensureTargDependency` (`:3062`) and `fetchPackage` (`:3254`)

**Interfaces:**
- Consumes: `ensureTargDependency` from Task 2, `fetchPackage` (existing).
- Produces: `func goGetCmd(dir, arg string) *exec.Cmd` — a configured `go get` command. Streams and
  error handling stay with the caller.

**The constraint this task must not break:** `README.md:824` — "Re-running `--sync` updates the
module version (like `go get -u`)". `fetchPackage`'s unversioned get is *deliberate*; `--sync`'s job
is to upgrade. Convergence is command construction only. `fetchPackage` keeps `dir == ""` (process
CWD, exactly today's behavior) and keeps streaming to `os.Stdout`/`os.Stderr`.

**Success criterion:** the `//nolint:gosec` count on `go get` execs goes 2 → 1.

- [ ] **Step 1: Add `goGetCmd`**

Alphabetical placement: between `goEnv` (`:3645`) and `groupByModule` (`:3660`) — `goE` < `goG` <
`gr`. Confirm against the live file rather than trusting this line; neighbours move as the file
does:

```bash
cd /Users/joe/repos/personal/targ
grep -n '^func g' internal/runner/runner.go
```

Then:

```go
// goGetCmd builds the `go get arg` command, rooted at dir. An empty dir means the
// process working directory. Callers own the streams and the error: the two sites
// differ deliberately -- the bootstrap get is pinned and quiet, `--sync` is an
// unversioned upgrade that streams its progress.
func goGetCmd(dir, arg string) *exec.Cmd {
	//nolint:gosec // build tool runs go get by design; arg is a module path
	cmd := exec.CommandContext(context.Background(), "go", "get", arg)
	cmd.Dir = dir

	return cmd
}
```

- [ ] **Step 2: Point `ensureTargDependency` at it**

```go
-	var output bytes.Buffer
-
-	//nolint:gosec // build tool runs go get by design
-	getCmd := exec.CommandContext(context.Background(), "go", "get", arg)
-	getCmd.Dir = importRoot
-	getCmd.Stdout = &output
-	getCmd.Stderr = &output
+	var output bytes.Buffer
+
+	getCmd := goGetCmd(importRoot, arg)
+	getCmd.Stdout = &output
+	getCmd.Stderr = &output
```

- [ ] **Step 3: Point `fetchPackage` at it**

```go
 // fetchPackage runs go get to fetch a package.
 func fetchPackage(packagePath string) error {
-	//nolint:gosec // G204: packagePath is from parsed Go source imports
-	cmd := exec.CommandContext(context.Background(), "go", "get", packagePath)
+	// dir is empty on purpose: `--sync` has always run in the process working
+	// directory, and its unversioned get is a deliberate upgrade (README "Re-running
+	// --sync updates the module version"). Neither is changed here.
+	cmd := goGetCmd("", packagePath)
 	cmd.Stdout = os.Stdout
 	cmd.Stderr = os.Stderr
```

- [ ] **Step 4: Verify the suppression count went down**

```bash
cd /Users/joe/repos/personal/targ
grep -c 'nolint:gosec' internal/runner/runner.go
grep -n 'nolint:gosec' internal/runner/runner.go | grep -i 'go get'
```

Expected: exactly one `go get`-related suppression, inside `goGetCmd`.

- [ ] **Step 5: Verify `--sync` still behaves**

```bash
cd /Users/joe/repos/personal/targ
go build ./... && go vet ./internal/runner/
go test -tags=integration -run 'TestIntegration' ./internal/runner/ -count=1
targ reorder-decls-check
```

Expected: all pass. `fetchPackage`'s only behavioral inputs — its arg, its cwd, its streams, its
error wrapping — are unchanged by construction; read the diff and confirm that by eye.

- [ ] **Step 6: Commit**

```bash
cd /Users/joe/repos/personal/targ
git add internal/runner/runner.go
git commit -m "refactor(runner): build both go get commands through one constructor

The bootstrap get and --sync's fetch constructed the same exec by hand, each
carrying its own gosec suppression. goGetCmd owns the construction and the one
remaining suppression; the callers keep their streams and error handling, which
differ deliberately -- the bootstrap get is pinned and quiet, --sync is an
unversioned upgrade that streams progress.

Refs #41

AI-Used: [claude]"
```

---

## Task 5: delete the dead `EnsureFallbackModuleRoot`

**Files:**
- Modify: `internal/runner/runner.go:437-438…` (delete the function and its doc comment)

**Interfaces:**
- Consumes: nothing. Produces: nothing. This is a pure deletion.

**Why the deadcode gate misses it:** `deadcode -test ./...` does not report exported symbols in
`internal/` packages as unreachable, because an exported identifier is a plausible entry point. It
has zero callers anywhere, including tests — verified by grep across `*.go`.

- [ ] **Step 1: Re-verify zero callers before deleting**

```bash
cd /Users/joe/repos/personal/targ
grep -rn "EnsureFallbackModuleRoot" --include='*.go' .
```

Expected: exactly two hits, both in `internal/runner/runner.go` — the doc comment and the
definition. Any third hit means a caller exists; stop and do not delete.

- [ ] **Step 2: Delete the function and its doc comment**

Remove `// EnsureFallbackModuleRoot creates a fallback module root for isolated builds.` and the
entire `func EnsureFallbackModuleRoot(...)` body.

- [ ] **Step 3: Verify the build and the unused-helper fallout**

```bash
cd /Users/joe/repos/personal/targ
go build ./... && go vet ./internal/runner/ && targ --no-binary-cache lint-full
```

Expected: all pass. If lint now reports a newly-unused *unexported* helper that only
`EnsureFallbackModuleRoot` called, delete that too and re-run — `CLAUDE.md`: fix all instances, not
just the first.

- [ ] **Step 4: Commit**

```bash
cd /Users/joe/repos/personal/targ
git add internal/runner/runner.go
git commit -m "refactor(runner): delete EnsureFallbackModuleRoot, which has no callers

Zero references anywhere in the tree including tests. Exported from an internal
package, so 'deadcode -test ./...' treats it as a plausible entry point and never
flagged it.

Refs #41

AI-Used: [claude]"
```

---

## Task 6: documentation

**Files:**
- Modify: `README.md:706-708` (Multi-Module Targets)
- Modify: `CLAUDE.md:38` and `CLAUDE.md:40`

Dispositions come from the enumeration grep above; every other hit is `keep` or `N/A` with its
reason recorded there.

- [ ] **Step 1: Update `README.md`'s Multi-Module Targets section**

```markdown
 ### Multi-Module Targets

 Each ancestor with targ files is built as its own module group. Ancestors with a `go.mod` use normal module build; ancestors without one get an isolated build (synthetic `go.mod`).
+
+Building a module group never changes the version your `go.mod` pins targ at. Targ reads the
+version you already require and builds against exactly that. The one exception is genuine
+first-time integration — no targ require line at all — where targ adds one and says so on stderr,
+because the build itself runs under `-mod=readonly` and cannot. To move the pin, run
+`go get github.com/toejough/targ@<version>` yourself.
```

- [ ] **Step 2: Update `CLAUDE.md`'s IO-mocking rule**

```markdown
-- **No IO mocking.** Do not mock filesystem, network, or other IO in unit tests. If a test's sole purpose is verifying IO behavior, tag it as an integration test with `//go:build integration`.
+- **No IO mocking.** Do not mock filesystem, network, or other IO in unit tests. If a test's sole purpose is verifying IO behavior, tag it as an integration test with `//go:build integration` and name it `TestIntegration...` — `targ test-integration` selects tagged tests by that prefix, and `check-full` runs it. A tagged test named anything else never executes.
```

- [ ] **Step 3: Update `CLAUDE.md`'s check-full rule**

```markdown
-- **Always run `check-full` before declaring done.** Use `targ check-full`. This reports ALL failures at once (lint, coverage, ordering, dead code, nil checks).
+- **Always run `check-full` before declaring done.** Use `targ check-full`. This reports ALL failures at once (lint, coverage, ordering, dead code, nil checks, integration tests).
```

(Leave the rest of that bullet — the `check-for-fail` warning and the stale-lint-cache gotcha —
byte-identical.)

- [ ] **Step 4: Verify no other doc claims the old behavior**

```bash
cd /Users/joe/repos/personal/targ
grep -rn "go get -u" README.md docs/archive/architecture.md
```

Expected: `README.md:824` and `docs/archive/architecture.md:709`, both about `--sync`, both
correctly unchanged. Any *other* hit is a doc-surface miss — dispose of it before finishing.

- [ ] **Step 5: Commit**

```bash
cd /Users/joe/repos/personal/targ
git add README.md CLAUDE.md
git commit -m "docs: state that targ never moves a consumer's targ pin

Refs #41

AI-Used: [claude]"
```

---

## Task 7: full verification

- [ ] **Step 1: Confirm cwd and branch**

```bash
cd /Users/joe/repos/personal/targ
pwd && git branch --show-current && git status --short
```

Expected: `/Users/joe/repos/personal/targ`, `issue-41-gomod-pin-drift`, clean.

- [ ] **Step 2: Run the full gate**

```bash
cd /Users/joe/repos/personal/targ
targ --no-binary-cache check-full
```

Expected: all legs green, including the new `test-integration`. `check-full` reports every failure
rather than stopping at the first — read the whole report. If `lint-full` cites a path that no
longer exists, run `golangci-lint cache clean` and re-run before treating it as real.

- [ ] **Step 3: Verify the real binary, on a real fixture**

Tests passing is not the bar; the binary behaving is. Build targ from this branch and run it against
a two-group fixture whose pin is deliberately stale, per the issue's reproduction:

```bash
cd /Users/joe/repos/personal/targ
go build -o /tmp/targ-41/bin/targ ./cmd/targ
```

Then follow the issue's Reproduction section verbatim, substituting `/tmp/targ-41/bin/targ` for the
binary. Expected, versus the issue's recorded "before":

- the positive case's `go.mod` and `go.sum` are now **byte-identical** after `targ --help`
- the negative control is still byte-identical (unchanged behavior)
- `targ __complete "targ "` no longer moves the pin

Record the sha256 of `go.mod`/`go.sum` before and after each run. A pass here is the closure
evidence; a green suite alone is not.

- [ ] **Step 4: Confirm the working tree is clean**

```bash
cd /Users/joe/repos/personal/targ
git status --porcelain
```

Expected: empty. `go.mod`/`go.sum` appearing here would mean targ dirtied its own repo during
verification.

# Portable Targets - CLI UX Design

## Core Interaction Model

Portable targets use Go's import system as the foundation. Remote packages export target variables and register them via `init()`. Local code controls what's active using `targ.DeregisterFrom()` and selective `targ.Register()`.

### Three usage tiers

**Tier 1: Use everything (blank import)**
```go
import _ "github.com/toejough/go-targets"
```
All targets available at whatever organization the remote defined. Zero config.

**Tier 2: Reorganize (named import + deregister + re-register)**
```go
import targets "github.com/toejough/go-targets"

func init() {
    targ.DeregisterFrom("github.com/toejough/go-targets")
    targ.Register(
        targets.Lint,
        targets.Test.Name("unit-test"),
        targ.Group("ci", targets.Lint, targets.Test, targets.Coverage),
    )
}
```

**Tier 3: Multiple sources (resolve conflicts)**
```go
import (
    alice "github.com/alice/go-targets"
    _     "github.com/bob/go-targets"
)

func init() {
    targ.DeregisterFrom("github.com/alice/go-targets")
    targ.Register(alice.Test)
    // bob's targets remain as-is
}
```

## API Surface

### New function: `targ.DeregisterFrom(packagePath string)`

Removes all targets that were registered by the named package's `init()` function.

**Behavior:**
- Removes all targets whose defining function lives in the named package
- Errors if no targets were registered from that package (catches typos, stale imports)
- Must be called from `init()` (before targ's build phase)
- Can be called multiple times for different packages

**Error on no match:**
```
targ: DeregisterFrom("github.com/typo/wrong"): no targets registered from this package
```

### Changed behavior: Deferred conflict detection

Currently, `targ.Register()` may detect duplicate names immediately. The new behavior:

- `targ.Register()` queues registrations during `init()` phase
- After all `init()` functions have run (at build time), targ resolves the queue
- Conflicts are detected at build time, not registration time
- This gives users a chance to call `DeregisterFrom()` before conflicts are checked

**Conflict error (actionable):**
```
targ: conflict: "lint" registered by both:
  - github.com/alice/go-targets
  - github.com/bob/go-targets
Use targ.DeregisterFrom() to resolve.
```

### Existing API: Builder methods for customization

These already exist and work on synced targets the same as local ones:

```go
targets.Test.Name("unit-test")           // Rename
targets.Test.Description("Run tests")    // Override description
targ.Group("ci", targets.Lint, ...)      // Reorganize into groups
```

## Sync Command UX

### `targ --sync <package-path>` (existing, unchanged)

Adds a blank import to the local targ file. This is the "Tier 1" entry point.

```
$ targ --sync github.com/toejough/go-targets
A dev/targets.go  (added import)
```

### Re-sync / Update

Not a targ command. Use Go's module system:

```
$ go get -u github.com/toejough/go-targets
```

This updates `go.mod`/`go.sum`. Next `targ` invocation picks up new targets automatically since they're code imports.

### `targ --list` or `targ` (existing, enhanced)

Shows all registered targets with source attribution:

```
$ targ
Targets:
  lint           Lint codebase              (github.com/toejough/go-targets)
  unit-test      Run tests                  (github.com/toejough/go-targets, renamed)
  coverage       Check coverage             (github.com/toejough/go-targets)
  check          Run all checks             (local)
  my-target      Custom target              (local)

Groups:
  ci             lint, unit-test, coverage   (local)
```

Source attribution shows:
- Package path for synced targets
- `(local)` for targets defined in the project
- `(renamed)` annotation when `.Name()` was used on a synced target

## Error Messages

All errors are terse, actionable, and name the fix.

### Conflict: duplicate target names
```
targ: conflict: "lint" registered by both:
  - github.com/alice/go-targets
  - github.com/bob/go-targets
Use targ.DeregisterFrom() to resolve.
```

### DeregisterFrom: no match
```
targ: DeregisterFrom("github.com/typo/wrong"): no targets registered from this package
```

### DeregisterFrom: called after build phase
```
targ: DeregisterFrom() must be called during init(), not after targ has started
```

## Configuration for Synced Targets

### Environment variables for parameterized targets

Remote targets that need project-specific values use targ's existing env var support via struct tags:

```go
// In remote package
type TestArgs struct {
    CoverageThreshold float64 `targ:"flag,env=COVERAGE_THRESHOLD,default=80"`
    CoverPkg          string  `targ:"flag,env=COVER_PKG,default=./..."`
}

var Test = targ.Targ(test).Description("Run tests")

func test(args TestArgs) error {
    // Uses args.CoverageThreshold, which comes from:
    // 1. CLI flag --coverage-threshold
    // 2. Env var COVERAGE_THRESHOLD
    // 3. Default: 80
}
```

```
# Local .env or shell environment
COVERAGE_THRESHOLD=90
COVER_PKG=./internal/...
```

This requires no new mechanism. Targ already supports env vars for arguments.

### Config files (out of scope for now)

Golangci configs and similar files are not handled by sync. They remain copy-paste-and-adapt. This is acceptable because:
- Getting the targets synced is the higher-value problem
- Config files change less frequently than target code
- A future iteration can address config templating

## User Flows

### Flow 1: New project, sync standard targets

```
$ mkdir myproject && cd myproject
$ go mod init github.com/me/myproject
$ targ --sync github.com/toejough/go-targets
A dev/targets.go  (added import)

$ targ
Targets:
  lint       Lint codebase       (github.com/toejough/go-targets)
  fmt        Format code         (github.com/toejough/go-targets)
  test       Run tests           (github.com/toejough/go-targets)
  ...

$ targ test
Running tests...
```

### Flow 2: Customize synced targets

Edit `dev/targets.go`:
```go
import targets "github.com/toejough/go-targets"

func init() {
    targ.DeregisterFrom("github.com/toejough/go-targets")
    targ.Register(
        targets.Lint,
        targets.Fmt,
        targets.Test.Name("unit-test"),
        targ.Group("ci", targets.Lint, targets.Fmt, targets.Test),
    )
    targ.Register(Check, Deploy)
}
```

```
$ targ
Targets:
  lint         Lint codebase     (github.com/toejough/go-targets)
  fmt          Format code       (github.com/toejough/go-targets)
  unit-test    Run tests         (github.com/toejough/go-targets, renamed)
  check        Run all checks    (local)
  deploy       Deploy app        (local)

Groups:
  ci           lint, fmt, unit-test  (local)
```

### Flow 3: Pull from multiple sources

```go
import (
    goTargets  "github.com/toejough/go-targets"
    k8sTargets "github.com/company/k8s-targets"
)

func init() {
    // Use all of go-targets
    // Cherry-pick from k8s-targets
    targ.DeregisterFrom("github.com/company/k8s-targets")
    targ.Register(k8sTargets.Deploy, k8sTargets.Rollback)
}
```

### Flow 4: Update synced targets

```
$ go get -u github.com/toejough/go-targets
go: upgraded github.com/toejough/go-targets v0.3.1 => v0.4.0

$ targ test
Running tests...  (now uses v0.4.0 improvements)
```

### Flow 5: Debug "where did this come from?"

```
$ targ --list
lint           Lint codebase              github.com/toejough/go-targets
unit-test      Run tests                  github.com/toejough/go-targets (renamed from "test")
deploy         Deploy to k8s              github.com/company/k8s-targets
check          Run all checks             local (dev/targets.go)
```

### Flow 6: Resolve conflict

```
$ targ
targ: conflict: "lint" registered by both:
  - github.com/alice/go-targets
  - github.com/bob/go-targets
Use targ.DeregisterFrom() to resolve.
```

User edits `dev/targets.go`:
```go
func init() {
    targ.DeregisterFrom("github.com/bob/go-targets")
    targ.Register(bobTargets.Deploy) // just want deploy from bob
}
```

```
$ targ
Targets:
  lint       ...    (github.com/alice/go-targets)
  deploy     ...    (github.com/bob/go-targets)
  ...
```

## Design Principles

1. **Go code is the config.** No new file formats. Imports, function calls, builder methods.
2. **Explicit over implicit.** `DeregisterFrom` names the package. No magic inference.
3. **Errors name the fix.** Every error message tells you what to do.
4. **Deferred detection.** Give users a chance to resolve conflicts in their own `init()`.
5. **Git for versioning.** `go.mod` tracks versions. `go get -u` updates. No reinvention.
6. **Tier 1 is zero config.** Blank import works. Customization is opt-in.

## Out of Scope

- **Config file sync** (golangci TOML, etc.) - future iteration
- **"All except N" shorthand** - user deregisters and re-registers; no exclude syntax needed
- **Version pinning beyond go.mod** - Go modules handle this
- **GUI/TUI for sync management** - CLI output only

# Architecture

How targ implements the requirements.

## Overview

A target has four configurable aspects (**Anatomy**), and targ provides eight operations on targets.

### Target Anatomy

| Aspect    | What it defines                | Defined by         |
| --------- | ------------------------------ | ------------------ |
| Arguments | CLI flags and positionals      | [Args struct](#arguments) |
| Execution | How target runs (deps, retry)  | [Target Builder](#execution) |
| Hierarchy | Where target appears in CLI    | [Group](#hierarchy) |
| Source    | Which file contains the code   | [Function](#source) |

### Operations × Anatomy

✓ = specified, Gap = needs design

|           | Arguments              | Execution                | Hierarchy                | Source                   |
| --------- | ---------------------- | ------------------------ | ------------------------ | ------------------------ |
| Discover  | [✓](#arguments)        | [✓](#execution)          | [✓](#hierarchy)          | [✓](#source)             |
| Inspect   | [✓](#inspect)          | [✓](#inspect)            | [✓](#inspect)            | [✓](#inspect)            |
| Specify   | [✓](#arguments)        | [✓](#execution)          | [✓](#hierarchy)          | [✓](#source)             |
| Run       | [✓](#run)              | [✓](#run)                | [✓](#run)                | [✓](#source)             |
| Create    | [✓](#create)           | [✓](#create)             | [✓](#create)             | [✓](#create)             |
| Sync      | [✓](#sync)             | [✓](#sync)               | [✓](#sync)               | [✓](#sync)               |

### Constraints

Cross-cutting concerns that apply to all operations.

**Invariants** - What must hold true:

| Property            | Create              | Sync                              |
| ------------------- | ------------------- | --------------------------------- |
| Existing signatures | Unaffected          | Only imported targets affected    |
| Help text refs      | Unaffected          | Remain valid                      |
| CLI invocation      | Unaffected          | Only imported targets affected    |
| Dependencies        | Unaffected          | Remain valid                      |
| Direct call sites   | Unaffected          | Remain valid                      |
| Behavior            | Unaffected          | Only imported targets affected    |

Create with `--deps` validates that referenced targets exist.
Sync errors on naming conflicts with existing hierarchy.

**Reversible** - All operations reversible through the command surface:
- Create → delete generated code
- Sync → remove import and registrations

**Minimal changes** - Prefer minimal code changes; add to existing file rather than creating new ones.

**Fail clearly** - If invariants cannot be maintained, fail with a clear error message.

### Global Flags

Flags on `targ` itself (must appear before target path):

| Flag         | Short | Description                          |
| ------------ | ----- | ------------------------------------ |
| --parallel   | -p    | Run multiple targets in parallel     |
| --completion |       | Print shell completion script        |
| --source     | -s    | Specify source (local or remote)     |
| --create     | -c    | Scaffold new target from command     |
| --to-func    |       | Convert string target to function    |
| --to-string  |       | Convert function target to string    |
| --sync       |       | Import targets from remote repo      |

`--source` infers local vs remote from format:
- `./path` or `/path` → local file
- `github.com/...` → remote

`--to-func` expands a string target to a full function with args struct.
`--to-string` errors if the function does more than a basic `targ.Shell()` call.

`--help` is universal (works on any target or group).

---

## Requirements Traceability

Maps requirements to architecture. Coverage: Necessary (inherent), Needs design, or section link (already covered).

### Model: Targets

| Requirement     | Architecture                         | Coverage                |
| --------------- | ------------------------------------ | ----------------------- |
| Basic           | Source - `func Name()`               | [Source](#source)       |
| Failure         | Source - `func Name() error`         | [Source](#source)       |
| Cancellation    | Source - `func Name(ctx) error`      | [Source](#source)       |
| Dependencies    | Execution - `.Deps()`                | [Execution](#execution) |
| Parallel        | Execution - `.ParallelDeps()`        | [Execution](#execution) |
| Serial          | Execution - `.Deps()` (default)      | [Execution](#execution) |
| Help text       | Execution - `.Description()`         | [Execution](#execution) |
| Arguments       | Arguments - struct parameter         | [Arguments](#arguments) |
| Repeated args   | Arguments - `[]T` field              | [Arguments](#arguments) |
| Map args        | Arguments - `map[K]V` field          | [Arguments](#arguments) |
| Variadic args   | Arguments - trailing `[]T`           | [Arguments](#arguments) |
| Subcommands     | Hierarchy - `targ.Group()`           | [Hierarchy](#hierarchy) |
| Result caching  | Execution - `.Cache()`               | [Execution](#execution) |
| Watch mode      | Execution - `.Watch()`               | [Execution](#execution) |
| Retry + backoff | Execution - `.Retry()`, `.Backoff()` | [Execution](#execution) |
| Repetition      | Execution - `.Times()`               | [Execution](#execution) |
| Time-bounded    | Execution - `.For()`                 | [Execution](#execution) |
| Condition-based | Execution - `.While()`               | [Execution](#execution) |

### Model: Hierarchy

| Requirement                      | Architecture               | Coverage                |
| -------------------------------- | -------------------------- | ----------------------- |
| Namespace nodes (non-executable) | Hierarchy - Groups         | [Hierarchy](#hierarchy) |
| Target nodes (executable)        | Source - functions only    | [Source](#source)       |
| Path addressing                  | Specify × Hierarchy        | [Hierarchy](#hierarchy) |
| Simplest possible definition     | Source - `//go:build targ` | [Source](#source)       |
| Scales to complex hierarchies    | Hierarchy - nested Groups  | [Hierarchy](#hierarchy) |
| Easy CLI binary transition       | Source                     | [Source](#cli-binary-transition) |

### Model: Sources

| Requirement    | Architecture               | Coverage          |
| -------------- | -------------------------- | ----------------- |
| Local targets  | Source - `//go:build targ` | [Source](#source) |
| Remote targets | Sync × Source              | [Sync](#sync)     |

### Operations

| Requirement          | Architecture         | Coverage     |
| -------------------- | -------------------- | ------------ |
| Create (scaffold)    | Create × Source      | [Create](#create) |
| Invoke: CLI          | Run × all aspects    | [Run](#run) |
| Invoke: modifiers    | Run × Execution      | [Run](#run) |
| Invoke: programmatic | Run × all aspects    | [Execution](#programmatic-invocation) |
| Transform: Rename    | Edit source          | Necessary    |
| Transform: Relocate  | Edit source          | Necessary    |
| Transform: Delete    | Edit source          | Necessary    |
| Manage Dependencies  | Edit source          | Necessary    |
| Sync (remote)        | Sync × Source        | [Sync](#sync) |
| Inspect: Where       | Inspect × Source     | [Inspect](#inspect) |
| Inspect: Tree        | Inspect × Hierarchy  | [Inspect](#inspect) |
| Inspect: Deps        | Inspect × Execution  | [Inspect](#inspect) |
| Shell Integration    | Run × Arguments      | Needs design |

### Constraints

| Requirement           | Architecture | Coverage                    |
| --------------------- | ------------ | --------------------------- |
| Invariants maintained | Constraints  | [Constraints](#constraints) |
| Reversible operations | Constraints  | [Constraints](#constraints) |
| Minimal changes       | Constraints  | [Constraints](#constraints) |
| Fail clearly          | Constraints  | [Constraints](#constraints) |

---

# Target Anatomy

## Arguments

What CLI inputs the target accepts. Defined by a struct parameter with tags.

```go
type DeployArgs struct {
    Env      string            `targ:"flag,required,desc=Target environment"`
    DryRun   bool              `targ:"flag,short=n,desc=Preview mode"`
    Services []string          `targ:"positional,desc=Services to deploy"`
    Labels   map[string]string `targ:"flag,desc=Labels to apply"`
}

func Deploy(ctx context.Context, args DeployArgs) error { ... }
```

| Field type        | Behavior                              |
| ----------------- | ------------------------------------- |
| `T`               | Single value                          |
| `[]T`             | Repeated (accumulates)                |
| `map[K]V`         | Key=value pairs                       |
| Trailing `[]T`    | Variadic positional (captures rest)   |

### Ordered arguments

Arguments preserve their CLI ordering across flags and positionals. Useful for filter chains where order matters:

```
targ find --include "*.go" --exclude "vendor/*" --include "*.mod"
```

```go
type FindArgs struct {
    Include []targ.Interleaved[string] `targ:"flag,desc=Patterns to include"`
    Exclude []targ.Interleaved[string] `targ:"flag,desc=Patterns to exclude"`
}
```

Each `Interleaved[T]` contains `Value T` and `Position int`. Merge and sort by position to reconstruct CLI order.

targ reflects on the function signature, finds the args struct, reads its tags.

## Execution

How the target runs. Defined by the Target Builder.

```go
targ.Targ(fn)                     // wrap a function
targ.Targ("cmd $arg ...")         // shell command (runs in calling shell)

    .Deps(targets...)             // dependencies (serial by default)
    .DepMode(targ.Parallel)       // run deps in parallel
    .Cache(patterns...)           // skip if inputs unchanged
    .Watch(patterns...)           // file patterns that trigger re-run
    .Retry(n)                     // retry on failure
    .Backoff(initial, multiplier) // exponential delay between retries
    .Times(n)                     // run N times
    .Timeout(duration)            // cancel after duration
    .While(func() bool)           // run while predicate true
    .Name(s)                      // override CLI name
    .Description(s)               // help text
```

`.Deps()` accepts raw functions and `*Target`. `.DepMode()` takes `targ.Serial` (default) or `targ.Parallel`.

Shell command strings run in the calling shell (bash, fish, zsh, etc.) so aliases and functions work.

For function targets that need to shell out:

```go
func Deploy(ctx context.Context, args DeployArgs) error {
    return targ.Shell(ctx, "kubectl apply -n $namespace -f $file", args)
}
```

`targ.Shell(ctx, cmd, args)` substitutes `$var` from struct fields and runs in the calling shell.

### Discovery

Execution metadata is discovered when targets are registered via `targ.Run()` in an init function. See [Source](#source).

### Example

```go
// Function-based targets
var format = targ.Targ(Format)
var build = targ.Targ(Build).Deps(format)
var deploy = targ.Targ(Deploy).Deps(build)

// Shell command targets (infers --path/-p flag from $path)
var lint = targ.Targ("golangci-lint run $path").Deps(format).Cache("**/*.go")
var test = targ.Targ("go test $pkg").Deps(build)
```

### Programmatic Invocation

Call a target from Go code with full execution config (deps, cache, retry, etc.):

```go
err := build.Run(ctx)
err := deploy.Run(ctx, DeployArgs{Env: "prod"})
```

Calling the raw function (`Build(ctx)`) skips all execution config. Use `.Run()` to invoke the full target.

Dependencies run exactly once per execution context. If multiple targets share a dep, it runs once.

### Runtime Overrides

Users can override execution settings via CLI flags:

```
targ build --watch "**/*.go"
targ build --cache "**/*.go,go.sum"
targ build --timeout 5m
targ build --retry 3 --backoff 1s,2
targ build --no-cache
targ build --deps lint,test
targ build --deps lint,test --dep-mode parallel
```

**Ownership model**:

- **targ manages by default**: `--watch`, `--cache`, `--timeout`, `--retry`, `--backoff`, `--deps`, `--dep-mode` are reserved flags
- **Conflict = error**: If your args struct defines a field that conflicts with a targ-managed flag, targ errors
- **targ.Disabled = you take over**: Disable targ's management, define the flag yourself, use targ APIs

```go
// Disable targ's --watch management
var build = targ.Targ(Build).Watch(targ.Disabled)

type BuildArgs struct {
    Watch []string `targ:"flag,desc=Patterns to watch"`
}

func Build(ctx context.Context, args BuildArgs) error {
    if len(args.Watch) > 0 {
        return targ.WatchAndRun(args.Watch, func() error {
            // build logic
        })
    }
    // ...
}
```

## Hierarchy

Where the target appears in the CLI namespace. Defined by Group membership.

```go
func Group(name string, members ...any) *Group
```

Members can be raw functions, `*Target`, or nested `*Group`.

```go
var Lint = targ.Group("lint", lintFast, lintFull)
var Test = targ.Group("test", testUnit, testIntegration)
var Dev = targ.Group("dev", format, build, Lint, Test, deploy)
```

Results in:
```
targ dev format
targ dev build
targ dev lint fast
targ dev lint full
targ dev test unit
targ dev deploy --env prod
```

Groups are non-executable (pure namespace). Functions are the only executable targets.

### Discovery

Hierarchy is discovered when groups are registered via `targ.Run()` in an init function. See [Source](#source).

### Path Specification

Stack-based traversal with glob support. Same syntax for all operations (run, inspect, modify, delete).

```
targ dev build test          # dev/build, dev/test
targ dev lint fast full      # dev/lint/fast, dev/lint/full
targ dev build ^ prod deploy # dev/build, then prod/deploy
targ dev lint *              # all targets under dev/lint
targ dev **                  # all targets under dev, recursively
targ ** test                 # all targets named "test" anywhere
```

- Names push onto the stack (descend into groups, or select targets at current level)
- `^` resets the stack to root
- `*` matches any single level
- `**` matches any depth (fish-style)

### Why explicit names?

Group names must be explicit because:
1. Go reflection cannot retrieve variable names
2. Nested groups are values - no way to derive "lint" from `lintGroup`

## Source

Which file contains the implementation. The function's location in the codebase.

Functions are discovered in files with `//go:build targ` tag. Execution metadata and hierarchy are registered via `targ.Run()` in an init function:

```go
//go:build targ

package dev

func Build(ctx context.Context) error { ... }
func Test() error { ... }

var build = targ.Targ(Build).Cache("**/*.go")
var test = targ.Targ(Test)
var Dev = targ.Group("dev", build, test)

func init() {
    targ.Run(Dev)
}
```

`targ.Run()` accepts raw functions, `*Target`, or `*Group`.

Function signature capabilities (independently optional):

| Element                | Capability   | Without it                    |
| ---------------------- | ------------ | ----------------------------- |
| `error` return         | Failure      | Always succeeds               |
| `ctx context.Context`  | Cancellation | Can't respond to timeout/interrupt |
| `args T` parameter     | Arguments    | No CLI flags/positionals      |

Examples: `func Name()`, `func Name(args T) error`, `func Name(ctx context.Context) error`

### Source Resolution

How targ finds and builds sources (applies to both local and remote):

**Explicit specification**:
```
targ --source ./dev/targs.go build   # local
targ --source github.com/foo/bar build # remote
targ -s ./dev/targs.go build         # short form
```

**Default local discovery**:
1. Recursive search down from cwd
2. Stop at first level containing a targ file (`//go:build targ`)
3. Multiple targ files at same level → error (user must `--file` or cd to resolve)

**Module resolution**:
1. Search up from targ file toward repo root
2. Use first `go.mod` found
3. No `go.mod` → create temporary module in temp build dir

### CLI Binary Transition

To convert targ targets to a standalone CLI binary:

1. Remove `//go:build targ`
2. Change `package dev` to `package main`
3. Rename `func init()` to `func main()`

The `targ.Run()` call becomes the binary's entry point.

---

# Inspect

Running a group with no arguments (or `--help`) prints its tree with descriptions:

```
targ dev

dev
  format    Format source code
  build     Build the binary
  lint
    fast    Quick lint checks
    full    Comprehensive lint
  deploy    Deploy to environment
```

Running `--help` on a target shows all aspects:

```
targ dev deploy --help

deploy - Deploy to environment

Source: dev/deploy.go:42

Arguments:
  --env       (required)  Target environment
  --dry-run, -n           Preview mode
  <services>              Services to deploy

Execution:
  Deps: build, lint-full
  Cache: **/*.go
  Retry: 3 (backoff: 1s × 2)
```

---

# Run

## Arguments

Parse CLI flags and positionals into the args struct.

**Supported types**:
- Builtins: `string`, `int`, `bool`, `float64`, `time.Duration`, etc.
- Custom: Any type implementing `encoding.TextUnmarshaler` or `Set(string) error`
- Unsupported type → error at discovery (not runtime)

**Validation**:
- Required fields must be provided
- Env var fallback if `env=VAR` tag present
- Default value if `default=X` tag present

## Execution

Order of operations:

1. **Deps**: Run dependencies (serial or parallel per `.Deps()`/`.ParallelDeps()`)
2. **Cache check**: Skip if cached and inputs unchanged
3. **Function**: Invoke the target function
4. **Retry**: On failure, retry with backoff if configured

**Multiple targets**:
```
targ build test            # sequential (default)
targ --parallel build test # parallel
targ -p build test         # parallel (short form)
```
- Shared dep state within invocation (dep runs once even if multiple targets need it)

**Watch mode**:
- Re-run full dep chain on file change
- Cancel in-progress run, restart from deps

**Cache**:
- Persistent across invocations (file-based checksums)
- `--no-cache` bypasses

## Hierarchy

Resolve target path using stack traversal (see [Path Specification](#path-specification)).

- Group with no target → show tree (see [Inspect](#inspect))
- Target found → execute
- No match → error with suggestions

---

# Create

Scaffold new targets from shell commands.

## Source

Created in discovered targ file, or `./targs.go` if none exists.

## Hierarchy

Path before command becomes the target location:

```
targ --create lint "golangci-lint run"           # creates: lint
targ --create dev lint fast "golangci-lint run"  # creates: dev/lint/fast
```

Creates groups as needed.

## Arguments

Inferred from `$var` placeholders in the command:

```
targ --create deploy "kubectl apply -n $namespace -f $file"
```

Generates:
```go
type DeployArgs struct {
    Namespace string `targ:"flag,short=n,desc=namespace"`
    File      string `targ:"flag,short=f,desc=file"`
}
```

- All inferred args are `string` type, flags with `--name -n` form
- Short flag from first letter; collisions skip short for later args
- Edit generated code to change types, add descriptions, mark required

## Execution

Execution settings via flags:

```
targ --create --cache "**/*.go" lint "golangci-lint run"
targ --create --deps build,test deploy "kubectl apply"
targ --create --deps build,test --dep-mode parallel deploy "kubectl apply"
targ --create --retry 3 --backoff 1s,2 flaky "curl ..."
targ --create --timeout 5m slow "long-running-cmd"
targ --create --watch "**/*.go" dev "go build"
```

---

# Sync

Import targets from remote repositories.

```
targ --sync github.com/foo/bar
```

**Behavior**:
- **No targ file exists**: Create one with import and register all exported targets/groups
- **Targ file exists, no import**: Add import, register exported targets/groups
- **Targ file exists, has import**: Update module version (`go get -u` style)

**Generated code**:
```go
//go:build targ

package main

import "github.com/foo/bar"

func init() {
    targ.Run(bar.Build, bar.Test, bar.Deploy)
}
```

**Imports**: Any exported targets (`*Target`), groups (`*Group`), or functions.

**Naming conflicts**: Error clearly if any imported names would conflict with existing hierarchy.


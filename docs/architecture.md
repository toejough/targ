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
| Modify    | Gap                    | Gap                      | Gap                      | Gap                      |
| Specify   | Gap                    | Gap                      | Gap                      | Gap                      |
| Run       | Gap                    | Gap                      | Gap                      | Gap                      |
| Create    | Gap                    | Gap                      | Gap                      | Gap                      |
| Delete    | Gap                    | Gap                      | Gap                      | Gap                      |
| Sync      | Gap                    | Gap                      | Gap                      | Gap                      |

### Constraints

Cross-cutting concerns that apply to all operations:

| Constraint            | Status |
| --------------------- | ------ |
| Invariants maintained | Gap    |
| Reversible operations | Gap    |
| Minimal changes       | Gap    |
| Fail clearly          | Gap    |

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
| Path addressing                  | Specify × Hierarchy        | Needs design            |
| Simplest possible definition     | Source - `//go:build targ` | [Source](#source)       |
| Scales to complex hierarchies    | Hierarchy - nested Groups  | [Hierarchy](#hierarchy) |
| Easy CLI binary transition       | Run × Hierarchy            | Needs design            |

### Model: Sources

| Requirement    | Architecture               | Coverage          |
| -------------- | -------------------------- | ----------------- |
| Local targets  | Source - `//go:build targ` | [Source](#source) |
| Remote targets | Sync × Source              | Needs design      |

### Operations

| Requirement          | Architecture         | Coverage     |
| -------------------- | -------------------- | ------------ |
| Create (scaffold)    | Create × Source      | Needs design |
| Invoke: CLI          | Run × all aspects    | Needs design |
| Invoke: modifiers    | Run × Execution      | Needs design |
| Invoke: programmatic | Run × all aspects    | Needs design |
| Transform: Rename    | Modify × Hierarchy   | Needs design |
| Transform: Relocate  | Modify × Source      | Needs design |
| Transform: Delete    | Delete × all aspects | Needs design |
| Manage Dependencies  | Modify × Execution   | Needs design |
| Sync (remote)        | Sync × Source        | Needs design |
| Inspect: Where       | Inspect × Source     | [Inspect](#inspect) |
| Inspect: Tree        | Inspect × Hierarchy  | [Inspect](#inspect) |
| Inspect: Deps        | Inspect × Execution  | [Inspect](#inspect) |
| Shell Integration    | Run × Arguments      | Needs design |

### Constraints

| Requirement           | Architecture | Coverage     |
| --------------------- | ------------ | ------------ |
| Invariants maintained | Constraints  | Needs design |
| Reversible operations | Constraints  | Needs design |
| Minimal changes       | Constraints  | Needs design |
| Fail clearly          | Constraints  | Needs design |

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

targ reflects on the function signature, finds the args struct, reads its tags.

## Execution

How the target runs. Defined by the Target Builder.

```go
targ.Targ(fn)                     // wrap a function
    .Deps(targets...)             // serial dependencies
    .ParallelDeps(targets...)     // parallel dependencies
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

`.Deps()` and `.ParallelDeps()` accept both raw functions and `*Target`.

### Discovery

Execution metadata is discovered when targets are registered via `targ.Run()` in an init function. See [Source](#source).

### Example

```go
var format = targ.Targ(Format)
var build = targ.Targ(Build).Deps(format)
var lintFast = targ.Targ(LintFast).ParallelDeps(format, build).Cache("**/*.go")
var lintFull = targ.Targ(LintFull).Deps(lintFast)
var deploy = targ.Targ(Deploy).Deps(build, lintFull)
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


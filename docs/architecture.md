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

✓ = specified, Gap = needs design, - = not applicable

|           | Arguments              | Execution                | Hierarchy                | Source                   |
| --------- | ---------------------- | ------------------------ | ------------------------ | ------------------------ |
| Discover  | [✓](#arguments)        | Gap                      | Gap                      | [✓](#source)             |
| Inspect   | Gap                    | Gap                      | Gap                      | Gap                      |
| Modify    | -                      | Gap                      | Gap                      | Gap                      |
| Specify   | -                      | -                        | Gap                      | Gap                      |
| Run       | Gap                    | Gap                      | Gap                      | -                        |
| Create    | -                      | Gap                      | Gap                      | Gap                      |
| Delete    | Gap                    | Gap                      | Gap                      | Gap                      |
| Sync      | -                      | Gap                      | Gap                      | Gap                      |

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

Maps requirements to architecture overview. See Overview for ✓/Gap status.

### Model: Targets

| Requirement     | Architecture                         |
| --------------- | ------------------------------------ |
| Basic           | Source - `func Name()`               |
| Failure         | Source - `func Name() error`         |
| Cancellation    | Source - `func Name(ctx) error`      |
| Dependencies    | Execution - `.Deps()`                |
| Parallel        | Execution - `.ParallelDeps()`        |
| Serial          | Execution - `.Deps()` (default)      |
| Help text       | Execution - `.Description()`         |
| Arguments       | Arguments - struct parameter         |
| Repeated args   | Arguments - `[]T` field              |
| Map args        | Arguments - `map[K]V` field          |
| Variadic args   | Arguments - trailing `[]T`           |
| Subcommands     | Hierarchy - `targ.Group()`           |
| Result caching  | Execution - `.Cache()`               |
| Watch mode      | Execution - `.Watch()`               |
| Retry + backoff | Execution - `.Retry()`, `.Backoff()` |
| Repetition      | Execution - `.Times()`               |
| Time-bounded    | Execution - `.For()`                 |
| Condition-based | Execution - `.While()`               |

### Model: Hierarchy

| Requirement                      | Architecture               |
| -------------------------------- | -------------------------- |
| Namespace nodes (non-executable) | Hierarchy - Groups         |
| Target nodes (executable)        | Source - functions only    |
| Path addressing                  | Specify × Hierarchy        |
| Simplest possible definition     | Source - `//go:build targ` |
| Scales to complex hierarchies    | Hierarchy - nested Groups  |
| Easy CLI binary transition       | Run × Hierarchy            |

### Model: Sources

| Requirement    | Architecture               |
| -------------- | -------------------------- |
| Local targets  | Source - `//go:build targ` |
| Remote targets | Sync × Source              |

### Operations

| Requirement          | Architecture         |
| -------------------- | -------------------- |
| Create (scaffold)    | Create × Source      |
| Invoke: CLI          | Run × all aspects    |
| Invoke: modifiers    | Run × Execution      |
| Invoke: programmatic | Run × all aspects    |
| Transform: Rename    | Modify × Hierarchy   |
| Transform: Relocate  | Modify × Source      |
| Transform: Delete    | Delete × all aspects |
| Manage Dependencies  | Modify × Execution   |
| Sync (remote)        | Sync × Source        |
| Inspect: Where       | Inspect × Source     |
| Inspect: Tree        | Inspect × Hierarchy  |
| Inspect: Deps        | Inspect × Execution  |
| Shell Integration    | Run × Arguments      |

### Constraints

| Requirement           | Architecture |
| --------------------- | ------------ |
| Invariants maintained | Constraints  |
| Reversible operations | Constraints  |
| Minimal changes       | Constraints  |
| Fail clearly          | Constraints  |

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
    .For(duration)                // run until duration elapsed
    .While(func() bool)           // run while predicate true
    .Name(s)                      // override CLI name
    .Description(s)               // help text
```

`.Deps()` and `.ParallelDeps()` accept both raw functions and `*Target`.

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

### Why explicit names?

Group names must be explicit because:
1. Go reflection cannot retrieve variable names
2. Nested groups are values - no way to derive "lint" from `lintGroup`

## Source

Which file contains the implementation. The function's location in the codebase.

Functions are discovered in files with `//go:build targ` tag:

```go
//go:build targ

package dev

func Build(ctx context.Context) error { ... }
```

Progressive function signatures:

| Signature                        | Capabilities           |
| -------------------------------- | ---------------------- |
| `func Name()`                    | Basic                  |
| `func Name() error`              | + Failure              |
| `func Name(ctx context.Context) error` | + Cancellation   |
| `func Name(ctx context.Context, args T) error` | + Arguments |


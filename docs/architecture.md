# Architecture

How targ implements the requirements.

## Overview

A target has four configurable aspects (**Anatomy**), and targ provides five categories of operations on targets (**Operations**).

### Target Anatomy

| Aspect    | What it defines                | Defined by                |
| --------- | ------------------------------ | ------------------------- |
| Arguments | CLI flags and positionals      | Args struct + tags        |
| Execution | How target runs (deps, retry)  | Target Builder            |
| Hierarchy | Where target appears in CLI    | Group membership          |
| Source    | Which file contains the code   | File location             |

### Operations

| Operation | What it does                        | User-facing            |
| --------- | ----------------------------------- | ---------------------- |
| Discover  | targ finds targets in codebase      | (automatic)            |
| Inspect   | Query information about targets     | --help, --tree, --where, --deps |
| Modify    | Change target aspects via CLI       | --rename, --relocate, --delete |
| Specify   | Reference targets by path/pattern   | dotted paths, globs    |
| Run       | Execute specified targets           | `targ <path> [args]`   |

---

## Requirements Traceability

Maps requirements to architecture. **Gap** = not yet addressed.

### Model: Targets

| Requirement     | Status | Architecture                                       |
| --------------- | ------ | -------------------------------------------------- |
| Basic           | ✓      | [Source](#source) - `func Name()`                  |
| Failure         | ✓      | [Source](#source) - `func Name() error`            |
| Cancellation    | ✓      | [Source](#source) - `func Name(ctx) error`         |
| Dependencies    | ✓      | [Execution](#execution) - `.Deps()`                |
| Parallel        | ✓      | [Execution](#execution) - `.ParallelDeps()`        |
| Serial          | ✓      | [Execution](#execution) - `.Deps()` (default)      |
| Help text       | ✓      | [Execution](#execution) - `.Description()`         |
| Arguments       | ✓      | [Arguments](#arguments) - struct parameter         |
| Repeated args   | ✓      | [Arguments](#arguments) - `[]T` field              |
| Map args        | ✓      | [Arguments](#arguments) - `map[K]V` field          |
| Variadic args   | ✓      | [Arguments](#arguments) - trailing `[]T`           |
| Subcommands     | ✓      | [Hierarchy](#hierarchy) - `targ.Group()`           |
| Result caching  | ✓      | [Execution](#execution) - `.Cache()`               |
| Watch mode      | ✓      | [Execution](#execution) - `.Watch()`               |
| Retry + backoff | ✓      | [Execution](#execution) - `.Retry()`, `.Backoff()` |
| Repetition      | ✓      | [Execution](#execution) - `.Times()`               |
| Time-bounded    | ✓      | [Execution](#execution) - `.For()`                 |
| Condition-based | ✓      | [Execution](#execution) - `.While()`               |

### Model: Hierarchy

| Requirement                      | Status | Architecture                            |
| -------------------------------- | ------ | --------------------------------------- |
| Namespace nodes (non-executable) | ✓      | [Hierarchy](#hierarchy) - Groups        |
| Target nodes (executable)        | ✓      | [Source](#source) - functions only      |
| Path addressing                  | Gap    | [Specify](#specify)                     |
| Simplest possible definition     | ✓      | [Discover](#discover) - auto-discovery  |
| Scales to complex hierarchies    | ✓      | [Hierarchy](#hierarchy) - nested Groups |
| Easy CLI binary transition       | Gap    |                                         |

### Model: Sources

| Requirement    | Status | Architecture                   |
| -------------- | ------ | ------------------------------ |
| Local targets  | ✓      | [Discover](#discover)          |
| Remote targets | Gap    |                                |

### Operations

| Requirement          | Status | Architecture                   |
| -------------------- | ------ | ------------------------------ |
| Create (scaffold)    | Gap    |                                |
| Invoke: CLI          | Gap    | [Run](#run)                    |
| Invoke: modifiers    | Gap    | [Run](#run)                    |
| Invoke: programmatic | Gap    | [Run](#run)                    |
| Transform: Rename    | Gap    | [Modify](#modify)              |
| Transform: Relocate  | Gap    | [Modify](#modify)              |
| Transform: Delete    | Gap    | [Modify](#modify)              |
| Manage Dependencies  | Gap    | [Modify](#modify)              |
| Sync (remote)        | Gap    |                                |
| Inspect: Where       | Gap    | [Inspect](#inspect)            |
| Inspect: Tree        | Gap    | [Inspect](#inspect)            |
| Inspect: Deps        | Gap    | [Inspect](#inspect)            |
| Shell Integration    | Gap    | [Run](#run)                    |

### Constraints

| Requirement           | Status | Architecture |
| --------------------- | ------ | ------------ |
| Invariants maintained | Gap    |              |
| Reversible operations | Gap    |              |
| Minimal changes       | Gap    |              |
| Fail clearly          | Gap    |              |

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

---

# Operations

## Discover

How targ finds targets in the codebase.

**Gap** - needs design

Questions:
- Build tag filtering (`//go:build targ`)
- How are `targ.Targ()` wrappers and `targ.Group()` discovered?
- AST parsing vs runtime reflection?

## Inspect

How users query information about targets.

**Gap** - needs design

| Query   | What it shows                    | CLI              |
| ------- | -------------------------------- | ---------------- |
| Help    | Arguments, description, examples | `--help`         |
| Tree    | Full hierarchy visualization     | `--tree`         |
| Where   | Source file and line number      | `--where <path>` |
| Deps    | What target depends on           | `--deps <path>`  |

## Modify

How users change target aspects via CLI.

**Gap** - needs design

| Operation          | Aspect affected | CLI                      |
| ------------------ | --------------- | ------------------------ |
| Rename             | Hierarchy       | `--rename OLD NEW`       |
| Relocate           | Source          | `--relocate PATH FILE`   |
| Delete             | All             | `--delete PATH`          |
| Manage Dependencies| Execution       | `--deps-add`, `--deps-rm`|

Constraints:
- Reversible via command surface
- Minimal code changes
- Fail clearly if invariants can't be maintained

## Specify

How users reference targets.

**Gap** - needs design

Questions:
- Path syntax: `dev.lint.fast` or `dev lint fast`?
- Pattern matching: `dev.lint.*` for all lint targets?
- How to specify for different operations (run vs modify)?

## Run

How targets execute.

**Gap** - needs design

| Mode         | Description                           |
| ------------ | ------------------------------------- |
| CLI single   | `targ dev build`                      |
| CLI multiple | `targ dev build test` (sequence)      |
| CLI args     | `targ dev deploy --env prod`          |
| Programmatic | `targ.Deps(Build, Test)`              |

Runtime modifiers (CLI flags):
- `--watch` - re-run on file changes
- `--timeout` - execution timeout
- Possibly: `--retry`, `--repeat`, `--for`

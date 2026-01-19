# Architecture

How targ implements the requirements.

## Requirements Traceability

Maps requirements to architecture. **Gap** = not yet addressed.

### Model: Targets

| Requirement     | Status | Architecture                                                             |
| --------------- | ------ | ------------------------------------------------------------------------ |
| Basic           | ✓      | [Target Representation](#target-representation) - `func Name()`          |
| Failure         | ✓      | [Target Representation](#target-representation) - `func Name() error`    |
| Cancellation    | ✓      | [Target Representation](#target-representation) - `func Name(ctx) error` |
| Dependencies    | ✓      | [Target Builder](#target-builder) - `.Deps()`                            |
| Parallel        | ✓      | [Target Builder](#target-builder) - `.ParallelDeps()`                    |
| Serial          | ✓      | [Target Builder](#target-builder) - `.Deps()` (default)                  |
| Help text       | ✓      | [Target Builder](#target-builder) - `.Description()`                     |
| Arguments       | ✓      | [Arguments](#arguments) - struct parameter with tags                     |
| Repeated args   | ✓      | [Arguments](#arguments) - `[]T` field accumulates values                 |
| Map args        | ✓      | [Arguments](#arguments) - `map[K]V` field with `key=value` syntax        |
| Variadic args   | ✓      | [Arguments](#arguments) - trailing `[]T` positional captures rest        |
| Subcommands     | ✓      | [Hierarchy](#hierarchy) - `targ.Group()`                                 |
| Result caching  | ✓      | [Target Builder](#target-builder) - `.Cache()`                           |
| Watch mode      | ✓      | [Target Builder](#target-builder) - `.Watch()`                           |
| Retry + backoff | ✓      | [Target Builder](#target-builder) - `.Retry()`, `.Backoff()`             |
| Repetition      | ✓      | [Target Builder](#target-builder) - `.Times()`                           |
| Time-bounded    | ✓      | [Target Builder](#target-builder) - `.For()`                             |
| Condition-based | ✓      | [Target Builder](#target-builder) - `.While()`                           |

### Model: Hierarchy

| Requirement                      | Status | Architecture                                                     |
| -------------------------------- | ------ | ---------------------------------------------------------------- |
| Namespace nodes (non-executable) | ✓      | [Hierarchy](#hierarchy) - Groups are not executable              |
| Target nodes (executable)        | ✓      | [Target Representation](#target-representation) - functions only |
| Path addressing                  | Gap    |                                                                  |
| Simplest possible definition     | ✓      | Functions discovered automatically                               |
| Scales to complex hierarchies    | ✓      | [Hierarchy](#hierarchy) - nested Groups                          |
| Easy CLI binary transition       | Gap    |                                                                  |

### Model: Sources

| Requirement    | Status | Architecture              |
| -------------- | ------ | ------------------------- |
| Local targets  | ✓      | Functions in tagged files |
| Remote targets | Gap    |                           |

### Operations

| Requirement          | Status | Architecture |
| -------------------- | ------ | ------------ |
| Create (scaffold)    | Gap    |              |
| Invoke: CLI          | Gap    |              |
| Invoke: modifiers    | Gap    |              |
| Invoke: programmatic | Gap    |              |
| Transform: Rename    | Gap    |              |
| Transform: Relocate  | Gap    |              |
| Transform: Delete    | Gap    |              |
| Manage Dependencies  | Gap    |              |
| Sync (remote)        | Gap    |              |
| Inspect: Where       | Gap    |              |
| Inspect: Tree        | Gap    |              |
| Inspect: Deps        | Gap    |              |
| Shell Integration    | Gap    |              |

### Constraints

| Requirement           | Status | Architecture |
| --------------------- | ------ | ------------ |
| Invariants maintained | Gap    |              |
| Reversible operations | Gap    |              |
| Minimal changes       | Gap    |              |
| Fail clearly          | Gap    |              |

---

## Target Representation

Functions are the only executable targets. Capabilities are added progressively via signature changes:

| Signature                                      | Capabilities   |
| ---------------------------------------------- | -------------- |
| `func Name()`                                  | Basic          |
| `func Name() error`                            | + Failure      |
| `func Name(ctx context.Context) error`         | + Cancellation |
| `func Name(ctx context.Context, args T) error` | + Arguments    |

### Arguments

When a function needs CLI arguments, add a struct parameter. The struct's fields define flags and positionals via tags:

```go
type DeployArgs struct {
    Env    string `targ:"flag,required,desc=Target environment"`
    DryRun bool   `targ:"flag,short=n,desc=Preview mode"`
}

func Deploy(ctx context.Context, args DeployArgs) error {
    // args.Env and args.DryRun populated from CLI
}
```

Inline anonymous structs also work:

```go
func Deploy(ctx context.Context, args struct {
    Env    string `targ:"flag,required"`
    DryRun bool   `targ:"flag,short=n"`
}) error { ... }
```

targ reflects on the function signature, finds the args struct, reads its tags.

## Target Builder

Target metadata (dependencies, caching, etc.) is declared via a builder pattern. This separates:

- **Function**: executable logic
- **Builder**: target metadata
- **Group**: hierarchy only

```go
func Targ(fn any) *Target
```

### Builder methods

```go
targ.Targ(fn)                 // wrap a function
    .Deps(targets...)         // serial dependencies (run before target)
    .ParallelDeps(targets...) // parallel dependencies
    .Cache(patterns...)       // skip if inputs unchanged
    .Watch(patterns...)       // file patterns that trigger re-run
    .Retry(n)                 // retry on failure
    .Backoff(initial, multiplier) // delay between retries, grows exponentially
    .Times(n)                 // run N times regardless of outcome
    .For(duration)            // run until duration elapsed
    .While(func() bool)       // run while predicate returns true
    .Name(s)                  // override CLI name (default: function name)
    .Description(s)           // help text
```

`.Deps()` and `.ParallelDeps()` accept both raw functions and wrapped `*Target`. This allows chaining dependencies:

### Example

```go
func Format(ctx context.Context) error { ... }
func Build(ctx context.Context) error { ... }
func LintFast(ctx context.Context) error { ... }
func LintFull(ctx context.Context) error { ... }
func Deploy(ctx context.Context, args DeployArgs) error { ... }

// Wrap with modifiers - deps can reference other *Target or raw functions
var format = targ.Targ(Format)
var build = targ.Targ(Build).Deps(format)
var lintFast = targ.Targ(LintFast).ParallelDeps(format, build).Cache("**/*.go")
var lintFull = targ.Targ(LintFull).Deps(lintFast)
var deploy = targ.Targ(Deploy).Deps(build, lintFull)

// Group for hierarchy
var Lint = targ.Group("lint", lintFast, lintFull)
var Dev = targ.Group("dev", format, build, Lint, deploy)
```

### Raw functions in Groups

Groups accept both wrapped `*Target` and raw functions. Raw functions get default settings (no deps, no caching):

```go
var Dev = targ.Group("dev",
    format,    // wrapped *Target
    Build,     // raw function - equivalent to targ.Targ(Build)
)
```

## Hierarchy

Groups create the command namespace hierarchy. They are purely organizational (not executable).

```go
func Group(name string, members ...any) *Group
```

Members can be:
- Raw functions
- Wrapped `*Target` (from `targ.Targ()`)
- Nested `*Group`

### Nesting

```go
var Lint = targ.Group("lint", lintFast, lintFull)
var Test = targ.Group("test", testUnit, testIntegration)
var Dev = targ.Group("dev", build, Lint, Test, deploy)

// targ dev build
// targ dev lint fast
// targ dev test unit
```

### Why explicit names?

Group names must be explicit because:

1. Go reflection cannot retrieve variable names
2. Nested groups are values - no way to derive "lint" from `lintGroup`'s value

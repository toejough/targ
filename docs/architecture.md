# Architecture

How targ implements the requirements.

## Target Representation

Functions are the only executable targets. Capabilities are added progressively via signature changes:

| Signature | Capabilities |
|-----------|--------------|
| `func Name()` | Basic |
| `func Name() error` | + Failure |
| `func Name(ctx context.Context) error` | + Cancellation |
| `func Name(ctx context.Context, args T) error` | + Arguments |

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

## Hierarchy

Namespaces are created with `targ.Group`. Functions are the only executable targets; Groups are purely organizational (not executable).

```go
func Group(name string, members ...any) *Group
```

### Basic usage

```go
func LintFast(ctx context.Context) error { ... }
func LintForFail(ctx context.Context) error { ... }

var Lint = targ.Group("lint", LintFast, LintForFail)
// targ lint fast
// targ lint for-fail
```

### Nesting

Groups can contain other Groups:

```go
var lint = targ.Group("lint", LintFast, LintForFail)
var test = targ.Group("test", TestUnit, TestIntegration)

var Dev = targ.Group("dev", lint, test)
// targ dev lint fast
// targ dev test unit
```

### Mixed

Groups can contain both functions and nested Groups:

```go
var Dev = targ.Group("dev",
    Build,  // targ dev build
    targ.Group("lint", LintFast, LintForFail),  // targ dev lint fast
)
```

### Why explicit names?

The name must be passed explicitly because:
1. Go reflection cannot retrieve variable names
2. Nested groups are values - `targ.Group("dev", lintGroup, testGroup)` has no way to derive "lint" from `lintGroup`'s value without the Group carrying its own name

## Open Questions

### Discovery

How does targ find Groups and functions in build tool mode?

### Help Text

Where do target descriptions come from? Doc comments? Methods?

### Transformations

How do rename/relocate/delete operations work with this model?

### Other Capabilities

How are these expressed?
- Dependencies (`targ.Deps` exists)
- Parallel/Serial execution
- Result caching
- Watch mode
- Retry, Repetition, Time-bounded, Condition-based

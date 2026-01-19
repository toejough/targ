# Architecture

How targ implements the requirements.

## Requirements Traceability

Maps requirements to architecture. **Gap** = not yet addressed.

### Model: Targets

| Requirement | Status | Architecture |
|-------------|--------|--------------|
| Basic | ✓ | [Target Representation](#target-representation) - `func Name()` |
| Failure | ✓ | [Target Representation](#target-representation) - `func Name() error` |
| Cancellation | ✓ | [Target Representation](#target-representation) - `func Name(ctx) error` |
| Dependencies | Gap | |
| Parallel | Gap | |
| Serial | Gap | |
| Help text | Gap | |
| Arguments | ✓ | [Arguments](#arguments) - struct parameter with tags |
| Repeated args | Gap | |
| Map args | Gap | |
| Variadic args | Gap | |
| Subcommands | ✓ | [Hierarchy](#hierarchy) - `targ.Group()` |
| Result caching | Gap | |
| Watch mode | Gap | |
| Retry | Gap | |
| Repetition | Gap | |
| Time-bounded | Gap | |
| Condition-based | Gap | |

### Model: Hierarchy

| Requirement | Status | Architecture |
|-------------|--------|--------------|
| Namespace nodes (non-executable) | ✓ | [Hierarchy](#hierarchy) - Groups are not executable |
| Target nodes (executable) | ✓ | [Target Representation](#target-representation) - functions only |
| Path addressing | Gap | |
| Simplest possible definition | ✓ | Functions discovered automatically |
| Scales to complex hierarchies | ✓ | [Hierarchy](#hierarchy) - nested Groups |
| Easy CLI binary transition | Gap | |

### Model: Sources

| Requirement | Status | Architecture |
|-------------|--------|--------------|
| Local targets | ✓ | Functions in tagged files |
| Remote targets | Gap | |

### Operations

| Requirement | Status | Architecture |
|-------------|--------|--------------|
| Create (scaffold) | Gap | |
| Invoke: CLI | Gap | |
| Invoke: modifiers | Gap | |
| Invoke: programmatic | Gap | |
| Transform: Rename | Gap | |
| Transform: Relocate | Gap | |
| Transform: Delete | Gap | |
| Manage Dependencies | Gap | |
| Sync (remote) | Gap | |
| Inspect: Where | Gap | |
| Inspect: Tree | Gap | |
| Inspect: Deps | Gap | |
| Shell Integration | Gap | |

### Constraints

| Requirement | Status | Architecture |
|-------------|--------|--------------|
| Invariants maintained | Gap | |
| Reversible operations | Gap | |
| Minimal changes | Gap | |
| Fail clearly | Gap | |

---

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


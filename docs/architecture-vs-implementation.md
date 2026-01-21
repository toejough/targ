# Architecture vs Current Implementation

A systematic comparison of the architecture spec versus the current codebase.

## Summary

The architecture document proposes a **function-based model with Target Builders**, while the current implementation uses a **struct-based model with methods**. These are fundamentally different paradigms.

---

## Target Definition Model

### Architecture (Proposed)

```go
// Function-based with Target Builder pattern
var build = targ.Targ(Build).Deps(format).Cache("**/*.go")
var lint = targ.Targ("golangci-lint run $path")  // shell command string

func init() {
    targ.Run(Dev)
}
```

### Current Implementation

```go
// Struct-based with methods
type Coverage struct {
    HTML bool `targ:"flag,desc=Open HTML report in browser"`
}

func (c *Coverage) Description() string { return "Display coverage report" }
func (c *Coverage) Run() error { ... }

// OR plain functions with inline deps
func Check(ctx context.Context) error {
    return targ.Deps(Fmt, Lint, Test)
}
```

### Decision Points

| Feature         | Architecture            | Current                      | Change Type |
| --------------- | ----------------------- | ---------------------------- | ----------- |
| Target wrapping | `targ.Targ(fn)`         | N/A                          | **ADD**     |
| Shell commands  | `targ.Targ("cmd $var")` | N/A                          | **ADD**     |
| Struct targets  | Not mentioned           | `type X struct` with `Run()` | **REMOVE?** |
| Plain functions | Supported               | Supported                    | KEEP        |

---

## Dependency Declaration

### Architecture (Proposed)

```go
// Declarative on Target Builder
var build = targ.Targ(Build).Deps(format, lint)
var deploy = targ.Targ(Deploy).Deps(build).DepMode(targ.Parallel)
```

### Current Implementation

```go
// Imperative within function body
func Check(ctx context.Context) error {
    return targ.Deps(
        func() error { return Fmt(ctx) },
        func() error { return Lint(ctx) },
        targ.Parallel(),
    )
}
```

### Decision Points

| Feature           | Architecture              | Current                  | Change Type |
| ----------------- | ------------------------- | ------------------------ | ----------- |
| Declaration style | Declarative (builder)     | Imperative (inline)      | **CHANGE**  |
| Parallel deps     | `.DepMode(targ.Parallel)` | `targ.Parallel()` option | RENAME      |
| Continue on error | Not mentioned             | `targ.ContinueOnError()` | KEEP        |
| Context passing   | Implicit                  | `targ.WithContext(ctx)`  | KEEP        |
| Reset deps        | `targ.ResetDeps()`        | `targ.ResetDeps()`       | KEEP        |

---

## Execution Features

### Architecture (Proposed)

```go
targ.Targ(fn)
    .Cache(patterns...)           // skip if inputs unchanged
    .Watch(patterns...)           // re-run on file change
    .Retry(n)                     // retry on failure
    .Backoff(initial, multiplier) // exponential delay
    .Times(n)                     // run N times
    .Timeout(duration)            // cancel after duration
    .While(func() bool)           // run while predicate true
```

Runtime overrides:

```
targ build --watch "**/*.go"
targ build --cache "**/*.go"
targ build --timeout 5m
targ build --retry 3 --backoff 1s,2
```

### Current Implementation

- `file.Watch()` - standalone utility, not integrated
- `file.Checksum()` - standalone utility, not integrated
- `--timeout` - global CLI flag only
- No retry, backoff, times, while

### Decision Points

| Feature              | Architecture | Current                 | Change Type    |
| -------------------- | ------------ | ----------------------- | -------------- |
| `.Cache()` builder   | Per-target   | N/A                     | **ADD**        |
| `.Watch()` builder   | Per-target   | `file.Watch()` utility  | **ADD**        |
| `.Retry()` builder   | Per-target   | N/A                     | **ADD**        |
| `.Backoff()` builder | Per-target   | N/A                     | **ADD**        |
| `.Times()` builder   | Per-target   | N/A                     | **ADD**        |
| `.Timeout()` builder | Per-target   | `--timeout` global flag | **ADD**        |
| `.While()` builder   | Per-target   | N/A                     | **ADD**        |
| `--watch` CLI flag   | Override     | N/A                     | **ADD**        |
| `--cache` CLI flag   | Override     | N/A                     | **ADD**        |
| `--retry` CLI flag   | Override     | N/A                     | **ADD**        |
| `--timeout` CLI flag | Override     | Global only             | KEEP (extend?) |

---

## Hierarchy Model

### Architecture (Proposed)

```go
// Explicit Group function - non-executable
var Lint = targ.Group("lint", lintFast, lintFull)
var Dev = targ.Group("dev", format, build, Lint)

// Only functions are executable
```

### Current Implementation

```go
// Struct fields with subcommand tag
type Math struct {
    Add *AddCmd `targ:"subcommand"`
    Run *RunCmd `targ:"subcommand=run"`
}

// Parent structs CAN be executable (have Run() method)
func (m *Math) Run() { ... }
```

### Decision Points

| Feature             | Architecture              | Current                | Change Type |
| ------------------- | ------------------------- | ---------------------- | ----------- |
| Group definition    | `targ.Group("name", ...)` | Struct fields with tag | **CHANGE**  |
| Group executability | Non-executable            | Can have `Run()`       | **CHANGE**  |
| Naming              | Explicit string           | Field name or tag      | CHANGE      |
| Nesting             | Via `*Group` members      | Via struct composition | CHANGE      |

---

## CLI Flags

### Architecture (Proposed)

| Flag           | Short | Description                       |
| -------------- | ----- | --------------------------------- |
| `--parallel`   | `-p`  | Run multiple targets in parallel  |
| `--completion` |       | Print shell completion script     |
| `--source`     | `-s`  | Specify source (local or remote)  |
| `--create`     | `-c`  | Scaffold new target from command  |
| `--to-func`    |       | Convert string target to function |
| `--to-string`  |       | Convert function target to string |
| `--sync`       |       | Import targets from remote repo   |

### Current Implementation

| Flag           | Description                        |
| -------------- | ---------------------------------- |
| `--no-cache`   | Disable cached build tool binaries |
| `--keep`       | Keep generated bootstrap file      |
| `--timeout`    | Set execution timeout              |
| `--completion` | Print completion script            |
| `--init`       | Create starter targets file        |
| `--alias`      | Add shell command target           |
| `--move`       | Reorganize commands into hierarchy |
| `--help`       | Print help information             |

### Decision Points

| Flag                | Architecture            | Current     | Change Type |
| ------------------- | ----------------------- | ----------- | ----------- |
| `--parallel` / `-p` | Proposed                | N/A         | **ADD**     |
| `--source` / `-s`   | Proposed                | N/A         | **ADD**     |
| `--create` / `-c`   | Proposed                | N/A         | **ADD**     |
| `--to-func`         | Proposed                | N/A         | **ADD**     |
| `--to-string`       | Proposed                | N/A         | **ADD**     |
| `--sync`            | Proposed                | N/A         | **ADD**     |
| `--completion`      | Proposed                | Exists      | KEEP        |
| `--no-cache`        | Not mentioned           | Exists      | KEEP?       |
| `--keep`            | Not mentioned           | Exists      | KEEP?       |
| `--timeout`         | Extend to per-target    | Global only | EXTEND      |
| `--init`            | Subsumed by `--create`? | Exists      | REMOVE?     |
| `--alias`           | Subsumed by `--create`? | Exists      | REMOVE?     |
| `--move`            | Not mentioned           | Exists      | KEEP        |

---

## Arguments System

### Matching Features

Both support:

- `targ:"flag"` and `targ:"positional"` tags
- `required` option
- `default=` option
- `env=` option
- `enum=` option
- `desc=` for descriptions
- `short=` for short flags
- `name=` for custom names
- `placeholder=` for help text
- `encoding.TextUnmarshaler` support
- `Set(string) error` interface support
- `Interleaved[T]` for ordered arguments
- `[]T` for repeated values
- `map[K]V` for key-value pairs

### No Changes Needed

The arguments system is well-aligned.

---

## Shell Execution

### Architecture (Proposed)

```go
// Polymorphic Targ - accepts function OR string
var lint = targ.Targ("golangci-lint run $path")

// Shell helper for functions
func Deploy(ctx context.Context, args DeployArgs) error {
    return targ.Shell(ctx, "kubectl apply -n $namespace", args)
}
```

### Current Implementation

```go
// Separate sh package
sh.Run("go", "build", "./...")
sh.RunV("go", "test", "./...")  // verbose
sh.Output("go", "version")
```

### Decision Points

| Feature                      | Architecture         | Current | Change Type |
| ---------------------------- | -------------------- | ------- | ----------- |
| `targ.Targ(string)`          | Shell command target | N/A     | **ADD**     |
| `targ.Shell(ctx, cmd, args)` | Var substitution     | N/A     | **ADD**     |
| `sh.Run()` etc               | Not mentioned        | Exists  | KEEP        |

---

## Programmatic API

### Architecture (Proposed)

```go
// Invoke target with full execution config
err := build.Run(ctx)
err := deploy.Run(ctx, DeployArgs{Env: "prod"})
```

### Current Implementation

```go
// Execute for testing
result, err := targ.Execute(args, targets...)
result, err := targ.ExecuteWithOptions(args, opts, targets...)
```

### Decision Points

| Feature           | Architecture  | Current | Change Type |
| ----------------- | ------------- | ------- | ----------- |
| `target.Run(ctx)` | Proposed      | N/A     | **ADD**     |
| `targ.Execute()`  | Not mentioned | Exists  | KEEP        |

---

## Create / Scaffold

### Architecture (Proposed)

```
targ --create deploy "kubectl apply -n $namespace -f $file"
targ --create --cache "**/*.go" lint "golangci-lint run"
targ --create dev lint fast "golangci-lint run"  # creates: dev/lint/fast
```

### Current Implementation

```
targ --init [FILE]           # create starter file
targ --alias NAME "CMD" [FILE]  # add shell command target
```

### Decision Points

| Feature            | Architecture            | Current                       | Change Type |
| ------------------ | ----------------------- | ----------------------------- | ----------- |
| `--create` unified | Proposed                | `--init` + `--alias` separate | **CHANGE**  |
| Path specification | `dev lint fast`         | N/A                           | **ADD**     |
| Execution flags    | `--cache`, `--deps` etc | N/A                           | **ADD**     |
| Arg inference      | `$var` → flag           | N/A                           | **ADD**     |

---

## Sync / Remote

### Architecture (Proposed)

```
targ --sync github.com/foo/bar
```

### Current Implementation

Not implemented.

### Decision Points

| Feature  | Architecture | Current | Change Type |
| -------- | ------------ | ------- | ----------- |
| `--sync` | Proposed     | N/A     | **ADD**     |

---

## Questions for Review

1. **Struct vs Function model**: The current struct-based model allows parent commands to be executable. The architecture proposes functions-only with non-executable groups. Which is preferred?

2. **Declarative vs Imperative deps**: Builder pattern (`.Deps()`) vs inline function calls (`targ.Deps()`). The builder is more declarative but less flexible.

3. **--init and --alias**: Should these be kept alongside `--create`, or replaced entirely?

4. **--no-cache and --keep**: These are targ-internal flags for binary caching. Keep or remove?

5. **Execution features**: The architecture proposes many new features (retry, backoff, times, while). Are all needed for v1?

6. **Runtime overrides ownership model**: The architecture proposes `targ.Disabled` for user takeover. Worth the complexity?

## Answers

1. Function model.
2. declarative.
3. Replace with --create.
4. we don't need keep. I think --no-cache is good to keep, but call it --no-binary-cache to be more explicit.
5. Yes, all needed.
6. Yes, keep it. we need a way for users to fully take over execution behavior.


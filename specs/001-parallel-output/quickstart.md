# Quickstart: Parallel Output

**Feature**: 001-parallel-output | **Date**: 2026-02-19

## For Target Authors

### Using parallel-aware output

Replace `fmt.Print` with `targ.Print` in your target functions to get automatic parallel prefixing:

```go
import "github.com/toejough/targ"

func Build(ctx context.Context) error {
    targ.Print(ctx, "compiling...\n")
    targ.Printf(ctx, "built %d files\n", count)
    return nil
}
```

- **Serial mode**: output is `compiling...` (unchanged)
- **Parallel mode**: output is `[build] compiling...`

### Custom lifecycle hooks

```go
target := targ.Targ(Build).
    OnStart(func(ctx context.Context, name string) {
        targ.Printf(ctx, ">> %s starting <<\n", name)
    }).
    OnStop(func(ctx context.Context, name string, result targ.Result, d time.Duration) {
        targ.Printf(ctx, ">> %s %s in %s <<\n", name, result, d)
    })
```

### Shell command targets

Shell commands are automatically captured and prefixed in parallel mode. No changes needed:

```go
target := targ.Targ("go build ./...")
```

Output in parallel mode: `[build] ...` for each line of shell output.

## For Implementers

### Build and test

```bash
go test -tags sqlite_fts5 ./internal/core/ -run TestExecInfo -v    # Task 1
go test -tags sqlite_fts5 ./internal/core/ -run TestPrinter -v     # Task 2
go test -tags sqlite_fts5 ./internal/core/ -run TestPrefixWriter -v # Task 3
go test -tags sqlite_fts5 ./internal/core/ -run TestPrint -v       # Task 4
go test -tags sqlite_fts5 ./internal/core/ -run TestResult -v      # Task 5
go test -tags sqlite_fts5 ./internal/core/ -run TestOnStart -v     # Task 6
go test -tags sqlite_fts5 ./internal/core/ -run TestParallelOutput -v # Task 7
go test -tags sqlite_fts5 ./...                                     # Full suite
```

### Implementation order

Tasks are designed to be implemented sequentially (each builds on the previous):

1. ExecInfo context metadata (standalone, no deps)
2. Printer goroutine (standalone, no deps)
3. PrefixWriter (depends on Printer)
4. Print/Printf (depends on ExecInfo + Printer)
5. Result type (standalone, no deps)
6. OnStart/OnStop hooks (depends on Result)
7. Wire parallel executors (depends on all above)
8. Shell command integration + public API exports (depends on 7)

### Key implementation notes

- **Thread `io.Writer` to Printer**: In tests, output goes to `ExecuteEnv.output` (strings.Builder), not `os.Stdout`. The Printer must be constructed with the test's output buffer. Don't rely on the `printOutput` global for the Printer's writer — thread it through from `RunEnv.Stdout()`.
- **Prefix padding**: Compute `maxNameLen` across all targets in the parallel group before spawning goroutines. Pad with spaces: `fmt.Sprintf("[%-*s] ", maxNameLen, name)`.
- **Duration tracking**: Record `start := time.Now()` inside each goroutine, compute duration after `d.Run(tctx)` returns. Send duration with the result via the error channel.

### Existing implementation plan

The detailed task-by-task implementation plan with test code and implementation code is at:
`docs/plans/2026-02-19-parallel-output.md`

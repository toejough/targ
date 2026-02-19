# Parallel Output Coordination

## Problem

When targets run in parallel (both dep-level via `.Deps(..., Parallel)` and top-level via `--parallel`), output interleaves unpredictably. Lines from different targets garble together. There is no identification of which target produced which output. When fail-fast cancels sibling targets, there is no indication of what was cancelled vs what actually failed — LLMs and humans alike incorrectly assume the single reported failure is the only problem.

## Goals

1. **Prefixed output** — parallel target output is prefixed with `[name]` so every line is attributed
2. **Line atomicity** — lines never interleave mid-line across targets
3. **Lifecycle announcements** — targets announce start and final state (pass/fail/cancelled/errored)
4. **Summary line** — final terse summary: `PASS:1 FAIL:1 CANCELLED:1 ERRORED:0`
5. **No change to serial execution** — serial targets print directly to stdout as today
6. **User-overridable** — hooks for start/stop, explicit `io.Writer` override for `sh.Run`

## Design

### Context Metadata (Data Only)

The parallel executor injects execution metadata into context. Context carries **data**, not behavior — no writers or channels in context.

```go
type ExecInfo struct {
    Parallel bool   // true if running in a parallel group
    Name     string // target name, used as prefix
}

// context key
type execInfoKey struct{}

func WithExecInfo(ctx context.Context, info ExecInfo) context.Context
func GetExecInfo(ctx context.Context) (ExecInfo, bool)
```

### Printer Goroutine (Package-Level Singleton)

A single goroutine owns stdout during parallel execution. All output flows through a buffered channel.

```go
type printLine struct {
    text string // already prefixed, newline-terminated
}

type Printer struct {
    ch   chan printLine
    done chan struct{}
    out  io.Writer
}

func newPrinter(out io.Writer, bufSize int) *Printer
func (p *Printer) Send(line string)  // non-blocking until buffer full
func (p *Printer) Close()            // drains channel, waits for goroutine to exit
```

Lifecycle:
1. Parallel executor creates Printer before spawning target goroutines
2. Printer goroutine reads from channel, writes to `out` (os.Stdout)
3. After all targets complete, executor calls `Close()` — drains remaining lines, then prints summary

The Printer is package-level (singleton). Only one parallel execution scope is active at a time (parallel groups nest, they don't run concurrently at the same level).

### `targ.Print` / `targ.Printf`

Public API for target authors:

```go
func Print(ctx context.Context, args ...any)
func Printf(ctx context.Context, format string, args ...any)
```

Behavior:
- Reads `ExecInfo` from context
- If `Parallel == true`: formats as `[name] text\n`, sends to package-level Printer
- If serial (or no ExecInfo): `fmt.Print(args...)`

Multi-line content is split — each line gets its own prefix.

### `sh.Run` Integration

`sh.Run` and related functions use `targ.Print` logic by default for stdout/stderr:
- Default: a writer that routes through `targ.Print` (prefix if parallel, direct if serial)
- Override: user passes explicit `io.Writer`, bypassing all prefix/channel logic

```go
// Existing API, with optional writer override
sh.Run(ctx, "go build ./...")                           // default: uses targ.Print
sh.RunWithWriter(ctx, w, "go build ./...")              // override: user's writer
```

### OnStart / OnStop Hooks

Builder methods on `*Target`:

```go
func (t *Target) OnStart(fn func(ctx context.Context, name string)) *Target
func (t *Target) OnStop(fn func(ctx context.Context, name string, result Result)) *Target
```

Defaults (used if not overridden):
- `OnStart`: `targ.Printf(ctx, "starting...")`
- `OnStop`: `targ.Printf(ctx, "%s (%s)", result, duration)` — e.g., `[build] PASS (1.2s)`

Hooks fire inside the target's goroutine, so they use the target's context (with correct ExecInfo).

### Result Tracking

```go
type Result int

const (
    Pass      Result = iota
    Fail
    Cancelled
    Errored
)
```

The parallel executor collects results per target:

```go
type TargetResult struct {
    Name     string
    Status   Result
    Duration time.Duration
    Err      error
}
```

Classification:
- Returns `nil` → `Pass`
- Returns `context.Canceled` and is not the first failure → `Cancelled`
- Returns `context.DeadlineExceeded` → `Errored`
- Returns any other error → `Fail`

### Summary Line

After Printer is drained, the executor prints directly to stdout:

```
PASS:2 FAIL:1 CANCELLED:1
```

Only non-zero counts are shown.

### Output Examples

**Interactive parallel execution:**
```
[build] starting...
[test]  starting...
[lint]  starting...
[build] compiling main.go...
[build] compiling utils.go...
[test]  running tests...
[build] PASS (1.2s)
[test]  FAIL (0.3s)
[lint]  CANCELLED (test failed)

PASS:1 FAIL:1 CANCELLED:1
```

**Serial execution (unchanged):**
```
compiling main.go...
compiling utils.go...
```

## Scope

### In Scope
- Both levels of parallelism: dep-level (`.Deps(..., Parallel)`) and top-level (`--parallel`)
- `targ.Print` / `targ.Printf` public API
- `sh.Run` default writer integration with explicit override
- `OnStart` / `OnStop` hooks with sensible defaults
- Per-target result tracking with summary line

### Out of Scope
- TTY detection / interactive vs non-interactive formatting differences
- JSONL or structured output modes
- Buffered/grouped output (output interleaves at line level, but each line is atomic and prefixed)
- Color / ANSI formatting

## Data Model Changes

```go
// Target struct additions:
type Target struct {
    // ... existing fields ...
    onStart func(ctx context.Context, name string)
    onStop  func(ctx context.Context, name string, result Result)
}
```

## Files Affected

| File | Change |
|------|--------|
| `internal/core/target.go` | Add OnStart/OnStop fields, Result type, result tracking in `runGroupParallel` |
| `internal/core/run_env.go` | Result tracking in `executeDefaultParallel`, Printer lifecycle, summary line |
| `internal/core/exec_info.go` | New: ExecInfo type, context helpers |
| `internal/core/printer.go` | New: Printer goroutine, channel, lifecycle |
| `internal/core/print.go` | New: `targ.Print` / `targ.Printf` (or in public API file) |
| `internal/sh/sh.go` | Default writer uses targ.Print logic, add explicit writer override |
| `targ.go` | Re-export Print/Printf, Result, OnStart/OnStop |

# Data Model: Parallel Output

**Feature**: 001-parallel-output | **Date**: 2026-02-19

## Entities

### ExecInfo

Execution metadata carried through `context.Context`. Data only — no behavior.

| Field | Type | Description |
|-------|------|-------------|
| Parallel | `bool` | true if running in a parallel group |
| Name | `string` | target name, used as output prefix |

**Location**: `internal/core/exec_info.go`
**Context key**: unexported `execInfoKey` struct

### Printer

Singleton goroutine that serializes output from parallel targets. Owns stdout during parallel execution.

| Field | Type | Description |
|-------|------|-------------|
| ch | `chan string` | buffered channel for complete, prefixed lines |
| done | `chan struct{}` | signals goroutine exit after drain |
| out | `io.Writer` | destination writer (typically os.Stdout or test buffer) |

**Location**: `internal/core/printer.go`
**Lifecycle**: Created before spawning goroutines, closed after all targets complete.

### PrefixWriter

`io.Writer` adapter that buffers partial lines and sends complete, prefixed lines to a Printer. Used as stdout/stderr for shell commands in parallel mode.

| Field | Type | Description |
|-------|------|-------------|
| prefix | `string` | prefix string, e.g. `[build] ` |
| printer | `*Printer` | destination printer |
| buf | `strings.Builder` | partial line buffer |

**Location**: `internal/core/prefix_writer.go`

### Result

Outcome classification for a parallel target execution.

| Value | Int | Description |
|-------|-----|-------------|
| Pass | 0 | Target succeeded (err == nil) |
| Fail | 1 | Target failed (non-nil error, not cancelled/timeout) |
| Cancelled | 2 | Target cancelled due to sibling failure (context.Canceled, not first failure) |
| Errored | 3 | Target timed out (context.DeadlineExceeded) |

**Location**: `internal/core/result.go`

### TargetResult

Per-target outcome in a parallel group.

| Field | Type | Description |
|-------|------|-------------|
| Name | `string` | target name |
| Status | `Result` | outcome classification |
| Duration | `time.Duration` | wall-clock execution time |
| Err | `error` | original error (nil for Pass) |

**Location**: `internal/core/result.go`

## Modifications to Existing Entities

### Target (existing: `internal/core/target.go`)

Two new fields added to the `Target` struct:

| Field | Type | Description |
|-------|------|-------------|
| onStart | `func(ctx context.Context, name string)` | lifecycle hook, fires at target start in parallel mode |
| onStop | `func(ctx context.Context, name string, result Result, duration time.Duration)` | lifecycle hook, fires at target completion in parallel mode |

Both default to `nil`. When nil, the parallel executor uses built-in defaults:
- **onStart default**: `Print(ctx, "starting...\n")`
- **onStop default**: `Printf(ctx, "%s (%s)\n", result, duration)`

## Relationships

```text
Context ──carries──> ExecInfo
    │
    ├── targ.Print(ctx) reads ExecInfo to decide output path
    │       │
    │       ├── serial: fmt.Fprint(stdout)
    │       └── parallel: Printer.Send(prefixed line)
    │
    └── PrefixWriter(io.Writer) ──sends──> Printer.Send()
            │
            └── used as cmd.Stdout/cmd.Stderr for shell targets

Printer ──writes──> io.Writer (os.Stdout or test buffer)

Target.Run()
    ├── onStart hook (or default)
    ├── execute (function or shell)
    └── onStop hook (or default)
            │
            └── TargetResult collected by parallel executor
                    │
                    └── FormatSummary() prints "PASS:N FAIL:N ..."
```

## State Transitions

### Target Execution in Parallel Mode

```text
[pending] ──spawn goroutine──> [starting]
    │                              │
    │                         onStart fires
    │                              │
    │                         [running]
    │                              │
    │                    ┌─────────┴─────────┐
    │                    │                   │
    │              err == nil          err != nil
    │                    │                   │
    │                 [Pass]          classify error
    │                                        │
    │                              ┌─────────┼─────────┐
    │                              │         │         │
    │                        [Fail]   [Cancelled]  [Errored]
    │                              │         │         │
    │                              └─────────┴─────────┘
    │                                        │
    │                                   onStop fires
    │                                        │
    └────────────────────────────────> [complete]
```

## Validation Rules

- ExecInfo.Name must be non-empty when Parallel is true
- Printer buffer size must be > 0
- PrefixWriter prefix must be non-empty
- FormatSummary only includes non-zero status counts
- ClassifyResult: `context.Canceled` with `isFirstFailure=true` maps to Fail (the target that caused cancellation failed, it wasn't cancelled)

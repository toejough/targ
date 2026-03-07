# Research: Parallel Output

**Feature**: 001-parallel-output | **Date**: 2026-02-19

## Research Tasks

### 1. Line-atomic output coordination pattern

**Decision**: Single Printer goroutine with buffered channel

**Rationale**: One goroutine owns the output writer (stdout). All parallel targets send complete, prefixed lines via a buffered channel. The Printer goroutine reads from the channel and writes sequentially, guaranteeing atomicity without mutexes on the write path. Channel back-pressure prevents output loss (writes block when buffer is full).

**Alternatives considered**:
- **Mutex on stdout**: Simpler, but requires all callsites to hold the lock for the full line write. Easy to miss a callsite. Doesn't compose with shell command output that writes arbitrary chunks.
- **Per-target buffer with sequential flush**: Buffers all output per target, prints each target's output as a block after completion. Loses real-time streaming — users don't see output until a target finishes. Rejected for long-running targets.
- **io.Writer wrapper with mutex**: Each write acquires a mutex, but raw `Write()` calls from shell commands are arbitrary byte chunks, not line-aligned. Would need a line-buffering layer anyway, converging on the PrefixWriter + Printer design.

### 2. Context vs. global state for parallel detection

**Decision**: Context carries metadata (ExecInfo struct); Printer is package-level singleton

**Rationale**: Context is the idiomatic Go mechanism for request-scoped data. ExecInfo carries only data (parallel flag, target name) — no writers or channels in context. The Printer is a singleton because only one parallel execution scope is active at a time (parallel groups nest but don't run concurrently at the same level). This avoids threading a Printer reference through every function signature.

**Alternatives considered**:
- **Printer in context**: Would require extracting from context at every output call. Couples context to behavior, not just data. Rejected per Go best practices.
- **Thread Printer through function params**: Too invasive — would require changing the signature of `Target.Run`, `runGroupParallel`, and all functions in the call chain. Existing code uses package-level `fmt.Print` patterns.

### 3. Shell command output integration

**Decision**: PrefixWriter (io.Writer adapter) wraps Printer; used as stdout/stderr for `exec.Cmd`

**Rationale**: Shell commands write arbitrary byte chunks via `io.Writer`. PrefixWriter buffers partial lines (bytes until `\n`), then sends complete prefixed lines to Printer. On target completion, `Flush()` emits any remaining partial line. This reuses the same Printer channel, guaranteeing line atomicity for shell output alongside programmatic output.

**Alternatives considered**:
- **Pipe through `bufio.Scanner`**: Would require spawning additional goroutines per stream (stdout + stderr per target). More complex, and Scanner has a max line size (64KB default). PrefixWriter is simpler with unbounded line buffering.
- **Capture all output, replay after completion**: Loses real-time streaming. Same rejection as per-target buffer approach.

### 4. Result classification strategy

**Decision**: Classify based on error type and first-failure tracking

**Rationale**: `context.Canceled` with `isFirstFailure=false` means the target was cancelled due to a sibling's failure (CANCELLED). `context.DeadlineExceeded` means timeout (ERRORED). Any other non-nil error is FAIL. `nil` is PASS. The parallel executor tracks which goroutine first triggered `cancel()` to distinguish the failing target from cancelled siblings.

**Alternatives considered**:
- **Custom error types per result**: Would require wrapping errors in Result-aware types throughout the codebase. Overengineered — the existing error + context pattern is sufficient.
- **Separate signal channel for first-failure**: Adds complexity. A simple `firstErrIdx` integer suffices since we already collect all results.

### 5. Prefix alignment (right-padding)

**Decision**: Compute max target name length within the parallel group; right-pad all prefixes

**Rationale**: When targets have names of different lengths (e.g., "build" vs "test"), padding aligns the output columns for readability. The max length is known before spawning goroutines since all targets in the group are known upfront.

**Alternatives considered**:
- **No padding**: Simpler, but misaligned output is harder to scan visually. The spec explicitly requires alignment (FR-014).
- **Fixed-width prefix**: Would truncate long names or waste space for short names. Dynamic padding is better.

### 6. Backward compatibility with serial mode

**Decision**: `targ.Print`/`targ.Printf` check context for ExecInfo; absent or `Parallel=false` writes directly to stdout

**Rationale**: Zero behavior change for serial targets. The Print functions are additive — existing code that uses `fmt.Print` directly still works in serial mode. Only targets that opt into `targ.Print` get automatic parallel awareness. Shell commands in serial mode continue using `DefaultShellEnv()` with `os.Stdout`.

**Alternatives considered**:
- **Always buffer output**: Would add latency to serial mode for no benefit. Rejected.
- **Separate parallel-only Print functions**: Would split the API. `targ.Print` with context detection is cleaner — one API, two behaviors.

## Unresolved Items

None. All technical decisions are resolved.

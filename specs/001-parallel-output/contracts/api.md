# Public API Contract: Parallel Output

**Feature**: 001-parallel-output | **Date**: 2026-02-19

## New Public API (exported from `targ.go`)

### Output Functions

```go
// Print writes output. In parallel mode (detected via context), prefixes each
// line with [target-name]. In serial mode, writes directly to stdout.
func Print(ctx context.Context, args ...any)

// Printf writes formatted output. Same parallel/serial behavior as Print.
func Printf(ctx context.Context, format string, args ...any)
```

**Behavior contract**:
- When `ExecInfo` is absent from context OR `Parallel == false`: equivalent to `fmt.Print(args...)`
- When `Parallel == true`: splits text on `\n`, sends each line as `[name] line\n` to Printer
- Multi-line text: each line individually prefixed
- Trailing `\n` does not produce an empty prefixed line

### Result Type

```go
type Result = core.Result

const (
    Pass      Result = iota // target succeeded
    Fail                    // target failed (non-nil error)
    Cancelled               // target cancelled due to sibling failure
    Errored                 // target timed out (deadline exceeded)
)
```

### Target Builder Methods

```go
// OnStart sets a lifecycle hook that fires when the target begins parallel execution.
// If nil (default), prints "[name] starting..."
func (t *Target) OnStart(fn func(ctx context.Context, name string)) *Target

// OnStop sets a lifecycle hook that fires when the target completes parallel execution.
// If nil (default), prints "[name] PASS (1.2s)"
func (t *Target) OnStop(fn func(ctx context.Context, name string, result Result, duration time.Duration)) *Target
```

## Internal API (not exported, but documented for implementers)

### ExecInfo (`internal/core/exec_info.go`)

```go
func WithExecInfo(ctx context.Context, info ExecInfo) context.Context
func GetExecInfo(ctx context.Context) (ExecInfo, bool)
```

### Printer (`internal/core/printer.go`)

```go
func NewPrinter(out io.Writer, bufSize int) *Printer
func (p *Printer) Send(line string)
func (p *Printer) Close()
```

### PrefixWriter (`internal/core/prefix_writer.go`)

```go
func NewPrefixWriter(prefix string, printer *Printer) *PrefixWriter
func (w *PrefixWriter) Write(p []byte) (int, error)  // implements io.Writer
func (w *PrefixWriter) Flush()                        // emits partial line
```

### Result Classification (`internal/core/result.go`)

```go
func ClassifyResult(err error, isFirstFailure bool) Result
func FormatSummary(results []TargetResult) string
```

## Output Format Contract

### Prefixed Line Format

```
[{name}] {content}\n
```

- `{name}`: target name as-is, wrapped in brackets
- Prefix right-padded to longest name in the parallel group (FR-014)
- One prefix per line of content

### Summary Line Format

```
{STATUS}:{count}[ {STATUS}:{count}]*
```

- Statuses in order: PASS, FAIL, CANCELLED, ERRORED
- Only non-zero counts included
- Printed after a blank line following all target output
- Examples: `PASS:3`, `PASS:1 FAIL:1 CANCELLED:1`

### Lifecycle Messages (defaults)

```
[{name}] starting...
[{name}] {STATUS} ({duration})
```

- Duration rounded to milliseconds
- STATUS is the Result string (PASS, FAIL, CANCELLED, ERRORED)

## Backward Compatibility

- Serial mode targets: zero behavior change. `targ.Print` with no ExecInfo in context writes directly to stdout.
- Existing `fmt.Print` calls in target functions: unaffected. Only `targ.Print`/`targ.Printf` are parallel-aware.
- Shell commands in serial mode: continue using `DefaultShellEnv()` with `os.Stdout` directly.
- `--parallel` flag behavior: enhanced with prefixed output and summary, but still executes targets concurrently.

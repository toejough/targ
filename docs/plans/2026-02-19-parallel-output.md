# Parallel Output Coordination Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add prefixed, line-atomic output for parallel target execution with per-target result tracking (pass/fail/cancelled/errored) and a summary line.

**Architecture:** Context carries execution metadata (parallel flag + target name). A package-level Printer goroutine owns stdout during parallel execution — targets send complete, prefixed lines via a buffered channel. `targ.Print`/`targ.Printf` read context to decide between prefixed channel output and direct stdout. Shell commands get a PrefixWriter as their stdout/stderr. OnStart/OnStop hooks announce lifecycle. Summary line printed after all targets complete.

**Tech Stack:** Go, gomega, rapid (property-based testing)

---

### Task 1: ExecInfo context metadata

**Files:**
- Create: `internal/core/exec_info.go`
- Test: `internal/core/exec_info_test.go`

**Step 1: Write the failing test**

Create `internal/core/exec_info_test.go`:

```go
package core_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestExecInfo(t *testing.T) {
	t.Parallel()

	t.Run("RoundTrip", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		ctx := core.WithExecInfo(context.Background(), core.ExecInfo{
			Parallel: true,
			Name:     "build",
		})

		info, ok := core.GetExecInfo(ctx)
		g.Expect(ok).To(BeTrue())
		g.Expect(info.Parallel).To(BeTrue())
		g.Expect(info.Name).To(Equal("build"))
	})

	t.Run("MissingReturnsFalse", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		_, ok := core.GetExecInfo(context.Background())
		g.Expect(ok).To(BeFalse())
	})

	t.Run("SerialMode", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		ctx := core.WithExecInfo(context.Background(), core.ExecInfo{
			Parallel: false,
			Name:     "test",
		})

		info, ok := core.GetExecInfo(ctx)
		g.Expect(ok).To(BeTrue())
		g.Expect(info.Parallel).To(BeFalse())
		g.Expect(info.Name).To(Equal("test"))
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestExecInfo -v`
Expected: FAIL — `core.ExecInfo`, `core.WithExecInfo`, `core.GetExecInfo` not defined

**Step 3: Write minimal implementation**

Create `internal/core/exec_info.go`:

```go
package core

import "context"

// ExecInfo carries execution metadata through context.
// Context carries data only — no writers or channels.
type ExecInfo struct {
	Parallel bool   // true if running in a parallel group
	Name     string // target name, used as output prefix
}

type execInfoKey struct{}

// WithExecInfo returns a new context carrying the given ExecInfo.
func WithExecInfo(ctx context.Context, info ExecInfo) context.Context {
	return context.WithValue(ctx, execInfoKey{}, info)
}

// GetExecInfo retrieves ExecInfo from context.
// Returns false if no ExecInfo is present (serial execution).
func GetExecInfo(ctx context.Context) (ExecInfo, bool) {
	info, ok := ctx.Value(execInfoKey{}).(ExecInfo)
	return info, ok
}
```

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestExecInfo -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/exec_info.go internal/core/exec_info_test.go
git commit -m "feat(core): add ExecInfo context metadata for parallel output"
```

---

### Task 2: Printer goroutine

**Files:**
- Create: `internal/core/printer.go`
- Test: `internal/core/printer_test.go`

**Step 1: Write the failing test**

Create `internal/core/printer_test.go`:

```go
package core_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestPrinter(t *testing.T) {
	t.Parallel()

	t.Run("SendAndClose", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder
		p := core.NewPrinter(&buf, 10)

		p.Send("[build] compiling...\n")
		p.Send("[test]  running...\n")
		p.Close()

		output := buf.String()
		g.Expect(output).To(ContainSubstring("[build] compiling...\n"))
		g.Expect(output).To(ContainSubstring("[test]  running...\n"))
	})

	t.Run("PreservesOrder", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder
		p := core.NewPrinter(&buf, 10)

		for i := range 100 {
			p.Send(strings.Repeat("x", i) + "\n")
		}
		p.Close()

		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		g.Expect(lines).To(HaveLen(100))
		for i, line := range lines {
			g.Expect(line).To(Equal(strings.Repeat("x", i)))
		}
	})

	t.Run("CloseFlushesAll", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder
		p := core.NewPrinter(&buf, 1) // tiny buffer

		p.Send("line1\n")
		p.Send("line2\n")
		p.Send("line3\n")
		p.Close()

		g.Expect(buf.String()).To(Equal("line1\nline2\nline3\n"))
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestPrinter -v`
Expected: FAIL — `core.NewPrinter` not defined

**Step 3: Write minimal implementation**

Create `internal/core/printer.go`:

```go
package core

import (
	"io"
)

// Printer serializes output from parallel targets through a single goroutine.
// Targets send complete lines to the channel; the printer goroutine writes them
// to the output writer sequentially, guaranteeing line atomicity.
type Printer struct {
	ch   chan string
	done chan struct{}
	out  io.Writer
}

// NewPrinter creates a Printer that writes to out with the given channel buffer size.
// Starts the printer goroutine immediately.
func NewPrinter(out io.Writer, bufSize int) *Printer {
	p := &Printer{
		ch:   make(chan string, bufSize),
		done: make(chan struct{}),
		out:  out,
	}

	go p.run()

	return p
}

// Send queues a line for printing. Blocks only if the channel buffer is full.
func (p *Printer) Send(line string) {
	p.ch <- line
}

// Close drains remaining lines and waits for the printer goroutine to exit.
func (p *Printer) Close() {
	close(p.ch)
	<-p.done
}

func (p *Printer) run() {
	defer close(p.done)

	for line := range p.ch {
		_, _ = io.WriteString(p.out, line)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestPrinter -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/printer.go internal/core/printer_test.go
git commit -m "feat(core): add Printer goroutine for line-atomic parallel output"
```

---

### Task 3: PrefixWriter (io.Writer adapter)

**Files:**
- Create: `internal/core/prefix_writer.go`
- Test: `internal/core/prefix_writer_test.go`

This is the `io.Writer` that shell commands write to. It buffers partial lines, splits on `\n`, and sends each complete line (prefixed) to the Printer.

**Step 1: Write the failing test**

Create `internal/core/prefix_writer_test.go`:

```go
package core_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestPrefixWriter(t *testing.T) {
	t.Parallel()

	t.Run("CompleteLine", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder
		p := core.NewPrinter(&buf, 10)
		w := core.NewPrefixWriter("[build] ", p)

		_, err := w.Write([]byte("compiling...\n"))
		g.Expect(err).ToNot(HaveOccurred())
		p.Close()

		g.Expect(buf.String()).To(Equal("[build] compiling...\n"))
	})

	t.Run("PartialLinesFlushedOnClose", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder
		p := core.NewPrinter(&buf, 10)
		w := core.NewPrefixWriter("[test]  ", p)

		_, _ = w.Write([]byte("partial"))
		w.Flush()
		p.Close()

		g.Expect(buf.String()).To(Equal("[test]  partial\n"))
	})

	t.Run("MultipleLines", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder
		p := core.NewPrinter(&buf, 10)
		w := core.NewPrefixWriter("[build] ", p)

		_, _ = w.Write([]byte("line1\nline2\nline3\n"))
		p.Close()

		g.Expect(buf.String()).To(Equal("[build] line1\n[build] line2\n[build] line3\n"))
	})

	t.Run("ChunkedWrites", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder
		p := core.NewPrinter(&buf, 10)
		w := core.NewPrefixWriter("[x] ", p)

		_, _ = w.Write([]byte("hel"))
		_, _ = w.Write([]byte("lo\nwor"))
		_, _ = w.Write([]byte("ld\n"))
		p.Close()

		g.Expect(buf.String()).To(Equal("[x] hello\n[x] world\n"))
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestPrefixWriter -v`
Expected: FAIL — `core.NewPrefixWriter` not defined

**Step 3: Write minimal implementation**

Create `internal/core/prefix_writer.go`:

```go
package core

import "strings"

// PrefixWriter is an io.Writer that buffers partial lines and sends
// complete, prefixed lines to a Printer. Used as stdout/stderr for
// shell commands in parallel mode.
type PrefixWriter struct {
	prefix  string
	printer *Printer
	buf     strings.Builder
}

// NewPrefixWriter creates a PrefixWriter that prefixes each line and sends to printer.
func NewPrefixWriter(prefix string, printer *Printer) *PrefixWriter {
	return &PrefixWriter{
		prefix:  prefix,
		printer: printer,
	}
}

// Write implements io.Writer. Buffers partial lines, sends complete lines to Printer.
func (w *PrefixWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.buf.Write(p)

	for {
		line, err := w.extractLine()
		if err != nil {
			break
		}

		w.printer.Send(w.prefix + line + "\n")
	}

	return n, nil
}

// Flush sends any remaining partial line to the Printer.
func (w *PrefixWriter) Flush() {
	if w.buf.Len() > 0 {
		w.printer.Send(w.prefix + w.buf.String() + "\n")
		w.buf.Reset()
	}
}

// extractLine extracts one complete line (up to \n) from the buffer.
// Returns error if no complete line is available.
func (w *PrefixWriter) extractLine() (string, error) {
	s := w.buf.String()
	idx := strings.IndexByte(s, '\n')

	if idx < 0 {
		return "", errNoCompleteLine
	}

	line := s[:idx]
	w.buf.Reset()
	w.buf.WriteString(s[idx+1:])

	return line, nil
}

type prefixWriterError string

func (e prefixWriterError) Error() string { return string(e) }

const errNoCompleteLine = prefixWriterError("no complete line")
```

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestPrefixWriter -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/prefix_writer.go internal/core/prefix_writer_test.go
git commit -m "feat(core): add PrefixWriter io.Writer adapter for parallel output"
```

---

### Task 4: targ.Print / targ.Printf

**Files:**
- Create: `internal/core/print.go`
- Test: `internal/core/print_test.go`
- Modify: `targ.go` (re-export)

**Step 1: Write the failing test**

Create `internal/core/print_test.go`:

```go
package core_test

import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestPrint(t *testing.T) {
	t.Parallel()

	t.Run("SerialWritesDirectly", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder
		core.SetPrintOutput(&buf)
		defer core.SetPrintOutput(nil)

		ctx := context.Background()
		core.Print(ctx, "hello world\n")

		g.Expect(buf.String()).To(Equal("hello world\n"))
	})

	t.Run("ParallelPrefixesAndSendsToPrinter", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder
		p := core.NewPrinter(&buf, 10)
		core.SetActivePrinter(p)
		defer core.SetActivePrinter(nil)

		ctx := core.WithExecInfo(context.Background(), core.ExecInfo{
			Parallel: true,
			Name:     "build",
		})

		core.Print(ctx, "compiling...\n")
		p.Close()

		g.Expect(buf.String()).To(Equal("[build] compiling...\n"))
	})

	t.Run("PrintfFormatsCorrectly", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder
		p := core.NewPrinter(&buf, 10)
		core.SetActivePrinter(p)
		defer core.SetActivePrinter(nil)

		ctx := core.WithExecInfo(context.Background(), core.ExecInfo{
			Parallel: true,
			Name:     "test",
		})

		core.Printf(ctx, "result: %d\n", 42)
		p.Close()

		g.Expect(buf.String()).To(Equal("[test]  result: 42\n"))
	})

	t.Run("MultiLineSplitsAndPrefixesEach", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder
		p := core.NewPrinter(&buf, 10)
		core.SetActivePrinter(p)
		defer core.SetActivePrinter(nil)

		ctx := core.WithExecInfo(context.Background(), core.ExecInfo{
			Parallel: true,
			Name:     "lint",
		})

		core.Print(ctx, "line1\nline2\n")
		p.Close()

		g.Expect(buf.String()).To(Equal("[lint]  line1\n[lint]  line2\n"))
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestPrint -v`
Expected: FAIL — `core.Print`, `core.Printf`, `core.SetPrintOutput`, `core.SetActivePrinter` not defined

**Step 3: Write minimal implementation**

Create `internal/core/print.go`:

```go
package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// Package-level state for the active printer (singleton).
//
//nolint:gochecknoglobals // singleton pattern for parallel output coordination
var (
	activePrinter *Printer
	printOutput   io.Writer = os.Stdout
)

// SetActivePrinter sets the package-level printer for parallel output.
// Called by the parallel executor before/after spawning goroutines.
func SetActivePrinter(p *Printer) {
	activePrinter = p
}

// SetPrintOutput sets the default output writer for serial mode.
// Passing nil resets to os.Stdout.
func SetPrintOutput(w io.Writer) {
	if w == nil {
		printOutput = os.Stdout
	} else {
		printOutput = w
	}
}

// Print writes output, prefixed with [name] if running in parallel mode.
func Print(ctx context.Context, args ...any) {
	text := fmt.Sprint(args...)
	info, ok := GetExecInfo(ctx)

	if !ok || !info.Parallel || activePrinter == nil {
		fmt.Fprint(printOutput, text)
		return
	}

	prefix := formatPrefix(info.Name)
	sendPrefixed(prefix, text)
}

// Printf writes formatted output, prefixed with [name] if running in parallel mode.
func Printf(ctx context.Context, format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	info, ok := GetExecInfo(ctx)

	if !ok || !info.Parallel || activePrinter == nil {
		fmt.Fprint(printOutput, text)
		return
	}

	prefix := formatPrefix(info.Name)
	sendPrefixed(prefix, text)
}

// formatPrefix creates a right-padded prefix like "[build] " or "[test]  ".
func formatPrefix(name string) string {
	return fmt.Sprintf("[%s] ", name)
}

// FormatPrefix is exported for use by OnStart/OnStop defaults and PrefixWriter creation.
func FormatPrefix(name string) string {
	return formatPrefix(name)
}

func sendPrefixed(prefix, text string) {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		// Skip empty last element from trailing \n
		if i == len(lines)-1 && line == "" {
			continue
		}

		activePrinter.Send(prefix + line + "\n")
	}
}
```

Add to `internal/core/export_test.go`:

```go
// SetActivePrinterForTest and SetPrintOutputForTest are already exported
// via the public SetActivePrinter/SetPrintOutput functions.
```

Actually, `SetActivePrinter` and `SetPrintOutput` are already exported. The test can use them directly.

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestPrint -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/print.go internal/core/print_test.go
git commit -m "feat(core): add Print/Printf with parallel-aware prefixed output"
```

---

### Task 5: Result type and classification

**Files:**
- Create: `internal/core/result.go`
- Test: `internal/core/result_test.go`

**Step 1: Write the failing test**

Create `internal/core/result_test.go`:

```go
package core_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestResult(t *testing.T) {
	t.Parallel()

	t.Run("StringValues", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.Pass.String()).To(Equal("PASS"))
		g.Expect(core.Fail.String()).To(Equal("FAIL"))
		g.Expect(core.Cancelled.String()).To(Equal("CANCELLED"))
		g.Expect(core.Errored.String()).To(Equal("ERRORED"))
	})

	t.Run("ClassifyNilIsPass", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyResult(nil, false)).To(Equal(core.Pass))
	})

	t.Run("ClassifyContextCanceledNotFirstIsCancelled", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyResult(context.Canceled, false)).To(Equal(core.Cancelled))
	})

	t.Run("ClassifyContextCanceledFirstIsFail", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyResult(context.Canceled, true)).To(Equal(core.Fail))
	})

	t.Run("ClassifyDeadlineExceededIsErrored", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyResult(context.DeadlineExceeded, false)).To(Equal(core.Errored))
	})

	t.Run("ClassifyOtherErrorIsFail", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(core.ClassifyResult(errors.New("boom"), false)).To(Equal(core.Fail))
		g.Expect(core.ClassifyResult(errors.New("boom"), true)).To(Equal(core.Fail))
	})

	t.Run("FormatSummaryNonZeroOnly", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		results := []core.TargetResult{
			{Name: "build", Status: core.Pass},
			{Name: "test", Status: core.Fail},
			{Name: "lint", Status: core.Cancelled},
		}

		g.Expect(core.FormatSummary(results)).To(Equal("PASS:1 FAIL:1 CANCELLED:1"))
	})

	t.Run("FormatSummaryAllPass", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		results := []core.TargetResult{
			{Name: "build", Status: core.Pass},
			{Name: "test", Status: core.Pass},
		}

		g.Expect(core.FormatSummary(results)).To(Equal("PASS:2"))
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestResult -v`
Expected: FAIL — `core.Result`, `core.Pass`, etc. not defined

**Step 3: Write minimal implementation**

Create `internal/core/result.go`:

```go
package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Result represents the outcome of a parallel target execution.
type Result int

// Result values.
const (
	Pass      Result = iota
	Fail
	Cancelled
	Errored
)

// String returns the string representation of the result.
func (r Result) String() string {
	switch r {
	case Pass:
		return "PASS"
	case Fail:
		return "FAIL"
	case Cancelled:
		return "CANCELLED"
	case Errored:
		return "ERRORED"
	default:
		return "UNKNOWN"
	}
}

// TargetResult holds the outcome of a single target in a parallel group.
type TargetResult struct {
	Name     string
	Status   Result
	Duration time.Duration
	Err      error
}

// ClassifyResult determines the Result from an error and whether this was the first failure.
func ClassifyResult(err error, isFirstFailure bool) Result {
	if err == nil {
		return Pass
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return Errored
	}

	if errors.Is(err, context.Canceled) && !isFirstFailure {
		return Cancelled
	}

	return Fail
}

// FormatSummary formats results as a summary line showing only non-zero counts.
func FormatSummary(results []TargetResult) string {
	counts := map[Result]int{}
	for _, r := range results {
		counts[r.Status]++
	}

	var parts []string
	for _, status := range []Result{Pass, Fail, Cancelled, Errored} {
		if c := counts[status]; c > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", status, c))
		}
	}

	return strings.Join(parts, " ")
}
```

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestResult -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/result.go internal/core/result_test.go
git commit -m "feat(core): add Result type with classification and summary formatting"
```

---

### Task 6: OnStart/OnStop hooks on Target

**Files:**
- Modify: `internal/core/target.go:52-75` (Target struct)
- Test: `internal/core/target_test.go` (add test)

**Step 1: Write the failing test**

Add to `internal/core/target_test.go`:

```go
func TestOnStartOnStop(t *testing.T) {
	t.Parallel()

	t.Run("OnStartSetsHook", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var called bool
		target := core.Targ(func() {}).OnStart(func(ctx context.Context, name string) {
			called = true
		})

		hook := target.GetOnStart()
		g.Expect(hook).ToNot(BeNil())
		hook(context.Background(), "test")
		g.Expect(called).To(BeTrue())
	})

	t.Run("OnStopSetsHook", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var got core.Result
		target := core.Targ(func() {}).OnStop(func(ctx context.Context, name string, result core.Result, _ time.Duration) {
			got = result
		})

		hook := target.GetOnStop()
		g.Expect(hook).ToNot(BeNil())
		hook(context.Background(), "test", core.Pass, 0)
		g.Expect(got).To(Equal(core.Pass))
	})

	t.Run("DefaultsAreNil", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := core.Targ(func() {})
		g.Expect(target.GetOnStart()).To(BeNil())
		g.Expect(target.GetOnStop()).To(BeNil())
	})
}
```

Note: the test imports `context` and `time` — add to imports.

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestOnStartOnStop -v`
Expected: FAIL — `OnStart`, `OnStop`, `GetOnStart`, `GetOnStop` not defined

**Step 3: Write minimal implementation**

Add fields to `Target` struct in `internal/core/target.go:52-75`:

```go
	// Lifecycle hooks for parallel output
	onStart func(ctx context.Context, name string)
	onStop  func(ctx context.Context, name string, result Result, duration time.Duration)
```

Add builder methods and getters (append to target.go, before the unexported section):

```go
// OnStart sets a hook that fires when the target begins execution in parallel mode.
func (t *Target) OnStart(fn func(ctx context.Context, name string)) *Target {
	t.onStart = fn
	return t
}

// OnStop sets a hook that fires when the target completes execution in parallel mode.
func (t *Target) OnStop(fn func(ctx context.Context, name string, result Result, duration time.Duration)) *Target {
	t.onStop = fn
	return t
}

// GetOnStart returns the configured OnStart hook, or nil if not set.
func (t *Target) GetOnStart() func(ctx context.Context, name string) {
	return t.onStart
}

// GetOnStop returns the configured OnStop hook, or nil if not set.
func (t *Target) GetOnStop() func(ctx context.Context, name string, result Result, duration time.Duration) {
	return t.onStop
}
```

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestOnStartOnStop -v`
Expected: PASS

**Step 5: Run all tests**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS (no regressions)

**Step 6: Commit**

```bash
git add internal/core/target.go internal/core/target_test.go
git commit -m "feat(core): add OnStart/OnStop lifecycle hooks to Target"
```

---

### Task 7: Wire parallel executors (dep-level + top-level)

This is the main integration task. Modifies `runGroupParallel` in `target.go:449-471`, `executeDefaultParallel` in `run_env.go:282-328`, and `executeMultiRootParallel` in `run_env.go:411-453` to:
1. Create Printer, set as active
2. Inject ExecInfo into context per target
3. Fire OnStart/OnStop hooks (with defaults)
4. Collect TargetResults
5. Print summary line after close

**Files:**
- Modify: `internal/core/target.go:449-471` (runGroupParallel)
- Modify: `internal/core/run_env.go:282-328` (executeDefaultParallel)
- Modify: `internal/core/run_env.go:411-453` (executeMultiRootParallel)
- Test: `internal/core/command_test.go` or new `internal/core/parallel_output_test.go`

**Step 1: Write the failing test**

Create `internal/core/parallel_output_test.go`:

```go
package core_test

import (
	"context"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestParallelOutputDepLevel(t *testing.T) {
	t.Parallel()

	t.Run("ParallelDepsProducePrefixedOutput", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		aCalled := false
		bCalled := false

		a := core.Targ(func(ctx context.Context) {
			core.Print(ctx, "from a\n")
			aCalled = true
		}).Name("a")
		b := core.Targ(func(ctx context.Context) {
			core.Print(ctx, "from b\n")
			bCalled = true
		}).Name("b")
		main := core.Targ(func() {}).Name("main").Deps(a, b, core.DepModeParallel)

		result, err := core.Execute([]string{"app", "main"}, main)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(aCalled).To(BeTrue())
		g.Expect(bCalled).To(BeTrue())

		// Output should contain prefixed lines and summary
		g.Expect(result.Output).To(ContainSubstring("[a]"))
		g.Expect(result.Output).To(ContainSubstring("[b]"))
		g.Expect(result.Output).To(ContainSubstring("from a"))
		g.Expect(result.Output).To(ContainSubstring("from b"))
		g.Expect(result.Output).To(ContainSubstring("PASS:2"))
	})

	t.Run("FailFastReportsCancelledTargets", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() error {
			return errors.New("boom")
		}).Name("a")
		b := core.Targ(func(ctx context.Context) {
			// Simulate slow task that gets cancelled
			select {
			case <-ctx.Done():
			case <-time.After(5 * time.Second):
			}
		}).Name("b")
		main := core.Targ(func() {}).Name("main").Deps(a, b, core.DepModeParallel)

		result, _ := core.Execute([]string{"app", "main"}, main)

		// Should show FAIL for a, CANCELLED for b
		g.Expect(result.Output).To(ContainSubstring("FAIL"))
		g.Expect(result.Output).To(ContainSubstring("CANCELLED"))
	})
}

func TestParallelOutputTopLevel(t *testing.T) {
	t.Parallel()

	t.Run("TopLevelParallelProducesPrefixedOutput", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func(ctx context.Context) {
			core.Print(ctx, "hello\n")
		}).Name("a")
		b := core.Targ(func(ctx context.Context) {
			core.Print(ctx, "world\n")
		}).Name("b")

		result, err := core.ExecuteWithOptions(
			[]string{"app", "--parallel", "a", "b"},
			core.RunOptions{},
			a, b,
		)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(result.Output).To(ContainSubstring("[a]"))
		g.Expect(result.Output).To(ContainSubstring("[b]"))
		g.Expect(result.Output).To(ContainSubstring("PASS:2"))
	})
}
```

Add `"errors"` to the imports.

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run "TestParallelOutput" -v`
Expected: FAIL — output won't contain prefixes or summary (current implementation doesn't have them)

**Step 3: Implement the wiring**

Modify `runGroupParallel` in `internal/core/target.go:449-471`:

```go
func runGroupParallel(ctx context.Context, targets []*Target) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up printer for parallel output
	printer := NewPrinter(printOutput, len(targets)*10)
	prevPrinter := activePrinter
	SetActivePrinter(printer)

	type targetErr struct {
		index int
		err   error
	}

	errs := make(chan targetErr, len(targets))
	results := make([]TargetResult, len(targets))

	for i, dep := range targets {
		results[i].Name = dep.GetName()
		go func(idx int, d *Target) {
			name := d.GetName()
			tctx := WithExecInfo(ctx, ExecInfo{Parallel: true, Name: name})

			// Fire OnStart hook (or default)
			if d.onStart != nil {
				d.onStart(tctx, name)
			} else {
				Print(tctx, "starting...\n")
			}

			start := time.Now()
			err := d.Run(tctx)
			duration := time.Since(start)

			errs <- targetErr{index: idx, err: err}
		}(i, dep)
	}

	var firstErr error
	firstErrIdx := -1
	for range targets {
		te := <-errs
		if te.err != nil && firstErr == nil {
			firstErr = te.err
			firstErrIdx = te.index
			cancel()
		}
		results[te.index].Err = te.err
		results[te.index].Duration = time.Since(time.Now()) // filled below
	}

	// Classify results
	for i := range results {
		isFirst := i == firstErrIdx
		results[i].Status = ClassifyResult(results[i].Err, isFirst)

		name := results[i].Name
		tctx := WithExecInfo(ctx, ExecInfo{Parallel: true, Name: name})
		target := targets[i]

		// Fire OnStop hook (or default)
		if target.onStop != nil {
			target.onStop(tctx, name, results[i].Status, results[i].Duration)
		} else {
			Printf(tctx, "%s (%s)\n", results[i].Status, results[i].Duration.Round(time.Millisecond))
		}
	}

	// Drain printer and print summary
	SetActivePrinter(prevPrinter)
	printer.Close()

	summary := FormatSummary(results)
	if summary != "" {
		fmt.Fprintln(printOutput, "\n"+summary)
	}

	return firstErr
}
```

Note: The duration tracking above is simplified. A proper version should record start time per goroutine and compute duration in the goroutine. Adjust during implementation — the key pattern is:

```go
start := time.Now()
err := d.Run(tctx)
duration := time.Since(start)
errs <- targetResult{index: idx, err: err, duration: duration}
```

Similarly modify `executeDefaultParallel` in `run_env.go:282-328` and `executeMultiRootParallel` in `run_env.go:411-453` with the same pattern: create Printer, inject ExecInfo, collect results, fire hooks, print summary.

**Important**: For `executeDefaultParallel` and `executeMultiRootParallel`, the target name comes from the command name (the arg string), not `GetName()`. The existing code already has access to `cmdName` in the goroutine.

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run "TestParallelOutput" -v`
Expected: PASS

**Step 5: Run all tests**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS (existing parallel tests should still work — they don't assert on output format)

**Step 6: Commit**

```bash
git add internal/core/target.go internal/core/run_env.go internal/core/parallel_output_test.go
git commit -m "feat(core): wire parallel output with prefixing, result tracking, and summary"
```

---

### Task 8: sh.Run integration and public API exports

**Files:**
- Modify: `internal/core/target.go:676-685` (runShellCommand)
- Modify: `targ.go` (re-export Print, Printf, Result, etc.)
- Test: `internal/core/parallel_output_test.go` (add shell command test)

**Step 1: Write the failing test**

Add to `internal/core/parallel_output_test.go`:

```go
func TestParallelOutputShellCommand(t *testing.T) {
	t.Parallel()

	t.Run("ShellCommandOutputIsPrefixed", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ("echo hello-from-shell").Name("echo-test")
		b := core.Targ(func() {}).Name("noop")
		main := core.Targ(func() {}).Name("main").Deps(a, b, core.DepModeParallel)

		result, err := core.Execute([]string{"app", "main"}, main)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(result.Output).To(ContainSubstring("[echo-test]"))
		g.Expect(result.Output).To(ContainSubstring("hello-from-shell"))
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestParallelOutputShellCommand -v`
Expected: FAIL — shell command output goes directly to os.Stdout, not through Printer

**Step 3: Implement shell command integration**

Modify `runShellCommand` in `internal/core/target.go:676-685` to be context-aware:

```go
func runShellCommand(ctx context.Context, cmd string) error {
	info, ok := GetExecInfo(ctx)

	var env *internalsh.ShellEnv
	if ok && info.Parallel && activePrinter != nil {
		prefix := FormatPrefix(info.Name)
		pw := NewPrefixWriter(prefix, activePrinter)
		env = &internalsh.ShellEnv{
			ExecCommand: exec.Command,
			IsWindows:   func() bool { return runtime.GOOS == "windows" },
			Stdin:       os.Stdin,
			Stdout:      pw,
			Stderr:      pw,
			Cleanup:     internalsh.DefaultCleanup(),
		}
	}

	err := internalsh.RunContextWithIO(ctx, env, "sh", []string{"-c", cmd})
	if err != nil {
		return fmt.Errorf("shell command failed: %w", err)
	}

	// Flush any partial line from PrefixWriter
	if env != nil {
		if pw, ok := env.Stdout.(*PrefixWriter); ok {
			pw.Flush()
		}
	}

	return nil
}
```

Note: This requires `DefaultCleanup()` to be exported from the sh package (or use `nil` and let DefaultShellEnv fill it). Check if `defaultCleanup` is already accessible. If not, add a `DefaultCleanup()` export to `internal/sh/sh.go`. Also add `"os/exec"` and `"runtime"` imports to target.go if not already present.

Then add re-exports to `targ.go`:

```go
// Result represents the outcome of a parallel target execution.
type Result = core.Result

// Exported result constants.
const (
	Pass      = core.Pass
	Fail      = core.Fail
	Cancelled = core.Cancelled
	Errored   = core.Errored
)

// Print writes output, prefixed with [name] if running in parallel mode.
func Print(ctx context.Context, args ...any) {
	core.Print(ctx, args...)
}

// Printf writes formatted output, prefixed with [name] if running in parallel mode.
func Printf(ctx context.Context, format string, args ...any) {
	core.Printf(ctx, format, args...)
}
```

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestParallelOutputShellCommand -v`
Expected: PASS

**Step 5: Run all tests**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/core/target.go internal/sh/sh.go targ.go internal/core/parallel_output_test.go
git commit -m "feat: integrate parallel output with shell commands and export public API"
```

---

### Notes for the implementer

**Test command:** Always use `go test -tags sqlite_fts5` for all test runs.

**Test conventions:** Tests use `gomega` (`NewWithT(t)`, `g.Expect(...)`) and `rapid` for property-based testing. Blackbox tests in `package core_test`. Parallel by default.

**Key concern — the `printOutput` global:** In Task 7, `runGroupParallel` uses `printOutput` (which defaults to `os.Stdout`). But when called via `Execute()` in tests, output goes to `ExecuteEnv.output` (a strings.Builder). The Printer needs to write to the _test's_ output buffer, not `os.Stdout`. This means `runGroupParallel` should receive the output writer (from `RunEnv.Stdout()`) rather than using the global. **During implementation, thread `io.Writer` through rather than relying on the global for the Printer's output.** The global `printOutput` is fine for `targ.Print` in serial mode, but the Printer should be constructed with the correct writer.

**Prefix padding:** The design shows `[test]  ` (padded to match `[build] `). During implementation, compute the max target name length in the parallel group and right-pad all prefixes to align. This is a nice-to-have; start without it and add if time permits.

**sh.DefaultCleanup:** If `defaultCleanup` isn't exported from `internal/sh`, add `func DefaultCleanup() *CleanupManager { return defaultCleanup }`. This is a minimal export for internal use.

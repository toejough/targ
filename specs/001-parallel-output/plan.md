# Implementation Plan: Parallel Output

**Branch**: `001-parallel-output` | **Date**: 2026-02-19 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-parallel-output/spec.md`

## Summary

Add prefixed, line-atomic output for parallel target execution with per-target result tracking and summary line. Architecture: context carries execution metadata (parallel flag + target name); a package-level Printer goroutine owns stdout during parallel execution; `targ.Print`/`targ.Printf` read context to choose between prefixed channel output and direct stdout; shell commands get a PrefixWriter as their stdout/stderr; OnStart/OnStop hooks announce lifecycle; summary line printed after all targets complete.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: gomega (assertions), rapid (property-based testing), lipgloss (ANSI styling, existing)
**Storage**: N/A
**Testing**: `go test -tags sqlite_fts5` with gomega + rapid; blackbox tests in `package core_test`
**Target Platform**: macOS/Linux (darwin, POSIX shell)
**Project Type**: Single Go module (`github.com/toejough/targ`)
**Performance Goals**: Line atomicity (zero interleaved lines); fail-fast cancellation within 1s of first failure
**Constraints**: Zero output loss (writes may block); backward compatibility with serial mode (no behavior change)
**Scale/Scope**: ~8 new files, ~3 modified files; affects `internal/core`, `internal/sh`, and root `targ.go`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is not yet configured for this project (template placeholders only). No gates to evaluate. Proceeding.

## Project Structure

### Documentation (this feature)

```text
specs/001-parallel-output/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── api.md           # Public API contract
└── tasks.md             # Phase 2 output (NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/core/
├── exec_info.go          # NEW: ExecInfo context metadata
├── exec_info_test.go     # NEW: ExecInfo tests
├── printer.go            # NEW: Printer goroutine for line-atomic output
├── printer_test.go       # NEW: Printer tests
├── prefix_writer.go      # NEW: io.Writer adapter for shell commands
├── prefix_writer_test.go # NEW: PrefixWriter tests
├── print.go              # NEW: Print/Printf context-aware output
├── print_test.go         # NEW: Print/Printf tests
├── result.go             # NEW: Result type, classification, summary
├── result_test.go        # NEW: Result tests
├── target.go             # MODIFY: Add OnStart/OnStop hooks, wire parallel output
├── run_env.go            # MODIFY: Wire top-level parallel output
└── parallel_output_test.go # NEW: Integration tests

internal/sh/
└── sh.go                 # MODIFY: DefaultCleanup export (if needed)

targ.go                   # MODIFY: Re-export Print, Printf, Result, OnStart, OnStop
```

**Structure Decision**: Single Go module, all new code in `internal/core/` alongside existing execution machinery. Public API re-exported from root `targ.go`. No new packages needed.

## Complexity Tracking

No constitution violations to justify.

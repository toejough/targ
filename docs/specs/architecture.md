# L3: Architecture

Bottom-up derivation from existing code structure and test suite.

## ARCH-1: Target Type System

Traces to: REQ-1, REQ-7, DES-5

The `Target` type is the central abstraction. Created via `Targ(fn)` (function target) or `Targ("cmd")` (shell command target). Builder methods return `*Target` for chaining: `.Name()`, `.Description()`, `.Deps()`, `.Cache()`, `.Watch()`, `.Timeout()`, `.Times()`, `.Retry()`, `.Backoff()`, `.While()`.

`TargetGroup` wraps named groups of targets via `Group(name, members...)`. Groups nest arbitrarily.

`DepGroup` models serial/parallel dependency chains. `.Deps()` calls coalesce same-mode groups; mixed modes create separate groups. `DepMode` enum: Serial (default), Parallel, Mixed.

Key files: `internal/core/target.go`, `internal/core/group.go`

## ARCH-2: Command Parser

Traces to: REQ-2, REQ-3, REQ-12, DES-2, DES-3

Struct fields with `targ:"..."` tags are parsed into CLI flags and positionals. Tag syntax: `targ:"kind,key=value,..."` where kind is `flag` (default) or `positional`. Keys: `name`, `short`, `desc`, `default`, `required`, `enum`, `env`.

Supported field types: bool, int/int64, float64, string, time.Duration, []T (repeated flags), map[K]V (key=value), `Interleaved[T]` (order-preserving), embedded structs. TextUnmarshaler and StringSetter interfaces supported.

Function name → kebab-case command name via CamelToKebab. Override with `.Name()`.

Shell command targets extract `$variable` references as positional fields.

Conflict detection: if a target has a builder setting (e.g., `.Cache()`) AND the user passes the CLI flag (e.g., `--cache`), execution errors unless the target used `targ.Disabled` to opt out.

Key files: `internal/core/command.go`, `internal/core/parse.go`, `internal/parse/`

## ARCH-3: Execution Engine

Traces to: REQ-7, REQ-8, REQ-9, REQ-10, REQ-11, DES-5, DES-6

Executes targets with full lifecycle: resolve deps → check cache → run function → handle errors.

- **Dependencies:** Serial by default. Parallel via `targ.DepModeParallel`. Chained `.Deps()` calls create sequential groups. Parallel failures collected via `MultiError`.
- **Caching:** Glob-based input patterns. Skip execution when files unchanged (checksum-based).
- **Watching:** Glob-based patterns. Re-runs target on file changes.
- **Timeout:** `context.WithTimeout` wrapping. Cancels entire process tree.
- **Retry/Backoff:** `.Times(n)` for iteration count, `.Retry()` to continue on failure, `.Backoff(initial, factor)` for exponential delay.
- **While:** `.While(fn)` predicate loop.

CLI overrides: `--timeout`, `--times`, `--retry`, `--backoff`, `--cache`, `--watch`, `--deps`, `--dep-mode`, `--cache-dir`. Override requires `targ.Disabled` on target or absence of target setting.

Key files: `internal/core/execute.go`, `internal/core/state.go`, `internal/core/override.go`

## ARCH-4: Registry

Traces to: REQ-4, REQ-16, DES-10

Global target registration via `Register(targets...)`. Resolution at execution time detects name conflicts across packages. Error messages include conflicting name, source packages, and suggest `DeregisterFrom`.

`DeregisterFrom(packagePath)` removes all targets from a package. Must be called before resolution. Idempotent. Unknown packages error. Queue cleared after resolution.

Source tracking: local targets (main module) have source cleared; remote targets preserve source package path. Groups track source same as targets.

Key files: `internal/core/registry.go`, `internal/core/source.go`

## ARCH-5: Help Renderer

Traces to: REQ-5, REQ-14, DES-4

Structured help output with ANSI styling (lipgloss). Sections in order: Description → Source → Usage → Positionals → Flags → Subcommands → Execution → Examples.

Empty sections omitted. ANSI codes properly paired. No trailing whitespace. Example code blocks are ANSI-free. Global flags appear before command-specific flags.

Binary mode: hides targ-specific flags (--source, --no-cache, --keep), changes "Global Flags" label to "Flags", examples use binary name.

Builder pattern: `NewBuilder(name)` → `.WithDescription()` → `.WithUsage()` → `.AddPositionals()` → `.AddGlobalFlags()` → `.AddRootOnlyFlags()`.

Auto-generated examples for root and target help. User examples replace auto-generated ones.

Key files: `internal/help/`, `internal/core/print.go`, `internal/core/printer.go`

## ARCH-6: Completion Engine

Traces to: REQ-6, DES-6

Shell completion scripts generated for bash, zsh, fish. `--completion [shell]` flag. Auto-detects shell from environment. Unsupported shells error. Windows path detection.

`__complete` subcommand for runtime suggestions: subcommands, flags, enum values, quoted values. Completion does not execute targets.

Disabled completion (no --completion flag) when target opts out.

Key files: `internal/core/completion.go`

## ARCH-7: Build Tool Runner

Traces to: REQ-15, REQ-16, DES-9, DES-10

The `targ` CLI binary (`cmd/targ/main.go`) delegates to `internal/runner.Run()`.

Subcommands:
- `--create NAME [CMD]` — Scaffolds function or shell command target. Generates `init()` with `targ.Register()`. Supports `--deps`, `--cache`, `--watch`, `--timeout`, `--times`, `--retry`, `--backoff`, `--dep-mode`.
- `--sync PACKAGE` — Adds blank import + `DeregisterFrom` boilerplate. Updates module version.
- `--to-func NAME` — Converts string target to function target.
- `--to-string NAME` — Converts function target to string command.

Code generation: kebab-to-PascalCase conversion. Finds/creates targ files. Adds to existing `Register()` call or creates new `init()`.

Key files: `internal/runner/`, `cmd/targ/main.go`

## ARCH-8: Discovery

Traces to: DES-9

Discovers Go source files with `//go:build targ` tag. Walks directory tree. Skips test files, generated files, hidden dirs, special dirs (vendor, node_modules, .git). Detects `targ.Register()` calls including aliased imports. Extracts package doc. Sorts files alphabetically. Rejects multiple package names in same directory.

Default build tag: `targ`. Default start directory: current directory.

Key files: `internal/discover/discover.go`

## ARCH-9: File Utilities

Traces to: REQ-8, REQ-9

- `Match(patterns...)` — Glob-based file matching with `**` support.
- `Newer(inputs, outputs)` — Modtime comparison. Skip if outputs newer than inputs.
- `Checksum(inputs, dest)` — Content-based change detection via checksums.
- `Watch(ctx, patterns, opts, callback)` — File watcher with context cancellation.

Key files: `internal/file/`

## ARCH-10: Shell Helpers

Traces to: REQ-12

- `Run(name, args...)` / `RunV(name, args...)` — Execute external command, inherit stdout/stderr. V variant prints command first.
- `RunContext(ctx, name, args...)` / `RunContextV(ctx, ...)` — Context-aware variants. Cancellation kills entire process tree.
- `Output(name, args...)` / `OutputContext(ctx, ...)` — Capture stdout.

Key files: `internal/sh/`

## ARCH-11: Public API Facade

Traces to: DES-1

`targ.go` re-exports types and functions from internal packages. Provides the public API surface. Type aliases for `Target`, `TargetGroup`, `DepGroup`, `DepMode`, `Example`, `ExecuteResult`, `ExitError`, `Interleaved[T]`, `MultiError`, `Result`, `RunOptions`, `RuntimeOverrides`, `TagKind`, `TagOptions`, `WatchOptions`, `ChangeSet`.

Two entry points: `Register()` + `ExecuteRegistered()` for build tool mode; `Main()` for binary mode; `Execute()` for testing.

Key files: `targ.go`

## ARCH-12: Source Tracking

Traces to: REQ-4

Runtime caller detection via `runtime.Callers()` to attribute targets to source files and packages. `CallerPackagePath` extracts package from call stack. `ExtractPackagePath` parses package path from file path.

Used for: help output "Source:" line, conflict detection source attribution, local vs remote target distinction.

Key files: `internal/core/source.go`, `internal/core/exec_info.go`

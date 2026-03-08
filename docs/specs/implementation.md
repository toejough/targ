# L5: Implementation

Items derived from: ground truth (existing codebase)

## IMPL-1: Root Public API

**Package:** `targ` (root)
**File:** `targ.go`
**Purpose:** Thin re-export layer providing the public API surface. Type aliases, constant re-exports, and delegating functions to `internal/core`, `internal/file`, and `internal/sh`.
**Key exports:** `Targ()`, `Main()`, `Register()`, `Execute()`, `Group()`, `Run()`, `RunContext()`, `Output()`, `Match()`, `Watch()`, `Checksum()`, `Print()`, `Printf()`
**Traces to:** ARCH-14

## IMPL-2: CLI Entry Point

**Package:** `cmd/targ`
**File:** `cmd/targ/main.go`
**Purpose:** Binary entry point for the `targ` build tool. Calls `runner.Run()`.
**Traces to:** ARCH-12

## IMPL-3: Build Targets (dev)

**Package:** `dev`
**File:** `dev/targets.go` (`//go:build targ`)
**Purpose:** Project-internal build targets — test, lint, coverage, formatting, dead code detection, declaration reordering, fuzzing, mutation testing, etc.
**Key exports:** `Check`, `CheckFull`, `Test`, `Lint`, `Coverage`, `Fmt`, `Tidy`, `Deadcode`, `ReorderDecls`, `Fuzz`, `Mutate`, `Watch`
**Traces to:** project tooling — no direct ARCH trace

## IMPL-4: Target Definition and Builder

**Package:** `internal/core`
**Files:** `target.go`, `types.go`
**Purpose:** The `Target` struct and its builder methods (`.Name()`, `.Description()`, `.Deps()`, `.Cache()`, `.Watch()`, `.Timeout()`, `.Times()`, `.Retry()`, `.Backoff()`, `.While()`, `.OnStart()`, `.OnStop()`). Dependency mode types (`DepMode`, `DepGroup`). `Targ()` constructor accepting functions or shell command strings.
**Key types:** `Target`, `DepMode`, `DepGroup`, `DepOption`, `TagOptions`, `TagKind`
**Traces to:** ARCH-1

## IMPL-5: Command Parsing and Execution

**Package:** `internal/core`
**Files:** `command.go`, `run_env.go`, `execute.go`, `override.go`, `parse.go`
**Purpose:** CLI argument parsing, command resolution, target execution with runtime overrides (`--times`, `--timeout`, `--watch`, `--cache`, `--parallel`). Conflict detection between compile-time config and CLI flags. `Execute()` and `ExecuteWithOptions()` for programmatic/test usage. `RunEnv` interface for testable I/O. `Main()` for standalone binaries.
**Key functions:** `Execute()`, `ExecuteWithOptions()`, `Main()`, `ExecuteRegistered()`, `RunWithEnv()`, `ExtractOverrides()`
**Traces to:** ARCH-2, ARCH-3, ARCH-15

## IMPL-6: Target Registry

**Package:** `internal/core`
**Files:** `registry.go`, `state.go`
**Purpose:** Global mutable registry for targets registered via `init()`. `RegistryState` manages registration, deregistration (by package path), conflict detection (duplicate names), and resolution. Supports `DeregisterFrom()` for remote target management.
**Key types:** `RegistryState`, `Conflict`, `ConflictError`, `DeregistrationError`
**Traces to:** ARCH-4

## IMPL-7: Target Groups

**Package:** `internal/core`
**File:** `group.go`
**Purpose:** `TargetGroup` — named collections of targets for hierarchical CLI organization (e.g., `targ dev lint fast`). `Group()` constructor accepts targets and nested groups.
**Key types:** `TargetGroup`
**Traces to:** ARCH-5

## IMPL-8: Result Handling

**Package:** `internal/core`
**File:** `result.go`
**Purpose:** Execution result classification (`Pass`, `Fail`, `Cancelled`, `Errored`), `MultiError` for parallel failures, `FormatSummary()` and `FormatDetailedSummary()` for human-readable output.
**Key types:** `Result`, `MultiError`, `TargetResult`
**Traces to:** ARCH-6

## IMPL-9: Parallel Output

**Package:** `internal/core`
**Files:** `exec_info.go`, `print.go`, `prefix_writer.go`, `printer.go`
**Purpose:** Prefixed output for parallel target execution. `PrefixWriter` prepends target name to each line. `Printer` buffers and serializes output from concurrent targets. `ExecInfo` context propagation for serial/parallel mode detection. `Print()`/`Printf()` automatically route through printer in parallel mode.
**Key types:** `PrefixWriter`, `Printer`, `ExecInfo`
**Traces to:** ARCH-7

## IMPL-10: Completion

**Package:** `internal/core`
**File:** `completion.go`
**Purpose:** Shell completion script generation for bash, zsh, and fish. Supports commands, subcommands, flags, and enum values.
**Key function:** `PrintCompletionScriptTo()`
**Traces to:** ARCH-10

## IMPL-11: Git Utilities

**Package:** `internal/core`
**File:** `git.go`
**Purpose:** Git integration — clean work tree check, repo URL detection from `.git/config`, URL normalization. Used for help text (source links) and `CheckCleanWorkTree()` target precondition.
**Key functions:** `CheckCleanWorkTree()`, `DetectRepoURL()`, `NormalizeGitURL()`, `ParseGitConfigContent()`
**Traces to:** ARCH-13

## IMPL-12: Source Location

**Package:** `internal/core`
**File:** `source.go`
**Purpose:** Runtime caller detection to determine source file locations for targets. Used to show "Source: dev/targets.go:42" in help output.
**Traces to:** ARCH-13

## IMPL-13: Target Discovery

**Package:** `internal/discover`
**File:** `internal/discover/discover.go`
**Purpose:** Discovers `//go:build targ` files in a Go project. Scans directories for tagged files, detects `targ.Register()` calls and aliased imports. Used by the build tool runner to find target definitions.
**Key functions:** `Discover()`, `TaggedFiles()`
**Key types:** `PackageInfo`, `TaggedFile`, `Options`
**Traces to:** ARCH-11

## IMPL-14: File Utilities

**Package:** `internal/file`
**Files:** `match.go`, `checksum.go`, `watch.go`
**Purpose:** File operations — glob matching (fish-style `**` and `{a,b}`), content-based checksumming for cache invalidation, and file watching with polling.
**Key functions:** `Match()`, `Checksum()`, `Watch()`
**Key types:** `ChangeSet`, `WatchOptions`, `FileOps`
**Traces to:** ARCH-9

## IMPL-15: Flag Definitions

**Package:** `internal/flags`
**Files:** `flags.go`, `placeholders.go`
**Purpose:** Registry of all built-in CLI flags (`--no-cache`, `--keep`, `--create`, `--completion`, `--sync`, `--to-func`, `--to-string`, `--source`, `--times`, `--timeout`, `--watch`, `--cache`, `--parallel`, `--retry`, `--backoff`). Flag mode classification (targ-only vs. binary mode). Placeholder definitions.
**Key types:** `Def`, `FlagMode`, `Placeholder`
**Traces to:** ARCH-10

## IMPL-16: Help System

**Package:** `internal/help`
**Files:** `builder.go`, `content.go`, `render.go`, `generators.go`, `styles.go`, `render_helpers.go`
**Purpose:** Help text generation and rendering. Fluent `ContentBuilder` API for composing help output with sections (flags, positionals, subcommands, examples, execution info). ANSI styling via lipgloss. Root and target help generators. Binary mode support (different flag visibility).
**Key types:** `Builder`, `ContentBuilder`, `RootHelpOpts`, `TargetHelpOpts`, `Styles`
**Key functions:** `WriteRootHelp()`, `WriteTargetHelp()`, `StripANSI()`
**Traces to:** ARCH-10

## IMPL-17: Parse Utilities

**Package:** `internal/parse`
**File:** `internal/parse/parse.go`
**Purpose:** Low-level parsing utilities — struct tag parsing (`ReflectTag`), `CamelToKebab` name conversion, build tag detection, Go source file filtering, targ import detection.
**Key functions:** `CamelToKebab()`, `HasBuildTag()`, `IsGoSourceFile()`, `TargImportInfo()`
**Key types:** `ReflectTag`
**Traces to:** ARCH-2

## IMPL-18: Build Tool Runner

**Package:** `internal/runner`
**File:** `internal/runner/runner.go`
**Purpose:** The build tool's main logic — discovers targets, compiles a bootstrap binary, executes it. Handles `--create` (target scaffolding), `--sync` (remote targets), `--to-func`/`--to-string` (target conversion), `--source` (custom targ file location). Code generation for bootstrap files. Import management for targ files.
**Key functions:** `Run()`, `FindOrCreateTargFile()`, `AddTargetToFileWithOptions()`, `AddImportToTargFile()`, `ConvertStringTargetToFunc()`, `ConvertFuncTargetToString()`, `ExtractTargFlags()`
**Key types:** `CreateOptions`, `SyncOptions`, `TargFlags`, `FileOps`
**Traces to:** ARCH-12

## IMPL-19: Shell Execution

**Package:** `internal/sh`
**Files:** `sh.go`, `context.go`, `cleanup.go`, `context_unix.go`, `context_windows.go`, `cleanup_unix.go`, `cleanup_windows.go`
**Purpose:** Command execution abstraction — `Run()`, `RunV()`, `Output()` for basic execution. Context-aware variants (`RunContextWithIO()`, `OutputContext()`) with process group management for cancellation. `CleanupManager` for SIGINT/SIGTERM handling — kills all spawned child processes. Platform-specific process group and kill implementations (Unix vs Windows).
**Key functions:** `Run()`, `RunV()`, `Output()`, `RunContextWithIO()`, `OutputContext()`, `EnableCleanup()`, `KillProcessGroup()`
**Key types:** `CleanupManager`, `ShellEnv`, `SafeBuffer`
**Traces to:** ARCH-8

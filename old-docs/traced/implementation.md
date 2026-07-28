# L4: Implementation

Items derived from: ground truth (existing codebase)

## IMPL-1: Root Public API

**Package:** `targ` (root)
**File:** `targ.go`
**Purpose:** Thin re-export layer providing the public API surface. Type aliases, constant re-exports, and delegating functions to `internal/core`, `internal/file`, and `internal/sh`.
**Key exports:** `Targ()`, `Main()`, `Register()`, `Execute()`, `Group()`, `Cmd()`, `Run()`, `RunV()`, `RunContext()`, `RunContextV()`, `Output()`, `OutputContext()`, `Match()`, `Watch()`, `Checksum()`, `Print()`, `Printf()`
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
**Files:** `command.go`, `cmd.go`, `run_env.go`, `execute.go`, `override.go`, `parse.go`
**Purpose:** CLI argument parsing, command resolution, target execution with runtime overrides (`--times`, `--timeout`, `--watch`, `--cache`, `--parallel`, `--retry`, `--backoff`, `--dep-mode`). Conflict detection between compile-time config and CLI flags — scope of that detection superseded, see `openspec/specs/execution-engine/spec.md`. `Execute()` and `ExecuteWithOptions()` for programmatic/test usage. `RunEnv` interface for testable I/O. `MainWithSkip()` for standalone binaries (the public `targ.Main()` wraps it). `cmd.go` holds the `Command` builder (`Cmd()`); `RunContext()`, `RunContextV()` and `OutputContext()` all delegate to it — whether it's the only construction path for a context-aware subprocess is superseded, see `openspec/specs/process-execution/spec.md`.
**Key functions:** `Execute()`, `ExecuteWithOptions()`, `MainWithSkip()`, `ExecuteRegistered()`, `RunWithEnv()`, `ExtractOverrides()`, `Cmd()`, `RunContext()`, `RunContextV()`, `OutputContext()`
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
**Purpose:** Execution result classification (`Pass`, `Fail`, `Cancelled`, `Errored`), `MultiError` for CollectAllErrors failures (parallel or serial), `FormatSummary()` and `FormatDetailedSummary()` for human-readable output.
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
**Purpose:** Runtime caller detection to determine source file locations for targets. What's shown from that in help output — superseded, see `openspec/specs/help-rendering/spec.md`.
**Traces to:** ARCH-13

## IMPL-13: Target Discovery

**Package:** `internal/discover`
**File:** `internal/discover/discover.go`
**Purpose:** Discovers `//go:build targ` files in a Go project. Scans directories bidirectionally: downward (CWD non-recursive + CWD/dev/ recursive) and upward (linear ancestor path, stopping before filesystem root to avoid walking system directories like `/dev`, checking each ancestor directory non-recursively and its `dev/` subtree recursively). `TaggedFiles` mirrors the same scoping as `Discover`. Detects `targ.Register()` calls and aliased imports. Used by the build tool runner to find target definitions.
**Key functions:** `Discover()`, `TaggedFiles()`, `findAncestorTaggedDirs()`
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
**Purpose:** Registry of all built-in CLI flags (`--completion`, `--help`, `--source`, `--timeout`, `--parallel`, `--times`, `--retry`, `--backoff`, `--watch`, `--cache`, `--while`, `--dep-mode`, `--no-binary-cache`, `--create`, `--sync`, `--to-func`, `--to-string`, `--init`, `--alias`, `--move`, plus the deprecated `--no-cache` alias). Flag mode classification (targ-only vs. binary mode). Placeholder definitions.
**Caveat:** `--dep-mode` (`serial`|`parallel`) overrides how a target's dependency groups execute; the override flattens all dependency groups into a single group. The `CollectAllErrors` option survives the flatten (any group having it sets it on the flattened group) — e.g. `--dep-mode serial` on `check-full` runs the legs one at a time and still reports all failures.
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
**Purpose:** The build tool's main logic — discovers targets, compiles a bootstrap binary, executes it. Handles `--create` (target scaffolding), `--sync` (remote targets), `--to-func`/`--to-string` (target conversion), `--source` (custom targ file location). Code generation for bootstrap files. Import management for targ files. Pseudo-module handling: `groupByModule` uses each package's directory as module root (not CWD); `prepareBuildContext` creates isolated build directories for pseudo-modules via `createIsolatedBuildDir`; `resolveModuleForBuild` remaps package infos to the isolated directory.
**Key functions:** `Run()`, `FindOrCreateTargFile()`, `AddTargetToFileWithOptions()`, `AddImportToTargFile()`, `ConvertStringTargetToFunc()`, `ConvertFuncTargetToString()`, `ExtractTargFlags()`
**Key types:** `CreateOptions`, `SyncOptions`, `TargFlags`, `FileOps`
**Traces to:** ARCH-12

## IMPL-19: Shell Execution

**Package:** `internal/sh`
**Files:** `sh.go`, `context.go`, `cleanup.go`, `context_unix.go`, `context_windows.go`, `cleanup_unix.go`, `cleanup_windows.go`
**Purpose:** Command execution abstraction — `Run()`, `RunV()`, `Output()` for basic execution. Context-aware variants (`RunContextWithIO()`, `RunContextV()`, `OutputContext()`) with process group management for cancellation. Each takes an `envv []string` setting the child's environment; `nil` inherits the parent's, which is what every caller that declares no environment passes. `ShellEnv.Foreground` controls process group behavior: when true (default for real OS stdio), child inherits parent's foreground process group for interactive TTY access; when false (parallel execution with redirected IO), child gets its own process group via `Setpgid` for clean cancellation of process trees. `CleanupManager` for SIGINT/SIGTERM handling — kills all spawned child processes. Platform-specific process group and kill implementations (Unix vs Windows).
**Key functions:** `Run()`, `RunV()`, `Output()`, `RunContextWithIO()`, `RunContextV()`, `OutputContext()`, `EnableCleanup()`, `KillProcessGroup()`
**Key types:** `CleanupManager`, `ShellEnv`, `SafeBuffer`
**Traces to:** ARCH-8

## IMPL-20: Superseding Logic

**Package:** `internal/runner`
**Files:** `runner.go`, `export_test.go`, `supersede_test.go`
**Purpose:** When the same command name exists at multiple discovery levels, the most-local version (closest to CWD) wins dispatch. `cmdEntry` carries source and superseding metadata. `collectSortedCommands` uses `annotateSuperseding` and `buildPrimarySourceMap` to detect duplicates and annotate primary/superseded entries. `writeCommandList` renders superseding annotations in help output. `groupByModule` preserves discovery proximity order (insertion order) instead of alphabetical sort.
**Key functions:** `annotateSuperseding()`, `buildPrimarySourceMap()`, `writeCommandList()`, `collectSortedCommands()`, `groupByModule()`
**Key types:** `cmdEntry` (with `source`, `supersededBy`, `supersedes` fields)
**Traces to:** ARCH-16

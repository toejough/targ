# L2: Architecture

Items induced from: L3 test items + L4 implementation items (bottom-up)

## ARCH-1: Target Data Model

The `Target` struct is the core data type. Created via `Targ(fn)` or `Targ("shell cmd")`. Builder methods configure: name, description, dependencies (serial/parallel/mixed with chained `.Deps()` calls), cache patterns, watch patterns, timeout, times, retry, backoff, while-predicate, lifecycle hooks (OnStart/OnStop). `DepMode` (Serial/Parallel/Mixed), `DepGroup`, `DepOption` (CollectAllErrors) control dependency execution. Immutable after construction.

**Induced from:** IMPL-4, T-1
**Traces to:** REQ-1, DES-3, DES-5

## ARCH-2: Struct-Tag CLI Parsing

Struct fields with `targ:"..."` tags define CLI arguments. Supported tag keys: `flag`, `positional`, `name`, `short`, `desc`, `env`, `default`, `enum`, `required`, `placeholder`. Function parameter struct is reflected at runtime to build the argument parser. CamelCase field names convert to kebab-case flag names. Supports: bool/int/float64/string flags, slice flags (repeated), map flags (key=value), embedded structs, `Interleaved[T]` for order-preserving repeated flags, `TextUnmarshaler` types. `TagOptions` interface allows runtime override of tag values.

**Induced from:** IMPL-5, IMPL-17, T-2, T-14
**Traces to:** REQ-1, REQ-2

## ARCH-3: Command Resolution and Execution

`Execute(args, targets...)` is the main entry point. Resolves command name from args[1], looks up target in flat list or group hierarchy, parses remaining args against the target's struct tags, invokes the target function. `RuntimeOverrides` extract `--times`, `--timeout`, `--watch`, `--cache`, `--parallel`, `--retry`, `--backoff`, `--dep-mode` from args before target-specific parsing. Conflict detection: if a target has compile-time config (e.g., `.Cache("**/*.go")`) and CLI provides the same flag, error unless `targ.Disabled` was used. `RunEnv` interface abstracts I/O for testability. `Main()` wraps `Execute` with `os.Args` and `os.Exit`. A module registering exactly one target dispatches through the same serial path (`executeRoots`), with the sole root's name prepended. `explicit=!hasDefault` distinguishes whether an unmatched trailing arg errors immediately (default mode) or is handed back for root-chaining (multi-root mode). Parallel mode (`-p`) resolves each arg to a `parallelUnit`: glob first, then an explicit root match, then (default mode only) the sole root's subcommand. One `parallelCap`-bounded fan-out runs the resulting units, shared by both modes.

**Induced from:** IMPL-5, T-3, T-16, T-17, T-18
**Traces to:** REQ-1, REQ-3, REQ-4, DES-1, DES-2

## ARCH-4: Target Registry

Global mutable `RegistryState` stores targets registered via `Register()` in `init()`. `DeregisterFrom(packagePath)` removes all targets from a package. Conflict detection: duplicate target names across packages produce `ConflictError`. Resolution phase finalizes registry before execution. Used by build tool mode (targets register themselves) and remote targets (`--sync`).

**Induced from:** IMPL-6, T-4
**Traces to:** REQ-5

## ARCH-5: Command Hierarchy

`TargetGroup` wraps named collections of targets and nested groups, forming a tree. `Group("name", members...)` creates groups. Command resolution walks the tree: `targ dev lint fast` → Group("dev") → Group("lint") → Target("fast"). Glob patterns match multiple targets within groups.

**Induced from:** IMPL-7, T-5, T-17
**Traces to:** REQ-6

## ARCH-6: Result Classification and Reporting

`Result` enum: Pass, Fail, Cancelled, Errored. `ClassifyResult()` maps error types: nil→Pass, context.Canceled→Cancelled (or Fail if first), DeadlineExceeded→Errored, other→Fail. `MultiError` collects all failures from CollectAllErrors mode (parallel or serial). `FormatSummary()` and `FormatDetailedSummary()` produce human-readable output with per-target status, truncated error snippets.

**Induced from:** IMPL-8, T-6
**Traces to:** REQ-7, DES-5

## ARCH-7: Parallel Output System

When targets run in parallel, each target's output is prefixed with its name. `PrefixWriter` wraps an `io.Writer`, prepending `[target-name] ` to each line. `Printer` serializes output from concurrent goroutines — each target sends output via `Send()`, printer drains in order on `Close()`. `ExecInfo` propagated via context to detect serial/parallel mode. `Print()`/`Printf()` auto-route: direct to stdout in serial, through printer in parallel. Prefix alignment pads names to max length.

**Induced from:** IMPL-9, T-7, T-19
**Traces to:** REQ-8, DES-5

## ARCH-8: Shell Execution

`Run()`, `RunV()`, `Output()` execute external commands. Context-aware variants (`RunContextWithIO()`, `OutputContext()`) support cancellation via process group kill. `ShellEnv` configures stdout/stderr/stdin routing and a `Foreground` flag: when true (default for real OS stdio), the child inherits the parent's foreground process group so interactive programs (e.g., `less`, `claude`) can read/write the terminal; when false (parallel execution with redirected IO), the child gets its own process group via `Setpgid` for clean cancellation of process trees. `CleanupManager` registers spawned processes and kills them on SIGINT/SIGTERM. Platform-specific: Unix uses process groups, Windows uses job objects. `SafeBuffer` provides thread-safe output capture.

**Induced from:** IMPL-19, T-3 (indirect), T-25
**Traces to:** REQ-9

## ARCH-9: File Utilities

`Match()` expands fish-style globs (`**`, `{a,b}`). `Checksum()` computes content hash of matched files, stores at dest path, reports changed-or-not. `Watch()` polls for file changes, invokes callback with `ChangeSet` (created/modified/deleted). Used by cache invalidation and watch mode.

**Induced from:** IMPL-14
**Traces to:** REQ-10

## ARCH-10: Help System

Fluent `ContentBuilder` API composes help output in sections: description, usage, positionals, flags (command, global, root-only, targ-filtered), subcommands, groups, examples, execution info, source file, more-info link. `WriteRootHelp()` and `WriteTargetHelp()` are top-level generators. ANSI styling via lipgloss `Styles`. `StripANSI()` for width calculation. `FlagMode` (FlagModeAll/FlagModeTargOnly) controls flag visibility — binary mode hides targ-specific flags. Auto-generated examples for root and target help.

**Induced from:** IMPL-10, IMPL-15, IMPL-16, T-8, T-12, T-13
**Traces to:** REQ-11, REQ-12

## ARCH-11: Target Discovery

`Discover()` scans directories for `//go:build targ` files in two directions. **Downward:** CWD is checked non-recursively; CWD/dev/ is walked recursively (a full recursive walk from CWD would hang on large directories like `~`). **Upward:** linear walk from start directory, stopping before the filesystem root (to avoid system directories like `/dev`); at each ancestor, checks the ancestor directory itself and recursively walks `<ancestor>/dev/` if it exists. No sibling discovery — only the direct ancestor path. Detects `targ.Register()` calls and aliased imports via AST-level inspection. Returns `PackageInfo` with file paths, package name, and registration status. Used by the build tool runner to locate target definitions before compilation.

**Induced from:** IMPL-13, T-11
**Traces to:** REQ-13, REQ-17, DES-6

## ARCH-12: Build Tool Runner

`Run()` is the `cmd/targ` main logic. Workflow: discover target files (downward and upward) → group by module (`groupByModule`) → build per module group: ancestors with `go.mod` use normal module build, ancestors without `go.mod` use isolated build (synthetic `go.mod`, copied files) → aggregate commands across binaries → execute. Handles meta-commands: `--create` (scaffold targets with code generation), `--sync` (import remote module targets), `--to-func`/`--to-string` (convert target types), `--source` (custom targ file location). `FindOrCreateTargFile()` creates `dev/targets.go` if missing. Import management adds/checks imports in targ files.

**Induced from:** IMPL-2, IMPL-18, T-15
**Traces to:** REQ-13, REQ-16, DES-1, DES-4

## ARCH-13: Git and Source Integration

`CheckCleanWorkTree()` verifies no uncommitted changes (used as target precondition). `DetectRepoURL()` parses `.git/config` for origin URL, normalizes various formats (SSH, HTTPS, .git suffix). Source location detection uses `runtime.Caller()` to find target definition file:line for help output.

**Induced from:** IMPL-11, IMPL-12, T-9, T-10
**Traces to:** REQ-14, REQ-15

## ARCH-14: Public API Surface

Root `targ` package is a thin re-export layer: type aliases, constant re-exports, and delegating functions to `internal/*`. No logic — ensures stable public API independent of internal refactoring. `internal/` enforces Go visibility rules.

**Induced from:** IMPL-1
**Traces to:** DES-1

## ARCH-16: Superseding Logic

When multiple module registries contain commands with the same name, the most-local version (closest to CWD) takes priority. Superseding is determined by discovery order — `Discover()` returns CWD targets first, then CWD/dev/ targets, then ancestors in ascending distance order. `groupByModule` must preserve this proximity ordering (not alphabetical). `collectSortedCommands` annotates duplicates: the primary (most-local) entry notes what it supersedes, and higher-level duplicates are marked as superseded. `findCommandBinary` returns the most-local match. `cmdEntry` carries source information (module root path) for display. `printCommandList` renders superseding annotations with dimmed styling for superseded entries.

**Derived from:** REQ-18, DES-7
**Traces to:** REQ-18, DES-7

## ARCH-15: Conflict-Free Configuration

**Source: standard** — induced from the "No Surprises" design principle observed in IMPL-5 and T-3.
**Rationale:** When both compile-time target config and CLI flags specify the same setting (cache, watch, deps), targ errors rather than silently choosing one. `targ.Disabled` sentinel explicitly opts into CLI flag control. This prevents configuration ambiguity.

**Induced from:** IMPL-5, T-3
**Traces to:** REQ-3, REQ-4

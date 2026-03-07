# L3 Architecture: targ

Bottom-up adoption of the actual package structure. Each ARCH item maps to one package/component.

## Package Dependency Graph

```
cmd/targ
  └── internal/runner
        ├── internal/discover
        │     └── internal/parse
        ├── internal/flags
        └── internal/help
              └── internal/flags

targ.go (root public API)
  ├── internal/core
  │     └── internal/sh (via RunContext/RunContextV)
  ├── internal/file
  └── internal/sh

internal/core
  ├── internal/file (via checksum/match in cache)
  └── internal/sh (via shell command execution)
```

---

## ARCH-1: Root public API (`targ.go`)

**Traces to:** REQ-1-1, REQ-1-2, REQ-1-3, REQ-1-5, REQ-1-6, REQ-3-1, REQ-4-1, REQ-4-4, REQ-5-1, REQ-13-1, REQ-13-2, REQ-13-3, REQ-13-4, REQ-14-1, REQ-14-3, REQ-15-1, DES-1-1, DES-1-2

**Key types/functions:**
- `Targ(fn ...any) *Target` — target creation facade
- `Main(targets ...any)` — standalone binary entry point
- `Register(targets ...any)` — global registry registration
- `ExecuteRegistered()` / `ExecuteRegisteredWithOptions(opts)` — run registered targets
- `Execute(args, targets...)` / `ExecuteWithOptions(args, opts, targets...)` — test-friendly execution
- `Group(name, members...)` — named group creation
- `Run`, `RunV`, `RunContext`, `RunContextV`, `Output`, `OutputContext` — shell execution
- `Match(patterns...)` — glob matching
- `Checksum(inputs, dest)` — content hashing
- `Watch(ctx, patterns, opts, callback)` — file watching
- `Print(ctx, args...)`, `Printf(ctx, format, args...)` — parallel-aware output
- `EnableCleanup()` — signal-based process cleanup
- `DeregisterFrom(packagePath)` — remote target deregistration
- `ExeSuffix()`, `WithExeSuffix(name)`, `IsWindows()` — platform helpers
- Re-exported types: `Target`, `TargetGroup`, `DepMode`, `DepOption`, `DepGroup`, `Result`, `ExecuteResult`, `ExitError`, `MultiError`, `TagKind`, `TagOptions`, `Interleaved[T]`, `ChangeSet`, `WatchOptions`, `RunOptions`, `RuntimeOverrides`, `Example`
- Re-exported constants: `Disabled`, `Parallel` (via `DepModeParallel`), `CollectAllErrors`, `Pass`, `Fail`, `Errored`, `Cancelled`, `TagKindFlag`, `TagKindPositional`, `TagKindUnknown`
- Re-exported errors: `ErrEmptyDest`, `ErrNoInputPatterns`, `ErrNoPatterns`, `ErrUnmatchedBrace`

**Dependencies:** ARCH-2 (internal/core), ARCH-8 (internal/file), ARCH-9 (internal/sh)

**Source files:** `targ.go`

---

## ARCH-2: internal/core — Target definition, execution engine, registry

**Traces to:** REQ-1-1 through REQ-1-13, REQ-2-1 through REQ-2-12, REQ-3-1 through REQ-3-6, REQ-4-1 through REQ-4-5, REQ-5-1, REQ-5-2, REQ-5-7, REQ-5-8, REQ-6-7 through REQ-6-10, REQ-7-1 through REQ-7-4, REQ-8-4, REQ-8-5, REQ-8-6, REQ-11-1 through REQ-11-11, REQ-12-1 through REQ-12-5, DES-1-1, DES-1-2, DES-2-1, DES-2-2, DES-3-1, DES-6-3, DES-7-1, DES-8-2, DES-11-1, DES-11-2, DES-11-3, DES-12-1, DES-12-2, DES-12-3

**Key types/functions:**
- `Target` — target definition with builder methods (`Name`, `Description`, `Deps`, `Cache`, `CacheDir`, `Watch`, `Timeout`, `Times`, `Retry`, `Backoff`, `While`, `Examples`)
- `TargetGroup` — named group of targets/groups
- `Targ(fn ...any)`, `Group(name, members...)` — constructors
- `Main(targets...)`, `ExecuteRegistered()`, `ExecuteWithResolution(env, opts)` — execution entry points
- `RunEnv` interface / `osRunEnv` / `ExecuteEnv` — environment abstraction
- `commandNode`, `parseTarget`, `parseGroupLike`, `executeGroupWithParents` — command tree and execution
- `ExtractOverrides`, `ExecuteWithOverrides`, `checkConflicts` — runtime override system
- `RegisterTarget`, `DeregisterFrom`, `resolveRegistry`, `detectConflicts` — global registry
- `PrintCompletionScriptTo`, `doCompletion`, `tokenizeCommandLine` — shell completion
- `Printer`, `PrefixWriter`, `ExecInfo` — parallel output serialization
- `Print`, `Printf`, `FormatPrefix` — parallel-aware output
- `RunContext`, `RunContextV` — context-aware shell execution (delegates to sh)
- `TagOptions`, `TagKind`, `Interleaved[T]` — CLI argument types
- `Result`, `MultiError`, `ExitError`, `ExecuteResult` — result types
- `CheckCleanWorkTree` — git utility

**Dependencies:** ARCH-8 (internal/file — for cache checksums), ARCH-9 (internal/sh — for shell command execution)

**Source files:** `target.go`, `command.go`, `execute.go`, `override.go`, `run_env.go`, `registry.go`, `group.go`, `completion.go`, `print.go`, `printer.go`, `prefix_writer.go`, `exec_info.go`, `types.go`, `result.go`, `state.go`, `source.go`, `parse.go`, `git.go`, `doc.go`

---

## ARCH-3: internal/runner — CLI tool orchestration

**Traces to:** REQ-8-1 through REQ-8-3, REQ-9-1 through REQ-9-6, REQ-10-1 through REQ-10-10, DES-8-1, DES-9-1, DES-9-2, DES-10-1, DES-10-2

**Key types/functions:**
- `Run() int` — main entry point for the `targ` CLI tool
- `Discover` integration — calls `discover.Discover()` for tagged file discovery
- `groupByModule()` — groups packages by Go module for compilation
- `handleSingleModule()`, `handleMultiModule()`, `handleIsolatedModule()` — three-path compilation
- Bootstrap generation — `main.go` template importing discovered packages
- Binary caching — `tryRunCached()`, content-hash cache keys
- `ExtractTargFlags()` — separates targ flags from target args
- `handleSyncFlag()` — remote target sync (`--sync`)
- `ParseSyncArgs()`, `ParseCreateArgs()` — argument parsing
- `AddImportToTargFileWithFileOps()` — AST-based import injection
- `AddTargetToFileWithFileOps()` — target code generation and file insertion
- `ConvertFuncTargetToString()`, `ConvertStringTargetToFunc()` — format conversion
- `FindOrCreateTargFileWithFileOps()` — targ file location/creation
- `CreateGroupMemberPatch()` — group modification for nested targets
- `ContentPatch`, `CreateOptions`, `FileOps` — supporting types

**Dependencies:** ARCH-4 (internal/discover), ARCH-6 (internal/help), ARCH-7 (internal/flags)

**Source files:** `runner.go`

---

## ARCH-4: internal/discover — File discovery

**Traces to:** REQ-10-1, REQ-10-2, REQ-10-10, DES-10-2

**Key types/functions:**
- `Discover(opts Options) ([]PackageInfo, error)` — BFS directory walk for tagged files
- `Options` — discovery config (`StartDir`, `BuildTag`, `FileSystem`)
- `PackageInfo` — discovered package info (`Dir`, `PackageName`, `DocComment`, `Files`, `ExplicitRegistration`)
- `FileInfo` — file path info (`Path`, `Base`)
- `FileSystem` interface — injectable filesystem for testing
- `parsePackageInfo()` — AST-based package validation
- `findTaggedDirs()` — recursive directory walk
- Errors: `ErrMainFunctionNotAllowed`, `ErrMultiplePackageNames`, `ErrNoTaggedFiles`

**Dependencies:** ARCH-5 (internal/parse — for build tag detection)

**Source files:** `discover.go`

---

## ARCH-5: internal/parse — Utility parsing

**Traces to:** REQ-2-12, REQ-10-1

**Key types/functions:**
- `CamelToKebab(s string) string` — name conversion for targets and flags
- `HasBuildTag(content, tag string) bool` — build tag detection in file content
- `IsGoSourceFile(name string) bool` — filters `.go` files (excludes `_test.go`)
- `ReflectTag` — struct tag parser (`Get(key)` for key-value extraction)
- `NewReflectTag(tag string) ReflectTag` — constructor
- `FindRegistrationCalls(file *ast.File, funcName string) []ast.Expr` — AST helper for finding `init()` calls

**Dependencies:** None

**Source files:** `parse.go`

---

## ARCH-6: internal/help — Help output rendering

**Traces to:** REQ-6-1 through REQ-6-6, DES-6-1, DES-6-2

**Key types/functions:**
- `Builder` — entry point, type-state pattern
- `ContentBuilder` — section accumulator (`AddFlags`, `AddSubcommands`, `AddCommandGroups`, `AddPositionals`, `AddExamples`, `AddTargFlagsFiltered`, etc.)
- `New(commandName) *Builder` — constructor (panics on empty name)
- `WithDescription(desc) *ContentBuilder` — type transition
- `Render() string` — final output generation
- `WriteRootHelp(w, ...)` — root-level help generation
- `WriteTargetHelp(w, ...)` — target-level help generation
- `TargFlagFilter` — controls which targ flags are visible
- Styles: lipgloss-based ANSI styling for headers, flags, placeholders

**Dependencies:** ARCH-7 (internal/flags — for flag definitions)

**Source files:** `builder.go`, `content.go`, `generators.go`, `render.go`, `render_helpers.go`, `styles.go`

---

## ARCH-7: internal/flags — CLI flag definitions

**Traces to:** REQ-11-9, REQ-11-10, DES-11-2

**Key types/functions:**
- `Def` — flag definition struct (`Long`, `Short`, `Desc`, `Placeholder`, `TakesValue`, `RootOnly`, `Hidden`, `Removed`, `Mode`)
- `Placeholder` — value placeholder with format info
- `FlagMode` — `FlagModeAll` (both modes) or `FlagModeTargOnly` (targ CLI only)
- `All` — complete flag registry (slice of `Def`)
- Query functions: `BooleanFlags()`, `ValueFlags()`, `RootOnlyFlags()`, `GlobalFlags()`, `VisibleFlags()`

**Dependencies:** None

**Source files:** `flags.go`, `placeholders.go`

---

## ARCH-8: internal/file — File utilities

**Traces to:** REQ-4-4, REQ-4-6, REQ-4-7, REQ-4-8, REQ-5-3 through REQ-5-6, REQ-5-9, REQ-14-1 through REQ-14-5, DES-4-1, DES-5-1, DES-14-1, DES-14-2

**Key types/functions:**
- `Match(patterns ...string) ([]string, error)` — glob matching with brace expansion and `**` support (via doublestar)
- `expandBraces(pattern string) ([]string, error)` — recursive `{a,b}` expansion
- `Checksum(inputs, dest, matchFn, ops) (bool, error)` — content-based SHA-256 hashing
- `FileOps` — injectable filesystem operations for checksum
- `Watch(ctx, patterns, opts, callback, matchFn, ops) error` — polling-based file watcher
- `WatchOps` — injectable ticker/stat for watch
- `ChangeSet` — added/removed/modified file lists
- `WatchOptions` — configurable poll interval
- Errors: `ErrNoPatterns`, `ErrNoInputPatterns`, `ErrEmptyDest`, `ErrUnmatchedBrace`

**Dependencies:** None (uses `github.com/bmatcuk/doublestar/v4` external dependency)

**Source files:** `match.go`, `checksum.go`, `watch.go`

---

## ARCH-9: internal/sh — Shell execution

**Traces to:** REQ-13-1 through REQ-13-8, REQ-15-1 through REQ-15-6, DES-13-1, DES-13-2, DES-15-1, DES-15-2, DES-15-3

**Key types/functions:**
- `Run(env, name, args...) error` — streaming command execution
- `RunV(env, name, args...) error` — verbose (prints command first)
- `Output(env, name, args...) (string, error)` — captured output execution
- `OutputContext(ctx, name, args, stdin) (string, error)` — context-aware captured output
- `RunContextWithIO(ctx, name, args, env) error` — context-aware streaming
- `ShellEnv` — dependency injection for exec, IO, platform, cleanup
- `DefaultShellEnv() ShellEnv` — OS defaults
- `SafeBuffer` — thread-safe `bytes.Buffer`
- `ExeSuffix(env)`, `WithExeSuffix(env, name)` — platform helpers
- `IsWindowsOS() bool` — platform detection
- `EnableCleanup()` — signal handler installation
- `CleanupManager` — process registration/kill lifecycle
- `RegisterProcess`, `UnregisterProcess`, `KillAllProcesses` — process tracking
- `PlatformKillProcess(p)` — platform-specific kill
- `SetProcGroup(cmd)` — process group isolation
- `KillProcessGroup(cmd)` — group termination

**Dependencies:** None

**Source files:** `sh.go`, `context.go`, `cleanup.go`, `context_unix.go`, `context_windows.go`, `cleanup_unix.go`, `cleanup_windows.go`

---

## ARCH-10: cmd/targ — CLI entry point

**Traces to:** REQ-10-1, DES-10-2

**Key types/functions:**
- `main()` — calls `runner.Run()` and exits with its return code

**Dependencies:** ARCH-3 (internal/runner)

**Source files:** `main.go`

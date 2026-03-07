# L5 Implementation Mapping: targ

Bottom-up mapping of every non-test source file to its ARCH item and UC traces.

---

## ARCH-1: Root public API (`targ.go`)

| File | Key Exports | Traces to | Status |
|---|---|---|---|
| `targ.go` | Targ, Main, Register, ExecuteRegistered, ExecuteRegisteredWithOptions, Execute, ExecuteWithOptions, Group, Run, RunV, RunContext, RunContextV, Output, OutputContext, Match, Checksum, Watch, Print, Printf, EnableCleanup, DeregisterFrom, ExeSuffix, WithExeSuffix, IsWindows; re-exported types (Target, TargetGroup, DepMode, DepOption, DepGroup, Result, ExecuteResult, ExitError, MultiError, TagKind, TagOptions, Interleaved, ChangeSet, WatchOptions, RunOptions, RuntimeOverrides, Example); constants (Disabled, Parallel, CollectAllErrors, Pass, Fail, Errored, Cancelled, TagKindFlag, TagKindPositional, TagKindUnknown); errors (ErrEmptyDest, ErrNoInputPatterns, ErrNoPatterns, ErrUnmatchedBrace) | UC-1, UC-3, UC-4, UC-5, UC-8, UC-12, UC-13, UC-14, UC-15 | Complete |

---

## ARCH-2: internal/core — Target definition, execution engine, registry

| File | Key Exports | Traces to | Status |
|---|---|---|---|
| `internal/core/target.go` | Target, DepMode, DepModeSerial, DepModeParallel, DepModeMixed, DepOption, DepGroup (builder methods: Name, Description, Deps, Cache, CacheDir, Watch, Timeout, Times, Retry, Backoff, While, Examples) | UC-1, UC-3, UC-4, UC-5, UC-11 | Complete |
| `internal/core/command.go` | DepGroupDisplay, GroupLike, commandNode, parseTarget, parseGroupLike, executeGroupWithParents (command tree construction and CLI dispatch) | UC-1, UC-2, UC-3, UC-4, UC-5, UC-6, UC-7 | Complete |
| `internal/core/execute.go` | CallerSkipPublicAPI, Deregistration, DeregisterFrom, Main, ExecuteRegistered, ExecuteWithResolution (execution entry points) | UC-1, UC-8 | Complete |
| `internal/core/override.go` | RuntimeOverrides, ExtractOverrides, ExecuteWithOverrides, checkConflicts | UC-4, UC-5, UC-11 | Complete |
| `internal/core/run_env.go` | ExecuteEnv, NewExecuteEnv, RunEnv interface, osRunEnv (environment abstraction for execution) | UC-1 | Complete |
| `internal/core/registry.go` | Conflict, ConflictError, RegisterTarget, resolveRegistry, detectConflicts (global target registry and conflict detection) | UC-1, UC-8 | Complete |
| `internal/core/group.go` | TargetGroup, Group (named group creation and member management) | UC-7 | Complete |
| `internal/core/completion.go` | PrintCompletionScriptTo, doCompletion, tokenizeCommandLine (shell completion for bash/zsh/fish) | UC-6 | Complete |
| `internal/core/print.go` | FormatPrefix, Print, Printf (parallel-aware output with prefix formatting) | UC-12 | Complete |
| `internal/core/printer.go` | Printer, NewPrinter (serialized output from parallel targets) | UC-12 | Complete |
| `internal/core/prefix_writer.go` | PrefixWriter, NewPrefixWriter (line-buffered prefixed io.Writer for parallel shell output) | UC-12 | Complete |
| `internal/core/exec_info.go` | ExecInfo, GetExecInfo (parallel execution metadata via context) | UC-12 | Complete |
| `internal/core/types.go` | TagKind, TagKindFlag, TagKindPositional, TagKindUnknown, Example, ExecuteResult, Interleaved, RunEnv, RunOptions, TagOptions (shared type definitions) | UC-1, UC-2, UC-6, UC-11 | Complete |
| `internal/core/result.go` | Result, Pass, Fail, Cancelled, Errored, MultiError, ExitError (execution result types) | UC-1, UC-3 | Complete |
| `internal/core/state.go` | RegistryState, NewRegistryState (mutable registry state for DI/testing) | UC-1, UC-8 | Complete |
| `internal/core/source.go` | callerPackagePath (runtime caller introspection for source tracking) | UC-1, UC-8 | Complete |
| `internal/core/parse.go` | CLI argument parsing internals (flag/positional parsing, type coercion, validation) | UC-2 | Complete |
| `internal/core/git.go` | CommandRunner, FileOpener, CheckCleanWorkTree (git working tree check utility) | UC-1 | Complete |
| `internal/core/doc.go` | (package documentation) | — | Complete |

---

## ARCH-3: internal/runner — CLI tool orchestration

| File | Key Exports | Traces to | Status |
|---|---|---|---|
| `internal/runner/runner.go` | Run, ContentPatch, CreateOptions, FileOps, ExtractTargFlags, ParseSyncArgs, ParseCreateArgs, AddImportToTargFileWithFileOps, AddTargetToFileWithFileOps, ConvertFuncTargetToString, ConvertStringTargetToFunc, FindOrCreateTargFileWithFileOps, CreateGroupMemberPatch, handleSyncFlag, groupByModule, handleSingleModule, handleMultiModule, handleIsolatedModule, tryRunCached | UC-8, UC-9, UC-10 | Complete |

---

## ARCH-4: internal/discover — File discovery

| File | Key Exports | Traces to | Status |
|---|---|---|---|
| `internal/discover/discover.go` | Discover, Options, PackageInfo, FileInfo, FileSystem, ErrMainFunctionNotAllowed, ErrMultiplePackageNames, ErrNoTaggedFiles | UC-10 | Complete |

---

## ARCH-5: internal/parse — Utility parsing

| File | Key Exports | Traces to | Status |
|---|---|---|---|
| `internal/parse/parse.go` | CamelToKebab, HasBuildTag, IsGoSourceFile, ReflectTag, NewReflectTag, FindRegistrationCalls | UC-2, UC-10 | Complete |

---

## ARCH-6: internal/help — Help output rendering

| File | Key Exports | Traces to | Status |
|---|---|---|---|
| `internal/help/builder.go` | Builder, New (type-state help builder constructor) | UC-6 | Complete |
| `internal/help/content.go` | ContentBuilder, AddFlags, AddSubcommands, AddCommandGroups, AddPositionals, AddExamples, AddTargFlagsFiltered (section accumulation) | UC-6 | Complete |
| `internal/help/generators.go` | WriteRootHelp, WriteTargetHelp (top-level help generation) | UC-6 | Complete |
| `internal/help/render.go` | Render (final output assembly) | UC-6 | Complete |
| `internal/help/render_helpers.go` | TargFlagFilter (helper functions for rendering) | UC-6 | Complete |
| `internal/help/styles.go` | lipgloss-based ANSI styles for headers, flags, placeholders | UC-6 | Complete |

---

## ARCH-7: internal/flags — CLI flag definitions

| File | Key Exports | Traces to | Status |
|---|---|---|---|
| `internal/flags/flags.go` | Def, FlagMode, FlagModeAll, FlagModeTargOnly, All, BooleanFlags, ValueFlags, RootOnlyFlags, GlobalFlags, VisibleFlags | UC-6, UC-11 | Complete |
| `internal/flags/placeholders.go` | Placeholder (value placeholder with format info) | UC-6, UC-11 | Complete |

---

## ARCH-8: internal/file — File utilities

| File | Key Exports | Traces to | Status |
|---|---|---|---|
| `internal/file/match.go` | Match, expandBraces, ErrNoPatterns, ErrUnmatchedBrace | UC-4, UC-5, UC-14 | Complete |
| `internal/file/checksum.go` | Checksum, FileOps, ErrEmptyDest, ErrNoInputPatterns | UC-4, UC-14 | Complete |
| `internal/file/watch.go` | Watch, WatchOps, ChangeSet, WatchOptions | UC-5 | Complete |

---

## ARCH-9: internal/sh — Shell execution

| File | Key Exports | Traces to | Status |
|---|---|---|---|
| `internal/sh/sh.go` | SafeBuffer, ShellEnv, DefaultShellEnv, Run, RunV, Output, ExeSuffix, WithExeSuffix, IsWindowsOS | UC-13 | Complete |
| `internal/sh/context.go` | OutputContext, RunContextWithIO (context-aware execution) | UC-13 | Complete |
| `internal/sh/cleanup.go` | CleanupManager, NewCleanupManager, EnableCleanup, RegisterProcess, UnregisterProcess, KillAllProcesses | UC-15 | Complete |
| `internal/sh/context_unix.go` | SetProcGroup, KillProcessGroup (Unix process group management) | UC-13, UC-15 | Complete |
| `internal/sh/context_windows.go` | SetProcGroup, KillProcessGroup (Windows process group management) | UC-13, UC-15 | Complete |
| `internal/sh/cleanup_unix.go` | PlatformKillProcess (Unix signal-based kill) | UC-15 | Complete |
| `internal/sh/cleanup_windows.go` | PlatformKillProcess (Windows process termination) | UC-15 | Complete |

---

## ARCH-10: cmd/targ — CLI entry point

| File | Key Exports | Traces to | Status |
|---|---|---|---|
| `cmd/targ/main.go` | main (calls runner.Run, exits with return code) | UC-10 | Complete |

---

## Build tooling (dev/)

| File | Key Exports | Traces to | Status |
|---|---|---|---|
| `dev/targets.go` | Check, CheckFull, CheckForFail, Test, TestForFail, Lint, LintFull, LintFast, LintForFail, Fmt, Tidy, Modernize, Coverage, Deadcode, DeleteDeadcode, Fuzz, Generate, Mutate, ReorderDecls, ReorderDeclsCheck, FindRedundantTests, CheckCoverage, CheckCoverageForFail, CheckUncommitted, CheckNils, CheckNilsFix, CheckNilsForFail, CheckThinAPI, Clean, InstallTools (build/CI targets) | — (tooling, not product code) | Complete |

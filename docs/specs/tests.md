# L3: Test List

Items derived from: L4 implementation (existing test suite)

## T-1: Target builder API

**Given** a function or shell string, **When** `Targ()` is called with builder methods (`.Name()`, `.Deps()`, `.Cache()`, `.Watch()`, `.Timeout()`, `.Times()`, `.Retry()`, `.Backoff()`, `.While()`, `.OnStart()`, `.OnStop()`), **Then** the target retains all configured values and reports correct state via getters.

- Property: default target is not renamed (`IsRenamed()` false)
- Property: default source is empty
- Property: dep group chaining produces correct mode and membership
- Property: lifecycle hooks (OnStart/OnStop) are registered and default to nil

**Tests:** `TestProperty_DefaultIsNotRenamed`, `TestProperty_DefaultSourceIsEmpty`, `TestProperty_DepGroupChaining`, `TestOnStartOnStop`, `TestDepModeString`
**Traces to L4:** IMPL-4 (Target Definition and Builder)

## T-2: Command parsing and argument handling

**Given** a target with struct-tag-annotated arguments, **When** CLI args are parsed, **Then** flags, positionals, short names, defaults, enums, environment variables, required fields, maps, slices, embedded structs, and interleaved flags are correctly resolved.

- Property: struct field names convert to kebab-case flag names
- Property: unrecognized tag keys produce errors
- Property: env vars provide fallback values, overridden by explicit flags
- Property: TextUnmarshaler types are supported
- Property: bool flags parse arbitrary strings without panic
- Property: arbitrary CLI args, flag names, and flag values don't crash
- Property: float64 flags parse correctly

**Tests:** `TestProperty_StructFieldNameToKebabCase`, `TestProperty_UnrecognizedTagKeyError`, `TestProperty_EnvVarBehavior`, `FuzzBoolFlag_ArbitraryStrings`, `FuzzExecute_ArbitraryCLIArgs`, `FuzzExecute_ArbitraryFlagNames`, `FuzzExecute_ArbitraryFlagValues`, `TestFloat64FlagParsing`
**Traces to L4:** IMPL-5 (Command Parsing and Execution), IMPL-17 (Parse Utilities)

## T-3: Execution behavior

**Given** registered targets, **When** executed via `Execute()`, **Then** targets run in correct order, runtime overrides (`--times`, `--timeout`, `--parallel`, `--retry`, `--backoff`, `--watch`, `--cache`, `--dep-mode`) are applied, and conflicts between compile-time config and CLI flags produce errors.

- Property: no targets prints message
- Property: multiple targets run sequentially
- Property: exit error has message
- Property: AllowDefault=false requires explicit command
- Property: duplicate names produce error
- Property: empty string target panics
- Property: failure has clear error message
- Property: help flag does not execute target
- Property: timeout flag enforces limit
- Property: times flag controls repetition
- Fuzz: backoff arbitrary parameters, builder chain ordering, cache patterns, deps, descriptions
- Property: parallel dep concurrency never exceeds min(n, max(2, GOMAXPROCS/2))
- Property: serial CollectAllErrors runs every dep in order and reports all failures
- Property: --dep-mode flatten preserves CollectAllErrors (serial and parallel)
- Property: --dep-mode serial on a deps-only target runs its deps serially in declaration order
- Property: with exactly one registered target, `targ <name>` runs the target
- Property: a name-shaped first positional binds as the command name, not as data
- Property: top-level `-p` fan-out never exceeds parallelCap(n, GOMAXPROCS)
- Property: an unresolvable token runs nothing — not the target, not its deps — in serial chains, under `-p`, and for shell-command targets
- Property: the chain resolve pass does not double-bind a variadic positional

**Tests:** `TestProperty_Execution/ResolveOnlyRunsNeitherTargetNorDeps`, `TestProperty_Execution/UnresolvableChainTokenRunsNothing`, `TestProperty_Execution/CallerSuppliedResolveOnlyStaysASinglePass`, `TestProperty_Execution/HelpPrintsOnceWhenTheChainIsWalkedTwice`, `TestProperty_Execution/ChainPrePassBindsVariadicPositionalOnce`, `TestProperty_Execution/ParallelUnresolvableArgRunsNoSiblingUnit`, `TestProperty_ShellCommandDeps/UnknownBareArgRunsNeitherDepNorShellCommand`, `TestProperty_ShellCommandDeps/ResolveOnlyStillRejectsUnknownBareArg`, `TestProperty_ShellCommandDeps/ResolveOnlyRunsNeitherShellCommandNorDeps`
**Traces to L4:** IMPL-5 (Command Parsing and Execution)

## T-4: Target registry and deregistration

**Given** targets registered via `Register()` from multiple packages, **When** `DeregisterFrom()` is called, **Then** targets from the specified package are removed while others are preserved. Conflict detection prevents duplicate names.

- Property: deregistration after resolution errors
- Property: deregister then re-register works
- Property: deregistration preserves groups from other packages
- Property: deregistration removes groups from deregistered packages

**Tests:** `TestProperty_DeregisterFromAfterResolutionErrors`, `TestProperty_DeregisterThenReregister`, `TestProperty_ApplyDeregistrations_PreservesGroupsFromOtherPackages`, `TestProperty_ApplyDeregistrations_RemovesGroupsFromDeregisteredPackages`
**Traces to L4:** IMPL-6 (Target Registry)

## T-5: Target groups and hierarchy

**Given** targets organized into `Group()` hierarchies, **When** commands are issued with subcommand paths (e.g., `targ dev lint fast`), **Then** the correct target is resolved and executed. Glob patterns match multiple targets.

- Property: contains match works (glob)
- Property: no matches returns error
- Property: pattern matches multiple targets and subcommands
- Fuzz: caret reset chains, glob patterns, group names, nested groups, mixed roots

**Tests:** `TestProperty_Hierarchy`, `FuzzCaretReset_ArbitraryChains`, `FuzzGlob_ArbitraryPatterns`, `FuzzGroupName_ValidPatterns`, `FuzzGroups_ArbitraryNesting`, `FuzzMixedRoots_ArbitraryMix`
**Traces to L4:** IMPL-7 (Target Groups), IMPL-5 (Command Parsing and Execution)

## T-6: Result classification and reporting

**Given** target execution outcomes (nil, context.Canceled, DeadlineExceeded, other errors), **When** classified, **Then** correct Result status (Pass/Fail/Cancelled/Errored) is assigned. Multi-error collects all failures from CollectAllErrors mode (parallel or serial). Summary formatting shows per-target status.

- Property: string values for each status
- Property: nil → Pass, context.Canceled → Cancelled (or Fail if first), DeadlineExceeded → Errored
- Property: multi-error contains all failures, uses first line of multi-line errors
- Property: format summary shows non-zero counts only
- Property: detailed summary shows per-target status with truncated error snippets

**Tests:** `TestResult`, `TestMultiError`, `TestFormatDetailedSummary`
**Traces to L4:** IMPL-8 (Result Handling)

## T-7: Parallel output prefixing

**Given** targets running in parallel, **When** they produce output, **Then** each line is prefixed with the target name, prefixes are aligned, and output is serialized through the printer. Error text appears before summary. Lifecycle messages appear.

- Property: prefix writer handles complete lines, partial lines, multi-line, chunked writes
- Property: printer preserves order, flushes all on close
- Property: Print/Printf route through printer in parallel, write directly in serial
- Property: parallel deps produce prefixed output, aligned prefixes
- Property: fail-fast reports cancelled targets
- Property: shell command output is prefixed (single and multi-line)
- Property: top-level parallel produces prefixed output
- Property: RunContext/RunContextV route through printer in parallel mode

**Tests:** `TestPrefixWriter`, `TestPrinter`, `TestPrint`, `TestParallelOutputDepLevel`, `TestParallelOutputShellCommand`, `TestParallelOutputTopLevel`, `TestRunContext`, `TestRunContextInParallelMode`, `TestRunContextV`, `TestRunContextVInParallelMode`
**Traces to L4:** IMPL-9 (Parallel Output)

## T-8: Shell completion

**Given** a targ application, **When** `--completion [shell]` is invoked, **Then** a valid completion script is generated for bash, zsh, or fish. Unsupported shells produce errors.

- Property: script generation for bash, zsh, fish
- Property: unsupported shell returns error
- Property: empty SHELL env shows usage
- Property: Windows path shell handling
- Property: completion example with getenv (shell detection)

**Tests:** `TestProperty_Completion`, `TestProperty_CompletionExampleWithGetenv`
**Traces to L4:** IMPL-10 (Completion)

## T-9: Git integration

**Given** a git repository, **When** `CheckCleanWorkTree()` is called, **Then** modified/untracked/staged files produce errors, clean tree returns nil. Repo URL detection parses `.git/config`.

- Property: clean tree returns nil, modified/untracked/staged return errors
- Property: git command failure returns error, whitespace-only output returns nil
- Property: default runner executes git

**Tests:** `TestProperty_CleanWorkTree`
**Traces to L4:** IMPL-11 (Git Utilities)

## T-10: Source location detection

**Given** a target defined in a Go file, **When** source location is requested, **Then** the correct file path and line number are returned. Package paths are extracted from runtime caller information.

- Property: invalid depth returns error
- Property: extracted path is prefix with no trailing dot
- Property: known extractions match expected output
- Property: local targets use source file, remote targets use source package

**Tests:** `TestProperty_CallerPackagePath`, `TestProperty_ExtractPackagePath`, `TestParseTargetLike_*`
**Traces to L4:** IMPL-12 (Source Location)

## T-11: Target discovery

**Given** a Go project with `//go:build targ` files, **When** `Discover()` is called, **Then** tagged files are found, `targ.Register()` calls are detected, and aliased imports are recognized.

- Property: finds tagged files in directory
- Property: detects explicit registration
- Property: detects aliased import

**Tests:** `TestProperty_Discovery`
**Traces to L4:** IMPL-13 (Target Discovery)

## T-12: Flag definitions and filtering

**Given** the built-in flag registry, **When** flags are queried by mode, **Then** targ-only flags are hidden in binary mode, and flag.Find() validates short flags.

- Property: binary mode true hides targ-only flags, false shows them
- Property: Find rejects non-single-char short flags
- Property: unknown short flags return nil
- Property: all flags have explicit mode
- Property: user examples shown in root help, repo URL shown when no more-info text

**Tests:** `TestBinaryModePropagation`, `TestProperty_FindRejectsNonSingleShort`, `TestProperty_FindUnknownShortReturnsNil`, `TestAllFlagsHaveExplicitMode`, `TestPrintUsageWithExamples`
**Traces to L4:** IMPL-15 (Flag Definitions)

## T-13: Help text generation and rendering

**Given** help content (flags, positionals, subcommands, examples, execution info), **When** rendered, **Then** ANSI codes are paired correctly, empty sections are omitted, examples have no ANSI codes, and all content types render correctly.

- Property: ANSI codes paired correctly
- Property: empty sections omitted
- Property: examples have no ANSI codes
- Property: builder methods accumulate and are chainable (positionals, global flags, root-only flags)
- Property: content types can be created (Example, Flag, Format, Positional, Subcommand)
- Property: auto-generated root/target examples, user examples replace defaults
- Property: binary mode help output (root and target)
- Property: StripANSI removes escape bytes and well-formed sequences
- Property: help shows flag descriptions, usage line, subcommand list
- Property: usage line shows positional args (optional, variadic), enum values

**Tests:** `TestProperty_ANSICodesPairedCorrectly`, `TestProperty_EmptySectionsOmitted`, `TestProperty_ExamplesHaveNoANSICodes`, `TestProperty_AddGlobalFlagsFromRegistryIgnoresUnknownAndIsChainable`, `TestProperty_AddPositionalsAccumulates`, `TestProperty_AddRootOnlyFlagsAppendsAndIsChainable`, `TestProperty_ExampleCanBeCreated`, `TestProperty_FlagCanBeCreated`, `TestProperty_FormatCanBeCreated`, `TestProperty_PositionalCanBeCreated`, `TestProperty_SubcommandCanBeCreated`, `TestAutoGeneratedRootExamples`, `TestAutoGeneratedTargetExamples`, `TestBinaryModeHelpOutput`, `TestProperty_StripANSI_*`, `TestProperty_CommandHelp`, `TestProperty_UsageLine`
**Traces to L4:** IMPL-16 (Help System)

## T-14: Parse utilities

**Given** Go identifiers and struct tags, **When** parsed, **Then** CamelToKebab produces correct kebab-case, build tags are detected, and targ imports are identified.

- Property: output is lowercase, contains only lowercase and hyphens
- Property: preserves letter count
- Property: known conversions (FooBar→foo-bar, APIServer→api-server)
- Property: HasBuildTag detects targ build tags

**Tests:** `TestProperty_Parsing`
**Traces to L4:** IMPL-17 (Parse Utilities)

## T-15: Build tool runner

**Given** the `targ` build tool, **When** `--create`, `--sync`, `--to-func`, `--to-string` commands are used, **Then** correct code is generated. Help output matches golden files. ContainsHelpFlag detects help args.

- Property: code generation with Register produces correct init() functions
- Property: golden file help output for create, sync, to-func, to-string
- Property: ContainsHelpFlag matches args
- Property: runner properties (with in-memory filesystem)

**Tests:** `TestCreateCodegenWithRegister`, `TestGoldenFile_HelpOutput`, `TestProperty_ContainsHelpFlagMatchesArgs`, runner_properties_test.go
**Traces to L4:** IMPL-18 (Build Tool Runner)

## T-16: Example helpers

**Given** the example API, **When** `BuiltinExamples()`, `EmptyExamples()`, `AppendBuiltinExamples()`, `PrependBuiltinExamples()` are called, **Then** examples are combined in the correct order with expected lengths.

- Property: empty returns empty slice, builtin returns non-empty
- Property: append preserves custom first, prepend preserves custom last
- Property: append and prepend have same total length
- Property: portable examples compile

**Tests:** `TestProperty_ExampleHelpers`, `TestProperty_PortableExamplesCompile`
**Traces to L4:** IMPL-5 (Command Parsing and Execution)

## T-17: Chain and command examples

**Given** a command hierarchy, **When** chain examples are generated, **Then** nil nodes use fallback, nested groups show caret syntax, flat sources show both names.

**Tests:** `TestChainExample`
**Traces to L4:** IMPL-5 (Command Parsing and Execution), IMPL-7 (Target Groups)

## T-18: ExecuteEnv test helper

**Given** an ExecuteEnv, **When** environment variables are looked up, **Then** custom env map values are returned, with OS fallback.

**Tests:** `TestExecuteEnvGetenv`
**Traces to L4:** IMPL-5 (Command Parsing and Execution)

## T-19: Parallel failure reporting

**Given** parallel target execution with failures, **When** a target fails, **Then** the error is reported correctly with target identification.

**Tests:** `TestParallelFailureReportsError`
**Traces to L4:** IMPL-8 (Result Handling), IMPL-9 (Parallel Output)

## T-20: Mutation testing

**Given** the full test suite, **When** mutation testing runs with ooze, **Then** 100% of mutations are caught.

**Tests:** `TestMutation`
**Traces to L4:** IMPL-3 (Build Targets), all other IMPL items (mutation tests exercise entire codebase)

## T-21: Upward directory discovery

**Given** a directory tree with targ-tagged `.go` files in ancestor directories, **When** `Discover()` is called from a subdirectory, **Then** targets from ancestor directories are found alongside targets in the subtree below.

- Property: discovers targets in parent directory
- Property: discovers targets in grandparent directory
- Property: walks the full ancestor path, stopping before filesystem root (no artificial project-root boundary)
- Property: does not discover targets in sibling directories of ancestors
- Property: discovers targets in ancestor `dev/` subtrees when present
- Property: ignores ancestor `dev/` when it does not exist
- Property: combines upward and downward results without duplicates

**Tests:** `TestProperty_UpwardDiscovery` (DiscoversTargetsInParentDirectory, DiscoversTargetsInGrandparentDirectory, DoesNotDiscoverSiblingDirectories, DiscoversAncestorDevSubtree, IgnoresAncestorDevWhenAbsent, CombinesUpwardAndDownwardResults, NoDuplicatesWhenStartDirIsAlsoAncestor, DiscoversNestedDevSubtree)
**Traces to L2:** ARCH-11 (Target Discovery)
**Traces to L2:** REQ-13, REQ-17, DES-6

## T-22: Ancestor module grouping and build

**Given** ancestor targets discovered via upward walk with varying module configurations, **When** the build tool groups and compiles them, **Then** each ancestor module group builds independently and commands are aggregated.

- Property: ancestor with `go.mod` uses normal module build
- Property: ancestor without `go.mod` uses isolated build (synthetic `go.mod`)
- Property: multiple ancestors produce multiple module groups
- Property: ancestor targets and local targets coexist (multi-module aggregation)
- Property: conflict between ancestor and local target names produces `ConflictError`

**Tests:** Integration-verified via existing multi-module build path (T-15 runner properties) and end-to-end usability gate. Discovery layer (T-21) feeds ancestor targets into the existing `groupByModule()` → `handleMultiModule()` path which is already tested.
**Traces to L2:** ARCH-12 (Build Tool Runner)
**Traces to L2:** REQ-16

## T-23: Superseding detection and dispatch

**Given** multiple module registries containing commands with the same name at different discovery depths, **When** commands are collected for dispatch, **Then** the most-local version (closest to CWD) is selected.

- Property: when a command exists in both local and ancestor registries, `findCommandBinary` returns the local binary
- Property: registries are ordered by proximity (CWD first, then CWD/dev/, then ancestors by ascending distance)
- Property: `collectSortedCommands` annotates duplicate commands with superseding metadata

**Tests:** `TestProperty_SupersedingDetection` (MostLocalCommandWinsDispatch, CollectAnnotatesDuplicatesWithSuperseding, RegistryOrderDeterminesLocality, NonDuplicateCommandsUnaffected) in supersede_test.go
**Traces to L2:** ARCH-16 (Superseding Logic)
**Traces to L4:** IMPL-20 (Superseding Logic)
**Traces to L2:** REQ-18

## T-24: Superseding display in help

**Given** multiple module registries with duplicate command names, **When** `printMultiModuleHelp` renders the help output, **Then** the primary (most-local) version shows normally with a "supersedes <source>" annotation, and higher-level duplicates are marked as superseded with dimmed styling.

- Property: superseded commands appear in help output with superseding annotation
- Property: primary commands show what they supersede
- Property: non-duplicate commands render without annotations
- Property: single-module help (no duplicates) is unaffected

**Tests:** `TestProperty_SupersedingDisplay` (SupersededCommandsShowAnnotation, SingleModuleHelpUnaffected) in supersede_test.go
**Traces to L2:** ARCH-16 (Superseding Logic), ARCH-10 (Help System)
**Traces to L4:** IMPL-20 (Superseding Logic)
**Traces to L2:** DES-7

## T-25: Foreground process group for interactive commands

**Given** a `ShellEnv` configuration, **When** commands are executed via `RunContextWithIO`, **Then** foreground commands (default) inherit the parent's process group for TTY access, while background commands (parallel mode) get isolated process groups for clean cancellation.

- Property: `DefaultShellEnv()` sets `Foreground = true`
- Property: parallel shell env sets `Foreground = false`
- Property: foreground commands skip `SetProcGroup` (no `Setpgid`)
- Property: background commands use `SetProcGroup` (with `Setpgid`)

**Tests:** `TestProperty_ForegroundProcessGroup` in `sh` package or `core` package
**Traces to L2:** ARCH-8 (Shell Execution), ARCH-7 (Parallel Output)
**Traces to L4:** IMPL-19 (Shell Execution)
**Traces to L2:** REQ-9

## Coverage Gaps (L4 items with no dedicated L3 test coverage)

- **IMPL-1 (Root Public API):** No direct tests — tested indirectly through integration tests in `test/`. Thin re-export layer, expected.
- **IMPL-2 (CLI Entry Point):** No tests — `main()` is excluded from coverage per CLAUDE.md. Expected.
- **IMPL-3 (Build Targets):** No unit tests for dev targets themselves — they are consumers of targ, tested via mutation testing (T-20).
- **IMPL-14 (File Utilities):** No dedicated test file found. Match/Checksum/Watch tested indirectly through integration tests.
- **IMPL-19 (Shell Execution):** Foreground/background distinction covered by T-25. Run/Output/RunContext also tested indirectly through other packages.

# L4: Test List (Adoption)

Adopted from existing test suite. Each entry is a test function name — the function IS the identifier.

## Cluster: Argument Parsing

Tests covering struct tag parsing, flag/positional argument handling, name derivation, and environment variable behavior.

### test/ (integration)

- **TestProperty_StructTagParsing** — Struct tag parsing (test/arguments_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_NameDerivation** — Argument name derivation from struct fields (test/arguments_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_EnvVarBehavior** — Environment variable handling for flags (test/arguments_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_HelpOutput** — Help output generation for arguments (test/arguments_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

### internal/core/

- **TestProperty_StructFieldNameToKebabCase** — Struct field to kebab-case conversion (internal/core/command_internal_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_UnrecognizedTagKeyError** — Error handling for unknown struct tags (internal/core/command_internal_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestFloat64FlagParsing** — Float64 flag parsing support (internal/core/parse_float_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

### internal/parse/

- **TestProperty_Parsing** — Argument parsing with various types (internal/parse/parse_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

### Fuzz (argument parsing)

- **FuzzBoolFlag_ArbitraryStrings** — Fuzz testing bool flags (test/arguments_fuzz_test.go)
- **FuzzExecute_ArbitraryCLIArgs** — Fuzz testing arbitrary CLI args (test/arguments_fuzz_test.go)
- **FuzzExecute_ArbitraryFlagNames** — Fuzz testing arbitrary flag names (test/arguments_fuzz_test.go)
- **FuzzExecute_ArbitraryFlagValues** — Fuzz testing arbitrary flag values (test/arguments_fuzz_test.go)
- **FuzzIntFlag_ArbitraryStrings** — Fuzz testing int flags (test/arguments_fuzz_test.go)
- **FuzzMapFlag_ArbitraryKeyValueStrings** — Fuzz testing map flags (test/arguments_fuzz_test.go)
- **FuzzSliceFlag_ArbitraryValues** — Fuzz testing slice flags (test/arguments_fuzz_test.go)
- **FuzzTargetName_ArbitraryStrings** — Fuzz testing target names (test/arguments_fuzz_test.go)
- **FuzzTimeoutFlag_ArbitraryStrings** — Fuzz testing timeout flags (test/arguments_fuzz_test.go)

---

## Cluster: Target Execution

Tests covering target execution, dependency modes (serial/parallel), error handling, timeout, caching, and repetition.

### test/ (integration)

- **TestProperty_Execution** — Target execution properties (test/execution_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestParallelFailureReportsError** — Parallel failure error reporting (test/execution_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

### internal/core/

- **TestExecuteEnvGetenv** — ExecuteEnv environment variable lookup (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestExecInfo** — Execution info context management (internal/core/exec_info_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestOnStartOnStop** — Lifecycle hook registration and execution (internal/core/lifecycle_hooks_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

### internal/core/ (results)

- **TestFormatDetailedSummary** — Detailed summary formatting (internal/core/result_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestMultiError** — MultiError message composition (internal/core/result_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestResult** — Result status string values (internal/core/result_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

### Fuzz (execution)

- **FuzzBackoff_ArbitraryParameters** — Fuzz testing backoff (test/execution_fuzz_test.go)
- **FuzzBuilderChain_ArbitraryOrder** — Fuzz testing builder chains (test/execution_fuzz_test.go)
- **FuzzCache_ArbitraryPatterns** — Fuzz testing cache patterns (test/execution_fuzz_test.go)
- **FuzzDeps_ArbitraryDependencies** — Fuzz testing dependencies (test/execution_fuzz_test.go)
- **FuzzDescription_ArbitraryStrings** — Fuzz testing descriptions (test/execution_fuzz_test.go)
- **FuzzShellCommand_ArbitraryCommandStrings** — Fuzz testing shell commands (test/execution_fuzz_test.go)
- **FuzzTimeout_ArbitraryDurations** — Fuzz testing timeouts (test/execution_fuzz_test.go)
- **FuzzTimes_ArbitraryValues** — Fuzz testing times (test/execution_fuzz_test.go)
- **FuzzWatch_ArbitraryPatterns** — Fuzz testing watch patterns (test/execution_fuzz_test.go)
- **FuzzWhile_ArbitraryPredicates** — Fuzz testing while predicates (test/execution_fuzz_test.go)

---

## Cluster: Hierarchy & Groups

Tests covering target groups, nested hierarchies, naming, path resolution, and group name validation.

### test/ (integration)

- **TestProperty_Hierarchy** — Target hierarchy properties (test/hierarchy_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_GroupNameValidation** — Group name validation (test/hierarchy_fuzz_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

### Fuzz (hierarchy)

- **FuzzCaretReset_ArbitraryChains** — Fuzz testing caret reset (test/hierarchy_fuzz_test.go)
- **FuzzGlob_ArbitraryPatterns** — Fuzz testing glob patterns (test/hierarchy_fuzz_test.go)
- **FuzzGroupName_ValidPatterns** — Fuzz testing group names (test/hierarchy_fuzz_test.go)
- **FuzzGroups_ArbitraryNesting** — Fuzz testing group nesting (test/hierarchy_fuzz_test.go)
- **FuzzMixedRoots_TargetsAndGroups** — Fuzz testing mixed roots (test/hierarchy_fuzz_test.go)
- **FuzzMultipleRoots_ArbitraryNames** — Fuzz testing multiple roots (test/hierarchy_fuzz_test.go)
- **FuzzPathResolution_ArbitraryPathSegments** — Fuzz testing path resolution (test/hierarchy_fuzz_test.go)
- **FuzzPathResolution_DeepNesting** — Fuzz testing deep nesting (test/hierarchy_fuzz_test.go)

---

## Cluster: Registry & Conflict Resolution

Tests covering target registration, deregistration, conflict detection, and resolution ordering.

### internal/core/

- **TestProperty_CleanRegistryPassesResolution** — Clean registry resolves successfully (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_SameNameSameSourceNoConflict** — Same name from same package no conflict (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_SameNameDifferentSourceConflicts** — Same name from different packages conflicts (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_UniqueNamesNoConflict** — All unique names no conflict (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_MultipleConflictsAllReported** — All conflicts reported at once (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ErrorMessageContainsName** — ConflictError contains target name (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ErrorMessageContainsSources** — ConflictError contains source packages (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ErrorMessageSuggestsFix** — ConflictError suggests DeregisterFrom (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DeregisteredPackageFullyRemoved** — All targets from deregistered package removed (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DeregistrationBeforeConflictCheck** — Deregistration happens before conflict check (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DeregistrationErrorMessage** — DeregistrationError message format (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DeregistrationErrorStopsResolution** — Bad deregistration stops resolution (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DuplicateDeregistrationIsIdempotent** — Duplicate deregistration is idempotent (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_EmptyDeregistrationsNoOp** — Empty deregistration list is no-op (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_MultiplePackagesDeregistered** — Multiple packages can be deregistered (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_NonTargetItemsPreserved** — Non-Target items preserved in registry (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_OtherPackagesUntouched** — Targets from other packages preserved (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_QueueClearedAfterResolution** — Deregistration queue cleared after resolution (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_UnknownPackageErrors** — Deregistering unknown package errors (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DetectConflicts_AllowsSameGroupFromSamePackage** — Same group from same package allowed (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DetectConflicts_CatchesGroupNameConflicts** — Group name conflicts detected (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ApplyDeregistrations_PreservesGroupsFromOtherPackages** — Groups from other packages preserved (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ApplyDeregistrations_RemovesGroupsFromDeregisteredPackages** — Groups from deregistered packages removed (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ClearLocalTargetSources_ClearsGroupsFromMainModule** — Local groups have source cleared (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ClearLocalTargetSources_MixedTargetsAndGroups** — Both targets and groups cleared locally (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ClearLocalTargetSources_NoMainModuleNoChange** — Missing main module leaves sources untouched (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ClearLocalTargetSources_PreservesRemoteGroups** — Remote groups retain source package (internal/core/registry_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

### internal/core/ (execute_test.go — registry integration)

- **TestProperty_DeregisterFromAfterResolutionErrors** — DeregisterFrom fails after resolution (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DeregisterThenReregister** — Deregister and re-register preserves targets (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DeregisterWithoutReregister** — Deregister without re-register removes all (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ExecuteRegisteredResolution_ConflictPreventsExecution** — Registry conflicts prevent execution (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ExecuteRegisteredResolution_DeregistrationErrorPreventsExecution** — Bad deregistration prevents execution (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ExecuteRegisteredResolution_ExistingBehaviorUnchanged** — Clean registry works as before (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_LocalTargetsHaveSourcePkgCleared** — Local targets lose source package on resolution (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_MixedLocalAndRemoteTargetsHandled** — Local/remote targets handled separately (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_RegisterTargetWithSkip_PreservesExplicitGroupSource** — Explicit group source not overwritten (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_RegisterTargetWithSkip_SetsSourceOnGroups** — RegisterTarget sets source on groups (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ResolveRegistryReturnsDeregisteredPackages** — ResolveRegistry returns deregistered packages (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ResolveRegistryReturnsEmptyDeregisteredWhenNone** — Empty deregistered list when none (internal/core/execute_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Help Rendering

Tests covering help text generation, formatting, styling, section ordering, and binary mode output.

### internal/help/

- **TestProperty_NewBuilderAcceptsAnyNonEmptyCommandName** — Builder accepts non-empty names (internal/help/builder_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_NewBuilderPanicsOnEmptyName** — Builder panics on empty name (internal/help/builder_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_WithDescriptionCarriesOverCommandName** — Description carries command name (internal/help/builder_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_WithUsageStoresValue** — With usage stores value (internal/help/builder_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_AddGlobalFlagsFromRegistryIgnoresUnknownAndIsChainable** — Global flags builder chaining (internal/help/builder_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_AddPositionalsAccumulates** — Positional arguments accumulate (internal/help/builder_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_AddRootOnlyFlagsAppendsAndIsChainable** — Root-only flags builder chaining (internal/help/builder_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ExampleCanBeCreated** — Example struct creation (internal/help/content_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_FlagCanBeCreated** — Flag struct creation (internal/help/content_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_FormatCanBeCreated** — Format struct creation (internal/help/content_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_PositionalCanBeCreated** — Positional struct creation (internal/help/content_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_SubcommandCanBeCreated** — Subcommand struct creation (internal/help/content_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestAutoGeneratedRootExamples** — Auto-generated root examples (internal/help/generators_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestAutoGeneratedTargetExamples** — Auto-generated target examples (internal/help/generators_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ANSICodesPairedCorrectly** — ANSI codes properly paired (internal/help/render_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_EmptySectionsOmitted** — Empty help sections omitted (internal/help/render_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ExamplesHaveNoANSICodes** — Examples lack ANSI codes (internal/help/render_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_GlobalFlagsBeforeCommandFlags** — Global flags precede command flags (internal/help/render_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_NoTrailingWhitespace** — Help has no trailing whitespace (internal/help/render_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_RenderIncludesValuesWhenPresent** — Render includes values when present (internal/help/render_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_RenderSectionOrderIsCorrect** — Render section order correct (internal/help/render_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_StripANSI_RemovesEscapeBytes** — StripANSI removes escape bytes (internal/help/render_helpers_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_StripANSI_RemovesWellFormedSequences** — StripANSI removes well-formed sequences (internal/help/render_helpers_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestBinaryModeHelpOutput** — Binary mode help output filtering (internal/help/binary_mode_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestFlagSectionLabel** — Flag section label in targ vs binary mode (internal/help/binary_mode_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Shell Commands

Tests covering shell command target creation, execution, error handling, flags, and help output.

### test/ (integration)

- **TestProperty_ShellCommandExecution** — Shell command execution (test/shell_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ShellCommandErrors** — Shell command error handling (test/shell_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ShellCommandFlags** — Shell command flag handling (test/shell_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ShellCommandHelp** — Shell command help output (test/shell_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ShellCommandNaming** — Shell command naming (test/shell_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_CommandHelp** — Shell command help (test/shell_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Completion

Tests covering shell completion script generation and suggestion behavior.

### test/ (integration)

- **TestProperty_Completion** — Shell completion generation (test/completion_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_CompletionSuggestions** — Completion suggestions (test/completion_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

### internal/core/

- **TestProperty_CompletionExampleWithGetenv** — Shell completion example generation (internal/core/command_internal_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Validation & Constraints

Tests covering input validation, field constraints (required, enums), and usage line generation.

### test/ (integration)

- **TestProperty_Validation** — Validation behavior (test/validation_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_UsageLine** — Usage line generation (test/validation_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_Invariant** — Constraint invariants (test/constraints_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Overrides

Tests covering runtime override behavior (--cache, --watch, --times, etc.).

### test/ (integration)

- **TestProperty_Overrides** — Override behavior properties (test/overrides_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Parallel Output

Tests covering parallel-safe output, prefix writers, and printer ordering.

### internal/core/

- **TestParallelOutputTopLevel** — Top-level parallel output (internal/core/parallel_output_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestParallelOutputDepLevel** — Parallel output with dependency levels (internal/core/parallel_output_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestParallelOutputShellCommand** — Parallel output for shell commands (internal/core/parallel_output_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestRunContext** — RunContext serial/parallel modes (internal/core/parallel_output_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestRunContextInParallelMode** — RunContext routes output through printer (internal/core/parallel_output_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestRunContextV** — RunContextV serial/parallel modes (internal/core/parallel_output_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestRunContextVInParallelMode** — RunContextV routes output through printer (internal/core/parallel_output_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestPrefixWriter** — Prefix writer line formatting (internal/core/prefix_writer_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestPrinter** — Printer ordering and flushing (internal/core/printer_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestPrint** — Print with serial/parallel mode handling (internal/core/print_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Target Definition & Metadata

Tests covering target creation, naming, source tracking, dependency group chaining, and command building.

### internal/core/

- **TestDepModeString** — DepMode string representation (internal/core/target_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DefaultIsNotRenamed** — Default target not renamed (internal/core/target_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DefaultSourceIsEmpty** — Default target source is empty (internal/core/target_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DepGroupChaining** — Dependency group chaining and coalescing (internal/core/target_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DepsOnlyTargetCapturesSourceFile** — Deps-only target captures source file (internal/core/target_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_DepsOnlyTargetIsNotRenamed** — Deps-only target not renamed (internal/core/target_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_FuncTargetHasNoSourceFile** — Function targets have no source file (internal/core/target_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_GetSourceReturnsSetValue** — GetSource returns set source package (internal/core/target_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_NameAfterRegistrationIsRenamed** — Name after registration is renamed (internal/core/target_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_NameBeforeRegistrationIsNotRenamed** — Name before registration not renamed (internal/core/target_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ShellCommandTargetIsNotRenamed** — Shell command targets not renamed (internal/core/target_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_StringTargetCapturesSourceFile** — String targets capture source file (internal/core/target_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestChainExample** — Example chain generation for nested groups (internal/core/command_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestParseTargetLike_LocalDepsOnlyTargetUsesSourceFile** — Source file capture for deps-only targets (internal/core/command_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestParseTargetLike_LocalFuncTargetKeepsExistingSourceFile** — Source file for function targets (internal/core/command_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestParseTargetLike_LocalStringTargetUsesSourceFile** — Source file for string shell command targets (internal/core/command_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestParseTargetLike_RemoteFuncTargetUsesSourcePkg** — Source package for remote function targets (internal/core/command_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestParseTargetLike_RemoteStringTargetUsesSourcePkg** — Source package for remote string targets (internal/core/command_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestParseTargetLike_RemoteTargetUsesSourcePkg** — Remote target source package handling (internal/core/command_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestPrintCommandHelp_BasicFuncTarget** — Help output for function targets (internal/core/command_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestPrintCommandHelp_StringTarget** — Help output for shell command targets (internal/core/command_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ConvertExamplesPreservesShape** — Example conversion preserves structure (internal/core/command_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ResolveMoreInfoTextPrefersMoreInfoText** — MoreInfoText takes precedence over RepoURL (internal/core/command_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Source Tracking & Discovery

Tests covering caller package path extraction, source package detection, and target file discovery.

### internal/core/

- **TestProperty_CallerPackagePath** — Caller package path extraction (internal/core/source_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ExtractPackagePath** — Package path extraction from function names (internal/core/source_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

### internal/discover/

- **TestProperty_Discovery** — Target file discovery with build tags (internal/discover/discover_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Flags

Tests covering flag registry, flag finder, and flag mode classification.

### internal/flags/

- **TestProperty_FindRejectsNonSingleShort** — Flag finder rejects non-single-char shorts (internal/flags/coverage_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestAllFlagsHaveExplicitMode** — All flags have explicit mode classification (internal/flags/flags_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_FindUnknownShortReturnsNil** — Unknown short flags return nil (internal/flags/flags_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Runner & Code Generation

Tests covering the CLI runner, code generation, and help output in runner context.

### internal/runner/

- **TestCreateCodegenWithRegister** — Code generation with explicit registration (internal/runner/create_codegen_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestGoldenFile_HelpOutput** — Golden file help output validation (internal/runner/runner_help_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_ContainsHelpFlagMatchesArgs** — Help flag detection in arguments (internal/runner/runner_help_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_HelpOutputStructure** — Help output structure validation (internal/runner/runner_help_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_CodeGeneration** — Code generation with rapid (internal/runner/runner_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Git Integration

Tests covering git repository detection and clean worktree checks.

### internal/core/

- **TestProperty_CleanWorkTree** — Git clean worktree detection (internal/core/git_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_GitDetection** — Repository URL detection from git config (internal/core/git_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Examples

Tests covering example helpers and portable example compilation.

### test/ (integration)

- **TestProperty_ExampleHelpers** — Example helper functions (test/examples_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestProperty_PortableExamplesCompile** — Example code compiles (test/examples_properties_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Cluster: Dev Tooling

Tests covering development-time quality checks (not user-facing features).

### dev/

- **TestLint_NoDirectANSICodesOutsideHelp** — Ensures ANSI escape codes are only used in internal/help (dev/ansi_lint_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)
- **TestMutation** — Mutation testing with ooze framework (dev/mutation_test.go)
  - Traces to: (see architecture.md for ARCH→L2 mapping)

---

## Unspecified Code (no test coverage)

- **internal/file/** — Match, Newer, Checksum, Watch (globbing, checksumming, file watching)
- **internal/sh/** — Run, RunV, Output, RunContext, EnableCleanup (shell execution, process management)

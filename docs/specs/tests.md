# L4 Test List: targ

Bottom-up adoption of actual test functions. Each section maps tests to their traced ARCH/REQ items.

See also: [tests-uc16.md](tests-uc16.md) for UC-16 specific test specifications.

---

## ARCH-1: Root public API (`targ.go`) — Blackbox tests

Blackbox tests in `test/` exercise the public API surface (`package targ_test`).

### Property-based tests

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestProperty_EnvVarBehavior | test/arguments_properties_test.go | REQ-2-5 | Environment variable fallback for struct tag flags |
| TestProperty_HelpOutput | test/arguments_properties_test.go | REQ-6-1, REQ-6-3 | Help output contains flag descriptions and names |
| TestProperty_NameDerivation | test/arguments_properties_test.go | REQ-1-4, REQ-2-12 | camelToKebab name derivation for targets and flags |
| TestProperty_StructTagParsing | test/arguments_properties_test.go | REQ-2-1, REQ-2-2 | Struct tag parsing for flag/positional definitions |
| TestParallelFailureReportsError | test/execution_properties_test.go | REQ-11-1, REQ-12-1 | Parallel execution reports errors from failed targets |
| TestProperty_Execution | test/execution_properties_test.go | REQ-1-5, REQ-1-7, REQ-1-9, REQ-1-10, REQ-1-11 | Target execution lifecycle: invocation, timeout, repetition |
| TestProperty_Hierarchy | test/hierarchy_properties_test.go | REQ-3-1, REQ-3-2, REQ-3-3, REQ-3-5 | Group nesting, path traversal, glob matching |
| TestProperty_Completion | test/completion_properties_test.go | REQ-7-1, REQ-7-2 | Shell completion script generation (bash/zsh/fish) |
| TestProperty_CompletionSuggestions | test/completion_properties_test.go | REQ-7-3, REQ-7-4 | Dynamic completion suggestions for target names |
| TestProperty_Invariant | test/constraints_properties_test.go | REQ-1-5, REQ-1-7, REQ-2-1 | Constraint invariants: AllowDefault, type validation, error handling |
| TestProperty_ExampleHelpers | test/examples_properties_test.go | REQ-6-5 | Example helper functions (Empty/Builtin/Append/Prepend) |
| TestProperty_PortableExamplesCompile | test/examples_properties_test.go | REQ-6-5 | Built-in examples are valid and compile |
| TestProperty_Overrides | test/overrides_properties_test.go | REQ-11-1, REQ-11-2, REQ-11-3, REQ-11-9 | CLI flag overrides: --timeout, --times, --parallel, --verbose |
| TestProperty_CommandHelp | test/shell_properties_test.go | REQ-6-1, REQ-6-3, REQ-2-1 | Help output for targets with flags and positionals |
| TestProperty_ShellCommandErrors | test/shell_properties_test.go | REQ-1-8, REQ-13-1 | Shell command error propagation |
| TestProperty_ShellCommandExecution | test/shell_properties_test.go | REQ-1-8, REQ-13-1, REQ-13-2 | Shell command execution and output capture |
| TestProperty_ShellCommandFlags | test/shell_properties_test.go | REQ-2-1, REQ-2-2, REQ-2-5 | Flag parsing for shell command targets |
| TestProperty_ShellCommandHelp | test/shell_properties_test.go | REQ-6-1, REQ-1-8 | Help output for shell command targets |
| TestProperty_ShellCommandNaming | test/shell_properties_test.go | REQ-1-4, REQ-1-8 | Name derivation for shell command targets |
| TestProperty_UsageLine | test/validation_properties_test.go | REQ-6-1, REQ-6-3 | Usage line formatting with positional args |
| TestProperty_Validation | test/validation_properties_test.go | REQ-2-1, REQ-2-6 | Argument validation: required flags, type checking |
| TestProperty_GroupNameValidation | test/hierarchy_fuzz_test.go | REQ-3-1 | Group name validation rules |

### Fuzz tests

| Test Function | File | Traces to | Description |
|---|---|---|---|
| FuzzBoolFlag_ArbitraryStrings | test/arguments_fuzz_test.go | REQ-2-2 | Bool flag parsing robustness with arbitrary strings |
| FuzzExecute_ArbitraryCLIArgs | test/arguments_fuzz_test.go | REQ-1-7, REQ-2-1 | Execute does not panic with arbitrary CLI args |
| FuzzExecute_ArbitraryFlagNames | test/arguments_fuzz_test.go | REQ-2-1 | Execute handles arbitrary flag names without panic |
| FuzzExecute_ArbitraryFlagValues | test/arguments_fuzz_test.go | REQ-2-2 | Execute handles arbitrary flag values without panic |
| FuzzIntFlag_ArbitraryStrings | test/arguments_fuzz_test.go | REQ-2-2 | Int flag parsing robustness |
| FuzzMapFlag_ArbitraryKeyValueStrings | test/arguments_fuzz_test.go | REQ-2-2 | Map flag parsing robustness |
| FuzzSliceFlag_ArbitraryValues | test/arguments_fuzz_test.go | REQ-2-2 | Slice flag parsing robustness |
| FuzzTargetName_ArbitraryStrings | test/arguments_fuzz_test.go | REQ-1-4 | Target name handling with arbitrary strings |
| FuzzTimeoutFlag_ArbitraryStrings | test/arguments_fuzz_test.go | REQ-11-2 | Timeout flag parsing robustness |
| FuzzBackoff_ArbitraryParameters | test/execution_fuzz_test.go | REQ-1-10 | Backoff builder handles arbitrary parameters |
| FuzzBuilderChain_ArbitraryOrder | test/execution_fuzz_test.go | REQ-1-1 | Builder chain methods in arbitrary order |
| FuzzCache_ArbitraryPatterns | test/execution_fuzz_test.go | REQ-4-1 | Cache builder handles arbitrary patterns |
| FuzzDeps_ArbitraryDependencies | test/execution_fuzz_test.go | REQ-3-1 | Deps builder handles arbitrary dependencies |
| FuzzDescription_ArbitraryStrings | test/execution_fuzz_test.go | REQ-1-4 | Description builder handles arbitrary strings |
| FuzzShellCommand_ArbitraryCommandStrings | test/execution_fuzz_test.go | REQ-1-2 | Shell command target creation with arbitrary strings |
| FuzzTimeout_ArbitraryDurations | test/execution_fuzz_test.go | REQ-1-11 | Timeout builder handles arbitrary durations |
| FuzzTimes_ArbitraryValues | test/execution_fuzz_test.go | REQ-1-10 | Times builder handles arbitrary values |
| FuzzWatch_ArbitraryPatterns | test/execution_fuzz_test.go | REQ-5-1 | Watch builder handles arbitrary patterns |
| FuzzWhile_ArbitraryPredicates | test/execution_fuzz_test.go | REQ-1-10 | While builder handles arbitrary predicates |
| FuzzCaretReset_ArbitraryChains | test/hierarchy_fuzz_test.go | REQ-3-5 | Caret reset with arbitrary command chains |
| FuzzGlob_ArbitraryPatterns | test/hierarchy_fuzz_test.go | REQ-3-6 | Glob patterns in command args |
| FuzzGroupName_ValidPatterns | test/hierarchy_fuzz_test.go | REQ-3-1 | Group name with valid naming patterns |
| FuzzGroups_ArbitraryNesting | test/hierarchy_fuzz_test.go | REQ-3-2 | Group nesting with arbitrary depth |
| FuzzMixedRoots_TargetsAndGroups | test/hierarchy_fuzz_test.go | REQ-3-1 | Mixed targets and groups as roots |
| FuzzMultipleRoots_ArbitraryNames | test/hierarchy_fuzz_test.go | REQ-1-5 | Multiple root targets with arbitrary names |
| FuzzPathResolution_ArbitraryPathSegments | test/hierarchy_fuzz_test.go | REQ-3-5 | Path resolution with arbitrary segments |
| FuzzPathResolution_DeepNesting | test/hierarchy_fuzz_test.go | REQ-3-2, REQ-3-5 | Path resolution with deeply nested groups |

---

## ARCH-2: internal/core — Target definition, execution engine, registry

### Target definition

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestDepModeString | internal/core/target_test.go | REQ-11-1 | DepMode String() method output |
| TestProperty_DefaultIsNotRenamed | internal/core/target_test.go | REQ-1-4 | Default target name is not renamed before registration |
| TestProperty_DefaultSourceIsEmpty | internal/core/target_test.go | REQ-1-1 | Default target has empty source |
| TestProperty_DepGroupChaining | internal/core/target_test.go | REQ-3-1, REQ-11-1 | Dep group chaining with Parallel/CollectAllErrors options |
| TestProperty_DepsOnlyTargetCapturesSourceFile | internal/core/target_test.go | REQ-1-3 | Deps-only target captures caller source file |
| TestProperty_DepsOnlyTargetIsNotRenamed | internal/core/target_test.go | REQ-1-3, REQ-1-4 | Deps-only target is not renamed |
| TestProperty_FuncTargetHasNoSourceFile | internal/core/target_test.go | REQ-1-1 | Function target does not capture source file |
| TestProperty_GetSourceReturnsSetValue | internal/core/target_test.go | REQ-1-6 | GetSource returns the set sourcePkg value |
| TestProperty_NameAfterRegistrationIsRenamed | internal/core/target_test.go | REQ-1-4, REQ-1-6 | Name override after registration marks as renamed |
| TestProperty_NameBeforeRegistrationIsNotRenamed | internal/core/target_test.go | REQ-1-4 | Name set before registration is not marked renamed |
| TestProperty_ShellCommandTargetIsNotRenamed | internal/core/target_test.go | REQ-1-2, REQ-1-4 | Shell command target is not renamed |
| TestProperty_StringTargetCapturesSourceFile | internal/core/target_test.go | REQ-1-2 | String target captures caller source file |

### Command tree and parsing

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestChainExample | internal/core/command_test.go | REQ-3-5 | Chain execution (sequential targets via CLI) |
| TestParseTargetLike_LocalDepsOnlyTargetUsesSourceFile | internal/core/command_test.go | REQ-1-3 | Local deps-only target uses sourceFile for source |
| TestParseTargetLike_LocalFuncTargetKeepsExistingSourceFile | internal/core/command_test.go | REQ-1-1 | Local func target keeps existing source file |
| TestParseTargetLike_LocalStringTargetUsesSourceFile | internal/core/command_test.go | REQ-1-2 | Local string target uses sourceFile for source |
| TestParseTargetLike_RemoteFuncTargetUsesSourcePkg | internal/core/command_test.go | REQ-1-6 | Remote func target uses sourcePkg |
| TestParseTargetLike_RemoteStringTargetUsesSourcePkg | internal/core/command_test.go | REQ-1-6 | Remote string target uses sourcePkg |
| TestParseTargetLike_RemoteTargetUsesSourcePkg | internal/core/command_test.go | REQ-1-6 | Remote target uses sourcePkg for source attribution |
| TestPrintCommandHelp_BasicFuncTarget | internal/core/command_test.go | REQ-6-1 | Help output for basic function target |
| TestPrintCommandHelp_StringTarget | internal/core/command_test.go | REQ-6-1, REQ-1-2 | Help output for string (shell command) target |
| TestProperty_ConvertExamplesPreservesShape | internal/core/command_test.go | REQ-6-5 | Example conversion preserves structure |
| TestProperty_ResolveMoreInfoTextPrefersMoreInfoText | internal/core/command_test.go | REQ-6-1 | MoreInfoText takes precedence in help |
| TestProperty_CompletionExampleWithGetenv | internal/core/command_internal_test.go | REQ-7-1 | Completion example handles getenv |
| TestProperty_StructFieldNameToKebabCase | internal/core/command_internal_test.go | REQ-2-12 | Struct field names convert to kebab-case |
| TestProperty_UnrecognizedTagKeyError | internal/core/command_internal_test.go | REQ-2-1 | Unrecognized struct tag key produces error |

### Execution engine

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestExecuteEnvGetenv | internal/core/execute_test.go | DES-1-2 | ExecuteEnv getenv returns configured values |
| TestProperty_DeregisterFromAfterResolutionErrors | internal/core/execute_test.go | REQ-8-5 | Deregistration after resolution produces error |
| TestProperty_DeregisterThenReregister | internal/core/execute_test.go | REQ-8-5 | Deregister then re-register works correctly |
| TestProperty_DeregisterWithoutReregister | internal/core/execute_test.go | REQ-8-5 | Deregistration without re-registration removes targets |
| TestProperty_ExecuteRegisteredResolution_ConflictPreventsExecution | internal/core/execute_test.go | REQ-8-6 | Name conflicts prevent execution |
| TestProperty_ExecuteRegisteredResolution_DeregistrationErrorPreventsExecution | internal/core/execute_test.go | REQ-8-5 | Deregistration errors prevent execution |
| TestProperty_ExecuteRegisteredResolution_ExistingBehaviorUnchanged | internal/core/execute_test.go | REQ-1-6 | Existing registration behavior preserved |
| TestProperty_LocalTargetsHaveSourcePkgCleared | internal/core/execute_test.go | REQ-1-6 | Local targets have sourcePkg cleared |
| TestProperty_MixedLocalAndRemoteTargetsHandled | internal/core/execute_test.go | REQ-1-6, REQ-8-5 | Mixed local and remote targets handled correctly |
| TestProperty_RegisterTargetWithSkip_PreservesExplicitGroupSource | internal/core/execute_test.go | REQ-3-1, REQ-1-6 | Register preserves explicit group source |
| TestProperty_RegisterTargetWithSkip_SetsSourceOnGroups | internal/core/execute_test.go | REQ-3-1, REQ-1-6 | Register sets source on groups |
| TestProperty_RemoteTargetsKeepSourcePkg | internal/core/execute_test.go | REQ-1-6 | Remote targets retain sourcePkg |
| TestProperty_ResolveRegistryReturnsDeregisteredPackages | internal/core/execute_test.go | REQ-8-5 | Registry resolution returns deregistered package list |
| TestProperty_ResolveRegistryReturnsEmptyDeregisteredWhenNone | internal/core/execute_test.go | REQ-8-5 | No deregistrations returns empty list |

### Registry and conflict detection

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestProperty_ApplyDeregistrations_PreservesGroupsFromOtherPackages | internal/core/registry_test.go | REQ-8-5 | Deregistration preserves groups from other packages |
| TestProperty_ApplyDeregistrations_RemovesGroupsFromDeregisteredPackages | internal/core/registry_test.go | REQ-8-5 | Deregistration removes groups from deregistered packages |
| TestProperty_CleanRegistryPassesResolution | internal/core/registry_test.go | REQ-1-6 | Clean registry passes resolution |
| TestProperty_ClearLocalTargetSources_ClearsGroupsFromMainModule | internal/core/registry_test.go | REQ-1-6, REQ-3-1 | Clearing local sources affects main module groups |
| TestProperty_ClearLocalTargetSources_MixedTargetsAndGroups | internal/core/registry_test.go | REQ-1-6, REQ-3-1 | Mixed targets and groups source clearing |
| TestProperty_ClearLocalTargetSources_NoMainModuleNoChange | internal/core/registry_test.go | REQ-1-6 | No main module means no source changes |
| TestProperty_ClearLocalTargetSources_PreservesRemoteGroups | internal/core/registry_test.go | REQ-1-6, REQ-3-1 | Remote group sources preserved during local clearing |
| TestProperty_DeregisteredPackageFullyRemoved | internal/core/registry_test.go | REQ-8-5 | Deregistered package fully removed from registry |
| TestProperty_DeregistrationBeforeConflictCheck | internal/core/registry_test.go | REQ-8-5, REQ-8-6 | Deregistration applied before conflict detection |
| TestProperty_DeregistrationErrorMessage | internal/core/registry_test.go | REQ-8-5 | Deregistration error message format |
| TestProperty_DeregistrationErrorStopsResolution | internal/core/registry_test.go | REQ-8-5 | Deregistration error halts resolution |
| TestProperty_DetectConflicts_AllowsSameGroupFromSamePackage | internal/core/registry_test.go | REQ-8-6 | Same group from same package is allowed |
| TestProperty_DetectConflicts_CatchesGroupNameConflicts | internal/core/registry_test.go | REQ-8-6 | Different packages with same group name conflicts |
| TestProperty_DuplicateDeregistrationIsIdempotent | internal/core/registry_test.go | REQ-8-5 | Duplicate deregistration is idempotent |
| TestProperty_EmptyDeregistrationsNoOp | internal/core/registry_test.go | REQ-8-5 | Empty deregistration list is no-op |
| TestProperty_ErrorMessageContainsName | internal/core/registry_test.go | REQ-8-6 | Conflict error message contains target name |
| TestProperty_ErrorMessageContainsSources | internal/core/registry_test.go | REQ-8-6 | Conflict error message contains source locations |
| TestProperty_ErrorMessageSuggestsFix | internal/core/registry_test.go | REQ-8-6 | Conflict error message suggests DeregisterFrom fix |
| TestProperty_MultipleConflictsAllReported | internal/core/registry_test.go | REQ-8-6 | Multiple conflicts all reported at once |
| TestProperty_MultiplePackagesDeregistered | internal/core/registry_test.go | REQ-8-5 | Multiple packages can be deregistered |
| TestProperty_NonTargetItemsPreserved | internal/core/registry_test.go | REQ-8-5 | Non-target items preserved during deregistration |
| TestProperty_OtherPackagesUntouched | internal/core/registry_test.go | REQ-8-5 | Unrelated packages unaffected by deregistration |
| TestProperty_QueueClearedAfterResolution | internal/core/registry_test.go | REQ-8-5 | Deregistration queue cleared after resolution |
| TestProperty_SameNameDifferentSourceConflicts | internal/core/registry_test.go | REQ-8-6 | Same name from different sources conflicts |
| TestProperty_SameNameSameSourceNoConflict | internal/core/registry_test.go | REQ-8-6 | Same name from same source does not conflict |
| TestProperty_UniqueNamesNoConflict | internal/core/registry_test.go | REQ-8-6 | Unique names produce no conflicts |
| TestProperty_UnknownPackageErrors | internal/core/registry_test.go | REQ-8-5 | Unknown package deregistration produces error |

### Source attribution

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestProperty_CallerPackagePath | internal/core/source_test.go | REQ-1-6 | CallerPackagePath extracts correct package path |
| TestProperty_ExtractPackagePath | internal/core/source_test.go | REQ-1-6 | ExtractPackagePath parses file path to package |

### Parallel output and printing

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestParallelOutputDepLevel | internal/core/parallel_output_test.go | REQ-12-1, REQ-12-2 | Parallel dep output gets prefixed |
| TestParallelOutputShellCommand | internal/core/parallel_output_test.go | REQ-12-1, REQ-1-8 | Shell commands in parallel mode get prefixed output |
| TestParallelOutputTopLevel | internal/core/parallel_output_test.go | REQ-12-1, REQ-12-3 | Top-level parallel output serialization |
| TestRunContext | internal/core/parallel_output_test.go | REQ-13-1, REQ-13-2 | RunContext executes shell commands with context |
| TestRunContextInParallelMode | internal/core/parallel_output_test.go | REQ-13-1, REQ-12-1 | RunContext routes output through prefix writer in parallel |
| TestRunContextV | internal/core/parallel_output_test.go | REQ-13-1, REQ-13-3 | RunContextV prints command before executing |
| TestRunContextVInParallelMode | internal/core/parallel_output_test.go | REQ-13-1, REQ-12-1 | RunContextV in parallel mode with prefix |
| TestPrefixWriter | internal/core/prefix_writer_test.go | REQ-12-2 | PrefixWriter prepends prefix to each line |
| TestPrint | internal/core/print_test.go | REQ-12-4 | Print/Printf parallel-aware output functions |
| TestPrinter | internal/core/printer_test.go | REQ-12-3 | Printer serializes concurrent output |

### Result types

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestFormatDetailedSummary | internal/core/result_test.go | REQ-12-5 | Detailed summary formatting for execution results |
| TestMultiError | internal/core/result_test.go | REQ-11-1 | MultiError collects and formats multiple errors |
| TestResult | internal/core/result_test.go | REQ-1-13 | Result type (Pass/Fail/Errored/Cancelled) |

### Exec info

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestExecInfo | internal/core/exec_info_test.go | REQ-11-5 | ExecInfo captures execution metadata |

### Git utilities

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestProperty_CleanWorkTree | internal/core/git_test.go | REQ-4-5 | CheckCleanWorkTree detects dirty/clean state |
| TestProperty_GitDetection | internal/core/git_test.go | REQ-4-5 | Git repository detection |

### Lifecycle hooks

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestOnStartOnStop | internal/core/lifecycle_hooks_test.go | REQ-1-9, DES-2-1 | OnStart/OnStop lifecycle hooks set and fire |

### Binary mode propagation

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestBinaryModePropagation | internal/core/binary_mode_propagation_test.go | REQ-8-1, DES-8-2 | BinaryMode flows from RunOptions to help rendering |
| TestPrintUsageWithExamples | internal/core/binary_mode_propagation_test.go | REQ-6-5 | Usage printing includes examples |

### Float parsing

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestFloat64FlagParsing | internal/core/parse_float_test.go | REQ-2-2 | Float64 flag parsing handles valid and invalid inputs |

---

## ARCH-3: internal/runner — CLI tool orchestration

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestProperty_CodeGeneration | internal/runner/runner_properties_test.go | REQ-10-3, REQ-10-4 | Bootstrap main.go code generation |
| TestGoldenFile_HelpOutput | internal/runner/runner_help_test.go | REQ-6-1, DES-8-1 | Golden file comparison for targ CLI help output |
| TestProperty_ContainsHelpFlagMatchesArgs | internal/runner/runner_help_test.go | REQ-6-1, REQ-11-9 | Help flag detection in argument list |
| TestProperty_HelpOutputStructure | internal/runner/runner_help_test.go | REQ-6-1, DES-8-1 | Help output structural invariants |
| TestCreateCodegenWithRegister | internal/runner/create_codegen_test.go | REQ-10-7 | Code generation for targ create with Register pattern |

---

## ARCH-4: internal/discover — File discovery

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestProperty_Discovery | internal/discover/discover_properties_test.go | REQ-10-1, REQ-10-2, REQ-10-10 | BFS directory walk discovers tagged files, validates PackageInfo |

---

## ARCH-5: internal/parse — Utility parsing

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestProperty_Parsing | internal/parse/parse_properties_test.go | REQ-2-12, REQ-10-1 | CamelToKebab, HasBuildTag, IsGoSourceFile, ReflectTag |

---

## ARCH-6: internal/help — Help output rendering

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestBinaryModeHelpOutput | internal/help/binary_mode_test.go | REQ-6-1, DES-6-1, DES-8-2 | Help output differs between binary and library mode |
| TestFlagSectionLabel | internal/help/binary_mode_test.go | REQ-6-3, DES-6-1 | Flag section labels adapt to binary/library mode |
| TestProperty_AddGlobalFlagsFromRegistryIgnoresUnknownAndIsChainable | internal/help/builder_test.go | REQ-6-3, DES-6-2 | Global flag injection ignores unknowns, is chainable |
| TestProperty_AddPositionalsAccumulates | internal/help/builder_test.go | REQ-6-3 | Positional args accumulate in help builder |
| TestProperty_AddRootOnlyFlagsAppendsAndIsChainable | internal/help/builder_test.go | REQ-6-3, DES-6-2 | Root-only flags append and chain |
| TestProperty_NewBuilderAcceptsAnyNonEmptyCommandName | internal/help/builder_test.go | DES-6-2 | Builder accepts any non-empty command name |
| TestProperty_NewBuilderPanicsOnEmptyName | internal/help/builder_test.go | DES-6-2 | Builder panics on empty name |
| TestProperty_WithDescriptionCarriesOverCommandName | internal/help/builder_test.go | DES-6-2 | WithDescription transitions state, carries command name |
| TestProperty_WithUsageStoresValue | internal/help/builder_test.go | REQ-6-1 | WithUsage stores custom usage string |
| TestProperty_ExampleCanBeCreated | internal/help/content_test.go | REQ-6-5 | Example content type construction |
| TestProperty_FlagCanBeCreated | internal/help/content_test.go | REQ-6-3 | Flag content type construction |
| TestProperty_FormatCanBeCreated | internal/help/content_test.go | REQ-6-1 | Format content type construction |
| TestProperty_PositionalCanBeCreated | internal/help/content_test.go | REQ-6-3 | Positional content type construction |
| TestProperty_SubcommandCanBeCreated | internal/help/content_test.go | REQ-6-2 | Subcommand content type construction |
| TestAutoGeneratedRootExamples | internal/help/generators_test.go | REQ-6-5 | Auto-generated root-level examples |
| TestAutoGeneratedTargetExamples | internal/help/generators_test.go | REQ-6-5 | Auto-generated target-level examples |
| TestProperty_WriteRootHelpWithDeregisteredPackages | internal/help/generators_test.go | REQ-6-1, REQ-8-5 | Root help includes deregistered package info |
| TestProperty_StripANSI_RemovesEscapeBytes | internal/help/render_helpers_test.go | DES-6-1 | StripANSI removes raw escape bytes |
| TestProperty_StripANSI_RemovesWellFormedSequences | internal/help/render_helpers_test.go | DES-6-1 | StripANSI removes well-formed ANSI sequences |
| TestProperty_ANSICodesPairedCorrectly | internal/help/render_test.go | DES-6-1 | ANSI codes are properly opened and closed |
| TestProperty_EmptySectionsOmitted | internal/help/render_test.go | REQ-6-1 | Empty sections omitted from rendered output |
| TestProperty_ExamplesHaveNoANSICodes | internal/help/render_test.go | REQ-6-5 | Example blocks contain no ANSI codes |
| TestProperty_GlobalFlagsBeforeCommandFlags | internal/help/render_test.go | REQ-6-3 | Global flags appear before command-specific flags |
| TestProperty_NoTrailingWhitespace | internal/help/render_test.go | DES-6-1 | No trailing whitespace in rendered output |
| TestProperty_RenderIncludesValuesWhenPresent | internal/help/render_test.go | REQ-6-3 | Rendered output includes flag value placeholders |
| TestProperty_RenderSectionOrderIsCorrect | internal/help/render_test.go | REQ-6-1, DES-6-1 | Section ordering in rendered help is correct |

---

## ARCH-7: internal/flags — CLI flag definitions

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestAllFlagsHaveExplicitMode | internal/flags/flags_test.go | REQ-11-9, REQ-11-10 | Every flag definition has explicit FlagMode set |
| TestProperty_FindUnknownShortReturnsNil | internal/flags/flags_test.go | REQ-11-9 | Unknown short flag lookup returns nil |
| TestProperty_FindRejectsNonSingleShort | internal/flags/coverage_test.go | REQ-11-9 | Find rejects multi-char short flag names |

---

## ARCH-8: internal/file — File utilities

No dedicated test files. File utilities (Match, Checksum, Watch) are tested transitively through ARCH-1 blackbox tests and ARCH-2 integration.

---

## ARCH-9: internal/sh — Shell execution

No dedicated test files. Shell execution is tested transitively through ARCH-1 blackbox tests (test/shell_properties_test.go) and ARCH-2 parallel output tests.

---

## ARCH-10: cmd/targ — CLI entry point

No dedicated test files. Thin entry point (`main.go` calls `runner.Run()`); tested through ARCH-3 runner tests.

---

## Dev tooling (not traced to ARCH)

| Test Function | File | Traces to | Description |
|---|---|---|---|
| TestLint_NoDirectANSICodesOutsideHelp | dev/ansi_lint_test.go | DES-6-1 | Ensures ANSI codes only appear in internal/help |
| TestMutation | dev/mutation_test.go | (quality) | Mutation testing via ooze (build tag: mutation) |

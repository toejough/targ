# L3: Architecture (Adoption)

Induced bottom-up from L4 test clusters. Each ARCH item traces to ≥1 L4 test function.

## ARCH-1: Struct Tag Argument System

Parses Go struct fields with `targ:"..."` tags into CLI arguments (flags, positionals, env vars). Converts struct field names to kebab-case. Supports multiple value types: bool, int, float64, string, slice, map, time.Duration.

- Traces to: REQ-1, REQ-5
- L4 items: TestProperty_StructTagParsing, TestProperty_NameDerivation, TestProperty_EnvVarBehavior, TestProperty_StructFieldNameToKebabCase, TestProperty_UnrecognizedTagKeyError, TestFloat64FlagParsing, TestProperty_Parsing, FuzzBoolFlag_ArbitraryStrings, FuzzExecute_ArbitraryCLIArgs, FuzzExecute_ArbitraryFlagNames, FuzzExecute_ArbitraryFlagValues, FuzzIntFlag_ArbitraryStrings, FuzzMapFlag_ArbitraryKeyValueStrings, FuzzSliceFlag_ArbitraryValues, FuzzTargetName_ArbitraryStrings, FuzzTimeoutFlag_ArbitraryStrings

## ARCH-2: Execution Engine

Runs target functions with context, handles serial/parallel dependency modes, error aggregation (MultiError), lifecycle hooks (OnStart/OnStop), result tracking (Pass/Fail/Errored/Cancelled), and environment variable injection.

- Traces to: REQ-2
- L4 items: TestProperty_Execution, TestParallelFailureReportsError, TestExecuteEnvGetenv, TestExecInfo, TestOnStartOnStop, TestFormatDetailedSummary, TestMultiError, TestResult, FuzzBackoff_ArbitraryParameters, FuzzDeps_ArbitraryDependencies, FuzzTimeout_ArbitraryDurations, FuzzTimes_ArbitraryValues, FuzzWhile_ArbitraryPredicates

## ARCH-3: Target Hierarchy

Supports nested target groups with named paths. Validates group names. Resolves path segments for nested targets. Handles mixed roots (targets and groups at same level). Supports caret reset for hierarchy navigation.

- Traces to: REQ-6
- L4 items: TestProperty_Hierarchy, TestProperty_GroupNameValidation, FuzzCaretReset_ArbitraryChains, FuzzGlob_ArbitraryPatterns, FuzzGroupName_ValidPatterns, FuzzGroups_ArbitraryNesting, FuzzMixedRoots_TargetsAndGroups, FuzzMultipleRoots_ArbitraryNames, FuzzPathResolution_ArbitraryPathSegments, FuzzPathResolution_DeepNesting

## ARCH-4: Target Registry

Global registry for target registration (from init). Detects naming conflicts across packages. Supports deregistration of entire packages. Resolves local vs remote source packages. Clears local target source markers. Reports all conflicts at once (not fail-fast).

- Traces to: REQ-3, DES-4
- L4 items: TestProperty_CleanRegistryPassesResolution, TestProperty_SameNameSameSourceNoConflict, TestProperty_SameNameDifferentSourceConflicts, TestProperty_UniqueNamesNoConflict, TestProperty_MultipleConflictsAllReported, TestProperty_ErrorMessageContainsName, TestProperty_ErrorMessageContainsSources, TestProperty_ErrorMessageSuggestsFix, TestProperty_DeregisteredPackageFullyRemoved, TestProperty_DeregistrationBeforeConflictCheck, TestProperty_DeregistrationErrorMessage, TestProperty_DeregistrationErrorStopsResolution, TestProperty_DuplicateDeregistrationIsIdempotent, TestProperty_EmptyDeregistrationsNoOp, TestProperty_MultiplePackagesDeregistered, TestProperty_NonTargetItemsPreserved, TestProperty_OtherPackagesUntouched, TestProperty_QueueClearedAfterResolution, TestProperty_UnknownPackageErrors, TestProperty_DetectConflicts_AllowsSameGroupFromSamePackage, TestProperty_DetectConflicts_CatchesGroupNameConflicts, TestProperty_ApplyDeregistrations_PreservesGroupsFromOtherPackages, TestProperty_ApplyDeregistrations_RemovesGroupsFromDeregisteredPackages, TestProperty_ClearLocalTargetSources_ClearsGroupsFromMainModule, TestProperty_ClearLocalTargetSources_MixedTargetsAndGroups, TestProperty_ClearLocalTargetSources_NoMainModuleNoChange, TestProperty_ClearLocalTargetSources_PreservesRemoteGroups, TestProperty_DeregisterFromAfterResolutionErrors, TestProperty_DeregisterThenReregister, TestProperty_DeregisterWithoutReregister, TestProperty_ExecuteRegisteredResolution_ConflictPreventsExecution, TestProperty_ExecuteRegisteredResolution_DeregistrationErrorPreventsExecution, TestProperty_ExecuteRegisteredResolution_ExistingBehaviorUnchanged, TestProperty_LocalTargetsHaveSourcePkgCleared, TestProperty_MixedLocalAndRemoteTargetsHandled, TestProperty_RegisterTargetWithSkip_PreservesExplicitGroupSource, TestProperty_RegisterTargetWithSkip_SetsSourceOnGroups, TestProperty_ResolveRegistryReturnsDeregisteredPackages, TestProperty_ResolveRegistryReturnsEmptyDeregisteredWhenNone

## ARCH-5: Help System

Builder-pattern API for constructing help text. Renders structured sections (usage, description, subcommands, flags, examples, positionals). ANSI styling via lipgloss. Properly pairs ANSI codes. Omits empty sections. Strips trailing whitespace. Supports binary mode (filters targ-specific flags). Auto-generates examples.

- Traces to: REQ-4, DES-3
- L4 items: TestProperty_NewBuilderAcceptsAnyNonEmptyCommandName, TestProperty_NewBuilderPanicsOnEmptyName, TestProperty_WithDescriptionCarriesOverCommandName, TestProperty_WithUsageStoresValue, TestProperty_AddGlobalFlagsFromRegistryIgnoresUnknownAndIsChainable, TestProperty_AddPositionalsAccumulates, TestProperty_AddRootOnlyFlagsAppendsAndIsChainable, TestProperty_ExampleCanBeCreated, TestProperty_FlagCanBeCreated, TestProperty_FormatCanBeCreated, TestProperty_PositionalCanBeCreated, TestProperty_SubcommandCanBeCreated, TestAutoGeneratedRootExamples, TestAutoGeneratedTargetExamples, TestProperty_ANSICodesPairedCorrectly, TestProperty_EmptySectionsOmitted, TestProperty_ExamplesHaveNoANSICodes, TestProperty_GlobalFlagsBeforeCommandFlags, TestProperty_NoTrailingWhitespace, TestProperty_RenderIncludesValuesWhenPresent, TestProperty_RenderSectionOrderIsCorrect, TestProperty_StripANSI_RemovesEscapeBytes, TestProperty_StripANSI_RemovesWellFormedSequences, TestBinaryModeHelpOutput, TestFlagSectionLabel, TestProperty_HelpOutput

## ARCH-6: Shell Command Targets

Targets can be defined from shell command strings (not just Go functions). Shell commands support flag handling, naming, help output, error reporting, and execution.

- Traces to: REQ-1, DES-1
- L4 items: TestProperty_ShellCommandExecution, TestProperty_ShellCommandErrors, TestProperty_ShellCommandFlags, TestProperty_ShellCommandHelp, TestProperty_ShellCommandNaming, TestProperty_CommandHelp, FuzzShellCommand_ArbitraryCommandStrings

## ARCH-7: Shell Completion

Generates shell completion scripts and suggestions for targets, flags, and arguments.

- Traces to: REQ-7, DES-3
- L4 items: TestProperty_Completion, TestProperty_CompletionSuggestions, TestProperty_CompletionExampleWithGetenv

## ARCH-8: Input Validation

Validates CLI input against field constraints (required fields, enum values). Generates usage lines. Reports validation errors.

- Traces to: REQ-5
- L4 items: TestProperty_Validation, TestProperty_UsageLine, TestProperty_Invariant

## ARCH-9: Runtime Overrides

CLI flags (--cache, --watch, --times, etc.) override target configuration at runtime.

- Traces to: REQ-8, DES-2
- L4 items: TestProperty_Overrides

## ARCH-10: Parallel Output

Parallel-safe output routing. Prefix writer adds target name prefixes to output lines. Printer maintains ordering and flushing for interleaved output. Context-aware Print/Printf functions route through parallel or serial paths.

- Traces to: REQ-2, DES-2
- L4 items: TestParallelOutputTopLevel, TestParallelOutputDepLevel, TestParallelOutputShellCommand, TestRunContext, TestRunContextInParallelMode, TestRunContextV, TestRunContextVInParallelMode, TestPrefixWriter, TestPrinter, TestPrint

## ARCH-11: Target Builder

Fluent API for target construction. Targets support naming, descriptions, dependencies, caching, watching, timeouts, and repetition. Tracks source file/package for local vs remote targets. Supports deps-only targets (no function, just dependencies).

- Traces to: REQ-1, DES-1
- L4 items: TestDepModeString, TestProperty_DefaultIsNotRenamed, TestProperty_DefaultSourceIsEmpty, TestProperty_DepGroupChaining, TestProperty_DepsOnlyTargetCapturesSourceFile, TestProperty_DepsOnlyTargetIsNotRenamed, TestProperty_FuncTargetHasNoSourceFile, TestProperty_GetSourceReturnsSetValue, TestProperty_NameAfterRegistrationIsRenamed, TestProperty_NameBeforeRegistrationIsNotRenamed, TestProperty_ShellCommandTargetIsNotRenamed, TestProperty_StringTargetCapturesSourceFile, TestChainExample, TestParseTargetLike_LocalDepsOnlyTargetUsesSourceFile, TestParseTargetLike_LocalFuncTargetKeepsExistingSourceFile, TestParseTargetLike_LocalStringTargetUsesSourceFile, TestParseTargetLike_RemoteFuncTargetUsesSourcePkg, TestParseTargetLike_RemoteStringTargetUsesSourcePkg, TestParseTargetLike_RemoteTargetUsesSourcePkg, TestPrintCommandHelp_BasicFuncTarget, TestPrintCommandHelp_StringTarget, TestProperty_ConvertExamplesPreservesShape, TestProperty_ResolveMoreInfoTextPrefersMoreInfoText, FuzzBuilderChain_ArbitraryOrder, FuzzCache_ArbitraryPatterns, FuzzDescription_ArbitraryStrings, FuzzWatch_ArbitraryPatterns

## ARCH-12: Source Tracking & Discovery

Extracts caller package paths from runtime stack. Discovers target files by `//go:build targ` tags. Distinguishes local vs remote packages.

- Traces to: REQ-3, REQ-9, DES-4, DES-5
- L4 items: TestProperty_CallerPackagePath, TestProperty_ExtractPackagePath, TestProperty_Discovery

## ARCH-13: Flag Registry

Central registry of known flags (--cache, --watch, etc.) with short forms, modes (targ-only, binary, universal), and placeholder substitution.

- Traces to: REQ-8, DES-4
- L4 items: TestProperty_FindRejectsNonSingleShort, TestAllFlagsHaveExplicitMode, TestProperty_FindUnknownShortReturnsNil

## ARCH-14: CLI Runner

Entry point for `targ` binary. Discovers targ files, compiles temporary binaries, invokes with arguments. Supports `--create` (scaffolding), `--sync` (remote import). Generates Go code with AST manipulation.

- Traces to: REQ-9, DES-5
- L4 items: TestCreateCodegenWithRegister, TestGoldenFile_HelpOutput, TestProperty_ContainsHelpFlagMatchesArgs, TestProperty_HelpOutputStructure, TestProperty_CodeGeneration

## ARCH-15: Git Integration

Detects git repository URL from config. Checks worktree cleanliness for pre-build guards.

- Traces to: REQ-10
- L4 items: TestProperty_CleanWorkTree, TestProperty_GitDetection

## ARCH-16: Examples

Example helpers and portable example compilation verification. Ensures example code in examples/ compiles correctly.

- Traces to: REQ-11
- L4 items: TestProperty_ExampleHelpers, TestProperty_PortableExamplesCompile

## ARCH-17: Dev Quality (standard)

Lint rules (ANSI code containment) and mutation testing. Not a user-facing feature — enforces internal code quality.

- Source: standard
- Rationale: Internal quality enforcement — ensures ANSI codes stay in help package and mutation testing validates test effectiveness.
- L4 items: TestLint_NoDirectANSICodesOutsideHelp, TestMutation

# L4: Test List

Bottom-up adoption from existing test suite. Test function names are item IDs.

## Argument Parsing Tests

Traces to: ARCH-2 (Command Parser), ARCH-1 (Target Type System)

### test/arguments_properties_test.go
- **TestProperty_StructTagParsing** — Struct tag parsing for flags, positionals, shorts, maps, slices, embedded structs, defaults, required, grouped flags, enums
- **TestProperty_EnvVarBehavior** — Env var fallback and override by flag, TextUnmarshaler, StringSetter
- **TestProperty_NameDerivation** — Automatic name derivation (acronyms, multi-word)
- **TestProperty_HelpOutput** — Help formatting for positionals, dep groups (serial/parallel/chained)

### test/arguments_fuzz_test.go
- **FuzzBoolFlag_ArbitraryStrings** — Bool parsing robustness
- **FuzzExecute_ArbitraryCLIArgs** — Execute with arbitrary CLI args
- **FuzzExecute_ArbitraryFlagNames** — Execute with arbitrary flag names
- **FuzzExecute_ArbitraryFlagValues** — Execute with arbitrary struct fields and values
- **FuzzIntFlag_ArbitraryStrings** — Int parsing robustness
- **FuzzMapFlag_ArbitraryKeyValueStrings** — Map flag parsing robustness
- **FuzzSliceFlag_ArbitraryValues** — Slice flag parsing robustness
- **FuzzTargetName_ArbitraryStrings** — Target name special characters
- **FuzzTimeoutFlag_ArbitraryStrings** — Duration parsing robustness

### test/validation_properties_test.go
- **TestProperty_Validation** — Function signature validation, target type validation
- **TestProperty_UsageLine** — Usage line formatting (positional, optional, variadic, enum wrapping)

### test/constraints_properties_test.go
- **TestProperty_Invariant** — Error handling invariants: duplicate names, empty strings, nil targets, unknown commands, invalid flags, invalid durations, help-does-not-execute

### internal/core/command_internal_test.go
- **TestProperty_CompletionExampleWithGetenv** — Shell detection for completion examples
- **TestProperty_StructFieldNameToKebabCase** — Struct field to kebab-case conversion
- **TestProperty_UnrecognizedTagKeyError** — Unknown struct tag keys produce errors

### internal/core/parse_float_test.go
- **TestFloat64FlagParsing** — Float64 flag value parsing

### internal/flags/flags_test.go
- **TestAllFlagsHaveExplicitMode** — All flags declare FlagMode
- **TestProperty_FindUnknownShortReturnsNil** — Unknown short flags return nil

### internal/flags/coverage_test.go
- **TestProperty_FindRejectsNonSingleShort** — Non-single-char short flags rejected

### internal/parse/parse_properties_test.go
- **TestProperty_Parsing** — CamelToKebab, HasBuildTag, IsGoSourceFile, ShouldSkipGoFile, ShouldSkipDir, ReflectTag, ReturnStringLiteral, TargImportInfo

## Execution & Control Tests

Traces to: ARCH-3 (Execution Engine)

### test/execution_properties_test.go
- **TestParallelFailureReportsError** — Parallel dep failure reporting
- *(40+ subtests covering deps, timeout, retry, backoff, times, while, cache, watch, parallel execution)*

### test/execution_fuzz_test.go
- **FuzzBackoff_ArbitraryParameters** — Backoff parameter robustness
- **FuzzBuilderChain_ArbitraryOrder** — Builder method chaining order
- **FuzzCache_ArbitraryPatterns** — Cache pattern robustness
- **FuzzDeps_ArbitraryDependencies** — Nil/empty dependency handling
- **FuzzDescription_ArbitraryStrings** — Description string robustness
- **FuzzShellCommand_ArbitraryCommandStrings** — Shell command string robustness
- **FuzzTimeout_ArbitraryDurations** — Timeout duration robustness
- **FuzzTimes_ArbitraryValues** — Times parameter robustness
- **FuzzWatch_ArbitraryPatterns** — Watch pattern robustness
- **FuzzWhile_ArbitraryPredicates** — While predicate robustness

### test/overrides_properties_test.go
- **TestProperty_Overrides** — CLI override flags: --timeout, --times, --retry, --backoff, --while, --cache, --watch, --dep-mode, --cache-dir, --deps; conflict detection with targ.Disabled

## Hierarchy & Shell Command Tests

Traces to: ARCH-1 (Target Type System), ARCH-2 (Command Parser)

### test/hierarchy_properties_test.go
- **TestProperty_Hierarchy** — Group nesting, glob patterns, exact match, namespace nodes, caret reset, list command
- **TestProperty_GroupNameValidation** — Valid/invalid group names (empty, digit-start, uppercase, spaces, dash-start)

### test/hierarchy_fuzz_test.go
- **FuzzCaretReset_ArbitraryChains** — Caret reset robustness
- **FuzzGlob_ArbitraryPatterns** — Glob pattern robustness
- **FuzzGroupName_ValidPatterns** — Group name validation robustness
- **FuzzGroups_ArbitraryNesting** — Nested group robustness
- **FuzzMixedRoots_TargetsAndGroups** — Mixed root robustness
- **FuzzMultipleRoots_ArbitraryNames** — Multiple root name robustness
- **FuzzPathResolution_ArbitraryPathSegments** — Path resolution robustness
- **FuzzPathResolution_DeepNesting** — Deep nesting robustness

### test/shell_properties_test.go
- **TestProperty_ShellCommandExecution** — Variable substitution, failure returns error
- **TestProperty_ShellCommandErrors** — Missing var, unknown long/short flags
- **TestProperty_ShellCommandFlags** — Long, equals, short flag parsing
- **TestProperty_ShellCommandHelp** — Shows variables, includes description
- **TestProperty_ShellCommandNaming** — Name derived from command string
- **TestProperty_CommandHelp** — Help shows flags, usage, subcommands, enums, cache/watch patterns

## Help & Completion Tests

Traces to: ARCH-5 (Help Renderer), ARCH-6 (Completion Engine)

### test/completion_properties_test.go
- **TestProperty_Completion** — Script generation (bash/zsh/fish), unsupported shell, empty env, Windows path, unmatched root, help example, does-not-execute, disabled rejection
- **TestProperty_CompletionSuggestions** — __complete suggestions: subcommands, roots, flags, enums, quotes, edge cases (45+ subtests)

### test/examples_properties_test.go
- **TestProperty_ExampleHelpers** — EmptyExamples, BuiltinExamples, Append/Prepend
- **TestProperty_PortableExamplesCompile** — Examples compile with targ_example tag

### internal/help/render_test.go
- **TestProperty_ANSICodesPairedCorrectly** — ANSI escape pairing
- **TestProperty_EmptySectionsOmitted** — Empty sections not rendered
- **TestProperty_ExamplesHaveNoANSICodes** — Example code ANSI-free
- **TestProperty_GlobalFlagsBeforeCommandFlags** — Flag section ordering
- **TestProperty_NoTrailingWhitespace** — No trailing whitespace
- **TestProperty_RenderIncludesValuesWhenPresent** — Values rendered when present
- **TestProperty_RenderSectionOrderIsCorrect** — Correct section ordering

### internal/help/render_helpers_test.go
- **TestProperty_StripANSI_RemovesEscapeBytes** — ANSI byte removal
- **TestProperty_StripANSI_RemovesWellFormedSequences** — ANSI sequence removal

### internal/help/binary_mode_test.go
- **TestBinaryModeHelpOutput** — Binary mode root/target help, examples use binary name, flag mode
- **TestFlagSectionLabel** — Targ mode "Global Flags" vs binary mode "Flags"

### internal/help/builder_test.go
- **TestProperty_AddGlobalFlagsFromRegistryIgnoresUnknownAndIsChainable** — Global flags builder
- **TestProperty_AddPositionalsAccumulates** — Positional accumulation
- **TestProperty_AddRootOnlyFlagsAppendsAndIsChainable** — Root-only flags
- **TestProperty_NewBuilderAcceptsAnyNonEmptyCommandName** — Builder name acceptance
- **TestProperty_NewBuilderPanicsOnEmptyName** — Empty name panic
- **TestProperty_WithDescriptionCarriesOverCommandName** — Description chaining
- **TestProperty_WithUsageStoresValue** — Usage storage

### internal/help/content_test.go
- **TestProperty_ExampleCanBeCreated** — Example struct creation
- **TestProperty_FlagCanBeCreated** — Flag struct creation
- **TestProperty_FormatCanBeCreated** — Format struct creation
- **TestProperty_PositionalCanBeCreated** — Positional struct creation
- **TestProperty_SubcommandCanBeCreated** — Subcommand struct creation

### internal/help/generators_test.go
- **TestAutoGeneratedRootExamples** — Targ/binary mode root examples, user examples replace
- **TestAutoGeneratedTargetExamples** — Target examples with positionals, duration/backoff placeholders, user examples replace
- **TestProperty_WriteRootHelpWithDeregisteredPackages** — Deregistered packages in help

## Registration & Conflict Tests

Traces to: ARCH-4 (Registry)

### internal/core/registry_test.go
- **TestProperty_CleanRegistryPassesResolution** — Clean registry resolves
- **TestProperty_SameNameSameSourceNoConflict** — Same source no conflict
- **TestProperty_SameNameDifferentSourceConflicts** — Different source conflicts
- **TestProperty_UniqueNamesNoConflict** — Unique names no conflict
- **TestProperty_MultipleConflictsAllReported** — All conflicts reported
- **TestProperty_ErrorMessageContainsName** — Error includes name
- **TestProperty_ErrorMessageContainsSources** — Error includes sources
- **TestProperty_ErrorMessageSuggestsFix** — Error suggests DeregisterFrom
- **TestProperty_NonTargetItemsPreserved** — Non-target items preserved
- **TestProperty_DeregisteredPackageFullyRemoved** — Deregistered package removed
- **TestProperty_OtherPackagesUntouched** — Other packages preserved
- **TestProperty_DuplicateDeregistrationIsIdempotent** — Idempotent deregistration
- **TestProperty_EmptyDeregistrationsNoOp** — Empty deregistration no-op
- **TestProperty_UnknownPackageErrors** — Unknown package errors
- **TestProperty_MultiplePackagesDeregistered** — Multiple package deregistration
- **TestProperty_QueueClearedAfterResolution** — Queue cleared after resolution
- **TestProperty_DeregistrationBeforeConflictCheck** — Deregistration before conflicts
- **TestProperty_DeregistrationErrorMessage** — Error message format
- **TestProperty_DeregistrationErrorStopsResolution** — Error stops resolution
- **TestProperty_ApplyDeregistrations_PreservesGroupsFromOtherPackages** — Group preservation
- **TestProperty_ApplyDeregistrations_RemovesGroupsFromDeregisteredPackages** — Group removal
- **TestProperty_DetectConflicts_AllowsSameGroupFromSamePackage** — Group conflict allowed
- **TestProperty_DetectConflicts_CatchesGroupNameConflicts** — Group conflict detected
- **TestProperty_ClearLocalTargetSources_ClearsGroupsFromMainModule** — Local source clearing
- **TestProperty_ClearLocalTargetSources_PreservesRemoteGroups** — Remote group preservation
- **TestProperty_ClearLocalTargetSources_NoMainModuleNoChange** — No module no change
- **TestProperty_ClearLocalTargetSources_MixedTargetsAndGroups** — Mixed source clearing

### internal/core/execute_test.go
- **TestExecuteEnvGetenv** — Env var retrieval from map and OS fallback
- **TestProperty_ExecuteRegisteredResolution_ExistingBehaviorUnchanged** — Registry resolution behavior
- **TestProperty_ExecuteRegisteredResolution_ConflictPreventsExecution** — Conflict prevents execution
- **TestProperty_ExecuteRegisteredResolution_DeregistrationErrorPreventsExecution** — Deregistration error prevents execution
- **TestProperty_DeregisterFromAfterResolutionErrors** — Late deregistration errors
- **TestProperty_DeregisterThenReregister** — Deregister-then-reregister workflow
- **TestProperty_DeregisterWithoutReregister** — Deregister without reregister removes
- **TestProperty_LocalTargetsHaveSourcePkgCleared** — Local target source clearing
- **TestProperty_MixedLocalAndRemoteTargetsHandled** — Mixed target handling
- **TestProperty_RegisterTargetWithSkip_PreservesExplicitGroupSource** — Explicit group source
- **TestProperty_RegisterTargetWithSkip_SetsSourceOnGroups** — Source set on groups
- **TestProperty_RemoteTargetsKeepSourcePkg** — Remote source preservation
- **TestProperty_ResolveRegistryReturnsDeregisteredPackages** — Deregistered package list
- **TestProperty_ResolveRegistryReturnsEmptyDeregisteredWhenNone** — Empty deregistered list

## Build Tool Tests

Traces to: ARCH-7 (Build Tool Runner), ARCH-8 (Discovery)

### internal/discover/discover_properties_test.go
- **TestProperty_Discovery** — Finds tagged files, detects registration, aliased imports, rejects main, skips test/generated/special files, recursive, package doc, sorted, multiple packages rejected, default tag/dir

### internal/runner/runner_properties_test.go
- **TestProperty_CodeGeneration** — Valid/invalid target names, code generation, duplicate rejection, cache/watch patterns, find targ files, add target, create new file, init creation, import/deregister, kebab-to-pascal, create arg parsing (deps, cache, watch, timeout, times, retry, backoff, dep-mode), sync args, help request parsing, namespace paths

### internal/runner/runner_help_test.go
- **TestGoldenFile_HelpOutput** — Golden file tests for create, sync, to-func, to-string
- **TestProperty_ContainsHelpFlagMatchesArgs** — Help flag detection
- **TestProperty_HelpOutputStructure** — Help structure for create, sync, to-func, to-string

### internal/runner/create_codegen_test.go
- **TestCreateCodegenWithRegister** — Init with targ.Register generation

## Target Type & Utility Tests

Traces to: ARCH-1 (Target Type System), ARCH-12 (Source Tracking)

### internal/core/target_test.go
- **TestDepModeString** — Serial/Parallel/Mixed/Default string representation
- **TestProperty_DepGroupChaining** — Single serial/parallel, coalescing, mixed mode, GetDeps flattens, no deps empty
- **TestProperty_DefaultIsNotRenamed** — New targets not renamed
- **TestProperty_DefaultSourceIsEmpty** — New targets empty source
- **TestProperty_DepsOnlyTargetCapturesSourceFile** — Deps-only source capture
- **TestProperty_DepsOnlyTargetIsNotRenamed** — Deps-only not renamed
- **TestProperty_FuncTargetHasNoSourceFile** — Func targets no source file
- **TestProperty_GetSourceReturnsSetValue** — GetSource returns set value
- **TestProperty_NameAfterRegistrationIsRenamed** — Name() after registration sets IsRenamed
- **TestProperty_NameBeforeRegistrationIsNotRenamed** — Name() before registration no IsRenamed
- **TestProperty_ShellCommandTargetIsNotRenamed** — Shell targets not renamed
- **TestProperty_StringTargetCapturesSourceFile** — String targets capture source

### internal/core/command_test.go
- **TestChainExample** — Nil nodes fallback, nested group caret syntax, flat two-source names
- **TestParseTargetLike_Local*** — Local func/string/deps-only targets capture source file
- **TestParseTargetLike_Remote*** — Remote func/string targets use source package
- **TestPrintCommandHelp_BasicFuncTarget** — Help for function targets
- **TestPrintCommandHelp_StringTarget** — Help for string targets
- **TestProperty_ConvertExamplesPreservesShape** — Example conversion shape
- **TestProperty_ResolveMoreInfoTextPrefersMoreInfoText** — More-info text resolution

### internal/core/source_test.go
- **TestProperty_CallerPackagePath** — Invalid depth returns error
- **TestProperty_ExtractPackagePath** — Path extraction (prefix, no dot, empty, known)

### internal/core/git_test.go
- **TestProperty_CleanWorkTree** — Clean/modified/untracked/staged detection, command args, whitespace handling
- **TestProperty_GitDetection** — Repo URL detection, git config parsing, SSH→HTTPS normalization, .git suffix removal, error handling

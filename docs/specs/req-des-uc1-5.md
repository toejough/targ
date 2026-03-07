# L2 Requirements and Design: UC-1 through UC-5

## UC-1: Define and run targets

### Requirements

#### REQ-1-1: Function target creation
- **Traces to:** UC-1
- **Source:** `internal/core/target.go:648-682` (`Targ()`)
- **Acceptance criteria:** `Targ(fn)` accepts a Go function and returns a `*Target`. The function must be non-nil and of kind `reflect.Func`; any other type panics with "expected func or string".

#### REQ-1-2: Shell command target creation
- **Traces to:** UC-1
- **Source:** `internal/core/target.go:665-673` (`Targ()` string branch)
- **Acceptance criteria:** `Targ("cmd")` accepts a non-empty string and returns a `*Target` with `fn` set to the string. Empty strings panic. The `sourceFile` is captured via `runtime.Caller`.

#### REQ-1-3: Deps-only target creation
- **Traces to:** UC-1
- **Source:** `internal/core/target.go:649-653` (`Targ()` zero-arg branch)
- **Acceptance criteria:** `Targ()` with no arguments returns a `*Target` with `fn == nil`. The `sourceFile` is captured. The target executes only its dependencies.

#### REQ-1-4: Target naming
- **Traces to:** UC-1
- **Source:** `internal/core/target.go:241-258` (`GetName()`), `internal/core/target.go:302-311` (`Name()`)
- **Acceptance criteria:** Default name is derived from the function name via `camelToKebab`. `Name(s)` overrides it. If the target has `sourcePkg` set (registered), `Name()` marks `nameOverridden = true`.

#### REQ-1-5: Target execution via Main
- **Traces to:** UC-1
- **Source:** `internal/core/execute.go:72-80` (`Main()`)
- **Acceptance criteria:** `Main(targets...)` registers targets, creates an `osRunEnv`, and calls `ExecuteWithResolution` with `AllowDefault: true, BinaryMode: true`. The process exits on error.

#### REQ-1-6: Target execution via Register + ExecuteRegistered
- **Traces to:** UC-1
- **Source:** `internal/core/execute.go:36-40` (`ExecuteRegistered()`), `internal/core/execute.go:86-97` (`RegisterTarget`, `RegisterTargetWithSkip`)
- **Acceptance criteria:** `Register(targets...)` adds targets to a global registry with `sourcePkg` attribution via `runtime.Caller`. `ExecuteRegistered()` resolves the registry (applying deregistrations) and runs via `osRunEnv` with `AllowDefault: true`.

#### REQ-1-7: Function target invocation
- **Traces to:** UC-1
- **Source:** `internal/core/target.go:703-748` (`callFunc`)
- **Acceptance criteria:** Function targets are called via reflection. `context.Context` parameters are injected automatically. The last return value is checked for `error` interface; non-nil errors propagate. Missing args get zero values.

#### REQ-1-8: Shell command invocation
- **Traces to:** UC-1
- **Source:** `internal/core/target.go:1030-1044` (`runShellCommand`)
- **Acceptance criteria:** Shell commands execute via `sh -c <cmd>`. In parallel mode, stdout/stderr route through `PrefixWriter`. Errors are wrapped with "shell command failed:".

#### REQ-1-9: Execution lifecycle (runOnce)
- **Traces to:** UC-1
- **Source:** `internal/core/target.go:519-551` (`runOnce`)
- **Acceptance criteria:** A single execution applies timeout (if set), runs dependencies, checks cache (skipping if cache hit), then runs the target with repetition handling. Order is: timeout → deps → cache → execute.

#### REQ-1-10: Repetition (Times, While, Retry, Backoff)
- **Traces to:** UC-1
- **Source:** `internal/core/target.go:554-587` (`runWithRepetition`)
- **Acceptance criteria:** `Times(n)` sets iteration count. `While(fn)` is checked before each iteration. Without `Retry()`, first error stops execution. With `Retry()`, execution continues on failure but stops on first success. `Backoff(initial, factor)` applies exponential delay between retries.

#### REQ-1-11: Timeout enforcement
- **Traces to:** UC-1
- **Source:** `internal/core/target.go:521-526`
- **Acceptance criteria:** `Timeout(d)` wraps the context with `context.WithTimeout`. When exceeded, the context cancels and the target receives `context.DeadlineExceeded`.

#### REQ-1-12: Signal handling
- **Traces to:** UC-1
- **Source:** `internal/core/run_env.go:853-856` (`setupContext`)
- **Acceptance criteria:** When `env.SupportsSignals()` is true, the execution context is wrapped with `signal.NotifyContext` for `os.Interrupt` and `syscall.SIGTERM`. Cancellation propagates to all running targets.

#### REQ-1-13: Exit code propagation
- **Traces to:** UC-1
- **Source:** `internal/core/run_env.go:186-197` (`RunWithEnv`)
- **Acceptance criteria:** Errors are converted to exit codes: `ExitError{Code: N}` exits with code N; other errors exit with code 1. `env.Exit()` is called to allow test environments to capture exit codes.

### Design

#### DES-1-1: Target definition API
- **Traces to:** UC-1
- **Interaction model:** Developer calls `targ.Targ(fn)`, `targ.Targ("cmd")`, or `targ.Targ()` at package level. Chaining methods (`.Name()`, `.Description()`, `.Timeout()`, `.Times()`, `.Retry()`, `.Backoff()`, `.While()`) configure behavior. Targets are passed to `targ.Main()` or registered via `targ.Register()` in `init()`.

#### DES-1-2: Execution environment abstraction
- **Traces to:** UC-1
- **Interaction model:** The `RunEnv` interface abstracts OS interactions (args, stdout, exit, env vars, signals). Production uses `osRunEnv`; tests use `ExecuteEnv` which captures output and exit codes. `targ.Execute()` provides a test-friendly entry point that returns `ExecuteResult`.

---

## UC-2: CLI argument parsing

### Requirements

#### REQ-2-1: Struct tag parsing
- **Traces to:** UC-2
- **Source:** `internal/core/command.go:376-407` (`applyTagPart`), `internal/core/types.go:102-113` (`TagOptions`)
- **Acceptance criteria:** Struct fields with `targ:"..."` tags are parsed into `TagOptions`. Recognized tag parts: `name=`, `short=`, `env=`, `default=`, `enum=`, `placeholder=`, `desc=`/`description=`, `required`. Unrecognized keys produce an error.

#### REQ-2-2: Tag kind classification
- **Traces to:** UC-2
- **Source:** `internal/core/types.go:10-14` (`TagKind` constants)
- **Acceptance criteria:** Fields are classified as `TagKindFlag` (named --flags), `TagKindPositional` (unnamed positional args), or `TagKindUnknown`. The kind determines parsing behavior.

#### REQ-2-3: Flag parsing
- **Traces to:** UC-2
- **Source:** `internal/core/command.go:440-453` (`buildFlagMaps`), `internal/core/command.go:197-206` (`flagSpec`)
- **Acceptance criteria:** Flag fields support long names (`--name`), short aliases (`-n`), required validation, defaults, environment variable fallback, and enum validation. Boolean flags support short-flag grouping (e.g., `-abc`).

#### REQ-2-4: Positional argument parsing
- **Traces to:** UC-2
- **Source:** `internal/core/command.go:776-800` (`collectPositionalHelp`)
- **Acceptance criteria:** Positional fields are consumed in declaration order. Required positionals must appear before optional ones. Unexported positional fields produce an error.

#### REQ-2-5: Default and environment variable resolution
- **Traces to:** UC-2
- **Source:** `internal/core/command.go:296-326` (`applyDefaultsAndEnv`)
- **Acceptance criteria:** For unvisited flags: environment variables are checked first (via `os.Getenv`), then defaults. Both are applied via `setFieldFromString`. Invalid values produce descriptive errors.

#### REQ-2-6: Required flag validation
- **Traces to:** UC-2
- **Source:** `internal/core/command.go:662-681` (`checkRequiredFlags`)
- **Acceptance criteria:** Required flags that are not set by CLI args, env vars, or defaults produce `errMissingRequiredFlag` with the flag name (including short alias if present).

#### REQ-2-7: Unknown flag validation
- **Traces to:** UC-2
- **Source:** `internal/core/command.go:685-705` (`checkUnknownFlags`)
- **Acceptance criteria:** Remaining args starting with `--` or `-` that don't match known flags produce `errFlagNotDefined` with the flag name.

#### REQ-2-8: Shell command variable extraction
- **Traces to:** UC-2
- **Source:** `internal/core/command.go:102-103` (`shellVarPattern`), `internal/core/command.go:122-124` (`ShellVars`)
- **Acceptance criteria:** Shell command strings have `$var` and `${var}` patterns extracted as lowercase variable names. These become CLI flags automatically (e.g., `$namespace` → `--namespace`).

#### REQ-2-9: Function signature validation
- **Traces to:** UC-2
- **Source:** `internal/core/command.go:80-84` (error variables)
- **Acceptance criteria:** Function targets are validated: must accept `context.Context` or be niladic (with optional struct param). Must return only `error` or nothing. Violations produce descriptive errors.

#### REQ-2-10: TagOptions override method
- **Traces to:** UC-2
- **Source:** `internal/core/command.go:328-374` (`applyTagOptionsOverride`)
- **Acceptance criteria:** If the args struct has a `TagOptions(fieldName string, opts TagOptions) (TagOptions, error)` method, it is called per field to allow programmatic override of tag options. Signature is validated; mismatches produce errors.

#### REQ-2-11: Interleaved positional arguments
- **Traces to:** UC-2
- **Source:** `internal/core/types.go:29-32` (`Interleaved[T]`)
- **Acceptance criteria:** Fields of type `Interleaved[T]` capture both the value and its position index among all positional arguments, enabling mixed positional/flag argument ordering.

#### REQ-2-12: CamelCase to kebab-case conversion
- **Traces to:** UC-2
- **Source:** `internal/parse/parse.go:57-75` (`CamelToKebab`)
- **Acceptance criteria:** Function names and struct field names are converted: lowercase after uppercase transitions get hyphens. Acronyms are handled (e.g., `APIServer` → `api-server`).

### Design

#### DES-2-1: Struct-tag-driven CLI
- **Traces to:** UC-2
- **Interaction model:** Developer defines a struct with `targ:"..."` tags on fields, then passes a function accepting that struct to `targ.Targ(fn)`. At runtime, CLI args are parsed into the struct and the function is called with the populated instance. Example: `func Build(ctx context.Context, args struct { Verbose bool `targ:"short=v"` }) error`.

#### DES-2-2: Shell variable auto-flagging
- **Traces to:** UC-2
- **Interaction model:** Developer creates a shell command target with `$var` placeholders: `targ.Targ("kubectl apply -n $namespace")`. The framework extracts variable names and auto-generates `--namespace` flags. Short flags are auto-assigned from the first letter of each variable name.

---

## UC-3: Dependency management

### Requirements

#### REQ-3-1: Dependency group creation
- **Traces to:** UC-3
- **Source:** `internal/core/target.go:132-169` (`Deps()`)
- **Acceptance criteria:** `.Deps(args...)` accepts `*Target`, `DepMode`, and `DepOption` values. Targets are collected into a `depGroup`. Default mode is `DepModeSerial`. Consecutive `.Deps()` calls with the same mode and collectAll setting coalesce into a single group; different modes create separate groups.

#### REQ-3-2: Serial dependency execution
- **Traces to:** UC-3
- **Source:** `internal/core/target.go:1016-1025` (`runGroupSerial`)
- **Acceptance criteria:** Serial deps execute one at a time in declaration order. First error stops execution and propagates.

#### REQ-3-3: Parallel dependency execution
- **Traces to:** UC-3
- **Source:** `internal/core/target.go:783-901` (`runGroupParallel`)
- **Acceptance criteria:** Passing `targ.Parallel` as the last arg to `.Deps()` runs all targets concurrently. First failure cancels remaining targets via context. Results are classified as Pass/Fail/Cancelled/Errored. Output is prefixed with `[target-name]` and serialized through a `Printer`.

#### REQ-3-4: Collect-all-errors mode
- **Traces to:** UC-3
- **Source:** `internal/core/target.go:904-1014` (`runGroupParallelAll`)
- **Acceptance criteria:** Passing `targ.CollectAllErrors` with `targ.Parallel` runs all targets to completion without cancellation. Failures are collected into a `MultiError`. A detailed summary is printed showing all results.

#### REQ-3-5: Mixed dependency modes
- **Traces to:** UC-3
- **Source:** `internal/core/target.go:497-516` (`runDeps`), `internal/core/target.go:205-218` (`GetDepMode`)
- **Acceptance criteria:** Multiple `.Deps()` calls with different modes create separate groups. Groups execute sequentially (group 1 completes before group 2 starts). `GetDepMode()` returns `DepModeMixed` when groups have different modes.

#### REQ-3-6: Dependency group sequential execution
- **Traces to:** UC-3
- **Source:** `internal/core/target.go:497-516` (`runDeps`)
- **Acceptance criteria:** Dependency groups always execute in declaration order. Each group must complete (success or error) before the next group starts. First group error stops all subsequent groups.

### Design

#### DES-3-1: Dependency pipeline model
- **Traces to:** UC-3
- **Interaction model:** Developer chains `.Deps()` calls to build execution pipelines: `targ.Targ(deploy).Deps(build, test).Deps(lint, vet, targ.Parallel)`. This creates two groups: first runs build→test serially, then lint+vet in parallel. The pipeline is a sequence of groups where each group is either serial or parallel.

---

## UC-4: File caching

### Requirements

#### REQ-4-1: Cache pattern configuration
- **Traces to:** UC-4
- **Source:** `internal/core/target.go:106-117` (`Cache()`)
- **Acceptance criteria:** `.Cache("pattern"...)` sets file glob patterns for cache invalidation. Passing `targ.Disabled` (sentinel `"__targ_disabled__"`) sets `cacheDisabled = true` and clears patterns, allowing CLI `--cache` to control caching.

#### REQ-4-2: Cache directory configuration
- **Traces to:** UC-4
- **Source:** `internal/core/target.go:121-124` (`CacheDir()`)
- **Acceptance criteria:** `.CacheDir(dir)` sets where cache checksum files are stored. Default is `.targ-cache` in the current directory.

#### REQ-4-3: Cache file path generation
- **Traces to:** UC-4
- **Source:** `internal/core/target.go:430-452` (`cacheFilePath`)
- **Acceptance criteria:** Cache file paths are generated by SHA-256 hashing the sorted patterns, taking the first 16 hex chars, and appending `.sum`. This produces deterministic, collision-resistant filenames.

#### REQ-4-4: Cache hit detection
- **Traces to:** UC-4
- **Source:** `internal/core/target.go:456-470` (`checkCache`), `internal/file/checksum.go:32-75` (`Checksum`)
- **Acceptance criteria:** `Checksum` expands patterns via `Match`, computes SHA-256 over file paths and contents, compares against stored hash at dest. Returns `true` (changed) or `false` (unchanged). On change, writes new hash to dest. Missing dest file is treated as changed.

#### REQ-4-5: Cache skip on hit
- **Traces to:** UC-4
- **Source:** `internal/core/target.go:537-547` (cache check in `runOnce`)
- **Acceptance criteria:** If cache patterns are set and `checkCache` returns `false` (unchanged), the target's execution is skipped entirely. Dependencies still run before the cache check.

#### REQ-4-6: Checksum computation
- **Traces to:** UC-4
- **Source:** `internal/file/checksum.go:88-114` (`computeChecksum`)
- **Acceptance criteria:** Checksum includes both file paths and file contents in the SHA-256 hash. Files are processed in the order returned by `Match`. Path strings are null-terminated in the hash to prevent ambiguity. Errors opening or reading files propagate.

#### REQ-4-7: Checksum storage
- **Traces to:** UC-4
- **Source:** `internal/file/checksum.go:125-142` (`writeChecksum`)
- **Acceptance criteria:** The checksum directory is created with mode 0755 if needed. Checksum files are written with mode 0644. The hash is stored as a hex string.

#### REQ-4-8: Input validation
- **Traces to:** UC-4
- **Source:** `internal/file/checksum.go:38-44`
- **Acceptance criteria:** Empty inputs return `ErrNoInputPatterns`. Empty dest returns `ErrEmptyDest`.

### Design

#### DES-4-1: Content-hash cache model
- **Traces to:** UC-4
- **Interaction model:** Developer configures `.Cache("src/**/*.go")` on a target. On each run, targ expands the glob, hashes all matching files, and compares against the stored hash in `.targ-cache/<hash>.sum`. If identical, execution is skipped. This is a content-based (not timestamp-based) cache.

---

## UC-5: File watching

### Requirements

#### REQ-5-1: Watch pattern configuration
- **Traces to:** UC-5
- **Source:** `internal/core/target.go:387-398` (`Watch()`)
- **Acceptance criteria:** `.Watch("pattern"...)` sets file glob patterns for watch mode. Passing `targ.Disabled` sets `watchDisabled = true` and clears patterns, allowing CLI `--watch` to control watching.

#### REQ-5-2: Watch loop in Target.Run
- **Traces to:** UC-5
- **Source:** `internal/core/target.go:337-362` (`Run()`)
- **Acceptance criteria:** When watch patterns are set, `Run()` executes once, then enters a watch loop that re-runs on file changes. The loop continues until context cancellation.

#### REQ-5-3: Polling-based file watching
- **Traces to:** UC-5
- **Source:** `internal/file/watch.go:45-85` (`Watch`)
- **Acceptance criteria:** File watching uses polling with a configurable interval (default 250ms). Each tick takes a snapshot of matching files and their modification times. Empty patterns return `ErrNoPatterns`.

#### REQ-5-4: Change detection via snapshots
- **Traces to:** UC-5
- **Source:** `internal/file/watch.go:106-141` (`diffSnapshot`)
- **Acceptance criteria:** Changes are detected by comparing consecutive snapshots. A `ChangeSet` is produced with `Added` (new files), `Removed` (deleted files), and `Modified` (changed mod-time) lists. All lists are sorted. If no changes, returns nil (no callback invocation).

#### REQ-5-5: Snapshot construction
- **Traces to:** UC-5
- **Source:** `internal/file/watch.go:169-196` (`snapshot`)
- **Acceptance criteria:** Snapshots expand patterns via `matchFn`, then `Stat` each file to record `ModTime().UnixNano()`. Files are sorted. Stat errors propagate.

#### REQ-5-6: Watch cancellation
- **Traces to:** UC-5
- **Source:** `internal/file/watch.go:74-77`
- **Acceptance criteria:** The watch loop checks `ctx.Done()` on each tick. Cancellation returns "watch cancelled:" wrapping `ctx.Err()`.

#### REQ-5-7: Watch via CLI override
- **Traces to:** UC-5
- **Source:** `internal/core/override.go:89-93` (`ExecuteWithOverrides` watch branch), `internal/core/override.go:359-377` (`executeWithWatch`)
- **Acceptance criteria:** When `--watch` CLI flag is set (or target has watch patterns), `ExecuteWithOverrides` wraps the execution in a watch loop. It runs once first, then watches for changes and re-runs.

#### REQ-5-8: Watch conflict detection
- **Traces to:** UC-5
- **Source:** `internal/core/override.go:243-245` (`checkConflicts` watch check)
- **Acceptance criteria:** If both CLI `--watch` and target `.Watch()` are set and `.Watch(targ.Disabled)` was NOT called, an error is returned: "--watch conflicts with target's watch configuration".

#### REQ-5-9: Dependency-injected operations
- **Traces to:** UC-5
- **Source:** `internal/file/watch.go:24-28` (`WatchOps`), `internal/file/watch.go:36-41` (`DefaultWatchOps`)
- **Acceptance criteria:** Watch operations (`NewTicker`, `Stat`) are injectable via `WatchOps` struct. Default uses `time.NewTicker` and `os.Stat`. Nil ops uses defaults. This enables testing without real filesystem or timing.

### Design

#### DES-5-1: Polling watch model
- **Traces to:** UC-5
- **Interaction model:** Developer configures `.Watch("src/**/*.go")` on a target. On first execution, targ takes a snapshot of matching files and mod-times. Every 250ms, it re-snapshots and diffs. Changes trigger re-execution with a `ChangeSet` describing what changed. The loop runs until Ctrl-C or context cancellation.

#### DES-5-2: CLI watch override
- **Traces to:** UC-5
- **Interaction model:** Developer marks a target with `.Watch(targ.Disabled)` to allow CLI control. End user runs `targ build --watch "src/**/*.go"` to enable watching at runtime. Without `targ.Disabled`, mixing CLI and code-level watch patterns is an error to prevent confusion.

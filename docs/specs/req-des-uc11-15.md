# L2: Requirements and Design — UC-11 through UC-15

## UC-11: Runtime Overrides

### Requirements

#### REQ-11-1: CLI flag extraction
- **Traces to:** UC-11
- **Source:** `internal/core/override.go:102-146` (`ExtractOverrides`)
- **Acceptance criteria:** Args are scanned for `--times`, `--retry`, `--backoff`, `--watch`, `--cache`, `--cache-dir`, `--dep-mode`, `--while`, `--deps`, and `--parallel`/`-p`. Recognized flags are consumed; remaining args are returned unchanged. Both `--flag value` and `--flag=value` forms are accepted for value-taking flags.

#### REQ-11-2: Position-sensitive parallel flag
- **Traces to:** UC-11
- **Source:** `internal/core/override.go:124-128` (parallel gate), `internal/core/override.go:660-662` (`isParallelFlag`)
- **Acceptance criteria:** `--parallel` and `-p` are only recognized before the first non-flag argument (target name). After a target name is seen, they pass through as target arguments.

#### REQ-11-3: Conflict detection
- **Traces to:** UC-11
- **Source:** `internal/core/override.go:241-259` (`checkConflicts`)
- **Acceptance criteria:** CLI `--watch` errors if target has `.Watch()` patterns and did not set `targ.Disabled`. CLI `--cache` errors if target has `.Cache()` patterns and did not set `targ.Disabled`. CLI `--deps` errors if target has `.Deps()`. Each returns a distinct sentinel error with remediation guidance.

#### REQ-11-4: Override execution — times, retry, backoff
- **Traces to:** UC-11
- **Source:** `internal/core/override.go:319-356` (`executeOnce`), `internal/core/override.go:268-316` (`executeIteration`)
- **Acceptance criteria:** `--times N` runs the function N times (default 1). `--retry` continues iterating on failure and stops on first success. `--backoff D,M` sleeps between retry iterations with exponential growth. Context cancellation during backoff preserves the original error.

#### REQ-11-5: Override execution — while condition
- **Traces to:** UC-11
- **Source:** `internal/core/override.go:262-266` (`checkWhileCondition`), `internal/core/override.go:277`
- **Acceptance criteria:** `--while "cmd"` runs a shell command via `sh -c` before each iteration. If the command exits non-zero, iteration stops without error.

#### REQ-11-6: Override execution — cache skip
- **Traces to:** UC-11
- **Source:** `internal/core/override.go:218-238` (`checkCacheHit`), `internal/core/override.go:325-332`
- **Acceptance criteria:** When cache patterns are active (from CLI or target config), execution is skipped if file checksums match the stored hash. Default cache directory is `.targ-cache`. `--cache-dir` overrides the directory.

#### REQ-11-7: Override execution — watch loop
- **Traces to:** UC-11
- **Source:** `internal/core/override.go:359-377` (`executeWithWatch`)
- **Acceptance criteria:** When watch patterns are active, the function runs once then re-runs on each file change. Watch runs until context cancellation or error.

#### REQ-11-8: Pattern merge precedence
- **Traces to:** UC-11
- **Source:** `internal/core/override.go:72-81`
- **Acceptance criteria:** CLI override patterns take precedence when present. Target compile-time patterns are used as fallback when CLI patterns are empty (and no conflict exists).

#### REQ-11-9: Centralized flag registry
- **Traces to:** UC-11
- **Source:** `internal/flags/flags.go:34-172` (`All`)
- **Acceptance criteria:** All CLI flags are defined in a single registry with long name, optional short name, description, placeholder, value-taking, root-only, hidden, removed, and mode metadata. Registry serves help, completion, and detection subsystems.

#### REQ-11-10: Flag mode filtering
- **Traces to:** UC-11
- **Source:** `internal/flags/flags.go:7-16` (`FlagMode`), `internal/flags/flags.go:174-293` (filter functions)
- **Acceptance criteria:** Flags are partitioned into `FlagModeAll` (shown in both targ CLI and compiled binary help) and `FlagModeTargOnly` (targ CLI only). Query functions provide boolean flags, value-taking flags, root-only flags, global flags, and visible flags.

#### REQ-11-11: Variadic deps extraction
- **Traces to:** UC-11
- **Source:** `internal/core/override.go:381-427` (`extractDepsVariadic`)
- **Acceptance criteria:** `--deps target1 target2` collects all non-flag args until the next `--` or flag. Errors if `--deps` appears with no targets following.

### Design

#### DES-11-1: Two-phase override processing
- **Traces to:** UC-11
- **Interaction model:** Phase 1: `ExtractOverrides` parses CLI args into `RuntimeOverrides` struct, returns remaining args. Phase 2: `ExecuteWithOverrides` applies overrides during target execution with conflict detection against `TargetConfig`.

#### DES-11-2: Handler chain pattern
- **Traces to:** UC-11
- **Interaction model:** Each override flag has a dedicated handler function (`handleTimesFlag`, `handleWatchFlag`, etc.). `processOverrideFlag` iterates through `overrideFlagHandlers()` until one claims the arg. This keeps individual flag parsing isolated and testable.

#### DES-11-3: Opt-in override via targ.Disabled
- **Traces to:** UC-11
- **Interaction model:** Targets that want CLI overrideability for watch/cache call `.Watch(targ.Disabled)` or `.Cache(targ.Disabled)`. This sets `WatchDisabled`/`CacheDisabled` in `TargetConfig`, suppressing conflict errors. Deps have no disabled mode — they must be defined in exactly one place.

---

## UC-12: Parallel Output

### Requirements

#### REQ-12-1: Prefixed output in parallel mode
- **Traces to:** UC-12
- **Source:** `internal/core/print.go:22-33` (`Print`), `internal/core/print.go:36-47` (`Printf`)
- **Acceptance criteria:** When `ExecInfo.Parallel` is true and a `Printer` is set, output lines are prefixed with `[target-name]` followed by padding to align columns. When not in parallel mode, output goes directly to the context's writer or `os.Stdout`.

#### REQ-12-2: Prefix formatting with alignment
- **Traces to:** UC-12
- **Source:** `internal/core/print.go:12-19` (`FormatPrefix`)
- **Acceptance criteria:** Prefix format is `[name] ` with space padding to `maxLen` (the longest target name in the parallel group). This right-aligns the output column across all parallel targets.

#### REQ-12-3: Line-atomic serialized printing
- **Traces to:** UC-12
- **Source:** `internal/core/printer.go:8-12` (`Printer` struct), `internal/core/printer.go:16-26` (`NewPrinter`), `internal/core/printer.go:39-44` (`run`)
- **Acceptance criteria:** A `Printer` goroutine reads from a buffered channel and writes lines sequentially to the output writer. This guarantees no interleaving between parallel targets. `Close()` drains remaining lines and waits for the goroutine to exit.

#### REQ-12-4: Partial line buffering for shell output
- **Traces to:** UC-12
- **Source:** `internal/core/prefix_writer.go:8-12` (`PrefixWriter`), `internal/core/prefix_writer.go:31-45` (`Write`), `internal/core/prefix_writer.go:23-28` (`Flush`)
- **Acceptance criteria:** `PrefixWriter` implements `io.Writer` for use as stdout/stderr of shell commands. It buffers partial lines in a `strings.Builder`, emitting complete prefixed lines to the `Printer` as newlines arrive. `Flush()` sends any remaining partial line with a trailing newline.

#### REQ-12-5: Execution context propagation
- **Traces to:** UC-12
- **Source:** `internal/core/exec_info.go:10-16` (`ExecInfo`), `internal/core/exec_info.go:20-23` (`GetExecInfo`), `internal/core/exec_info.go:26-28` (`WithExecInfo`)
- **Acceptance criteria:** `ExecInfo` is stored in context via `WithExecInfo` and retrieved via `GetExecInfo`. It carries: `Parallel` flag, target `Name`, `MaxNameLen` for alignment, `Printer` reference, and optional `Output` writer. Serial execution returns `ok=false` from `GetExecInfo`.

### Design

#### DES-12-1: Channel-based serialization
- **Traces to:** UC-12
- **Interaction model:** Parallel targets send prefixed lines to a shared `Printer` via its buffered channel. The `Printer` goroutine is the single writer to the output stream, eliminating races. Each parallel group creates one `Printer`; all targets in the group share it via context.

#### DES-12-2: Two output paths
- **Traces to:** UC-12
- **Interaction model:** `Print`/`Printf` check `ExecInfo` from context. In parallel mode: lines are split, prefixed, and sent through the `Printer`. In serial mode (or no `ExecInfo`): output goes directly to the context writer or `os.Stdout`. This makes the API transparent — target code calls `targ.Print(ctx, ...)` regardless of execution mode.

#### DES-12-3: Shell command integration via PrefixWriter
- **Traces to:** UC-12
- **Interaction model:** Shell commands in parallel mode get a `PrefixWriter` as stdout/stderr. The `PrefixWriter` buffers byte-level writes into complete lines, then sends each line through the `Printer` with the target's prefix. This handles the mismatch between `io.Writer` (arbitrary byte chunks) and the line-oriented `Printer`.

---

## UC-13: Shell Command Execution

### Requirements

#### REQ-13-1: Streaming command execution
- **Traces to:** UC-13
- **Source:** `internal/sh/sh.go:142-167` (`Run`)
- **Acceptance criteria:** `Run` executes a command with stdout/stderr/stdin connected to the shell environment's IO. The process is placed in its own process group (`SetProcGroup`). The process is registered with `CleanupManager` during execution and unregistered after.

#### REQ-13-2: Verbose command execution
- **Traces to:** UC-13
- **Source:** `internal/sh/sh.go:170-178` (`RunV`)
- **Acceptance criteria:** `RunV` prints `+ command args` to stdout before executing via `Run`. Arguments are quoted using `QuoteArg` (empty strings become `""`, strings with spaces/tabs/newlines/quotes are `strconv.Quote`d).

#### REQ-13-3: Captured output execution
- **Traces to:** UC-13
- **Source:** `internal/sh/sh.go:98-126` (`Output`)
- **Acceptance criteria:** `Output` executes a command and returns combined stdout+stderr as a string. Uses `SafeBuffer` (mutex-protected `bytes.Buffer`) for thread-safe concurrent writes. Process is registered/unregistered with `CleanupManager`.

#### REQ-13-4: Context-aware execution
- **Traces to:** UC-13
- **Source:** `internal/sh/context.go:12-49` (`OutputContext`), `internal/sh/context.go:63-74` (`RunContextWithIO`)
- **Acceptance criteria:** Context variants (`OutputContext`, `RunContextWithIO`, `RunContextV`) support cancellation. On context cancellation, the entire process group is killed (`KillProcessGroup`) and the function waits for the process to exit before returning.

#### REQ-13-5: Process group isolation (Unix)
- **Traces to:** UC-13
- **Source:** `internal/sh/context_unix.go:21-23` (`SetProcGroup`), `internal/sh/context_unix.go:13-18` (`KillProcessGroup`)
- **Acceptance criteria:** On Unix, `SetProcGroup` sets `Setpgid: true` so child processes run in their own process group. `KillProcessGroup` sends `SIGKILL` to the negative PID, killing the entire group.

#### REQ-13-6: Windows process termination
- **Traces to:** UC-13
- **Source:** `internal/sh/context_windows.go:13-17` (`KillProcessGroup`), `internal/sh/context_windows.go:20-23` (`SetProcGroup`)
- **Acceptance criteria:** On Windows, `SetProcGroup` is a no-op. `KillProcessGroup` calls `Process.Kill()`. Child processes may not be terminated (Job Objects not implemented).

#### REQ-13-7: Shell environment DI
- **Traces to:** UC-13
- **Source:** `internal/sh/sh.go:41-48` (`ShellEnv`), `internal/sh/sh.go:51-60` (`DefaultShellEnv`)
- **Acceptance criteria:** `ShellEnv` struct provides dependency injection for `exec.Command`, `IsWindows`, stdin/stdout/stderr writers, and `CleanupManager`. `DefaultShellEnv` returns OS defaults. All execution functions accept `nil` env and fall back to defaults.

#### REQ-13-8: Exe suffix handling
- **Traces to:** UC-13
- **Source:** `internal/sh/sh.go:68-78` (`ExeSuffix`), `internal/sh/sh.go:181-195` (`WithExeSuffix`)
- **Acceptance criteria:** `ExeSuffix` returns `.exe` on Windows, empty string otherwise. `WithExeSuffix` appends `.exe` to a name on Windows if not already present.

### Design

#### DES-13-1: Uniform execution pattern
- **Traces to:** UC-13
- **Interaction model:** All execution functions follow the pattern: create command, set IO, call `SetProcGroup`, `Start`, register with `CleanupManager`, `Wait`, unregister. Context variants add a select on `ctx.Done()` with process group kill on cancellation. This ensures consistent cleanup regardless of execution mode.

#### DES-13-2: Platform abstraction via build tags
- **Traces to:** UC-13
- **Interaction model:** `context_unix.go` and `context_windows.go` provide platform-specific `SetProcGroup`, `KillProcessGroup`, and `runWithContext` behind `//go:build` tags. The rest of the shell package calls these functions without platform awareness.

---

## UC-14: File Utilities

### Requirements

#### REQ-14-1: Glob pattern matching with brace expansion
- **Traces to:** UC-14
- **Source:** `internal/file/match.go:22-68` (`Match`)
- **Acceptance criteria:** `Match` accepts one or more glob patterns, expands `{a,b}` brace syntax, then matches via `doublestar.Glob` (supporting `**`). Results are deduplicated and sorted. Returns `ErrNoPatterns` if no patterns provided. Handles both relative and absolute paths.

#### REQ-14-2: Nested brace expansion
- **Traces to:** UC-14
- **Source:** `internal/file/match.go:70-106` (`expandBraces`)
- **Acceptance criteria:** Brace expansion handles nested braces (e.g., `{a,{b,c}}`) recursively. Unmatched braces return `ErrUnmatchedBrace`. Patterns without braces pass through unchanged.

#### REQ-14-3: Content-based file checksumming
- **Traces to:** UC-14
- **Source:** `internal/file/checksum.go:32-75` (`Checksum`)
- **Acceptance criteria:** `Checksum` computes SHA-256 over matched file paths and their contents. Compares against stored hash at `dest`. Returns `true` (changed) if hashes differ and writes the new hash. Returns `false` (unchanged) if hashes match. First run always returns `true`. Returns `ErrNoInputPatterns` or `ErrEmptyDest` for invalid args.

#### REQ-14-4: Checksum file management
- **Traces to:** UC-14
- **Source:** `internal/file/checksum.go:116-142` (`readChecksum`, `writeChecksum`)
- **Acceptance criteria:** Checksum files are plain text containing the hex-encoded SHA-256. Parent directories are created with 0755 permissions. Files are written with 0644 permissions. Missing checksum file (first run) is treated as "changed" without error.

#### REQ-14-5: File operations DI
- **Traces to:** UC-14
- **Source:** `internal/file/checksum.go:22-27` (`FileOps`), `internal/file/checksum.go:78-86` (`DefaultFileOps`)
- **Acceptance criteria:** `FileOps` struct provides injectable `MkdirAll`, `OpenFile`, `ReadFile`, `WriteFile` operations. `Checksum` accepts `nil` ops and falls back to `DefaultFileOps` (standard `os` functions).

### Design

#### DES-14-1: Match as foundation
- **Traces to:** UC-14
- **Interaction model:** `Match` is the shared glob engine used by both caching (`Checksum`) and watching subsystems. It is passed as a `matchFn` callback, keeping `Checksum` decoupled from the glob implementation. Users call `targ.Match(patterns...)` directly for custom file operations.

#### DES-14-2: Hash-then-compare caching
- **Traces to:** UC-14
- **Interaction model:** `Checksum` computes a full content hash each time (path names + file contents concatenated with null separators into SHA-256). It compares against the previously stored hash. On change, the new hash is persisted atomically. This approach is simple and correct — no file-level mtime tracking or incremental updates.

---

## UC-15: Process Cleanup

### Requirements

#### REQ-15-1: Signal-triggered cleanup
- **Traces to:** UC-15
- **Source:** `internal/sh/cleanup.go:29-50` (`EnableCleanup`)
- **Acceptance criteria:** `EnableCleanup` installs a signal handler for `SIGINT` and `SIGTERM`. On signal receipt, all registered processes are killed and the program exits with code 130. Signal handler is installed at most once (idempotent). Cleanup is off by default — must be explicitly enabled.

#### REQ-15-2: Process registration lifecycle
- **Traces to:** UC-15
- **Source:** `internal/sh/cleanup.go:69-76` (`RegisterProcess`), `internal/sh/cleanup.go:79-84` (`UnregisterProcess`)
- **Acceptance criteria:** Processes are registered after `Start()` and unregistered after `Wait()`. Registration is only tracked when cleanup is enabled. The registry is a `map[*os.Process]struct{}` protected by a mutex for concurrent access.

#### REQ-15-3: Kill all tracked processes
- **Traces to:** UC-15
- **Source:** `internal/sh/cleanup.go:53-66` (`KillAllProcesses`)
- **Acceptance criteria:** `KillAllProcesses` snapshots the process map under lock, then kills each process without holding the lock (avoiding deadlock with unregister). Uses the injected `killFunc` for platform-specific killing.

#### REQ-15-4: Platform-specific kill — Unix
- **Traces to:** UC-15
- **Source:** `internal/sh/cleanup_unix.go:12-15` (`PlatformKillProcess`)
- **Acceptance criteria:** On Unix, kills the entire process group via `syscall.Kill(-p.Pid, SIGKILL)`. This ensures child processes spawned by the target are also terminated.

#### REQ-15-5: Platform-specific kill — Windows
- **Traces to:** UC-15
- **Source:** `internal/sh/cleanup_windows.go:9-11` (`PlatformKillProcess`)
- **Acceptance criteria:** On Windows, calls `p.Kill()`. Child processes may survive (no Job Object support).

#### REQ-15-6: CleanupManager DI
- **Traces to:** UC-15
- **Source:** `internal/sh/cleanup.go:21-26` (`NewCleanupManager`), `internal/sh/sh.go:200-201` (`defaultCleanup`)
- **Acceptance criteria:** `CleanupManager` accepts an injected `killFunc` for testing. A package-level `defaultCleanup` singleton is created with `PlatformKillProcess`. `ShellEnv` carries a `Cleanup` reference, making the cleanup manager available throughout the shell execution chain.

### Design

#### DES-15-1: Opt-in cleanup model
- **Traces to:** UC-15
- **Interaction model:** User calls `targ.EnableCleanup()` at the top of their targ file. This activates signal handling and process tracking. Without this call, processes are not tracked and no signal handler is installed. This avoids interfering with programs that manage their own signal handling.

#### DES-15-2: Register/unregister bracketing
- **Traces to:** UC-15
- **Interaction model:** Every `Run`/`Output` call brackets the process lifetime: `Start` -> `RegisterProcess` -> `Wait` -> `UnregisterProcess`. If a signal arrives between register and unregister, the process is in the kill list. After unregister, the process is no longer tracked. This ensures cleanup only targets live processes.

#### DES-15-3: Platform kill abstraction
- **Traces to:** UC-15
- **Interaction model:** `PlatformKillProcess` is the only platform-varying function in cleanup. Unix uses process-group kill (negative PID + SIGKILL) for full tree cleanup. Windows uses `Process.Kill()` for the direct process only. The `CleanupManager` delegates to this function via the injected `killFunc`, enabling test doubles.

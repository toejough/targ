# Issues & Roadmap

## Active Issues

### Issue #1: Explore CLI Tool Design
- **Status**: Done
- **Description**: Initial exploration and prototyping of the library.

## Backlog: Mage Feature Parity & Enhancements

The following issues are recommendations to bring `commander` closer to `mage` in terms of utility for build tasks, and to address current architectural limitations.

### Issue #2: Shell Execution Helpers (`commander/sh`)
- **Type**: Feature (Mage Parity)
- **Priority**: High
- **Description**: Currently, users must use `os/exec` manually, which is verbose. Mage provides a `sh` package for one-line command execution.
- **Proposed Features**:
  - `sh.Run(cmd, args...)`: Stream stdout/stderr.
  - `sh.Output(cmd, args...)`: Capture stdout.
  - `sh.RunV(cmd, args...)`: Run verbose (always print) vs quiet (print only on error).
  - Cross-platform `rm`, `copy` helpers.

### Issue #3: Dependency Management (`Once`)
- **Type**: Feature (Mage Parity)
- **Priority**: Medium
- **Description**: Build targets often depend on other targets (e.g., `Build` depends on `Generate`). Simply calling the method `g.Run()` works, but if multiple targets depend on it, it runs multiple times.
- **Proposed Feature**: A mechanism like `commander.Deps(TargetFunc)` that ensures a target runs exactly once per execution graph.

### Issue #4: File Modification Checks (`target`)
- **Type**: Feature (Mage Parity)
- **Priority**: Medium
- **Description**: Build tools often need to skip steps if the output is newer than the input.
- **Proposed Feature**: Helper functions to compare file modification times (e.g., `commander.Newer(src, dst)`).

### Issue #5: Error Return Support
- **Type**: Enhancement
- **Priority**: High
- **Description**: Currently `Run` methods must be `func()`. They should optionally return `error` so the library can handle exit codes and logging consistently.
- **Proposed Change**: Support `func() error` signature in reflection parsing.

### Issue #6: Context Support & Timeout
- **Type**: Enhancement
- **Priority**: Medium
- **Description**: Long-running build tasks often need cancellation or timeouts.
- **Proposed Change**: Support `func(context.Context)` signature. `commander.Run` should set up a signal-canceling root context.

### Issue #7: Compilation-Safe Documentation
- **Type**: Bug/Enhancement
- **Priority**: High
- **Description**: The current Help system parses Go source files at runtime to extract doc strings. This works in "Build Tool Mode" (go run), but fails if the application is compiled into a binary and moved away from the source code.
- **Proposed Fix**:
  - **Option A**: For standalone binaries, require `desc` tags (fallback).
  - **Option B**: In Build Tool Mode (bootstrap), generate code that embeds the comments into the compiled struct, so the binary is self-contained.

### Issue #8: Persistent Flags & Lifecycle Hooks
- **Type**: Enhancement (Cobra Parity)
- **Priority**: Low
- **Description**: There is no way to define a flag like `--verbose` at the root that applies to all subcommands, or code that runs *before* a subcommand (setup/teardown).
- **Proposed Feature**:
  - Support `PersistentBefore() error` and `PersistentAfter() error` methods on parent structs.
  - Mechanism to propagate flags down the tree.

### Issue #9: Namespace/Category Organization
- **Type**: Enhancement
- **Priority**: Low
- **Description**: As build files grow, flat command lists get messy. Mage supports `// mg:ns` namespaces. Commander handles this via Struct nesting, but we might want a way to "flatten" or "group" commands visually in the help output.

## Backlog: go-arg Feature Parity

### Issue #10: Custom Type Support (TextUnmarshaler)
- **Type**: Feature (go-arg Parity)
- **Priority**: High
- **Description**: Currently `commander` only supports basic types (string, int, bool). `go-arg` supports any type that implements `encoding.TextUnmarshaler` (e.g., `time.Duration`, `net.IP`, custom types).
- **Proposed Feature**: Add interface check in flag parsing loop to support `Set(string) error` or `UnmarshalText([]byte) error`.

### Issue #11: Default Value Tags
- **Type**: Feature (go-arg Parity)
- **Priority**: Low
- **Description**: Support `default="value"` in tags. Currently defaults must be set by initializing the struct before passing it to Run, which works for Root commands but is harder for auto-instantiated subcommands.

### Issue #12: Placeholder Customization
- **Type**: Enhancement
- **Priority**: Low
- **Description**: Support `placeholder="FILE"` tag to change help text from `-output string` to `-output FILE`.

## Backlog: Advanced Features

### Issue #13: Watch Mode
- **Type**: Feature (Enhancement)
- **Priority**: Medium
- **Description**: Helper to watch for file changes and re-run a command. Should support cancelling the previous run if it's still in progress.
- **Proposed Feature**:
  - `commander.Watch(patterns []string, runFunc func() error)`
  - Use globbing (fish-like `**/*.go`) for file patterns.
  - Handle process cancellation (context cancellation or killing process).

### Issue #14: Parallel Execution
- **Type**: Feature (Performance)
- **Priority**: Medium
- **Description**: When running dependencies (Issue #3) or lists of tasks, support parallel execution to speed up builds.
- **Proposed Feature**: `commander.Parallel(funcs...)` or `commander.Deps(Parallel(Build, Lint))`.

### Issue #15: .env File Loading
- **Type**: Feature (DX)
- **Priority**: Medium
- **Description**: Automatically load environment variables from a `.env` file if present, populating flags that use `env=...` tags.
- **Proposed Feature**: `commander.LoadEnv()` or auto-load in `Run()`.

### Issue #16: Interactive UI Helpers
- **Type**: Feature (DX)
- **Priority**: Low
- **Description**: Helpers for common CLI interactions.
- **Proposed Feature**: `ui` package with `Confirm(msg)`, `Select(msg, options)`, `Prompt(msg)`.

### Issue #17: Checksum-based Caching
- **Type**: Feature (Build Efficiency)
- **Priority**: Low
- **Description**: Skip tasks if input files haven't changed content (more robust than timestamp checks).
- **Proposed Feature**: `target.Checksum(srcs, dest)` to complement timestamp checks.

## Audit Findings

### Issue #18: Positional Args Are Also Registered As Flags
- **Type**: Bug
- **Priority**: High
- **Description**: Fields tagged `commander:"positional"` are still registered as flags, so they can be set via `-field` and conflict with positional parsing.
- **Impact**: Positional semantics are inconsistent and can mask argument errors.
- **Proposed Fix**: Skip flag registration for `positional` fields during parsing and help output.

### Issue #19: Struct Default Values Are Overwritten By Flag Defaults
- **Type**: Bug/Enhancement
- **Priority**: Medium
- **Description**: Field defaults set on the struct instance are overwritten by flag registration, which uses zero or env defaults only.
- **Impact**: Preconfigured defaults are lost at runtime.
- **Proposed Fix**: Initialize defaults from the struct instance when registering flags, and only override with env if present.

### Issue #20: `required` Tags Are Not Enforced
- **Type**: Bug
- **Priority**: High
- **Description**: `commander:"required"` is parsed but never validated.
- **Impact**: Missing required args pass silently.
- **Proposed Fix**: Track `flag.Visit` and validate required flags/positionals after parsing.

### Issue #21: Nil Pointer Inputs Can Panic
- **Type**: Bug
- **Priority**: High
- **Description**: Passing nil pointers to `Run` (or nil subcommand pointers) causes `v.Elem()` panics.
- **Impact**: Crashes on common misconfiguration.
- **Proposed Fix**: Validate pointers before `Elem()` and return a descriptive error.

### Issue #22: Subcommand Assignment Fails For Non-Pointer Fields
- **Type**: Bug
- **Priority**: Medium
- **Description**: Subcommand assignment always uses a pointer, which panics if the struct field is not a pointer type.
- **Impact**: Non-pointer subcommand fields fail at runtime.
- **Proposed Fix**: Support both pointer and value subcommand fields during assignment.

### Issue #23: Unexported Tagged Fields Can Panic
- **Type**: Bug
- **Priority**: Medium
- **Description**: Unexported fields tagged for flags/positionals can panic when set via reflection.
- **Impact**: Runtime panics with minimal guidance.
- **Proposed Fix**: Validate field export status and emit a friendly error.

### Issue #24: Build Tool Mode Includes Non-Commands
- **Type**: Bug/Enhancement
- **Priority**: Low
- **Description**: Build tool mode includes every exported struct, even those without `Run` or subcommands.
- **Impact**: Non-commands show up in help and can error on invocation.
- **Proposed Fix**: Filter to structs with `Run` or subcommands in `cmd/commander`.

### Issue #25: Completion Tokenization Ignores Quotes/Escapes
- **Type**: Bug/Enhancement
- **Priority**: Low
- **Description**: Completion uses `strings.Fields`, so quoted/escaped args are split incorrectly.
- **Impact**: Completion breaks for args with spaces.
- **Proposed Fix**: Use a shell-aware tokenizer for completion input.

### Issue #26: Invalid Env Defaults Are Silently Ignored
- **Type**: Bug/Enhancement
- **Priority**: Low
- **Description**: Invalid env values for int/bool fall back to zero/false without warning.
- **Impact**: Misconfigurations are hard to spot.
- **Proposed Fix**: Validate env parsing and surface errors or warnings.

## Backlog: Build Tool Mode Parity (Mage Lessons)

### Issue #27: Build Tag Filtering For Build Tool Mode
- **Type**: Feature (Build Tool Mode)
- **Priority**: Medium
- **Description**: Restrict command discovery to Go files with a specific build tag (Mage-style), to avoid accidental inclusion.
- **Proposed Feature**: Support a build tag (e.g. `//go:build commander`) for discovery in build tool mode.

### Issue #28: Build Tool Mode Compiled Binary Cache
- **Type**: Feature (Build Tool Mode Performance)
- **Priority**: Medium
- **Description**: Cache a compiled binary to avoid `go run` on every invocation.
- **Proposed Feature**: Generate a deterministic cache key (source hash + args) and reuse the compiled executable when valid.

### Issue #29: Temporary Generated Main File Handling
- **Type**: Feature (Build Tool Mode)
- **Priority**: Low
- **Description**: Improve handling of generated bootstrap file (naming, location, cleanup).
- **Proposed Feature**: Generate into a temp dir with a stable name, keep with a `--keep` flag, and ensure robust cleanup.

### Issue #30: Function Targets Support (Direct + Build Tool Modes)
- **Type**: Feature
- **Priority**: High
- **Description**: Support niladic functions as commands alongside struct-based commands.
- **Proposed Feature**: Allow `Run` to accept functions; build tool mode discovers exported niladic functions.

### Issue #31: Build Tool Mode Build-Tag Discovery
- **Type**: Feature (Build Tool Mode)
- **Priority**: High
- **Description**: Discover commands only in directories containing files with `//go:build commander`.
- **Proposed Feature**: Recursive search for tagged files; enforce a single package per directory per Go rules.

### Issue #32: Build Tool Mode Depth Gating
- **Type**: Feature (Build Tool Mode)
- **Priority**: High
- **Description**: Without `--package`, stop at the first depth where tagged files are found and error if multiple directories exist at that depth.
- **Proposed Feature**: Track depth in recursive search; emit a clear error listing the conflicting directories.

### Issue #33: Build Tool Mode Package Grouping
- **Type**: Feature (Build Tool Mode)
- **Priority**: High
- **Description**: When `--package` is set, always add the package name as the first subcommand (even for a single directory).
- **Proposed Feature**: Wrap discovered commands under a package node; require no default command in build tool mode.

### Issue #34: Build Tool Mode Subcommand Filtering For Functions
- **Type**: Feature (Build Tool Mode)
- **Priority**: Medium
- **Description**: Filter out exported functions that are named as subcommands of exported structs.
- **Proposed Feature**: Treat struct field subcommand names as occupied and exclude matching functions.

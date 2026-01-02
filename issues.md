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
- **Description**: The current Help system parses Go source files at runtime to extract doc strings. This works in "CLI Mode" (go run), but fails if the application is compiled into a binary and moved away from the source code.
- **Proposed Fix**:
  - **Option A**: For standalone binaries, require `desc` tags (fallback).
  - **Option B**: In CLI Mode (bootstrap), generate code that embeds the comments into the compiled struct, so the binary is self-contained.

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

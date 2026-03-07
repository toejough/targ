# Use Cases: targ

## UC-1: Define and run targets

- **Actor:** Developer building a CLI or build tool
- **Starting state:** Developer has Go functions or shell command strings they want to expose as CLI commands.
- **End state:** Targets are defined and executable via `targ <name>` or as a standalone binary.
- **Key interactions:** `targ.Targ(fn)`, `targ.Targ("cmd")`, `targ.Targ().Name("all").Deps(...)`. Execute via `targ.Main()` or `targ.Register()`.
- **Source files:** `targ.go` (public API), `internal/core/target.go` (Target type), `internal/core/execute.go` (execution), `internal/core/run_env.go` (environment)
- **Test files:** `test/execution_properties_test.go`, `test/execution_fuzz_test.go`, `internal/core/execute_test.go`, `internal/core/target_test.go`
- **Status:** Complete

## UC-2: CLI argument parsing

- **Actor:** Developer defining target functions with parameters
- **Starting state:** Developer wants typed CLI flags and positional arguments.
- **End state:** Struct fields parsed from CLI args with validation, defaults, help text.
- **Key interactions:** Struct with `targ:"..."` tags (required, positional, flag, name, short, desc, enum, default, env).
- **Source files:** `internal/core/command.go` (parsing logic), `internal/core/parse.go` (stub), `internal/parse/parse.go` (tag parsing)
- **Test files:** `test/arguments_properties_test.go`, `test/arguments_fuzz_test.go`, `internal/core/command_internal_test.go`, `internal/parse/parse_properties_test.go`
- **Status:** Complete

## UC-3: Dependency management

- **Actor:** Developer composing build pipelines
- **Starting state:** Targets with ordering and parallelism requirements.
- **End state:** Dependencies execute before the target with serial, parallel, or mixed modes.
- **Key interactions:** `.Deps(a, b)`, `.Deps(a, b, targ.Parallel)`, `CollectAllErrors`.
- **Source files:** `internal/core/target.go` (depGroup, DepMode), `internal/core/command.go` (execution)
- **Test files:** `test/execution_properties_test.go` (dependency tests), `internal/core/target_test.go` (DepGroupChaining)
- **Status:** Complete

## UC-4: File caching

- **Actor:** Developer wanting to skip redundant work
- **Starting state:** Target with source file patterns.
- **End state:** Execution skipped if matched files haven't changed.
- **Key interactions:** `.Cache("src/**/*.go")`, `.CacheDir(dir)`, `targ.Disabled`.
- **Source files:** `internal/file/checksum.go`, `internal/core/command.go` (cache check logic), `internal/core/override.go` (CLI override)
- **Test files:** `test/overrides_properties_test.go` (cache override tests)
- **Status:** Complete

## UC-5: File watching

- **Actor:** Developer wanting continuous re-execution
- **Starting state:** Target with file watch patterns.
- **End state:** Re-executes on file changes with polling.
- **Key interactions:** `.Watch("src/**/*.go")`, `--watch`, `targ.Watch()`.
- **Source files:** `internal/file/watch.go`, `internal/core/command.go` (watch loop), `internal/core/override.go`
- **Test files:** `test/overrides_properties_test.go` (watch override tests)
- **Status:** Complete

## UC-6: Help and shell completion

- **Actor:** Developer or end user discovering commands
- **Starting state:** User wants available commands, flags, usage.
- **End state:** Formatted help with groups, flags, examples. Shell completion for bash/zsh/fish.
- **Key interactions:** `targ --help`, `targ <cmd> --help`, `targ --completion [bash|zsh|fish]`.
- **Source files:** `internal/help/builder.go`, `internal/help/render.go`, `internal/help/content.go`, `internal/help/generators.go`, `internal/help/styles.go`, `internal/core/completion.go`
- **Test files:** `internal/help/builder_test.go`, `internal/help/render_test.go`, `internal/help/content_test.go`, `internal/help/generators_test.go`, `internal/help/binary_mode_test.go`, `internal/help/render_helpers_test.go`, `test/completion_properties_test.go`, `internal/core/binary_mode_propagation_test.go`
- **Status:** Complete

## UC-7: Target grouping

- **Actor:** Developer organizing commands into hierarchies
- **Starting state:** Many targets needing logical structure.
- **End state:** Named groups with nested hierarchies (e.g., `targ dev lint fast`).
- **Key interactions:** `targ.Group("dev", build, lint)`. Nested groups.
- **Source files:** `internal/core/group.go`, `internal/core/command.go` (subcommand resolution)
- **Test files:** `test/hierarchy_properties_test.go`, `test/hierarchy_fuzz_test.go`
- **Status:** Complete

## UC-8: Remote target sync

- **Actor:** Developer importing shared targets from external modules
- **Starting state:** Wants targets from another Go package.
- **End state:** External targets available locally with version management.
- **Key interactions:** `targ --sync github.com/org/targets`, `targ.DeregisterFrom()`.
- **Source files:** `internal/runner/runner.go` (sync logic), `internal/core/execute.go` (deregistration), `internal/core/registry.go` (conflict detection)
- **Test files:** `internal/runner/runner_properties_test.go` (sync tests), `internal/core/registry_test.go`, `internal/core/execute_test.go` (deregistration)
- **Status:** Complete

## UC-9: Target scaffolding

- **Actor:** Developer creating new targets quickly
- **Starting state:** Wants a new target without boilerplate.
- **End state:** Target function or shell command scaffolded in targ file.
- **Key interactions:** `targ --create NAME [CMD]`, `targ --to-func`, `targ --to-string`.
- **Source files:** `internal/runner/runner.go` (create/convert logic), `internal/runner/create_codegen.go`
- **Test files:** `internal/runner/runner_properties_test.go` (code generation tests), `internal/runner/create_codegen_test.go`, `internal/runner/runner_help_test.go`
- **Status:** Complete

## UC-10: Build tool discovery and compilation

- **Actor:** Developer running `targ` from a project directory
- **Starting state:** Project has Go files with `//go:build targ` tags.
- **End state:** Targ discovers files, generates bootstrap, compiles, caches, executes.
- **Key interactions:** Automatic — `targ` discovers, builds, runs. Handles single-module, multi-module, isolated cases.
- **Source files:** `internal/discover/discover.go`, `internal/runner/runner.go` (groupByModule, handleSingleModule, handleMultiModule, handleIsolatedModule, bootstrap generation)
- **Test files:** `internal/discover/discover_properties_test.go`, `internal/runner/runner_properties_test.go`
- **Status:** Complete

## UC-11: Runtime overrides

- **Actor:** Developer temporarily changing target behavior from CLI
- **Starting state:** Target with configuration, developer wants one-time override.
- **End state:** CLI flags override target config with conflict detection.
- **Key interactions:** `--timeout`, `--parallel`, `--times`, `--retry`, `--backoff`, `--watch`, `--cache`, `--dep-mode`, `--while`. `targ.Disabled` for opt-in.
- **Source files:** `internal/core/override.go`, `internal/flags/flags.go`
- **Test files:** `test/overrides_properties_test.go`, `internal/flags/flags_test.go`, `internal/flags/coverage_test.go`
- **Status:** Complete

## UC-12: Parallel output

- **Actor:** Developer running targets concurrently
- **Starting state:** Multiple targets in parallel producing output.
- **End state:** Output prefixed with `[target-name]`, serialized to prevent interleaving.
- **Key interactions:** Automatic in parallel mode. `targ.Print(ctx, ...)`, `targ.Printf(ctx, ...)`.
- **Source files:** `internal/core/print.go`, `internal/core/printer.go`, `internal/core/prefix_writer.go`, `internal/core/exec_info.go`
- **Test files:** `internal/core/parallel_output_test.go`, `internal/core/print_test.go`, `internal/core/printer_test.go`, `internal/core/prefix_writer_test.go`, `internal/core/exec_info_test.go`
- **Status:** Complete

## UC-13: Shell command execution

- **Actor:** Developer running external commands from within targets
- **Starting state:** Target function needs to invoke shell commands.
- **End state:** Commands execute with streaming or captured output.
- **Key interactions:** `targ.Run()`, `targ.RunV()`, `targ.RunContext()`, `targ.Output()`, `targ.OutputContext()`.
- **Source files:** `internal/sh/sh.go`, `internal/sh/context.go`, `internal/sh/cleanup.go`, `internal/sh/context_unix.go`, `internal/sh/context_windows.go`, `internal/sh/cleanup_unix.go`, `internal/sh/cleanup_windows.go`
- **Test files:** `test/shell_properties_test.go`
- **Status:** Complete

## UC-14: File utilities

- **Actor:** Developer working with files in target functions
- **Starting state:** Target needs glob matching, modification time comparison, or content hashing.
- **End state:** File operations complete.
- **Key interactions:** `targ.Match(patterns...)`, `targ.Newer(inputs, outputs)`, `targ.Checksum(inputs, dest)`.
- **Source files:** `internal/file/match.go`, `internal/file/checksum.go`
- **Test files:** (tested indirectly through cache/watch tests; match.go uses doublestar library)
- **Status:** Complete

## UC-15: Process cleanup

- **Actor:** Developer running long-lived or forked processes from targets
- **Starting state:** Target spawns child processes needing cleanup.
- **End state:** Child processes terminated on SIGINT/SIGTERM.
- **Key interactions:** `targ.EnableCleanup()`. Platform-specific.
- **Source files:** `internal/sh/cleanup.go`, `internal/sh/cleanup_unix.go`, `internal/sh/cleanup_windows.go`
- **Test files:** (platform-specific signal handling; tested via execution tests)
- **Status:** Complete

## UC-16: Directory tree traversal (NEW — Issue #11)

- **Actor:** Developer at a terminal
- **Starting state:** Targ target files at multiple levels of the directory tree.
- **End state:** All targets across the tree discoverable and executable from any subdirectory.
- **Key interactions:** Run `targ <name>` from any directory. Discovers down (subtree) and up (linear ancestors + each ancestor's `dev/` subtree).
- **Constraints:**
  - Linear ancestor path only (no sibling dirs), plus `dev/` subtree at each ancestor.
  - Walks to filesystem root.
  - Automatic, no opt-in.
  - Ancestor targets without go.mod compile via isolated build.
  - Conflict handling unchanged.
- **Source files:** `internal/discover/discover.go` (to modify), `internal/runner/runner.go` (to modify)
- **Test files:** (to be written)
- **Status:** Pending

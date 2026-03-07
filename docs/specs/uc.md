# Use Cases: targ

## UC-1: Define and run targets

- **Actor:** Developer building a CLI or build tool
- **Starting state:** Developer has Go functions or shell command strings they want to expose as CLI commands.
- **End state:** Targets are defined and executable via `targ <name>` or as a standalone binary.
- **Key interactions:** Use `targ.Targ(fn)` for function targets, `targ.Targ("cmd")` for shell commands, `targ.Targ().Name("all").Deps(...)` for deps-only. Execute via `targ.Main()` (standalone) or `targ.Register()` + build tag discovery.
- **Status:** Complete (existing feature)

## UC-2: CLI argument parsing

- **Actor:** Developer defining target functions with parameters
- **Starting state:** Developer wants typed CLI flags and positional arguments for their targets.
- **End state:** Struct fields are automatically parsed from CLI args with validation, defaults, and help text.
- **Key interactions:** Define a struct with `targ:"..."` tags (required, positional, flag, name, short, desc, enum, default, env). Supported types: scalars, slices, maps, embedded structs, `Interleaved[T]`.
- **Status:** Complete (existing feature)

## UC-3: Dependency management

- **Actor:** Developer composing build pipelines
- **Starting state:** Developer has targets that must run in a specific order, some parallelizable.
- **End state:** Dependencies execute before the target, with serial, parallel, or mixed modes. Each dep runs exactly once.
- **Key interactions:** `.Deps(a, b)` for serial, `.Deps(a, b, targ.Parallel)` for parallel, chained `.Deps()` for mixed. `CollectAllErrors` option for comprehensive error reporting.
- **Status:** Complete (existing feature)

## UC-4: File caching

- **Actor:** Developer wanting to skip redundant work
- **Starting state:** Developer has a target that only needs to re-run when source files change.
- **End state:** Target execution is skipped if matched files haven't changed since last run.
- **Key interactions:** `.Cache("src/**/*.go")` on target. Content-hash based (SHA256). Cache dir defaults to `.targ-cache`. Use `targ.Disabled` to allow CLI `--cache` override.
- **Status:** Complete (existing feature)

## UC-5: File watching

- **Actor:** Developer wanting continuous re-execution during development
- **Starting state:** Developer wants a target to re-run automatically when files change.
- **End state:** Target re-executes on file changes with polling-based detection.
- **Key interactions:** `.Watch("src/**/*.go")` on target, or `--watch` CLI flag. 250ms default poll interval. Returns `ChangeSet` with Added/Removed/Modified.
- **Status:** Complete (existing feature)

## UC-6: Help and shell completion

- **Actor:** Developer or end user of a targ-built CLI
- **Starting state:** User wants to discover available commands, flags, and usage.
- **End state:** Formatted help output with commands grouped by source, flags documented, examples shown. Shell completion for bash/zsh/fish.
- **Key interactions:** `targ --help`, `targ <cmd> --help`, `targ --completion [bash|zsh|fish]`. Custom examples via `AppendBuiltinExamples`/`PrependBuiltinExamples`.
- **Status:** Complete (existing feature)

## UC-7: Target grouping

- **Actor:** Developer organizing commands into hierarchies
- **Starting state:** Developer has many targets and wants logical subcommand structure.
- **End state:** Targets are organized into named groups with nested hierarchies (e.g., `targ dev lint fast`).
- **Key interactions:** `targ.Group("dev", build, lint)`. Groups can contain targets or other groups. Members are `*Target` or `*TargetGroup`.
- **Status:** Complete (existing feature)

## UC-8: Remote target sync

- **Actor:** Developer importing shared targets from external Go modules
- **Starting state:** Developer wants to use targets defined in another Go package.
- **End state:** External targets are available locally via blank import, with version management.
- **Key interactions:** `targ --sync github.com/org/targets`. Adds blank import and `DeregisterFrom()` call. Re-run to update version. `targ.DeregisterFrom()` to remove specific package's targets.
- **Status:** Complete (existing feature)

## UC-9: Target scaffolding

- **Actor:** Developer creating new targets quickly
- **Starting state:** Developer wants to add a new target without writing boilerplate.
- **End state:** A new target function or shell command is scaffolded in the targ file.
- **Key interactions:** `targ --create NAME [CMD]` with options for deps, cache, watch, timeout, times, retry, backoff, dep-mode. `targ --to-func NAME` and `targ --to-string NAME` for conversion.
- **Status:** Complete (existing feature)

## UC-10: Build tool discovery and compilation

- **Actor:** Developer running `targ` from a project directory
- **Starting state:** Project has Go files with `//go:build targ` tags in various subdirectories.
- **End state:** Targ discovers all tagged files, generates a bootstrap binary, compiles, caches, and executes it.
- **Key interactions:** Automatic — `targ` discovers files, generates imports, builds binary. Handles single-module, multi-module, and no-module cases. Binary caching for fast subsequent runs.
- **Status:** Complete (existing feature)

## UC-11: Runtime overrides

- **Actor:** Developer wanting to temporarily change target behavior from CLI
- **Starting state:** A target has configuration (timeout, deps, cache, watch) but the developer wants to override it for one run.
- **End state:** CLI flags override target config where allowed, with explicit conflict detection.
- **Key interactions:** `--timeout`, `--parallel`, `--times`, `--retry`, `--backoff`, `--watch`, `--cache`, `--dep-mode`, `--while`. Targets use `targ.Disabled` to opt in to CLI override. Conflicts between CLI and target config produce errors.
- **Status:** Complete (existing feature)

## UC-12: Parallel output

- **Actor:** Developer running targets concurrently
- **Starting state:** Multiple targets execute in parallel, producing interleaved output.
- **End state:** Output is prefixed with `[target-name]` and serialized to prevent interleaving.
- **Key interactions:** Automatic in parallel mode. `targ.Print(ctx, ...)` and `targ.Printf(ctx, ...)` for manual prefixed output. Thread-safe line-buffered writer. Lifecycle hooks `.OnStart(fn)` and `.OnStop(fn)`.
- **Status:** Complete (existing feature)

## UC-13: Shell command execution

- **Actor:** Developer running external commands from within targets
- **Starting state:** A target function needs to invoke shell commands.
- **End state:** Commands execute with stdout/stderr streaming or captured output.
- **Key interactions:** `targ.Run(name, args...)`, `targ.RunV(...)` (prints command), `targ.RunContext(ctx, ...)`, `targ.Output(name, args...)`, `targ.OutputContext(ctx, ...)`. Context-aware with cancellation.
- **Status:** Complete (existing feature)

## UC-14: File utilities

- **Actor:** Developer working with files in target functions
- **Starting state:** A target needs to find files, check if they're newer than outputs, or detect content changes.
- **End state:** File operations complete with glob matching, modification time comparison, or content hashing.
- **Key interactions:** `targ.Match(patterns...)` for fish-style glob expansion (`**`, `{a,b}`). `targ.Newer(inputs, outputs)` for modification time comparison. `targ.Checksum(inputs, dest)` for content-based change detection.
- **Status:** Complete (existing feature)

## UC-15: Process cleanup

- **Actor:** Developer running long-lived or forked processes from targets
- **Starting state:** Target spawns child processes that need cleanup on interruption.
- **End state:** Child processes are terminated when the parent receives SIGINT/SIGTERM.
- **Key interactions:** `targ.EnableCleanup()` enables signal handling. Automatic process tree termination. Platform-specific implementations (Unix/Windows).
- **Status:** Complete (existing feature)

## UC-16: Directory tree traversal

- **Actor:** Developer at a terminal
- **Starting state:** Developer has targ target files at multiple levels of the directory tree — in ancestor directories, their `dev/` subdirectories, and/or descendant directories.
- **End state:** All targets across the tree are discoverable and executable from any subdirectory.
- **Key interactions:** Run `targ <name>` from any directory. Targ discovers targets by walking down the full subtree from CWD (existing) and up the linear ancestor path (new), including each ancestor's `dev/` subdirectory. No configuration or opt-in required.
- **Constraints:**
  - Discovery walks the linear ancestor path only (no sibling directories), plus each ancestor's `dev/` subtree.
  - Walks all the way to filesystem root (no root boundary marker).
  - Automatic — no opt-in or configuration needed.
  - Ancestor targets without a `go.mod` compile via isolated build (synthetic go.mod).
  - Conflict handling matches existing behavior (error + deregister suggestion).
  - Help output includes ancestor targets with source attribution.
- **Status:** Pending (issue #11)

## Discovery Model (UC-16)

From CWD `~/repos/personal/project/src/`:

```
~/repos/personal/project/src/**    (full subtree down — unchanged from today)
~/repos/personal/project/          (ancestor) + ~/repos/personal/project/dev/**
~/repos/personal/                  (ancestor) + ~/repos/personal/dev/**
~/repos/                           (ancestor) + ~/repos/dev/**
~/                                 (ancestor) + ~/dev/**
/                                  (ancestor) + /dev/**
```

At each ancestor: check the directory itself for targ-tagged files, plus recursively walk its `dev/` subdirectory if present. No other siblings are discovered.

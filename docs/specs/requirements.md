# L2: Requirements and Design

Items induced from: L2 architecture items (bottom-up)

## REQ-1: Define targets from functions or shell strings

A target can be created from a Go function (with optional struct parameter for CLI args) or a shell command string. Both forms produce the same `Target` type with the same builder methods. Shell strings run in the user's shell with variable interpolation as CLI flags.

**Induced from:** ARCH-1, ARCH-2, ARCH-3
**Traces to:** UC-1, UC-2

## REQ-2: Struct tags define CLI arguments

Struct fields tagged with `targ:"..."` become CLI arguments. Tags specify: flag/positional kind, name override, short alias, description, default value, enum constraints, environment variable fallback, required status. Supports bool, int, float64, string, slices, maps, embedded structs, and `Interleaved[T]` for order-preserving repeated flags. `TagOptions` interface allows runtime override.

**Induced from:** ARCH-2
**Traces to:** UC-1, UC-2

## REQ-3: Target execution with runtime overrides

Targets execute their function with parsed arguments. Built-in runtime flags (`--times`, `--timeout`, `--watch`, `--cache`, `--parallel`, `--retry`, `--backoff`, `--dep-mode`) modify execution behavior. Overrides are extracted before target-specific argument parsing.

**Induced from:** ARCH-3, ARCH-15
**Traces to:** UC-1

## REQ-4: Configuration conflict detection

When both compile-time target configuration and CLI flags specify the same setting, execution errors rather than silently choosing one. `targ.Disabled` sentinel explicitly opts a setting into CLI flag control.

**Induced from:** ARCH-15, ARCH-3
**Traces to:** UC-1

## REQ-5: Target registration and deregistration

Targets register via `Register()` in `init()` functions. `DeregisterFrom(packagePath)` removes all targets from a package. Duplicate target names across packages produce a conflict error. Resolution finalizes the registry before execution.

**Induced from:** ARCH-4
**Traces to:** UC-2

## REQ-6: Hierarchical command groups

Targets can be organized into named groups forming a command tree. Groups can nest. Command resolution walks the tree by matching path segments. Glob patterns match multiple targets within groups.

**Induced from:** ARCH-5
**Traces to:** UC-2

## REQ-7: Execution results are classified and reported

Every target execution produces a result: Pass, Fail, Cancelled, or Errored. Parallel mode with `CollectAllErrors` collects all failures. Summary output shows per-target status with truncated error snippets.

**Induced from:** ARCH-6
**Traces to:** UC-5

## REQ-8: Parallel output is prefixed and serialized

When targets run in parallel, each output line is prefixed with the target name. Output from concurrent targets is serialized — no interleaving within a line. Prefix alignment pads names to max length.

**Induced from:** ARCH-7
**Traces to:** UC-5

## REQ-9: External commands are context-cancellable

Shell commands run via `Run()`/`Output()` with optional context support. Context cancellation kills the entire process tree (platform-specific: Unix process groups, Windows job objects). SIGINT/SIGTERM cleanup kills all spawned child processes.

**Induced from:** ARCH-8
**Traces to:** UC-1

## REQ-10: File operations for caching and watching

Glob matching (fish-style `**` and `{a,b}`), content-based checksumming for cache invalidation, and file change polling for watch mode. These support the `--cache` and `--watch` execution features.

**Induced from:** ARCH-9
**Traces to:** UC-1

## REQ-11: Help text is auto-generated

Help output is generated from target metadata: description, usage, flags (with short aliases), positionals, subcommands, groups, examples, execution info (deps, cache, watch, timeout), and source file location. ANSI styling for terminal. Binary mode hides targ-specific flags.

**Induced from:** ARCH-10
**Traces to:** UC-3

## REQ-12: Shell completion for bash, zsh, fish

`--completion [shell]` generates shell-specific completion scripts supporting commands, subcommands, flags, and enum values. Auto-detects shell from environment.

**Induced from:** ARCH-10
**Traces to:** UC-3

## REQ-13: Build tool discovers and compiles targets

The `targ` CLI discovers `//go:build targ` files by walking both downward (CWD non-recursively, then CWD/dev/ recursively) and upward (linear ancestor path, stopping before the filesystem root to avoid system directories like /dev). At each ancestor directory, targ checks the directory itself and recursively walks its `dev/` subtree if present. No sibling directories are searched. Generates a bootstrap Go binary, compiles it (cached in `~/.cache/targ/`), and executes it. Meta-commands scaffold, sync, and convert targets.

**Induced from:** ARCH-11, ARCH-12
**Traces to:** UC-4, UC-6

## REQ-14: Source location shown in help

Help output includes the source file:line where a target is defined. Derived from runtime caller information.

**Induced from:** ARCH-13
**Traces to:** UC-3

## REQ-15: Git clean check as precondition

`CheckCleanWorkTree()` verifies no uncommitted changes exist. Can be used as a target precondition (e.g., before deploy).

**Induced from:** ARCH-13
**Traces to:** UC-3

## REQ-16: Ancestor targets build as independent module groups

Each ancestor directory with targ-tagged files forms its own module group. Ancestors with a `go.mod` use normal module build. Ancestors without `go.mod` use isolated build (synthetic `go.mod`, copied files). No cross-module imports between ancestor groups. Existing multi-module aggregation handles command merging.

**Derived from:** UC-6
**Traces to:** UC-6

## REQ-17: Upward discovery is automatic with no root boundary

Upward discovery requires no opt-in — it is always active. The walk continues up the linear ancestor path (parent, grandparent, ...) stopping before the filesystem root — no project lives at `/` and checking `/dev/` would walk system device directories. Sibling directories of ancestors are never searched.

**Derived from:** UC-6
**Traces to:** UC-6

## REQ-18: Most-local-wins superseding for duplicate commands

When the same command name is discovered at multiple directory levels (e.g., CWD/dev/ and an ancestor), the most-local version (closest to CWD) wins for dispatch. Help output shows the primary version normally and marks higher-level duplicates as superseded. The primary version annotates what it supersedes (source directory). Superseding is determined by discovery depth — lower depth = more local = higher priority.

**Derived from:** UC-3, UC-6
**Traces to:** UC-3, UC-6

## DES-7: Superseding display format in help

In multi-module help, commands are grouped by source. When a command is superseded by a more-local version, it is displayed with a visual annotation (e.g., strikethrough or dimmed styling with "superseded by <source>" label). The primary (most-local) version shows a note like "supersedes <source>" after its name. This gives the developer full visibility into what's available at each level while making clear which version will actually run.

**Derived from:** UC-3, UC-6
**Traces to:** UC-3, UC-6

## DES-6: Ancestor dev/ subtree convention

At each ancestor directory during upward discovery, targ checks two locations: (1) the ancestor directory itself for targ-tagged `.go` files, and (2) if `<ancestor>/dev/` exists, recursively walks it for targ-tagged files. This supports the convention of placing build tooling in a `dev/` directory without polluting the project root.

**Derived from:** UC-6
**Traces to:** UC-6

## DES-1: Two-mode usage (build tool and library)

**Build tool mode:** Targets defined in `//go:build targ` files with `Register()` in `init()`. Invoked via `targ <command>`. **Library mode:** Targets passed to `Main()` in a standalone binary. Same target definition API, different entry point. Smooth migration path: remove build tag, change `init()` to `main()`, change `Register()` to `Main()`.

**Induced from:** ARCH-3, ARCH-4, ARCH-12, ARCH-14
**Traces to:** UC-2

## DES-2: Progressive target complexity

**Stage 1:** Shell command strings — `targ.Targ("go test ./...")`. Variables become flags. **Stage 2:** Functions with struct parameters — typed flags, conditional logic. **Stage 3:** Standalone binary — same code, `Main()` instead of `Register()`. Each stage is a superset of the previous.

**Induced from:** ARCH-1, ARCH-2, ARCH-3
**Traces to:** UC-1, UC-2

## DES-3: Builder pattern for target configuration

Fluent builder API: `targ.Targ(fn).Name("x").Deps(a, b).Cache("**/*.go").Watch("**/*.go").Timeout(5*time.Minute)`. Methods return `*Target` for chaining. Dependencies support serial (default), parallel, and mixed modes via chained `.Deps()` calls. Deps-only targets: `targ.Targ().Name("all").Deps(build, test)`.

**Induced from:** ARCH-1
**Traces to:** UC-1

## DES-4: Scaffold and convert targets via CLI

`--create NAME [CMD]` scaffolds new targets (function or shell string). `--to-func NAME` converts string target to function. `--to-string NAME` converts function to shell string. `--sync PACKAGE` imports remote module targets with DeregisterFrom boilerplate.

**Induced from:** ARCH-12
**Traces to:** UC-4

## DES-5: Dependency execution model

`.Deps(targets..., mode)` declares dependencies. Serial by default. `targ.DepModeParallel` for parallel. Chain `.Deps()` for mixed serial/parallel groups. In parallel mode, fail-fast cancels remaining targets (default) or `CollectAllErrors` runs all and reports all failures.

**Induced from:** ARCH-1, ARCH-6, ARCH-7
**Traces to:** UC-1, UC-5

# Requirements & Design: UC-6 through UC-10

## UC-6: Help and shell completion

### Requirements

#### REQ-6-1: Type-safe help builder
- **Traces to:** UC-6
- **Source:** `internal/help/builder.go:10-26`, `internal/help/content.go:16-37`
- **Acceptance criteria:** Help output is constructed via a Builder using a type-state pattern. `New()` requires a non-empty command name (panics otherwise). `WithDescription()` transitions to `ContentBuilder` which accumulates sections: flags, subcommands, command groups, positionals, examples, formats, values, execution info, source file, shell command, usage, and more-info text.

#### REQ-6-2: Canonical section ordering in rendered output
- **Traces to:** UC-6
- **Source:** `internal/help/render.go:9-16`, `internal/help/render.go:89-115`, `internal/help/render.go:267-283`
- **Acceptance criteria:** `Render()` produces sections in fixed order: Description, Source, Command, Usage, Targ flags (Global/Root-only), Values, Formats, Positionals, Command-specific flags, Subcommands, Command groups, Execution info, Examples, More info. Empty sections are omitted. Sections are joined by double newlines.

#### REQ-6-3: Root-level help generation
- **Traces to:** UC-6
- **Source:** `internal/help/generators.go:130-176`
- **Acceptance criteria:** `WriteRootHelp()` renders root help with: usage line (`<binary> [targ flags...] [<command>...]`), commands grouped by source file, auto-generated examples if none provided, deregistered packages listed separately. Binary mode uses `[flags...]` instead of `[targ flags...]`.

#### REQ-6-4: Target-level help generation
- **Traces to:** UC-6
- **Source:** `internal/help/generators.go:179-246`
- **Acceptance criteria:** `WriteTargetHelp()` renders target help with: description, source file, shell command (if string target), usage line, targ flags (non-root), target-specific flags, subcommands, execution info (deps/cache/watch/timeout/times/retry), and auto-generated examples. Examples auto-generated from target name and flags if none provided.

#### REQ-6-5: Targ flag filtering by mode
- **Traces to:** UC-6
- **Source:** `internal/help/builder.go:108-137`, `internal/help/builder.go:182-227`
- **Acceptance criteria:** `AddTargFlagsFiltered()` filters visible flags based on `TargFlagFilter`: root-only flags hidden at non-root level, binary mode shows only `FlagModeAll` flags, individual flags (completion, help, timeout) can be disabled. Automatically adds format descriptions for flag placeholders.

#### REQ-6-6: ANSI-styled output
- **Traces to:** UC-6
- **Source:** `internal/help/styles.go`, `internal/help/render.go:193-227`
- **Acceptance criteria:** Help output uses lipgloss styles for headers, flags, placeholders, and subsections. Flag rendering aligns descriptions at a minimum column width (30 visible chars). ANSI sequences are stripped for width calculation.

#### REQ-6-7: Shell completion for bash, zsh, and fish
- **Traces to:** UC-6
- **Source:** `internal/core/completion.go:14-33`
- **Acceptance criteria:** `PrintCompletionScriptTo()` writes shell-specific completion scripts for bash, zsh, and fish. Each script invokes `<binary> __complete "<request>"` to get completions. Returns `errUnsupportedShell` for unknown shells.

#### REQ-6-8: Context-aware completion suggestions
- **Traces to:** UC-6
- **Source:** `internal/core/completion.go:679-692`, `internal/core/completion.go:217-249`, `internal/core/completion.go:279-317`
- **Acceptance criteria:** `doCompletion()` tokenizes the command line, resolves the command chain (root -> subcommands), then suggests: subcommands (including siblings and parent), enum values for flags and positionals, available flags (command-specific + targ global/root-only), and root commands when positionals are complete. Prefix filtering applied to all suggestions.

#### REQ-6-9: Command-line tokenization for completion
- **Traces to:** UC-6
- **Source:** `internal/core/completion.go:77-160`
- **Acceptance criteria:** `tokenizeCommandLine()` handles single/double quotes, backslash escaping, and whitespace splitting. Tracks whether cursor is at a new argument position (trailing whitespace) vs mid-token. Unclosed quotes suppress the new-arg flag.

#### REQ-6-10: Targ flag skipping in completion
- **Traces to:** UC-6
- **Source:** `internal/core/completion.go:913-943`
- **Acceptance criteria:** `skipTargFlags()` removes targ-level flags (both boolean and value-taking) from completion args so they don't interfere with command resolution. Handles `--flag value`, `--flag=value`, and boolean flag formats.

### Design

#### DES-6-1: Type-state builder pattern
- **Traces to:** UC-6
- **Interaction model:** Library code calls `help.New(name).WithDescription(desc)` to get a `ContentBuilder`, then chains `Add*()` methods to accumulate sections. `Render()` produces the final string. The type transition from `Builder` to `ContentBuilder` enforces that description is set first.

#### DES-6-2: Two-tier help generation
- **Traces to:** UC-6
- **Interaction model:** `WriteRootHelp()` is invoked for `targ --help` (or `<binary> --help` in binary mode). `WriteTargetHelp()` is invoked for `targ <target> --help`. Both write to an `io.Writer` and auto-generate examples when none are provided. Root help includes command groups by source file; target help includes execution configuration.

#### DES-6-3: Completion via `__complete` internal command
- **Traces to:** UC-6
- **Interaction model:** User runs `targ --completion bash|zsh|fish` to install a completion script. The script calls `<binary> __complete "<command-line>"` on each tab press. The binary tokenizes the line, walks the command tree, and prints matching suggestions to stdout (one per line).

---

## UC-7: Target grouping

### Requirements

#### REQ-7-1: Named group creation with validation
- **Traces to:** UC-7
- **Source:** `internal/core/group.go:40-69`
- **Acceptance criteria:** `Group()` creates a `TargetGroup` with a name matching `^[a-z][a-z0-9-]*$`. Panics if name is empty, invalid, or any member is not `*Target` or `*TargetGroup`. Members can be targets or nested groups for hierarchical organization.

#### REQ-7-2: Group members as subcommands
- **Traces to:** UC-7
- **Source:** `internal/core/command.go:1442-1473`
- **Acceptance criteria:** `parseGroupLike()` converts a `TargetGroup` into a `commandNode` with subcommands. Each member is recursively parsed via `parseTarget()`. Subcommands are stored in a map keyed by name. Parent pointers are set for navigation. Source package attribution is preserved from the group.

#### REQ-7-3: Group navigation and execution
- **Traces to:** UC-7
- **Source:** `internal/core/command.go:968-1016`
- **Acceptance criteria:** `executeGroupWithParents()` handles group nodes (subcommands but no function). Given args, it looks up the first arg as a subcommand name (case-insensitive match). Supports glob patterns for matching multiple subcommands. When a glob matches, all matching subcommands execute. Returns remaining args for chaining.

#### REQ-7-4: Help display for groups
- **Traces to:** UC-7
- **Source:** `internal/core/command.go:760-768`
- **Acceptance criteria:** `collectHelpSubcommands()` builds a sorted list of subcommand names and descriptions from a node's Subcommands map, displayed in target help output under the "Subcommands:" section.

### Design

#### DES-7-1: Hierarchical command tree via `commandNode`
- **Traces to:** UC-7
- **Interaction model:** User defines groups with `targ.Group("dev", build, lint)`. Groups can nest: `targ.Group("ci", dev, deploy)`. At runtime, `targ dev lint` navigates the tree: root -> "dev" group -> "lint" target. The `commandNode` tree supports parent pointers for sibling suggestions in completion and the `^` escape to parent.

---

## UC-8: Remote target sync

### Requirements

#### REQ-8-1: Sync argument parsing and validation
- **Traces to:** UC-8
- **Source:** `internal/runner/runner.go:746-761`
- **Acceptance criteria:** `ParseSyncArgs()` requires exactly one argument — a module path that passes `looksLikeModulePath()` validation. Returns `errSyncUsage` if missing, `errInvalidPackagePath` if invalid format.

#### REQ-8-2: Package fetch and import injection
- **Traces to:** UC-8
- **Source:** `internal/runner/runner.go:1637-1692`
- **Acceptance criteria:** `handleSyncFlag()` fetches the remote package via `go get`, adds a blank import to the targ file, adds a `targ.DeregisterFrom()` call in `init()`, and ensures the `targ` import exists. Checks for duplicate imports before syncing. Prints instructions for selective re-registration.

#### REQ-8-3: Import and deregistration code generation
- **Traces to:** UC-8
- **Source:** `internal/runner/runner.go:147-183`
- **Acceptance criteria:** `AddImportToTargFileWithFileOps()` parses the Go file AST, adds a blank import for the package path, ensures `github.com/toejough/targ` is imported, adds `targ.DeregisterFrom("<pkg>")` to `init()`, and writes the formatted result. Uses injected `FileOps` for testability.

#### REQ-8-4: Conflict detection across packages
- **Traces to:** UC-8
- **Source:** `internal/core/registry.go:140-194`
- **Acceptance criteria:** `detectConflicts()` scans all registered targets and groups, building a map of name -> source packages. If any name appears with multiple different sources, returns a `*ConflictError` listing all conflicts with resolution guidance ("Use targ.DeregisterFrom() to resolve").

#### REQ-8-5: Deregistration with validation
- **Traces to:** UC-8
- **Source:** `internal/core/registry.go:87-106`, `internal/core/registry.go:55-81`
- **Acceptance criteria:** `applyDeregistrations()` filters registry items by package path, only removing items registered before the deregistration call (using `RegistryLen` snapshot). Returns `DeregistrationError` if a deregistered package had zero matches. `resolveRegistry()` applies deregistrations first, then checks for conflicts, then clears local target sources.

#### REQ-8-6: Local source attribution clearing
- **Traces to:** UC-8
- **Source:** `internal/core/registry.go:110-136`
- **Acceptance criteria:** `clearLocalTargetSources()` clears `sourcePkg` for targets and groups belonging to the main module (determined via `mainModuleProvider`). A package belongs to the main module if its path equals or is prefixed by the module path. This ensures local targets show no source attribution in help output.

### Design

#### DES-8-1: Sync workflow with safe defaults
- **Traces to:** UC-8
- **Interaction model:** User runs `targ --sync github.com/org/targets`. Targ fetches the package, adds a blank import plus `DeregisterFrom()` to the targ file. All synced targets are deregistered by default to prevent conflicts. User edits the targ file to selectively re-register desired targets. `targ --sync --help` shows usage.

#### DES-8-2: Registry resolution pipeline
- **Traces to:** UC-8
- **Interaction model:** At startup, the compiled binary calls `resolveRegistry()` which: (1) applies all deregistrations (removing targets from specified packages), (2) detects name conflicts across remaining targets, (3) clears source attribution for main-module targets. This three-phase pipeline ensures deterministic conflict resolution.

---

## UC-9: Target scaffolding

### Requirements

#### REQ-9-1: Create argument parsing
- **Traces to:** UC-9
- **Source:** `internal/runner/runner.go:690-717`
- **Acceptance criteria:** `ParseCreateArgs()` requires at least 2 arguments (name + shell command). Last argument is the shell command. Preceding arguments are parsed for: positional path/name components, `--deps`, `--cache`, `--watch` (list flags), `--timeout`, `--backoff`, `--dep-mode` (single-value flags), `--times` (int), `--retry` (bool). Path components split into group path + target name.

#### REQ-9-2: Target code generation and file insertion
- **Traces to:** UC-9
- **Source:** `internal/runner/runner.go:188-249`
- **Acceptance criteria:** `AddTargetToFileWithFileOps()` reads the target file, checks for duplicate targets (by var name or `.Name()` call), generates target code, creates group modifications if path is nested, adds time import if needed, inserts `targ.Register()` call in `init()`, and writes the result. For nested groups, registers the top-level group var.

#### REQ-9-3: Group path handling for nested targets
- **Traces to:** UC-9
- **Source:** `internal/runner/runner.go:373-421`
- **Acceptance criteria:** `CreateGroupMemberPatch()` finds an existing `targ.Group()` declaration in file content and creates a patch to add a new member. Handles nested parentheses. Returns nil if the member already exists (idempotent). Group variable names use PascalCase from path components via `PathToPascal()`.

#### REQ-9-4: Find or create targ file
- **Traces to:** UC-9
- **Source:** `internal/runner/runner.go:542-547`
- **Acceptance criteria:** `FindOrCreateTargFile()` / `FindOrCreateTargFileWithFileOps()` locates the nearest existing targ file or creates a new one with the `//go:build targ` tag and necessary imports.

#### REQ-9-5: Target format conversion
- **Traces to:** UC-9
- **Source:** `internal/runner/runner.go:304-369`
- **Acceptance criteria:** `ConvertFuncTargetToString()` finds a function target whose function body is a simple `sh.Run` call, extracts the shell command, and replaces the function reference with a string literal. `ConvertStringTargetToFunc()` does the reverse — replaces a string literal with a function reference and generates a wrapper function. Both use AST manipulation and `go/format` for output.

#### REQ-9-6: Create option validation
- **Traces to:** UC-9
- **Source:** `internal/runner/runner.go:3939`
- **Acceptance criteria:** `validateCreateOptions()` validates all options before code generation: name format, timeout duration parsing, backoff format, dep-mode values, and times count.

### Design

#### DES-9-1: CLI-driven scaffolding workflow
- **Traces to:** UC-9
- **Interaction model:** User runs `targ --create test "go test ./..."` or with options like `targ --create dev lint --deps fmt --cache "**/*.go" "golangci-lint run"`. Targ finds/creates the targ file, generates a target variable with the appropriate `targ.Targ()` chain, adds it to `init()` via `targ.Register()`, and prints confirmation. For groups, creates or patches `targ.Group()` declarations.

#### DES-9-2: Bidirectional format conversion
- **Traces to:** UC-9
- **Interaction model:** `targ --to-func <name>` converts a string target (e.g., `targ.Targ("go test")`) to a function target with a generated wrapper function. `targ --to-string <name>` converts the other direction if the function is a simple `targ.Run` call. Both use Go AST parsing for safe code transformation. `--help` available for each.

---

## UC-10: Build tool discovery and compilation

### Requirements

#### REQ-10-1: Tagged file discovery via directory traversal
- **Traces to:** UC-10
- **Source:** `internal/discover/discover.go:68-96`, `internal/discover/discover.go:316-342`
- **Acceptance criteria:** `Discover()` walks directories breadth-first from `StartDir`, reading each file to check for `//go:build targ` tags. Skips hidden dirs, vendor, testdata, and non-Go files. Returns `[]PackageInfo` with directory, package name, doc comment, file paths, and whether explicit registration is used. Default build tag is `"targ"`.

#### REQ-10-2: Package info extraction from AST
- **Traces to:** UC-10
- **Source:** `internal/discover/discover.go:138-263`
- **Acceptance criteria:** `parsePackageInfo()` parses all tagged files in a directory, validates consistent package names (returns `ErrMultiplePackageNames` if mixed), captures package doc comments, detects `main()` functions (returns `ErrMainFunctionNotAllowed`), and checks for `targ.Register()` calls in `init()` to determine explicit registration mode.

#### REQ-10-3: Module grouping for multi-module builds
- **Traces to:** UC-10
- **Source:** `internal/runner/runner.go:3310-3353`
- **Acceptance criteria:** `groupByModule()` groups discovered packages by their Go module (found via `FindModuleForPath()`). Packages without a module use `startDir` as a pseudo-module with a local module name. Results are sorted by module root for deterministic ordering.

#### REQ-10-4: Single-module build path
- **Traces to:** UC-10
- **Source:** `internal/runner/runner.go:1588-1612`
- **Acceptance criteria:** `handleSingleModule()` handles the case where all packages belong to one module. Collects file paths, checks for `go.mod`. If no module found, delegates to `handleIsolatedModule()`. Otherwise, generates bootstrap code, computes cache key, checks binary cache, and builds+runs if needed.

#### REQ-10-5: Multi-module build path
- **Traces to:** UC-10
- **Source:** `internal/runner/runner.go:1542-1575`
- **Acceptance criteria:** `handleMultiModule()` builds separate binaries for each module group via `buildMultiModuleBinaries()`, creating a command registry. For help requests, prints combined help. For commands, dispatches to the appropriate module's binary via `dispatchCommand()`. Propagates exit codes from child processes.

#### REQ-10-6: Isolated module build for no-module targets
- **Traces to:** UC-10
- **Source:** `internal/runner/runner.go:1493-1540`
- **Acceptance criteria:** `handleIsolatedModule()` handles targets without a `go.mod`. Creates an isolated build directory with a synthetic `go.mod`, resolves targ as a dependency (from build info), remaps package paths, generates bootstrap, and builds in the isolated directory. Uses `startDir` for cache key computation to enable caching across runs.

#### REQ-10-7: Bootstrap code generation
- **Traces to:** UC-10
- **Source:** `internal/runner/runner.go:910-912`
- **Acceptance criteria:** Bootstrap code is generated from a template that creates a `package main` with `import` statements for all discovered packages and a `main()` that calls the targ runtime. The bootstrap file is written to a temp file, compiled via `go build`, and cleaned up after.

#### REQ-10-8: Binary caching
- **Traces to:** UC-10
- **Source:** `internal/runner/runner.go:1287-1323`, `internal/runner/runner.go:1532-1536`
- **Acceptance criteria:** Compiled binaries are cached by a content-based hash of source files. `tryRunCached()` checks if a valid cached binary exists before rebuilding. `--no-binary-cache` flag disables caching. Cache key uses SHA-256 of source file contents for deterministic invalidation.

#### REQ-10-9: Targ flag extraction
- **Traces to:** UC-10
- **Source:** `internal/runner/runner.go:459-497`
- **Acceptance criteria:** `ExtractTargFlags()` separates targ-specific flags (`--no-binary-cache`, `--source`/`-s`) from args before passing remaining args to the compiled binary. `--source` is position-sensitive — only recognized before the first target name. `--no-cache` is accepted as deprecated alias with a warning.

#### REQ-10-10: Package main rejection
- **Traces to:** UC-10
- **Source:** `internal/runner/runner.go:1325-1346`, `internal/discover/discover.go:255-263`
- **Acceptance criteria:** Tagged files must not use `package main`. Discovery validates at the AST level (`ErrMainFunctionNotAllowed` if `main()` found). Runner validates at the package level (`errPackageMainNotAllowed` if package name is "main"), with guidance to use a named package instead.

### Design

#### DES-10-1: Three-path compilation strategy
- **Traces to:** UC-10
- **Interaction model:** When user runs `targ`, the runner discovers tagged files and groups them by module. Three paths: (1) **Single module** — all packages in one `go.mod`, bootstrap imports them and builds in-module. (2) **Multi-module** — packages span multiple `go.mod` files, each module built separately, commands dispatched to the right binary. (3) **Isolated** — no `go.mod` found, synthetic module created with targ as dependency, files copied to isolated build dir.

#### DES-10-2: Discovery and bootstrap pipeline
- **Traces to:** UC-10
- **Interaction model:** Automatic on every `targ` invocation. Pipeline: (1) Discover tagged files via BFS directory walk, (2) Parse package info from AST, (3) Group by module, (4) Generate bootstrap `main.go` importing all packages, (5) Check binary cache by content hash, (6) Build if cache miss, (7) Execute binary with remaining args. The user sees only the final command output.

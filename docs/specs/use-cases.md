# L1: Use Cases (Adoption)

Induced bottom-up from L2 REQ/DES items.

## UC-1: Developer Defines and Runs Build Targets

**Actor:** Developer using targ as a build system or CLI framework.

**Starting state:** A Go project exists. The developer wants to automate build tasks or create a CLI.

**End state:** Build targets execute correctly — functions run, dependencies resolve, errors surface, output is readable even in parallel.

**Key interactions:**
1. Developer writes a Go function (or shell command string) and wraps it with `Targ()`.
2. Developer adds struct tags to function parameters for CLI argument parsing.
3. Developer chains builder methods (`.Name()`, `.Deps()`, `.Cache()`, `.Watch()`, `.Timeout()`).
4. Developer calls `Main()` or `ExecuteRegistered()` to run.
5. System parses CLI args from struct tags, validates input (required, enum, types), and executes.
6. Dependencies run in serial or parallel mode before the target.
7. Parallel output is prefix-tagged and ordered.
8. Runtime flags (--cache, --watch, --times) override target config without code changes.
9. Errors aggregate in parallel mode (MultiError). Results report Pass/Fail/Errored/Cancelled.

**Constraints:**
- No configuration surprises — conflicting settings error, never silently pick a winner.
- Shell command and Go function targets are interchangeable in the API.
- Parallel output is prefix-tagged and ordered — never interleaved.

**L2 items:** REQ-1, REQ-2, REQ-5, REQ-8, DES-1, DES-2, REQ-10 (git-aware guards)

## UC-2: Developer Manages Multi-Package Target Libraries

**Actor:** Developer or team maintaining shared build targets across packages.

**Starting state:** Multiple packages define targets (local and remote via blank import). Some target names may conflict.

**End state:** Targets from all packages are available, conflicts are resolved, and the developer can scaffold new targets or import remote ones.

**Key interactions:**
1. Developer registers targets via `Register()` in `init()`.
2. Remote packages provide targets via blank import.
3. System detects naming conflicts across packages — reports all at once with fix suggestions.
4. Developer uses `DeregisterFrom()` to remove a conflicting package's targets, then re-registers replacements.
5. Developer organizes targets into groups via `Group()` with nested namespacing and dot-path addressing.
6. `targ` CLI auto-discovers target files via `//go:build targ` tags.
7. `targ --create` scaffolds new target files with correct Go code (AST-based generation).
8. `targ --sync` imports remote targets.

**Constraints:**
- Conflict detection reports all conflicts at once — not fail-fast on first.
- Deregistration is idempotent.
- Code generation uses AST manipulation, not string templating.

**L2 items:** REQ-3, REQ-6, REQ-9, DES-4, DES-5, REQ-11 (example correctness)

## UC-3: Developer Discovers Available Targets and Their Usage

**Actor:** Developer using a targ-based project (may not be the author).

**Starting state:** A targ-based project exists with registered targets.

**End state:** The developer understands what targets are available, how to invoke them, and what arguments they accept.

**Key interactions:**
1. Developer runs `<binary> --help` to see available targets and global flags.
2. Developer runs `<binary> <target> --help` to see target-specific help.
3. Help output shows sections in fixed order: usage, description, subcommands, flags, positionals, examples.
4. ANSI styling highlights structure; examples are code-only (no styling).
5. Binary mode filters targ-specific flags from help output.
6. Tab completion suggests targets, flags, and argument values in the shell.

**Constraints:**
- ANSI codes are always properly paired.
- Empty help sections are omitted, not shown as blank.

**L2 items:** REQ-4, REQ-7, DES-3

## UC-4: Developer Graduates Build Targets to Standalone CLI

**Actor:** Developer who started with targ build targets and wants to ship a standalone CLI binary.

**Starting state:** A working set of targ targets exists (defined via `Targ()`, running via `targ` CLI or `ExecuteRegistered()`).

**End state:** The same target definitions produce a standalone CLI binary that looks and behaves like a purpose-built tool — no targ internals visible to end users.

**Key interactions:**
1. Developer switches from `targ`-based discovery to `Main()` in a `cmd/` package.
2. Binary mode activates — targ-specific flags (--cache, --watch, etc.) are hidden from help.
3. Help output shows only the CLI's own flags and subcommands.
4. Flag section label changes from "Targ Flags" to "Flags".
5. Shell completion works for the standalone binary.
6. The developer makes minimal code changes — same target definitions, different entry point.

**Constraints:**
- Binary mode hides targ internals — the CLI looks like a standalone tool, not a targ wrapper.
- The graduation path requires minimal changes — ideally just switching the entry point.
- All target features (args, deps, groups, help, completion) work identically in both modes.

**L2 items:** REQ-4 (binary mode filtering), REQ-7 (completion), DES-3 (binary mode UX), DES-1 (target definition unchanged), DES-5 (runner/scaffolding)

# L1: Use Cases

Bottom-up derivation from requirements, design, and existing implementation.

## UC-1: Define and Execute Targets

**Actor:** Developer
**Starting state:** Go project, targ installed
**End state:** Developer runs `targ <command>` to execute custom build targets

**Key interactions:**
1. Create a Go file with `//go:build targ` tag
2. Define targets via `targ.Targ(func)` or `targ.Targ("shell command")`
3. Register targets in `init()` with `targ.Register()`
4. Run `targ <command>` from terminal

**Covers:** Function targets (8 signatures), string command targets ($variable substitution), deps-only targets, builder method chaining, automatic kebab-case naming, `.Name()` override.

## UC-2: Parse CLI Arguments from Struct Tags

**Actor:** Developer
**Starting state:** Target function exists with no parameters
**End state:** Target accepts typed CLI arguments parsed from struct tags

**Key interactions:**
1. Add struct parameter to target function
2. Annotate fields with `targ:"flag,short=f,desc=..."` tags
3. Run `targ <command> --flag=value` or `targ <command> positional-arg`

**Covers:** Flags, positionals, short aliases, defaults, required, enums, env var fallback, maps (key=value), slices (repeated flags), interleaved (order-preserving), embedded structs, TextUnmarshaler/StringSetter, float64, Duration.

## UC-3: Organize Targets in Groups

**Actor:** Developer
**Starting state:** Flat list of targets
**End state:** Hierarchical command structure with `targ group subcommand`

**Key interactions:**
1. Wrap targets in `targ.Group("name", targets...)`
2. Nest groups arbitrarily
3. Run `targ group subcommand` or use glob patterns `targ dev *`

**Covers:** Nested groups, group name validation, glob selection (*, **), caret reset (^), namespace nodes, list command.

## UC-4: Control Execution Behavior

**Actor:** Developer
**Starting state:** Target that runs once with no execution control
**End state:** Target with dependencies, caching, watching, retries, timeouts

**Key interactions:**
1. Chain builder methods: `.Deps()`, `.Cache()`, `.Watch()`, `.Timeout()`, `.Times()`, `.Retry()`, `.Backoff()`, `.While()`
2. Configure dep mode: serial (default) or parallel
3. Run target — execution engine handles lifecycle

**Covers:** Serial/parallel deps, dep chaining, cache skip, file watching, timeout enforcement, retry with backoff, while predicate, parallel failure collection.

## UC-5: Override Target Settings from CLI

**Actor:** End user
**Starting state:** Target has coded execution settings
**End state:** Settings overridden via CLI flags

**Key interactions:**
1. Run `targ build --timeout=30s` to override target's timeout
2. If target has `.Timeout(5m)`, get conflict error
3. Developer uses `targ.Disabled` to allow CLI override

**Covers:** Override flags (--timeout, --times, --retry, --backoff, --cache, --watch, --dep-mode, --cache-dir, --deps), conflict detection, targ.Disabled sentinel.

## UC-6: Get Help Output

**Actor:** End user
**Starting state:** Targets registered
**End state:** Formatted help displayed

**Key interactions:**
1. Run `targ --help` for root help with subcommand list
2. Run `targ <command> --help` for target help with flags, usage, examples
3. View ANSI-styled output with proper section ordering

**Covers:** Description, source file:line, usage line, positional placeholders, flag descriptions, enum values, dep group display, cache/watch patterns, auto-generated examples, user-provided examples, binary mode adjustments.

## UC-7: Get Shell Completions

**Actor:** End user
**Starting state:** Targets registered, shell configured
**End state:** Tab completion works for commands, flags, and values

**Key interactions:**
1. Run `source <(targ --completion)` to install completions
2. Type `targ <TAB>` for command suggestions
3. Type `targ build --<TAB>` for flag suggestions

**Covers:** Bash/zsh/fish script generation, auto shell detection, __complete subcommand, subcommand/flag/enum suggestions, disabled completion, completion does not execute.

## UC-8: Build Dedicated CLI Binary

**Actor:** Developer
**Starting state:** Build tool targets with `//go:build targ`
**End state:** Standalone binary with `go build`

**Key interactions:**
1. Remove `//go:build targ` tag
2. Change to `package main`
3. Replace `Register()` + `init()` with `Main()` in `main()`
4. Build: `go build -o mytool .`

**Covers:** Binary mode flag hiding, "Flags" label (not "Global Flags"), examples use binary name, Main() entry point.

## UC-9: Scaffold and Import Targets

**Actor:** Developer
**Starting state:** Project with targ installed
**End state:** New targets scaffolded or remote targets imported

**Key interactions:**
1. `targ --create build` scaffolds function target
2. `targ --create tidy "go mod tidy"` scaffolds shell command target
3. `targ --sync github.com/company/shared-targets` imports remote targets

**Covers:** Function/string scaffolding, --create flags (--deps, --cache, --watch, etc.), --sync blank import + DeregisterFrom, --to-func, --to-string conversion, kebab-to-PascalCase, valid code generation.

## UC-10: Run Commands and Check Files

**Actor:** Developer
**Starting state:** Target function needs to run external commands or check file freshness
**End state:** External commands executed, file freshness checked

**Key interactions:**
1. Call `targ.Run("go", "build", "./...")` from target function
2. Call `targ.Newer(inputs, outputs)` to check if rebuild needed
3. Call `targ.Checksum(inputs, dest)` for content-based checking
4. Use context variants for cancellation support

**Covers:** Run/RunV/RunContext/RunContextV, Output/OutputContext, process tree kill on cancel, Newer (modtime), Checksum (content hash), Match (glob), Watch (file watcher).

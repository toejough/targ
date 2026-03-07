# L2: Requirements

Bottom-up derivation from architecture and existing behavior.

## REQ-1: Target Definition Accepts Functions and Strings

Traces to: UC-1

`Targ()` accepts: Go functions (8 valid signatures: niladic through context+struct+error), string shell commands, or nothing (deps-only target). Any other type panics with a clear error message. Empty strings panic.

## REQ-2: Struct Tags Produce CLI Arguments

Traces to: UC-2

Struct fields annotated with `targ:"..."` tags become CLI flags or positional arguments. Supported types: bool, int, int64, float64, string, time.Duration, []T, map[K]V, Interleaved[T], embedded structs. Tags support: `flag`, `positional`, `name`, `short`, `desc`, `default`, `required`, `enum`, `env`. Unknown tag keys produce an error. Short flags must be single characters.

## REQ-3: Conflicting Overrides Error

Traces to: UC-5

When a target has a builder setting (e.g., `.Cache("**/*.go")`) and the user passes the corresponding CLI flag (e.g., `--cache=*.go`), execution returns an error. To allow CLI override, the target must use `targ.Disabled` for that setting. This prevents silent precedence ambiguity.

## REQ-4: Duplicate Names Detected

Traces to: UC-1

Two targets with the same name from different source packages produce an error listing both sources and suggesting `DeregisterFrom`. Same name from same source is allowed (idempotent registration). Group names checked independently from target names within same scope.

## REQ-5: Help Output Contains Required Sections

Traces to: UC-6

Help output includes: description, source file:line, usage line with positional placeholders, flag descriptions with short aliases and defaults, subcommand list, execution settings (deps, cache, watch), and examples. Sections appear in fixed order. Empty sections omitted. Enum values shown inline (short) or wrapped (long).

## REQ-6: Completion Supports Three Shells

Traces to: UC-7

Shell completion scripts generated for bash, zsh, and fish. Unsupported shells produce an error. Completion suggests subcommands, flags, and enum values. Completion never executes targets.

## REQ-7: Dependencies Execute Before Target

Traces to: UC-4

Dependencies declared with `.Deps()` execute before the target function. Serial by default. Parallel with `DepModeParallel`. Chained `.Deps()` calls create ordered groups. Parallel failures collected into `MultiError`.

## REQ-8: Cache Skips Unchanged

Traces to: UC-4

Targets with `.Cache(patterns...)` skip execution when matched files are unchanged since last run. Checksum-based comparison. Cache directory configurable with `.CacheDir()`.

## REQ-9: Watch Re-runs on Change

Traces to: UC-4

Targets with `.Watch(patterns...)` re-run when matched files change. Context-aware — cancellation stops watching.

## REQ-10: Timeout Terminates Execution

Traces to: UC-4

`.Timeout(duration)` enforces maximum execution time. Exceeded timeout cancels the context and kills the process tree.

## REQ-11: Retry Continues on Failure

Traces to: UC-4

`.Times(n)` runs target n times. `.Retry()` continues despite failures. `.Backoff(initial, factor)` adds exponential delay between attempts. `.While(fn)` loops while predicate returns true.

## REQ-12: Shell Commands Substitute Variables

Traces to: UC-1

String targets like `"go test $package"` extract `$variable` references as CLI arguments. Variables become flags/positionals based on context.

## REQ-13: Groups Create Hierarchies

Traces to: UC-3

`Group(name, members...)` creates nested command namespaces. Group names must be valid (non-empty, lowercase, no leading digit/dash, no spaces). Glob patterns (`*`, `**`) select across groups. Caret (`^`) resets to parent. `targ group subcommand` invocation.

## REQ-14: Binary Mode Hides Targ Flags

Traces to: UC-8

When using `Main()` (binary mode), targ-specific flags (--source, --no-cache, --keep) are hidden from help and completion. "Global Flags" label becomes "Flags". Examples use binary name instead of "targ".

## REQ-15: Scaffolding Generates Valid Code

Traces to: UC-9

`--create` generates syntactically valid Go code with proper imports, `init()` function, and `targ.Register()` call. Kebab-case names converted to PascalCase. Supports `--deps`, `--cache`, `--watch`, `--timeout`, `--times`, `--retry`, `--backoff`, `--dep-mode` flags.

## REQ-16: Remote Import Adds Boilerplate

Traces to: UC-9

`--sync PACKAGE` adds a blank import triggering the remote package's `init()` and a `DeregisterFrom` call preventing name conflicts by default. Re-running updates the module version.

## REQ-17: Builder Methods Are Chainable

Traces to: UC-1, UC-4

All builder methods on `*Target` return `*Target`, enabling fluent chaining: `Targ(fn).Name("x").Deps(a).Cache("**/*.go").Timeout(5*time.Minute)`.

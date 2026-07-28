# Execution Engine Specification

## Purpose

This bounded context covers running a resolved target or dependency graph serially or in
parallel, enforcing timeouts/retries/cache/watch re-invocation, and reporting outcomes. Its
behavior lives in `internal/core/run_env.go`, `override.go`, the dispatch portion of
`command.go`, `result.go`, and the run loop in `target.go`.

## Requirements

### Requirement: Validation timing differs between function and shell targets
Per-target validation of required flags, environment values, and defaults MUST be deferred
past the resolve-only pre-pass for FUNCTION targets, but MUST run unconditionally — even
during the resolve-only pre-pass — for SHELL targets.

For a function target, `executeTargetWithParents` MUST return before calling
`checkRequiredFlags` or `applyDefaultsAndEnv` whenever `RunOptions.ResolveOnly` (or
`HelpOnly`) is set (`internal/core/command.go:946-949`).

For a shell target, `executeShellCommand` MUST call `validateShellVars` unconditionally
(`internal/core/command.go:1044`), before the `ResolveOnly` check that gates execution
(`internal/core/command.go:1064`). This ordering is deliberate: the code's own comment at
`internal/core/command.go:1062-1063` notes that resolve mode "validates but does not
execute," so shell-var validation is placed below the unknown-flag and explicit-chaining
checks rather than beside the early `HelpOnly` return that function targets use.

#### Scenario: Resolving a function target without executing it
- **WHEN** targ resolves a command chain ending in a function target under
  `RunOptions.ResolveOnly`
- **THEN** required-flag checks and default/environment application are skipped for that
  target, and only argument parsing and remaining-token extraction occur

#### Scenario: Resolving a shell target without executing it
- **WHEN** targ resolves a command chain ending in a shell-command target under
  `RunOptions.ResolveOnly`
- **THEN** `validateShellVars` still runs and can fail the resolve pass, even though the
  shell command itself is not executed

### Requirement: No collision detection between args-struct fields and targ-managed flags
targ SHALL NOT detect or error on a collision between a target's args-struct field and a
targ-managed runtime flag (`--watch`, `--cache`, `--retry`, `--times`, `--backoff`,
`--dep-mode`, `--while`, and related flags). The collision is silent: the CLI flag is
consumed as a runtime override and the args-struct field is left at its zero value, with no
error or warning in either direction.

`ExtractOverrides` (`internal/core/override.go:102-146`, invoked from
`internal/core/run_env.go:316`) strips targ-managed flags out of the argument list globally,
before any target-specific flag parsing occurs, regardless of whether the target's args
struct also declares a field of the same name. `checkConflicts`
(`internal/core/override.go:241-259`) only compares CLI overrides against the Target
builder's compile-time configuration (`.Watch()`, `.Cache()`, `.Deps()`); it never inspects
args-struct field names, and no other code path performs that comparison.

As a consequence: a target with an args field named `Watch` invoked with `--watch <pattern>`
leaves that field at its zero value while putting the CLI into watch mode; a target with a
`Cache` field invoked with `--cache <pattern>` leaves that field at its zero value while
enabling cache-skip behavior on subsequent runs (the target function is not invoked once a
cache hit is recorded); a target with a `Retry` field invoked with `--retry` leaves that
field false while the CLI retries the target on failure. This holds for any args-struct field
whose name matches a targ-managed flag.

#### Scenario: An args-struct field name collides with a targ-managed flag
- **WHEN** a target's args struct declares a field (e.g. `Watch []string`, `Cache []string`,
  or `Retry bool`) whose name corresponds to a targ-managed runtime flag, and the target is
  invoked with the corresponding CLI flag (e.g. `--watch <pattern>`, `--cache <pattern>`, or
  `--retry`)
- **THEN** targ applies the flag as a runtime override (entering watch mode, recording a
  cache checksum, or enabling retry) and leaves the args-struct field at its zero value,
  without producing any error or warning about the name collision

### Requirement: Chain resolution is always root-relative; `--` is not a reset operator
Once a subcommand chain enters a group and reaches a target, any leftover words in the chain
MUST be resolved against the top-level root list, never against the group the previous target
was found in. `--` SHALL NOT act as an operator that resets chain resolution to the root,
because chain resolution is already unconditionally root-relative.

`executeGroupWithParents` (`internal/core/command.go:970-1021`) consumes exactly one
subcommand token per group entry and returns the remainder unconsumed. That remainder
propagates back up to `walkRoots` (`internal/core/run_env.go:743-798`), whose loop resolves
each leftover leading token only via `findMatchingRoot`
(`internal/core/run_env.go:328-336`) — a lookup over the top-level root list, with no
awareness of which group the prior token resolved inside. A leftover word that names a
sibling target inside the same group, rather than a top-level root, MUST fail as an unknown
command, and — because the whole chain is resolved before anything executes — no target in
that chain runs, including ones that already resolved successfully earlier in the chain.

Because resolution is already root-relative regardless of `--`, an invocation with `--`
inserted before a top-level target name MUST behave identically to the same invocation
without it. `--` SHALL retain meaning only in two other, unrelated contexts: terminating a
variadic positional argument (`internal/core/parse.go:48-52`) and terminating a `--deps`
variadic flag list (`internal/core/override.go:392-406`).

#### Scenario: A leftover chain word names a sibling target in the same group
- **WHEN** a group contains two targets (e.g. `build` and `test`) and is invoked as
  `targ grp build test`
- **THEN** `build` resolves inside `grp`, but the leftover word `test` is looked up against
  the top-level root list rather than against `grp`'s subcommands, resolution fails with an
  unknown-command error, and neither `build` nor `test` executes

#### Scenario: `--` before a top-level target name changes nothing
- **WHEN** a chain such as `targ grp build prod` (where `prod` is a top-level root) is
  compared against the same chain with `--` inserted before `prod`, i.e.
  `targ grp build -- prod`
- **THEN** both invocations resolve and execute identically, because `--` does not alter how
  the leftover word `prod` is resolved

#### Scenario: `--` terminating a variadic positional or `--deps` list
- **WHEN** `--` appears while parsing a variadic positional argument or a `--deps` flag's
  argument list
- **THEN** it ends that variadic sequence, and this is the only functional effect `--` has
  within targ's argument handling (variadic-positional termination itself is
  `argument-binding`'s concern, not this context's — this scenario is cited here only as
  supporting evidence that `--` is not a root-reset operator; look in
  `openspec/specs/argument-binding/spec.md` for changes to variadic termination itself)

### Requirement: `**` does not recurse below an entered group
A `**` glob pattern used as a subcommand inside an already-entered group MUST match only that
group's direct subcommands; it MUST NOT recurse into nested groups beneath it. Recursive
expansion of `**` is available only when the pattern is evaluated at the root level, before
any group has been entered.

Root-level `**` resolution goes through `findMatchingRootsGlob`
(`internal/core/run_env.go:339-355`), which — for a pattern with a `**` prefix — calls
`expandRecursive` (`internal/core/run_env.go:902-919`). `expandRecursive` walks
`node.Subcommands` and recurses into each one, so it collects matches at every depth.

Inside an already-entered group, `**` is instead handled by `executeGroupWithParents`
(`internal/core/command.go:990-991`) via `findMatchingSubcommands`
(`internal/core/command.go:1195-1206`), which iterates only `node.Subcommands` for the
current node and does not recurse into any subcommand's own `Subcommands`. A target nested
two or more levels below the entered group MUST NOT be matched by a `**` issued at that
group's level.

#### Scenario: Root-level `**` matches recursively
- **WHEN** targ is invoked as `targ **` and the command tree has a group `grp` containing a
  direct target and a nested group with its own target, alongside other top-level roots
- **THEN** all matching targets run, including ones nested multiple levels deep under `grp`
  and any other top-level roots

#### Scenario: Group-level `**` matches only direct subcommands
- **WHEN** targ is invoked as `targ grp **` and `grp` contains a direct target plus a nested
  group `sub` containing its own target
- **THEN** only `grp`'s direct target runs; the target nested inside `sub` is silently
  skipped because `findMatchingSubcommands` does not descend into `sub`'s subcommands

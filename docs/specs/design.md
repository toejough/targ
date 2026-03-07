# L2: Design

Bottom-up derivation from architecture and existing interaction model.

## DES-1: Three-Stage Progression

Traces to: UC-1, UC-8

Stage 1: String command targets with `//go:build targ` tag, `init()` + `Register()`, run via `targ <command>`. Stage 2: Function targets with struct parameters, same registration model. Stage 3: Remove build tag, switch to `package main`, use `Main()` instead of `Register()`. Same target definitions work across all stages.

## DES-2: Conflict Detection Model

Traces to: UC-5

Three configuration layers: target definition (builder methods), CLI flags, global defaults. When target definition and CLI flag both set the same setting, targ errors with a clear message. To allow CLI override, use `targ.Disabled` in the target definition. This is explicit opt-in, not implicit precedence.

## DES-3: Struct Tag Syntax

Traces to: UC-2

Format: `targ:"kind,key=value,..."`. Kind: `flag` (default) or `positional`. Keys: `name=X` (custom name), `short=X` (single-char alias), `desc=...` (help text), `default=X` (default value), `required` (must be provided), `enum=a|b|c` (allowed values), `env=VAR` (env var fallback). Combine with commas. Embedded structs inherit fields.

## DES-4: Help Layout

Traces to: UC-6

Fixed section order: Description → Source (file:line) → Usage → Positionals → Flags (global before command) → Subcommands → Execution (deps, cache, watch) → Examples. ANSI styling via lipgloss. Empty sections omitted. No trailing whitespace. Example code blocks ANSI-free.

## DES-5: Dependency Chaining Model

Traces to: UC-4

`.Deps(a)` = serial group with `a`. `.Deps(a, b, Parallel)` = parallel group with `a, b`. Chained calls create ordered groups: `.Deps(gen).Deps(lint, test, Parallel).Deps(deploy)` = gen → lint||test → deploy. Same-mode adjacent groups coalesce. `GetDeps()` flattens all groups. Deps-only targets: `Targ().Name("all").Deps(build, test)`.

## DES-6: CLI Override Flags

Traces to: UC-5

Override flags: `--timeout=DURATION`, `--times=N`, `--retry`, `--backoff=INITIAL,FACTOR`, `--while=PREDICATE`, `--cache=PATTERNS`, `--watch=PATTERNS`, `--dep-mode=serial|parallel`, `--cache-dir=DIR`, `--deps=TARGETS`. Each requires `targ.Disabled` on target or absence of target setting.

## DES-7: Glob-Based Group Selection

Traces to: UC-3

Command path resolution supports glob patterns: `targ dev *` matches all in dev group. `targ **` matches recursively. Exact match preferred over glob. Namespaced nodes: `targ dev lint fast`. Caret (`^`) resets to parent group.

## DES-8: Shell Command Variable Model

Traces to: UC-1

String targets like `"go test -race $package"` parse `$variable` tokens. Each variable becomes a CLI flag with the variable name. Long flags (`--flag`), equals syntax (`--flag=val`), and short flags (`-f`) supported. Missing variables error. Unknown flags error.

## DES-9: Target Scaffolding Model

Traces to: UC-9

`--create NAME` generates a function target stub. `--create NAME "cmd"` generates a string command target. Generated code: `//go:build targ` header, `package dev`, targ import, `init()` with `targ.Register()`. If targ file exists, appends to existing `Register()` call. If no targ file, creates new one. Supports builder flags: `--deps`, `--cache`, `--watch`, `--timeout`, `--times`, `--retry`, `--backoff`, `--dep-mode`.

## DES-10: Remote Target Import Model

Traces to: UC-9

`--sync PACKAGE` adds: blank import `_ "package"` (triggers remote init/Register), `targ.DeregisterFrom("package")` call (prevents conflicts by default). User edits to use targets: remove DeregisterFrom (use all), selective re-register, or rename with `.Name()`.

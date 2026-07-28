# Target Model Specification

## Purpose

This bounded context covers declaring a runnable target or named group and configuring
its dependencies, caching, watch, retry/backoff, and timeout. Its behavior lives in
`internal/core/target.go` (the `Target` struct, its builder methods, and `Targ()`) and
`internal/core/group.go` (the `TargetGroup` type and `Group()` constructor).

## Requirements

### Requirement: Deps silently discards arguments of unsupported types
`Target.Deps` (`internal/core/target.go:133-150`) SHALL accept a variadic list of `any` and
match each argument with a type switch against exactly `*Target`, `DepMode`, and `DepOption`.
This switch SHALL have no `default` case, so any argument of another type — including a raw
`func` — MUST be silently discarded: `Deps` MUST NOT return an error, MUST NOT panic, and
MUST NOT emit a warning when an argument matches none of the supported types.

This SHALL contrast with `targ.Register()` (`internal/core/command.go:1637-1659`), which
SHALL accept a raw function directly — `parseTarget` dispatches any argument whose
`reflect.Value.Kind()` is `reflect.Func` to `parseFunc`. The two entry points therefore
differ in what argument types they accept for attaching a function to targ's execution
model: `Register()` takes a raw function, `Deps()` silently drops one.

#### Scenario: A raw func passed to Deps
- **WHEN** a target's `.Deps()` call includes a plain `func()` argument (instead of, or
  alongside, a `*Target`)
- **THEN** the `func()` argument is silently discarded — it is never added to the target's
  dependency groups, the target run completes with `err == nil` and exit code `0`, and no
  warning is printed anywhere in the output

#### Scenario: The same raw function passed to Register instead
- **WHEN** the same raw `func()` value is passed to `targ.Register()` rather than to
  `.Deps()`
- **THEN** `Register()` accepts it and registers it for execution, unlike the identical
  value silently dropped by `.Deps()`

### Requirement: Group panics on a member of an unsupported type
`Group()` (`internal/core/group.go:40-63`) SHALL validate each variadic member against a
type switch matching only `*Target` and `*TargetGroup`. Any member whose type matches
neither MUST cause `Group()` to panic immediately, via an explicit `default` case, rather
than returning an error or silently omitting the member from the resulting `TargetGroup`.
A malformed group SHALL therefore abort the calling process rather than degrade gracefully.

#### Scenario: An unsupported member type passed to Group
- **WHEN** `Group(name, members...)` is called with a member whose runtime type is not
  `*Target` or `*TargetGroup`
- **THEN** `Group()` panics with a message naming the member's index and type, aborting
  the process instead of returning an error or ignoring the member

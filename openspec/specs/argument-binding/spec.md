# Argument Binding Specification

## Purpose

This bounded context covers turning a target's function or struct signature and its
`targ:"..."` tags into flag and positional metadata, binding argv values onto that
metadata, and predicting shell-completion candidates. Its behavior lives in
`internal/core/parse.go`, `internal/core/completion.go`, the tag helpers in
`internal/core/command.go`, and `internal/flags`.

## Requirements

### Requirement: TagOptions overrides are resolved by reflective method lookup, not interface satisfaction
`TagOptions` (`internal/core/types.go:105`) SHALL be a plain struct type, not an interface —
no `TagOptions` interface exists anywhere in targ. A struct field's tag options SHALL be
overridden by reflectively locating a method literally named `TagOptions` on the owning
instance, via `target.MethodByName("TagOptions")` (`internal/core/command.go:344`), rather
than by asserting the instance against an interface type. A type therefore participates in
the override mechanism purely by declaring a method with that exact name and the expected
signature `func(string, TagOptions) (TagOptions, error)` — nothing enforces this at compile
time.

When no method named `TagOptions` exists on the instance (for example because of a
misspelled method name, or because the intended method resolved onto the wrong receiver),
`MethodByName` MUST return an invalid `reflect.Value` and `applyTagOptionsOverride` MUST
return the original, unmodified `TagOptions` with a nil error: no override is applied and
no diagnostic of any kind is produced. When a method named `TagOptions` does exist but has
the wrong signature, `validateTagOptionsSignature` MUST instead return a non-nil error that
`applyTagOptionsOverride` propagates to its caller — this mistake IS surfaced, but only at
runtime, since no compile-time interface check exists to catch it earlier.

#### Scenario: A type with a correctly named and signed TagOptions method
- **WHEN** a struct field's owning instance has a method
  `TagOptions(name string, opts TagOptions) (TagOptions, error)`
- **THEN** `applyTagOptionsOverride` locates it via `MethodByName("TagOptions")` and uses
  its returned `TagOptions` value, without ever checking the instance against an interface
  type

#### Scenario: A type with no TagOptions method, such as a misspelled name
- **WHEN** a struct field's owning instance has no method literally named `TagOptions`
- **THEN** `MethodByName("TagOptions")` returns an invalid `reflect.Value`,
  `applyTagOptionsOverride` returns the original tag options unchanged with a nil error,
  and no override is applied — the mismatch produces no error, warning, or other
  diagnostic

#### Scenario: A TagOptions method with the wrong signature
- **WHEN** a struct field's owning instance declares a method named `TagOptions` whose
  signature does not match `func(string, TagOptions) (TagOptions, error)`
- **THEN** `validateTagOptionsSignature` returns a non-nil error, which
  `applyTagOptionsOverride` propagates to its caller — the mistake surfaces only as a
  runtime error, since nothing catches it at compile time

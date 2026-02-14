# Help System Overhaul

## Problem

The help system is fragmented and inconsistently applied. Binary mode (compiled Go programs using `targ.Main()`) incorrectly shows targ-specific flags, uses "targ" in labels and examples, and there's no mechanism to prevent new flags from accidentally appearing in binary help. Examples are hardcoded rather than derived from actual command metadata.

## Solution

Three changes: (1) tag-based flag mode classification that's enforced by tests, (2) proper binary mode propagation from `targ.Main()`, and (3) auto-generated examples from command metadata.

## 1. Flag Mode Tags

Every targ flag in `internal/flags/flags.go` gets an explicit `Mode` field:

```go
type FlagMode int

const (
    FlagModeAll      FlagMode = iota // Both targ CLI and binary
    FlagModeTargOnly                 // Only targ CLI
)
```

Classification:

| Mode | Flags |
|------|-------|
| `FlagModeAll` | `--help`, `--completion` |
| `FlagModeTargOnly` | `--source`, `--timeout`, `--parallel`, `--times`, `--retry`, `--backoff`, `--watch`, `--cache`, `--while`, `--dep-mode`, `--no-binary-cache`, `--create`, `--sync`, `--to-func`, `--to-string`, `--no-cache`, `--init`, `--alias`, `--move` |

A test enforces every `flags.Def` entry has been consciously classified. Adding a new flag without setting `Mode` fails the test.

`shouldSkipTargFlag` in `builder.go` uses `Mode` instead of the current ad-hoc allowlist:

```go
if filter.BinaryMode && f.Mode != FlagModeAll {
    return true
}
```

## 2. Binary Mode Propagation

`targ.Main()` in `internal/core/execute.go` sets `BinaryMode = true` on `RunOptions`. This flows to `TargFlagFilter.BinaryMode` when building help output in `command.go`. The field already exists on `TargFlagFilter` and the conditional logic in `generators.go` already checks it — nobody sets it today.

## 3. Help Labels

| Context | Usage line | Flag section header |
|---------|-----------|-------------------|
| Targ CLI root | `targ [global flags...] <command>...` | "Global flags:" |
| Targ CLI target | `targ [global flags...] cmd [flags...]` | "Global flags:" + "Flags:" |
| Binary root | `mybin [flags...] <command>...` | "Flags:" |
| Binary target | `mybin cmd [flags...]` | "Flags:" |

"Targ flags:" label replaced with "Global flags:" in targ CLI mode. In binary mode, just "Flags:".

## 4. Auto-Generated Examples

A new function generates examples deterministically from command metadata (flags, positional args, subcommands). Replaces all hardcoded examples.

### Root help (targ CLI)

```
Examples:
  Run a command:            targ build
  Chain commands:           targ build test
  With timeout:             targ build --timeout 30s
```

Uses actual command names from the registry. Shows chaining only if 2+ commands exist. Shows one global flag example.

### Root help (binary)

```
Examples:
  Run a command:            mybin greet
```

Uses actual binary name and first subcommand. If no subcommands, shows basic invocation. No targ-specific examples.

### Target help (either mode)

```
Examples:
  Basic usage:              targ deploy config.yaml
  With options:             targ deploy config.yaml --env production --port 8080
```

- "Basic usage:" includes required positional args
- "With options:" adds one or two optional flags (if any exist)
- Omit "with options" if no optional flags exist
- Omit examples entirely if the target has no args at all (just shows `targ deploy`)

### Type-aware example values

| Type | Example value |
|------|--------------|
| `string` | Use placeholder text (e.g., `VALUE`, `PATH`) |
| `int` | `10` |
| `float64` | `0.5` |
| `bool` | Flag name only (no value) |
| `duration` | `30s` |

### User override

If `.Examples()` is called on a target, those replace auto-generated examples entirely. No merging.

## Affected Files

| File | Change |
|------|--------|
| `internal/flags/flags.go` | Add `FlagMode` type and field to `Def`, classify all flags |
| `internal/flags/flags_test.go` | Test: every flag has explicit mode set |
| `internal/help/builder.go` | Use `FlagMode` in filter, update section labels |
| `internal/help/generators.go` | Auto-example generator, label changes, remove hardcoded examples |
| `internal/help/generators_test.go` | Tests for auto-examples and mode-dependent output |
| `internal/core/execute.go` | Set `BinaryMode = true` in `Main()` |
| `internal/core/command.go` | Pass `BinaryMode` to `TargFlagFilter`, pass metadata for examples |
| `internal/core/types.go` | Add `BinaryMode` to `RunOptions` if not already there |
| Existing help tests | Update expectations for new labels and examples |

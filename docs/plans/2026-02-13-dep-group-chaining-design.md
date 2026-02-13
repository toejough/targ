# Dep Group Chaining

## Problem

`Deps()` uses a single `depMode` field — calling it twice overwrites the mode, silently downgrading parallel deps to serial (or vice versa). Users cannot express "run A serially, then B and C in parallel, then D serially."

## Solution

Replace the flat `deps []*Target` + `depMode DepMode` with an ordered slice of `depGroup` structs. Each group has its own mode. Groups execute sequentially; targets within a group execute according to the group's mode.

## API

`Deps()` determines mode from args (serial default, parallel if `targ.Parallel` / `targ.DepModeParallel` is passed). If the last existing group has the same mode, targets coalesce into it. Otherwise a new group is appended.

```go
// Single group (unchanged behavior):
t.Deps(a, b)                             // serial [a, b]
t.Deps(a, b, targ.DepModeParallel)       // parallel [a, b]

// Coalescing — back-to-back same mode merges:
t.Deps(a).Deps(b)                        // serial [a, b]
t.Deps(a, Parallel).Deps(b, Parallel)    // parallel [a, b]

// Chaining — different modes create separate groups:
t.Deps(generate).
  Deps(lint, test, Parallel).
  Deps(deploy)
// Execution: generate → (lint ∥ test) → deploy
```

## Data Model

```go
type depGroup struct {
    targets []*Target
    mode    DepMode
}

// Target struct changes:
// - deps []*Target      → removed
// - depMode DepMode     → removed
// + depGroups []depGroup → added
```

## Execution

`runDeps` iterates groups in order. Each group completes fully before the next starts. Within a parallel group, first error cancels remaining and returns immediately (existing behavior preserved).

## Accessors

| Method | Behavior |
|--------|----------|
| `GetDeps()` | Returns all targets flattened across groups (backward compatible) |
| `GetDepMode()` | Returns the mode if all groups share one; `DepModeMixed` if mixed |
| `GetDepGroups()` | New — returns `[]DepGroup` (exported view) for full structure |

## Help Display

Current: `Deps: lint, test (parallel)`

With chaining: `Deps: generate → lint, test (parallel) → deploy`

Serial groups list targets comma-separated (no mode annotation since serial is default). Parallel groups append `(parallel)`. Groups separated by ` → `.

Single-mode targets display unchanged from current behavior.

## `--dep-mode` Override

Currently parsed but not applied at runtime. With this change, `--dep-mode serial` flattens all groups into one serial group. `--dep-mode parallel` flattens into one parallel group.

## Affected Files

| File | Change |
|------|--------|
| `internal/core/target.go` | `depGroup` struct, rewrite `Deps()`, `runDeps()`, `GetDepMode()`, add `GetDepGroups()` |
| `internal/core/types.go` | Update `TargetExecutionLike` interface, add `DepModeMixed` |
| `internal/core/command.go` | Update `commandNode` display, apply `--dep-mode` override at runtime |
| `targ.go` | Export `DepModeMixed`, `DepGroup` type |
| `internal/runner/runner.go` | Code gen for chained deps |
| Tests | New chaining/coalescing tests, update existing dep mode tests |

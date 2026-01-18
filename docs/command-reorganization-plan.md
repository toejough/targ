# Command Reorganization Plan

## Problem Statement

Users want to reorganize targ commands into hierarchies (e.g., move `targets.lint`, `targets.lint-fast`, `targets.lint-for-fail` under a single `lint` parent command).

The challenge: original functions may be called programmatically by other code. Simply renaming or wrapping them breaks those call sites.

## Current Approach (Failed)

The initial `--move` implementation:
1. Renamed functions to unexported (`Lint` → `lint`)
2. Generated wrapper structs calling unexported versions
3. **Problem:** Did NOT update call sites like `Check()` which still called `Lint(ctx)`

Result: Compilation errors because `Lint` is now a struct, not a function.

## Proposed Solution

### Core Insight

To avoid breaking call sites:
1. Rename original functions to unexported (`Lint` → `lint`)
2. **Update ALL call sites** in the same package (`Lint(ctx)` → `lint(ctx)`)
3. Generate new struct with the exported name

### AST Work Required

```go
// Before
func Lint(ctx context.Context) error { ... }
func Check(ctx context.Context) error {
    return targ.Deps(func() error { return Lint(ctx) })  // call site
}

// After
func lint(ctx context.Context) error { ... }
func Check(ctx context.Context) error {
    return targ.Deps(func() error { return lint(ctx) })  // updated automatically
}
type Lint struct { ... }
func (l *Lint) Run(ctx context.Context) error { return lint(ctx) }
```

To update call sites:
1. Parse all `.go` files in the same package
2. Find all `*ast.Ident` nodes matching function names being renamed
3. In call expressions or function references, replace with unexported name
4. Write back the modified AST

### Scope Limitation

- **Same-package calls:** Straightforward AST replacement
- **Cross-package calls:** Much more complex (import analysis needed). Warn user instead.

## Proposed Commands

### `--rename OLD_PATH NEW_PATH`

Single command rename/move. No globbing. Full paths only.

```bash
targ --rename targets.lint lint
targ --rename targets.lint-fast targets.fast
```

### `--nest DEST SOURCE_PATTERN`

Create subcommand hierarchy. Uses globbing for source pattern.

```bash
targ --nest lint targets.lint*
```

Creates:
- `lint` (parent) → calls `lint()`
- `lint fast` → calls `lintFast()`
- `lint for-fail` → calls `lintForFail()`

## Implementation Plan

### Approach: Self-Contained Rename Using go/packages + go/types

The `golang.org/x/tools/refactor/rename` package is obsolete and doesn't work with Go modules.
gopls works but requires external dependency.

Instead, implement our own rename logic:
- Use `golang.org/x/tools/go/packages` to load packages with full type info
- Use `go/types` Info.Uses and Info.Defs to find all references to a symbol
- Replace identifiers in AST
- Write back modified files

This keeps targ self-contained with no external tool dependencies.

### Phase 1: Rename Infrastructure

Implement `renameFunction(modulePath, funcName, newName string) error`:

```go
// 1. Load all packages in module with type info
cfg := &packages.Config{
    Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax | packages.NeedFiles,
    Dir:  modulePath,
}
pkgs, err := packages.Load(cfg, "./...")

// 2. Find the function's types.Object (the definition)
// 3. For each package, find all Uses of that object in TypesInfo.Uses
// 4. Collect file positions of all references
// 5. For each file with references:
//    - Parse AST (or use already-loaded syntax)
//    - Replace identifier names at those positions
//    - Write back using go/format
```

### Phase 2: `--rename` Command

1. Discover commands, find the target function
2. Validate destination doesn't conflict
3. Call `renameFunction` to rename to unexported (e.g., `Lint` → `lint`)
4. Generate wrapper struct with exported name at destination

### Phase 3: `--nest` Command

1. Parse source pattern, find all matching functions
2. Validate destination doesn't conflict
3. For each function, call `renameFunction` to rename to unexported
4. Generate parent struct with subcommand wrappers

## Open Questions

1. What if destination already exists? (Error vs. merge as subcommands)
2. Should we support nested destinations like `tools.lint`? (Start with top-level only)

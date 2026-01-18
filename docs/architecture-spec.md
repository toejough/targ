# Architecture Specification

How targ implements the problem specification.

## Overview

Targ is a build tool framework with two modes:
- **Build tool mode**: Discovers and runs targets from tagged Go files
- **Direct mode**: Library for building standalone CLI applications

This document focuses on build tool mode, which is the primary use case.

## Layers

```
┌─────────────────────────────────────────────────┐
│                   CLI Surface                   │
│         (targ command, flags, dispatch)         │
├─────────────────────────────────────────────────┤
│                  Transformation                 │
│        (rename, relocate, delete targets)       │
├─────────────────────────────────────────────────┤
│                 Code Generation                 │
│      (bootstrap main.go, wrapper structs)       │
├─────────────────────────────────────────────────┤
│                    Discovery                    │
│    (find targets, parse AST, build hierarchy)   │
├─────────────────────────────────────────────────┤
│                    Execution                    │
│   (run targets, dependencies, parallel/serial)  │
├─────────────────────────────────────────────────┤
│               Target Representation             │
│        (functions, structs, capabilities)       │
└─────────────────────────────────────────────────┘
```

## Target Representation

How users define targets in code.

### Progressive Enhancement Model

Targets start as simple functions and gain capabilities by changing their signature or switching to struct form:

| Form | Capabilities Enabled |
|------|---------------------|
| `func Name()` | Basic |
| `func Name() error` | + Failure |
| `func Name(ctx context.Context) error` | + Cancellation |
| `type Name struct{}` + `Run()` | + Arguments, Subcommands, Help text |

### Function Targets

```go
//go:build targ

package dev

// Simplest target
func Build() {}

// With failure indication
func Test() error { return sh.Run("go", "test", "./...") }

// With cancellation support
func Watch(ctx context.Context) error { ... }
```

### Struct Targets

```go
type Deploy struct {
    Env     string `targ:"flag,required,desc=Target environment"`
    DryRun  bool   `targ:"flag,short=n,desc=Preview without executing"`
    Targets []string `targ:"positional,desc=Services to deploy"`
}

func (d *Deploy) Run(ctx context.Context) error { ... }
func (d *Deploy) Description() string { return "Deploy services to environment" }
```

### Capability APIs

Capabilities are accessed through the `targ` package:

| Capability | API |
|------------|-----|
| Dependencies | `targ.Deps(A, B, C)` |
| Parallel | `targ.Deps(A, B, targ.Parallel())` |
| Serial | `targ.Deps(A, B)` (default) |
| Result caching | `file.Newer(inputs, outputs)` |
| Watch mode | `file.Watch(ctx, patterns, opts, callback)` |

### Subcommand Hierarchy

Struct fields with `targ:"subcommand"` create nested commands:

```go
type Lint struct {
    Fast    *LintFast    `targ:"subcommand"`
    ForFail *LintForFail `targ:"subcommand"`
}

func (l *Lint) Run(ctx context.Context) error { ... }
```

Results in: `targ lint`, `targ lint fast`, `targ lint for-fail`

## Discovery

How targ finds targets.

### Build Tag Filtering

Only files with `//go:build targ` are considered:

```go
//go:build targ

package dev
```

### Discovery Process

1. Walk directory tree from current working directory
2. Find all `.go` files with `//go:build targ`
3. Parse AST of each file
4. Extract exported functions and structs with `Run()` methods
5. Filter out types that are subcommands of other types
6. Build hierarchy from file paths and struct relationships

### File-Based Namespacing

Directory structure creates command namespaces:

```
dev/
├── targets.go      → targ build, targ test
└── db/
    └── targets.go  → targ db migrate, targ db seed
```

### Hierarchy Resolution

1. **File path** determines namespace prefix
2. **Struct subcommands** create nested hierarchy
3. **Common prefix collapse** simplifies single-child directories

## Code Generation

How targ creates the executable.

### Bootstrap Generation

Targ generates a temporary `main.go` that:
1. Imports all packages containing targets
2. Creates wrapper structs for function targets
3. Wires up the command tree
4. Calls `targ.Run()`

```go
// Generated bootstrap (simplified)
package main

import (
    "github.com/toejough/targ"
    "myproject/dev"
)

type BuildCommand struct{}
func (c *BuildCommand) Run() error { return dev.Build() }
func (c *BuildCommand) Name() string { return "build" }

func main() {
    targ.Run(&BuildCommand{}, ...)
}
```

### Binary Caching

Compiled binaries are cached in `.targ/cache/` with content-based keys:
- Key = hash of (source files + go.mod + go.sum)
- Rebuild only when inputs change
- `--no-cache` forces rebuild

## Execution

How targets run.

### Invocation Flow

```
CLI args → Parse early flags → Discovery → Generate bootstrap → Compile → Execute
                    ↓
            (--move, --rename, etc. exit early)
```

### Dependency Execution

`targ.Deps()` ensures each target runs exactly once per CLI execution:

```go
func Build(ctx context.Context) error {
    return targ.Deps(Generate, Compile)  // Generate runs once even if called elsewhere
}
```

### Execution Modes

| Mode | Behavior |
|------|----------|
| Serial (default) | Run in order, fail-fast |
| Serial + ContinueOnError | Run all, collect errors |
| Parallel | Run concurrently, fail-fast |
| Parallel + ContinueOnError | Run all concurrently, collect errors |

### Context Propagation

- Root context created at CLI entry
- Cancelled on SIGINT/SIGTERM
- `--timeout` wraps with deadline
- Passed to `Run(ctx)` methods

## Transformation

How targ modifies target code.

### Primitive Operations

| Operation | Implementation |
|-----------|---------------|
| Rename symbol | `go/packages` + `go/types` to find all references, AST rewrite |
| Add code | Append to file with `go/format` |
| Remove code | AST deletion + cleanup |
| Move code | Copy to destination + remove from source |

### Rename (--rename)

Unified command for all path changes:

```bash
targ --rename OLD_PATH NEW_PATH
```

Implementation:
1. Parse paths (dotted syntax: `targets.lint` → namespace=targets, name=lint)
2. Discover current targets
3. Find target at OLD_PATH
4. Validate NEW_PATH doesn't conflict
5. Rename underlying function to unexported (if creating wrapper)
6. Update all same-package call sites
7. Generate wrapper struct at new location (if hierarchy changes)
8. Clean up empty namespaces

### Symbol Renaming

Using `go/packages` + `go/types`:

```go
// Load with type info
cfg := &packages.Config{
    Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
    BuildFlags: []string{"-tags=targ"},
}
pkgs, _ := packages.Load(cfg, "./...")

// Find all uses of the symbol
for _, pkg := range pkgs {
    for ident, obj := range pkg.TypesInfo.Uses {
        if obj == targetObj {
            // ident.Name needs to be renamed
        }
    }
}
```

### Cross-Package References

Same-package call sites are updated automatically. Cross-package calls:
- Warn user about external callers
- Cannot be automatically updated (different module)

## CLI Surface

The targ command interface.

### Flag Categories

**Meta flags** (exit before discovery):
- `--version` - Print version
- `--help` - Show targ help

**Transformation flags** (exit after transform):
- `--rename OLD NEW` - Rename/move target
- `--delete TARGET` - Remove target

**Runtime flags** (affect execution):
- `--timeout DURATION` - Execution timeout
- `--completion SHELL` - Generate completion script
- `--no-cache` - Force recompile

**Pass-through**: Everything else goes to discovered commands

### Completion

Shell completion for:
- Target names (from discovery)
- Flag names (from struct tags)
- Enum values (from `enum=` tag)

Generated via `--completion bash|zsh|fish`

## File Organization

### Project Structure

```
targ/
├── targ.go              # Public API (Deps, Parallel, Run, etc.)
├── internal/
│   └── core/            # Implementation (parsing, execution, etc.)
├── cmd/targ/
│   └── main.go          # CLI entry point, discovery, generation
├── buildtool/           # Discovery and code generation
├── file/                # Watch, Newer utilities
└── sh/                  # Shell execution helpers
```

### User Project Structure

```
myproject/
├── go.mod
├── dev/
│   └── targets.go       # //go:build targ
└── .targ/
    └── cache/           # Compiled binary cache
```

## Invariant Enforcement

How transformations maintain invariants.

### Call Site Updates

When renaming `Foo` → `foo`:
1. Find `Foo` object via `go/types`
2. Find all `Uses` of that object
3. For each use in same package, rename identifier
4. Write back modified files

### Dangling Reference Prevention

Before delete:
1. Find all references to target
2. If references exist outside the target itself, fail with list
3. Only proceed if no external references

### Namespace Cleanup

After any transformation:
1. Check for empty namespace nodes (structs with only subcommand fields, no Run)
2. If subcommands removed and no Run, remove the struct
3. Recursively up the hierarchy

## Future: Create

Scaffold a target from a shell command (not yet implemented).

### Syntax

```bash
targ --create lint "golangci-lint run"
```

### Process

1. Parse command string
2. Generate function in appropriate file:
   ```go
   func Lint() error {
       return sh.Run("golangci-lint", "run")
   }
   ```
3. If file doesn't exist, create with build tag

## Future: Inspect

Query operations (not yet implemented).

### Where

```bash
targ --where lint
# → dev/targets.go:47
```

Implementation: Discovery already tracks source file/line.

### Tree

```bash
targ --tree
# targets
#   lint
#     fast
#     for-fail
#   test
#   build
```

Implementation: Walk discovered hierarchy, format as tree.

### Deps

```bash
targ --deps build
# build depends on:
#   generate
#   compile
```

Implementation: Parse AST for `targ.Deps()` calls, resolve targets.

## Future: Manage Dependencies

Modify target relationships (not yet implemented).

### Operations

```bash
targ --deps-add build test      # build now depends on test
targ --deps-remove build test   # remove dependency
targ --deps-mode build parallel # change to parallel execution
```

### Implementation

1. Find target's Run method
2. Parse `targ.Deps()` calls in body
3. Modify AST to add/remove/change
4. Write back with formatting

## Future: Relocate

Move implementation to different file (not yet implemented).

### Syntax

```bash
targ --relocate lint dev/lint.go
```

### Process

1. Find target definition
2. Copy function/struct to destination file
3. Update imports in destination
4. Remove from source
5. Path/name unchanged

## Future: Remote Targets

Architecture for remote target sync (not yet implemented).

### Tracking Metadata

```go
// .targ/sources.json
{
    "sources": [{
        "url": "github.com/org/shared-targets",
        "ref": "v1.2.0",
        "targets": ["lint", "test", "deploy"]
    }]
}
```

### Sync Process

1. Fetch remote at specified ref
2. Discover targets in remote
3. For each target in metadata:
   - If still in remote: update if changed
   - If removed from remote: remove locally
4. For new targets in remote: prompt to add
5. Update metadata

## Future: Invocation Modifiers

Architecture for CLI-level modifiers (not yet implemented).

### Syntax Options

```bash
targ --watch lint          # Re-run on changes
targ --retry=3 test        # Retry up to 3 times
targ --repeat=10 benchmark # Run 10 times
targ --until=5m soak       # Run for 5 minutes
```

### Implementation

Modifiers wrap the execution loop:
- `--watch`: Use `file.Watch()` around execution
- `--retry`: Loop with backoff on failure
- `--repeat`: Loop N times regardless of outcome
- `--until`: Loop until duration elapsed

# Targ Technical Design

This document describes the intended architecture and behavior for Targ, including direct binary mode and build tool mode, with support for struct- and function-based commands.

## Goals

- Make it trivial to define subcommands via structs and tags.
- Support niladic functions as commands for ultra-lightweight targets.
- Provide a build tool mode that auto-discovers commands with clear, predictable rules.
- Keep the runtime fast and deterministic, with clear errors for ambiguous input.

## Non-Goals

- Full Cobra parity.
- Interpreting Go AST to execute commands directly.
- Supporting arbitrary function signatures (only niladic functions).

## Core Concepts

- **Command**: a struct with a `Run()` method, or a niladic function.
- **Subcommand**: a struct field tagged with `targ:"subcommand"` (or `subcommand=name`).
- **Root**: the top-level command node(s) passed to `Run` or discovered in build tool mode.

## Behavioral Rules

### Direct Binary Mode

- `Run(...)` accepts any combination of structs and niladic functions (exported or not).
- If exactly one target is passed, it is the default command.
- If more than one target is passed, there is no default command (all are subcommands).

### Build Tool Mode

- Recursively search for directories containing files with `//go:build targ`.
- Per directory:
  - Enforce a single package name (Go rule); mixed names are an error.
  - Discover exported structs and niladic functions.
  - Filter out structs/functions whose name matches a subcommand name of another exported struct.
- Without `--multipackage`:
  - Find the first directory depth that has tagged files.
  - If multiple directories exist at that same depth, error with a list of those paths.
  - Use that single directory as the command source.
- With `--multipackage`:
  - Always insert the package name as the first subcommand.
  - Functions and structs are grouped under that package node.
- Build tool mode never has a default command (all commands are invoked by name).

## C4 Diagrams

### C4: System Context

```text
User
  |
  v
Targ (library + build tool)
  |
  v
Go Toolchain (go run/build)
  |
  v
User Project (tagged files + code)
```

### C4: Container Diagram

```text
Container: Build Tool Binary (cmd/targ)
  - discovers tagged files
  - generates bootstrap
  - invokes go toolchain

Container: Targ Library (targ)
  - command graph
  - argument parsing
  - execution + help + completion

Container: User Project
  - tagged files with structs + functions

Container: Go Toolchain
  - builds/runs the generated program
```

### C4: Component Diagram (Targ Library)

```text
Targ Library
  |-- Discovery
  |     - parseStruct
  |     - parseFunction
  |-- Command Graph
  |     - CommandNode
  |     - subcommand resolution
  |-- Parser
  |     - flags
  |     - positionals
  |-- Executor
  |     - Run dispatch
  |-- Help/Completion
        - usage
        - __complete
```

## Sequence Diagrams

### Direct Binary Mode (single target)

```text
User -> main.go: Run(MyCmd)
main.go -> targ.Run: targets=[MyCmd]
targ.Run -> parseStruct/parseFunction: build graph
targ.Run -> execute: parse args + call Run()
```

### Direct Binary Mode (multiple targets)

```text
User -> main.go: Run(Clean, Build)
targ.Run -> parseStruct/parseFunction: build graph (2 roots)
User -> binary: "build"
targ.Run -> execute root "build"
```

### Build Tool Mode (no --multipackage, single depth)

```text
User -> targ: run from repo root
targ -> discover: recursive search for tagged files
discover -> depth gate: pick first depth with tagged files
discover -> parse package dir: exported structs/functions
targ -> generate bootstrap -> go run .
bootstrap -> targ.Run: targets=[...]
```

### Build Tool Mode (--multipackage)

```text
User -> targ --multipackage
discover -> recursive search for tagged files
discover -> collect package dirs (any depth)
discover -> build package nodes (pkg -> cmds)
bootstrap -> targ.Run: targets=[pkg1, pkg2...]
User -> binary: "pkg1 build"
```

## Data Model

```text
CommandNode
  - Name
  - Type (struct) or Func (function)
  - Subcommands map[string]*CommandNode
  - RunMethod / FuncValue
  - Description
```

## Discovery Algorithms

### Direct Binary Mode

1) For each target:
   - If struct: parse fields, identify subcommands.
   - If function: wrap as CommandNode with a callable FuncValue.
   - Function descriptions are only available via generated wrappers.
2) Determine default command:
   - If exactly one root, treat it as default.
   - If multiple roots, no default.

### Build Tool Mode

1) Generate function wrapper structs for exported niladic functions (one file per package):
   - File name: `generated_targ_<pkg>.go`
   - Contains `Run`, `Name`, and optional `Description` based on function comments.
   - Uses the build tag `targ` so the wrappers are only included in build tool mode.
2) Recursively walk from start dir.
3) Collect directories containing files with `//go:build targ`.
4) Enforce per-directory package name consistency.
5) Without `--multipackage`:
   - Find minimum depth with tagged dirs.
   - If multiple dirs at that depth, error with paths.
   - Use that single directory for command discovery.
6) With `--multipackage`:
   - Discover commands per directory.
   - Create a package root node for each directory (name = package name).
7) In each package:
   - Parse exported structs and niladic functions.
   - Collect subcommand names from exported structs.
   - Filter any struct/function whose name matches a subcommand name.
   - Prefer generated `*Command` wrapper structs over same-named functions.

## Examples

### File Structure (Build Tool Mode)

```text
repo/
  mage/
    build.go      //go:build targ (package build)
    deploy.go     //go:build targ (package deploy)
  tools/
    gen/
      gen.go      //go:build targ (package gen)
```

### Commands (Build Tool Mode, --multipackage)

```text
$ targ --multipackage build lint
$ targ --multipackage deploy release
$ targ --multipackage gen generate
```

### Commands (Build Tool Mode, no --multipackage)

If `repo/mage` is the first depth with tagged files:

```text
$ targ build
$ targ deploy
```

### Struct + Function Commands

```go
//go:build targ

package build

type Build struct{}
func (b *Build) Run() {}

func Lint() {}
```

Result:
- Commands: `build` and `lint`
- With `--multipackage`: `build build` and `build lint`

## Error Messaging Guidelines

- Duplicate package names in same directory: list files + package names.
- Multiple tagged dirs at same depth without `--multipackage`: list directory paths.
- Invalid command: show available command names at that level.

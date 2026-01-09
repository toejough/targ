# Targ Feature Taxonomy

Purpose: a concise, defensible checklist of intended features and scope, organized so we can implement and test with confidence.

Legend:

- **Scope**: Core (must-have), Essential (baseline CLI), Extended (nice-to-have), Build (build-tool focus)
- **Status**: Planned | In Progress | Implemented | Deferred

---

## Command Model

| Feature | Tier | Status | Test Coverage |
| --- | --- | --- | --- |
| Root + subcommand discovery by struct graph | Core | Implemented | Single-root run; nested subcommands resolve |
| Function targets (niladic) as commands | Core | Implemented | Niladic function runs and appears in help |
| Root behavior when no subcommand selected | Core | Implemented | Root Run invoked on no subcommand |
| Subcommand naming overrides (`subcommand=` / `name=`) | Core | Implemented | Tag overrides field/type names |
| Command descriptions from docs / generated wrappers | Essential | Implemented | Struct Run doc + generated wrappers |
| Errors for invalid or missing commands | Essential | Implemented | Unknown command error surface |
| TagOptions dynamic overrides (flags/positional/subcommand) | Essential | Implemented | Override names, enums, required |

---

## Execution Modes

| Feature | Tier | Status | Test Coverage |
| --- | --- | --- | --- |
| Direct binary usage: `targ.Run(...)` | Core | Implemented | Root vs subcommand resolution |
| Build tool mode: `targ` binary discovery | Core | Implemented | Discovery + bootstrap execution |
| Single-root shorthand (no command name) | Essential | Implemented | Root handles args without name |
| Multiple root selection by name | Essential | Implemented | Multiple roots choose by name |
| Build tool mode discovery by build tag (`targ`) | Core | Implemented | Tagged file discovery |
| Recursive search + depth gating | Core | Implemented | Multiple dirs at same depth error |
| Auto namespacing by file path | Core | Implemented | Compress path segments, file-based namespaces |
| Function discovery in build tool mode | Core | Implemented | Exported niladic funcs appear |
| Filters out subcommand structs/functions | Core | Implemented | Subcommands not treated as roots |
| Build tool mode: no default command | Core | Implemented | Explicit command required |
| Build tool fallback module (no go.mod/go.sum) | Core | Implemented | Completion/build works without module |
| Local main package build (subdir) | Core | Implemented | Main packages built in-place |

---

## Argument Parsing

| Feature | Tier | Status | Test Coverage |
| --- | --- | --- | --- |
| Long flags (`--flag`) | Essential | Implemented | Flag parsing sets fields |
| Short flags (`-f`) | Essential | Implemented | Short alias works |
| Positional args | Essential | Implemented | Positionals map to fields |
| Required vs optional | Essential | Implemented | Missing required errors |
| Tag defaults (`default=`) | Essential | Implemented | Defaults applied when unset |
| Env var defaults (`env=`) | Essential | Implemented | Env applied when non-empty |
| Boolean flags | Essential | Implemented | Bool flags parse without value |
| Repeated flags | Extended | Planned | Accumulate repeated inputs |
| Variadic positionals | Extended | Planned | Slice-like positionals |
| Map-type args | Extended | Planned | `key=value` style mapping |
| Custom types via TextUnmarshaler | Extended | Implemented | TextUnmarshaler fields parse |

---

## Help, Usage, Completion

| Feature | Tier | Status | Test Coverage |
| --- | --- | --- | --- |
| `--help` root/subcommand | Essential | Implemented | Help output includes commands |
| Usage shows flags/positionals/subcommands | Essential | Implemented | Usage format stable |
| Shell completion (bash/zsh/fish) | Essential | Implemented | Script outputs completions |
| Completion for enums | Essential | Implemented | Tag enums show in completion |
| Completion respects TagOptions overrides | Essential | Implemented | Overrides propagate to completion |
| Completion with quoted/escaped args | Extended | Implemented | Quoted args parsed correctly |

---

## Execution Semantics

| Feature | Tier | Status | Test Coverage |
| --- | --- | --- | --- |
| Run signatures (niladic or context) | Essential | Implemented | Context + niladic variants |
| Error return support | Extended | Implemented | Run error propagates to exit |
| Context/cancellation support | Extended | Implemented | SIGINT cancels via context |
| Lifecycle hooks (Persistent Before/After) | Extended | Implemented | Hooks run in order |

---

## Build Tool Features

| Feature | Tier | Status | Test Coverage |
| --- | --- | --- | --- |
| Multiple commands in one invocation | Build | Planned | `targ build test` style |
| Dependencies (run-once) | Build | Implemented | Deps executed once |
| Parallel execution | Build | Implemented | ParallelDeps concurrency |
| File modification checks | Build | Implemented | `Newer` checks by cache |
| Checksum-based caching | Build | Implemented | `target.Checksum` |
| Watch mode | Build | Implemented | Add/remove/modify detection |
| Timeouts | Build | Planned | Cancel on timeout |
| Syscall helpers | Build | Planned | Convenience wrappers |
| Shell execution helpers | Build | Implemented | `targ/sh` helpers |

---

## Developer Experience

| Feature | Tier | Status | Test Coverage |
| --- | --- | --- | --- |
| Clear errors on invalid tags/types | Essential | Implemented | Invalid tags surface errors |
| Clear errors on unexported fields | Essential | Implemented | Unexported tagged field errors |
| Stable subcommand ordering in help | Essential | Implemented | Sorted subcommands in help |
| TagOptions error propagation | Essential | Implemented | TagOptions errors surface |
| README examples in sync | Extended | Planned | Doc/test sync checks |

---

## Compatibility / Constraints

| Feature | Tier | Status | Test Coverage |
| --- | --- | --- | --- |
| No source-code dependency at runtime | Core | Implemented | Binary safe (direct mode) |
| Deterministic behavior across platforms | Essential | Planned | Platform consistency checks |

---

## Prioritization Guide

| Priority | Focus |
| --- | --- |
| 1 | Core + Essential across Command Model, Execution Modes, Argument Parsing, Help |
| 2 | Execution semantics (errors, context, hooks) |
| 3 | Build-tool features (deps, watch, caching, multi-command) |
| 4 | Extended parser features (repeat/variadic/map/custom types) |

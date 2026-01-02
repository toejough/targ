# Commander Feature Taxonomy

Purpose: a concise, defensible checklist of intended features and scope, organized so we can implement and test with confidence.

How to use:

- Treat each row as a "feature contract" with explicit scope.
- When a row moves to "Implemented", add a minimal test entry.
- When a row is "Deferred", document the reason and any constraints.

Legend:

- Scope: Core (must-have), Essential (baseline CLI), Extended (nice-to-have), Build (build-tool focus)
- Status: Planned | In Progress | Implemented | Deferred

---

## Category: Command Model (Core)

- Root + subcommand discovery by struct graph (Core) | Status: Implemented
- Function targets (niladic) as commands (Core) | Status: Implemented
- Root behavior when no subcommand selected (Core) | Status: Implemented
- Subcommand naming overrides (Core) | Status: Implemented
- Help text from doc comments or tags (Essential) | Status: Implemented (struct Run comments + generated function wrappers)
- Errors for invalid/missing commands (Essential) | Status: Implemented

Tests:

- Single-root run with no args executes root Run()
- Nested subcommand invocation resolves correctly
- Name overrides match tag and field name precedence
- Niladic function command executes and appears in help

Status Notes:

- Implemented (struct Run comments + generated function wrappers) means `desc` tags are not currently used for command help.

---

## Category: Execution Modes (Core)

- Direct binary usage: main calls commander.Run(...) (Core) | Status: Implemented
- Build Tool Mode: external `commander` binary discovers commands and runs (Core) | Status: Implemented
- "Single root" shorthand (no command name) (Essential) | Status: Implemented
- Multiple root selection by name (Essential) | Status: Implemented
- Build tool mode discovery by build tag (Core) | Status: Implemented
- Build tool mode recursive search with depth gating (Core) | Status: Implemented
- Build tool mode package grouping (Core) | Status: Implemented
- Build tool mode function discovery (Core) | Status: Implemented
- Build tool mode filters subcommand structs/functions (Core) | Status: Implemented
- Build tool mode: no default command (Core) | Status: Implemented

Tests:

- Binary mode: root vs subcommand resolution
- Build tool mode: discovery filters non-commands; run works
- Build tool mode: multiple dirs at same depth errors with paths

---

## Category: Argument Parsing (Essential)

- Long flags (--flag) for basic types (Essential) | Status: Implemented
- Short flags (-f) (Essential) | Status: Implemented
- Positional args (Essential) | Status: Implemented (but see Issues)
- Required vs optional (Essential) | Status: Planned
- Default values from tags (Essential) | Status: Implemented
- Env var defaults (Essential) | Status: Implemented
- Boolean flags (Essential) | Status: Implemented
- Repeated flags (Extended) | Status: Planned
- Variadic positionals (Extended) | Status: Planned
- Map-type args (Extended) | Status: Planned
- Custom types via TextUnmarshaler (Extended) | Status: Implemented

Tests:

- Flag + positional parsing in same command
- Required validation and error surface
- Env overrides and default precedence
- Repeated/variadic behavior (if supported)

Status Notes:

- Implemented (but see Issues) for positional args refers to positional fields also being registered as flags.

---

## Category: Help, Usage, Completion (Essential)

- `--help` at root and subcommand levels (Essential) | Status: Implemented
- Usage shows subcommands and flags (Essential) | Status: Implemented
- Shell completion (bash/zsh/fish) (Essential) | Status: Implemented
- Completion with quoted/escaped args (Extended) | Status: Planned

Tests:

- Help output stable and includes descriptions
- Completion suggests subcommands + flags correctly

---

## Category: Execution Semantics (Essential)

- Run method signatures (niladic or context) (Essential) | Status: Implemented
- Error return support (Extended) | Status: Implemented
- Context/cancellation support (Extended) | Status: Implemented
- Lifecycle hooks (Before/After) (Extended) | Status: Planned

Tests:

- Run called once per command execution
- Error propagation to exit code
- Context cancellation cancels command

---

## Category: Multi-Command & Build-Tool Features (Build)

- Multiple commands in one invocation (Build) | Status: Planned
- Dependencies (run-once) (Build) | Status: Implemented
- Parallel execution (Build) | Status: Planned
- File modification checks (Build) | Status: Planned
- Checksum-based caching (Build) | Status: Planned
- Watch mode (Build) | Status: Planned
- Timeouts (Build) | Status: Planned
- Syscall helpers (Build) | Status: Planned
- Shell execution helpers (Build) | Status: Implemented

Tests:

- Dependency graph executes each target once
- Parallel order guarantees (when needed)
- Watcher cancels and restarts

---

## Category: Developer Experience (Essential)

- Clear errors on invalid tags or types (Essential) | Status: Planned
- Clear errors on unexported fields (Essential) | Status: Planned
- Stable command ordering in help (Essential) | Status: Implemented (subcommands only)
- Doc generation / README examples stay in sync (Extended) | Status: Planned

Tests:

- Invalid configs produce actionable error messages

Status Notes:

- Implemented (subcommands only) refers to stable sorting of subcommands; top-level command ordering is currently the input order.

---

## Category: Compatibility / Constraints (Core)

- No source-code dependency at runtime (binary safe) (Core) | Status: Planned
- Deterministic behavior across platforms (Essential) | Status: Planned

Tests:

- Build binary in temp dir; help still shows descriptions

---

## Prioritization Guide

1. Core + Essential across Command Model, Execution Modes, Argument Parsing, Help
2. Execution Semantics (error return, context)
3. Build-Tool features (deps, watch, parallel, caching)
4. Extended parser features (map, repeat, variadic, custom types)

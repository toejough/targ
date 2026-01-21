# Requirements vs Architecture

A systematic comparison of the requirements spec versus the architecture spec.

## Summary

The architecture generally implements the requirements, but with notable scope changes:

- **Removes CLI operations** for Transform and Manage Dependencies (users edit source directly)
- **Adds runtime overrides** as CLI flags (requirements says "behavior modification is done in code")
- **Adds features** not explicitly in requirements (shell commands, ordered args, ownership model)

---

## Model: Targets

### Progressive Capabilities

| Capability      | Requirement                           | Architecture              | Status |
| --------------- | ------------------------------------- | ------------------------- | ------ |
| Basic           | Executable behavior                   | `func Name()`             | MATCH  |
| Failure         | Indicate success/failure              | `func Name() error`       | MATCH  |
| Cancellation    | Respond to interrupt/timeout          | `func Name(ctx) error`    | MATCH  |
| Dependencies    | Run other targets first, exactly once | `.Deps(targets...)`       | MATCH  |
| Parallel        | Run multiple targets concurrently     | `.DepMode(targ.Parallel)` | MATCH  |
| Serial          | Run multiple targets sequentially     | `.Deps()` (default)       | MATCH  |
| Help text       | Documentation in CLI                  | `.Description(s)`         | MATCH  |
| Arguments       | Accept flags/positionals from CLI     | Args struct with tags     | MATCH  |
| Repeated args   | Accumulate multiple values            | `[]T` field               | MATCH  |
| Map args        | Key=value syntax                      | `map[K]V` field           | MATCH  |
| Variadic args   | Trailing positional captures rest     | Trailing `[]T`            | MATCH  |
| Subcommands     | Nested command hierarchy              | `targ.Group()`            | MATCH  |
| Result caching  | Skip if inputs unchanged              | `.Cache(patterns...)`     | MATCH  |
| Watch mode      | Re-run on file changes                | `.Watch(patterns...)`     | MATCH  |
| Retry + backoff | Re-run on failure with delay          | `.Retry(n)`, `.Backoff()` | MATCH  |
| Repetition      | Run N times regardless of outcome     | `.Times(n)`               | MATCH  |
| Time-bounded    | Run until duration elapsed            | `.Timeout(duration)`      | MATCH  |
| Condition-based | Run until predicate is true           | `.While(func() bool)`     | MATCH  |

### Architecture Adds (not in requirements)

| Feature               | Architecture                 | Decision |
| --------------------- | ---------------------------- | -------- |
| Shell command targets | `targ.Targ("cmd $var")`      | **ADD?** |
| Shell helper          | `targ.Shell(ctx, cmd, args)` | **ADD?** |
| Ordered arguments     | `Interleaved[T]` type        | **ADD?** |
| Target name override  | `.Name(s)`                   | **ADD?** |
| Reset deps for watch  | `targ.ResetDeps()`           | **ADD?** |

---

## Model: Hierarchy

### Concepts

| Requirement                      | Architecture                    | Status |
| -------------------------------- | ------------------------------- | ------ |
| Namespace nodes (non-executable) | Groups are non-executable       | MATCH  |
| Target nodes (executable)        | Functions are only executables  | MATCH  |
| Path addressing                  | Stack-based with `*`, `**`, `^` | MATCH  |

### Organization Requirements

| Requirement                     | Architecture                 | Status |
| ------------------------------- | ---------------------------- | ------ |
| Simplest possible definition    | `//go:build targ` + function | MATCH  |
| Targets near relevant code      | Any file with build tag      | MATCH  |
| Scales to complex hierarchies   | Nested `targ.Group()`        | MATCH  |
| Namespacing to avoid collisions | Groups provide namespacing   | MATCH  |
| Easy CLI binary transition      | Remove tag, rename init→main | MATCH  |

### Addressing Requirements

| Requirement                    | Architecture                   | Status   |
| ------------------------------ | ------------------------------ | -------- |
| Uniquely identify any point    | Full path specification        | MATCH    |
| Specify multiple locations     | Multiple names on command line | MATCH    |
| Select targets by pattern      | `*` and `**` globs             | MATCH    |
| Unambiguous user intent        | Stack-based traversal rules    | MATCH    |
| Lossless user/internal convert | Not explicitly documented      | **GAP?** |

### Implications

| Requirement                              | Architecture               | Status  |
| ---------------------------------------- | -------------------------- | ------- |
| Create intermediate nodes as needed      | "Creates groups as needed" | MATCH   |
| No empty namespace nodes after operation | Not documented             | **GAP** |

---

## Model: Sources

### Local Sources

| Requirement                   | Architecture            | Status |
| ----------------------------- | ----------------------- | ------ |
| Defined in current repository | `//go:build targ` files | MATCH  |

### Remote Sources

| Requirement                            | Architecture                | Status  |
| -------------------------------------- | --------------------------- | ------- |
| Add targets from another repo          | `--sync github.com/foo/bar` | MATCH   |
| Track which targets came from source   | Not documented              | **GAP** |
| Update when remote changes             | "Update module version"     | MATCH   |
| Only modify/remove from-source targets | Not documented              | **GAP** |

---

## Operations

### Create

| Requirement                    | Architecture                       | Status |
| ------------------------------ | ---------------------------------- | ------ |
| Scaffold from shell command    | `--create "cmd"`                   | MATCH  |
| Creates simplest possible      | String target, no execution config | MATCH  |
| User adds capabilities in code | Documented                         | MATCH  |

### Invoke

#### CLI Invocation

| Requirement                 | Architecture                          | Status |
| --------------------------- | ------------------------------------- | ------ |
| Single target: `targ lint`  | Documented                            | MATCH  |
| Multiple: `targ build test` | Documented (sequential default)       | MATCH  |
| With args: `--env prod`     | Args struct documented                | MATCH  |
| Shared dependency state     | "dep runs once even if multiple need" | MATCH  |

#### Invocation Modifiers

Requirements says: "Invocation modifiers" for watch, retry, repetition, time-bounded, condition-based.

Architecture provides these as **runtime CLI flags**:

| Modifier        | Requirement              | Architecture         | Status     |
| --------------- | ------------------------ | -------------------- | ---------- |
| Watch mode      | Re-run on file changes   | `--watch` CLI flag   | **CHANGE** |
| Retry           | Re-run on failure        | `--retry` CLI flag   | **CHANGE** |
| Repetition      | Run N times              | Not as CLI flag      | **GAP**    |
| Time-bounded    | Run until duration       | `--timeout` CLI flag | **CHANGE** |
| Condition-based | Run until predicate true | Not as CLI flag      | **GAP**    |

**Key tension**: Requirements scope says "Behavior modification is done in code, not via CLI." But architecture adds runtime override flags.

#### Programmatic Invocation

| Requirement                  | Architecture                                | Status |
| ---------------------------- | ------------------------------------------- | ------ |
| Call targets from other code | `target.Run(ctx)` / `target.Run(ctx, args)` | MATCH  |
| Express dependencies         | `.Deps(targets...)`                         | MATCH  |
| Deps run exactly once        | Documented                                  | MATCH  |

### Transform

Requirements defines three transformations:

| Transform | Requirement                                | Architecture  | Status      |
| --------- | ------------------------------------------ | ------------- | ----------- |
| Rename    | Change target's path (move, nest, flatten) | "Edit source" | **REMOVED** |
| Relocate  | Move implementation to different file      | "Edit source" | **REMOVED** |
| Delete    | Remove a target entirely                   | "Edit source" | **REMOVED** |

Architecture explicitly removed CLI support: "Modify/Delete - users edit source directly since the model is simple enough."

**Exception**: `--move` flag exists in current implementation for hierarchy reorganization. Not in architecture.

### Manage Dependencies

| Requirement                   | Architecture        | Status      |
| ----------------------------- | ------------------- | ----------- |
| List dependencies of a target | `--help` shows deps | PARTIAL     |
| Add a dependency              | "Edit source"       | **REMOVED** |
| Remove a dependency           | "Edit source"       | **REMOVED** |
| Change execution mode         | "Edit source"       | **REMOVED** |

### Sync

| Requirement                        | Architecture                    | Status  |
| ---------------------------------- | ------------------------------- | ------- |
| Add targets from remote repo       | `--sync github.com/foo/bar`     | MATCH   |
| Update targets when remote changes | Re-run `--sync` updates version | MATCH   |
| Remove targets no longer in remote | Not documented                  | **GAP** |

### Inspect

| Query | Requirement               | Architecture                       | Status  |
| ----- | ------------------------- | ---------------------------------- | ------- |
| Where | Source location of target | `--help` shows "Source: file:line" | MATCH   |
| Tree  | Full hierarchy            | Group with no args shows tree      | MATCH   |
| Deps  | What depends on a target  | `--help` shows deps                | PARTIAL |

Note: Requirements asks "what depends on a target" (reverse deps). Architecture shows "what this target depends on" (forward deps). These are different.

### Shell Integration

| Requirement              | Architecture        | Status |
| ------------------------ | ------------------- | ------ | ------ | ----- |
| Supports bash, zsh, fish | `--completion {bash | zsh    | fish}` | MATCH |
| Completes target names   | Documented          | MATCH  |
| Completes flag names     | Documented          | MATCH  |
| Completes enum values    | Documented          | MATCH  |

---

## Constraints

### Invariants

Requirements has a 4-column table (Create, Modify, Delete, Remote Sync).
Architecture has a 2-column table (Create, Sync) since Modify/Delete were removed.

| Property            | Req: Create           | Arch: Create       | Status |
| ------------------- | --------------------- | ------------------ | ------ |
| Existing signatures | Existing unaffected   | Unaffected         | MATCH  |
| Help text refs      | Existing unaffected   | Unaffected         | MATCH  |
| CLI invocation      | Existing unaffected   | Unaffected         | MATCH  |
| Dependencies        | Existing unaffected   | Unaffected         | MATCH  |
| Direct call sites   | Existing unaffected   | Unaffected         | MATCH  |
| Behavior            | Existing unaffected   | Unaffected         | MATCH  |
| Reversibility       | Reversible via delete | "delete generated" | MATCH  |

| Property            | Req: Sync                 | Arch: Sync                        | Status |
| ------------------- | ------------------------- | --------------------------------- | ------ |
| Existing signatures | Only from-source affected | Only imported affected            | MATCH  |
| Help text refs      | Remain valid              | Remain valid                      | MATCH  |
| CLI invocation      | Only from-source affected | Only imported affected            | MATCH  |
| Dependencies        | Remain valid              | Remain valid                      | MATCH  |
| Direct call sites   | Remain valid              | Remain valid                      | MATCH  |
| Behavior            | Only from-source affected | Only imported affected            | MATCH  |
| Reversibility       | Reversible via remove     | "remove import and registrations" | MATCH  |

### Principles

| Principle       | Requirement                            | Architecture               | Status |
| --------------- | -------------------------------------- | -------------------------- | ------ |
| Reversible      | All ops reversible via command surface | Create→delete, Sync→remove | MATCH  |
| Minimal changes | Prefer minimal code changes            | "add to existing file"     | MATCH  |
| Fail clearly    | Fail with clear error message          | Documented                 | MATCH  |

---

## Architecture Adds (not in requirements)

| Feature                | Description                                        | Decision |
| ---------------------- | -------------------------------------------------- | -------- |
| Runtime override flags | `--watch`, `--cache`, `--timeout`, `--retry`, etc. | **ADD?** |
| Ownership model        | `targ.Disabled` for user takeover of flags         | **ADD?** |
| `--parallel` / `-p`    | Run multiple targets in parallel                   | **ADD?** |
| `--source` / `-s`      | Specify source file explicitly                     | **ADD?** |
| `--to-func`            | Convert string target to function                  | **ADD?** |
| `--to-string`          | Convert function target to string                  | **ADD?** |
| `--no-binary-cache`    | Disable targ binary caching                        | **ADD?** |
| Shell command targets  | `targ.Targ("cmd $var")`                            | **ADD?** |
| `targ.Shell()` helper  | Run shell command with var substitution            | **ADD?** |
| Ordered arguments      | `Interleaved[T]` for preserving flag order         | **ADD?** |
| `.Name()` builder      | Override CLI name for target                       | **ADD?** |
| `targ.ResetDeps()`     | Clear dep cache for watch/repeat                   | **ADD?** |
| `^` path reset         | Reset to root in path traversal                    | **ADD?** |
| `*` / `**` globs       | Pattern matching in paths                          | **ADD?** |

---

## Questions for Review

1. **Runtime override flags**: Requirements says "behavior modification is done in code, not via CLI." Architecture adds `--watch`, `--cache`, `--timeout`, `--retry` as CLI flags. Keep or remove?

2. **Transform operations removed**: No CLI for rename/relocate/delete. Users edit source. Accept?

3. **Manage Dependencies removed**: No CLI to add/remove deps or change execution mode. Users edit source. Accept?

4. **Repetition/condition CLI flags**: Requirements lists these as invocation modifiers. Architecture doesn't provide `--times` or `--while` CLI flags. Add?

5. **Reverse deps**: Requirements asks "what depends on a target." Architecture shows forward deps only. Add reverse deps to inspect?

6. **Remote source tracking**: Requirements says "track which targets came from which source" and "only modify/remove targets originally from that source." Not explicit in architecture. Add?

7. **Empty namespace cleanup**: Requirements says "no empty namespace nodes at conclusion of operation." Not documented in architecture. Add?

8. **--move flag**: Current implementation has `--move`. Not in architecture. Keep?

## Answers

1. keep
2. accept. update the requirements to indicate the changed decision
3. accept. update the requirements to indicate the changed decision
4. let's talk about this one. What's enabled in the targ builder, that isn't exposed via CLI?
5. no, leave as is, and update the requirements to indicate the changed decision
6. add - I would like to see in the help where targets came from - list the source at the highest level possible, eg if
   most targets came from file X, say that at the top rather than per-target. If a group beneath that has targets from a
   different source, indicate that at the group level, etc.
7. this is unnecessary in the architecture since users edit source directly for modifications/deletions. Note that in
   the architecture.
8. remove.


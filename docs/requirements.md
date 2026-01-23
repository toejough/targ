# Target Manipulation: Problem Specification

## Model

What exists and how it's structured.

### Targets

**Requirements:**

- Start as simply as possible
- Add capabilities incrementally as needed
- Each capability has minimal syntax overhead

**Progressive capabilities:**

| Capability      | What it enables                           |
| --------------- | ----------------------------------------- |
| Basic           | Executable behavior                       |
| Failure         | Indicate success/failure                  |
| Cancellation    | Respond to interrupt/timeout              |
| Dependencies    | Run other targets first, exactly once     |
| Parallel        | Run multiple targets concurrently         |
| Serial          | Run multiple targets sequentially         |
| Help text       | Documentation in CLI                      |
| Arguments       | Accept flags/positionals from CLI         |
| Repeated args   | Accumulate multiple values for same flag  |
| Map args        | Key=value syntax for structured input     |
| Variadic args   | Trailing positional captures remaining    |
| Subcommands     | Nested command hierarchy                  |
| Result caching  | Skip if inputs unchanged                  |
| Watch mode      | Re-run on file changes                    |
| Retry           | Re-run on failure (with optional backoff) |
| Repetition      | Run N times regardless of outcome         |
| Time-bounded    | Run until duration elapsed                |
| Condition-based | Run until predicate is true               |

### Hierarchy

**Concepts:**

- **Node**: A point in the hierarchy
  - _Namespace node_: Organizes children, no executable behavior
  - _Target node_: Has executable behavior
- **Path**: A sequence of names identifying a location in the hierarchy
- **Target**: Executable behavior at a node

**Implications:**

- Paths can reference nodes that exist or don't exist
- A path may resolve to a namespace, a target, or nothing
- Operations may create intermediate nodes as needed
- Namespace nodes can exist without targets (pure organizational)
- No empty namespace nodes at the conclusion of any user operation (transient only)

**Organization requirements:**

- Simplest possible definition (write a function, it's discovered)
- Targets can live near relevant code (close context)
- Scales to complex hierarchies when needed
- Namespacing to avoid collisions at scale
- Easy transition to dedicated CLI binary (from `targ lint` to `./myapp lint`)

**Addressing requirements:**

- Uniquely identify any point in the hierarchy (variable depth)
- Specify multiple locations in a single operation
- Select targets by pattern without naming each explicitly
- User intent is unambiguous (or has clear defaults per operation)
- User-facing and internal representations convert losslessly
- Path traversal continues from current group after hitting a target
- `--` resets path to root for accessing top-level targets after nested ones
- Name collisions between top-level targets and groups error at registration

### Sources

Where targets come from.

**Local:** Defined in the current repository.

**Remote:**

- Add targets from another repository
- Track which targets came from which source
- Sync/update when remote definitions change (add, modify, remove)
- Only modify/remove targets originally from that source

## Operations

What users do.

**Scope:** CLI operations are for interacting with and reorganizing targets. Behavior modification (adding capabilities like retry, caching, arguments) is done in code, not via CLI.

### Create

Scaffold a target from a shell command.

- Creates the simplest possible target (Basic capability only)
- User adds capabilities in code as needed

### Invoke

Run targets.

**CLI invocation:** Run targets from command line.

- Single target: `targ lint`
- Multiple targets: `targ build test deploy` (runs in sequence, shared dependency state)
- With arguments: `targ deploy --env prod`

**Invocation modifiers:**

- Watch mode: Re-run on file changes
- Retry: Re-run on failure
- Repetition: Run N times
- Time-bounded: Run until duration elapsed
- Condition-based: Run until predicate is true

**Programmatic invocation:**

- Call targets from other code
- Express dependencies between targets
- Dependencies run exactly once per execution

### Transform

Change targets. **Users edit source directly** - the model is simple enough that CLI tooling is unnecessary.

| Transformation | What it does                                           |
| -------------- | ------------------------------------------------------ |
| Rename         | Change a target's path (includes move, nest, flatten)  |
| Relocate       | Move implementation to different file (path unchanged) |
| Delete         | Remove a target entirely                               |

### Manage Dependencies

Modify relationships between targets. **Users edit source directly** - the model is simple enough that CLI tooling is unnecessary.

- List dependencies of a target (via `--help`)
- Add/remove dependencies (edit source)
- Change execution mode (edit source)

### Sync

Manage remote targets.

- Add targets from a remote repository
- Update targets when remote changes
- Remove targets no longer in remote (if originally from that source)

### Inspect

Query information about targets.

| Query | What it answers              |
| ----- | ---------------------------- |
| Where | Source location of a target  |
| Tree  | Full hierarchy visualization |
| Deps  | Dependencies of a target     |

### Shell Integration

Generate shell completion scripts for tab-completion of targets and flags.

- Supports bash, zsh, fish
- Completes target names, flag names, and enum values

## Constraints

What must be true.

### Invariants

What must hold true across operations:

| Property            | Create                | Modify                 | Delete                 | Remote Sync                       |
| ------------------- | --------------------- | ---------------------- | ---------------------- | --------------------------------- |
| Existing signatures | Existing unaffected   | Unaffected             | Others unaffected      | Only from-source targets affected |
| Help text refs      | Existing unaffected   | Remain valid           | No dangling refs       | Remain valid                      |
| CLI invocation      | Existing unaffected   | Reflects current path  | Others unaffected      | Only from-source targets affected |
| Dependencies        | Existing unaffected   | Remain valid           | No dangling refs       | Remain valid                      |
| Direct call sites   | Existing unaffected   | Remain valid           | No dangling refs       | Remain valid                      |
| Behavior            | Existing unaffected   | Unchanged              | Others unaffected      | Only from-source targets affected |
| Reversibility       | Reversible via delete | Reversible via rename  | Reversible             | Reversible via remove             |

### Principles

- **Reversible**: All operations reversible through the command surface
- **Minimal changes**: Prefer minimal code/file changes; leave targets in current file rather than extracting/restructuring unnecessarily
- **Fail clearly**: If invariants cannot be maintained, the operation must fail with a clear error message

---

## Implementation Status

Verified 2026-01-23.

### Target Capabilities

| Capability      | Status | Implementation |
| --------------- | ------ | -------------- |
| Basic           | ✅ | `func Name()` |
| Failure         | ✅ | `func Name() error` |
| Cancellation    | ✅ | `func Name(ctx context.Context) error` |
| Dependencies    | ✅ | `.Deps()` |
| Parallel        | ✅ | `.ParallelDeps()`, `--parallel/-p` |
| Serial          | ✅ | `.Deps()` (default) |
| Help text       | ✅ | `.Description()` |
| Arguments       | ✅ | Struct parameter with `targ:` tags |
| Repeated args   | ✅ | `[]T` field type |
| Map args        | ✅ | `map[K]V` field type |
| Variadic args   | ✅ | Trailing `[]T` positional |
| Subcommands     | ✅ | `targ.Group()` |
| Result caching  | ✅ | `.Cache()` |
| Watch mode      | ✅ | `.Watch()` |
| Retry           | ✅ | `.Retry()`, `.Backoff()` |
| Repetition      | ✅ | `.Times()` |
| Time-bounded    | ✅ | `.Timeout()` |
| Condition-based | ✅ | `.While()` |

### Hierarchy

| Requirement | Status | Notes |
| ----------- | ------ | ----- |
| Namespace nodes | ✅ | `targ.Group()` |
| Target nodes | ✅ | `targ.Targ(fn)` |
| Path addressing | ✅ | Stack-based traversal |
| Glob patterns (`*`, `**`) | ⚠️ Gap | Not implemented |
| `--` resets to root | ✅ | Implemented |
| Name collision errors | ✅ | At registration |

### Operations

| Requirement | Status | Notes |
| ----------- | ------ | ----- |
| Create (scaffold) | ✅ | `--create` with `--deps`, `--cache` |
| Invoke: CLI | ✅ | `targ <target>` |
| Invoke: modifiers | ✅ | `--watch`, `--cache`, `--timeout`, etc. |
| Invoke: programmatic | ✅ | `target.Run(ctx)` |
| Transform | ✅ | Users edit source (by design) |
| Manage Dependencies | ✅ | Users edit source (by design) |
| Sync (remote) | ✅ | `--sync` |
| Inspect: Where | ⚠️ Gap | Source location not in `--help` |
| Inspect: Tree | ✅ | Group shows hierarchy |
| Inspect: Deps | ⚠️ Gap | Deps not shown in `--help` |
| Shell Integration | ✅ | `--completion` for bash/zsh/fish |

### Gaps Summary

1. **Glob patterns in paths** - `targ dev *` and `targ **` not supported
2. **Inspect: Where** - Target `--help` doesn't show source file location
3. **Inspect: Deps** - Target `--help` doesn't show configured dependencies/execution info

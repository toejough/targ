# Target Manipulation: Requirements

## Model

What exists and how it's structured.

### Targets

**Design Principles:**

- Start as simply as possible
- Add capabilities incrementally as needed
- Each capability has minimal syntax overhead

**Progressive Capabilities:**

### REQ-001: Basic

Executable behavior.

**Category:** Capability

### REQ-002: Failure

Indicate success/failure.

**Category:** Capability

### REQ-003: Cancellation

Respond to interrupt/timeout.

**Category:** Capability

### REQ-004: Dependencies

Run other targets first, exactly once.

**Category:** Capability

### REQ-005: Parallel

Run multiple targets concurrently. Supports chaining with serial groups for mixed execution modes.

**Category:** Capability

### REQ-006: Serial

Run multiple targets sequentially.

**Category:** Capability

### REQ-007: Help text

Documentation in CLI.

**Category:** Capability

### REQ-008: Arguments

Accept flags/positionals from CLI.

**Category:** Capability

### REQ-009: Repeated args

Accumulate multiple values for same flag.

**Category:** Capability

### REQ-010: Map args

Key=value syntax for structured input.

**Category:** Capability

### REQ-011: Variadic args

Trailing positional captures remaining.

**Category:** Capability

### REQ-012: Subcommands

Nested command hierarchy.

**Category:** Capability

### REQ-013: Result caching

Skip if inputs unchanged.

**Category:** Capability

### REQ-014: Watch mode

Re-run on file changes.

**Category:** Capability

### REQ-015: Retry

Re-run on failure (with optional backoff).

**Category:** Capability

### REQ-016: Repetition

Run N times regardless of outcome.

**Category:** Capability

### REQ-017: Time-bounded

Run until duration elapsed.

**Category:** Capability

### REQ-018: Condition-based

Run until predicate is true.

**Category:** Capability

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

**Organization:**

### REQ-019: Simple Definition

Simplest possible definition (write a function, it's discovered).

**Category:** Organization

### REQ-020: Close Context

Targets can live near relevant code.

**Category:** Organization

### REQ-021: Scalable Hierarchies

Scales to complex hierarchies when needed.

**Category:** Organization

### REQ-022: Namespacing

Namespacing to avoid collisions at scale.

**Category:** Organization

### REQ-023: CLI Binary Transition

Easy transition to dedicated CLI binary (from `targ lint` to `./myapp lint`).

**Category:** Organization

**Addressing:**

### REQ-024: Unique Identification

Uniquely identify any point in the hierarchy (variable depth).

**Category:** Addressing

### REQ-025: Multiple Locations

Specify multiple locations in a single operation.

**Category:** Addressing

### REQ-027: Unambiguous Intent

User intent is unambiguous (or has clear defaults per operation).

**Category:** Addressing

### REQ-028: Lossless Conversion

User-facing and internal representations convert losslessly.

**Category:** Addressing

### REQ-029: Path Continuation

Path traversal continues from current group after hitting a target.

**Category:** Addressing

### REQ-030: Root Reset

`--` resets path to root for accessing top-level targets after nested ones.

**Category:** Addressing

### REQ-031: Collision Detection

Name collisions between top-level targets and groups error at registration.

**Category:** Addressing

### Sources

Where targets come from.

### REQ-032: Local Targets

Local targets defined in the current repository.

**Category:** Sources

### REQ-033: Remote Targets

Add targets from another repository (remote).

**Category:** Sources

### REQ-034: Source Tracking

Track which targets came from which source.

**Category:** Sources

### REQ-035: Remote Sync

Sync/update when remote definitions change (add, modify, remove).

**Category:** Sources

### REQ-036: Source Isolation

Only modify/remove targets originally from that source.

**Category:** Sources

## Operations

What users do.

**Scope:** CLI operations are for interacting with and reorganizing targets. Behavior modification (adding capabilities like retry, caching, arguments) is done in code, not via CLI.

### Create

Scaffold a target from a shell command.

### REQ-037: Simple Scaffold

Creates the simplest possible target (Basic capability only).

**Category:** Create

### REQ-038: Code Extension

User adds capabilities in code as needed.

**Category:** Create

### Invoke

Run targets.

**CLI invocation:**

### REQ-039: Single Target

Single target: `targ lint`.

**Category:** Invoke

### REQ-040: Multiple Targets

Multiple targets: `targ build test deploy` (runs in sequence, shared dependency state).

**Category:** Invoke

### REQ-041: With Arguments

With arguments: `targ deploy --env prod`.

**Category:** Invoke

**Invocation modifiers:**

### REQ-042: Watch Modifier

Watch mode: Re-run on file changes.

**Category:** Invoke Modifier

### REQ-043: Retry Modifier

Retry: Re-run on failure.

**Category:** Invoke Modifier

### REQ-044: Repetition Modifier

Repetition: Run N times.

**Category:** Invoke Modifier

### REQ-045: Time-bounded Modifier

Time-bounded: Run until duration elapsed.

**Category:** Invoke Modifier

### REQ-046: Condition Modifier

Condition-based: Run until predicate is true.

**Category:** Invoke Modifier

**Programmatic invocation:**

### REQ-047: Code Invocation

Call targets from other code.

**Category:** Programmatic

### REQ-048: Dependency Expression

Express dependencies between targets.

**Category:** Programmatic

### REQ-049: Once-Only Dependencies

Dependencies run exactly once per execution.

**Category:** Programmatic

### Transform

Change targets. **Users edit source directly** - the model is simple enough that CLI tooling is unnecessary.

### REQ-050: Rename

Change a target's path (includes move, nest, flatten).

**Category:** Transform

### REQ-051: Relocate

Move implementation to different file (path unchanged).

**Category:** Transform

### REQ-052: Delete

Remove a target entirely.

**Category:** Transform

### Manage Dependencies

Modify relationships between targets. **Users edit source directly** - the model is simple enough that CLI tooling is unnecessary.

### REQ-053: List Dependencies

List dependencies of a target (via `--help`).

**Category:** Dependencies

### REQ-054: Add/Remove Dependencies

Add/remove dependencies (edit source).

**Category:** Dependencies

### REQ-055: Change Execution Mode

Change execution mode (edit source).

**Category:** Dependencies

### Sync

Manage remote targets.

### REQ-056: Add Remote

Add targets from a remote repository.

**Category:** Sync

### REQ-057: Update Remote

Update targets when remote changes.

**Category:** Sync

### REQ-058: Remove Stale

Remove targets no longer in remote (if originally from that source).

**Category:** Sync

### Inspect

Query information about targets.

### REQ-059: Where Query

Source location of a target.

**Category:** Inspect

### REQ-060: Tree Query

Full hierarchy visualization.

**Category:** Inspect

### REQ-061: Deps Query

Dependencies of a target.

**Category:** Inspect

### Shell Integration

Generate shell completion scripts for tab-completion of targets and flags.

### REQ-062: Shell Support

Supports bash, zsh, fish.

**Category:** Shell Integration

### REQ-063: Completion Scope

Completes target names, flag names, and enum values.

**Category:** Shell Integration


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

Verified 2026-01-30.

### Target Capabilities

| Capability      | Status | Implementation |
| --------------- | ------ | -------------- |
| REQ-001 Basic           | ✅ | `func Name()` |
| REQ-002 Failure         | ✅ | `func Name() error` |
| REQ-003 Cancellation    | ✅ | `func Name(ctx context.Context) error` |
| REQ-004 Dependencies    | ✅ | `.Deps()` |
| REQ-005 Parallel        | ✅ | `.Deps(..., DepModeParallel)`, chain calls for mixed groups, `--parallel/-p` |
| REQ-006 Serial          | ✅ | `.Deps()` (default) |
| REQ-007 Help text       | ✅ | `.Description()` |
| REQ-008 Arguments       | ✅ | Struct parameter with `targ:` tags |
| REQ-009 Repeated args   | ✅ | `[]T` field type |
| REQ-010 Map args        | ✅ | `map[K]V` field type |
| REQ-011 Variadic args   | ✅ | Trailing `[]T` positional |
| REQ-012 Subcommands     | ✅ | `targ.Group()` |
| REQ-013 Result caching  | ✅ | `.Cache()` |
| REQ-014 Watch mode      | ✅ | `.Watch()` |
| REQ-015 Retry           | ✅ | `.Retry()`, `.Backoff()` |
| REQ-016 Repetition      | ✅ | `.Times()` |
| REQ-017 Time-bounded    | ✅ | `.Timeout()` |
| REQ-018 Condition-based | ✅ | `.While()` |

### Hierarchy

| Requirement | Status | Notes |
| ----------- | ------ | ----- |
| REQ-019–022 Organization | ✅ | `targ.Group()`, discovery |
| REQ-023 CLI binary | ✅ | `targ.Run()` entry point |
| REQ-024–025 Addressing | ✅ | Stack-based traversal |
| REQ-027–028 Representations | ✅ | Lossless conversion |
| REQ-029–030 Path traversal | ✅ | Implemented |
| REQ-031 Name collisions | ✅ | Error at registration |

### Operations

| Requirement | Status | Notes |
| ----------- | ------ | ----- |
| REQ-037–038 Create | ✅ | `--create` with `--deps`, `--cache` |
| REQ-039–041 Invoke CLI | ✅ | `targ <target>` |
| REQ-042–046 Invoke modifiers | ✅ | `--watch`, `--cache`, `--timeout`, etc. |
| REQ-047–049 Invoke programmatic | ✅ | `target.Run(ctx)` |
| REQ-050–052 Transform | ✅ | Users edit source (by design) |
| REQ-053–055 Manage Dependencies | ✅ | Users edit source (by design) |
| REQ-056–058 Sync | ✅ | `--sync` |
| REQ-059 Inspect: Where | ✅ | `Source: path:line` in `--help` |
| REQ-060 Inspect: Tree | ✅ | Group shows hierarchy |
| REQ-061 Inspect: Deps | ✅ | `Execution:` section in `--help` |
| REQ-062–063 Shell Integration | ✅ | `--completion` for bash/zsh/fish |

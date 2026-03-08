# L2: Requirements (Adoption)

Induced bottom-up from L3 ARCH items. Traces filled mechanically as items are created.

## REQ-1: Declarative Target Definition

Targets are defined declaratively from Go functions or shell command strings. Struct tags on function parameters map to CLI flags and positional arguments. Target metadata (name, description, deps, cache, watch, timeout) is configured via a fluent builder API.

- Traces to: UC-1
- L3 items: ARCH-1, ARCH-6, ARCH-11

## REQ-2: Deterministic Execution

Targets execute their function or shell command. Dependencies run before the target in serial or parallel mode. Errors aggregate (MultiError for parallel). Execution respects timeouts and cancellation. Results track status (Pass/Fail/Errored/Cancelled).

- Traces to: UC-1
- L3 items: ARCH-2, ARCH-10

## REQ-3: Conflict-Free Multi-Package Registration

Multiple packages can register targets. Same-name targets from different packages are detected as conflicts before execution. Deregistration removes all targets from a package. Conflict errors report all conflicts at once with actionable fix suggestions.

- Traces to: UC-2
- L3 items: ARCH-4, ARCH-12

## REQ-4: Structured Help Output

Help text follows a defined section order (usage, description, subcommands, flags, examples). ANSI styling is properly paired. Empty sections are omitted. Trailing whitespace is stripped. Binary mode filters targ-specific content.

- Traces to: UC-3, UC-4
- L3 items: ARCH-5

## REQ-5: Input Validation

Required fields, enum constraints, and type constraints are enforced at parse time. Invalid input produces clear error messages with usage information.

- Traces to: UC-1
- L3 items: ARCH-1, ARCH-8

## REQ-6: Hierarchical Organization

Targets can be organized into named groups. Groups can nest. Path resolution navigates nested targets. Group names are validated.

- Traces to: UC-2
- L3 items: ARCH-3

## REQ-7: Shell Completion

Shell completion scripts are generated. Tab completion suggests targets, flags, and argument values.

- Traces to: UC-3, UC-4
- L3 items: ARCH-7

## REQ-8: Runtime Override

CLI flags (--cache, --watch, --times, etc.) override target configuration at runtime without modifying the target definition.

- Traces to: UC-1
- L3 items: ARCH-9, ARCH-13

## REQ-9: Auto-Discovery and Compilation

Target files with `//go:build targ` tags are automatically discovered. The CLI runner compiles a temporary binary and invokes it. Scaffolding (`--create`) generates correct Go code.

- Traces to: UC-2
- L3 items: ARCH-12, ARCH-14

## REQ-10: Git-Aware Operations

Repository URL and worktree cleanliness are detectable for build guards and metadata.

- Traces to: UC-1
- L3 items: ARCH-15

## REQ-11: Example Correctness

Example code in the repository compiles and demonstrates valid usage patterns.

- Traces to: UC-2
- L3 items: ARCH-16

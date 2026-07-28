# Targ Design Specification

Project-level design decisions for the targ CLI tool.

## CLI Interface

### DES-001: Help Text Structure

**Traces to:** REQ-007, REQ-053, REQ-060, REQ-061

Help output follows a consistent structure:
1. One-line description
2. Usage line with flag placeholders
3. Flags section (Global and Root-only subsections)
4. Formats section explaining placeholders
5. Commands section with source locations
6. Examples section

### DES-002: Flag Organization

**Traces to:** REQ-008, REQ-039, REQ-040, REQ-041

Flags are organized into two categories:
- **Global flags:** Apply to any invocation (`--timeout`, `--parallel`, `--retry`, etc.)
- **Root-only flags:** Only valid at the top level (`--create`, `--sync`, `--completion`)

Short flags use single letters: `-h`, `-p`, `-s`.

### DES-003: Command Discovery Display

**Traces to:** REQ-019, REQ-020, REQ-060

Superseded — see `openspec/specs/help-rendering/spec.md` for what `Source:` displays.

### DES-004: Path Stack Syntax

**Traces to:** REQ-024, REQ-025, REQ-029, REQ-030

Path traversal uses a stack-based syntax:
- Words traverse into groups until reaching a target
- What happens to leftover words after a target, and what `--` does — superseded, see
  `openspec/specs/execution-engine/spec.md`

```
targ dev build test     # dev/build, dev/test
targ dev build -- prod  # dev/build, then prod (see execution-engine spec for what -- does)
```

## Output Formatting

### DES-005: Error Message Format

**Traces to:** REQ-002

Errors are prefixed with "Error:" and written to stderr:
```
Error: no target files found
Error: unknown flag: --invalid
```

### DES-006: Progress Indication

**Traces to:** REQ-004, REQ-005, REQ-014

When running targets:
- Dependencies are run before the target
- Parallel execution shows concurrent progress
- Watch mode indicates file change triggers

## Interactive Patterns

### DES-007: Non-Interactive by Default

**Traces to:** REQ-037, REQ-038

The CLI is non-interactive by default:
- `--create` generates code without prompts
- No confirmation dialogs for operations
- Errors fail fast with clear messages

### DES-008: Shell Completion Integration

**Traces to:** REQ-062, REQ-063

Shell completion is installed via:
```bash
source <(targ --completion)        # Auto-detect shell
source <(targ --completion bash)   # Explicit shell
```

Completes: target names, flag names, flag values.

## Execution Patterns

### DES-009: Modifier Flag Syntax

**Traces to:** REQ-042, REQ-043, REQ-044, REQ-045, REQ-046

Execution modifiers use consistent flag patterns:
- `--times N` - repeat N times
- `--retry` - continue on failure
- `--backoff D,M` - exponential backoff (duration, multiplier)
- `--watch GLOB` - file watch patterns (repeatable)
- `--cache GLOB` - cache key patterns (repeatable)
- `--while CMD` - condition command

### DES-010: Dependency Mode Selection

**Traces to:** REQ-004, REQ-005, REQ-006

Dependencies can run serial (default) or parallel. Chain calls for mixed serial/parallel groups:
- Code: `.Deps(..., DepModeParallel)` or `.Deps(a).Deps(b, c, DepModeParallel).Deps(d)`
- CLI: `--dep-mode parallel` (overrides all groups)

### DES-011: Source Resolution

**Traces to:** REQ-019, REQ-032, REQ-033

Source files are discovered automatically:
- Recursive search from cwd
- Files with `//go:build targ` tag
- `--source` overrides for explicit paths

## Traceability Matrix

| Design ID | Design Element | Requirements Traced |
|-----------|----------------|---------------------|
| DES-001 | Help Text Structure | REQ-007, REQ-053, REQ-060, REQ-061 |
| DES-002 | Flag Organization | REQ-008, REQ-039, REQ-040, REQ-041 |
| DES-003 | Command Discovery Display | REQ-019, REQ-020, REQ-060 |
| DES-004 | Path Stack Syntax | REQ-024, REQ-025, REQ-029, REQ-030 |
| DES-005 | Error Message Format | REQ-002 |
| DES-006 | Progress Indication | REQ-004, REQ-005, REQ-014 |
| DES-007 | Non-Interactive by Default | REQ-037, REQ-038 |
| DES-008 | Shell Completion Integration | REQ-062, REQ-063 |
| DES-009 | Modifier Flag Syntax | REQ-042, REQ-043, REQ-044, REQ-045, REQ-046 |
| DES-010 | Dependency Mode Selection | REQ-004, REQ-005, REQ-006 |
| DES-011 | Source Resolution | REQ-019, REQ-032, REQ-033 |
### DES-012: Color Palette
**Traces to:** REQ-007

The Rich styling approach uses ANSI color codes for terminal output:

- **Section Headers** (Usage:, Description:, Flags:, etc.): Bold white (`\x1b[1m`)
- **Flag Names** (--timeout, -h): Cyan (`\x1b[36m`)
- **Placeholders** (<duration>, <command>): Yellow (`\x1b[33m`)
- **Subsection Headers** (e.g. `Global:`, `Root only:` — see help-rendering spec for the actual header names): Bold (`\x1b[1m`)
- **Examples**: Plain text (no styling)
- **Format Names** (in Formats section): Yellow (`\x1b[33m`)
- **Reset**: (`\x1b[0m`) after each styled element

**Color accessibility:** All colors have sufficient contrast against standard terminal backgrounds (both light and dark modes). Bold text is used for headers to ensure visibility without color.

### DES-013: Typography Scale
**Traces to:** REQ-007

Terminal output uses monospace fonts with the following hierarchy:

- **Section Headers**: Bold, title case with colon (e.g., "Usage:", "Flags:")
- **Subsection Headers**: Bold, title case with colon (actual header names: see help-rendering spec)
- **Body Text**: Regular weight, sentence case
- **Code Elements**: Inline, styled per DES-012 (flags in cyan, placeholders in yellow)
- **Indentation**: 2 spaces for section content, 4 spaces for nested content

### DES-014: Spacing System
**Traces to:** REQ-007

Vertical rhythm:

- **Between sections**: 1 blank line
- **Within sections**: No blank lines between items
- **After final section**: No trailing newline (clean output)

Horizontal spacing:

- **Base indentation**: 0 spaces for section headers
- **Content indentation**: 2 spaces from left margin
- **Flag descriptions**: Aligned after flag name with sufficient padding (varies by longest flag name)

### DES-015: Section Structure
**Traces to:** REQ-007

The actual canonical section order — superseded, see `openspec/specs/help-rendering/spec.md`.

Sections are omitted if empty (REQ-019, REQ-020, REQ-021).

## Components

### DES-016: Section Header Component
**Traces to:** REQ-007

**Structure:**
```
<BOLD><SECTION_NAME>:</BOLD>
```

**Properties:**
- Text style: Bold white
- Trailing: Colon
- Line break: After header

**Variants:**
- Usage:
- Positionals:
- Flags:
- Formats:
- Subcommands:
- Examples:

**Implementation note:** Uses `\x1b[1m` for bold, `\x1b[0m` for reset.

### DES-017: Subsection Header Component
**Traces to:** REQ-007

**Structure:**
```
  <BOLD><SUBSECTION_NAME>:</BOLD>
```

**Properties:**
- Indentation: 2 spaces
- Text style: Bold
- Trailing: Colon
- Line break: After header

Actual subsection variants, their nesting, and their ordering — superseded, see
`openspec/specs/help-rendering/spec.md`.

### DES-018: Flag Entry Component
**Traces to:** REQ-007, REQ-008

**Structure for boolean flags:**
```
    <CYAN>--<FLAG_NAME></CYAN>, <CYAN>-<SHORT></CYAN>    <DESCRIPTION>
```

**Structure for value-taking flags:**
```
    <CYAN>--<FLAG_NAME></CYAN> <YELLOW><<PLACEHOLDER>></YELLOW>    <DESCRIPTION>
```

**Properties:**
- Indentation: 4 spaces from left margin
- Flag name color: Cyan
- Placeholder color: Yellow
- Description: Plain text, aligned
- Padding: Sufficient space between flag and description

**Example:**
```
    --timeout <duration>    Set execution timeout
    --help, -h              Show help
```

### DES-019: Usage Line Component
**Traces to:** REQ-007

**Structure:** superseded, see `openspec/specs/help-rendering/spec.md` for whether `Usage:`
is a standalone header or an inline prefix.

**Variants:**
- Root-only flags: `targ <FLAG> [args...]`
- Target execution: `targ [flags...] <target> [args...]`
- Subcommands: `targ <command> [flags...] [subcommand]`

### DES-020: Example Entry Component
**Traces to:** REQ-007

**Structure:**
```
  targ <COMMAND_WITH_FLAGS>
```

**Properties:**
- Indentation: 2 spaces
- Text style: Plain (no coloring)
- Prefix: Always starts with "targ "
- Format: Complete, runnable command

**Ordering:**
- First example: Simplest/most common use case
- Middle examples: Progressive complexity
- Last example: Advanced features

**Count:** 2-3 examples per command (REQ-014), minimum 1 acceptable (REQ-023).

### DES-021: Positionals Entry Component
**Traces to:** REQ-007, REQ-008

Structure, and whether a free-text description renders — superseded, see
`openspec/specs/help-rendering/spec.md`.

### DES-022: Format Entry Component
**Traces to:** REQ-007

Whether this section documents CLI value-placeholder syntax or output formats, and its
rendered structure — superseded, see `openspec/specs/help-rendering/spec.md`.

### DES-023: Subcommands Entry Component
**Traces to:** REQ-007, REQ-012

**Structure:**
```
  <COMMAND>    <DESCRIPTION>
```

**Properties:**
- Indentation: 2 spaces
- Command name: Plain text
- Description: Plain text, aligned with padding
- Position: Before Examples section (REQ-012, REQ-017)

**Omission rule:** Only appears for commands with actual subcommands (REQ-021, REQ-024).

## Screens (Help Pages)

### DES-024: Root Help Screen
**Traces to:** REQ-007

**Command:** `targ --help` (as the build-tool CLI, `internal/runner`'s `printMultiModuleHelp` —
this is what real `targ` users see; it does not go through the `internal/help` ContentBuilder
used for `--create --help` and target-level help)

**Structure (live, this repo):**
```
targ is a build-tool runner that discovers tagged commands and executes them.

Usage: targ [FLAGS...] COMMAND [COMMAND_ARGS...]

Commands:
    check                     Run all checks & fixes
    ...
    watch                     Watch and re-run checks

Flags:
    --completion                 Generate shell completion script
    --help, -h                   Show help
    --source, -s                 Use targ files from specified directory
    --timeout                    Set execution timeout
    --parallel, -p                Run multiple targets concurrently
    ...
    --create                     Create a new target
    --sync                       Sync targets from a remote package
    --to-func                    Convert string target to function
    --to-string                  Convert function target to string

More info: https://github.com/toejough/targ#readme
```

Differences from the structure this doc originally specified: the description text and
usage-line shape are both different; flags render as one flat list (no Global/Command
split — `--source` sits among the general flags rather than under a distinct "Root only"
grouping); there is no `Formats:` section and no `Examples:` section here (unlike
`--create --help` and target-level help); Commands are grouped by source only when the
project is actually multi-module.

A **second, structurally different** root-help renderer exists for library-mode standalone
binaries (`targ.Main(targets...)`, via `internal/help.WriteRootHelp`). Live-verified from a
throwaway binary registering two targets:
```
Usage:
  myapp [flags...] [<command>...]

Flags:
  --help, -h                  Show help
  --completion {bash|zsh|fish}  Generate shell completion script

Commands:

  Source: main.go
  build
  test

Examples:
  Run a command:
    myapp build
```
Binary mode hides targ-only flags (only `--help`/`--completion` show), Commands render
under a `Source:` sub-header, and only 1 auto-generated example appears here (2 is what a
synthetic `WriteRootHelp` unit test with `TargFlagFilter{IsRoot: true}` — a different,
non-binary-mode configuration — produces). Which renderer and which flag-visibility mode
apply depends on how the binary was built and invoked, not on the flag typed — this doc
did not distinguish any of this.

**Node ID:** N/A (text output only, no .pen file)

### DES-025: --create Help Screen
**Traces to:** REQ-007, REQ-037

**Command:** `targ --create --help`

**Current implementation:** See `/Users/joe/repos/personal/targ/internal/runner/runner.go:1691-1716`

**Structure:** Already follows the design pattern:
- Description (line 1)
- Blank line
- Usage section
- Positionals section
- Flags section (command-specific flags only)
- Examples section (3 examples, progressive complexity)

**Styling to apply:**
- Section headers: Bold
- Flag names: Cyan
- Placeholders: Yellow
- Format names: N/A (no formats for --create)

**Node ID:** N/A (text output only, no .pen file)

### DES-026: --sync Help Screen
**Traces to:** REQ-007, REQ-033

**Command:** `targ --sync --help`

**Current implementation:** See `/Users/joe/repos/personal/targ/internal/runner/runner.go:1719-1728`

**Structure:** Already follows the design pattern:
- Description (line 1)
- Blank line
- Usage section with placeholder
- Examples section (2 examples)

**Sections present:** Usage, Examples
**Sections omitted:** Positionals (no positional args beyond package-path), Flags (no command-specific flags)

**Styling to apply:**
- Section headers: Bold
- Placeholders: Yellow

**Node ID:** N/A (text output only, no .pen file)

### DES-027: --to-func Help Screen
**Traces to:** REQ-007

**Command:** `targ --to-func --help`

**Current implementation:** See `/Users/joe/repos/personal/targ/internal/runner/runner.go:1731-1740`

**Structure:** Already follows the design pattern:
- Description (line 1)
- Blank line
- Usage section with placeholder
- Examples section (2 examples)

**Sections present:** Usage, Examples
**Sections omitted:** Positionals, Flags (no command-specific flags)

**Styling to apply:**
- Section headers: Bold
- Placeholders: Yellow

**Node ID:** N/A (text output only, no .pen file)

### DES-028: --to-string Help Screen
**Traces to:** REQ-007

**Command:** `targ --to-string --help`

**Current implementation:** See `/Users/joe/repos/personal/targ/internal/runner/runner.go:1743-1752`

**Structure:** Already follows the design pattern:
- Description (line 1)
- Blank line
- Usage section with placeholder
- Examples section (2 examples)

**Sections present:** Usage, Examples
**Sections omitted:** Positionals, Flags (no command-specific flags)

**Styling to apply:**
- Section headers: Bold
- Placeholders: Yellow

**Node ID:** N/A (text output only, no .pen file)

### DES-029: Target Execution Help

**Traces to:** REQ-007, REQ-053, REQ-061

**Command:** `targ <target> --help`

This ships — the "(Future)" framing was stale — and additionally renders `Source:`,
`Formats:`, `Execution:` (when the target has deps/cache/watch/retry configured), and a
`More info:` trailer that this entry originally didn't account for.

**Structure (live, `targ status --help` in this repo):**
```
Source: github.com/toejough/targ/dev

Usage:
  targ [targ flags...] status

Global flags:
  Global:
    --help, -h                Show help
    --timeout <duration>      Set execution timeout
    --parallel, -p            Run multiple targets concurrently
    --times <n>                Run the command n times
    --retry                   Continue on failure
    --backoff <duration,mult> Exponential backoff
    --watch <glob>             Re-run on file changes (repeatable)
    --cache <glob>             Skip if files unchanged (repeatable)
    --while <cmd>              Run while shell command succeeds
    --dep-mode {serial|parallel}  Dependency mode

Formats:
  <duration>  time value like 30s, 5m, 1h
  <duration,mult>  duration and multiplier like 1s,2.0
  <glob>  glob pattern like **/*.go, src/**

Examples:
  Basic usage:
    targ status

More info:
  https://github.com/toejough/targ
```

**Sections present here:** Source, Usage, Global flags (nested `Global:`/`Root only:`
subsections), Formats, Examples, More info. An `Execution:` section (Deps/Cache/Retry/etc.,
see `first-gen/architecture.md` ARCH-007) additionally renders for targets that configure it.
**Sections omitted here:** command-specific Flags (this target takes no args), Positionals
(none for this target), Subcommands (leaf target).

**Node ID:** N/A (text output only, no .pen file)

## Implementation Mapping

### DES-030: Flag Registry Integration
**Traces to:** REQ-007, REQ-008

**Source:** `/Users/joe/repos/personal/targ/internal/flags/flags.go`

**Design decision:** The `flags.Def` struct serves as the single source of truth for all flag metadata:

```go
type Def struct {
    Long       string // Flag name for display (without "--")
    Short      string // Short flag (without "-")
    Desc       string // Description text
    TakesValue bool   // Determines placeholder display
    RootOnly   bool   // Determines Global vs Command categorization
    Hidden     bool   // Excluded from help
    Removed    string // Error message for removed flags
}
```

**Rendering logic:**
1. `GlobalFlags()` returns all flags with `RootOnly == false`
2. `RootOnlyFlags()` returns all flags with `RootOnly == true`
3. Help renderer uses these functions to populate flag sections — actual section/subsection names: see `openspec/specs/help-rendering/spec.md`
4. `TakesValue` determines whether to show `<placeholder>` after flag name
5. `Desc` provides description text

**Enforcement:** All help output MUST derive from `flags.All()` registry. Manual flag documentation is prohibited.

### DES-031: Help Validation Testing
**Traces to:** REQ-007

**Source:** `/Users/joe/repos/personal/targ/internal/runner/runner_help_test.go`

**Design decision:** Property-based tests validate structural invariants:

```go
type helpSpec struct {
    command        string // Flag being documented
    hasPositionals bool   // Expect Positionals section
    hasFlags       bool   // Expect Flags section
}

func validateHelpOutput(g Gomega, output string, spec helpSpec) {
    // Validates:
    // - Non-empty description
    // - Correct section ordering
    // - No trailing whitespace
    // - Examples start with "targ"
    // - Flag lines start with "--"
}
```

**Coverage:** Tests exist for `--create`, `--sync`, `--to-func`, `--to-string`. Future commands must add similar tests.

**Enforcement:** CI runs these tests; PRs with failing help tests are blocked.

### DES-032: Help Rendering Architecture
**Traces to:** REQ-007

~~**Design decision:** Structured help builder (future implementation), with compile-time
enforcement of section ordering.~~ **Removed — shipped differently.** The actual
implementation is `internal/help.ContentBuilder`; section order is enforced at runtime
inside `Render()`, not at compile time. See `openspec/specs/help-rendering/spec.md`.

## Design Rules

### DR-001: Section Order Invariant
**Traces to:** REQ-007

The actual section order — superseded, see `openspec/specs/help-rendering/spec.md` (see also DES-015).

**Enforcement:** Property tests validate section index ordering. `ContentBuilder.Render()` enforces this order in code, not via a compile-time type-state guarantee.

### DR-002: Flag Subsection Order
**Traces to:** REQ-007

Superseded — see `openspec/specs/help-rendering/spec.md` for the actual flag section/subsection structure and ordering.

**Rationale:** Users need to understand global context before command-specific options.

### DR-003: Section Omission
**Traces to:** REQ-007

**Rule:** Empty sections MUST be omitted entirely. Do not render section headers with no content.

**Enforcement:**
- No Positionals header if no positional args
- No command-specific `Flags:` section if the command has no command-specific flags (REQ-019)
- No Formats section if command doesn't use formats (REQ-020)
- No Subcommands section if command has no subcommands (REQ-021)

### DR-004: Example Requirements
**Traces to:** REQ-007

**Rule:** Every command MUST have 2-3 examples showing progressive complexity. Minimum 1 example is acceptable.

**Requirements:**
- Examples are command-specific (not generic flag tutorials)
- First example shows simplest/most common use case
- Later examples show advanced features
- All examples are runnable commands

**Enforcement:** Code review checklist; help validation tests check for "Examples:" section.

### DR-005: Terminology Consistency
**Traces to:** REQ-007

**Standard terms:**
- "Flags" (not "Options")
- Actual section/subsection header names — superseded, see `openspec/specs/help-rendering/spec.md`
- "Usage:" (not "Syntax:")
- "Examples:" (not "Example Usage:")
- "Positionals:" (not "Arguments:")

**Enforcement:** Grep-based linting; property tests validate section header text.

### DR-006: Self-Contained Help
**Traces to:** REQ-007

**Rule:** Each command's help page should be self-contained. Users should not need to run `targ help formats` separately.

**Implementation:**
- Commands with format support include Formats section
- Formats section shows only relevant formats (REQ-027)
- Format descriptions are brief but sufficient

**Trade-off:** Some duplication between per-command help and `targ help formats`, but improves discoverability.

### DR-007: Styling Consistency
**Traces to:** REQ-007

**Rule:** All help output MUST use Rich styling as defined in DES-023.

**Application:**
- Section headers: Bold
- Flag names: Cyan
- Placeholders: Yellow
- Subsection headers: Bold
- Examples: Plain
- Format names: Yellow

**Implementation note:** Use ANSI escape codes; reset after each styled element to prevent bleed.

## Implementation Notes

**Removed 2026-07-28.** This section was a pre-implementation "next steps" plan (styling,
a `HelpBuilder` type, Global/Command Flags subsections, a Formats section, validation
tests) written before any of it existed. All of it has since shipped, none of it as
described here (see DES-032, DR-002, DR-005). It carried no historical decision-rationale
worth preserving as a plan — it's superseded in full by
`openspec/specs/help-rendering/spec.md` and the current `internal/help` implementation.

## Open Questions

None. All design decisions are finalized based on user's Rich styling preference.

## Summary

This design specification defines a complete visual and structural design for targ CLI help output using the Rich styling approach. The design prioritizes:

1. **Consistency:** All help pages follow the same structure and styling
2. **Discoverability:** Predictable section ordering helps users find information
3. **Enforcement:** Section ordering is enforced in code (runtime), not via compile-time guarantees — see DES-032
4. **Maintainability:** Single source of truth (flags registry) prevents drift
5. **Usability:** Self-contained help pages with clear visual hierarchy

Next phase: Task breakdown for implementation (TDD discipline).

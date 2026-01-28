# Help System Consistency - Design Specification

## Overview

This design specification defines the visual and structural design for targ CLI help output using the Rich styling approach. Every design element is assigned a DES-NNN traceability ID linked to upstream REQ-NNN requirements.

## Design System

### DES-001: Color Palette
**Traces to:** REQ-001 (structural consistency), REQ-002 (predictable locations), REQ-018 (consistent terminology)

The Rich styling approach uses ANSI color codes for terminal output:

- **Section Headers** (Usage:, Description:, Flags:, etc.): Bold white (`\x1b[1m`)
- **Flag Names** (--timeout, -h): Cyan (`\x1b[36m`)
- **Placeholders** (<duration>, <command>): Yellow (`\x1b[33m`)
- **Subsection Headers** (Global Flags:, Command Flags:): Bold (`\x1b[1m`)
- **Examples**: Plain text (no styling)
- **Format Names** (in Formats section): Yellow (`\x1b[33m`)
- **Reset**: (`\x1b[0m`) after each styled element

**Color accessibility:** All colors have sufficient contrast against standard terminal backgrounds (both light and dark modes). Bold text is used for headers to ensure visibility without color.

### DES-002: Typography Scale
**Traces to:** REQ-001 (structural consistency), REQ-006 (same structure)

Terminal output uses monospace fonts with the following hierarchy:

- **Section Headers**: Bold, title case with colon (e.g., "Usage:", "Flags:")
- **Subsection Headers**: Bold, title case with colon (e.g., "Global Flags:", "Command Flags:")
- **Body Text**: Regular weight, sentence case
- **Code Elements**: Inline, styled per DES-001 (flags in cyan, placeholders in yellow)
- **Indentation**: 2 spaces for section content, 4 spaces for nested content

### DES-003: Spacing System
**Traces to:** REQ-001 (structural consistency), REQ-012 (section order)

Vertical rhythm:

- **Between sections**: 1 blank line
- **Within sections**: No blank lines between items
- **After final section**: No trailing newline (clean output)

Horizontal spacing:

- **Base indentation**: 0 spaces for section headers
- **Content indentation**: 2 spaces from left margin
- **Flag descriptions**: Aligned after flag name with sufficient padding (varies by longest flag name)

### DES-004: Section Structure
**Traces to:** REQ-012 (exact section order), REQ-025 (section order invariant)

All help pages follow this canonical section order:

1. **Description** (first line, no header)
2. **Usage**
3. **Positionals** (if applicable)
4. **Flags** (with Global/Command subsections)
5. **Formats** (if applicable)
6. **Subcommands** (if applicable)
7. **Examples**

Sections are omitted if empty (REQ-019, REQ-020, REQ-021).

## Components

### DES-005: Section Header Component
**Traces to:** REQ-001 (structural consistency), REQ-018 (consistent terminology)

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

### DES-006: Subsection Header Component
**Traces to:** REQ-013 (Global/Command Flags subsections), REQ-026 (Global before Command invariant)

**Structure:**
```
  <BOLD><SUBSECTION_NAME>:</BOLD>
```

**Properties:**
- Indentation: 2 spaces
- Text style: Bold
- Trailing: Colon
- Line break: After header

**Variants:**
- Global Flags:
- Command Flags:

**Ordering rule:** Global Flags must always appear before Command Flags when both exist (REQ-026).

### DES-007: Flag Entry Component
**Traces to:** REQ-007 (distinguish global vs command-specific), REQ-013 (subsections)

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

### DES-008: Usage Line Component
**Traces to:** REQ-001 (structural consistency), REQ-012 (Usage section)

**Structure:**
```
Usage: targ <COMMAND_CONTEXT> [<CYAN>flags</CYAN>...]
```

**Properties:**
- Prefix: "Usage: targ "
- Command context: Command-specific (e.g., "--create [group...] <name>")
- Flag placeholders: Cyan colored
- Angle brackets: Yellow for required args
- Square brackets: Plain text for optional args

**Variants:**
- Root-only flags: `targ <FLAG> [args...]`
- Target execution: `targ [flags...] <target> [args...]`
- Subcommands: `targ <command> [flags...] [subcommand]`

### DES-009: Example Entry Component
**Traces to:** REQ-014 (2-3 examples), REQ-015 (command-specific), REQ-016 (command-specific examples)

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

### DES-010: Positionals Entry Component
**Traces to:** REQ-001 (structural consistency)

**Structure:**
```
  <NAME>    <DESCRIPTION>
```

**Properties:**
- Indentation: 2 spaces
- Name: Plain text (lowercase or kebab-case)
- Description: Plain text, aligned with padding
- Required vs optional: Indicated in description or usage line

**Example:**
```
Positionals:
  group            Optional group path components (e.g. "dev lint")
  name             Target name in kebab-case (e.g. "test", "check-all")
  shell-command    Shell command to execute (always last argument)
```

### DES-011: Format Entry Component
**Traces to:** REQ-014 (Formats section for relevant commands), REQ-027 (subset of formats), REQ-028 (document supported formats)

**Structure:**
```
  <YELLOW><FORMAT_NAME></YELLOW>    <DESCRIPTION>
```

**Properties:**
- Indentation: 2 spaces
- Format name: Yellow colored
- Description: Plain text, aligned with padding
- Only show formats relevant to current command (REQ-008, REQ-027)

**Example:**
```
Formats:
  json       Output as JSON
  yaml       Output as YAML
  plain      Plain text output (default)
```

### DES-012: Subcommands Entry Component
**Traces to:** REQ-017 (Subcommands section), REQ-021 (omit if none), REQ-024 (leaf nodes don't hint)

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

### DES-013: Root Help Screen
**Traces to:** REQ-022 (root command shows available commands)

**Command:** `targ --help` or `targ help`

**Structure:**
```
targ is a task runner for Go projects.

Usage: targ [flags...] [command] [args...]

Global Flags:
  --help, -h              Show help
  --source <dir>          Use targ files from specified directory
  --timeout <duration>    Set execution timeout
  --parallel, -p          Run multiple targets concurrently
  --times <n>             Run the command n times
  --retry                 Continue on failure
  --backoff <spec>        Exponential backoff (duration,multiplier)
  --watch <pattern>       Re-run on file changes (repeatable)
  --cache <pattern>       Skip if files unchanged (repeatable)
  --while <command>       Run while shell command succeeds
  --dep-mode <mode>       Dependency mode: serial or parallel
  --no-binary-cache       Disable binary caching

Command Flags:
  --create                Create a new target
  --sync <package>        Sync targets from a remote package
  --to-func <target>      Convert string target to function
  --to-string <target>    Convert function target to string
  --completion <shell>    Generate shell completion script

Examples:
  targ test
  targ --timeout 30s test
  targ --parallel test lint
```

**Node ID:** N/A (text output only, no .pen file)

### DES-014: --create Help Screen
**Traces to:** REQ-001, REQ-006, REQ-012, REQ-015

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

### DES-015: --sync Help Screen
**Traces to:** REQ-001, REQ-006, REQ-012, REQ-015

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

### DES-016: --to-func Help Screen
**Traces to:** REQ-001, REQ-006, REQ-012, REQ-015

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

### DES-017: --to-string Help Screen
**Traces to:** REQ-001, REQ-006, REQ-012, REQ-015

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

### DES-018: Target Execution Help (Future)
**Traces to:** REQ-001, REQ-006, REQ-007, REQ-013

**Command:** `targ help <target>` or `targ <target> --help`

**Structure:**
```
<TARGET_DESCRIPTION>

Usage: targ [flags...] <target-name> [args...]

Global Flags:
  --help, -h              Show help
  --timeout <duration>    Set execution timeout
  --parallel, -p          Run multiple targets concurrently
  --times <n>             Run the command n times
  --retry                 Continue on failure
  --backoff <spec>        Exponential backoff (duration,multiplier)
  --watch <pattern>       Re-run on file changes (repeatable)
  --cache <pattern>       Skip if files unchanged (repeatable)
  --while <command>       Run while shell command succeeds
  --dep-mode <mode>       Dependency mode: serial or parallel

Examples:
  targ <target-name>
  targ --timeout 30s <target-name>
```

**Sections present:** Description, Usage, Global Flags, Examples
**Sections omitted:** Command Flags (targets don't have command-specific flags), Positionals (handled in usage line), Formats (targets don't output structured data)

**Node ID:** N/A (text output only, no .pen file)

## Implementation Mapping

### DES-019: Flag Registry Integration
**Traces to:** REQ-003 (automatic conformance), REQ-004 (objective code review), REQ-010 (single source of truth)

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
3. Help renderer uses these functions to populate Global Flags and Command Flags subsections
4. `TakesValue` determines whether to show `<placeholder>` after flag name
5. `Desc` provides description text

**Enforcement:** All help output MUST derive from `flags.All` registry. Manual flag documentation is prohibited.

### DES-020: Help Validation Testing
**Traces to:** REQ-004 (objective code review), REQ-005 (self-contained help)

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

### DES-021: Help Rendering Architecture
**Traces to:** REQ-003 (automatic conformance), REQ-011 (easy path is right path)

**Design decision:** Structured help builder (future implementation):

```go
type HelpBuilder struct {
    description  string
    usage        string
    positionals  []PositionalArg
    globalFlags  []flags.Def // Auto-populated from flags.GlobalFlags()
    cmdFlags     []flags.Def // Auto-populated from context
    formats      []Format     // Optional
    subcommands  []Subcommand // Optional
    examples     []string
}

func (h *HelpBuilder) Render() string {
    // Enforces canonical section order
    // Applies Rich styling
    // Omits empty sections
}
```

**Benefits:**
- Impossible to render sections out of order (compile-time enforcement)
- Automatic flag population from registry (prevents drift)
- Styling applied consistently (single rendering implementation)
- Easy to use (declarative API)

**Migration path:** Refactor `PrintCreateHelp`, `PrintSyncHelp`, etc. to use `HelpBuilder`.

## Design Rules

### DR-001: Section Order Invariant
**Traces to:** REQ-012, REQ-025

**Rule:** All help pages MUST render sections in this exact order:

1. Description (no header)
2. Usage:
3. Positionals: (if present)
4. Flags: (with subsections)
5. Formats: (if present)
6. Subcommands: (if present)
7. Examples:

**Enforcement:** Property tests validate section index ordering. `HelpBuilder` enforces at compile time.

### DR-002: Flag Subsection Order
**Traces to:** REQ-013, REQ-026

**Rule:** When both Global Flags and Command Flags exist, Global Flags MUST appear first.

**Rationale:** Users need to understand global context before command-specific options.

**Enforcement:** `HelpBuilder.Render()` always emits Global Flags before Command Flags.

### DR-003: Section Omission
**Traces to:** REQ-019, REQ-020, REQ-021

**Rule:** Empty sections MUST be omitted entirely. Do not render section headers with no content.

**Enforcement:**
- No Positionals header if no positional args
- No Command Flags subsection if only global flags exist (REQ-019)
- No Formats section if command doesn't use formats (REQ-020)
- No Subcommands section if command has no subcommands (REQ-021)

### DR-004: Example Requirements
**Traces to:** REQ-014, REQ-015, REQ-016, REQ-023

**Rule:** Every command MUST have 2-3 examples showing progressive complexity. Minimum 1 example is acceptable.

**Requirements:**
- Examples are command-specific (not generic flag tutorials)
- First example shows simplest/most common use case
- Later examples show advanced features
- All examples are runnable commands

**Enforcement:** Code review checklist; help validation tests check for "Examples:" section.

### DR-005: Terminology Consistency
**Traces to:** REQ-018

**Standard terms:**
- "Flags" (not "Options")
- "Global Flags" / "Command Flags" (not "Common Flags" / "Specific Flags")
- "Usage:" (not "Syntax:")
- "Examples:" (not "Example Usage:")
- "Positionals:" (not "Arguments:")

**Enforcement:** Grep-based linting; property tests validate section header text.

### DR-006: Self-Contained Help
**Traces to:** REQ-005, REQ-008

**Rule:** Each command's help page should be self-contained. Users should not need to run `targ help formats` separately.

**Implementation:**
- Commands with format support include Formats section
- Formats section shows only relevant formats (REQ-027)
- Format descriptions are brief but sufficient

**Trade-off:** Some duplication between per-command help and `targ help formats`, but improves discoverability.

### DR-007: Styling Consistency
**Traces to:** REQ-001, REQ-002

**Rule:** All help output MUST use Rich styling as defined in DES-001.

**Application:**
- Section headers: Bold
- Flag names: Cyan
- Placeholders: Yellow
- Subsection headers: Bold
- Examples: Plain
- Format names: Yellow

**Implementation note:** Use ANSI escape codes; reset after each styled element to prevent bleed.

## Traceability Matrix

| Design ID | Design Element | Requirements Traced |
|-----------|----------------|---------------------|
| DES-001 | Color Palette | REQ-001, REQ-002, REQ-018 |
| DES-002 | Typography Scale | REQ-001, REQ-006 |
| DES-003 | Spacing System | REQ-001, REQ-012 |
| DES-004 | Section Structure | REQ-012, REQ-025 |
| DES-005 | Section Header Component | REQ-001, REQ-018 |
| DES-006 | Subsection Header Component | REQ-013, REQ-026 |
| DES-007 | Flag Entry Component | REQ-007, REQ-013 |
| DES-008 | Usage Line Component | REQ-001, REQ-012 |
| DES-009 | Example Entry Component | REQ-009, REQ-014, REQ-015, REQ-016 |
| DES-010 | Positionals Entry Component | REQ-001 |
| DES-011 | Format Entry Component | REQ-014, REQ-027, REQ-028 |
| DES-012 | Subcommands Entry Component | REQ-017, REQ-021, REQ-024 |
| DES-013 | Root Help Screen | REQ-022 |
| DES-014 | --create Help Screen | REQ-001, REQ-006, REQ-012, REQ-015 |
| DES-015 | --sync Help Screen | REQ-001, REQ-006, REQ-012, REQ-015 |
| DES-016 | --to-func Help Screen | REQ-001, REQ-006, REQ-012, REQ-015 |
| DES-017 | --to-string Help Screen | REQ-001, REQ-006, REQ-012, REQ-015 |
| DES-018 | Target Execution Help | REQ-001, REQ-006, REQ-007, REQ-013 |
| DES-019 | Flag Registry Integration | REQ-003, REQ-004, REQ-010 |
| DES-020 | Help Validation Testing | REQ-004, REQ-005 |
| DES-021 | Help Rendering Architecture | REQ-003, REQ-011 |
| DR-001 | Section Order Invariant | REQ-012, REQ-025 |
| DR-002 | Flag Subsection Order | REQ-013, REQ-026 |
| DR-003 | Section Omission | REQ-019, REQ-020, REQ-021 |
| DR-004 | Example Requirements | REQ-014, REQ-015, REQ-016, REQ-023 |
| DR-005 | Terminology Consistency | REQ-018 |
| DR-006 | Self-Contained Help | REQ-005, REQ-008 |
| DR-007 | Styling Consistency | REQ-001, REQ-002 |

## Requirements Coverage

All requirements from `requirements.md` are addressed:

### Success Criteria
- ✅ SC-01 (REQ-001): Covered by DES-004, DR-001, DR-007
- ✅ SC-02 (REQ-002): Covered by DES-001, DES-003, DR-007
- ✅ SC-03 (REQ-003): Covered by DES-019, DES-021
- ✅ SC-04 (REQ-004): Covered by DES-019, DES-020
- ✅ SC-05 (REQ-005): Covered by DES-020, DR-006

### User Stories
- ✅ US-01 (REQ-006): Covered by DES-002, DES-014-018
- ✅ US-02 (REQ-007): Covered by DES-007, DES-018
- ✅ US-03 (REQ-008): Covered by DR-006
- ✅ US-04 (REQ-009): Covered by DES-009, DR-004
- ✅ US-05 (REQ-010): Covered by DES-019
- ✅ US-06 (REQ-011): Covered by DES-021

### Acceptance Criteria
- ✅ AC-01 (REQ-012): Covered by DES-004, DR-001
- ✅ AC-02 (REQ-013): Covered by DES-006, DR-002
- ✅ AC-03 (REQ-014): Covered by DES-011, DR-004
- ✅ AC-04 (REQ-015): Covered by DES-009, DR-004
- ✅ AC-05 (REQ-016): Covered by DES-009, DR-004
- ✅ AC-06 (REQ-017): Covered by DES-012
- ✅ AC-07 (REQ-018): Covered by DES-005, DR-005

### Edge Cases
- ✅ REQ-019: Covered by DR-003
- ✅ REQ-020: Covered by DR-003
- ✅ REQ-021: Covered by DR-003, DES-012

### Boundary Conditions
- ✅ REQ-022: Covered by DES-013
- ✅ REQ-023: Covered by DR-004
- ✅ REQ-024: Covered by DES-012

### Invariants
- ✅ INV-01 (REQ-025): Covered by DES-004, DR-001
- ✅ INV-02 (REQ-026): Covered by DES-006, DR-002
- ✅ INV-03 (REQ-027): Covered by DES-011
- ✅ INV-04 (REQ-028): Covered by DES-011

## Implementation Notes

### Current State Assessment

The existing help functions (`PrintCreateHelp`, `PrintSyncHelp`, `PrintToFuncHelp`, `PrintToStringHelp`) already follow the correct structural pattern:
- Description first (no header)
- Blank line
- Usage section
- Positionals section (where applicable)
- Flags section (where applicable)
- Examples section

**Gap:** Styling is not applied. All text is plain; no ANSI color codes are used.

### Next Steps for Implementation

1. **Add styling to existing help functions** (Quick win):
   - Wrap section headers in bold: `\x1b[1m` + text + `:\x1b[0m`
   - Wrap flag names in cyan: `\x1b[36m--flag\x1b[0m`
   - Wrap placeholders in yellow: `\x1b[33m<placeholder>\x1b[0m`

2. **Implement HelpBuilder** (Structural improvement):
   - Create `internal/help` package
   - Implement `HelpBuilder` struct with declarative API
   - Migrate existing help functions to use `HelpBuilder`
   - Ensures compile-time enforcement of section ordering

3. **Add Global vs Command Flags subsections** (Architectural change):
   - Modify help functions to query `flags.GlobalFlags()` and `flags.RootOnlyFlags()`
   - Render "Global Flags:" and "Command Flags:" subsections
   - Applies to root help and any command with both flag types

4. **Implement Formats section** (Future work):
   - Add Formats section to commands with output (e.g., `targ list`, `targ describe`)
   - Populate from format registry (similar to flags registry)

5. **Add comprehensive help validation tests** (Quality assurance):
   - Extend `validateHelpOutput` to check for ANSI codes
   - Add property tests for Rich styling consistency
   - Validate color code pairing (every `\x1b[Xm` has matching `\x1b[0m`)

## Open Questions

None. All design decisions are finalized based on user's Rich styling preference.

## Summary

This design specification defines a complete visual and structural design for targ CLI help output using the Rich styling approach. All 28 requirements are covered with traceability. The design prioritizes:

1. **Consistency:** All help pages follow the same structure and styling
2. **Discoverability:** Predictable section ordering helps users find information
3. **Enforcement:** Compile-time guarantees prevent inconsistencies
4. **Maintainability:** Single source of truth (flags registry) prevents drift
5. **Usability:** Self-contained help pages with clear visual hierarchy

Next phase: Task breakdown for implementation (TDD discipline).

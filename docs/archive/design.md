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

Commands are displayed with their source file location:
```
  Source: dev/targets.go
  check    Run all checks & fixes
  lint     Lint codebase
```

This helps users understand where targets are defined.

### DES-004: Path Stack Syntax

**Traces to:** REQ-024, REQ-025, REQ-029, REQ-030

Path traversal uses a stack-based syntax:
- Words traverse into groups until reaching a target
- After hitting a target, next word continues from current group level
- `--` resets to root for accessing top-level targets after nested ones

```
targ dev build test     # dev/build, dev/test
targ dev build -- prod  # dev/build, then prod (at root)
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
- **Subsection Headers** (Global Flags:, Command Flags:): Bold (`\x1b[1m`)
- **Examples**: Plain text (no styling)
- **Format Names** (in Formats section): Yellow (`\x1b[33m`)
- **Reset**: (`\x1b[0m`) after each styled element

**Color accessibility:** All colors have sufficient contrast against standard terminal backgrounds (both light and dark modes). Bold text is used for headers to ensure visibility without color.

### DES-013: Typography Scale
**Traces to:** REQ-007

Terminal output uses monospace fonts with the following hierarchy:

- **Section Headers**: Bold, title case with colon (e.g., "Usage:", "Flags:")
- **Subsection Headers**: Bold, title case with colon (e.g., "Global Flags:", "Command Flags:")
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

**Variants:**
- Global Flags:
- Command Flags:

**Ordering rule:** Global Flags must always appear before Command Flags when both exist.

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

### DES-022: Format Entry Component
**Traces to:** REQ-007

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

### DES-029: Target Execution Help (Future)
**Traces to:** REQ-007, REQ-053, REQ-061

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
3. Help renderer uses these functions to populate Global Flags and Command Flags subsections
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
**Traces to:** REQ-007

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
**Traces to:** REQ-007

**Rule:** When both Global Flags and Command Flags exist, Global Flags MUST appear first.

**Rationale:** Users need to understand global context before command-specific options.

**Enforcement:** `HelpBuilder.Render()` always emits Global Flags before Command Flags.

### DR-003: Section Omission
**Traces to:** REQ-007

**Rule:** Empty sections MUST be omitted entirely. Do not render section headers with no content.

**Enforcement:**
- No Positionals header if no positional args
- No Command Flags subsection if only global flags exist (REQ-019)
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
- "Global Flags" / "Command Flags" (not "Common Flags" / "Specific Flags")
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

This design specification defines a complete visual and structural design for targ CLI help output using the Rich styling approach. The design prioritizes:

1. **Consistency:** All help pages follow the same structure and styling
2. **Discoverability:** Predictable section ordering helps users find information
3. **Enforcement:** Compile-time guarantees prevent inconsistencies
4. **Maintainability:** Single source of truth (flags registry) prevents drift
5. **Usability:** Self-contained help pages with clear visual hierarchy

Next phase: Task breakdown for implementation (TDD discipline).

# Help Output Consistency - Product Specification

## Problem Statement

Command help output in targ is inconsistent across commands. Some commands show global flags, some don't. Examples vary wildly - some commands have 10+ examples showing every possible flag combination, others have none. Format descriptions appear inconsistently. This makes the CLI harder to learn and creates maintenance burden.

### Who is affected
- CLI users trying to understand command usage
- Maintainers adding new commands or flags
- Documentation readers expecting consistent patterns

### Impact
If we don't solve this:
- Users waste time searching through verbose help or missing information
- New commands perpetuate inconsistent patterns
- Help text maintenance becomes ad-hoc rather than systematic

## Current State

### User Journey
1. User types `targ <command> --help`
2. Sees help output with unpredictable structure
3. May or may not see global flags
4. May see 0 examples or 10+ examples
5. May or may not see format descriptions
6. Has to learn each command's help format separately

### Pain Points
- Global flags appear inconsistently (sometimes shown, sometimes omitted)
- Examples range from absent to exhaustive (some commands have 10+ examples)
- Format descriptions duplicated or missing
- No clear standard for what help should contain
- Subcommand help structure differs from leaf command help

### Constraints
- Using Kong CLI framework for parsing
- Help generation must work for both leaf commands and commands with subcommands
- Must support both global flags (defined at root) and command-specific flags
- Some commands use format flags (--format), others don't

## Desired Future State

### Success Criteria
- **SC-01 (REQ-001):** Every command's help output follows the same structural template
- **SC-02 (REQ-002):** Users can predict what sections will appear in help output
- **SC-03 (REQ-003):** Help output contains just enough information to use the command effectively, nothing more
- **SC-04 (REQ-004):** Maintainers can add new commands without needing to decide help structure

### User Stories
- **US-01 (REQ-005):** As a CLI user, I want to see command usage syntax first, so I understand the basic invocation pattern
- **US-02 (REQ-006):** As a CLI user, I want to see what flags are available, so I know how to modify command behavior
- **US-03 (REQ-007):** As a CLI user, I want to see 2-3 examples showing progressive complexity, so I can understand common usage patterns without being overwhelmed
- **US-04 (REQ-008):** As a CLI user, I want format descriptions only when the command uses format flags, so I don't see irrelevant information
- **US-05 (REQ-009):** As a CLI maintainer, I want a clear template to follow, so I don't have to make structural decisions for each command

### Acceptance Criteria

#### Structure
- **AC-01 (REQ-010):** Help output has exactly these sections in order: Usage, Description, Flags, Formats (conditional), Subcommands (conditional), Examples
- **AC-02 (REQ-011):** Usage line shows command path and argument patterns (e.g., `targ issues update <id> [flags]`)
- **AC-03 (REQ-012):** Description is a concise one-liner explaining command purpose
- **AC-04 (REQ-013):** Flags section shows Global Flags and Command Flags as separate labeled groups when both exist
- **AC-05 (REQ-014):** Flags section shows only "Flags:" header when no global flags exist
- **AC-06 (REQ-015):** Formats section appears only if command has format-related flags (--format, --output, etc.)
- **AC-07 (REQ-016):** Formats section shows only formats applicable to this command's flags
- **AC-08 (REQ-017):** Subcommands section appears only for commands that have subcommands
- **AC-09 (REQ-018):** Examples section contains 2-3 examples showing progressive complexity

#### Examples Content
- **AC-10 (REQ-019):** First example shows basic usage with no flags
- **AC-11 (REQ-020):** Subsequent examples (1-2) show common flag variations
- **AC-12 (REQ-021):** Examples are command-specific, not generic demonstrations of every flag
- **AC-13 (REQ-022):** Each example includes both the command and a brief comment explaining what it does

#### Global Flags Handling
- **AC-14 (REQ-023):** Global flags appear in the Flags section under "Global Flags:" label when command has command-specific flags
- **AC-15 (REQ-024):** Global flags appear in the Flags section under just "Flags:" when command has no command-specific flags
- **AC-16 (REQ-025):** Global flags include: --help, --version, --quiet, --verbose, --no-color

## Edge Cases

### Error Scenarios
- **REQ-026:** Command with no flags shows only Usage, Description, Examples sections
- **REQ-027:** Command with subcommands but no examples shows Subcommands section last
- **REQ-028:** Command with only global flags shows "Flags:" not "Global Flags:"

### Boundary Conditions
- **REQ-029:** Example count is always 2-3, never 0, never 10+
- **REQ-030:** Format descriptions appear exactly once per format type, not duplicated per flag
- **REQ-031:** Section ordering is strict and never varies by command

### Invariants
- **INV-01 (REQ-032):** Usage section always appears first
- **INV-02 (REQ-033):** Description always appears second
- **INV-03 (REQ-034):** Examples (when present) always appear last
- **INV-04 (REQ-035):** Sections never appear out of order
- **INV-05 (REQ-036):** No command shows more than one "Formats:" section

## Solution Guidance

### Current Implementation Context
- Kong CLI framework generates most help structure automatically
- Custom help generation happens in `internal/core/command.go` (HelpRenderer)
- Format descriptions are defined in `internal/flags/format.go`
- Examples are embedded in command Help fields
- Global vs command-specific flag distinction exists in Kong's model

### Approaches to Consider
1. Centralize help template logic in one place that all commands use
2. Create example count validation (enforce 2-3 range)
3. Audit existing commands and update to match template
4. Add tests that verify help structure compliance
5. Document template for future command authors

### Approaches to Avoid
- Don't make examples auto-generated - they need human curation for relevance
- Don't try to fix this piecemeal - define standard first, then bulk update
- Don't add more sections than specified - resist scope creep in help output

### Constraints
- Must work within Kong's help generation framework
- Can't break existing command functionality
- Changes should be implementable in one systematic pass
- Must maintain backward compatibility for scripts parsing help (if any exist)

### References
- Current inconsistent help visible in: `targ issues --help` (has examples), `targ dev check --help` (verbose), `targ sync register --help` (minimal)
- Kong help customization docs: https://github.com/alecthomas/kong#help

## Open Questions
- Q1: Are there any scripts or tools that parse help output and would break with structure changes?
- Q2: Should we validate example count in tests, or just document the guideline?
- Q3: Do we need format descriptions for formats that are self-explanatory (e.g., "json")?
- Q4: Should help output change based on terminal width, or always use fixed formatting?

## Traceability Summary
- Requirements assigned: REQ-001 through REQ-036
- Success Criteria: 4 (REQ-001 to REQ-004)
- User Stories: 5 (REQ-005 to REQ-009)
- Acceptance Criteria: 16 (REQ-010 to REQ-025)
- Edge Cases: 3 (REQ-026 to REQ-028)
- Boundary Conditions: 3 (REQ-029 to REQ-031)
- Invariants: 5 (REQ-032 to REQ-036)

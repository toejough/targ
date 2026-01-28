# Help System Consistency - Product Specification

## Problem Statement

The targ CLI help system has evolved in an ad-hoc, case-by-case manner, resulting in inconsistent structure, terminology, and formatting across commands. This creates a poor user experience where help pages look different depending on which command you're viewing, and maintainers must make subjective decisions about how to structure new help content. The current approach is reactive ("whack-a-mole") rather than systemic.

### Who is affected

- **End users**: Experience inconsistent help structure and terminology, making it harder to learn the tool
- **Maintainers**: Lack clear guidelines for writing help, leading to inconsistent implementations
- **Future contributors**: No obvious "right way" to structure new command help

### Impact

Without solving this:
- Help inconsistency will grow as new commands are added
- Users must learn different help patterns for different commands
- Code reviews require subjective judgment on help structure
- Technical debt accumulates in the help system

## Current State

### User Journey

1. User runs `targ <command> --help` or `targ help <command>`
2. System displays help page for that command
3. User reads help to understand command behavior
4. User struggles with:
   - Finding relevant information (sections in different orders)
   - Understanding flag relationships (global vs command-specific unclear)
   - Discovering available formats (scattered or omitted)
   - Learning from examples (inconsistent presence/quality)

### Pain Points

- Help structure varies across commands (sections present, absent, or in different orders)
- No clear distinction between global flags and command-specific flags
- Format documentation is inconsistent (some commands document all formats, others omit)
- Examples range from none to many, with varying quality
- Terminology inconsistency (e.g., "Options" vs "Flags")
- No authoritative reference for "how to write help"

### Constraints

- Help must remain embeddable in binary (no external file dependencies at runtime)
- Must work with existing cobra command framework
- Cannot break existing command-line interface
- Must be maintainable by multiple contributors

## Desired Future State

### Success Criteria

- **SC-01 (REQ-001):** All command help pages follow the same structural pattern (sections in consistent order)
- **SC-02 (REQ-002):** Users can find information in predictable locations across all commands
- **SC-03 (REQ-003):** New commands automatically conform to help structure without manual effort
- **SC-04 (REQ-004):** Code review can reject help inconsistencies objectively based on documented rules
- **SC-05 (REQ-005):** Help pages are self-contained (no need to run `targ help formats` separately)

### User Stories

- **US-01 (REQ-006):** As a user, I want to see the same help structure for every command, so that I can quickly find the information I need
- **US-02 (REQ-007):** As a user, I want to see which flags are global vs command-specific, so that I understand what applies where
- **US-03 (REQ-008):** As a user, I want to see only the formats relevant to the current command, so that I'm not overwhelmed with irrelevant information
- **US-04 (REQ-009):** As a user, I want to see 2-3 examples per command showing progressive complexity, so that I can learn by doing
- **US-05 (REQ-010):** As a maintainer, I want a single source of truth for help structure, so that I don't make subjective decisions
- **US-06 (REQ-011):** As a maintainer, I want the right structure to be the easy path, so that inconsistencies don't creep in

### Acceptance Criteria

- **AC-01 (REQ-012):** Help pages display sections in this exact order: Usage → Description → Flags → Formats → Subcommands → Examples
- **AC-02 (REQ-013):** Flags section shows two subsections: "Global Flags" and "Command Flags" (when both exist)
- **AC-03 (REQ-014):** Formats section appears only for commands that use formats, and shows only relevant formats for that command
- **AC-04 (REQ-015):** Examples section contains 2-3 examples per command, showing progression from basic to advanced usage
- **AC-05 (REQ-016):** Examples are command-specific (not generic "here's how flags work" examples)
- **AC-06 (REQ-017):** Subcommands section appears only for commands that have subcommands, positioned before Examples
- **AC-07 (REQ-018):** All help text uses consistent terminology ("Flags" not "Options", etc.)

## Edge Cases

### Error Scenarios

- **REQ-019:** If a command has no command-specific flags, only show "Global Flags" section (no empty "Command Flags" section)
- **REQ-020:** If a command doesn't use formats, omit the Formats section entirely (don't show empty section)
- **REQ-021:** If a command has no subcommands, omit the Subcommands section (don't show empty section)

### Boundary Conditions

- **REQ-022:** Root command (`targ help`) shows available commands and global help structure
- **REQ-023:** Commands with only one example should still include Examples section (prefer 2-3, but 1 is acceptable)
- **REQ-024:** Commands that are leaf nodes (no subcommands) should not hint at subcommand structure

### Invariants

- **INV-01 (REQ-025):** Section order must always be: Usage → Description → Flags → Formats → Subcommands → Examples (omitting absent sections)
- **INV-02 (REQ-026):** Global Flags must always appear before Command Flags when both exist
- **INV-03 (REQ-027):** Format documentation in per-command help must be a subset of `targ help formats` content
- **INV-04 (REQ-028):** Every command with output must document which formats it supports

## Solution Guidance

### Approaches to Consider

- Helper functions or types that enforce help structure at compile-time
- Cobra customization that renders help sections in mandatory order
- Template-based help rendering with required sections
- Linting or testing that validates help structure consistency
- Code generation for help structure boilerplate

### Approaches to Avoid

- Manual string building for help content (too error-prone)
- Per-command custom help formatting (defeats consistency goal)
- Runtime configuration of help structure (want compile-time enforcement)
- External help files that could drift from code (must be embedded)

### Constraints

- **Non-negotiable:** Help structure must prevent inconsistency, not just document desired structure
- **Non-negotiable:** Solution must make the right way the easy way (minimal boilerplate per command)
- **Preferred:** Compile-time enforcement over runtime checks where possible
- **Preferred:** Single source of truth for section ordering and terminology

### References

- Current `internal/core/command.go` has runner.Help() structure
- cobra library's templating system for help rendering
- Existing `help` command at `cmd/targ/commands/help/help.go`

## Open Questions

1. Should format descriptions be duplicated in per-command help, or reference a shared source?
2. How should nested subcommands (e.g., `targ dev test`) structure their help hierarchy?
3. Should we version the help structure to allow future evolution?
4. Do we need migration tooling to update existing commands, or manual refactor?

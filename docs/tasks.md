# Implementation Tasks

## Phase 1: Foundation - Package Structure and Dependencies

### TASK-001: Add lipgloss dependency
**Status:** pending | **Attempts:** 0

**Description:** Add charmbracelet/lipgloss to go.mod for ANSI styling support.

**Acceptance Criteria:**
- [ ] `go get github.com/charmbracelet/lipgloss` executed successfully
- [ ] go.mod contains lipgloss dependency
- [ ] go.sum updated with lipgloss checksums
- [ ] `go mod tidy` completes without errors

**Files:** Modify: `go.mod`, `go.sum`
**Dependencies:** None
**Traces to:** REQ-001, REQ-002, DES-001, ARCH-002

---

### TASK-002: Create internal/help package structure
**Status:** pending | **Attempts:** 0

**Description:** Create the internal/help directory with placeholder files for the help system.

**Acceptance Criteria:**
- [ ] Directory `internal/help` exists
- [ ] File `internal/help/builder.go` created with package declaration
- [ ] File `internal/help/content.go` created with package declaration
- [ ] File `internal/help/render.go` created with package declaration
- [ ] File `internal/help/styles.go` created with package declaration
- [ ] File `internal/help/formats.go` created with package declaration
- [ ] All files have package comment describing their purpose

**Files:** Create: `internal/help/builder.go`, `internal/help/content.go`, `internal/help/render.go`, `internal/help/styles.go`, `internal/help/formats.go`
**Dependencies:** None
**Traces to:** ARCH-003, ARCH-007

---

## Phase 2: Data Models and Content Structures

### TASK-003: Define content data structures
**Status:** pending | **Attempts:** 0

**Description:** Implement the core data structures for help content in content.go (Positional, Flag, Format, Subcommand, Example).

**Acceptance Criteria:**
- [ ] `Positional` struct defined with Name, Placeholder, Required fields
- [ ] `Flag` struct defined with Long, Short, Desc, Placeholder, Required fields
- [ ] `Format` struct defined with Name, Desc fields
- [ ] `Subcommand` struct defined with Name, Desc fields
- [ ] `Example` struct defined with Title, Code fields
- [ ] All structs have godoc comments
- [ ] Property tests validate struct field types match schema

**Files:** Modify: `internal/help/content.go`
**Dependencies:** TASK-002
**Traces to:** REQ-001, REQ-012, ARCH-007

**Test Properties:**
- Struct fields have correct types
- Structs are exported (capitalized)
- Required/optional fields match specification

---

### TASK-004: Define ContentBuilder struct
**Status:** pending | **Attempts:** 0

**Description:** Implement ContentBuilder struct that holds all help content before rendering.

**Acceptance Criteria:**
- [ ] `ContentBuilder` struct defined with all content fields (commandName, description, usage, positionals, globalFlags, commandFlags, formats, subcommands, examples)
- [ ] All fields are unexported (lowercase)
- [ ] Struct has godoc comment explaining its purpose
- [ ] Property tests validate field types

**Files:** Modify: `internal/help/content.go`
**Dependencies:** TASK-003
**Traces to:** REQ-012, ARCH-007

**Test Properties:**
- All expected fields present
- Field types match corresponding slice/string types
- Struct is exported but fields are not

---

## Phase 3: Styling Infrastructure

### TASK-005: Define lipgloss style constants
**Status:** pending | **Attempts:** 0

**Description:** Define all lipgloss style constants for section headers, flags, placeholders, and other styled elements.

**Acceptance Criteria:**
- [ ] `sectionHeaderStyle` defined (bold)
- [ ] `subsectionHeaderStyle` defined (bold)
- [ ] `flagNameStyle` defined (cyan, ANSI color 6)
- [ ] `placeholderStyle` defined (yellow, ANSI color 3)
- [ ] `formatNameStyle` defined (yellow, ANSI color 3)
- [ ] `exampleStyle` defined (plain)
- [ ] All styles have godoc comments
- [ ] Property tests validate styles render with correct ANSI codes
- [ ] Property tests validate ANSI codes are properly paired (open/close)

**Files:** Modify: `internal/help/styles.go`
**Dependencies:** TASK-001
**Traces to:** REQ-001, REQ-002, DES-001, DES-007, ARCH-002

**Test Properties:**
- Each style renders with correct ANSI escape sequences
- Bold styles include `\x1b[1m`
- Color styles include appropriate `\x1b[3Xm` codes
- All styled strings end with `\x1b[0m` reset

---

## Phase 4: Type-State Builder API

### TASK-006: Implement Builder initialization
**Status:** pending | **Attempts:** 0

**Description:** Implement NewBuilder function and Builder type for help construction entry point.

**Acceptance Criteria:**
- [ ] `NewBuilder(commandName string)` function defined
- [ ] `Builder` struct defined with commandName field
- [ ] NewBuilder validates commandName is non-empty (panics if empty)
- [ ] Property tests validate panic on empty commandName
- [ ] Property tests validate successful creation with non-empty name

**Files:** Modify: `internal/help/builder.go`
**Dependencies:** TASK-004
**Traces to:** REQ-011, ARCH-001, ARCH-004

**Test Properties:**
- NewBuilder panics when commandName is empty string
- NewBuilder returns non-nil Builder for non-empty string
- Builder stores commandName correctly

---

### TASK-007: Implement WithDescription phase transition
**Status:** pending | **Attempts:** 0

**Description:** Implement WithDescription method that transitions from Builder to ContentBuilder.

**Acceptance Criteria:**
- [ ] `WithDescription(desc string) *ContentBuilder` method on Builder
- [ ] Method returns ContentBuilder with description and commandName set
- [ ] ContentBuilder is a new type (not Builder)
- [ ] Property tests validate description is stored
- [ ] Property tests validate commandName is carried over

**Files:** Modify: `internal/help/builder.go`
**Dependencies:** TASK-006
**Traces to:** REQ-001, ARCH-001, ARCH-004

**Test Properties:**
- WithDescription returns *ContentBuilder type
- Returned builder has description field set to input
- Returned builder has commandName from original Builder

---

### TASK-008: Implement WithUsage method
**Status:** pending | **Attempts:** 0

**Description:** Implement WithUsage method for setting custom usage line.

**Acceptance Criteria:**
- [ ] `WithUsage(usage string) *ContentBuilder` method on ContentBuilder
- [ ] Method returns same ContentBuilder (chainable)
- [ ] Property tests validate usage is stored
- [ ] Property tests validate method chaining works

**Files:** Modify: `internal/help/builder.go`
**Dependencies:** TASK-007
**Traces to:** REQ-001, REQ-012, ARCH-004, ARCH-004

**Test Properties:**
- WithUsage returns *ContentBuilder (chainable)
- Usage field is set to input string
- Method can be chained with other builder methods

---

### TASK-009: Implement AddPositionals method
**Status:** pending | **Attempts:** 0

**Description:** Implement AddPositionals method for adding positional arguments to help.

**Acceptance Criteria:**
- [ ] `AddPositionals(pos ...Positional) *ContentBuilder` method on ContentBuilder
- [ ] Method accepts variadic Positional arguments
- [ ] Method returns same ContentBuilder (chainable)
- [ ] Property tests validate positionals are stored
- [ ] Property tests validate multiple calls append (not replace)

**Files:** Modify: `internal/help/builder.go`
**Dependencies:** TASK-007, TASK-003
**Traces to:** REQ-001, REQ-012, ARCH-004, ARCH-004

**Test Properties:**
- AddPositionals returns *ContentBuilder (chainable)
- Positionals are appended to existing slice
- Multiple calls accumulate positionals
- Empty call is valid (no-op)

---

### TASK-010: Implement AddGlobalFlags method
**Status:** pending | **Attempts:** 0

**Description:** Implement AddGlobalFlags method that populates global flags from the flags registry.

**Acceptance Criteria:**
- [ ] `AddGlobalFlags(flagNames ...string) *ContentBuilder` method on ContentBuilder
- [ ] Method accepts variadic flag name strings
- [ ] Method queries `flags.Find()` for each name
- [ ] Method converts `flags.Def` to `help.Flag` format
- [ ] Property tests validate flags are stored
- [ ] Property tests validate integration with flags registry

**Files:** Modify: `internal/help/builder.go`
**Dependencies:** TASK-007, TASK-003
**Traces to:** REQ-007, REQ-010, REQ-013, ARCH-008, ARCH-004, ARCH-008

**Test Properties:**
- AddGlobalFlags returns *ContentBuilder (chainable)
- Flags from registry are correctly converted
- Method handles non-existent flag names gracefully
- Multiple calls accumulate flags

---

### TASK-011: Implement AddCommandFlags method
**Status:** pending | **Attempts:** 0

**Description:** Implement AddCommandFlags method for command-specific flags.

**Acceptance Criteria:**
- [ ] `AddCommandFlags(flags ...Flag) *ContentBuilder` method on ContentBuilder
- [ ] Method accepts variadic Flag arguments
- [ ] Method returns same ContentBuilder (chainable)
- [ ] Property tests validate flags are stored
- [ ] Property tests validate multiple calls append

**Files:** Modify: `internal/help/builder.go`
**Dependencies:** TASK-007, TASK-003
**Traces to:** REQ-007, REQ-013, ARCH-004, ARCH-004

**Test Properties:**
- AddCommandFlags returns *ContentBuilder (chainable)
- Command flags are stored separately from global flags
- Multiple calls accumulate flags
- Empty call is valid (no-op)

---

### TASK-012: Implement AddFormats method with validation
**Status:** pending | **Attempts:** 0

**Description:** Implement AddFormats method with format name validation against registry.

**Acceptance Criteria:**
- [ ] `AddFormats(formats ...Format) *ContentBuilder` method on ContentBuilder
- [ ] Method accepts variadic Format arguments
- [ ] Method returns same ContentBuilder (chainable)
- [ ] Property tests validate formats are stored
- [ ] Property tests validate multiple calls append

**Files:** Modify: `internal/help/builder.go`
**Dependencies:** TASK-007, TASK-003
**Traces to:** REQ-008, REQ-014, REQ-027, ARCH-007, ARCH-004

**Test Properties:**
- AddFormats returns *ContentBuilder (chainable)
- Formats are appended to existing slice
- Multiple calls accumulate formats
- Empty call is valid (no-op)

---

### TASK-013: Implement AddSubcommands method
**Status:** pending | **Attempts:** 0

**Description:** Implement AddSubcommands method for nested subcommands.

**Acceptance Criteria:**
- [ ] `AddSubcommands(subs ...Subcommand) *ContentBuilder` method on ContentBuilder
- [ ] Method accepts variadic Subcommand arguments
- [ ] Method returns same ContentBuilder (chainable)
- [ ] Property tests validate subcommands are stored
- [ ] Property tests validate multiple calls append

**Files:** Modify: `internal/help/builder.go`
**Dependencies:** TASK-007, TASK-003
**Traces to:** REQ-017, REQ-021, ARCH-009, ARCH-004

**Test Properties:**
- AddSubcommands returns *ContentBuilder (chainable)
- Subcommands are appended to existing slice
- Multiple calls accumulate subcommands
- Empty call is valid (no-op)

---

### TASK-014: Implement AddExamples method with validation
**Status:** pending | **Attempts:** 0

**Description:** Implement AddExamples method with minimum 1 example enforcement.

**Acceptance Criteria:**
- [ ] `AddExamples(examples ...Example) *ContentBuilder` method on ContentBuilder
- [ ] Method panics if called with zero examples
- [ ] Method accepts 1 or more Example arguments
- [ ] Method returns same ContentBuilder (chainable)
- [ ] Property tests validate panic on empty
- [ ] Property tests validate examples are stored (1-3 examples)

**Files:** Modify: `internal/help/builder.go`
**Dependencies:** TASK-007, TASK-003
**Traces to:** REQ-009, REQ-015, REQ-023, ARCH-009, ARCH-004

**Test Properties:**
- AddExamples panics when called with zero arguments
- AddExamples returns *ContentBuilder for 1+ examples
- Examples are stored in order
- Calling twice replaces previous examples (not append)

---

## Phase 5: Rendering Infrastructure

### TASK-015: Implement renderDescription method
**Status:** pending | **Attempts:** 0

**Description:** Implement description rendering (first line, no header).

**Acceptance Criteria:**
- [ ] `renderDescription() string` method in render.go
- [ ] Returns description text with newline
- [ ] No section header (no "Description:")
- [ ] Empty description returns empty string
- [ ] Property tests validate output format

**Files:** Modify: `internal/help/render.go`
**Dependencies:** TASK-004
**Traces to:** REQ-001, REQ-012, ARCH-005, ARCH-007

**Test Properties:**
- Non-empty description renders as single line with trailing newline
- Empty description returns empty string
- No "Description:" header present
- Exactly one newline at end

---

### TASK-016: Implement renderUsage method
**Status:** pending | **Attempts:** 0

**Description:** Implement Usage section rendering with bold header.

**Acceptance Criteria:**
- [ ] `renderUsage() string` method in render.go
- [ ] Returns "Usage:" header (bold, via sectionHeaderStyle) followed by usage line
- [ ] Header has colon and newline
- [ ] Usage line has proper indentation (2 spaces)
- [ ] Property tests validate ANSI codes and format

**Files:** Modify: `internal/help/render.go`
**Dependencies:** TASK-005, TASK-004
**Traces to:** REQ-001, REQ-012, DES-008, ARCH-005, ARCH-007

**Test Properties:**
- Output starts with bold ANSI code for "Usage:"
- Colon present after "Usage"
- Newline after header
- Usage line is indented 2 spaces

---

### TASK-017: Implement renderPositionals method
**Status:** pending | **Attempts:** 0

**Description:** Implement Positionals section rendering with omission logic.

**Acceptance Criteria:**
- [ ] `renderPositionals() string` method in render.go
- [ ] Returns empty string if no positionals (section omission)
- [ ] Returns "Positionals:" header (bold) when positionals exist
- [ ] Each positional rendered with name, placeholder, description
- [ ] 2-space section indent, proper alignment
- [ ] Property tests validate omission and formatting

**Files:** Modify: `internal/help/render.go`
**Dependencies:** TASK-005, TASK-004
**Traces to:** REQ-012, REQ-019, DES-010, ARCH-005, ARCH-006, ARCH-007

**Test Properties:**
- Empty positionals list returns empty string
- Non-empty list includes "Positionals:" header
- Each positional is indented 2 spaces
- Required/optional distinction is visible

---

### TASK-018: Implement renderFlags method with subsections
**Status:** pending | **Attempts:** 0

**Description:** Implement Flags section rendering with Global/Command subsections and omission logic.

**Acceptance Criteria:**
- [ ] `renderFlags() string` method in render.go
- [ ] Returns empty string if no flags (section omission)
- [ ] Returns "Flags:" header (bold) when flags exist
- [ ] Shows "Global Flags:" subsection (bold, indented) if global flags present
- [ ] Shows "Command Flags:" subsection (bold, indented) if command flags present
- [ ] Global subsection always before Command subsection
- [ ] Flag entries rendered with cyan flag names, yellow placeholders
- [ ] 4-space flag entry indent
- [ ] Property tests validate subsection ordering and omission

**Files:** Modify: `internal/help/render.go`
**Dependencies:** TASK-005, TASK-004
**Traces to:** REQ-007, REQ-013, REQ-019, DES-006, DES-007, ARCH-006, ARCH-008, ARCH-007

**Test Properties:**
- Empty flags returns empty string
- Global flags appear before Command flags
- Subsection headers are bold and indented 2 spaces
- Flag names are cyan
- Placeholders are yellow
- Proper ANSI code pairing

---

### TASK-019: Implement renderFormats method with omission
**Status:** pending | **Attempts:** 0

**Description:** Implement Formats section rendering with omission logic.

**Acceptance Criteria:**
- [ ] `renderFormats() string` method in render.go
- [ ] Returns empty string if no formats (section omission)
- [ ] Returns "Formats:" header (bold) when formats exist
- [ ] Format names rendered in yellow
- [ ] 2-space section indent
- [ ] Property tests validate omission and styling

**Files:** Modify: `internal/help/render.go`
**Dependencies:** TASK-005, TASK-004
**Traces to:** REQ-008, REQ-014, REQ-020, REQ-027, DES-011, ARCH-006, ARCH-007, ARCH-007

**Test Properties:**
- Empty formats returns empty string
- Non-empty formats includes "Formats:" header
- Format names are yellow
- Each format is indented 2 spaces

---

### TASK-020: Implement renderSubcommands method with omission
**Status:** pending | **Attempts:** 0

**Description:** Implement Subcommands section rendering with omission logic.

**Acceptance Criteria:**
- [ ] `renderSubcommands() string` method in render.go
- [ ] Returns empty string if no subcommands (section omission)
- [ ] Returns "Subcommands:" header (bold) when subcommands exist
- [ ] 2-space section indent
- [ ] Property tests validate omission and format

**Files:** Modify: `internal/help/render.go`
**Dependencies:** TASK-005, TASK-004
**Traces to:** REQ-017, REQ-021, REQ-024, DES-012, ARCH-006, ARCH-007

**Test Properties:**
- Empty subcommands returns empty string
- Non-empty subcommands includes "Subcommands:" header
- Each subcommand is indented 2 spaces
- Subcommand names and descriptions aligned

---

### TASK-021: Implement renderExamples method
**Status:** pending | **Attempts:** 0

**Description:** Implement Examples section rendering (always present, minimum 1 example).

**Acceptance Criteria:**
- [ ] `renderExamples() string` method in render.go
- [ ] Returns "Examples:" header (bold) followed by examples
- [ ] Each example rendered as plain text (no styling)
- [ ] 2-space example indent
- [ ] Examples always present (enforced by AddExamples)
- [ ] Property tests validate format and plain styling

**Files:** Modify: `internal/help/render.go`
**Dependencies:** TASK-005, TASK-004
**Traces to:** REQ-009, REQ-015, REQ-016, DES-009, ARCH-005, ARCH-009, ARCH-007

**Test Properties:**
- Output includes "Examples:" header
- Examples are plain text (no ANSI codes on example content)
- Each example is indented 2 spaces
- Examples start with command name

---

### TASK-022: Implement Render orchestration method
**Status:** pending | **Attempts:** 0

**Description:** Implement Render method that orchestrates section rendering in canonical order.

**Acceptance Criteria:**
- [ ] `Render() string` method on ContentBuilder
- [ ] Method panics if AddExamples not called yet
- [ ] Calls render methods in canonical order: description, usage, positionals, flags, formats, subcommands, examples
- [ ] Concatenates sections with proper spacing (1 blank line between sections)
- [ ] Returns final help string
- [ ] Property tests validate section ordering across all configurations

**Files:** Modify: `internal/help/builder.go`, `internal/help/render.go`
**Dependencies:** TASK-015, TASK-016, TASK-017, TASK-018, TASK-019, TASK-020, TASK-021
**Traces to:** REQ-001, REQ-012, REQ-025, DES-004, ARCH-005, ARCH-006, ARCH-007

**Test Properties:**
- Render panics if examples not set
- Section indices follow canonical order: description < usage < positionals < flags < formats < subcommands < examples
- Blank line separates non-empty sections
- No trailing whitespace in output

---

## Phase 6: Format Registry

### TASK-023: Define format registry
**Status:** pending | **Attempts:** 0

**Description:** Implement centralized format registry with all supported formats.

**Acceptance Criteria:**
- [ ] `AllFormats` slice defined in formats.go with all supported formats
- [ ] Each format has Name and Desc fields populated
- [ ] Formats include: json, yaml, plain, table (minimum set)
- [ ] Property tests validate format registry is non-empty

**Files:** Modify: `internal/help/formats.go`
**Dependencies:** TASK-003
**Traces to:** REQ-008, REQ-014, REQ-027, REQ-028, ARCH-007

**Test Properties:**
- AllFormats is non-empty
- Each format has non-empty Name
- Each format has non-empty Desc
- Format names are lowercase

---

### TASK-024: Implement FormatNames helper
**Status:** pending | **Attempts:** 0

**Description:** Implement FormatNames helper that subsets AllFormats by name with validation.

**Acceptance Criteria:**
- [ ] `FormatNames(names ...string) []Format` function in formats.go
- [ ] Function panics if name not found in AllFormats
- [ ] Returns subset of AllFormats matching names
- [ ] Property tests validate panic on unknown name
- [ ] Property tests validate correct subsetting

**Files:** Modify: `internal/help/formats.go`
**Dependencies:** TASK-023
**Traces to:** REQ-027, ARCH-007

**Test Properties:**
- FormatNames panics for unknown format name
- FormatNames returns correct subset for valid names
- Returned formats are in order of input names
- Empty input returns empty slice

---

## Phase 7: Property-Based Testing

### TASK-025: Add gomega and rapid test dependencies
**Status:** pending | **Attempts:** 0

**Description:** Add gomega and rapid testing libraries to go.mod.

**Acceptance Criteria:**
- [ ] `go get github.com/onsi/gomega` executed successfully
- [ ] `go get pgregory.net/rapid` executed successfully
- [ ] go.mod contains both dependencies
- [ ] go.sum updated with checksums
- [ ] Test helper file created with imports

**Files:** Modify: `go.mod`, `go.sum`, Create: `internal/help/builder_test.go`
**Dependencies:** TASK-002
**Traces to:** ARCH-010, ARCH-011

---

### TASK-026: Implement section order invariant property test
**Status:** pending | **Attempts:** 0

**Description:** Implement property test that validates section ordering across random help configurations.

**Acceptance Criteria:**
- [ ] Test function `TestHelpSectionOrderInvariant` in builder_test.go
- [ ] Uses rapid to generate random ContentBuilder configurations
- [ ] Generates random presence of positionals, flags, formats, subcommands
- [ ] Always includes description and examples (required)
- [ ] Validates section order: description < usage < positionals < flags < formats < subcommands < examples
- [ ] Uses gomega assertions for readability

**Files:** Modify: `internal/help/builder_test.go`
**Dependencies:** TASK-025, TASK-022
**Traces to:** REQ-012, REQ-025, DES-004, ARCH-005, ARCH-010, ARCH-011

**Test Properties:**
- Description appears before Usage
- Usage appears before Positionals (if present)
- Positionals appear before Flags (if present)
- Flags appear before Formats (if present)
- Formats appear before Subcommands (if present)
- Subcommands appear before Examples
- Examples always last

---

### TASK-027: Implement ANSI code pairing property test
**Status:** pending | **Attempts:** 0

**Description:** Implement property test that validates ANSI codes are properly paired (open/close).

**Acceptance Criteria:**
- [ ] Test function `TestHelpANSICodesPaired` in builder_test.go
- [ ] Uses rapid to generate random help configurations
- [ ] Validates every ANSI escape sequence has matching reset code
- [ ] Validates no dangling escape codes
- [ ] Uses gomega assertions

**Files:** Modify: `internal/help/builder_test.go`
**Dependencies:** TASK-025, TASK-022
**Traces to:** REQ-001, REQ-002, DES-007, ARCH-002, ARCH-010

**Test Properties:**
- Count of `\x1b[` sequences matches count of `\x1b[0m` resets
- No unclosed ANSI codes
- Lipgloss styles are properly applied

---

### TASK-028: Implement no trailing whitespace property test
**Status:** pending | **Attempts:** 0

**Description:** Implement property test that validates no trailing whitespace in help output.

**Acceptance Criteria:**
- [ ] Test function `TestHelpNoTrailingWhitespace` in builder_test.go
- [ ] Uses rapid to generate random help configurations
- [ ] Validates no lines end with whitespace characters
- [ ] Validates no double blank lines
- [ ] Uses gomega assertions

**Files:** Modify: `internal/help/builder_test.go`
**Dependencies:** TASK-025, TASK-022
**Traces to:** REQ-001, DES-003, ARCH-010

**Test Properties:**
- No line ends with space or tab
- No consecutive blank lines
- Single newline at end of output

---

### TASK-029: Implement examples format property test
**Status:** pending | **Attempts:** 0

**Description:** Implement property test that validates examples start with binary name and are plain text.

**Acceptance Criteria:**
- [ ] Test function `TestHelpExamplesFormat` in builder_test.go
- [ ] Uses rapid to generate random example strings
- [ ] Validates examples start with "targ "
- [ ] Validates examples contain no ANSI escape codes (plain text)
- [ ] Uses gomega assertions

**Files:** Modify: `internal/help/builder_test.go`
**Dependencies:** TASK-025, TASK-021
**Traces to:** REQ-015, REQ-016, DES-009, ARCH-009, ARCH-010

**Test Properties:**
- All examples start with "targ "
- Examples section contains no ANSI codes on example lines (headers may have codes)
- Each example is a valid command string

---

### TASK-030: Implement flag subsection order property test
**Status:** pending | **Attempts:** 0

**Description:** Implement property test that validates Global Flags always appear before Command Flags.

**Acceptance Criteria:**
- [ ] Test function `TestHelpFlagSubsectionOrder` in builder_test.go
- [ ] Uses rapid to generate help configs with both global and command flags
- [ ] Validates "Global Flags:" subsection appears before "Command Flags:" subsection
- [ ] Uses gomega assertions

**Files:** Modify: `internal/help/builder_test.go`
**Dependencies:** TASK-025, TASK-018
**Traces to:** REQ-013, DES-006, ARCH-006, ARCH-008, ARCH-010

**Test Properties:**
- When both subsections present, Global Flags index < Command Flags index
- Only "Global Flags:" appears when only global flags
- Only "Command Flags:" appears when only command flags (as "Flags:")

---

### TASK-031: Implement section omission property test
**Status:** pending | **Attempts:** 0

**Description:** Implement property test that validates empty sections are omitted.

**Acceptance Criteria:**
- [ ] Test function `TestHelpSectionOmission` in builder_test.go
- [ ] Uses rapid to generate help configs with various empty sections
- [ ] Validates no "Positionals:" header when no positionals
- [ ] Validates no "Formats:" header when no formats
- [ ] Validates no "Subcommands:" header when no subcommands
- [ ] Validates "Command Flags:" omitted when only global flags
- [ ] Uses gomega assertions

**Files:** Modify: `internal/help/builder_test.go`
**Dependencies:** TASK-025, TASK-022
**Traces to:** REQ-019, REQ-020, REQ-021, ARCH-006, ARCH-010

**Test Properties:**
- Empty positionals → no "Positionals:" in output
- Empty formats → no "Formats:" in output
- Empty subcommands → no "Subcommands:" in output
- Only global flags → no "Command Flags:" subsection
- Description and Usage always present (even if empty description)
- Examples always present

---

## Phase 8: Integration and Migration

### TASK-032: Implement buildCreateHelp using new builder
**Status:** pending | **Attempts:** 0

**Description:** Migrate PrintCreateHelp to use help.Builder instead of manual string building.

**Acceptance Criteria:**
- [ ] New function `buildCreateHelp() string` in runner.go
- [ ] Uses help.NewBuilder("create")
- [ ] Populates description, usage, positionals, command flags, examples
- [ ] Returns rendered help string
- [ ] Unit test validates structure matches expected output

**Files:** Modify: `internal/runner/runner.go`
**Dependencies:** TASK-022
**Traces to:** REQ-015, DES-014, ARCH-009

---

### TASK-033: Implement buildSyncHelp using new builder
**Status:** pending | **Attempts:** 0

**Description:** Migrate PrintSyncHelp to use help.Builder.

**Acceptance Criteria:**
- [ ] New function `buildSyncHelp() string` in runner.go
- [ ] Uses help.NewBuilder("sync")
- [ ] Populates description, usage, examples (no positionals/flags)
- [ ] Returns rendered help string
- [ ] Unit test validates structure

**Files:** Modify: `internal/runner/runner.go`
**Dependencies:** TASK-022
**Traces to:** REQ-015, DES-015, ARCH-009

---

### TASK-034: Implement buildToFuncHelp using new builder
**Status:** pending | **Attempts:** 0

**Description:** Migrate PrintToFuncHelp to use help.Builder.

**Acceptance Criteria:**
- [ ] New function `buildToFuncHelp() string` in runner.go
- [ ] Uses help.NewBuilder("to-func")
- [ ] Populates description, usage, examples
- [ ] Returns rendered help string
- [ ] Unit test validates structure

**Files:** Modify: `internal/runner/runner.go`
**Dependencies:** TASK-022
**Traces to:** REQ-015, DES-016, ARCH-009

---

### TASK-035: Implement buildToStringHelp using new builder
**Status:** pending | **Attempts:** 0

**Description:** Migrate PrintToStringHelp to use help.Builder.

**Acceptance Criteria:**
- [ ] New function `buildToStringHelp() string` in runner.go
- [ ] Uses help.NewBuilder("to-string")
- [ ] Populates description, usage, examples
- [ ] Returns rendered help string
- [ ] Unit test validates structure

**Files:** Modify: `internal/runner/runner.go`
**Dependencies:** TASK-022
**Traces to:** REQ-015, DES-017, ARCH-009

---

### TASK-036: Update PrintCreateHelp to use builder
**Status:** pending | **Attempts:** 0

**Description:** Replace PrintCreateHelp implementation with buildCreateHelp call.

**Acceptance Criteria:**
- [ ] PrintCreateHelp now calls buildCreateHelp()
- [ ] Old manual string building code removed
- [ ] Existing tests still pass
- [ ] Integration test validates help output structure

**Files:** Modify: `internal/runner/runner.go`
**Dependencies:** TASK-032
**Traces to:** DES-014, ARCH-009

---

### TASK-037: Update PrintSyncHelp to use builder
**Status:** pending | **Attempts:** 0

**Description:** Replace PrintSyncHelp implementation with buildSyncHelp call.

**Acceptance Criteria:**
- [ ] PrintSyncHelp now calls buildSyncHelp()
- [ ] Old manual string building code removed
- [ ] Existing tests still pass
- [ ] Integration test validates help output structure

**Files:** Modify: `internal/runner/runner.go`
**Dependencies:** TASK-033
**Traces to:** DES-015, ARCH-009

---

### TASK-038: Update PrintToFuncHelp to use builder
**Status:** pending | **Attempts:** 0

**Description:** Replace PrintToFuncHelp implementation with buildToFuncHelp call.

**Acceptance Criteria:**
- [ ] PrintToFuncHelp now calls buildToFuncHelp()
- [ ] Old manual string building code removed
- [ ] Existing tests still pass
- [ ] Integration test validates help output structure

**Files:** Modify: `internal/runner/runner.go`
**Dependencies:** TASK-034
**Traces to:** DES-016, ARCH-009

---

### TASK-039: Update PrintToStringHelp to use builder
**Status:** pending | **Attempts:** 0

**Description:** Replace PrintToStringHelp implementation with buildToStringHelp call.

**Acceptance Criteria:**
- [ ] PrintToStringHelp now calls buildToStringHelp()
- [ ] Old manual string building code removed
- [ ] Existing tests still pass
- [ ] Integration test validates help output structure

**Files:** Modify: `internal/runner/runner.go`
**Dependencies:** TASK-035
**Traces to:** DES-017, ARCH-009

---

### TASK-040: Implement golden file regression tests
**Status:** pending | **Attempts:** 0

**Description:** Add golden file tests to detect regressions in help output structure.

**Acceptance Criteria:**
- [ ] Golden files created for each help command (create, sync, to-func, to-string)
- [ ] Test function validates rendered help matches golden file (structure, not exact ANSI codes)
- [ ] Test allows for styling differences but validates section presence and order
- [ ] Uses gomega assertions

**Files:** Modify: `internal/runner/runner_help_test.go`, Create: `internal/runner/testdata/*.golden`
**Dependencies:** TASK-036, TASK-037, TASK-038, TASK-039
**Traces to:** REQ-004, DES-020, ARCH-010

---

## Phase 9: Command Integration (Core)

### TASK-041: Update command.go to use help.Builder for printUsage
**Status:** pending | **Attempts:** 0

**Description:** Migrate printUsage in command.go to use help.Builder instead of manual string building.

**Acceptance Criteria:**
- [ ] printUsage function refactored to use help.Builder
- [ ] Global help includes all sections: description, usage, global/command flags, formats (if applicable), examples
- [ ] Existing behavior preserved (backward compatible)
- [ ] Unit tests validate help output structure

**Files:** Modify: `internal/core/command.go`
**Dependencies:** TASK-022
**Traces to:** REQ-001, REQ-006, DES-013, ARCH-009

---

### TASK-042: Update command.go to use help.Builder for printCommandHelp
**Status:** pending | **Attempts:** 0

**Description:** Migrate printCommandHelp in command.go to use help.Builder for target-specific help.

**Acceptance Criteria:**
- [ ] printCommandHelp function refactored to use help.Builder
- [ ] Target help includes: description, source, usage, flags, execution info, examples
- [ ] Existing behavior preserved
- [ ] Unit tests validate help output structure

**Files:** Modify: `internal/core/command.go`
**Dependencies:** TASK-022
**Traces to:** REQ-001, REQ-006, DES-018, ARCH-009

---

## Phase 10: Documentation and Validation

### TASK-043: Add package documentation for internal/help
**Status:** pending | **Attempts:** 0

**Description:** Write comprehensive package-level documentation for internal/help.

**Acceptance Criteria:**
- [ ] Package comment explains purpose and usage
- [ ] Example usage included in package doc
- [ ] All exported types have godoc comments
- [ ] All exported functions have godoc comments
- [ ] Documentation mentions type-state pattern and compile-time enforcement

**Files:** Modify: `internal/help/builder.go`, `internal/help/content.go`, `internal/help/render.go`, `internal/help/styles.go`, `internal/help/formats.go`
**Dependencies:** TASK-022
**Traces to:** REQ-011, ARCH-004

---

### TASK-044: Add linting rule for no direct ANSI codes
**Status:** pending | **Attempts:** 0

**Description:** Add linting rule to prevent direct ANSI escape codes outside internal/help package.

**Acceptance Criteria:**
- [ ] Linting rule added (e.g., in .golangci.yml or custom linter)
- [ ] Rule detects hardcoded ANSI codes like `\x1b[` outside internal/help
- [ ] Existing violations fixed or exempted
- [ ] CI enforces rule

**Files:** Create/Modify: `.golangci.yml` or linting configuration
**Dependencies:** TASK-022
**Traces to:** ARCH-002, ARCH-002

---

### TASK-045: Run full test suite and validate coverage
**Status:** pending | **Attempts:** 0

**Description:** Execute complete test suite and ensure coverage meets project standards.

**Acceptance Criteria:**
- [ ] `mage check` passes (or project equivalent)
- [ ] All property tests pass across multiple runs (randomized inputs)
- [ ] Unit test coverage for internal/help is ≥80%
- [ ] Integration tests pass for all help commands
- [ ] No regressions in existing functionality

**Files:** N/A (validation step)
**Dependencies:** TASK-040, TASK-041, TASK-042, TASK-043, TASK-044
**Traces to:** REQ-004, ARCH-010

---

## Dependency Graph

```
Phase 1: Foundation
TASK-001 (lipgloss dependency)
TASK-002 (package structure)

Phase 2: Data Models
TASK-003 (content structs) ← TASK-002
TASK-004 (ContentBuilder) ← TASK-003

Phase 3: Styling
TASK-005 (lipgloss styles) ← TASK-001

Phase 4: Builder API
TASK-006 (Builder init) ← TASK-004
TASK-007 (WithDescription) ← TASK-006
TASK-008 (WithUsage) ← TASK-007
TASK-009 (AddPositionals) ← TASK-007, TASK-003
TASK-010 (AddGlobalFlags) ← TASK-007, TASK-003
TASK-011 (AddCommandFlags) ← TASK-007, TASK-003
TASK-012 (AddFormats) ← TASK-007, TASK-003
TASK-013 (AddSubcommands) ← TASK-007, TASK-003
TASK-014 (AddExamples) ← TASK-007, TASK-003

Phase 5: Rendering
TASK-015 (renderDescription) ← TASK-004
TASK-016 (renderUsage) ← TASK-005, TASK-004
TASK-017 (renderPositionals) ← TASK-005, TASK-004
TASK-018 (renderFlags) ← TASK-005, TASK-004
TASK-019 (renderFormats) ← TASK-005, TASK-004
TASK-020 (renderSubcommands) ← TASK-005, TASK-004
TASK-021 (renderExamples) ← TASK-005, TASK-004
TASK-022 (Render orchestration) ← TASK-015, TASK-016, TASK-017, TASK-018, TASK-019, TASK-020, TASK-021

Phase 6: Format Registry
TASK-023 (format registry) ← TASK-003
TASK-024 (FormatNames helper) ← TASK-023

Phase 7: Property Testing
TASK-025 (test dependencies) ← TASK-002
TASK-026 (section order test) ← TASK-025, TASK-022
TASK-027 (ANSI pairing test) ← TASK-025, TASK-022
TASK-028 (no trailing whitespace test) ← TASK-025, TASK-022
TASK-029 (examples format test) ← TASK-025, TASK-021
TASK-030 (flag subsection order test) ← TASK-025, TASK-018
TASK-031 (section omission test) ← TASK-025, TASK-022

Phase 8: Integration
TASK-032 (buildCreateHelp) ← TASK-022
TASK-033 (buildSyncHelp) ← TASK-022
TASK-034 (buildToFuncHelp) ← TASK-022
TASK-035 (buildToStringHelp) ← TASK-022
TASK-036 (migrate PrintCreateHelp) ← TASK-032
TASK-037 (migrate PrintSyncHelp) ← TASK-033
TASK-038 (migrate PrintToFuncHelp) ← TASK-034
TASK-039 (migrate PrintToStringHelp) ← TASK-035
TASK-040 (golden file tests) ← TASK-036, TASK-037, TASK-038, TASK-039

Phase 9: Command Integration
TASK-041 (migrate printUsage) ← TASK-022
TASK-042 (migrate printCommandHelp) ← TASK-022

Phase 10: Documentation
TASK-043 (package docs) ← TASK-022
TASK-044 (linting rule) ← TASK-022
TASK-045 (full test suite) ← TASK-040, TASK-041, TASK-042, TASK-043, TASK-044
```

## Parallelism Opportunities

**Phase 1 (Parallel):**
- TASK-001, TASK-002 (independent: dependency vs. directory creation)

**Phase 3 + Phase 4 Builder Methods (Parallel after TASK-007):**
- TASK-008, TASK-009, TASK-010, TASK-011, TASK-012, TASK-013, TASK-014 (all independent builder methods)

**Phase 5 Render Methods (Parallel after TASK-005, TASK-004):**
- TASK-015, TASK-016, TASK-017, TASK-018, TASK-019, TASK-020, TASK-021 (independent render methods)

**Phase 6 (Parallel with Phase 7 dependencies setup):**
- TASK-023, TASK-024 (format registry)
- TASK-025 (test dependencies)

**Phase 7 Property Tests (Parallel after TASK-025, TASK-022):**
- TASK-026, TASK-027, TASK-028, TASK-029, TASK-030, TASK-031 (independent property tests)

**Phase 8 Build Functions (Parallel after TASK-022):**
- TASK-032, TASK-033, TASK-034, TASK-035 (independent help builders for each command)

**Phase 8 Migration Functions (Parallel after respective build functions):**
- TASK-036, TASK-037, TASK-038, TASK-039 (each depends only on its corresponding build function)

**Phase 9 (Parallel after TASK-022):**
- TASK-041, TASK-042 (independent command.go migrations)

**Phase 10 (Sequential):**
- TASK-043 (docs)
- TASK-044 (linting)
- TASK-045 (validation) - must be last

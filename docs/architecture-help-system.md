# Technical Architecture: targ Help System Consistency

## 1. Overview

This architecture implements compile-time enforcement of consistent help structure across all targ commands. The system uses a type-state pattern with bubbletea/lipgloss for ANSI styling, centered on a unified `internal/help` package that provides a declarative API for help construction.

The architecture enforces that help content is impossible to render incorrectly. Section order is structurally guaranteed, flag metadata is centrally registered, and all styling decisions are consolidated in a single rendering implementation. Maintainers describe what to show; the system enforces how to show it.

**Key technical decisions:**

- **ARCH-001:** Type-state pattern with builder phases
- **ARCH-002:** Centralized ANSI styling via bubbletea/lipgloss
- **ARCH-003:** Unified internal/help package architecture

## 2. Requirements Traceability

| Requirement | Technical Implication              | Addressed By       |
| ----------- | ---------------------------------- | ------------------ |
| REQ-001     | Structural consistency enforcement | ARCH-001, ARCH-005 |
| REQ-002     | Predictable section locations      | ARCH-005, ARCH-006 |
| REQ-003     | Automatic conformance              | ARCH-001, ARCH-004 |
| REQ-004     | Objective code review              | ARCH-010, ARCH-011 |
| REQ-005     | Self-contained help                | ARCH-007, ARCH-009 |
| REQ-006     | Same structure everywhere          | ARCH-001, ARCH-005 |
| REQ-007     | Global vs command flags            | ARCH-008, ARCH-014 |
| REQ-008     | Relevant formats only              | ARCH-007, ARCH-009 |
| REQ-009     | Progressive examples               | ARCH-009           |
| REQ-010     | Single source of truth             | ARCH-008, ARCH-014 |
| REQ-011     | Easy path is right path            | ARCH-001, ARCH-004 |
| REQ-012     | Exact section order                | ARCH-005, ARCH-006 |
| REQ-013     | Global/Command subsections         | ARCH-008, ARCH-014 |
| REQ-014     | Formats section                    | ARCH-007, ARCH-009 |
| REQ-015     | 2-3 examples                       | ARCH-009           |
| REQ-016     | Command-specific examples          | ARCH-009           |
| REQ-017     | Subcommands section                | ARCH-009           |
| REQ-018     | Consistent terminology             | ARCH-005, ARCH-012 |
| REQ-019     | Omit empty Command Flags           | ARCH-006           |
| REQ-020     | Omit empty Formats                 | ARCH-006           |
| REQ-021     | Omit empty Subcommands             | ARCH-006           |
| REQ-022     | Root command help                  | ARCH-009           |
| REQ-023     | Minimum 1 example                  | ARCH-009           |
| REQ-024     | Leaf nodes no subcommands          | ARCH-009           |
| REQ-025     | Section order invariant            | ARCH-005, ARCH-006 |
| REQ-026     | Global before Command              | ARCH-008, ARCH-014 |
| REQ-027     | Formats subset                     | ARCH-007           |
| REQ-028     | Document supported formats         | ARCH-007           |

## 3. Technology Stack

| Layer             | Choice               | Rationale                                                            | ARCH ID  |
| ----------------- | -------------------- | -------------------------------------------------------------------- | -------- |
| Language          | Go 1.23+             | Existing codebase, compile-time safety                               | ARCH-001 |
| ANSI Styling      | bubbletea/lipgloss   | Zero external dependencies at runtime, compile-time color management | ARCH-002 |
| Package Structure | Single internal/help | Unified content and rendering, no circular deps                      | ARCH-003 |
| Testing           | gomega + rapid       | Human-readable assertions + property-based validation                | ARCH-010 |

## 4. Architecture

### ARCH-001: Type-State Pattern for Compile-Time Safety

**Traces to:** REQ-003, REQ-006, REQ-011, REQ-025

The help builder uses the type-state pattern where each build phase returns a new type, making it impossible to call methods out of order or skip required steps.

**Design:**

```go
// Phase 1: Builder creation
type Builder struct { ... }

// Phase 2: Content collection (returns ContentBuilder)
type ContentBuilder struct { ... }

// Phase 3: Rendering (returns string)
type RenderedHelp string
```

**Enforcement mechanism:**

- `Builder.WithDescription()` returns `ContentBuilder`
- `ContentBuilder.AddFlags()` returns `ContentBuilder` (chainable)
- `ContentBuilder.Render()` returns `RenderedHelp`
- Each type exposes only valid next operations
- Sections are collected in order, rendered in canonical order

**Benefits:**

- Impossible to render sections out of order (compile error)
- Impossible to skip required content (type system enforces flow)
- Easy to use (method chaining with IntelliSense guidance)
- Self-documenting (types reveal valid next steps)

### ARCH-002: Centralized ANSI Styling via bubbletea/lipgloss

**Traces to:** DES-001, DES-007, REQ-001, REQ-002

All color and text styling is managed through lipgloss styles, ensuring consistency and preventing ANSI code leakage.

**Design:**

```go
var (
    sectionHeaderStyle = lipgloss.NewStyle().Bold(true)
    flagNameStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan
    placeholderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
    formatNameStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
)
```

**Style definitions (DES-001):**

- Section headers: Bold white
- Flag names: Cyan (#6)
- Placeholders: Yellow (#3)
- Subsection headers: Bold
- Examples: Plain
- Format names: Yellow (#3)

**Enforcement:**

- All rendering goes through styled helpers
- Direct ANSI codes are prohibited (linting rule)
- Color accessibility validated (contrast ratio checks)

### ARCH-003: Unified internal/help Package Architecture

**Traces to:** REQ-010, REQ-011

Single package combining content logic and rendering, eliminating circular dependencies and ensuring single source of truth.

**Structure:**

```
internal/help/
├── builder.go       # Type-state builder API
├── content.go       # Content collection (flags, examples, etc.)
├── render.go        # Rendering logic with section ordering
├── styles.go        # lipgloss style definitions
└── builder_test.go  # Property-based tests
```

**Registry consumption:**

- `internal/help` imports `internal/flags` (not vice versa)
- Builder consumes `flags.Def` directly
- No flag metadata duplication

**Benefits:**

- Clear ownership (help owns all presentation logic)
- No circular dependencies (flags → help, never help → flags)
- Easy to test (single package to validate invariants)
- Easy to extend (add new sections in one place)

### ARCH-004: Declarative Help Construction API

**Traces to:** REQ-003, REQ-011, DES-021

The builder API is declarative, describing what to show rather than how to render it.

**Example usage:**

```go
help := help.NewBuilder("create").
    WithDescription("Create a new target in the current targ file").
    WithUsage("targ --create [group...] <name>").
    AddPositionals(
        help.Positional{Name: "group", Required: false},
        help.Positional{Name: "name", Required: true},
    ).
    AddGlobalFlags(flags.GlobalFlags()...).
    AddCommandFlags(
        help.Flag{Long: "shell", Short: "s", Desc: "Create a shell command target"},
    ).
    AddExamples(
        help.Example{Title: "Create a simple target", Code: "targ --create test"},
        help.Example{Title: "Create in a group", Code: "targ --create dev lint"},
    ).
    Render()
```

**Properties:**

- Order of `.AddX()` calls doesn't matter (builder sorts internally)
- Canonical section order is enforced by `Render()`
- Empty sections are automatically omitted
- No manual string building required

### ARCH-005: Canonical Section Order

**Traces to:** REQ-001, REQ-012, REQ-025, DES-004, DR-001

All help output follows this exact order:

1. **Description** (first line, no header)
2. **Usage:**
3. **Positionals:** (if present)
4. **Flags:** (with Global/Command subsections)
5. **Formats:** (if present)
6. **Subcommands:** (if present)
7. **Examples:**

**Enforcement:**

- `Render()` method iterates sections in canonical order
- Sections are stored as typed fields, not a slice (prevents reordering)
- Property tests validate order in all generated help

### ARCH-006: Section Omission Rules

**Traces to:** REQ-019, REQ-020, REQ-021, DES-004, DR-003

Empty sections are automatically omitted during rendering.

**Omission logic:**

- No Positionals header if `len(positionals) == 0`
- No Command Flags subsection if only global flags exist
- No Formats section if `len(formats) == 0`
- No Subcommands section if `len(subcommands) == 0`
- Always show Description (even if empty string → blank line)
- Always show Usage (generated from metadata)
- Always show Examples (builder enforces minimum 1)

**Implementation:**

```go
func (r *Renderer) renderFlags() string {
    if len(r.globalFlags) == 0 && len(r.commandFlags) == 0 {
        return "" // omit entire section
    }

    var b strings.Builder
    b.WriteString(sectionHeaderStyle.Render("Flags:") + "\n")

    if len(r.globalFlags) > 0 {
        b.WriteString(subsectionHeaderStyle.Render("  Global Flags:") + "\n")
        // render global flags
    }

    if len(r.commandFlags) > 0 {
        b.WriteString(subsectionHeaderStyle.Render("  Command Flags:") + "\n")
        // render command flags
    }

    return b.String()
}
```

### ARCH-007: Format Registry and Subsetting

**Traces to:** REQ-008, REQ-014, REQ-027, REQ-028, DES-011

Centralized format definitions with per-command filtering.

**Format registry:**

```go
// internal/help/formats.go
type Format struct {
    Name string
    Desc string
}

var AllFormats = []Format{
    {Name: "json", Desc: "Output as JSON"},
    {Name: "yaml", Desc: "Output as YAML"},
    {Name: "plain", Desc: "Plain text output (default)"},
    {Name: "table", Desc: "Tabular output"},
}
```

**Per-command subsetting:**

```go
help.NewBuilder("list").
    AddFormats(
        help.FormatNames("json", "yaml", "plain"), // subset of AllFormats
    ).
    Render()
```

**Enforcement:**

- `FormatNames()` validates names exist in `AllFormats`
- Format section only rendered if non-empty
- Format descriptions are single source of truth

### ARCH-008: Flag Registry Integration

**Traces to:** REQ-007, REQ-010, REQ-013, REQ-026, DES-019

The `internal/flags` package is the single source of truth for all flag metadata.

**Registry structure:**

```go
// internal/flags/flags.go
type Def struct {
    Long       string // without "--"
    Short      string // without "-"
    Desc       string
    TakesValue bool
    RootOnly   bool
    Hidden     bool
    Removed    string
}

var All = []Def{ /* all flags */ }

func GlobalFlags() []string
func RootOnlyFlags() []string
func VisibleFlags() []Def
```

**Integration:**

```go
// internal/help/builder.go
func (b *ContentBuilder) AddGlobalFlags() *ContentBuilder {
    for _, name := range flags.GlobalFlags() {
        def := flags.Find("--" + name[2:]) // strip "--"
        b.globalFlags = append(b.globalFlags, convertFlagDef(def))
    }
    return b
}
```

**Benefits:**

- Flags automatically grouped by `RootOnly` property
- Hidden flags automatically excluded
- Flag changes in registry immediately reflected in all help

### ARCH-009: Example Requirements

**Traces to:** REQ-009, REQ-015, REQ-016, REQ-023, DES-009

Every command must have 1-3 examples showing progressive complexity.

**Structure:**

```go
type Example struct {
    Title string // e.g. "Create a simple target"
    Code  string // e.g. "targ --create test"
}
```

**Builder enforcement:**

```go
func (b *ContentBuilder) AddExamples(examples ...Example) *ContentBuilder {
    if len(examples) == 0 {
        panic("at least 1 example required")
    }
    b.examples = examples
    return b
}
```

**Validation:**

- Examples must start with binary name (validated by property test)
- Examples are command-specific (not generic flag tutorials)
- 2-3 examples preferred, 1 minimum acceptable

### ARCH-010: Property-Based Help Validation

**Traces to:** REQ-004, DES-020

Property tests validate structural invariants across all possible help configurations.

**Test strategy:**

```go
// internal/help/builder_test.go
func TestHelpInvariants(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        // Generate random help configuration
        builder := generateRandomBuilder(t)
        output := builder.Render()

        // Validate invariants
        g := NewWithT(t)
        g.Expect(output).To(MatchRegexp(`^[A-Z]`)) // Description first
        validateSectionOrder(g, output)
        validateNoTrailingWhitespace(g, output)
        validateExamplesStartWithBinary(g, output)
    })
}
```

**Properties validated:**

- Non-empty description
- Section ordering (Usage before Flags before Examples)
- No trailing whitespace
- Examples start with "targ"
- Flag lines start with "--"
- No empty section headers
- ANSI codes are properly paired (open/close)

### ARCH-011: Test Tooling

**Traces to:** REQ-004

**Human-readable matchers (gomega):**

```go
g := NewWithT(t)
g.Expect(output).To(ContainSubstring("Usage:"))
g.Expect(output).NotTo(ContainSubstring("  \n")) // no trailing whitespace
```

**Randomized property exploration (rapid):**

```go
rapid.Check(t, func(t *rapid.T) {
    // Generate random configurations
    hasFlags := rapid.Bool().Draw(t, "hasFlags")
    hasExamples := rapid.Bool().Draw(t, "hasExamples")

    // Build help with random config
    builder := help.NewBuilder("cmd")
    if hasFlags { builder.AddGlobalFlags() }
    if hasExamples { builder.AddExamples(...) }

    // Validate properties
    validateInvariants(t, builder.Render())
})
```

## 5. Data Models

### ARCH-012: Help Content Model

**Traces to:** REQ-001, REQ-012, REQ-018

```go
// internal/help/content.go

// ContentBuilder collects help content before rendering
type ContentBuilder struct {
    commandName    string
    description    string
    usage          string
    positionals    []Positional
    globalFlags    []Flag
    commandFlags   []Flag
    formats        []Format
    subcommands    []Subcommand
    examples       []Example
}

// Positional represents a positional argument
type Positional struct {
    Name        string
    Placeholder string
    Required    bool
}

// Flag represents a flag in help output
type Flag struct {
    Long        string
    Short       string
    Desc        string
    Placeholder string
    Required    bool
}

// Format represents an output format
type Format struct {
    Name string
    Desc string
}

// Subcommand represents a nested command
type Subcommand struct {
    Name string
    Desc string
}

// Example represents a usage example
type Example struct {
    Title string // e.g. "Create a simple target"
    Code  string // e.g. "targ --create test"
}
```

**Validation:**

- `commandName` must be non-empty
- `examples` must have at least 1 element
- `usage` is auto-generated if not explicitly set
- All other fields are optional (omitted if empty)

### ARCH-013: Style Definitions

**Traces to:** DES-001, DES-007, ARCH-002

```go
// internal/help/styles.go
import "github.com/charmbracelet/lipgloss"

var (
    // Section headers (Usage:, Flags:, etc.)
    sectionHeaderStyle = lipgloss.NewStyle().Bold(true)

    // Subsection headers (Global Flags:, Command Flags:)
    subsectionHeaderStyle = lipgloss.NewStyle().Bold(true)

    // Flag names (--timeout, -h)
    flagNameStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("6")) // ANSI cyan

    // Placeholders (<duration>, <command>)
    placeholderStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("3")) // ANSI yellow

    // Format names (json, yaml, plain)
    formatNameStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("3")) // ANSI yellow

    // Examples (plain text, no styling)
    exampleStyle = lipgloss.NewStyle() // default styling
)
```

**Color accessibility:**

- All colors tested against standard terminal backgrounds
- Bold used for headers (ensures visibility without color)
- Minimum contrast ratio: 4.5:1 (WCAG AA)

## 6. Service Interfaces

### ARCH-014: Help Builder API

**Traces to:** REQ-003, REQ-011, DES-021

```go
// internal/help/builder.go

// NewBuilder creates a new help builder for a command.
func NewBuilder(commandName string) *Builder

// Builder is the entry point for help construction.
type Builder struct {
    commandName string
}

// WithDescription sets the command description (first line of help).
func (b *Builder) WithDescription(desc string) *ContentBuilder

// ContentBuilder collects help content.
type ContentBuilder struct { ... }

// WithUsage sets a custom usage line (optional, auto-generated if omitted).
func (c *ContentBuilder) WithUsage(usage string) *ContentBuilder

// AddPositionals adds positional arguments to help.
func (c *ContentBuilder) AddPositionals(pos ...Positional) *ContentBuilder

// AddGlobalFlags adds flags valid at any command level.
func (c *ContentBuilder) AddGlobalFlags(flagNames ...string) *ContentBuilder

// AddCommandFlags adds flags specific to this command.
func (c *ContentBuilder) AddCommandFlags(flags ...Flag) *ContentBuilder

// AddFormats adds supported output formats.
func (c *ContentBuilder) AddFormats(formats ...Format) *ContentBuilder

// AddSubcommands adds nested subcommands.
func (c *ContentBuilder) AddSubcommands(subs ...Subcommand) *ContentBuilder

// AddExamples adds usage examples (minimum 1 required).
func (c *ContentBuilder) AddExamples(examples ...Example) *ContentBuilder

// Render produces the final help output.
func (c *ContentBuilder) Render() string
```

**Error handling:**

- Panic on invalid usage (e.g., empty examples, duplicate sections)
- Rationale: Help construction is build-time logic, panics caught in tests

### ARCH-015: Flag Registry API (Existing)

**Traces to:** REQ-010, ARCH-008

```go
// internal/flags/flags.go (existing)

// Find returns the flag definition for a given arg (e.g., "--create", "-p").
func Find(arg string) *Def

// GlobalFlags returns --long names of flags valid at any command level.
func GlobalFlags() []string

// RootOnlyFlags returns --long names of flags only valid at root.
func RootOnlyFlags() []string

// VisibleFlags returns all non-hidden, non-removed flags.
func VisibleFlags() []Def
```

**Integration point:**

- `help.Builder` calls `flags.GlobalFlags()` and `flags.RootOnlyFlags()`
- No flag metadata duplication

### ARCH-016: Rendering Service

**Traces to:** REQ-001, REQ-012, ARCH-005

```go
// internal/help/render.go

// Renderer applies section ordering and styling.
type Renderer struct {
    content *ContentBuilder
}

// renderDescription renders the description (first line, no header).
func (r *Renderer) renderDescription() string

// renderUsage renders the Usage: section.
func (r *Renderer) renderUsage() string

// renderPositionals renders the Positionals: section (if present).
func (r *Renderer) renderPositionals() string

// renderFlags renders the Flags: section with subsections.
func (r *Renderer) renderFlags() string

// renderFormats renders the Formats: section (if present).
func (r *Renderer) renderFormats() string

// renderSubcommands renders the Subcommands: section (if present).
func (r *Renderer) renderSubcommands() string

// renderExamples renders the Examples: section.
func (r *Renderer) renderExamples() string

// Render orchestrates section rendering in canonical order.
func (r *Renderer) Render() string {
    var b strings.Builder

    // Canonical section order (ARCH-005)
    b.WriteString(r.renderDescription())
    b.WriteString(r.renderUsage())
    b.WriteString(r.renderPositionals())
    b.WriteString(r.renderFlags())
    b.WriteString(r.renderFormats())
    b.WriteString(r.renderSubcommands())
    b.WriteString(r.renderExamples())

    return b.String()
}
```

**Enforcement:**

- Sections always rendered in this order (no configurability)
- Empty sections return empty string (automatic omission)

## 7. File Structure

```
targ/
├── internal/
│   ├── flags/
│   │   └── flags.go              # ARCH-015: Flag registry (existing)
│   ├── help/
│   │   ├── builder.go            # ARCH-014: Type-state builder API
│   │   ├── content.go            # ARCH-012: Content data structures
│   │   ├── render.go             # ARCH-016: Rendering with section order
│   │   ├── styles.go             # ARCH-013: lipgloss style definitions
│   │   ├── builder_test.go       # ARCH-010: Property-based tests
│   │   └── formats.go            # ARCH-007: Format registry
│   ├── core/
│   │   ├── command.go            # (existing) uses help.Builder
│   │   └── completion.go         # (existing) help integration
│   └── runner/
│       ├── runner.go             # (existing) calls help.Builder
│       └── runner_help_test.go   # (existing) help validation tests
├── targ.go                       # (existing) public API
└── go.mod                        # Add charmbracelet/lipgloss dependency
```

**Dependency flow:**

```
internal/flags (no deps)
    ↓
internal/help (imports flags)
    ↓
internal/core (imports help)
    ↓
internal/runner (imports core)
    ↓
targ.go (imports runner)
```

**Key directories:**

- `internal/flags/`: Single source of truth for flag metadata
- `internal/help/`: Unified help construction and rendering
- `internal/core/`: Command parsing and execution (consumers of help)
- `internal/runner/`: High-level orchestration (consumers of help)

## 8. Technology Decisions

### Decisions Made

| Decision          | Choice                | Alternatives Considered             | Rationale                                               | ARCH ID  |
| ----------------- | --------------------- | ----------------------------------- | ------------------------------------------------------- | -------- |
| Language          | Go 1.23+              | (existing codebase)                 | Type safety, compile-time checks                        | ARCH-001 |
| ANSI library      | bubbletea/lipgloss    | fatih/color, manual ANSI            | Zero-copy styling, style composition, no runtime deps   | ARCH-002 |
| Package structure | Unified internal/help | Separate help-content + help-render | Avoids circular deps, single source of truth            | ARCH-003 |
| API style         | Type-state builder    | Functional options, plain struct    | Compile-time ordering enforcement                       | ARCH-001 |
| Flag source       | Centralized registry  | Per-command flag lists              | Prevents drift, single source of truth                  | ARCH-008 |
| Format source     | Centralized registry  | Inline per-command                  | Consistency, easy to extend                             | ARCH-007 |
| Testing           | gomega + rapid        | testify, plain testing              | Human-readable + property exploration                   | ARCH-010 |
| Error handling    | Panic on misuse       | Return errors                       | Help construction is build-time, panics caught in tests | ARCH-014 |

### Patterns Used

| Pattern              | Where             | Why                                    | ARCH ID            |
| -------------------- | ----------------- | -------------------------------------- | ------------------ |
| Type-state           | Builder API       | Compile-time ordering enforcement      | ARCH-001           |
| Builder              | Help construction | Declarative API, fluent chaining       | ARCH-004, ARCH-014 |
| Registry             | Flags, Formats    | Single source of truth, prevents drift | ARCH-007, ARCH-008 |
| Dependency Injection | Testing           | Enables property-based validation      | ARCH-010           |
| Composite            | lipgloss styles   | Style reuse, consistent rendering      | ARCH-002, ARCH-013 |

## 9. Error Handling

### Build-Time Errors (Panics)

Help construction is build-time logic. Invalid usage panics immediately, caught by tests.

**Panic scenarios:**

- `AddExamples()` called with zero examples
- `Render()` called before `AddExamples()`
- `FormatNames()` given unknown format name
- Duplicate section additions (e.g., calling `AddFormats()` twice)

**Rationale:**

- Help bugs should be caught in tests, not runtime
- Panics provide clear stack traces for fix
- No need for error handling boilerplate in help construction code

### Runtime Errors (Never)

Help rendering never fails at runtime. All validation happens during construction.

**Guarantees:**

- `Render()` always returns valid string
- Empty sections are omitted, never error
- ANSI codes always properly paired (lipgloss handles)

## 10. Testing Strategy

### Test Tooling Requirements

**Human-readable matchers (gomega):**

- Assertions read like sentences
- Self-documenting test failures
- Example: `Expect(output).To(ContainSubstring("Usage:"))`

**Randomized property exploration (rapid):**

- Generate random help configurations
- Verify invariants hold across all cases
- Catches edge cases humans miss

### Testing by Layer

**Unit tests (builder.go, render.go):**

- Builder API usage patterns
- Section rendering correctness
- Style application

**Property tests (builder_test.go):**

- Section order invariant across all configs
- No trailing whitespace ever
- Examples always start with binary name
- ANSI codes always paired
- Empty sections omitted correctly

**Integration tests (runner_help_test.go):**

- Validate help output for actual commands (--create, --sync, etc.)
- Compare before/after migration (golden file testing)
- Regression detection

**Example property test:**

```go
func TestHelpSectionOrderInvariant(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        // Generate random help config
        builder := help.NewBuilder("cmd").
            WithDescription(rapid.String().Draw(t, "desc"))

        if rapid.Bool().Draw(t, "hasFlags") {
            builder.AddGlobalFlags(flags.GlobalFlags()...)
        }

        builder.AddExamples(help.Example{
            Title: "Example",
            Code:  "targ cmd",
        })

        output := builder.Render()

        // Validate section order
        g := NewWithT(t)
        descIdx := strings.Index(output, builder.description)
        usageIdx := strings.Index(output, "Usage:")
        examplesIdx := strings.Index(output, "Examples:")

        g.Expect(descIdx).To(BeNumerically("<", usageIdx))
        g.Expect(usageIdx).To(BeNumerically("<", examplesIdx))
    })
}
```

## 11. Implementation Plan

### Phase 1: Foundation (ARCH-002, ARCH-003, ARCH-013)

1. Add charmbracelet/lipgloss dependency
2. Create internal/help package structure
3. Define styles.go with lipgloss style definitions
4. Implement content.go data structures (ARCH-012)

### Phase 2: Builder API (ARCH-001, ARCH-004, ARCH-014)

1. Implement builder.go type-state API
2. Add method chaining for content collection
3. Implement panic-on-misuse error handling
4. Write unit tests for builder usage

### Phase 3: Rendering (ARCH-005, ARCH-006, ARCH-016)

1. Implement render.go with canonical section order
2. Add section omission logic
3. Apply lipgloss styling to all sections
4. Write rendering unit tests

### Phase 4: Registry Integration (ARCH-007, ARCH-008, ARCH-015)

1. Implement formats.go format registry
2. Add flag registry consumption in builder
3. Add format subsetting validation
4. Write registry integration tests

### Phase 5: Property Testing (ARCH-010, ARCH-011)

1. Add gomega and rapid dependencies
2. Implement property-based invariant tests
3. Add golden file regression tests
4. Validate against existing help output

### Phase 6: Migration (ARCH-009)

1. Migrate PrintCreateHelp to help.Builder
2. Migrate PrintSyncHelp to help.Builder
3. Migrate PrintToFuncHelp to help.Builder
4. Migrate PrintToStringHelp to help.Builder
5. Remove old help functions from runner.go

### Phase 7: Documentation and Validation

1. Add package documentation for internal/help
2. Update contributing guide with help construction patterns
3. Add linting rules (no direct ANSI codes outside help package)
4. Run full test suite and validate coverage

## 12. Migration Strategy

### Existing Help Functions

Current implementation in `internal/runner/runner.go`:

- `PrintCreateHelp()`
- `PrintSyncHelp()`
- `PrintToFuncHelp()`
- `PrintToStringHelp()`

These already follow correct structure but lack styling and compile-time enforcement.

### Migration Approach

**Step 1: Create parallel implementations using help.Builder**

```go
// internal/runner/runner.go (new implementations)

func buildCreateHelp() string {
    return help.NewBuilder("create").
        WithDescription("Create a new target in the current targ file").
        WithUsage("targ --create [group...] <name>").
        AddPositionals(
            help.Positional{Name: "group", Required: false},
            help.Positional{Name: "name", Required: true},
        ).
        AddCommandFlags(
            help.Flag{Long: "shell", Short: "s", Desc: "Create a shell command target"},
        ).
        AddExamples(
            help.Example{Title: "Create a simple target", Code: "targ --create test"},
            help.Example{Title: "Create in a group", Code: "targ --create dev lint"},
            help.Example{Title: "Create a shell command", Code: "targ --create --shell deploy"},
        ).
        Render()
}
```

**Step 2: Replace old implementations**

```go
// internal/runner/runner.go

// PrintCreateHelp prints help for the --create flag.
func PrintCreateHelp() {
    fmt.Println(buildCreateHelp()) // Use builder instead of manual strings
}
```

**Step 3: Validate with golden tests**

```go
// internal/runner/runner_help_test.go

func TestCreateHelpMatchesGolden(t *testing.T) {
    g := NewWithT(t)
    actual := buildCreateHelp()

    // Compare against golden file (old output)
    // Allow for styling differences, validate structure
    validateHelpStructure(g, actual, helpSpec{
        command:        "create",
        hasPositionals: true,
        hasFlags:       true,
    })
}
```

**Step 4: Remove old string-building code**

After all tests pass, delete old manual help functions.

### Rollback Plan

If issues arise:

1. Revert to old help functions (preserved in git history)
2. Keep help.Builder for future incremental adoption
3. No breaking changes to external API

## 13. Open Questions

None. All architectural decisions finalized.

## 14. Traceability Matrix

### Requirements Coverage

| REQ ID  | Requirement                | Architecture Elements        |
| ------- | -------------------------- | ---------------------------- |
| REQ-001 | Structural consistency     | ARCH-001, ARCH-005, ARCH-006 |
| REQ-002 | Predictable locations      | ARCH-005, ARCH-006, ARCH-013 |
| REQ-003 | Automatic conformance      | ARCH-001, ARCH-004, ARCH-014 |
| REQ-004 | Objective code review      | ARCH-010, ARCH-011           |
| REQ-005 | Self-contained help        | ARCH-007, ARCH-009           |
| REQ-006 | Same structure everywhere  | ARCH-001, ARCH-005           |
| REQ-007 | Global vs command flags    | ARCH-008, ARCH-014           |
| REQ-008 | Relevant formats only      | ARCH-007, ARCH-009           |
| REQ-009 | Progressive examples       | ARCH-009                     |
| REQ-010 | Single source of truth     | ARCH-008, ARCH-014           |
| REQ-011 | Easy path is right path    | ARCH-001, ARCH-004           |
| REQ-012 | Exact section order        | ARCH-005, ARCH-006           |
| REQ-013 | Global/Command subsections | ARCH-008, ARCH-014           |
| REQ-014 | Formats section            | ARCH-007, ARCH-009           |
| REQ-015 | 2-3 examples               | ARCH-009                     |
| REQ-016 | Command-specific examples  | ARCH-009                     |
| REQ-017 | Subcommands section        | ARCH-009                     |
| REQ-018 | Consistent terminology     | ARCH-005, ARCH-012           |
| REQ-019 | Omit empty Command Flags   | ARCH-006                     |
| REQ-020 | Omit empty Formats         | ARCH-006                     |
| REQ-021 | Omit empty Subcommands     | ARCH-006                     |
| REQ-022 | Root command help          | ARCH-009                     |
| REQ-023 | Minimum 1 example          | ARCH-009                     |
| REQ-024 | Leaf nodes no subcommands  | ARCH-009                     |
| REQ-025 | Section order invariant    | ARCH-005, ARCH-006           |
| REQ-026 | Global before Command      | ARCH-008, ARCH-014           |
| REQ-027 | Formats subset             | ARCH-007                     |
| REQ-028 | Document supported formats | ARCH-007                     |

### Design Coverage

| DES ID  | Design Element              | Architecture Elements        |
| ------- | --------------------------- | ---------------------------- |
| DES-001 | Color Palette               | ARCH-002, ARCH-013           |
| DES-002 | Typography Scale            | ARCH-013                     |
| DES-003 | Spacing System              | ARCH-016                     |
| DES-004 | Section Structure           | ARCH-005, ARCH-006           |
| DES-005 | Section Header Component    | ARCH-013, ARCH-016           |
| DES-006 | Subsection Header Component | ARCH-013, ARCH-016           |
| DES-007 | Flag Entry Component        | ARCH-008, ARCH-016           |
| DES-008 | Usage Line Component        | ARCH-016                     |
| DES-009 | Example Entry Component     | ARCH-009, ARCH-016           |
| DES-010 | Positionals Entry Component | ARCH-016                     |
| DES-011 | Format Entry Component      | ARCH-007, ARCH-016           |
| DES-012 | Subcommands Entry Component | ARCH-016                     |
| DES-019 | Flag Registry Integration   | ARCH-008, ARCH-014, ARCH-015 |
| DES-020 | Help Validation Testing     | ARCH-010, ARCH-011           |
| DES-021 | Help Rendering Architecture | ARCH-001, ARCH-004, ARCH-014 |

## 15. Summary

This architecture delivers compile-time enforcement of help consistency through:

1. **Type-state pattern (ARCH-001):** Impossible to render sections out of order or skip required content
2. **Centralized styling (ARCH-002):** All ANSI codes managed by lipgloss, no leakage
3. **Unified package (ARCH-003):** Single source of truth, no circular dependencies
4. **Declarative API (ARCH-004):** Describe what to show, system enforces how
5. **Registry integration (ARCH-007, ARCH-008):** Flags and formats automatically consistent

All 28 requirements are covered with full traceability to 16 architecture decisions. The implementation provides:

- Compile-time guarantees (impossible to violate structure)
- Easy to use (declarative API, method chaining)
- Easy to test (property-based validation)
- Easy to maintain (single package, clear ownership)

Next phase: Task breakdown for TDD implementation.

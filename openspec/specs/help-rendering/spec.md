# Help Rendering Specification

## Purpose

This bounded context covers targ's `--help` output. targ has TWO root-help renderers: a
styled renderer (`internal/help.WriteRootHelp`, backed by `internal/help.ContentBuilder`)
used for the single-module case, and a second, flat, unstyled renderer
(`printMultiModuleHelp`, `internal/runner/runner.go:3883-3895`) used whenever
`len(moduleGroups) > 1` (`internal/runner/runner.go:1878-1879`). Requirements 3 through 7
below describe the STYLED renderer only — no audit finding examined the multi-module
renderer, so this spec makes no claim about it.

## Requirements

### Requirement: Source attribution shows an import path or file path, never a line number
Target help SHALL display a `Source: <value>` line whose value is either the Go import
path of the package that registered the target, or a plain file path — and MUST NOT
include a line number in any case. `funcSourceFile` (`internal/core/command.go:1298-1301`)
extracts only the file component from `runtime.FuncForPC(...).FileLine(...)`, explicitly
discarding the returned line number. `resolveTargetSource`
(`internal/core/command.go:1972-1979`) resolves the displayed value with priority:
`t.GetSource()` (the registering package's import path, e.g.
`github.com/toejough/targ/dev`) first, then `t.GetSourceFile()` (a relative file path) as
fallback, then the already-set file path from `funcSourceFile`. `commandNode.SourceFile`
(`internal/core/command.go:118`) is declared as a plain `string` field with no companion
line-number field anywhere in the struct.

#### Scenario: A target registered from a named package shows an import-path-style source
- **WHEN** a target is registered via `targ.Register` from a package other than `main`
- **THEN** the rendered source line reads `Source: <import-path>` with no trailing
  `:<line>`, e.g. `Source: github.com/toejough/targ/dev`

#### Scenario: A target registered from the main package shows the bare package name
- **WHEN** `targ deploy --help` is run for a target registered from a `package main` file
  with `Retry()`, `Backoff(1*time.Second, 2.0)`, and `Times(3)` configured
- **THEN** the rendered header begins with `Source: main` — confirmed by building and
  running such a binary under a PTY, which printed exactly `Source: main` with no line
  number anywhere in the output

### Requirement: Dynamic help sections render in a fixed canonical order with Formats before Positionals
The styled renderer SHALL emit dynamic help sections in exactly this order: Targ flags,
Values, Formats, Positionals, Command flags, Subcommands, Command groups, Execution info,
Examples. This order is fixed by the `renderers` slice in `renderDynamicSections`
(`internal/help/render.go:89-100`), and Formats MUST always precede Positionals — never the
reverse.

#### Scenario: A target with both a Formats-eligible flag and positionals
- **WHEN** `targ --create --help` is run (or the golden file
  `internal/runner/testdata/golden/create.golden` is inspected)
- **THEN** the `Formats:` section appears before the `Positionals:` section:
  ```
  Formats:
    duration  <int><unit> where unit is s (seconds), m (minutes), h (hours)

  Positionals:
    <group...>
    <name> (required)
    "<shell-command>" (required)
  ```

### Requirement: Global flags render under one header with two nested subsections
The styled renderer SHALL render targ's built-in flags under a single `Global flags:`
header, containing two nested labelled subsections, `Global:` and `Root only:` — never as
two sibling top-level headers such as `Global Flags:` and `Command Flags:`. This structure
is produced by `renderTargModeFlags` (`internal/help/render.go:363-390`), which writes one
`Global flags:` header then conditionally nests a `Global:` subsection and (root help only)
a `Root only:` subsection. Flags are assigned to these two groups by
`AddTargFlagsFiltered` (`internal/help/generators.go:134-180`) based on each flag
definition's `RootOnly` bit. The `--source`/`-s` flag is defined with `RootOnly: true`
(`internal/flags/flags.go:55-63`), so it renders under `Root only:`, not `Global:`.

#### Scenario: Root help flag section structure
- **WHEN** root help is rendered for a single-module binary (e.g.
  `targ-example-simple --help` built from `examples/simple`)
- **THEN** the output contains one `Global flags:` header with nested subsections, observed
  live as:
  ```
  Global flags:
    Global:
      --help, -h                Show help
      --timeout <duration>      Set execution timeout
      ...
    Root only:
      --completion {bash|zsh|fish}  Generate shell completion script
      --source, -s <dir>        Use targ files from specified directory
      ...
  ```

### Requirement: Usage renders as a header line followed by an indented usage string
The styled renderer SHALL render `Usage:` as its own bold header line, with the usage
string on the following line, indented — never as a single inline line such as
`Usage: targ <CTX> [flags...]`. This is produced by `renderUsage`
(`internal/help/render.go:392-407`), which writes the `Usage:` header, a newline, two
spaces of indent, then the usage string. For root help the usage string itself reads
`<binary> [targ flags...] [<command>...]` (`internal/help/generators.go:143`).

#### Scenario: Root help usage block
- **WHEN** root help is rendered for a single-module binary
- **THEN** the output contains, on two separate lines:
  ```
  Usage:
    targ-example-simple [targ flags...] [<command>...]
  ```

### Requirement: Positionals render without a description, by data-model limitation
The `Positional` struct (`internal/help/content.go:71-75`) SHALL have no description
field — only `Name`, `Placeholder`, and `Required` — so rendered positionals MUST consist
of only the bare placeholder plus an optional trailing `(required)` marker, and MUST NOT
render a description column, e.g. never `<NAME>    <DESCRIPTION>`. This is a limitation of
the data model itself, not a rendering choice: there is no description to render even if
the renderer wanted to show one.

#### Scenario: Positionals section for a target with required positional arguments
- **WHEN** the golden file `internal/runner/testdata/golden/create.golden` is inspected
- **THEN** the `Positionals:` section shows only placeholders and required markers, with no
  description text of any kind:
  ```
  Positionals:
    <group...>
    <name> (required)
    "<shell-command>" (required)
  ```

### Requirement: The Formats section documents CLI value-placeholder syntax, not output formats
The `Formats:` section SHALL document the syntax of CLI value placeholders used by flags
(e.g. `<duration>`, `<glob>`), not a list of output formats such as `json`, `yaml`, or
`plain` — no output-format flag exists anywhere in targ. `AddTargFlagsFiltered`
(`internal/help/builder.go:106-136`) auto-populates the Formats section from
`flags.PlaceholdersUsedByFlags`, whose entries are defined in
`internal/flags/placeholders.go:39-74` (e.g. `placeholderDuration`, `placeholderGlob`),
each pairing a placeholder token with a human-readable format description.

#### Scenario: Root help Formats section for a binary with duration- and glob-taking flags
- **WHEN** root help is rendered for a single-module binary
- **THEN** the `Formats:` section lists value-placeholder syntax, not output formats,
  observed live as:
  ```
  Formats:
    <duration>  time value like 30s, 5m, 1h
    <duration,mult>  duration and multiplier like 1s,2.0
    <glob>  glob pattern like **/*.go, src/**
  ```

### Requirement: Root help groups commands under per-source headers and auto-generates up to two examples
For the STYLED single-module renderer, `renderCommandGroups`
(`internal/help/render.go:51-83`) SHALL render each command group under its own
`  Source: <value>` sub-header beneath a single `Commands:` header, rather than listing all
commands flat. When the caller supplies no examples, `WriteRootHelp`
(`internal/help/generators.go:150-155`) SHALL auto-generate root examples via
`GenerateRootExamples`, which produces at most two examples — a "Run a command" example
always, plus a "Chain commands" example only in targ (non-binary) mode with two or more
command names available (`internal/help/generators.go:60-77`). This requirement covers only
composition facts not already addressed by the flag-grouping (Requirement: Global flags
render under one header with two nested subsections) or Formats (Requirement: The Formats
section documents CLI value-placeholder syntax, not output formats) requirements above.

#### Scenario: Root help with commands from two source packages and no user-supplied examples
- **WHEN** root help is rendered for a single-module binary whose registered targets come
  from two different source packages, with no examples supplied by the author
- **THEN** the `Commands:` section shows one `Source:` sub-header per package, and exactly
  two auto-generated examples appear, observed live as:
  ```
  Commands:

    Source: main
    greet


    Source: (unknown)
    math

  Examples:
    Run a command:
      targ-example-simple greet
    Chain commands:
      targ-example-simple greet math
  ```

### Requirement: Execution info renders Retry and Times as separate lines with a formatted backoff
Target help SHALL render retry information as `Retry: yes (backoff: %s × %.1f)` when a
positive backoff is configured, and MUST render `Times: <n>` as its own separate line
rather than merging the run count into the Retry line. This is implemented by
`appendRetryLine` (`internal/core/command.go:263-280`), which formats the backoff duration
and multiplier with `%s` and `%.1f` respectively, and `appendTimesLine`
(`internal/core/command.go:290-296`), which appends `Times: <n>` as an independent line.

#### Scenario: Target help for a target configured with Retry, Backoff, and Times
- **WHEN** `<binary> deploy --help` is run for a target registered with
  `.Retry().Backoff(1*time.Second, 2.0).Times(3)`
- **THEN** the `Execution:` section renders two separate lines, confirmed live under a PTY:
  ```
  Execution:
    Times: 3
    Retry: yes (backoff: 1s × 2.0)
  ```

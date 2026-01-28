# Consistent help output across all scenarios

## Problem

Help output is inconsistent across different invocation patterns:

1. **`targ --help`** (top-level) shows Values/Formats sections that aren't referenced by the flags they describe — orphaned context
2. **`targ --help --create`** (flag-command help) shows unnamed items in Flags section (targets, patterns, duration) without describing what they mean — unlike top-level which has Values/Formats
3. **`targ <target> --help`** (target help) shows generic chaining/completion examples, not examples of running that target — inconsistent with `--create` help which shows relevant examples
4. **`targ --help <target>`** must produce identical output to **`targ <target> --help`** — position shouldn't matter

## Help scenarios

Two categories of flags:

| Category | Flags | Help behavior |
|----------|-------|---------------|
| **Flag-commands** (runner early flags) | `--create`, `--sync`, `--to-func`, `--to-string` | Dedicated help page per flag-command |
| **Modifier flags** | `--timeout`, `--cache`, `--watch`, `--parallel`, etc. | Shown in global help and per-command help where they apply |

All help scenarios:

| Invocation | What shows | Where generated |
|------------|-----------|-----------------|
| `targ --help` | Global: description, usage, targ flags, formats, commands, examples | `core/command.go:printUsage()` |
| `targ --help <target>` / `targ <target> --help` | Target: description, source, command, usage, targ flags, target flags, execution, examples | `core/command.go:printCommandHelp()` |
| `targ --help --create` / `targ --create --help` | Create: description, usage, positionals, flags, examples | `runner/runner.go:PrintCreateHelp()` |
| `targ --help --sync` / `targ --sync --help` | Sync: description, usage, examples | `runner/runner.go:PrintSyncHelp()` |
| `targ --help --to-func` / `targ --to-func --help` | ToFunc: description, usage, examples | `runner/runner.go:PrintToFuncHelp()` |
| `targ --help --to-string` / `targ --to-string --help` | ToString: description, usage, examples | `runner/runner.go:PrintToStringHelp()` |

## Changes

### 1. `internal/core/command.go` — Fix Values/Formats and examples

**`printValuesAndFormats()`** — Make the sections reference which flags use them:
- Values: `shell` → mention it's for `--completion`
- Formats: `duration` → mention it's for `--timeout`, `--backoff`

**`printExamples()`** — Target-level examples should show usage of that specific command, not generic targ examples. Currently `chainExample()` shows chaining two random commands. At the target level, show the chain example using the _current_ command name, not arbitrary ones. The completion example is irrelevant at target level and should not appear.

Current target-level examples (non-root, no user-supplied examples):
```go
examples = []Example{chainExample(nil)}
```
This produces a generic example. Change to pass the current node so it uses the actual command name.

### 2. `internal/core/command.go` — Consistent section structure

Define the canonical section order for all help pages:

```
Description          (always first, always present)
                     (blank line)
Source: <file>       (target help only, if available)
Command: <shell>     (target help only, if shell target)
Usage: ...           (always present)
                     (blank line)
Targ flags:          (global + target help; modifier flags applicable at this level)
Values:              (global help only; references --completion)
Formats:             (global + target help where duration flags apply; references --timeout/--backoff)
Positionals:         (flag-command help only, if has positionals)
Flags:               (target help for target-specific flags; flag-command help for command flags)
Subcommands:         (target help only, if has subcommands)
Commands:            (global help only)
Execution:           (target help only; deps, timeout, retry, etc.)
Examples:            (always last section; relevant to this specific command/flag-command)
More info:           (global + target help)
```

### 3. `internal/runner/runner.go` — Already mostly done

The `PrintCreateHelp` etc. functions are already implemented. One fix needed:

**`PrintCreateHelp()`** — The Flags section has value placeholders (`<targets...>`, `<patterns...>`, `<duration>`) that are undefined. Add a brief Formats sub-note under the flags for `duration` (same as global help's Formats section), so a user reading `--create` help understands what `<duration>` means without needing to consult global help.

### 4. Tests — `internal/core/command_test.go` + `internal/runner/runner_help_test.go`

Add/update tests:
- **Core**: `printValuesAndFormats` references flags by name
- **Core**: target-level `printExamples` uses the target name, not generic examples
- **Runner**: `PrintCreateHelp` includes format note for duration

## Detailed changes

### `internal/core/command.go`

1. **`printValuesAndFormats()`** — Add flag references:
   ```
   Values:
     shell: bash, zsh, fish (for --completion; default: current shell (detected: zsh))

   Formats:
     duration: <int><unit> where unit is s, m, h (used by --timeout, --backoff)
   ```

2. **`printExamples()` at target level** — When no user-supplied examples, don't generate irrelevant generic examples (currently shows "Run multiple commands: targ build test" which has nothing to do with the target). Simply skip the Examples section entirely when there are no user-supplied examples at the target level. The target's description, source, shell command, and execution info are already informative.

   Change in `printExamples`: when `!isRoot` and `opts.Examples == nil`, return early without printing anything.

### `internal/runner/runner.go`

4. **`PrintCreateHelp()`** — Add a Formats note after the Flags section:
   ```
   Formats:
     duration    <int><unit> where unit is s (seconds), m (minutes), h (hours)
   ```

### `internal/core/command_test.go`

5. Test that `printValuesAndFormats` output contains flag references (`--completion`, `--timeout`, `--backoff`)
6. Test that target-level help with no user-supplied examples omits Examples section

### `internal/runner/runner_help_test.go`

7. Test that `PrintCreateHelp` contains Formats section with duration explanation

## Files

| File | Change |
|------|--------|
| `internal/core/command.go` | Fix Values/Formats to reference flags; skip irrelevant target-level examples |
| `internal/runner/runner.go` | Add Formats section to PrintCreateHelp |
| `internal/runner/runner_help_test.go` | Update tests for Formats section |
| `internal/core/command_test.go` | Add tests for Values/Formats flag refs and target-level examples |

## Verification

1. `go build ./...`
2. `go_diagnostics` on changed files
3. `go test ./internal/runner/ ./internal/core/`
4. `targ --help` — Values/Formats sections reference their flags
5. `targ --help --create` — includes Formats section for duration
6. `targ <target> --help` — no irrelevant generic examples
7. `targ --help <target>` — same output as `targ <target> --help`

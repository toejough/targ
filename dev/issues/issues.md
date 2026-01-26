# Issue Tracker

A simple md issue tracker.

## Statuses

- backlog (to choose from)
- selected (to work on next)
- in progress (currently being worked on)
- review (ready for review/testing)
- done (completed)
- cancelled (not going to be done, for whatever reason, should have a reason)
- blocked (waiting on something else)

---

## Backlog

Issues to choose from for future work.

### 59. sh.RunWith - run command with custom environment

#### Universal

**Status**
backlog

**Description**
Add `RunWith(env map[string]string, cmd string, args ...string) error` and `RunWithV` variant to run commands with custom environment variables.

#### Planning

**Priority**
Low

**Acceptance**
Can run commands with additional/overridden environment variables.

### 60. sh.ExitStatus - extract exit code from error

#### Universal

**Status**
backlog

**Description**
Add `ExitStatus(err error) int` to extract the exit code from an exec error. Returns 0 if nil, the exit code if available, or 1 for other errors.

#### Planning

**Priority**
Low

**Acceptance**
Can get numeric exit code from command errors for conditional logic.

### 61. sh.CmdRan - check if command actually ran

#### Universal

**Status**
backlog

**Description**
Add `CmdRan(err error) bool` to distinguish between "command not found" and "command ran but failed". Returns true if command executed (even with non-zero exit), false if command couldn't start.

#### Planning

**Priority**
Low

**Acceptance**
Can distinguish missing commands from failed commands.

### 62. sh.RunCmd / sh.OutCmd - reusable command functions

#### Universal

**Status**
backlog

**Description**
Add `RunCmd(cmd string, args ...string) func(args ...string) error` and `OutCmd` variant to create reusable command functions with pre-baked arguments.

#### Planning

**Priority**
Low

**Acceptance**
Can create command aliases like `git := sh.RunCmd("git")` then call `git("status")`.

### 63. sh.Copy - file copy helper

#### Universal

**Status**
backlog

**Description**
Add `Copy(dst, src string) error` to robustly copy a file, overwriting destination if it exists.

#### Planning

**Priority**
Low

**Acceptance**
Can copy files without using shell commands.

### 64. sh.Rm - file/directory removal helper

#### Universal

**Status**
backlog

**Description**
Add `Rm(path string) error` to remove a file or directory (recursively). No error if path doesn't exist.

#### Planning

**Priority**
Low

**Acceptance**
Can remove files/directories without using shell commands.

### 68. Init targets from remote repo

#### Universal

**Status**
backlog

**Description**
A command to initialize targets based on a remote repo's targets.

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 69. Update targets from remote repo

#### Universal

**Status**
backlog

**Description**
A command to update targets from a remote repo (sync with upstream template).

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 70. Make a CLI from a target

#### Universal

**Status**
backlog

**Description**
A command to generate a standalone CLI binary from a targ target.

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 72. --nest: create struct-based hierarchy from flat commands

#### Universal

**Status**
backlog

**Description**
Add --nest NAME CMD... flag to group flat commands under a new subcommand using struct-based hierarchy.

#### Planning

**Priority**
Medium

**Acceptance**
TBD

### 73. --flatten: pull subcommands up with parent prefix

#### Universal

**Status**
backlog

**Description**
Add --flatten NAME flag to pull subcommands up one level, adding parent name as prefix. Errors on naming conflict. Uses dotted syntax.

#### Planning

**Priority**
Medium

**Acceptance**
TBD

### 74. --to-struct: convert file-based hierarchy to struct-based

#### Universal

**Status**
backlog

**Description**
Add --to-struct NAME flag to convert file/directory-based hierarchy to struct-based. Deletes original files and pulls code into parent file. Uses dotted syntax.

#### Planning

**Priority**
Medium

**Acceptance**
TBD

### 75. --to-files: convert struct-based hierarchy to file-based

#### Universal

**Status**
backlog

**Description**
Add --to-files NAME flag to explode struct-based hierarchy into directory structure. Opposite of --to-struct. Uses dotted syntax.

#### Planning

**Priority**
Medium

**Acceptance**
TBD

### 76. --move: relocate command in hierarchy

#### Universal

**Status**
backlog

**Description**
Add --move CMD DEST flag to move a command to a different location. Uses dotted syntax (e.g., --move check.lint validate.passes.linter).

#### Planning

**Priority**
Medium

**Acceptance**
TBD

### 77. --rename: rename a command

#### Universal

**Status**
backlog

**Description**
Add --rename OLD NEW flag to rename a command. Uses dotted syntax for nested commands.

#### Planning

**Priority**
Medium

**Acceptance**
TBD

### 78. --delete: remove command or unexport if dependency

#### Universal

**Status**
backlog

**Description**
Add --delete CMD flag. If nothing depends on it, delete entirely. If used via targ.Deps(), make unexported instead. Uses dotted syntax.

#### Planning

**Priority**
Medium

**Acceptance**
TBD

### 79. --tree: show command hierarchy

#### Universal

**Status**
backlog

**Description**
Add --tree flag to display full command hierarchy as a tree. Does not show unexported dependencies.

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 80. --where: show command source location

#### Universal

**Status**
backlog

**Description**
Add --where CMD flag to show where a command is defined. Uses dotted syntax. Output shows file path and line number.

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 54. Syscall helpers

#### Universal

**Status**
cancelled

**Description**
Provide syscall helpers for common process control patterns beyond `targ/sh`.

#### Planning

**Priority**
Low

**Acceptance**
Helpers cover common use cases without requiring direct syscall usage.

**Note**
Replaced with specific issues for individual helpers (59-64) based on Mage sh package analysis.

### 56. Deterministic behavior across platforms

#### Universal

**Status**
cancelled

**Description**
Ensure consistent behavior across supported platforms (paths, env, ordering).

#### Planning

**Priority**
Medium

**Note**
No known issues. Platform-aware code already exists (sh.IsWindows, path handling). Reopen if specific issues arise.

**Acceptance**
Cross-platform test coverage proves deterministic behavior where expected.

### 15. .env File Loading

#### Universal

**Status**
cancelled

**Description**
Load `.env` files to populate env-backed flags.

#### Planning

**Priority**
Medium

**Acceptance**
Add `targ.LoadEnv()` or auto-load in `Run()`.

### 16. Interactive UI Helpers

#### Universal

**Status**
cancelled

**Description**
Add basic CLI interaction helpers (confirm/select/prompt).

#### Planning

**Priority**
Low

**Acceptance**
Provide a `ui` package for common prompts.

### 43. Investigate markdown parsers for issuefile

#### Universal

**Status**
cancelled

**Description**
Evaluate third-party markdown parsers (e.g., goldmark) to replace or augment the manual issuefile parser.

#### Planning

**Priority**
Low

**Acceptance**
Shortlist parsers and document tradeoffs; decide whether to keep manual parser.

### 65. Recursive help for all commands and subcommands

#### Universal

**Status**
cancelled

**Description**
Show help and usage recursively for all commands and subcommands.

#### Planning

**Priority**
Low

**Acceptance**
TBD

## Done

Completed issues.

### 9. Namespace/Category Organization

#### Universal

**Status**
done

**Description**
Allow grouping/namespace organization in help output for large command sets.

#### Planning

**Priority**
Low

**Acceptance**
Support grouping or display organization beyond strict struct nesting.

### 8. Persistent Flags & Lifecycle Hooks

#### Universal

**Status**
done

**Description**
Add persistent flags and setup/teardown hooks on parent commands.

#### Planning

**Priority**
Low

**Acceptance**
Support `PersistentBefore()`/`PersistentAfter()` and propagate flags down the tree.

### 1. Explore CLI Tool Design

#### Universal

**Status**
done

**Description**
Initial exploration and prototyping of the library.

### 7. Compilation-Safe Documentation

#### Universal

**Status**
done

**Description**
Generate wrapper structs for function commands so descriptions are embedded in compiled binaries.

#### Implementation Notes

**Details**
- Build tool mode auto-generates `generated_targ_<pkg>.go` with `Name`/`Description`.
- Direct binaries can opt in via `targ gen` and pass the generated struct to `Run`.

### 10. Custom Type Support (TextUnmarshaler)

#### Universal

**Status**
done

**Description**
Support types implementing `encoding.TextUnmarshaler` for flags.

#### Implementation Notes

**Details**
- Flag parsing supports types that implement `UnmarshalText` or `Set(string) error`.
- Positional parsing uses the same custom type logic.

### 6. Context Support & Timeout

#### Universal

**Status**
done

**Description**
Support cancellation/timeouts for long-running tasks.

#### Implementation Notes

**Details**
- `Run` methods accept `context.Context` and receive a root context.
- Function commands support `func(context.Context)` and `func(context.Context) error`.
- Root context is cancelled on SIGINT/SIGTERM in CLI runs.

### 2. Shell Execution Helpers (targ/sh)

#### Universal

**Status**
done

**Description**
Provide Mage-style helpers for shell execution to avoid verbose os/exec usage.

#### Implementation Notes

**Details**
- Added `sh.Run`, `sh.RunV`, and `sh.Output` for command execution.
- Included helpers for Windows executable suffix handling.

### 11. Default Value Tags

#### Universal

**Status**
done

**Description**
Add tag-based default values (e.g. `default="value"`).

#### Implementation Notes

**Details**
- Defaults now come exclusively from `default=...` tags.
- Passing non-zero command structs to `Run` returns a clear error.

### 3. Dependency Management (Once)

#### Universal

**Status**
done

**Description**
Allow targets to declare dependencies that run exactly once per execution graph.

#### Implementation Notes

**Details**
- Added `targ.Deps` to run dependencies once per CLI execution.
- Dependencies can be functions or struct command instances.

### 4. File Modification Checks (target)

#### Universal

**Status**
done

**Description**
Provide helpers for skipping work when outputs are newer than inputs.

#### Implementation Notes

**Details**
- Added `targ.Newer` with tag/glob matching and XDG-backed cache when outputs are omitted.

### 13. Watch Mode

#### Universal

**Status**
done

**Description**
Watch files and re-run commands on changes.

#### Implementation Notes

**Details**
- Added `targ.Watch` with polling, glob matching, and add/remove/modify detection.

### 5. Error Return Support

#### Universal

**Status**
done

**Description**
Allow `Run` methods to return error for consistent failure handling.

#### Implementation Notes

**Details**
- `Run` methods returning `error` propagate through `execute` and `RunWithOptions`.
- Niladic function commands returning `error` propagate similarly.

### 27. Build Tag Filtering For Build Tool Mode

#### Universal

**Status**
done

**Description**
Restrict command discovery to Go files with a specific build tag.

### 30. Function Targets Support (Direct + Build Tool Modes)

#### Universal

**Status**
done

**Description**
Support niladic functions as commands alongside struct-based commands.

### 31. Build Tool Mode Build-Tag Discovery

#### Universal

**Status**
done

**Description**
Discover commands only in directories containing files with `//go:build targ`.

### 32. Build Tool Mode Path-Based Namespacing

#### Universal

**Status**
done

**Description**
Namespace commands by file path: drop common leading segments and collapse single-child directories to produce the minimal prefix.

### 33. Build Tool Mode Package Grouping

#### Universal

**Status**
cancelled

**Description**
Superseded by file-based auto-namespacing (package names are no longer used as subcommand prefixes).

### 34. Build Tool Mode Subcommand Filtering For Functions

#### Universal

**Status**
done

**Description**
Filter out exported functions named as subcommands of exported structs.

### 24. Build Tool Mode Includes Non-Commands

#### Universal

**Status**
done

**Description**
Build tool mode includes exported structs without `Run` or subcommands.

#### Implementation Notes

**Details**
- Only include exported structs that define `Run` or declare subcommands.

### 18. Positional Args Are Also Registered As Flags

#### Universal

**Status**
done

**Description**
Fields tagged `targ:"positional"` are also registered as flags.

### 20. Required Tags Are Not Enforced

#### Universal

**Status**
done

**Description**
`targ:"required"` is parsed but never validated.

### 19. Struct Default Values Are Overwritten By Flag Defaults

#### Universal

**Status**
done

**Description**
Struct defaults are overwritten by zero/env defaults during flag registration.

### 21. Nil Pointer Inputs Can Panic

#### Universal

**Status**
done

**Description**
Nil pointers passed to `Run` or subcommand pointers can panic on `Elem()`.

### 22. Subcommand Assignment Fails For Non-Pointer Fields

#### Universal

**Status**
cancelled

**Description**
Subcommand assignment assumes pointer fields and can panic on value fields.

#### Planning

**Note**
Value-type subcommands are ambiguous (cannot distinguish "not called" from zero-value args). We require pointer subcommands for explicit invocation semantics.

### 23. Unexported Tagged Fields Can Panic

#### Universal

**Status**
done

**Description**
Unexported tagged fields panic when set via reflection.

### 25. Completion Tokenization Ignores Quotes/Escapes

#### Universal

**Status**
done

**Description**
Completion uses `strings.Fields` and breaks for quoted/escaped args.

### 37. require long flags to be --flag instead of -flag

#### Universal

**Status**
done

**Description**
Reject single-dash long flags in favor of `--flag`.

#### Implementation Notes

**Details**
- Validate args before flag parsing and return a clear error when `-flag` is used.

### 28. Build Tool Mode Compiled Binary Cache

#### Universal

**Status**
done

**Description**
Cache compiled build tool binaries to avoid `go run` on every invocation.

#### Implementation Notes

**Details**
- Build cached executables under `.targ/cache` with a content-based key.
- Add `--no-cache` to force rebuild.

### 26. Invalid Env Defaults Are Silently Ignored

#### Universal

**Status**
done

**Description**
Invalid env values for int/bool silently fall back to zero/false.

#### Implementation Notes

**Details**
- Validate env-backed defaults and return a clear error on invalid values.

### 35. shell completion is broken for fish

#### Universal

**Status**
done

**Description**
TBD

#### Implementation Notes

**Details**
- Use `--completion` flag and fix long/short flag suggestions in completion.
- Support enum value completion with `enum=` tags.

### 39. issues tasks need descriptions, and better usage strings, and some kind of list of what valid options are for the inputs where those are known & limited (like for status filtering)

#### Universal

**Status**
done

**Description**
TBD

#### Implementation Notes

**Details**
- Add flag descriptions and enum value lists to issues commands.
- Normalize status/priorities and align filtering with documented options.

### 40. help still shows single - flags

#### Universal

**Status**
done

**Description**
Help output shows single-dash long flags instead of --flag.

#### Planning

**Priority**
Low

**Acceptance**
TBD


**Details**
Render help output with --long (and -s where set) and skip positional fields in flag lists.

### 42. the issues list command doesn't have column headers, and seems to have a duplicate second and third column

#### Universal

**Status**
done

**Description**
Issues list output should include headers and avoid duplicate columns.

#### Planning

**Priority**
Low

**Acceptance**
TBD


**Details**
List now emits ID, Status, Title and removes the section filter/column.

### 41. help formatting shouldn't have double spaces between flags

#### Universal

**Status**
done

**Description**
TBD

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 12. Placeholder Customization

#### Universal

**Status**
done

**Description**
Allow placeholder text in help output (e.g. `placeholder="FILE"`).

#### Planning

**Priority**
Low

**Acceptance**
Support placeholder tag affecting help text.

### 44. rename to something more concise, like subcmd

#### Universal

**Status**
done

**Description**
TBD

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 29. Temporary Generated Main File Handling

#### Universal

**Status**
done

**Description**
Improve handling of generated bootstrap files (naming, location, cleanup).

#### Planning

**Priority**
Low

**Acceptance**
Generate into a temp dir with a stable name and support `--keep`.

---

### 14. Parallel Execution

#### Universal

**Status**
done

**Description**
Run independent tasks in parallel when safe.

#### Planning

**Priority**
Medium

**Acceptance**
Add a parallel execution helper integrated with dependencies.

### 17. Checksum-based Caching

#### Universal

**Status**
done

**Description**
Skip tasks when inputs have not changed content (checksum-based).

#### Planning

**Priority**
Low

**Acceptance**
Add `target.Checksum(srcs, dest)`.

### 36. add some help for targ itself when running in build-tool mode (add a description)

#### Universal

**Status**
done

**Description**
Fish completion script does not match current flag syntax.

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 38. enable combo flags like -abc for -a -b -c

#### Universal

**Status**
done

**Description**
Fish completion script does not match current flag syntax.

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 47. show build tool help for discovered commands and namespaces

#### Universal

**Status**
done

**Description**
Help should list root commands and top-level namespaces derived from file paths.

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 45. add methods for dynamically setting tags

#### Universal

**Status**
done

**Description**
TBD

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 48. rename to just targ

#### Universal

**Status**
done

**Description**
TBD

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 46. Publish repository

#### Universal

**Status**
done

**Description**
Push targ to GitHub and set up the canonical repo at github.com/toejough/targ.

#### Planning

**Priority**
Medium

**Acceptance**
Repository exists on GitHub and README link is valid.


**Details**
Needs GitHub repo creation and push permissions (github.com/toejough/targ). Awaiting user credentials or repo setup.

### 49. Repeated flags

#### Universal

**Status**
done

**Description**
Allow flags to be specified multiple times and accumulate values (e.g. `--tag a --tag b`).

#### Planning

**Priority**
Medium

**Acceptance**
Repeated flag values accumulate into slice fields with predictable ordering.

#### Design

**Basic repeated flags**: `[]T` fields accumulate values in order.
```go
type MyCmd struct {
    Tag []string `targ:"flag"`
}
// --tag a --tag b → Tag = []string{"a", "b"}
```

**Interleaved repeated flags**: `[]targ.Interleaved[T]` tracks global position across all flags.
```go
type MyCmd struct {
    Include []targ.Interleaved[string] `targ:"flag"`
    Exclude []targ.Interleaved[string] `targ:"flag"`
}
// --include a --exclude b --include c →
// Include = [{Value:"a", Position:0}, {Value:"c", Position:2}]
// Exclude = [{Value:"b", Position:1}]
```

User can merge slices and sort by Position to reconstruct interleaved order.

### 50. Variadic positionals

#### Universal

**Status**
done

**Description**
Support variadic positional args (e.g. a `[]string` positional capturing remaining args).

#### Planning

**Priority**
Medium

**Acceptance**
Trailing slice positional captures remaining args and appears clearly in usage/help.

#### Implementation Notes

**Details**
Already implemented - slice positionals automatically capture remaining args. Usage line shows placeholder with brackets (e.g., `[FILES...]`).

### 51. Map-type args

#### Universal

**Status**
done

**Description**
Support map-type flags or positionals with `key=value` parsing.

#### Planning

**Priority**
Low

**Acceptance**
Map fields can be populated via repeated `key=value` inputs with validation.

#### Implementation Notes

**Details**
- Syntax: `--labels key=value` repeated for multiple entries
- Supports `map[string]string`, `map[string]int`, etc.
- Values containing `=` are handled correctly (splits on first `=` only)
- Later values overwrite earlier ones for same key

### 52. Multi-command invocation

#### Universal

**Status**
done

**Description**
Allow invoking multiple commands in a single targ run (e.g. `targ build test deploy`).

#### Planning

**Priority**
Medium

**Acceptance**
Multiple commands execute in order with shared dependency semantics.

#### Implementation Notes

**Details**
Already implemented - commands execute in sequence and `Deps()` calls are shared across the entire run (dependencies only execute once).

### 53. Timeouts for build tool runs

#### Universal

**Status**
done

**Description**
Provide timeout controls for build-tool executions (CLI flag or API option).

#### Planning

**Priority**
Low

**Acceptance**
Timeout cancels the run and surfaces a clear error/exit code.

#### Implementation Notes

**Details**
- Added `--timeout <duration>` flag (e.g., `--timeout 10m`, `--timeout 1h`)
- Supports both `--timeout 10m` and `--timeout=10m` syntax
- Timeout is off by default (no timeout unless flag is specified)
- Context is cancelled when timeout expires, propagating to commands
- Clear error messages for missing/invalid duration values

### 55. README examples in sync

#### Universal

**Status**
done

**Description**
Add checks or tooling to keep README examples in sync with real behavior.

#### Planning

**Priority**
Low

**Acceptance**
CI or tooling verifies README examples compile/run or match output.

#### Implementation Notes

**Details**
Manually verified README examples. Fixed one issue: removed `sh.Which` example (function doesn't exist), replaced with `sh.RunV` example. Other examples verified correct.

### 57. Default completion flag in direct-binary mode

#### Universal

**Status**
done

**Description**
Expose a built-in --completion flag in direct-binary mode (like build-tool mode) so users don't need to wire it manually.

#### Planning

**Priority**
Medium

**Acceptance**
- `./mycli --completion bash` prints completion script
- `./mycli --completion=zsh` also works
- Shows in help output
- Errors if used after a command (like other unrecognized flags)

### 58. Suppress automatic global flags

#### Universal

**Status**
done

**Description**
Allow users to suppress automatic global flags (--help, --completion, --timeout) via RunOptions or similar mechanism.

#### Planning

**Priority**
Low

**Acceptance**
- Add RunOptions fields to disable individual global flags: `DisableHelp`, `DisableTimeout`, `DisableCompletion`
- Help output should not show disabled flags
- Disabled flags are treated as unknown flags (error)

#### Design

**Option A**: Individual booleans
```go
RunOptions{
    DisableHelp:       true,
    DisableCompletion: true,
    DisableTimeout:    true,
}
```

**Option B**: String slice
```go
RunOptions{
    DisableGlobalFlags: []string{"timeout", "completion"},
}
```

### 66. Bug: don't show completion in targ options for subcommands

#### Universal

**Status**
done

**Description**
Completion is shown as a targ option for subcommands, but it only works at the root level.

#### Planning

**Priority**
Low

**Acceptance**
TBD

#### Implementation Notes

**Details**
- Completion example now only shows at top level help (`targ --help`), not for subcommands
- Subcommand help still shows the command chaining example
- Usage line updated to show chaining pattern: `<subcommand>... [^ <command>...]`

### 67. Consistent help message structure

#### Universal

**Status**
done

**Description**
Targ options show at the top for targ --help, but at the bottom for subcommands. Use a consistent structure: usage, description, options, subcommand help, for-more-info.

#### Planning

**Priority**
Low

**Acceptance**
TBD


**Details**
Help structure is now consistent between top-level and subcommand help: description, usage, targ flags, commands/subcommands, examples, more info.

### 71. Recursively search for issues.md

#### Universal

**Status**
done

**Description**
When no --file is specified, recursively search upward for an issues.md file and use the first one found. This matches how other tools find their config files (like .git, go.mod, etc.).

#### Planning

**Priority**
Low

**Acceptance**
TBD


**Details**
Moved issues.md to dev/issues/issues.md and updated default file path in all issue commands.

### 81. Bug: targ fails when run from inside a target directory

#### Universal

**Status**
done

**Description**
When running targ from inside a target directory (e.g., dev/), the generated bootstrap calls functions without package qualifier. Bootstrap is package main but tries to call Check(ctx) instead of dev.Check(ctx).

#### Planning

**Priority**
Medium

**Acceptance**
TBD


**Details**
Fixed setupImport to always import target packages. The local optimization was wrong - bootstrap is always package main, so it must import packages to access their symbols.

## local-12: Shell command executes before flag validation completes [FIXED]

**Status**: closed
**Created**: 2026-01-26

### Description

When a shell command target receives an unknown flag, the shell command still executes before the error is returned. Flags should be fully validated before any execution happens.

### Reproduction

```go
target := targ.Targ("echo $msg").Name("echo")
_, err := targ.Execute(
    []string{"app", "--msg", "hello", "--unknown", "value"},
    target,
)
// Error is returned, but "echo hello" has already executed and printed to stdout
```

### Expected Behavior

Flag validation should complete before shell command execution. If there's an unknown flag, the command should not execute at all.

### Actual Behavior

The shell command executes (printing "hello" to stdout), then the unknown flag error is returned.

## local-13: Shell tests execute real commands instead of using DI

**Status**: open
**Created**: 2026-01-26

### Description

Tests in `test/shell_properties_test.go` execute real shell commands (`true`, `exit 1`, etc.) on the system instead of using dependency injection. Property-based tests should mock shell execution.

### Affected Tests

The following tests actually execute shell commands:
- `ShellCommandExecutesWithVariables` - runs `true $msg`
- `ShellCommandSupportsLongFlags` - runs `true $greeting $name`
- `ShellCommandSupportsEqualsFlags` - runs `true $msg`
- `ShellCommandSupportsShortFlags` - runs `true $msg`
- `ShellCommandUnknownFlagReturnsError` - runs `true $msg`
- `ShellCommandFailureReturnsError` - runs `exit 1`

### Solution

The shell execution layer (`internal/sh`) should be injectable. Tests should:
1. Verify the correct command string is constructed
2. Verify environment variables are passed correctly
3. Mock the exit code rather than running real commands

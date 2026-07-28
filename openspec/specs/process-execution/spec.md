# Process Execution Specification

## Purpose

This bounded context covers spawning external programs and shell commands, streaming or
capturing their output, and cleaning up child processes on cancellation. Its behavior lives
in `internal/sh/*` (the context-aware, platform-specific execution primitives), the `Command`
builder `Cmd()` in `internal/core/cmd.go`, and the shell-execution call sites in
`internal/core/target.go`, `internal/core/command.go`, and `internal/core/override.go`.

## Requirements

### Requirement: Windows child-process cleanup is best-effort and top-level only
On Windows, cancellation MUST terminate only the top-level child process; targ SHALL NOT use
job objects or any other mechanism to track or kill a spawned process's descendants.
`SetProcGroup` (`internal/sh/context_windows.go:19-23`) MUST be a no-op, and
`KillProcessGroup` (`internal/sh/context_windows.go:13-17`) MUST terminate the process via
`cmd.Process.Kill()` alone. Grandchild processes spawned by the child MAY continue running
after cancellation. This SHALL contrast with Unix, where `SetProcGroup`
(`internal/sh/context_unix.go:21-23`) puts the child in its own process group via
`SysProcAttr{Setpgid: true}` and `KillProcessGroup` (`internal/sh/context_unix.go:12-18`)
kills the entire group.

#### Scenario: Context cancellation on Windows during a running shell command
- **WHEN** a command spawned via `internal/sh` on Windows is running and its context is
  cancelled
- **THEN** targ kills only the top-level process via `cmd.Process.Kill()`, and any
  grandchild processes the top-level process spawned are left running

#### Scenario: Process-group setup on Windows before a command starts
- **WHEN** targ prepares to start a command on Windows
- **THEN** `SetProcGroup` performs no action, and the command is started without any
  process-group or job-object association

### Requirement: Three distinct paths construct context-aware subprocesses
targ MUST support three independent code paths for constructing and running a
context-aware subprocess, not a single shared construction path.

The `Command` builder returned by `Cmd()` SHALL be used for code-defined shell targets:
`Target.execute` dispatches a string-typed target function to `runShellCommand`, which calls
`Cmd("sh", "-c", cmd).Run(ctx)` (`internal/core/target.go:1085-1092`).

CLI shell-command targets SHALL instead run through `runShellWithVars`, which — whenever
`RunOptions.ShellRunner` is nil (the production default, since nothing sets it outside of
tests) — calls `internalsh.RunContextWithIO` directly, bypassing the `Command` builder
(`internal/core/command.go:2019-2053`).

`--while` condition checks SHALL use a third, independent path: `checkWhileCondition` calls
`exec.CommandContext(ctx, "sh", "-c", cmd)` directly from the standard library, bypassing both
the `Command` builder and the `internal/sh` package entirely (`internal/core/override.go:262-267`).

#### Scenario: Running a code-defined shell target
- **WHEN** a target's function is a shell string (e.g. defined via a code target with a
  string body)
- **THEN** `Target.execute` calls `runShellCommand`, which builds and runs the command
  through the `Cmd()` builder

#### Scenario: Running a CLI shell-command target with no injected runner
- **WHEN** a CLI-defined shell-command target runs and `RunOptions.ShellRunner` is nil
- **THEN** `runShellWithVars` calls `internalsh.RunContextWithIO` directly, without going
  through the `Cmd()` builder

#### Scenario: Evaluating a `--while` condition
- **WHEN** targ evaluates a `--while` condition string for iteration control
- **THEN** `checkWhileCondition` runs the condition via a raw `exec.CommandContext(ctx, "sh",
  "-c", cmd)` call, independent of both the `Cmd()` builder and `internal/sh`

### Requirement: Shell targets execute under POSIX sh, not the user's interactive shell
Every shell-execution site in targ MUST invoke shell command strings via `sh -c`, regardless
of the user's configured or interactive shell (bash, fish, zsh, etc.). This SHALL hold at all
three shell-execution sites: `internal/core/target.go:1085-1092`,
`internal/core/command.go:2017-2053`, and `internal/core/override.go:262-267`.

The `$SHELL` environment variable SHALL be consulted only to select a shell-completion script
via `detectCompletionShell` (`internal/core/run_env.go:247`), and MUST NOT influence which
interpreter runs a shell target's command string.

As a consequence, shell functions, aliases, or builtins that exist only in a user's
interactive shell (for example, fish's `string length` builtin) SHALL be unavailable to a
shell target and MUST fail with a POSIX-sh error such as `sh: string: command not found`
rather than being interpreted by that shell.

#### Scenario: A shell target using a POSIX-compatible command
- **WHEN** a shell target's command string uses only POSIX `sh` syntax and builtins
- **THEN** it runs correctly via `sh -c`, independent of the user's `$SHELL` setting

#### Scenario: A shell target using a fish-only builtin
- **WHEN** a shell target's command string invokes a construct that exists only in fish
  (e.g. `string length`) and the user's interactive shell is fish
- **THEN** execution still goes through `sh -c`, and the command fails with an error such as
  `sh: string: command not found` because `sh` does not understand fish syntax

#### Scenario: Shell selection for completions
- **WHEN** targ generates a shell-completion script
- **THEN** it reads `$SHELL` via `detectCompletionShell` to choose the completion script
  format, and this is the only place `$SHELL` affects targ's behavior

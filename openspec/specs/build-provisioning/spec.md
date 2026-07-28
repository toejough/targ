# Build Provisioning Specification

## Purpose

This bounded context covers the standalone `targ` binary's provisioning surface: scaffolding
new target declarations (`--create`), pointing the binary at a local targ-file directory
(`--source`), converting targets between string and function form (`--to-func`,
`--to-string`), importing a remote package for the first time (`--sync`), the discovery
precondition a package must satisfy to be scanned at all, and the top-level flag set the CLI
actually exposes. Its behavior lives in `internal/runner/runner.go`, `internal/flags/flags.go`,
and `internal/discover/discover.go`.

## Requirements

### Requirement: `--create --deps` validates only kebab-case syntax
`validateCreateOptions` (`internal/runner/runner.go:4288-4296`) SHALL validate each name
passed to `--deps` using only `IsValidTargetName` (`internal/runner/runner.go:606-608`), a
kebab-case syntax check. It SHALL NOT look up whether a dependency of that name already
exists as a target. A syntactically valid but nonexistent dependency name MUST be accepted at
create time; the resulting reference MUST surface later, if at all, as a Go compile error when
the generated file is built.

#### Scenario: Creating a target with a dependency that does not exist
- **WHEN** a user runs `targ --create hello --deps nonexistent-dep "echo hi"` in a directory
  with no target named `nonexistent-dep`
- **THEN** targ prints `Created target "hello" in <path>` and exits 0, writing
  `targ.Register(targ.Targ("echo hi").Name("hello").Deps(NonexistentDep))` into the target
  file
- **AND** the invalid reference is only discovered on a later run, as a Go build failure:
  `./build.go:14:56: undefined: NonexistentDep` /
  `Error building command: running go build: exit status 1`

### Requirement: `--source` always resolves to a local directory
The `--source` flag SHALL treat its value exclusively as a local filesystem path. targ SHALL
resolve it with `filepath.Abs` and confirm it exists and is a directory via `os.Stat`
(`internal/runner/runner.go:1927-1949`, `validateSourceDir`). No branch of this resolution
SHALL recognize or special-case a Go module path (e.g. `github.com/org/repo`); a value shaped
like a module path MUST be treated as a relative directory name and MUST fail if no such
directory exists under the current working directory. Fetching a remote module is the
responsibility of `--sync`, not `--source`.

#### Scenario: Passing a module-path-shaped value to --source
- **WHEN** a user runs `targ --source github.com/toejough/imptest` from a directory that has
  no subdirectory by that name
- **THEN** targ reports `Error with --source directory: directory does not exist:
  <cwd>/github.com/toejough/imptest` and exits non-zero, without attempting any network fetch

### Requirement: `--to-func` produces a zero-parameter wrapper function
`--to-func` SHALL convert a string target into a function target whose generated function
takes no parameters and whose sole statement is a `return targ.Run(...)` call, with the
original shell command's whitespace-split words passed as individual string literal
arguments (`generateShellFunc`, `internal/runner/runner.go:3585-3628`). targ SHALL NOT
generate an arguments struct, and SHALL NOT infer or emit any parameters for the function.

#### Scenario: Converting a string target to a function target
- **WHEN** a user runs `targ --to-func hello` against a target declared as
  `var Hello = targ.Targ("echo hi").Name("hello")`
- **THEN** targ prints `Converted target "hello" to function in <path>` and rewrites the file
  to add:
  ```go
  func hello() error {
  	return targ.Run("echo", "hi")
  }
  ```
- **AND** the existing `var Hello = targ.Targ(hello).Name("hello")` reference is rewritten to
  point at the new function identifier, with no args struct introduced anywhere

### Requirement: `--to-string` requires the function body to be a single targ.Run call
`--to-string` SHALL only convert a function target back to a string target when the target
function's body is exactly one statement, `return targ.Run(...)` with string-literal
arguments (`extractShellCommand`). Any other function body — additional statements, other
return expressions, or calls other than `targ.Run` — MUST be rejected with the error
`function %s is not a simple targ.Run call` (`internal/runner/runner.go:334-336`), where `%s`
is the function's identifier name. This precondition, not the shape of any `targ.Shell` API,
is what determines convertibility.

#### Scenario: Converting an eligible function target back to a string
- **WHEN** a user runs `targ --to-string hello` against `func hello() error { return
  targ.Run("echo", "hi") }`
- **THEN** targ prints `Converted target "hello" to string in <path>` and rewrites the
  declaration to `var Hello = targ.Targ("echo hi").Name("hello")`, removing the function

#### Scenario: Rejecting a function target with extra logic
- **WHEN** a user runs `targ --to-string hello` against a function whose body does more than
  return a single `targ.Run(...)` call (e.g. it also calls `fmt.Println` first)
- **THEN** targ reports `error converting target: function hello is not a simple targ.Run
  call` and exits non-zero, leaving the file unmodified

### Requirement: `--sync` performs first-time import only
`--sync` SHALL perform first-time integration of a remote package and SHALL NOT support
updating an already-synced package's targets or removing targets that vanished upstream.
`handleSyncFlag` (`internal/runner/runner.go:1671-1726`) SHALL check via
`CheckImportExists` whether the target package path is already imported and, if so, MUST
return the error `errPackageAlreadySynced` (`internal/runner/runner.go:951`, gate at
`:1698-1702`) and exit non-zero without re-fetching or re-inspecting the remote package.
`ParseSyncArgs` (`internal/runner/runner.go:730-745`) SHALL expose no `--force` or `--update`
flag, and no code path SHALL perform staleness detection against the remote. Per-package
opt-out from an already-synced import is performed by hand-editing a `DeregisterFrom` call,
which is not a sync operation.

#### Scenario: Syncing a package for the first time
- **WHEN** a user runs `targ --sync github.com/toejough/imptest` in a directory where that
  package has not previously been synced
- **THEN** targ fetches the module, prints `Synced "github.com/toejough/imptest" to
  <targfile>`, and exits 0

#### Scenario: Re-running --sync on an already-synced package
- **WHEN** the same user immediately re-runs `targ --sync github.com/toejough/imptest`
- **THEN** targ prints `package already synced: github.com/toejough/imptest` and exits
  non-zero, performing no fetch and making no changes

### Requirement: A package must explicitly register to be discovered
targ SHALL skip a candidate package entirely during discovery unless that package calls
`targ.Register(...)` from an `init()` function. `PackageInfo.UsesExplicitRegistration`
(`internal/discover/discover.go:50`, populated at `:197`) SHALL be computed per package, and
the runner SHALL skip any package where it is false (`internal/runner/runner.go:1337-1339`),
regardless of whether the package exports `targ.Targ(...)`-typed values. `--create` SHALL
scaffold the required `init()`/`Register` boilerplate automatically, which is why a
hand-written package can omit this call without the author noticing until discovery fails.

#### Scenario: A targ-tagged package with no Register call is skipped
- **WHEN** a `//go:build targ` package defines `var Build = targ.Targ(build)` but its file
  contains no `targ.Register(...)` call in any `init()`
- **THEN** targ prints `warning: skipping <dir>: package <name> does not use explicit
  registration (targ.Register in init)`, skips that package, and — if it was the only
  candidate package — subsequently reports `Error: no target files found`

### Requirement: `--create` has no short flag
`create`'s flag definition (`internal/flags/flags.go:129`) SHALL carry no `Short` field, so
`--create` MUST have no single-letter alias. Only `--help`/`-h`, `--source`/`-s`, and
`--parallel`/`-p` (`internal/flags/flags.go:54,56-63,72-76`) SHALL have short forms; `-c`
MUST be rejected as an undefined flag.

#### Scenario: Attempting to use -c as a short form of --create
- **WHEN** a user runs `targ -c`
- **THEN** targ reports `Error: flag provided but not defined: -c` and exits non-zero; it
  does not behave as `--create`

### Requirement: The CLI's flag-writing operations and removed-flag handling
The `targ` binary SHALL expose exactly these flag-driven, file-writing or file-selecting
top-level operations: `--create`, `--sync`, `--to-func`, `--to-string`, `--source`, and
`--completion` (`internal/flags/flags.go`). The `--init`, `--alias`, and `--move` flags SHALL
remain defined with a non-empty `Removed` field (`internal/flags/flags.go:156-172`,
`removedUseCreate` at `:206`) rather than being deleted from the flag table, so that invoking
any of them MUST produce the migration message `flag has been removed; use --create instead`
rather than the generic `flag provided but not defined` error given for a flag the CLI has
never heard of.

#### Scenario: Invoking a removed flag
- **WHEN** a user runs `targ --init`
- **THEN** targ reports `--init: flag has been removed; use --create instead` and exits
  non-zero

#### Scenario: Invoking a flag that never existed
- **WHEN** a user runs `targ --bogus-flag`
- **THEN** targ reports `Error: flag provided but not defined: --bogus-flag`, distinct from
  the removed-flag migration message

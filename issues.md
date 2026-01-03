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

### 8. Persistent Flags & Lifecycle Hooks

#### Universal

**Status**
backlog

**Description**
Add persistent flags and setup/teardown hooks on parent commands.

#### Planning

**Priority**
Low

**Acceptance**
Support `PersistentBefore()`/`PersistentAfter()` and propagate flags down the tree.

### 9. Namespace/Category Organization

#### Universal

**Status**
backlog

**Description**
Allow grouping/namespace organization in help output for large command sets.

#### Planning

**Priority**
Low

**Acceptance**
Support grouping or display organization beyond strict struct nesting.

### 12. Placeholder Customization

#### Universal

**Status**
backlog

**Description**
Allow placeholder text in help output (e.g. `placeholder="FILE"`).

#### Planning

**Priority**
Low

**Acceptance**
Support placeholder tag affecting help text.

### 14. Parallel Execution

#### Universal

**Status**
backlog

**Description**
Run independent tasks in parallel when safe.

#### Planning

**Priority**
Medium

**Acceptance**
Add a parallel execution helper integrated with dependencies.

### 15. .env File Loading

#### Universal

**Status**
backlog

**Description**
Load `.env` files to populate env-backed flags.

#### Planning

**Priority**
Medium

**Acceptance**
Add `commander.LoadEnv()` or auto-load in `Run()`.

### 16. Interactive UI Helpers

#### Universal

**Status**
backlog

**Description**
Add basic CLI interaction helpers (confirm/select/prompt).

#### Planning

**Priority**
Low

**Acceptance**
Provide a `ui` package for common prompts.

### 17. Checksum-based Caching

#### Universal

**Status**
backlog

**Description**
Skip tasks when inputs have not changed content (checksum-based).

#### Planning

**Priority**
Low

**Acceptance**
Add `target.Checksum(srcs, dest)`.

### 29. Temporary Generated Main File Handling

#### Universal

**Status**
backlog

**Description**
Improve handling of generated bootstrap files (naming, location, cleanup).

#### Planning

**Priority**
Low

**Acceptance**
Generate into a temp dir with a stable name and support `--keep`.

---

### 35. shell completion is broken for fish

#### Universal

**Status**
backlog

**Description**
TBD

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 36. add some help for commander itself when running in build-tool mode (add a description)

#### Universal

**Status**
backlog

**Description**
TBD

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 38. enable combo flags like -abc for -a -b -c

#### Universal

**Status**
backlog

**Description**
TBD

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 39. issues tasks need descriptions, and better usage strings, and some kind of list of what valid options are for the inputs where those are known & limited (like for status filtering)

#### Universal

**Status**
backlog

**Description**
TBD

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 40. help still shows single - flags

#### Universal

**Status**
backlog

**Description**
TBD

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 41. help formatting shouldn't have double spaces between flags

#### Universal

**Status**
backlog

**Description**
TBD

#### Planning

**Priority**
Low

**Acceptance**
TBD

### 42. the issues list command doesn't have column headers, and seems to have a duplicate second and third column

#### Universal

**Status**
backlog

**Description**
TBD

#### Planning

**Priority**
Low

**Acceptance**
TBD

## Done

Completed issues.

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
- Build tool mode auto-generates `generated_commander_<pkg>.go` with `Name`/`Description`.
- Direct binaries can opt in via `commander gen` and pass the generated struct to `Run`.

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

### 2. Shell Execution Helpers (commander/sh)

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
- Added `commander.Deps` to run dependencies once per CLI execution.
- Dependencies can be functions or struct command instances.

### 4. File Modification Checks (target)

#### Universal

**Status**
done

**Description**
Provide helpers for skipping work when outputs are newer than inputs.

#### Implementation Notes

**Details**
- Added `commander.Newer` with tag/glob matching and XDG-backed cache when outputs are omitted.

### 13. Watch Mode

#### Universal

**Status**
done

**Description**
Watch files and re-run commands on changes.

#### Implementation Notes

**Details**
- Added `commander.Watch` with polling, glob matching, and add/remove/modify detection.

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
Discover commands only in directories containing files with `//go:build commander`.

### 32. Build Tool Mode Depth Gating

#### Universal

**Status**
done

**Description**
Without `--multipackage`, stop at the first depth with tagged files and error on ties.

### 33. Build Tool Mode Package Grouping

#### Universal

**Status**
done

**Description**
When `--multipackage` is set, always add package name as the first subcommand.

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
Fields tagged `commander:"positional"` are also registered as flags.

### 20. Required Tags Are Not Enforced

#### Universal

**Status**
done

**Description**
`commander:"required"` is parsed but never validated.

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
- Build cached executables under `.commander/cache` with a content-based key.
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

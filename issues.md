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

### 2. Shell Execution Helpers (commander/sh)

#### Universal

**Status**
backlog

**Description**
Provide Mage-style helpers for shell execution to avoid verbose os/exec usage.

#### Planning

**Priority**
High

**Acceptance**
Expose `sh.Run`, `sh.Output`, and `sh.RunV` plus basic cross-platform helpers.

### 3. Dependency Management (Once)

#### Universal

**Status**
backlog

**Description**
Allow targets to declare dependencies that run exactly once per execution graph.

#### Planning

**Priority**
Medium

**Acceptance**
Add `commander.Deps` (or equivalent) to coordinate run-once dependencies.

### 4. File Modification Checks (target)

#### Universal

**Status**
backlog

**Description**
Provide helpers for skipping work when outputs are newer than inputs.

#### Planning

**Priority**
Medium

**Acceptance**
Add `commander.Newer(src, dst)` or equivalent timestamp checks.

### 5. Error Return Support

#### Universal

**Description**
Allow `Run` methods to return error for consistent failure handling.

### 6. Context Support & Timeout

#### Universal

**Status**
backlog

**Description**
Support cancellation/timeouts for long-running tasks.

#### Planning

**Priority**
Medium

**Acceptance**
Support `func(context.Context)` and set up signal-canceling root context.

### 7. Compilation-Safe Documentation

#### Universal

**Status**
backlog

**Description**
Help currently parses source files at runtime, which breaks for relocated binaries.

#### Planning

**Priority**
High

**Acceptance**
Either require `desc` tags for binaries or embed comments in build tool mode.

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

### 10. Custom Type Support (TextUnmarshaler)

#### Universal

**Status**
backlog

**Description**
Support types implementing `encoding.TextUnmarshaler` for flags.

#### Planning

**Priority**
High

**Acceptance**
Parse flags into custom types via `UnmarshalText` or `Set(string) error`.

### 11. Default Value Tags

#### Universal

**Status**
backlog

**Description**
Add tag-based default values (e.g. `default="value"`).

#### Planning

**Priority**
Low

**Acceptance**
Support `default` tags for auto-instantiated subcommands.

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

### 13. Watch Mode

#### Universal

**Status**
backlog

**Description**
Watch files and re-run commands on changes.

#### Planning

**Priority**
Medium

**Acceptance**
Implement `commander.Watch` with cancellation and globbing support.

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

### 21. Nil Pointer Inputs Can Panic

#### Universal

**Description**
Nil pointers passed to `Run` or subcommand pointers can panic on `Elem()`.

#### Planning

**Priority**
High

**Acceptance**
Validate pointers before `Elem()` and return descriptive errors.

### 23. Unexported Tagged Fields Can Panic

#### Universal

**Status**
backlog

**Description**
Unexported tagged fields panic when set via reflection.

#### Planning

**Priority**
Medium

**Acceptance**
Validate field export status and return friendly errors.

### 24. Build Tool Mode Includes Non-Commands

#### Universal

**Status**
backlog

**Description**
Build tool mode includes exported structs without `Run` or subcommands.

#### Planning

**Priority**
Low

**Acceptance**
Filter to runnable structs or those with subcommands.

### 25. Completion Tokenization Ignores Quotes/Escapes

#### Universal

**Status**
backlog

**Description**
Completion uses `strings.Fields` and breaks for quoted/escaped args.

#### Planning

**Priority**
Low

**Acceptance**
Use a shell-aware tokenizer for completion input.

### 26. Invalid Env Defaults Are Silently Ignored

#### Universal

**Status**
backlog

**Description**
Invalid env values for int/bool silently fall back to zero/false.

#### Planning

**Priority**
Low

**Acceptance**
Validate env parsing and surface errors or warnings.

### 28. Build Tool Mode Compiled Binary Cache

#### Universal

**Status**
backlog

**Description**
Cache compiled build tool binaries to avoid `go run` on every invocation.

#### Planning

**Priority**
Medium

**Acceptance**
Generate a cache key and reuse the compiled executable when valid.

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

## Done

Completed issues.

### 1. Explore CLI Tool Design

#### Universal

**Status**
done

**Description**
Initial exploration and prototyping of the library.

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
Without `--package`, stop at the first depth with tagged files and error on ties.

### 33. Build Tool Mode Package Grouping

#### Universal

**Status**
done

**Description**
When `--package` is set, always add package name as the first subcommand.

### 34. Build Tool Mode Subcommand Filtering For Functions

#### Universal

**Status**
done

**Description**
Filter out exported functions named as subcommands of exported structs.

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

### 5. Error Return Support

#### Universal

**Status**
done

**Description**
Allow `Run` methods to return error for consistent failure handling.

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

# Issues

## Active

### ISSUE-001: Glob patterns in target paths
**Status:** Open
**Created:** 2026-01-30

Support glob patterns like `targ dev *` and `targ **` for matching multiple targets.

---

### ISSUE-002: sh.RunWith - run command with custom environment
**Status:** Open
**Created:** 2026-01-30

Add `RunWith(env map[string]string, cmd string, args ...string) error` and `RunWithV` variant to run commands with custom environment variables.

---

### ISSUE-003: sh.ExitStatus - extract exit code from error
**Status:** Open
**Created:** 2026-01-30

Add `ExitStatus(err error) int` to extract the exit code from an exec error. Returns 0 if nil, the exit code if available, or 1 for other errors.

---

### ISSUE-004: sh.CmdRan - check if command actually ran
**Status:** Open
**Created:** 2026-01-30

Add `CmdRan(err error) bool` to distinguish between "command not found" and "command ran but failed". Returns true if command executed (even with non-zero exit), false if command couldn't start.

---

### ISSUE-005: sh.RunCmd / sh.OutCmd - reusable command functions
**Status:** Open
**Created:** 2026-01-30

Add `RunCmd(cmd string, args ...string) func(args ...string) error` and `OutCmd` variant to create reusable command functions with pre-baked arguments. Example: `git := sh.RunCmd("git")` then call `git("status")`.

---

### ISSUE-006: sh.Copy - file copy helper
**Status:** Open
**Created:** 2026-01-30

Add `Copy(dst, src string) error` to robustly copy a file, overwriting destination if it exists.

---

### ISSUE-007: sh.Rm - file/directory removal helper
**Status:** Open
**Created:** 2026-01-30

Add `Rm(path string) error` to remove a file or directory (recursively). No error if path doesn't exist.

---

### ISSUE-008: Init targets from remote repo
**Status:** Open
**Created:** 2026-01-30
**Traces to:** REQ-033, REQ-056

A command to initialize targets based on a remote repo's targets.

---

### ISSUE-009: Update targets from remote repo
**Status:** Open
**Created:** 2026-01-30
**Traces to:** REQ-035, REQ-057

A command to update targets from a remote repo (sync with upstream template).

---

### ISSUE-010: Make a CLI from a target
**Status:** Open
**Created:** 2026-01-30
**Traces to:** REQ-023, ARCH-006

A command to generate a standalone CLI binary from a targ target.

---

### ISSUE-011: --nest flag for struct-based hierarchy
**Status:** Open
**Created:** 2026-01-30
**Traces to:** REQ-050

Add `--nest NAME CMD...` flag to group flat commands under a new subcommand using struct-based hierarchy.

---

### ISSUE-012: --flatten flag to pull subcommands up
**Status:** Open
**Created:** 2026-01-30
**Traces to:** REQ-050

Add `--flatten NAME` flag to pull subcommands up one level, adding parent name as prefix. Errors on naming conflict. Uses dotted syntax.

---

### ISSUE-013: --to-struct flag for hierarchy conversion
**Status:** Open
**Created:** 2026-01-30
**Traces to:** REQ-051

Add `--to-struct NAME` flag to convert file/directory-based hierarchy to struct-based. Deletes original files and pulls code into parent file. Uses dotted syntax.

---

### ISSUE-014: --to-files flag for hierarchy conversion
**Status:** Open
**Created:** 2026-01-30
**Traces to:** REQ-051

Add `--to-files NAME` flag to explode struct-based hierarchy into directory structure. Opposite of --to-struct. Uses dotted syntax.

---

### ISSUE-015: --move flag to relocate commands
**Status:** Open
**Created:** 2026-01-30
**Traces to:** REQ-050

Add `--move CMD DEST` flag to move a command to a different location. Uses dotted syntax (e.g., `--move check.lint validate.passes.linter`).

---

### ISSUE-016: --rename flag for commands
**Status:** Open
**Created:** 2026-01-30
**Traces to:** REQ-050

Add `--rename OLD NEW` flag to rename a command. Uses dotted syntax for nested commands.

---

### ISSUE-017: --delete flag for commands
**Status:** Open
**Created:** 2026-01-30
**Traces to:** REQ-052

Add `--delete CMD` flag. If nothing depends on it, delete entirely. If used via targ.Deps(), make unexported instead. Uses dotted syntax.

---

### ISSUE-018: --tree flag to show command hierarchy
**Status:** Open
**Created:** 2026-01-30
**Traces to:** REQ-060

Add `--tree` flag to display full command hierarchy as a tree. Does not show unexported dependencies.

---

### ISSUE-019: --where flag to show command source
**Status:** Open
**Created:** 2026-01-30
**Traces to:** REQ-059

Add `--where CMD` flag to show where a command is defined. Uses dotted syntax. Output shows file path and line number.

---

### ISSUE-020: Add tests for CLI binary transition (ARCH-006)
**Status:** Open
**Created:** 2026-01-31
**Traces to:** ARCH-006, REQ-023

ARCH-006 (CLI Binary Transition) has no test coverage. Need tests verifying the `targ.Run()` entry point workflow for converting targ targets to standalone CLI binaries.

---

### ISSUE-021: --create generates invalid targ files (uses old API)
**Status:** Open
**Created:** 2026-02-01

The `targ --create` command generates targ files using the old variable-based API instead of the required explicit registration API. The generated files fail immediately when running targ.

**Reproduction:**
```bash
mkdir /tmp/targ-repro && cd /tmp/targ-repro
go mod init example.com/repro
targ --create test "echo hello"
targ  # fails
```

**Generated (incorrect):**
```go
//go:build targ

package targrepro

import "github.com/toejough/targ"

var _ = targ.Targ
var Test = targ.Targ("echo hello").Name("test")
```

**Expected:**
```go
//go:build targ

package targrepro

import "github.com/toejough/targ"

func init() {
    targ.Register(targ.Targ("echo hello").Name("test"))
}
```

**Error:**
```
error preparing bootstrap: package does not use explicit registration (targ.Register in init): targrepro
```

---

## Completed

---

## Blocked

# Fix --help Source Attribution

## Problem

Issue #9: `targ --help` shows two source attribution problems:

1. **Remote targets show module cache paths** like `../../../go/pkg/mod/github.com/toejough/targ@v0.0.0-.../dev/targets.go` instead of clean package references.
2. **String/deps-only targets show `(unknown)`** because they lack a function pointer for `funcSourceFile()` to reflect on.

## Root Cause

- `funcSourceFile()` uses `runtime.FuncForPC` to get file paths — only works for function targets.
- String targets (`targ.Targ("echo hello")`) and deps-only targets (`targ.Targ()`) have no function pointer.
- Remote function targets get absolute module cache paths, which `relativeSourcePathWithGetwd` turns into ugly relative paths.
- The `sourcePkg` field on `Target` already has clean package paths (e.g., `github.com/toejough/targ/dev`) but isn't used for display.

## Design

### 1. Capture source file in `Targ()` for non-function targets

Add a `sourceFile` field to `Target`. In `Targ()`, for string and deps-only args, capture the defining file path via `runtime.Caller(1)`.

Function targets don't need this — `funcSourceFile()` already gets their path via reflection.

### 2. Prefer `sourcePkg` over file path for remote targets

In `parseTargetLike()`, after creating the commandNode:
- If target has non-empty `GetSource()` (i.e., it's a remote registered target), use that as `node.SourceFile`.
- Else if target has non-empty `GetSourceFile()` (local string/deps-only), use that.
- Else keep existing behavior (function targets use `funcSourceFile()`).

### 3. Expose source file via `GetSourceFile()`

Add `GetSourceFile() string` method to `Target` and the `TargetLike` interface (or use a type assertion).

## Result Matrix

| Case | Before | After |
|------|--------|-------|
| Local function target | `./targs.go` | `./targs.go` (unchanged) |
| Local string target | `(unknown)` | `./targs.go` |
| Local deps-only target | `(unknown)` | `./targs.go` |
| Remote function target | `../../../go/pkg/mod/...` | `github.com/toejough/targ/dev` |
| Remote string target | `(unknown)` | `github.com/toejough/targ/dev` |
| Remote deps-only target | `(unknown)` | `github.com/toejough/targ/dev` |

## Files Changed

- `internal/core/target.go` — Add `sourceFile` field, `GetSourceFile()`, capture in `Targ()`
- `internal/core/command.go` — Use `sourcePkg`/`sourceFile` in `parseTargetLike()`
- `internal/core/export_test.go` — Update test helper if needed
- `internal/core/target_test.go` — Tests for source file capture
- `internal/core/command_test.go` or `test/` — Integration tests for source display

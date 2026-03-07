# Architecture: UC-16 Directory Tree Traversal

Traces to: REQ-1, REQ-2, REQ-3, REQ-4, REQ-5, REQ-6, REQ-7, DES-1, DES-2, DES-3, DES-4, DES-5

## Overview

This feature modifies one package (`discover`) and one package (`runner`). No changes to `core`, `help`, `flags`, `file`, `sh`, or `parse`. The existing multi-module build path handles most of the complexity.

## ARCH-1: Extend discover.Discover() with upward walk

**Package:** `internal/discover`
**File:** `discover.go`

**Current:** `Discover(startDir, fs)` calls `findTaggedDirs()` which walks DOWN from `startDir` recursively.

**Change:** Add a new function `discoverAncestors(startDir, fs)` that:
1. Walks UP from `filepath.Dir(startDir)` to filesystem root
2. At each ancestor directory, checks for targ-tagged `.go` files in that directory (non-recursive — just files directly in that dir)
3. If `<ancestor>/dev/` exists, calls existing `findTaggedDirs()` on it (full recursive walk)
4. Returns `[]PackageInfo` for all ancestor-discovered packages

**Integration:** `Discover()` calls both `findTaggedDirs()` (down) and `discoverAncestors()` (up), then merges results. Deduplication by directory path — if CWD is inside an ancestor's `dev/` subtree, don't double-count.

**Satisfies:** REQ-1 (upward discovery), REQ-2 (dev/ walk), REQ-3 (no siblings), REQ-7 (downward unchanged)

## ARCH-2: Single-directory discovery helper

**Package:** `internal/discover`
**File:** `discover.go`

**New function:** `findTaggedFilesInDir(dir, fs)` — checks a single directory (non-recursive) for targ-tagged `.go` files. This is a subset of `findTaggedDirs()` that doesn't recurse into subdirectories.

Needed because the upward walk checks each ancestor directory itself without recursing into its children (except `dev/`).

**Satisfies:** REQ-1, REQ-3

## ARCH-3: Isolated build per ancestor directory

**Package:** `internal/runner`
**File:** `runner.go`

**Current:** `groupByModule()` groups unmoduled packages under `startDir` with path `"targ.local"`. The `EnsureFallbackModuleRoot()` function symlinks from `startDir`.

**Change:** Modify `groupByModule()` to group unmoduled packages by their actual containing directory rather than always under `startDir`. Each distinct directory with unmoduled targets becomes its own module group with:
- `moduleRoot` = the ancestor directory
- `modulePath` = `"targ.local"` (same synthetic name)
- Distinct hash for cache key (based on actual directory, not startDir)

This means ancestor directories without go.mod produce separate module groups, which the existing `handleMultiModule()` path builds as separate binaries.

**Satisfies:** REQ-4, DES-2, DES-3

## ARCH-4: Cache key includes ancestor path

**Package:** `internal/runner`
**File:** `runner.go`

The binary cache key for each build unit must be stable across different CWDs that discover the same ancestor targets.

**Current:** `setupBinaryPath()` uses `projectCacheDir(importRoot)` which hashes the import root path.

**No change needed** — if `importRoot` is set to the ancestor directory (per ARCH-3), the cache key naturally stabilizes. Two different CWDs discovering the same ancestor directory will compute the same `importRoot` and therefore the same cache path.

**Satisfies:** DES-4

## ARCH-5: No changes to conflict detection or help

**Packages:** `internal/core`, `internal/help`

No architectural changes needed. Ancestor targets flow through the same:
- `detectConflicts()` in `core/registry.go` — same-name targets from different packages produce `ConflictError`
- Help rendering in `help/` — source attribution from `sourcePkg` field naturally distinguishes ancestor targets

**Satisfies:** REQ-5, REQ-6

## Component Dependency Graph

```
discover.Discover()           -- MODIFIED (ARCH-1)
  ├── findTaggedDirs()        -- UNCHANGED (downward walk)
  ├── findTaggedFilesInDir()  -- NEW (ARCH-2, single-dir check)
  └── discoverAncestors()     -- NEW (ARCH-1, upward walk + dev/)

runner.groupByModule()        -- MODIFIED (ARCH-3)
  └── uses per-directory grouping instead of startDir fallback

runner.handleMultiModule()    -- UNCHANGED (handles multiple build units)
runner.handleIsolatedModule() -- UNCHANGED (builds with synthetic go.mod)
core.detectConflicts()        -- UNCHANGED
help rendering                -- UNCHANGED
```

## Behavioral Contracts

1. **Discovery contract:** `Discover()` returns `[]PackageInfo` from both downward and upward walks. Order: downward results first, then upward from nearest ancestor to root. No duplicates by directory.

2. **Module grouping contract:** Each directory with targ files and no go.mod gets its own module group. Directories sharing the same go.mod (via upward walk) are grouped together as today.

3. **Build contract:** Each module group builds independently. Multi-module aggregation combines all commands. Binary cache uses the build unit's root directory as key input, not CWD.

4. **Conflict contract:** Unchanged — same-name targets from different source packages error regardless of tree level.

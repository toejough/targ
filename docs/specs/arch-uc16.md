# Architecture: UC-16 Directory Tree Traversal

Traces to: UC-16 (Issue #11), [REQ/DES](req-des-uc16.md)

## Component Dependency Graph

```
Discover (internal/discover/discover.go)
  ├── discoverAncestors()  [NEW: ARCH-16-1]
  │     └── findTaggedFilesInDir()  [NEW: ARCH-16-2]
  │     └── findTaggedDirs()  [EXISTING: recursive dev/ walk]
  └── findTaggedDirs()  [EXISTING: downward walk, unchanged]

Runner (internal/runner/runner.go)
  ├── groupByModule()  [MODIFIED: ARCH-16-3]
  │     └── FindModuleForPath()  [EXISTING]
  └── setupBinaryPath()  [EXISTING: ARCH-16-4, no change needed]

Help (internal/help/builder.go, render.go)  [EXISTING: ARCH-16-5, no change needed]

Core (internal/core/registry.go)  [EXISTING: ARCH-16-5, no change needed]
```

## ARCH-16-1: Extend Discover() with upward walk

**Traces to:** REQ-16-1, REQ-16-2, REQ-16-3, DES-16-1

**Source files:** `internal/discover/discover.go` (`Discover`, new `discoverAncestors`)

**Change:** Add `discoverAncestors()` function that walks from `CWD/..` up to filesystem root. At each ancestor directory:

1. Call `findTaggedFilesInDir()` (ARCH-16-2) to check for targ-tagged `.go` files in that directory only (non-recursive).
2. If `<ancestor>/dev/` exists and is a directory, call the existing `findTaggedDirs()` on it for full recursive discovery.
3. Skip all other subdirectories of the ancestor — no sibling directory traversal (REQ-16-3).

Modify `Discover()` to call `discoverAncestors()` after the existing downward `findTaggedDirs()` call, then merge results. Dedup by directory path to handle edge cases where downward and upward walks overlap.

**Dependencies:** ARCH-16-2

## ARCH-16-2: Single-directory discovery helper

**Traces to:** REQ-16-1, REQ-16-3, DES-16-1

**Source files:** `internal/discover/discover.go` (new `findTaggedFilesInDir`)

**Change:** Add `findTaggedFilesInDir()` that reads a single directory (via `FileSystem.ReadDir`) and checks each `.go` file for the targ build tag. It does NOT recurse into subdirectories. Returns a `taggedDir` if any tagged files are found.

This reuses the existing `tryReadTaggedFile()` helper for file-level checks. The difference from `processDirectory()` is that subdirectory entries are ignored entirely rather than queued.

**Dependencies:** None (uses existing `tryReadTaggedFile`)

## ARCH-16-3: Isolated build per ancestor directory

**Traces to:** REQ-16-4, DES-16-2, DES-16-3

**Source files:** `internal/runner/runner.go` (`groupByModule`)

**Change:** In `groupByModule()`, when `FindModuleForPath()` returns `found == false`, use the package's own directory (`info.Dir`) as `modRoot` instead of `startDir`. Current code (line 3326):

```go
modRoot = startDir
```

Changes to:

```go
modRoot = info.Dir
```

This ensures each distinct ancestor directory without a `go.mod` becomes its own module group with its own isolated build context, rather than all unmoduled packages collapsing into `startDir`.

The rest of the isolated build pipeline (`createIsolatedBuildDir`, `handleIsolatedModule`, `writeIsolatedGoMod`) works unchanged — it already handles arbitrary `modRoot` values.

**Dependencies:** ARCH-16-1 (ancestor packages must be discovered first)

## ARCH-16-4: Cache key stability

**Traces to:** REQ-16-4, DES-16-4

**Source files:** `internal/runner/runner.go` (`setupBinaryPath`)

**Change:** No code change needed. `setupBinaryPath()` uses `importRoot` (which equals the module root directory) to compute `projectCacheDir`. With ARCH-16-3, each ancestor directory becomes its own `modRoot`, so `importRoot` is the ancestor directory path. This naturally produces stable cache keys regardless of which CWD the user invokes `targ` from.

**Dependencies:** ARCH-16-3

## ARCH-16-5: No changes to conflict detection or help

**Traces to:** REQ-16-5, REQ-16-6, REQ-16-7

**Source files:** `internal/core/registry.go`, `internal/help/builder.go`, `internal/help/render.go`

**Change:** No code changes needed. Existing mechanisms handle ancestor targets naturally:

- **Conflict detection:** `ConflictError` in `internal/core/registry.go` fires when two packages register the same target name, regardless of where the packages originate. Ancestor targets flow through the same registration path.
- **Help output:** `internal/help/builder.go` and `internal/help/render.go` render all registered targets with source attribution. Ancestor targets appear automatically once discovered and built.
- **Downward discovery:** The existing `findTaggedDirs()` BFS walk is called unchanged for the CWD subtree (REQ-16-7).

**Dependencies:** None

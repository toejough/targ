# Test List: UC-16 Directory Tree Traversal

Traces to: UC-16 (Issue #11), [ARCH](arch-uc16.md), [REQ/DES](req-des-uc16.md)

## T-16-1: Upward discovery finds ancestor targ files

**Traces to:** ARCH-16-1, REQ-16-1

- **Given** a directory tree where an ancestor directory contains a targ-tagged `.go` file
- **When** `Discover()` is called with `StartDir` set to a descendant directory
- **Then** the returned `PackageInfo` list includes the ancestor's package
- **Property:** Every ancestor directory containing targ-tagged files appears in discovery results

## T-16-2: Upward discovery walks dev/ subdirectories

**Traces to:** ARCH-16-1, REQ-16-2

- **Given** an ancestor directory with a `dev/` subdirectory containing targ-tagged files (possibly nested)
- **When** `Discover()` is called from a descendant directory
- **Then** packages from `<ancestor>/dev/` and its subdirectories are included in results
- **Property:** For every ancestor with a `dev/` subtree, all targ-tagged packages in that subtree are discovered

## T-16-3: Upward discovery excludes sibling directories

**Traces to:** ARCH-16-1, REQ-16-3

- **Given** a directory tree where the CWD's parent has sibling directories containing targ-tagged files
- **When** `Discover()` is called from the CWD
- **Then** packages from sibling directories (other than `dev/`) are NOT included
- **Property:** No discovered package has a path that is a sibling (non-ancestor, non-descendant) of any directory on the CWD-to-root path

## T-16-4: Single-directory discovery finds only direct files

**Traces to:** ARCH-16-2

- **Given** a directory containing targ-tagged files and subdirectories also containing targ-tagged files
- **When** `findTaggedFilesInDir()` is called on that directory
- **Then** only files directly in that directory are returned, not files in subdirectories
- **Property:** All returned file paths have the queried directory as their immediate parent

## T-16-5: Upward discovery reaches filesystem root

**Traces to:** ARCH-16-1, REQ-16-1

- **Given** a deeply nested CWD with no ancestor targ files
- **When** `Discover()` is called
- **Then** the upward walk terminates at the filesystem root without error, returning only downward results
- **Property:** The walk visits every directory on the path from `CWD/..` to `/` and terminates

## T-16-6: No duplicate packages from overlapping walks

**Traces to:** ARCH-16-1, DES-16-1

- **Given** a directory structure where the CWD itself contains targ-tagged files (found by both downward and upward logic)
- **When** `Discover()` is called
- **Then** each package directory appears exactly once in the results
- **Property:** The set of result directories contains no duplicates

## T-16-7: Downward discovery unchanged

**Traces to:** ARCH-16-5, REQ-16-7

- **Given** a directory tree with targ-tagged files only below the CWD
- **When** `Discover()` is called
- **Then** results are identical to the pre-UC-16 behavior
- **Property:** For any tree with no ancestor targ files, `Discover()` returns the same packages as the original implementation

## T-16-8: Unmoduled ancestor gets own module group

**Traces to:** ARCH-16-3, REQ-16-4, DES-16-2

- **Given** an ancestor directory with targ-tagged files and no `go.mod` anywhere above it
- **When** `groupByModule()` processes the discovered packages
- **Then** the ancestor directory becomes its own module group with `ModulePath = "targ.local"` and `ModuleRoot` set to the ancestor directory (not `startDir`)
- **Property:** Each distinct unmoduled ancestor directory produces a distinct module group

## T-16-9: Moduled ancestor groups with its module

**Traces to:** ARCH-16-3, DES-16-2

- **Given** an ancestor directory with targ-tagged files under an existing `go.mod`
- **When** `groupByModule()` processes the discovered packages
- **Then** the ancestor package groups with the module that contains it, not with `startDir`
- **Property:** `FindModuleForPath()` determines the grouping, same as for downward packages

## T-16-10: Cache key stable across CWDs

**Traces to:** ARCH-16-4, DES-16-4

- **Given** an ancestor directory with targ-tagged files discovered from two different descendant CWDs
- **When** `setupBinaryPath()` is called for each CWD's build
- **Then** the binary path for the ancestor module group is identical in both cases
- **Property:** The cache path is a pure function of `importRoot` (ancestor dir) and bootstrap content, independent of CWD

## T-16-11: Multi-module build aggregates ancestor commands

**Traces to:** ARCH-16-3, ARCH-16-5, DES-16-3

- **Given** targ-tagged files in both CWD (with its own module) and an unmoduled ancestor directory
- **When** the runner executes
- **Then** both module groups are built and their targets are aggregated via multi-module dispatch
- **Property:** The total set of registered targets equals the union of targets from all module groups

## T-16-12: Conflict detection works across tree levels

**Traces to:** ARCH-16-5, REQ-16-5

- **Given** a target name registered in both a CWD package and an ancestor package
- **When** the runner aggregates targets
- **Then** a `ConflictError` is produced with source locations from both packages
- **Property:** Conflict detection is symmetric — the error is the same regardless of which package is processed first

## T-16-13: Help output includes ancestor targets

**Traces to:** ARCH-16-5, REQ-16-6

- **Given** targ-tagged files in ancestor directories that register targets
- **When** the user runs `targ` or `targ --help`
- **Then** ancestor targets appear in the help output with their source package attribution
- **Property:** Every registered target (from any tree level) appears in help output exactly once

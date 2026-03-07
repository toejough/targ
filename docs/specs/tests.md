# Test List: UC-16 Directory Tree Traversal

Traces to: ARCH-1, ARCH-2, ARCH-3, ARCH-4, ARCH-5

## T-1: Upward discovery finds ancestor targ files (ARCH-1)

**Given** a directory tree with targ-tagged `.go` files in an ancestor directory
**When** `Discover()` is called from a descendant directory
**Then** the result includes `PackageInfo` entries from the ancestor directory

**Property:** For any directory tree depth N (1..10), targ files at level K (where K < N from root) are discovered when starting from level N.

## T-2: Upward discovery walks dev/ subdirectories (ARCH-1, ARCH-2)

**Given** a directory tree where an ancestor has a `dev/` subdirectory containing targ-tagged files
**When** `Discover()` is called from a descendant directory
**Then** the result includes `PackageInfo` entries from the ancestor's `dev/` subtree

**Property:** For any ancestor with a `dev/` directory containing targ files at any nesting depth, all files are discovered.

## T-3: Upward discovery excludes sibling directories (ARCH-1)

**Given** a directory tree with targ files in a sibling directory of an ancestor (not `dev/`)
**When** `Discover()` is called from a descendant
**Then** the result does NOT include packages from the sibling directory

**Property:** For any directory tree, only the linear ancestor path and each ancestor's `dev/` subtree are included. All other sibling directories at every level are excluded.

## T-4: Single-directory discovery finds only direct files (ARCH-2)

**Given** a directory containing targ-tagged `.go` files and subdirectories also containing targ-tagged files
**When** `findTaggedFilesInDir()` is called on that directory
**Then** only files directly in that directory are found, not files in subdirectories

**Property:** The returned files all have the same parent directory as the input directory.

## T-5: Upward discovery reaches filesystem root (ARCH-1)

**Given** a deep directory tree with targ files only at the root-most ancestor
**When** `Discover()` is called from the deepest directory
**Then** the root-most ancestor's targ files are discovered

**Property:** Discovery walks all the way up, limited only by the filesystem boundary (or test fixture boundary).

## T-6: No duplicate packages from overlapping walks (ARCH-1)

**Given** a directory tree where CWD is inside an ancestor's `dev/` subtree (e.g., CWD = `~/dev/tools/`)
**When** `Discover()` runs both downward and upward walks
**Then** packages are not duplicated — each directory appears at most once

**Property:** The union of downward and upward results has no duplicate directory entries.

## T-7: Downward discovery unchanged (ARCH-1, REQ-7)

**Given** a directory tree with targ files only in CWD and its subdirectories (no ancestor files)
**When** `Discover()` is called
**Then** the result is identical to the existing behavior (same packages, same order)

**Property:** When no ancestor targ files exist, `Discover()` returns the same result as before the change.

## T-8: Unmoduled ancestor gets own module group (ARCH-3)

**Given** ancestor targ files with no `go.mod` in their directory or above
**When** `groupByModule()` processes the discovered packages
**Then** the ancestor directory becomes its own module group with `modulePath = "targ.local"` and `moduleRoot` = the ancestor directory (not startDir)

**Property:** Each distinct ancestor directory without a go.mod produces a distinct module group.

## T-9: Moduled ancestor groups with its module (ARCH-3)

**Given** ancestor targ files where `FindModuleForPath()` finds a `go.mod` above them
**When** `groupByModule()` processes the discovered packages
**Then** the ancestor packages are grouped with that module, same as any other package under that module root

**Property:** Ancestor packages with a resolvable go.mod are grouped identically to descendant packages with the same go.mod.

## T-10: Cache key stable across CWDs (ARCH-4)

**Given** two different CWDs that both discover the same ancestor targ files
**When** the build unit for those ancestor targets is computed
**Then** the cache key (derived from `importRoot` = ancestor directory) is identical

**Property:** For any two CWDs sharing a common ancestor with targ files, the ancestor build unit's cache key is the same.

## T-11: Multi-module build aggregates ancestor commands (ARCH-3, ARCH-5)

**Given** targ files in CWD (under a go.mod) and ancestor targ files (different or no go.mod)
**When** the multi-module build path executes
**Then** commands from both the CWD module and ancestor module(s) are available to the user

**Property:** The aggregated command registry contains targets from all module groups.

## T-12: Conflict detection works across tree levels (ARCH-5)

**Given** a target named "build" in CWD's package and a target named "build" in an ancestor package
**When** registry resolution runs
**Then** a `ConflictError` is returned listing both source packages

**Property:** Conflict detection treats ancestor and descendant targets identically — same-name from different packages always conflicts.

## T-13: Help output includes ancestor targets (ARCH-5)

**Given** targets discovered from both CWD subtree and ancestor directories
**When** help is rendered
**Then** ancestor targets appear with source attribution distinguishing them from local targets

**Property:** Every discovered target (ancestor or descendant) appears in help output with a non-empty source attribution.

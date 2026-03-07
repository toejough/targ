# Requirements & Design: UC-16 Directory Tree Traversal

Traces to: UC-16

## Requirements

### REQ-1: Upward directory discovery

Targ must discover targ-tagged Go files in each ancestor directory of the CWD, walking from CWD parent to filesystem root.

**Acceptance criteria:**
- For each ancestor directory, check for `.go` files with `//go:build targ` tag in that directory.
- Discovery is automatic — no configuration or opt-in required.
- The linear path is: `CWD/..`, `CWD/../..`, etc. up to `/`.

### REQ-2: Ancestor dev/ subdirectory discovery

At each ancestor directory, targ must also recursively discover targ-tagged Go files in that ancestor's `dev/` subdirectory, if it exists.

**Acceptance criteria:**
- At each ancestor, if `<ancestor>/dev/` exists and is a directory, recursively walk it for targ-tagged files using the same logic as existing downward discovery.
- Only `dev/` is checked — no other sibling directories.

### REQ-3: No sibling directory discovery

Upward traversal must not discover targets in sibling directories of ancestors (other than `dev/`).

**Acceptance criteria:**
- If CWD is `~/repos/personal/`, targets in `~/repos/work/` are NOT discovered.
- Only the linear ancestor path and each ancestor's `dev/` subtree are searched.

### REQ-4: Ancestor targets compile without local go.mod

Ancestor target files that have no `go.mod` in their directory or any ancestor must still compile and execute.

**Acceptance criteria:**
- Each unmoduled ancestor directory becomes its own isolated build unit.
- The isolated build path (synthetic go.mod in cache dir) handles compilation.
- The synthetic go.mod includes a dependency on `github.com/toejough/targ` with appropriate replace directive.
- Files from directories above `startDir` are handled correctly (the current `createIsolatedBuildDir` limitation is resolved).

### REQ-5: Conflict detection across tree levels

Name conflicts between targets at different tree levels produce errors, same as today.

**Acceptance criteria:**
- `ConflictError` with source locations from both packages.
- Error message suggests `targ.DeregisterFrom()` to resolve.
- No implicit precedence based on tree level.

### REQ-6: Help output includes ancestor targets

Help output displays all discovered targets including those from ancestor directories.

**Acceptance criteria:**
- `targ` and `targ --help` show ancestor targets.
- Existing source attribution (source package path) distinguishes origin.
- No new grouping mechanism required.

### REQ-7: Downward discovery unchanged

Existing downward recursive discovery from CWD must not change behavior.

**Acceptance criteria:**
- The full subtree below CWD is walked exactly as today.
- No performance regression for projects that don't have ancestor targets.
- `source = "standard"` — preserving existing behavior is a correctness constraint, not derived from UC-16.

## Design

### DES-1: Discovery flow

**Horizontal UX:** The user experience is unchanged — run `targ` or `targ <name>` from any directory. No new flags, no configuration files, no opt-in. The only visible difference is that more targets may appear (from ancestors).

**Vertical behavior:**

1. Start at CWD.
2. Run existing downward discovery from CWD (unchanged).
3. Walk parent directories from `CWD/..` up to `/`:
   a. At each ancestor, discover targ-tagged files in the ancestor directory itself (non-recursive, just that directory).
   b. If `<ancestor>/dev/` exists, run full recursive discovery on it (same logic as downward walk).
4. Combine all discovered packages into one list.
5. Proceed with existing `groupByModule()` → build path selection.

### DES-2: Module grouping for ancestor targets

Ancestor targets feed into the existing `groupByModule()` pipeline:

- **Ancestor with own go.mod:** Becomes its own module group → builds as a separate binary (existing multi-module path).
- **Ancestor without go.mod but under some go.mod:** `FindModuleForPath()` walks up and finds it → groups with that module.
- **Ancestor with no go.mod anywhere above it:** Becomes its own isolated build unit with synthetic go.mod. Each such directory gets a separate isolated build (not grouped with other unmoduled directories).

### DES-3: Isolated build for above-startDir files

The current `createIsolatedBuildDir()` assumes all target files are under `startDir`. For ancestor targets, files are above `startDir`.

**Resolution:** When grouping unmoduled packages, group by their actual directory rather than by `startDir`. Each distinct ancestor directory with unmoduled targets gets its own isolated build:
- Its own temp directory
- Its own synthetic go.mod
- Its own binary (aggregated via multi-module dispatch)

This extends the existing multi-module path — ancestor directories without go.mod are treated as additional module groups with `modulePath = "targ.local"` but distinct `moduleRoot` values.

### DES-4: Cache key stability

Each ancestor build unit has a stable cache key based on:
- The ancestor directory path (not CWD)
- The content of targ-tagged files in that directory
- The synthetic go.mod content

This ensures the binary cache works correctly: running `targ` from different subdirectories that discover the same ancestor targets reuses the same cached binary.

### DES-5: Performance considerations

Upward discovery adds filesystem checks at each ancestor level. For a typical path depth of 5-10 directories:
- Each ancestor: one `ReadDir` (or stat of individual `.go` files) + one `Stat` of `dev/` subdirectory
- Recursive `dev/` walk only if the directory exists

This is negligible compared to the Go compilation step. No performance mitigation needed.

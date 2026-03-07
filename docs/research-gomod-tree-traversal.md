# Research: go.mod Handling for Tree-Traversal Discovery

Date: 2026-03-06

## Problem Statement

When targ discovers target files across multiple directories (especially parent/ancestor directories), how should it handle `go.mod`? The desired behavior: a user defines a `claude` command in `~/dev/targs.go`, then runs `targ claude` from `~/repos/myproject/` (which has its own `go.mod` or none at all). The target file in `~` needs to be compiled, but the CWD may not have a `go.mod`, or may have a different one.

---

## 1. Current go.mod Handling (as of main branch)

### 1.1 Overall Flow

The main entry point is `runner.Run()` -> `targRunner.run()` (lines 855-1852 of `internal/runner/runner.go`).

The flow is:
1. **Discover packages**: `discoverPackages()` uses `discover.Discover()` which walks DOWN from `startDir`, finding `.go` files with `//go:build targ` tags
2. **Group by module**: `groupByModule()` calls `FindModuleForPath()` on each package's first file, walking UP the directory tree to find the nearest `go.mod`
3. **Branch based on module count**:
   - **Multi-module** (>1 distinct `go.mod`): `handleMultiModule()` builds a separate binary per module
   - **Single module with go.mod**: `handleSingleModule()` builds one binary
   - **No module found**: `handleIsolatedModule()` creates a temp directory with synthetic `go.mod`

### 1.2 FindModuleForPath (lines 499-539)

Walks UP from a file path to the filesystem root looking for `go.mod`:

```go
func FindModuleForPath(path string) (string, string, bool, error) {
    dir := path
    if info, err := os.Stat(path); err == nil && !info.IsDir() {
        dir = filepath.Dir(path)
    }
    for {
        modPath := filepath.Join(dir, "go.mod")
        data, err := os.ReadFile(modPath)
        if err == nil {
            modulePath := parseModulePath(string(data))
            // returns (moduleRoot, modulePath, true, nil)
            return dir, modulePath, true, nil
        }
        parent := filepath.Dir(dir)
        if parent == dir { break }
        dir = parent
    }
    return "", "", false, nil
}
```

Key behavior: It finds the **nearest** `go.mod` above the target file. If none exists anywhere up to `/`, returns `found=false`.

### 1.3 groupByModule (lines 3308-3353)

Groups discovered packages by their module root. Packages without a module are grouped under `startDir` with module path `"targ.local"`:

```go
func groupByModule(infos []discover.PackageInfo, startDir string) ([]moduleTargets, error) {
    for _, info := range infos {
        modRoot, modPath, found, err := FindModuleForPath(info.Files[0].Path)
        if !found {
            modRoot = startDir
            modPath = targLocalModule  // "targ.local"
        }
        // group by modRoot
    }
}
```

### 1.4 Single Module Build (handleSingleModule, lines 1588-1635)

When all targets belong to one module with a `go.mod`:
1. Finds the module root via `FindModuleForPath`
2. Generates bootstrap `main.go` using a template that blank-imports all discovered packages
3. Computes a cache key from: module path, module root, bootstrap code, tagged files, go.mod/go.sum contents
4. Writes bootstrap to a temp file inside the module root
5. Runs `go build -tags targ -o <binaryPath> <bootstrapFile>` with `Dir` set to module root
6. Executes the built binary

The bootstrap template (lines 912-928):
```go
package main
import (
    "github.com/toejough/targ"
    _ "<discovered-package-import-path>"
)
func main() {
    targ.EnableCleanup()
    targ.ExecuteRegisteredWithOptions(targ.RunOptions{...})
}
```

Import paths are computed relative to the module root: `modulePath + "/" + filepath.ToSlash(relPath)`.

### 1.5 Isolated Build (handleIsolatedModule, lines 1493-1540)

When NO `go.mod` is found for targets:
1. `createIsolatedBuildDir()` (lines 2630-2696):
   - Creates a temp directory (`os.MkdirTemp("", "targ-build-")`)
   - Copies targ files with build tags stripped (so they compile without `-tags targ`)
   - Uses `NamespacePaths()` to collapse directory structure
   - Writes a **synthetic go.mod** via `writeIsolatedGoMod()`
2. `writeIsolatedGoMod()` (lines 4034-4076):
   ```
   module targ.build.local
   go 1.21
   require github.com/toejough/targ <version>
   replace github.com/toejough/targ => <local-cache-dir>
   ```
3. Runs `go build -tags targ -mod=mod ...` with `Dir` set to the temp dir
4. The `-mod=mod` flag is critical: allows Go to modify go.mod/go.sum during build

### 1.6 Fallback Module (EnsureFallbackModuleRoot, lines 423-451)

Used in multi-module mode when a package has no `go.mod` (`modulePath == "targ.local"`):
1. Creates a persistent cache directory at `<projectCacheDir>/mod/<hash>`
2. Symlinks all entries from `startDir` into the cache dir (EXCEPT `.git`, `go.mod`, `go.sum`)
3. Writes its own `go.mod` with the `targ.local` module path
4. Creates an empty `go.sum`

The symlink approach (via `linkModuleRoot`, lines 3457-3473) means Go sees the source files as part of the fallback module, but the `go.mod` is independent from any project `go.mod`.

### 1.7 resolveTargDependency (lines 3723-3745)

Determines how to depend on the targ library itself:
1. Reads own build info via `debug.ReadBuildInfo()`
2. Tries to find targ in the Go module cache (`$GOMODCACHE/github.com/toejough/targ@<version>`)
3. Falls back to finding targ's source root via `runtime.Caller`
4. Returns a `TargDependency` with `ModulePath`, `Version`, and `ReplaceDir`

This is used in `require` and `replace` directives in generated `go.mod` files.

---

## 2. Git History: Problems Encountered and Solutions Tried

### 2.1 Timeline of Changes

| Commit | Summary | Key Change |
|--------|---------|------------|
| `f3a2584` | **feat: allow build tool without go.mod** | Initial fallback: creates `.targ/cache/mod/` with symlinks and synthetic `go.mod`. Uses `"targ.local"` module path. |
| `e650867` | **fix: build tool fallback module** | Fixed bootstrap to write into fallback workspace and set `buildCmd.Dir` correctly |
| `b07f370` | **fix: align module path and build tag** | Fixed import path to `github.com/toejough/targ` and restored `"targ"` build tag |
| `59589ff` | **fix: restore fallback module bootstrap location** | Bootstrap must be inside `buildRoot` (not `importRoot`) so Go can find the `go.mod` with replace directive |
| `17b444a` | **fix: only use go.mod in same directory** | REMOVED walk-up behavior. Only checked for `go.mod` in the target file's exact directory. Prevented accidental use of parent module. |
| `2fa3f83` | **fix: prevent overwriting project go.mod** | `linkModuleRoot` was symlinking the project's `go.mod` into cache, then `writeFallbackGoMod` overwrote it through the symlink! Fixed by skipping `go.mod`/`go.sum` in symlinks. |
| `6880d0b` | **fix: clean up stale go.mod symlinks** | Added `cleanupStaleModSymlinks()` to remove leftover `go.mod` symlinks from before the fix |
| `d11f384` | **fix: walk up directory tree to find go.mod** | RE-ADDED walk-up behavior. Changed from `findModuleInDir` (exact directory) back to `findModuleForPath` (walks up). For real modules, builds from original directory. |
| `9a277d4` | **fix: include go.mod/go.sum in cache key** | Cache wasn't invalidated when dependencies changed |
| `2162e24` | **feat: multi-module support** | Groups packages by module, builds separate binary per module, aggregates via `__list` command |
| `a51533d` | **feat: isolated build without go.mod** | Creates temp directory with copied files (build tags stripped) and synthetic `go.mod`. Different from fallback (symlink) approach. |
| `8af9a00` | **fix: conditional isolation** | Only uses isolated build when actually needed (package conflicts), not always |

### 2.2 Detailed Problem History

#### Problem 1: go.mod Overwrite Through Symlinks (2fa3f83)
**What happened**: `linkModuleRoot()` symlinked ALL files from the project directory into the cache directory, including `go.mod`. Then `writeFallbackGoMod()` wrote a new `go.mod` to the cache directory, but since it was a symlink, it **overwrote the original project's go.mod**. This corrupted the user's actual module file.

**Fix**: Skip `go.mod` and `go.sum` when creating symlinks. The cache directory gets its own independent `go.mod`.

**Lesson**: When symlinking a directory tree, always exclude files you plan to generate/overwrite.

#### Problem 2: Bootstrap File Location (59589ff)
**What happened**: The bootstrap `main.go` was written outside the fallback module root. When `go build` ran, it couldn't find the `go.mod` with the `replace` directive, causing build failures.

**Fix**: Write bootstrap inside `buildRoot` (the cache directory with the synthetic `go.mod`), not inside `importRoot` (the original source directory).

**Lesson**: `go build` resolves modules relative to the file being built. The bootstrap file must be within the module tree that has the correct `go.mod`.

#### Problem 3: Walking Up vs. Same-Directory Only (17b444a -> d11f384)
**What happened (17b444a)**: Walking up to find `go.mod` caused problems when targets in a subdirectory accidentally used a parent module that didn't have targ as a dependency. Fix was to ONLY check the same directory.

**What happened (d11f384)**: But same-directory-only broke targets in subdirectories of a module (e.g., `myproject/dev/targets.go` needs `myproject/go.mod`). Fix was to RE-ADD walk-up behavior.

**Current state**: Walk-up is active. The "accidental parent module" problem was deemed less important than "targets in subdirectories must work."

**Lesson**: There's a fundamental tension between "find the right module" and "don't find the wrong module." Walk-up is necessary but can pick up unrelated modules.

#### Problem 4: Stale Symlinks (6880d0b)
**What happened**: After fixing the symlink issue (Problem 1), existing caches still had stale `go.mod` symlinks from before the fix. These continued to corrupt project files.

**Fix**: Added `cleanupStaleModSymlinks()` that explicitly removes any `go.mod`/`go.sum` symlinks in the cache directory.

**Lesson**: Cache corruption fixes need migration logic for existing caches.

#### Problem 5: Cache Invalidation (9a277d4)
**What happened**: Changes to `go.mod` (e.g., updating a dependency version) didn't invalidate the binary cache, so stale binaries were used.

**Fix**: Include `go.mod` and `go.sum` in `collectModuleFiles()` which feeds into the cache key hash.

**Lesson**: The cache key must include ALL inputs that affect the build output.

---

## 3. The Tree-Traversal Scenario

### 3.1 Desired Behavior

User has:
```
~/dev/targs.go          # defines "claude" command, package dev
~/repos/myproject/      # has its own go.mod (github.com/user/myproject)
~/repos/myproject/src/  # user's CWD
```

Running `targ claude` from `~/repos/myproject/src/` should:
1. Discover `~/dev/targs.go` (walking UP from CWD)
2. Compile it successfully
3. Execute the `claude` command

### 3.2 Current Gaps

1. **Discovery only walks DOWN**: `discover.Discover()` uses `findTaggedDirs()` which starts at `startDir` and walks into subdirectories. It never walks UP to parent/ancestor directories.

2. **Module mismatch**: Even if discovery walked up, `~/dev/targs.go` has no `go.mod` (or has a different one than `~/repos/myproject/go.mod`). The target file from `~` and any target files in `~/repos/myproject/` would belong to different modules (or one would have no module).

3. **Import path computation**: `bootstrapBuilder.computeImportPath()` computes import paths as `modulePath + "/" + relPath`. If the target file is ABOVE the module root, `filepath.Rel()` returns a `../` path, which is not a valid Go import path.

### 3.3 What Happens Today

If you add upward discovery, `groupByModule()` would:
- Find `~/repos/myproject/go.mod` for local targets
- Find NO `go.mod` for `~/dev/targs.go` (or a different one)
- Result: multi-module mode with 2 groups

The multi-module path would then:
- Build `~/repos/myproject/` targets normally
- Try to build `~/dev/targs.go` via the fallback module (isolated build)
- This SHOULD work but hasn't been tested for this scenario

---

## 4. Known Go Module Problems with Auto-Generated go.mod

### 4.1 Module Path Conflicts
If two packages in the same build have overlapping module paths, Go gets confused. The synthetic module names (`targ.build.local`, `targ.local`) avoid conflicts with real modules, but they create their own issues: packages can't import each other across synthetic module boundaries.

### 4.2 Replace Directives
The generated `go.mod` uses `replace` to point to targ's local source or module cache:
```
replace github.com/toejough/targ => /path/to/cache
```
This works for targ itself, but if user's target files import OTHER packages (not just targ), those dependencies won't be in the synthetic `go.mod`. The `-mod=mod` flag helps by allowing `go build` to add missing dependencies, but it can fail if the packages aren't fetchable.

### 4.3 Version Resolution
`resolveTargDependency()` tries to find the exact version of targ from its own build info:
- If installed via `go install`, `debug.ReadBuildInfo()` provides the version
- If built from source, falls back to the source root via `runtime.Caller`
- The `ReplaceDir` approach (pointing to local source) avoids version resolution entirely

Problem: If the user has a different version of targ's source available than what's in the module cache, builds may fail with version mismatches.

### 4.4 go mod tidy Failures
The isolated build uses `-mod=mod` which lets Go modify `go.mod`. But `go mod tidy` is never explicitly run. If the synthetic `go.mod` has inconsistencies (e.g., wrong Go version, missing indirect dependencies), the build may fail with cryptic errors.

### 4.5 Build Cache Invalidation
Go's own build cache (`$GOPATH/pkg/mod/cache`) is separate from targ's binary cache. Changes to the synthetic `go.mod` may not invalidate Go's build cache, leading to stale artifacts. Targ handles this by including `go.mod`/`go.sum` in its own cache key, but Go's internal cache may still cause issues.

### 4.6 Cross-Module Imports
If a target file in `~/dev/targs.go` imports a package from `~/repos/myproject/`, this creates a cross-module dependency. The synthetic `go.mod` for the isolated build has no way to resolve this import because:
- It doesn't know about `~/repos/myproject/go.mod`
- There's no `replace` directive pointing to it
- The package isn't in any module cache

---

## 5. Potential Solutions for Tree-Traversal

### 5.1 Multi-Module Build (Current Approach, Extended)

**How it works**: Already implemented for targets spanning multiple `go.mod` files. Each module group gets its own binary. Commands are aggregated via `__list` JSON output.

**For tree traversal**: Add upward discovery, then let `groupByModule()` naturally separate ancestor targets into their own module groups. Each group builds independently.

**Pros**:
- Already implemented and tested
- Clean separation of concerns
- No cross-module dependency issues (each binary is self-contained)

**Cons**:
- Multiple binaries per invocation (startup overhead)
- Commands from different modules can't share state
- Ancestor targets without `go.mod` would use isolated build (copying files to temp dir every time)

**Viability**: HIGH. This is the path of least resistance.

### 5.2 go.work (Workspace Files)

**How it works**: Go 1.18+ supports `go.work` files that combine multiple modules:
```
go 1.21
use (
    ./
    ../../../dev
)
```

**For tree traversal**: Generate a `go.work` file in a temp directory that references all discovered module roots.

**Pros**:
- Official Go mechanism for multi-module development
- All packages can import each other
- Single build, single binary

**Cons**:
- Requires all modules to be resolvable from the workspace root
- Module paths must not conflict
- `go.work` doesn't support modules without `go.mod` (would still need synthetic modules for unmoduled directories)
- Adds complexity to cache key computation
- Not well-tested with temporary/synthetic module combinations

**Viability**: MEDIUM. Could work but adds complexity for marginal benefit over multi-module build.

### 5.3 Overlay Files (-overlay flag)

**How it works**: `go build -overlay=overlay.json` lets you provide a JSON file that maps real file paths to replacement paths. This can inject files into a build without modifying the filesystem.

**For tree traversal**: Create an overlay that places ancestor target files into the current module's source tree.

**Pros**:
- No temp directories or file copying needed
- Works within the existing module structure
- Clean and fast

**Cons**:
- Overlay files can't add new directories/packages that don't exist in the module
- Package names must not conflict with existing packages in the module
- The overlay approach only works if all target files can be placed within ONE module's tree
- Doesn't solve the "no go.mod" case

**Viability**: LOW. Too many constraints for the general case.

### 5.4 Synthetic Module with Replace Directives

**How it works**: Generate a single synthetic `go.mod` that uses `replace` directives to point to all relevant source directories:

```
module targ.build.local
go 1.21
require github.com/toejough/targ v1.0.0
replace github.com/toejough/targ => /path/to/targ
```

Then copy/symlink all target files into subdirectories of the synthetic module.

**For tree traversal**: All ancestor targets get copied into the synthetic module, regardless of their original module context.

**Pros**:
- Single binary output
- All targets share the same build context
- Already partially implemented (isolated build mode)

**Cons**:
- **This is where targ has historically had the most bugs** (symlink overwrites, bootstrap location, stale caches)
- If target files import non-targ packages, those dependencies must be manually added to the synthetic `go.mod`
- Can't resolve user packages that are part of other modules without additional `replace` directives
- File copying is slow for large codebases

**Viability**: MEDIUM. Works for simple cases (targets that only use targ + stdlib), fragile for complex cases.

### 5.5 go run with Module-Aware Mode

**How it works**: Instead of `go build`, use `go run` with specific module settings:
```
GOFLAGS=-mod=mod go run ./bootstrap.go
```

**For tree traversal**: Generate bootstrap code and run it directly without creating a persistent binary.

**Pros**:
- Simpler than managing build artifacts
- Go handles module resolution automatically

**Cons**:
- No binary caching (must recompile every time)
- Same module issues apply (`go run` still needs a `go.mod`)
- Startup time penalty on every invocation

**Viability**: LOW. Defeats the purpose of targ's binary caching.

### 5.6 Vendoring Approach

**How it works**: Use `go mod vendor` to create a `vendor/` directory with all dependencies, then build with `-mod=vendor`.

**For tree traversal**: Create a vendored synthetic module containing all dependencies.

**Pros**:
- Hermetic builds (no network needed)
- Explicit dependency tracking

**Cons**:
- Vendor directory is large and slow to create
- Must be regenerated when dependencies change
- Same synthetic module issues as 5.4

**Viability**: LOW. Too heavyweight for a build tool.

---

## 6. Recommended Approach

### 6.1 Extend Multi-Module Build (Solution 5.1)

The multi-module build path is already implemented and handles the core challenge: targets from different modules are built into separate binaries and commands are aggregated. The main work needed is:

1. **Add upward discovery**: Modify `discover.Discover()` to also walk UP from `startDir` (not just down). The engram memory (`targ-bidirectional-tree-search.toml`) confirms this is the desired direction.

2. **Handle "no module" ancestors gracefully**: When ancestor targets have no `go.mod`, they'll be grouped under `"targ.local"` and built via the fallback module path. This already works.

3. **Cache the fallback module persistently**: The `EnsureFallbackModuleRoot()` function already creates a persistent cache at `<projectCacheDir>/mod/<hash>`. For ancestor directories, the hash would be based on the ancestor path, ensuring stable caching.

### 6.2 Key Considerations

**Discovery direction**: Walk UP first (to find ancestor targets), then DOWN (to find descendant targets). This ensures ancestor commands are always available.

**Module grouping**: The existing `groupByModule()` already handles mixed-module discovery correctly. Ancestor targets with their own `go.mod` get their own binary. Ancestor targets without `go.mod` use the fallback module.

**Cache stability**: Each distinct module root gets its own binary cache. The cache key includes `go.mod` content, so changes to dependencies invalidate correctly.

**Import path edge case**: For ancestor targets without a `go.mod`, the fallback module approach symlinks the ancestor directory. The `computeImportPath()` function uses `filepath.Rel()` from the module root, which works because the symlinks make the files appear to be within the fallback module root.

### 6.3 What Could Go Wrong

1. **Symlink depth**: If the ancestor directory has deep directory trees, symlinking everything could be slow or hit filesystem limits. Mitigation: only symlink directories that contain targ-tagged files.

2. **Name collisions**: Two target files in different ancestor directories defining the same command name. Mitigation: the multi-module aggregation already handles command collisions (last one wins, or error).

3. **Dependency resolution**: Ancestor targets that import packages not in the targ module or stdlib will fail to build. Mitigation: if the ancestor has its own `go.mod`, those dependencies are already handled. If not, the user needs to create a `go.mod` in the ancestor directory.

4. **Performance**: Building multiple binaries on first run could be slow. Mitigation: binary caching means this only happens once (or when source changes).

---

## 7. Source File Reference

All code analyzed is in `/Users/joe/repos/personal/targ/internal/runner/runner.go` unless otherwise noted:

- `FindModuleForPath`: lines 499-539
- `groupByModule`: lines 3308-3353
- `handleSingleModule`: lines 1588-1635
- `handleIsolatedModule`: lines 1493-1540
- `handleMultiModule`: lines 1542-1575
- `createIsolatedBuildDir`: lines 2630-2696
- `writeIsolatedGoMod`: lines 4034-4076
- `writeFallbackGoMod`: lines 3986-4014
- `EnsureFallbackModuleRoot`: lines 423-451
- `linkModuleRoot`: lines 3457-3473
- `resolveTargDependency`: lines 3723-3745
- `prepareBuildContext`: lines 3512-3534
- `executeBuild`: lines 1363-1390
- `buildBootstrapData`: lines 2128-2143
- `bootstrapBuilder.computeImportPath`: lines 985-992
- `discover.Discover`: `/Users/joe/repos/personal/targ/internal/discover/discover.go` lines 68-96
- `findTaggedDirs`: `/Users/joe/repos/personal/targ/internal/discover/discover.go` lines 316-342

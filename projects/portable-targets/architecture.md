# Technical Architecture: Portable Targets

## 1. Overview

Portable targets extends targ to support importing and composing build targets from remote Go packages. The implementation leverages Go's native import system, treating targets as exported package variables that are selectively registered via `init()` functions.

**Key architectural decisions:**
- Go code is the configuration (no new file formats)
- Source attribution via runtime reflection (zero-config for users)
- Deferred conflict detection (users resolve conflicts in their own `init()`)
- Dependency injection pattern (all state flows through existing registry)

**Architecture philosophy:** Extend the existing registration mechanism with source tracking, minimal API surface growth, and explicit conflict resolution.

## 2. Requirements Traceability

| Requirement | Technical Implication | Addressed By |
|-------------|----------------------|--------------|
| Sync from multiple remote sources | Track source package for each target | `Target.sourcePkg` field + `runtime.Caller` attribution |
| Targets work out of the box | Use Go conventions for defaults, env vars for overrides | Existing targ env var support (no changes) |
| Re-sync pulls improvements without losing local customizations | Go module system handles versioning | `go get -u` updates, `go.mod` pins versions |
| Clear attribution (where each target came from) | Source tracking and display in listing | `Target.sourcePkg` + enhanced `--list` output |
| Config files shareable alongside targets | No sync mechanism needed initially | Out of scope (manual copy-paste acceptable) |
| Opt-in/opt-out flexibility | Users can deregister all and re-register selectively | `DeregisterFrom()` + selective `Register()` |
| Simple override model | Single mechanism for resolution | `DeregisterFrom()` only, no exclude syntax |
| Transparent conflict resolution | Named conflicts with actionable errors | `ConflictError` with source list |

## 3. Technology Stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Language | Go | Required by project, existing codebase |
| Import mechanism | Go modules | Native package management, versioning, no new tooling |
| Source attribution | runtime.Caller + reflect | Zero-config for users, automatic package detection |
| Versioning | go.mod + go.sum | Standard Go practice, `go get -u` for updates |
| Conflict detection | Build-time validation | Deferred until after all `init()` complete |
| Testing | gomega + rapid | Existing targ test infrastructure |

## 4. Architecture

### 4.1 System Context

```
┌─────────────┐
│   User's    │
│  dev/       │
│  targets.go │
└──────┬──────┘
       │ import
       │
       ├──────────────┬─────────────┐
       │              │             │
       ▼              ▼             ▼
┌────────────┐  ┌─────────┐  ┌──────────┐
│github.com/ │  │github.  │  │ Local    │
│toejough/   │  │com/bob/ │  │ targets  │
│go-targets  │  │targets  │  │          │
└──────┬─────┘  └────┬────┘  └────┬─────┘
       │             │             │
       └──────┬──────┴─────────────┘
              │ init() registrations
              ▼
       ┌─────────────┐
       │    targ     │
       │  registry   │
       └─────────────┘
              │ resolveRegistry()
              ▼
       ┌─────────────┐
       │  Conflict   │
       │  detection  │
       │  & merge    │
       └─────────────┘
```

### 4.2 Component Overview

```
┌───────────────────────────────────────┐
│         Public API (targ.go)          │
│  Register, DeregisterFrom, Targ       │
└─────────────┬─────────────────────────┘
              │
              ▼
┌───────────────────────────────────────┐
│      Registration Layer               │
│    (internal/core/execute.go)         │
│  RegisterTarget, ExecuteRegistered    │
└─────────────┬─────────────────────────┘
              │
              ▼
┌───────────────────────────────────────┐
│      Registry Layer (NEW)             │
│    (internal/core/registry.go)        │
│  resolveRegistry, buildTargetMap      │
│  applyDeregistrations                 │
└─────────────┬─────────────────────────┘
              │
              ▼
┌───────────────────────────────────────┐
│      Source Attribution (NEW)         │
│    (internal/core/source.go)          │
│  attributeSource, extractPackagePath  │
└───────────────────────────────────────┘
```

### 4.3 Data Flows

#### Flow 1: Target Registration (init phase)

```
Remote package init()
    │
    ├─> targ.Register(targets...)
    │       │
    │       ├─> runtime.Caller(1) identifies source package
    │       ├─> Set target.sourcePkg for each target
    │       └─> Append to registry queue
    │
Local package init()
    │
    ├─> targ.DeregisterFrom("pkg")
    │       └─> Queue deregistration
    │
    └─> targ.Register(localTargets, remoteTargets.Modified)
            └─> Append to registry queue
```

#### Flow 2: Registry Resolution (build time)

```
ExecuteRegistered()
    │
    ├─> resolveRegistry()
    │       │
    │       ├─> applyDeregistrations(registry, deregistrations)
    │       │       │
    │       │       └─> Filter out targets from deregistered packages
    │       │
    │       └─> buildTargetMap(filteredRegistry)
    │               │
    │               ├─> Check for duplicate names
    │               ├─> Return ConflictError if found
    │               └─> Return name -> Target map
    │
    └─> Execute targets (existing logic)
```

#### Flow 3: Listing with Attribution

```
targ --list
    │
    └─> For each target in resolved registry:
            │
            ├─> Get target.GetName()
            ├─> Get target.sourcePkg
            ├─> Check target.nameOverridden
            └─> Format: "name    description    (source) [renamed]"
```

### 4.4 Dependency Injection

The architecture maintains targ's existing pattern where all state flows through the global registry. No new global state beyond what's necessary for deferred conflict detection.

**Existing flow (unchanged):**
```
User init() -> RegisterTarget() -> registry []any -> ExecuteRegistered() -> execution
```

**Extended flow (new):**
```
User init() -> RegisterTarget() -> registry []any (with sourcePkg)
                                          ↓
            DeregisterFrom() -> deregistrations []string
                                          ↓
                                 resolveRegistry() -> conflicts detected
                                          ↓
                                 ExecuteRegistered() -> execution
```

## 5. Data Models

### 5.1 Domain Entities

```go
// Target represents a build target (existing, extended)
type Target struct {
    // Existing fields
    fn              any
    name            string
    description     string
    deps            []*Target
    depMode         DepMode
    timeout         time.Duration
    cache           []string
    cacheDir        string
    watch           []string
    times           int
    whileFn         func() bool
    retry           bool
    backoffInitial  time.Duration
    backoffMultiply float64
    watchDisabled   bool
    cacheDisabled   bool

    // NEW: Source tracking
    sourcePkg      string // Package path that exported this target
    nameOverridden bool   // True if Name() was called
}

// ConflictError represents duplicate target names from different sources
type ConflictError struct {
    Name    string   // Target name with conflict
    Sources []string // Package paths that registered this name
}

func (e *ConflictError) Error() string {
    return fmt.Sprintf(
        "targ: conflict: %q registered by both:\n  - %s\n  - %s\nUse targ.DeregisterFrom() to resolve.",
        e.Name,
        e.Sources[0],
        e.Sources[1],
    )
}

// DeregistrationError represents attempt to deregister unknown package
type DeregistrationError struct {
    PackagePath string
}

func (e *DeregistrationError) Error() string {
    return fmt.Sprintf(
        "targ: DeregisterFrom(%q): no targets registered from this package",
        e.PackagePath,
    )
}

// registryEntry tracks a target and its source (internal)
type registryEntry struct {
    target    any    // *Target or *TargetGroup
    sourcePkg string // Package that registered this
}

// deregistration tracks a package to remove (internal)
type deregistration struct {
    packagePath string
}
```

### 5.2 Storage Schema

No persistent storage. All state is in-memory during build phase:

```go
// Global state (internal/core/execute.go and registry.go)
var (
    registry        []any            // Existing: queued registrations
    deregistrations []deregistration // NEW: queued deregistrations
)
```

## 6. Service Interfaces

### 6.1 Public API Extensions

```go
// DeregisterFrom removes all targets registered by the named package.
// Must be called from init() before targ's build phase.
//
// Example:
//   func init() {
//       targ.DeregisterFrom("github.com/alice/go-targets")
//       targ.Register(aliceTargets.Deploy) // Re-register just this one
//   }
//
// Returns error if:
// - No targets were registered from that package (catches typos)
// - Called after init() phase (too late to affect registry)
func DeregisterFrom(packagePath string) error
```

### 6.2 Internal Registry Services

```go
// resolveRegistry processes queued registrations and detects conflicts.
// Called at the start of ExecuteRegistered() after all init() complete.
//
// Steps:
// 1. Apply deregistrations (remove targets from specified packages)
// 2. Build target name -> entry map
// 3. Detect conflicts (same name from different packages)
// 4. Return resolved registry or ConflictError
func resolveRegistry() ([]any, error)

// buildTargetMap creates a map of target names to entries.
// Detects conflicts when multiple sources register the same name.
//
// Returns:
// - map[string]*Target on success
// - ConflictError if duplicate names found
func buildTargetMap(entries []registryEntry) (map[string]*Target, error)

// applyDeregistrations filters out targets from specified packages.
//
// For each deregistration:
// 1. Find all entries with matching sourcePkg
// 2. Remove them from the list
// 3. Track if any were found (for error reporting)
//
// Returns:
// - Filtered entries
// - DeregistrationError if a package had no matches
func applyDeregistrations(
    entries []registryEntry,
    dereg []deregistration,
) ([]registryEntry, error)
```

### 6.3 Source Attribution Services

```go
// attributeSource sets the source package for a target based on caller.
// Uses runtime.Caller to identify the calling package.
//
// Parameters:
// - target: Target to attribute
// - depth: Stack depth to caller (1 = direct caller, 2 = caller's caller)
//
// Only sets sourcePkg if it's currently empty (doesn't override explicit source).
func attributeSource(target *Target, depth int) error

// packagePathFromPC extracts package path from program counter.
// Uses runtime.FuncForPC to get function info, then parses package path.
//
// Example:
//   pc from runtime.Caller(1)
//   -> runtime.FuncForPC(pc).Name() = "github.com/toejough/go-targets.init"
//   -> extractPackagePath(...) = "github.com/toejough/go-targets"
func packagePathFromPC(pc uintptr) (string, error)

// extractPackagePath parses package path from fully qualified function name.
//
// Handles:
// - Standard packages: "github.com/user/repo/pkg.Func" -> "github.com/user/repo/pkg"
// - Init functions: "github.com/user/repo.init" -> "github.com/user/repo"
// - Nested packages: "github.com/user/repo/internal/pkg.Func" -> "github.com/user/repo/internal/pkg"
// - Local packages: "dev/targets.init" -> "dev/targets" (for local source)
func extractPackagePath(funcName string) string
```

### 6.4 Target Builder Extensions

```go
// Name sets the CLI name for this target.
// By default, the function name is used (converted to kebab-case).
//
// MODIFIED: Now sets nameOverridden = true for attribution display.
func (t *Target) Name(s string) *Target {
    t.name = s
    t.nameOverridden = true // NEW
    return t
}

// GetSource returns the package path that registered this target.
// Empty string means local target (registered in user's project).
func (t *Target) GetSource() string {
    return t.sourcePkg
}

// IsRenamed returns true if Name() was called to override the default name.
func (t *Target) IsRenamed() bool {
    return t.nameOverridden
}
```

## 7. File Structure

```
targ/
├── targ.go                          # Public API
│   ├── DeregisterFrom()             # NEW: Export deregistration
│   └── (existing exports)
│
├── internal/
│   ├── core/
│   │   ├── target.go                # Target struct + builders
│   │   │   ├── Target type          # MODIFIED: Add sourcePkg, nameOverridden fields
│   │   │   ├── Name()               # MODIFIED: Set nameOverridden = true
│   │   │   ├── GetSource()          # NEW: Return sourcePkg
│   │   │   └── IsRenamed()          # NEW: Return nameOverridden
│   │   │
│   │   ├── execute.go               # Registration + execution
│   │   │   ├── RegisterTarget()     # MODIFIED: Add source attribution
│   │   │   ├── ExecuteRegistered()  # MODIFIED: Call resolveRegistry()
│   │   │   └── (existing functions)
│   │   │
│   │   ├── registry.go              # NEW: Registry resolution
│   │   │   ├── resolveRegistry()
│   │   │   ├── buildTargetMap()
│   │   │   ├── applyDeregistrations()
│   │   │   ├── registryEntry type
│   │   │   ├── deregistration type
│   │   │   ├── ConflictError type
│   │   │   └── DeregistrationError type
│   │   │
│   │   ├── source.go                # NEW: Source attribution
│   │   │   ├── attributeSource()
│   │   │   ├── packagePathFromPC()
│   │   │   └── extractPackagePath()
│   │   │
│   │   ├── command.go               # Command execution (existing)
│   │   │   └── formatTargetList()   # MODIFIED: Show source attribution
│   │   │
│   │   └── (other existing files)
│   │
│   └── (other internal packages)
│
├── test/
│   ├── registration_properties_test.go  # NEW: Property tests
│   │   ├── TestSourceAttribution
│   │   ├── TestDeregistration
│   │   ├── TestConflictDetection
│   │   └── TestMultipleSourcesNoConflict
│   │
│   └── (existing test files)
│
└── projects/portable-targets/
    ├── requirements.md
    ├── design.md
    └── architecture.md              # THIS FILE
```

## 8. Technology Decisions

### 8.1 Decisions Made

| Decision | Choice | Alternatives Considered | Rationale |
|----------|--------|------------------------|-----------|
| Import mechanism | Go modules (native imports) | Custom sync command, git submodules | Zero new tooling, native Go workflow |
| Source attribution | runtime.Caller auto-detection | Explicit Source() builder, build tags | Zero-config for users, automatic |
| Conflict detection | Build-time (deferred) | Registration-time (immediate) | Allows user's init() to resolve before error |
| Deregistration API | DeregisterFrom(packagePath) | Exclude list in config file, per-target opt-out | Single mechanism, explicit, no config files |
| Versioning | go.mod (Go modules) | Targ-specific version manifest | Standard Go practice, no reinvention |
| Name override tracking | nameOverridden bool field | Compare name vs derived name at display time | Explicit flag is clearer, faster |
| Target extension | Add sourcePkg field | Parallel map[*Target]string | Direct ownership, simpler |

### 8.2 Patterns Used

| Pattern | Where | Why |
|---------|-------|-----|
| Builder | Target methods (Name, Deps, etc.) | Fluent API for target configuration |
| Registry | Global registry with deferred resolution | Single source of truth, init-time composition |
| Factory | Targ() function | Consistent target creation |
| Runtime reflection | Source attribution, function name extraction | Zero-config, automatic metadata |
| Deferred validation | Conflict detection after all init() | Users can resolve conflicts in their init() |
| Error types | ConflictError, DeregistrationError | Structured errors with actionable messages |

## 9. Error Handling

### 9.1 Error Strategy by Layer

**Public API (DeregisterFrom):**
- Validates package path is non-empty
- Queues deregistration (validation deferred to build time)
- Returns nil (errors happen at resolution time)

**Registry Resolution:**
- `applyDeregistrations`: Returns `DeregistrationError` if package not found
- `buildTargetMap`: Returns `ConflictError` if duplicate names
- `resolveRegistry`: Propagates errors, halts execution

**Source Attribution:**
- `runtime.Caller` failure: Log warning, use "local" as fallback
- `extractPackagePath` parse failure: Return "unknown" (non-fatal)

**Execution:**
- Registry resolution errors: Print to stderr, exit code 1
- All existing target execution errors: Unchanged

### 9.2 Error Messages

All errors are actionable and name the fix:

```
# Conflict
targ: conflict: "lint" registered by both:
  - github.com/alice/go-targets
  - github.com/bob/go-targets
Use targ.DeregisterFrom() to resolve.

# Deregistration not found
targ: DeregisterFrom("github.com/typo/wrong"): no targets registered from this package

# Deregistration called too late (if we add phase checking)
targ: DeregisterFrom() must be called during init(), not after targ has started
```

## 10. Testing Strategy

### Test Tooling Requirements

- **Human-readable matchers**: gomega (existing) for assertion clarity
- **Randomized property exploration**: rapid (existing) for edge case discovery

### Testing by Layer

**Source Attribution (internal/core/source_test.go):**
```go
// Property test: extracted package path is prefix of full function name
func TestExtractPackagePathIsPrefix(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        pkg := rapid.StringMatching(`[a-z]+(/[a-z]+)+`).Draw(t, "pkg")
        funcName := pkg + "." + rapid.StringN(1, 20, 'a', 'z', 0).Draw(t, "func")

        extracted := extractPackagePath(funcName)
        Expect(strings.HasPrefix(funcName, extracted)).To(BeTrue())
    })
}

// Example-based: known cases
func TestExtractPackagePathExamples(t *testing.T) {
    cases := map[string]string{
        "github.com/toejough/go-targets.init": "github.com/toejough/go-targets",
        "dev/targets.init": "dev/targets",
        "github.com/user/repo/internal/pkg.Func": "github.com/user/repo/internal/pkg",
    }

    for input, expected := range cases {
        Expect(extractPackagePath(input)).To(Equal(expected))
    }
}
```

**Registry Resolution (internal/core/registry_test.go):**
```go
// Property test: deregistering removes all targets from that package
func TestDeregisterRemovesAllFromPackage(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        pkgPath := rapid.String().Draw(t, "pkg")
        numTargets := rapid.IntRange(1, 10).Draw(t, "num")

        entries := make([]registryEntry, numTargets)
        for i := range entries {
            entries[i] = registryEntry{
                target: &Target{name: fmt.Sprintf("target-%d", i)},
                sourcePkg: pkgPath,
            }
        }

        filtered, err := applyDeregistrations(entries, []deregistration{{pkgPath}})
        Expect(err).To(BeNil())
        Expect(filtered).To(BeEmpty())
    })
}

// Example-based: conflict detection
func TestConflictDetection(t *testing.T) {
    entries := []registryEntry{
        {target: &Target{name: "lint"}, sourcePkg: "github.com/alice/targets"},
        {target: &Target{name: "lint"}, sourcePkg: "github.com/bob/targets"},
    }

    _, err := buildTargetMap(entries)
    Expect(err).To(BeAssignableToTypeOf(&ConflictError{}))

    conflictErr := err.(*ConflictError)
    Expect(conflictErr.Name).To(Equal("lint"))
    Expect(conflictErr.Sources).To(ContainElements(
        "github.com/alice/targets",
        "github.com/bob/targets",
    ))
}
```

**Integration (test/registration_properties_test.go):**
```go
// Property test: multiple sources without conflicts works
func TestMultipleSourcesNoConflict(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        numPkgs := rapid.IntRange(1, 5).Draw(t, "numPkgs")

        // Generate targets with unique names across all packages
        allNames := make(map[string]bool)
        for i := 0; i < numPkgs; i++ {
            pkgPath := fmt.Sprintf("pkg-%d", i)
            numTargets := rapid.IntRange(1, 3).Draw(t, "targets")

            for j := 0; j < numTargets; j++ {
                name := fmt.Sprintf("target-%d-%d", i, j)
                allNames[name] = true

                // Simulate registration from this package
                // (test helper that calls RegisterTarget via reflection)
                registerFromPackage(pkgPath, &Target{name: name})
            }
        }

        _, err := resolveRegistry()
        Expect(err).To(BeNil())
    })
}
```

**End-to-end (via Execute):**
```go
// Full flow: import, deregister, re-register, execute
func TestPortableTargetsFlow(t *testing.T) {
    // Setup: Create mock remote targets
    remoteTest := targ.Targ(func() error { return nil }).
        Name("test").
        source("github.com/remote/targets")

    // Local init simulation
    targ.DeregisterFrom("github.com/remote/targets")
    targ.Register(remoteTest.Name("unit-test"))

    // Execute
    result, err := targ.Execute([]string{"cmd", "--list"})
    Expect(err).To(BeNil())
    Expect(result.Output).To(ContainSubstring("unit-test"))
    Expect(result.Output).To(ContainSubstring("github.com/remote/targets"))
    Expect(result.Output).To(ContainSubstring("renamed"))
}
```

### Test Coverage Targets

- Source attribution: 100% (pure functions, deterministic)
- Registry resolution: 100% (core business logic)
- Integration flows: 90%+ (cover key user journeys)
- Edge cases via property tests:
  - Empty deregistrations
  - Deregister non-existent package
  - Multiple conflicts in one registry
  - Rename after deregister

## 11. Implementation Plan

### Phase 1: Source Attribution Foundation
**Files:** `internal/core/source.go`, `internal/core/source_test.go`

1. Implement `extractPackagePath(funcName string) string`
2. Implement `packagePathFromPC(pc uintptr) (string, error)`
3. Implement `attributeSource(target *Target, depth int) error`
4. Write property + example tests

**Acceptance:** All source attribution tests pass

### Phase 2: Target Extensions
**Files:** `internal/core/target.go`

1. Add `sourcePkg string` field to Target
2. Add `nameOverridden bool` field to Target
3. Modify `Name()` to set `nameOverridden = true`
4. Add `GetSource()` method
5. Add `IsRenamed()` method

**Acceptance:** Target builder tests pass, no regressions

### Phase 3: Registry Resolution
**Files:** `internal/core/registry.go`, `internal/core/registry_test.go`

1. Define `registryEntry`, `deregistration`, error types
2. Implement `buildTargetMap()`
3. Implement `applyDeregistrations()`
4. Implement `resolveRegistry()`
5. Write property + example tests for each function

**Acceptance:** All registry resolution tests pass

### Phase 4: Registration Integration
**Files:** `internal/core/execute.go`

1. Add `deregistrations []deregistration` global var
2. Modify `RegisterTarget()` to call `attributeSource()`
3. Implement `DeregisterFrom()` to queue deregistrations
4. Modify `ExecuteRegistered()` to call `resolveRegistry()` early
5. Handle resolution errors (print and exit)

**Acceptance:** Integration tests pass, error messages correct

### Phase 5: Public API Export
**Files:** `targ.go`

1. Export `DeregisterFrom()` wrapper
2. Update package doc comments

**Acceptance:** API is usable from external packages

### Phase 6: List Command Enhancement
**Files:** `internal/core/command.go`

1. Modify `formatTargetList()` to show source attribution
2. Add "(renamed)" annotation for overridden names
3. Update help text format

**Acceptance:** `targ --list` shows source info correctly

### Phase 7: Documentation + Examples
**Files:** `README.md`, `examples/portable/`

1. Create example remote package `examples/portable/remote/`
2. Create example local package `examples/portable/local/`
3. Update README with portable targets section
4. Add CLI examples to help text

**Acceptance:** Examples run successfully, docs clear

## 12. Open Questions

### Resolved During Design

1. **What is the right abstraction for "version" of a synced target?**
   - **Decision:** Git commit hash via go.mod. Go modules handle this.

2. **Should config files be synced as whole files or composed from fragments?**
   - **Decision:** Out of scope initially. Manual copy-paste is acceptable.

3. **Where does the sync manifest live?**
   - **Decision:** go.mod + go.sum. No additional manifest needed.

4. **How does this interact with targ's existing `targ sync` command?**
   - **Decision:** `targ --sync` adds blank import. This is sufficient.

5. **Should there be a "starter" repo that targ maintains?**
   - **Decision:** Yes, but not blocking. `github.com/toejough/go-targets` as reference.

6. **How do env var defaults work?**
   - **Decision:** Existing targ env var support via struct tags. No changes needed.

7. **Is "partial sync" per-target or per-file?**
   - **Decision:** Per-target. User deregisters package, re-registers individual targets.

### Remaining Questions

1. **Should DeregisterFrom validate package path format?**
   - Current design: Queue anything, validate at resolution (error if not found)
   - Alternative: Validate format early (catch typos sooner)
   - **Recommendation:** Validate at resolution (simpler, errors are clear anyway)

2. **Should source attribution work for TargetGroup?**
   - Current design: Only targets have sourcePkg
   - User concern: Groups defined in remote packages also need attribution
   - **Recommendation:** Extend to TargetGroup in Phase 2 if needed

3. **Should renamed targets show original name in listing?**
   - Current design: Show "(renamed)" annotation only
   - Alternative: Show "new-name (was: old-name)"
   - **Recommendation:** Start with annotation, add original name if users request it

4. **Should DeregisterFrom be callable multiple times for same package?**
   - Current design: Allowed (idempotent)
   - Alternative: Error on duplicate deregistration
   - **Recommendation:** Allow (simpler, no harm in redundant calls)

5. **How to handle targets with no discoverable source (edge case)?**
   - Example: Target created at runtime, not in any package init
   - Current design: sourcePkg = "" (treated as local)
   - **Recommendation:** Document this as expected behavior

## 13. Migration Path

### For Existing targ Users

**No breaking changes.** All existing code continues to work:
- `targ.Register(targets...)` works as before
- Targets without source attribution appear as "local" in listings
- No required changes to existing targets

### For New Portable Target Authors

**To create a portable target package:**

1. Create Go package with targets:
```go
package targets

import "github.com/toejough/targ"

var Test = targ.Targ(test).Description("Run tests")

func test() error { /* ... */ }

func init() {
    targ.Register(Test, Lint, Coverage)
}
```

2. Publish to GitHub (or any Go-accessible location)

3. Users import with blank import for full sync:
```go
import _ "github.com/yourorg/targets"
```

4. Or named import for selective sync:
```go
import targets "github.com/yourorg/targets"

func init() {
    targ.DeregisterFrom("github.com/yourorg/targets")
    targ.Register(targets.Test)
}
```

**Source attribution happens automatically via runtime.Caller().**

## 14. Performance Considerations

### Build-Time Impact

**Source attribution (runtime.Caller):**
- Called once per RegisterTarget invocation (during init)
- Negligible overhead (<1ms per call)
- Not in hot path (only runs once at startup)

**Registry resolution (conflict detection):**
- O(n) where n = number of registered targets
- Expected n < 100 for most projects
- One-time cost at ExecuteRegistered (before any target runs)

**No runtime performance impact:** All resolution happens before target execution begins.

### Memory Impact

**Additional fields per Target:**
- `sourcePkg string`: ~16 bytes (pointer) + package path length
- `nameOverridden bool`: 1 byte
- Total per target: ~20-50 bytes

**For 50 targets:** ~1-2.5 KB additional memory (negligible)

### Optimization Opportunities

If registry resolution becomes a bottleneck (unlikely):
1. Cache resolved registry (don't re-resolve on each Execute call)
2. Use map for deregistration lookup instead of linear scan
3. Parallel conflict detection if >1000 targets

**Current assessment:** No optimizations needed for foreseeable use cases.

## 15. Security Considerations

### Supply Chain

**Imported targets are code:** Users must trust the packages they import.
- Same trust model as any Go dependency
- `go.sum` provides integrity verification
- Use `go mod verify` to check dependencies

**Recommendations for users:**
1. Review imported target packages before use
2. Pin versions in go.mod (don't use `@latest` in production)
3. Use private repositories for sensitive build logic

### Code Execution

**Targets execute arbitrary Go functions:** No sandboxing.
- This is existing targ behavior (not new risk)
- Portable targets don't change security posture

**Mitigation:** Review imported packages, same as any dependency.

### Deregistration Bypass

**Malicious package could re-register after deregistration:**
```go
func init() {
    targ.Register(malicious)
    // User deregisters here
    targ.Register(malicious) // Sneaky re-registration
}
```

**Mitigation:** Init order is undefined in Go. This is a general Go issue, not specific to targ. Users should review imported packages.

## 16. Monitoring and Observability

### Build-Time Diagnostics

**Source attribution logging (optional debug mode):**
```
TARG_DEBUG=1 targ test
[DEBUG] Registered "test" from github.com/toejough/go-targets
[DEBUG] Registered "lint" from github.com/toejough/go-targets
[DEBUG] Deregistering all from github.com/toejough/go-targets
[DEBUG] Re-registered "test" from github.com/toejough/go-targets (local override)
```

**Conflict detection output:**
```
targ: conflict: "lint" registered by both:
  - github.com/alice/go-targets
  - github.com/bob/go-targets
Use targ.DeregisterFrom() to resolve.
```

**Listing output (normal mode):**
```
$ targ
Targets:
  lint           Lint codebase              (github.com/toejough/go-targets)
  unit-test      Run tests                  (github.com/toejough/go-targets, renamed)
  coverage       Check coverage             (github.com/toejough/go-targets)
  check          Run all checks             (local)
```

### Debugging Portable Targets

**Common issues and diagnostics:**

1. **"No targets registered from this package"**
   - Check package path spelling
   - Verify import is present
   - Use `targ --list` to see what's registered

2. **Conflict between packages**
   - `targ --list` shows sources
   - Deregister one package or rename targets

3. **Target not found**
   - May have been deregistered
   - Check if package was imported
   - Verify target is exported (capitalized name)

**Debug checklist:**
```bash
# 1. What's imported?
$ grep "import" dev/targets.go

# 2. What's registered?
$ targ --list

# 3. Any conflicts?
$ targ  # Will error if conflicts exist

# 4. What's the actual source?
$ TARG_DEBUG=1 targ --list
```

## 17. Future Enhancements (Out of Scope)

### Config File Templating

**Problem:** golangci.toml and similar configs are still copy-paste.

**Possible solution:** Template expansion at sync time
```toml
# golangci.base.toml (in remote package)
[linters]
  enable = ["gofmt", "govet"]

# golangci.toml (local)
{{- include "github.com/toejough/go-targets/golangci.base.toml" }}
[[linters.exclusions.rules]]
  linters = ["depguard"]
  path = "{{.ProjectPath}}/cmd/.*"
```

**Complexity:** TOML doesn't have native includes, would need preprocessing.

**Recommendation:** Defer until user demand is clear.

### GUI/TUI for Sync Management

**Problem:** CLI listing is text-based, harder to visualize dependencies.

**Possible solution:** Interactive TUI for exploring remote packages
```
┌─ Remote Targets ─────────────────────────┐
│ github.com/toejough/go-targets           │
│   [x] lint      Lint codebase            │
│   [x] test      Run tests                │
│   [ ] coverage  Check coverage           │
│                                           │
│ github.com/bob/targets                   │
│   [x] deploy    Deploy to k8s            │
└───────────────────────────────────────────┘
```

**Complexity:** Need TUI library, state management for checkboxes.

**Recommendation:** Wait for user request. CLI is sufficient for MVP.

### Automated Conflict Resolution

**Problem:** User must manually resolve conflicts via DeregisterFrom.

**Possible solution:** Smart conflict resolution
- Prefer local over remote
- Prefer explicitly imported over transitively imported
- Interactive prompt

**Complexity:** Heuristics are hard to get right, may violate transparency principle.

**Recommendation:** Explicit is better than implicit. Keep manual resolution.

### Community Target Registry

**Problem:** No central place to discover community targets.

**Possible solution:** pkg.go.dev-style search for targ targets
- Tag convention: `//go:build targ`
- Search API: Find packages with targ targets
- Ratings/reviews

**Complexity:** Infrastructure cost, moderation, discovery UX.

**Recommendation:** Start with README awesome-list. Build registry if ecosystem grows.

## 18. Appendix: Example Flows

### Example 1: First-Time Sync

```bash
# User starts new Go project
$ mkdir myproject && cd myproject
$ go mod init github.com/me/myproject

# Sync standard targets
$ targ --sync github.com/toejough/go-targets
```

**Generated dev/targets.go:**
```go
//go:build targ

package main

import _ "github.com/toejough/go-targets"
```

```bash
# See what's available
$ targ
Targets:
  lint       Lint codebase       (github.com/toejough/go-targets)
  fmt        Format code         (github.com/toejough/go-targets)
  test       Run tests           (github.com/toejough/go-targets)
  coverage   Check coverage      (github.com/toejough/go-targets)
  check      Run all checks      (github.com/toejough/go-targets)

# Run a target
$ targ test
Running tests...
PASS
```

### Example 2: Selective Import + Customization

```bash
# Edit dev/targets.go
```

**dev/targets.go:**
```go
//go:build targ

package main

import (
    targets "github.com/toejough/go-targets"
    "github.com/toejough/targ"
)

func init() {
    // Don't want most targets, just cherry-pick
    targ.DeregisterFrom("github.com/toejough/go-targets")

    // Rename and organize
    targ.Register(
        targets.Lint,
        targets.Test.Name("unit-test"),
        targ.Group("ci", targets.Lint, targets.Test),
    )

    // Add local targets
    targ.Register(Deploy, Rollback)
}

var Deploy = targ.Targ(deploy).Description("Deploy to production")
var Rollback = targ.Targ(rollback).Description("Rollback deployment")

func deploy() error { /* ... */ }
func rollback() error { /* ... */ }
```

```bash
$ targ
Targets:
  lint         Lint codebase              (github.com/toejough/go-targets)
  unit-test    Run tests                  (github.com/toejough/go-targets, renamed)
  deploy       Deploy to production       (local)
  rollback     Rollback deployment        (local)

Groups:
  ci           lint, unit-test            (local)

$ targ ci
Running lint...
Running unit-test...
All checks passed.
```

### Example 3: Multiple Sources with Conflict

```bash
# Edit dev/targets.go
```

**dev/targets.go:**
```go
//go:build targ

package main

import (
    _ "github.com/alice/go-targets"  // Has "lint" target
    _ "github.com/bob/go-targets"    // Also has "lint" target
)
```

```bash
$ targ
targ: conflict: "lint" registered by both:
  - github.com/alice/go-targets
  - github.com/bob/go-targets
Use targ.DeregisterFrom() to resolve.
```

**Fix: Choose one source**
```go
//go:build targ

package main

import (
    _ "github.com/alice/go-targets"
    bobTargets "github.com/bob/go-targets"
    "github.com/toejough/targ"
)

func init() {
    // Deregister bob's targets, just want his deploy
    targ.DeregisterFrom("github.com/bob/go-targets")
    targ.Register(bobTargets.Deploy)
}
```

```bash
$ targ
Targets:
  lint       Lint codebase       (github.com/alice/go-targets)
  fmt        Format code         (github.com/alice/go-targets)
  test       Run tests           (github.com/alice/go-targets)
  deploy     Deploy to k8s       (github.com/bob/go-targets)

$ targ lint
Running lint...
PASS
```

### Example 4: Update Synced Targets

```bash
# Update remote package to latest
$ go get -u github.com/toejough/go-targets
go: upgraded github.com/toejough/go-targets v0.3.1 => v0.4.0

# Next targ invocation picks up changes
$ targ test
Running tests with new v0.4.0 features...
PASS

# See what changed
$ git diff go.mod
- github.com/toejough/go-targets v0.3.1
+ github.com/toejough/go-targets v0.4.0

# Commit the upgrade
$ git add go.mod go.sum
$ git commit -m "Update go-targets to v0.4.0"
```

### Example 5: Creating a Portable Target Package

**Directory structure:**
```
github.com/myorg/build-targets/
├── go.mod
├── targets.go
└── internal/
    └── impl.go
```

**targets.go:**
```go
package targets

import "github.com/toejough/targ"

// Public targets (exported for import)
var (
    Lint     = targ.Targ(lint).Description("Lint Go code")
    Test     = targ.Targ(test).Description("Run tests")
    Coverage = targ.Targ(coverage).Description("Check coverage")
    Build    = targ.Targ(build).Description("Build binary")
)

// Register all targets when imported
func init() {
    targ.Register(Lint, Test, Coverage, Build)
}

// Implementation functions
func lint() error { /* ... */ }
func test() error { /* ... */ }
func coverage() error { /* ... */ }
func build() error { /* ... */ }
```

**Users import with:**
```go
import _ "github.com/myorg/build-targets"  // All targets
// or
import targets "github.com/myorg/build-targets"  // Selective
```

**Source attribution happens automatically via runtime.Caller() in targ.Register().**

---

## Summary

This architecture extends targ with portable target support by:
1. **Leveraging Go's import system** - no new tooling, native workflow
2. **Automatic source attribution** - runtime.Caller() gives zero-config tracking
3. **Deferred conflict detection** - users resolve conflicts in their own init()
4. **Minimal API surface** - one new function (DeregisterFrom), existing patterns extended

The implementation maintains targ's transparency principle: every decision is traceable, errors name the fix, and Go code is the configuration format.

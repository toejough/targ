# Implementation Tasks

## 1. Alignment Summary

### Coverage Matrix

| Requirement | Architecture Component | Status |
|-------------|----------------------|--------|
| Sync from multiple remote sources | `Target.sourcePkg` + `runtime.Caller` | Covered |
| Targets work out of the box | Existing env var support | Covered |
| Re-sync pulls improvements | `go get -u` / go.mod | Covered (no code) |
| Clear attribution | `sourcePkg` + enhanced listing | Covered |
| Opt-in/opt-out flexibility | `DeregisterFrom()` + selective `Register()` | Covered |
| Transparent conflict resolution | `ConflictError` with actionable messages | Covered |
| Error on ambiguity | Deferred conflict detection | Covered |
| Config file sharing | N/A | Deferred |

### Issues Found

- [RESOLVED] `DeregisterFrom` return type → Returns `error` immediately (validates format), deferred validation for "package not found" at resolution time
- [RESOLVED] `registryEntry` wrapper type → Not needed, source lives on `Target.sourcePkg` directly, `[]any` registry unchanged
- [RESOLVED] Init ordering → Go guarantees imported inits run before importer inits, so `DeregisterFrom` in local init can reference targets from remote init

---

## 2. Task List

### TASK-001: Implement extractPackagePath

**Description:** Pure function that extracts a Go package path from a fully qualified function name returned by `runtime.FuncForPC`. This is the foundation for source attribution.

**Acceptance Criteria:**
- [ ] Extracts package from standard function: `"github.com/user/repo/pkg.Func"` → `"github.com/user/repo/pkg"`
- [ ] Extracts package from init: `"github.com/user/repo.init"` → `"github.com/user/repo"`
- [ ] Extracts package from init suffix: `"github.com/user/repo.init.0"` → `"github.com/user/repo"`
- [ ] Extracts package from closure in init: `"github.com/user/repo.init.func1"` → `"github.com/user/repo"`
- [ ] Handles nested packages: `"github.com/user/repo/internal/pkg.Func"` → `"github.com/user/repo/internal/pkg"`
- [ ] Returns empty string for empty input

**Files:**
- Create: `internal/core/source.go`

**Dependencies:** None

**Test Properties:**
1. `ExtractedPathIsPrefix`: For any valid function name, extracted path is a prefix of the input
2. `ExtractedPathHasNoDot`: Extracted path never contains a trailing dot
3. `EmptyInputReturnsEmpty`: Empty string input returns empty string

---

### TASK-002: Implement callerPackagePath

**Description:** Function that uses `runtime.Caller` at a given stack depth to determine the calling package's import path. Wraps `extractPackagePath` with the runtime call.

**Acceptance Criteria:**
- [ ] Returns the package path of the caller at the specified depth
- [ ] Returns error if `runtime.Caller` fails (invalid depth)
- [ ] Depth 1 returns immediate caller's package
- [ ] Depth 2 returns caller's caller's package

**Files:**
- Modify: `internal/core/source.go`

**Dependencies:** TASK-001

**Test Properties:**
1. `CallerFromThisPackageReturnsThisPackage`: Calling from test file returns test package path
2. `InvalidDepthReturnsError`: Very large depth returns error

---

### TASK-003: Add sourcePkg and nameOverridden fields to Target

**Description:** Add two new fields to the Target struct: `sourcePkg string` for tracking which package registered the target, and `nameOverridden bool` for tracking if `Name()` was called.

**Acceptance Criteria:**
- [ ] `sourcePkg` field added to Target struct
- [ ] `nameOverridden` field added to Target struct
- [ ] `Name()` builder sets `nameOverridden = true`
- [ ] `GetSource()` method returns `sourcePkg`
- [ ] `IsRenamed()` method returns `nameOverridden`
- [ ] Existing tests pass (no regressions)

**Files:**
- Modify: `internal/core/target.go`

**Dependencies:** None

**Test Properties:**
1. `NameSetsOverriddenFlag`: Calling `.Name(x)` on any target sets `IsRenamed()` to true
2. `DefaultIsNotRenamed`: Targets without `.Name()` have `IsRenamed()` false
3. `GetSourceReturnsSetValue`: Setting sourcePkg and calling `GetSource()` returns same value
4. `DefaultSourceIsEmpty`: New targets have empty `GetSource()`

---

### TASK-004: Implement source attribution in RegisterTarget

**Description:** Modify `RegisterTarget` to automatically detect and set `sourcePkg` on each target being registered, using `callerPackagePath` from TASK-002.

**Acceptance Criteria:**
- [ ] Each target registered via `RegisterTarget` gets its `sourcePkg` set to the calling package
- [ ] If `sourcePkg` is already set (non-empty), it is NOT overwritten
- [ ] Groups and non-Target types in the registry are handled gracefully (no panic)
- [ ] Existing registration behavior unchanged for all other purposes

**Files:**
- Modify: `internal/core/execute.go`

**Dependencies:** TASK-002, TASK-003

**Test Properties:**
1. `RegisteredTargetsHaveSource`: After registration, every target has non-empty `GetSource()`
2. `ExplicitSourceNotOverwritten`: If sourcePkg set before registration, it's preserved
3. `LocalTargetsGetLocalSource`: Targets registered from test package get test package path

---

### TASK-005: Implement deregistrations queue and DeregisterFrom

**Description:** Add a global `deregistrations` slice and implement `DeregisterFrom(packagePath string) error` that validates the package path and queues it for later resolution.

**Acceptance Criteria:**
- [ ] `DeregisterFrom("")` returns error (empty package path)
- [ ] `DeregisterFrom("valid/path")` queues the path and returns nil
- [ ] Multiple calls for different packages all queue successfully
- [ ] Multiple calls for the same package are idempotent (no error, no duplicate)
- [ ] Queued deregistrations are accessible for resolution phase

**Files:**
- Modify: `internal/core/execute.go`

**Dependencies:** None

**Test Properties:**
1. `EmptyPathReturnsError`: Empty string always errors
2. `ValidPathQueuesSuccessfully`: Non-empty path returns nil
3. `IdempotentForSamePackage`: Calling twice with same path doesn't duplicate

---

### TASK-006: Implement applyDeregistrations

**Description:** Pure function that takes the current registry `[]any` and a list of package paths to deregister, returning a filtered registry with those packages' targets removed. Returns error if a deregistration targets a package with no registered targets.

**Acceptance Criteria:**
- [ ] Removes all targets whose `sourcePkg` matches a deregistered package
- [ ] Returns error if a deregistered package had no targets in registry
- [ ] Non-Target items in registry (groups, etc.) are handled: check if they carry source info, skip if not
- [ ] Targets from non-deregistered packages are untouched
- [ ] Empty deregistration list returns registry unchanged

**Files:**
- Create: `internal/core/registry.go`

**Dependencies:** TASK-003

**Test Properties:**
1. `DeregisteredPackageFullyRemoved`: No targets from deregistered package remain
2. `OtherPackagesUntouched`: Targets from other packages are preserved exactly
3. `UnknownPackageErrors`: Deregistering a package with no targets returns error
4. `EmptyDeregistrationsNoOp`: Empty list returns registry unchanged
5. `MultiplePackagesDeregistered`: Can deregister multiple packages in one call

---

### TASK-007: Implement conflict detection

**Description:** Pure function that takes a registry `[]any` and detects duplicate target names from different source packages. Returns `ConflictError` with the conflicting name and both source packages.

**Acceptance Criteria:**
- [ ] No conflict when all names unique across packages
- [ ] No conflict when same name from same package (idempotent registration)
- [ ] Returns `ConflictError` when same name registered from different packages
- [ ] `ConflictError.Name` contains the conflicting target name
- [ ] `ConflictError.Sources` contains both package paths
- [ ] `ConflictError.Error()` message includes the name, both sources, and mentions `DeregisterFrom`
- [ ] Multiple conflicts all reported (not just the first)

**Files:**
- Modify: `internal/core/registry.go`

**Dependencies:** TASK-003

**Test Properties:**
1. `UniqueNamesNoConflict`: Registry with all unique names never errors
2. `SameNameSameSourceNoConflict`: Same name from same package is fine
3. `SameNameDifferentSourceConflicts`: Same name from different packages errors
4. `ErrorMessageContainsName`: ConflictError.Error() contains the target name
5. `ErrorMessageContainsSources`: ConflictError.Error() contains both package paths
6. `ErrorMessageSuggestsFix`: ConflictError.Error() contains "DeregisterFrom"

---

### TASK-008: Implement resolveRegistry

**Description:** Function that orchestrates registry resolution: applies deregistrations, then checks for conflicts. Called at the start of `ExecuteRegistered` before any targets run.

**Acceptance Criteria:**
- [ ] Applies deregistrations first, then checks conflicts
- [ ] Returns filtered registry on success
- [ ] Returns `DeregistrationError` if deregistered package not found
- [ ] Returns `ConflictError` if duplicate names remain after deregistration
- [ ] Clears deregistration queue after resolution

**Files:**
- Modify: `internal/core/registry.go`

**Dependencies:** TASK-006, TASK-007

**Test Properties:**
1. `DeregistrationBeforeConflictCheck`: Deregistering one side of a conflict resolves it
2. `DeregistrationErrorStopsResolution`: Bad deregistration prevents conflict check
3. `CleanRegistryPassesResolution`: No deregistrations + no conflicts succeeds
4. `QueueClearedAfterResolution`: Deregistration queue is empty after resolve

---

### TASK-009: Integrate resolveRegistry into ExecuteRegistered

**Description:** Wire `resolveRegistry` into the existing `ExecuteRegistered` flow so that registry resolution happens before target execution. Handle resolution errors by printing to stderr and exiting.

**Acceptance Criteria:**
- [ ] `resolveRegistry()` called at start of `ExecuteRegistered`
- [ ] On `ConflictError`: prints actionable error to stderr, exits with code 1
- [ ] On `DeregistrationError`: prints actionable error to stderr, exits with code 1
- [ ] On success: execution proceeds with filtered registry
- [ ] Existing `ExecuteRegistered` behavior unchanged when no deregistrations and no conflicts

**Files:**
- Modify: `internal/core/execute.go`

**Dependencies:** TASK-004, TASK-005, TASK-008

**Test Properties:**
1. `ExistingBehaviorUnchanged`: Without deregistrations, execution works as before
2. `ConflictPreventsExecution`: Conflicting targets cause error exit, no targets run
3. `DeregistrationErrorPreventsExecution`: Bad deregistration causes error exit

---

### TASK-010: Export DeregisterFrom in public API

**Description:** Add thin wrapper in `targ.go` that delegates to `internal/core.DeregisterFrom`.

**Acceptance Criteria:**
- [ ] `targ.DeregisterFrom(packagePath string) error` is exported
- [ ] Delegates to internal implementation
- [ ] Doc comment includes usage example
- [ ] Follows existing thin wrapper pattern in targ.go

**Files:**
- Modify: `targ.go`

**Dependencies:** TASK-009

**Test Properties:**
1. `PublicAPIDelegatesToInternal`: Calling `targ.DeregisterFrom` has same effect as internal version

---

### TASK-011: Enhance target listing with source attribution

**Description:** Modify the target listing output (shown when running `targ` with no arguments or `targ --list`) to display source package for each target and a "(renamed)" annotation when `Name()` was used on a synced target.

**Acceptance Criteria:**
- [ ] Each target shows `(source/package)` after description
- [ ] Local targets (empty sourcePkg) show `(local)`
- [ ] Renamed targets show `(source/package, renamed)`
- [ ] Output is terse and aligned (columns line up)
- [ ] Existing listing behavior unchanged for projects with no synced targets

**Files:**
- Modify: `internal/core/command.go`

**Dependencies:** TASK-003, TASK-009

**Test Properties:**
1. `LocalTargetsShowLocal`: Targets with empty source display "(local)"
2. `RemoteTargetsShowSource`: Targets with source display the package path
3. `RenamedTargetsAnnotated`: Targets where IsRenamed() is true show "(renamed)"
4. `NoSyncedTargetsUnchanged`: Output matches current format when all targets are local

---

### TASK-012: Create example portable target package

**Description:** Create a minimal example package demonstrating how to author portable targets. Includes exported target variables, init registration, and a README showing import patterns.

**Acceptance Criteria:**
- [ ] Example package at `examples/portable/remote/` with 2-3 simple targets
- [ ] Targets use env var args for parameterization (demonstrate convention)
- [ ] init() registers all targets
- [ ] Example consumer at `examples/portable/local/` showing Tier 1 (blank import), Tier 2 (selective), and Tier 3 (conflict resolution) patterns
- [ ] Examples compile and run

**Files:**
- Create: `examples/portable/remote/targets.go`
- Create: `examples/portable/local/targets.go`

**Dependencies:** TASK-010, TASK-011

**Test Properties:**
1. `ExamplesCompile`: All example files compile without errors

---

## 3. Execution Order

### Phase 1: Foundation (no dependencies)
Tasks: TASK-001, TASK-003, TASK-005
Can run in parallel: Yes

### Phase 2: Source Attribution (depends on foundation)
Tasks: TASK-002
Blocked by: TASK-001

### Phase 3: Registry Logic (depends on Target fields)
Tasks: TASK-006, TASK-007
Blocked by: TASK-003
Can run in parallel: Yes

### Phase 4: Integration (depends on attribution + registry)
Tasks: TASK-004, TASK-008
TASK-004 blocked by: TASK-002, TASK-003
TASK-008 blocked by: TASK-006, TASK-007

### Phase 5: Wiring (depends on integration)
Tasks: TASK-009
Blocked by: TASK-004, TASK-005, TASK-008

### Phase 6: Public API + Display (depends on wiring)
Tasks: TASK-010, TASK-011
Blocked by: TASK-009 (TASK-010), TASK-003 + TASK-009 (TASK-011)
Can run in parallel: Yes

### Phase 7: Documentation (depends on everything)
Tasks: TASK-012
Blocked by: TASK-010, TASK-011

---

## 4. Deferred Items

Features explicitly out of scope for this plan:

- **Config file sync** (golangci TOML, etc.) - Deferred until target sync is proven useful. Manual copy-paste acceptable for now.
- **"All except N" shorthand** - User deregisters and re-registers; no exclude syntax. Can add later if ergonomics demand it.
- **Community target registry** - Start with README awesome-list. Build registry if ecosystem grows.
- **TUI for sync management** - CLI is sufficient for MVP.
- **Automated conflict resolution** - Explicit is better than implicit. Keep manual resolution.
- **Starter repo (github.com/toejough/go-targets)** - Blocked on this feature existing first. Create after implementation.

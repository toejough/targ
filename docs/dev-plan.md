# Targ Rebuild Plan

## Goal

Rebuild targ from struct-based model to function-based Target Builder pattern following TDD with property-based testing.

## Approach

**Implement replacement + remove old together**: Never leave functionality gaps. When removing --init/--alias, have --create working first.

**Migrate dev/targets.go along the way**: Each phase converts local targets to use new patterns, proving the new APIs work.

**Stop and discuss**: If anything proves difficult or surprising, stop and talk through it.

## Priorities (User-Specified)

1. **Remove complexity first** - makes subsequent edits easier
2. **Add complexity last** - keeps edits/complexity down longer
3. **Keep targ functional** - override above if needed

## Test Architecture

**Stack**: imptest (DI/mocks), rapid (property-based), gomega (assertions)

**E2E without side effects**:

- Use existing `runEnv` interface and `ExecuteEnv` for testing
- Use `sh.execCommand` injection point for exec mocking
- Use `FileSystem` interface in buildtool for file operations

**Few integration tests**: Only verify dependency interactions (go modules, shell, file watching)

**User acceptance**: Only for usability/delight, not functionality

---

## Phase 1: Target Builder + Group (Minimal for --create)

Create Target and Group types that can be discovered and executed. This enables Phase 2 (--create).

### 1.1 Create Target Type

**New file**: `target.go`

```go
type Target struct {
    fn          any          // func or string
    name        string
    description string
}

func Targ(fn any) *Target
func (t *Target) Name(s string) *Target
func (t *Target) Description(s string) *Target
```

Start minimal - just enough for `targ.Targ("cmd")` to work. Add execution features (deps, cache, etc.) in later phases.

**Properties**:

```go
// Targ accepts function
rapid.Check(t, func(t *rapid.T) {
    target := Targ(func() {})
    gomega.Expect(target).NotTo(gomega.BeNil())
    gomega.Expect(target.fn).NotTo(gomega.BeNil())
})

// Targ accepts string
rapid.Check(t, func(t *rapid.T) {
    cmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(t, "cmd")
    target := Targ(cmd)
    gomega.Expect(target.fn).To(gomega.Equal(cmd))
})

// Builder chains preserve settings
rapid.Check(t, func(t *rapid.T) {
    name := rapid.StringMatching(`[a-z]+`).Draw(t, "name")
    desc := rapid.StringMatching(`[a-zA-Z ]+`).Draw(t, "desc")

    target := Targ(func() {}).Name(name).Description(desc)
    gomega.Expect(target.name).To(gomega.Equal(name))
    gomega.Expect(target.description).To(gomega.Equal(desc))
})
```

### 1.2 Create Group Type

**New file**: `group.go`

```go
type Group struct {
    name    string
    members []any // *Target or *Group
}

func Group(name string, members ...any) *Group
```

**Properties**:

```go
// Group accepts Targets and nested Groups
rapid.Check(t, func(t *rapid.T) {
    t1 := Targ(func() {})
    t2 := Targ(func() {})
    g1 := Group("inner", t1)
    g2 := Group("outer", g1, t2)

    gomega.Expect(len(g2.members)).To(gomega.Equal(2))
})
```

### 1.3 Integrate Target/Group with Discovery

**Files**: `internal/core/command.go` - extend `parseTarget`

Extend to handle `*Target` and `*Group` in addition to existing structs.

**Properties**:

- `parseTarget(*Target)` creates commandNode
- `parseTarget(*Group)` creates commandNode with children
- String targets (from `Targ("cmd")`) execute via shell
- Help output shows name and description

**Functional check**: Can register and run a simple `targ.Targ(func() {})` target

---

## Phase 2: --create + Remove Old Flags

Now that Target exists, implement --create and remove obsolete flags.

### 2.1 Implement --create

**New**: `internal/create/create.go`

```
targ --create lint "golangci-lint run"
targ --create dev lint fast "golangci-lint run"  # creates dev/lint/fast
```

**Behavior**:

- Creates groups as needed for nested paths
- Generates `targ.Targ("cmd")` code
- Appends to existing targ file (or creates ./targs.go)

**Properties**:

```go
// Path creates nested groups
rapid.Check(t, func(t *rapid.T) {
    depth := rapid.IntRange(1, 4).Draw(t, "depth")
    path := make([]string, depth)
    for i := range path {
        path[i] = rapid.StringMatching(`[a-z]+`).Draw(t, fmt.Sprintf("seg%d", i))
    }
    args := append([]string{"targ", "--create"}, path...)
    args = append(args, "echo hello")

    result := executeCreate(args)
    // verify generated code has targ.Targ() and targ.Group()
})
```

### 2.2 Remove --init, --alias, --move

**Files**: `cmd/targ/main.go`

Remove:

- `handleInitFlag`, `handleAliasFlag`, `handleMoveFlag`
- All supporting functions (~800 LOC)

**Functional check**:

- `targ --create test "echo hello"` works
- `targ build` still works
- Old flags error with "use --create instead"

### 2.3 Rename --no-cache to --no-binary-cache

**Files**: `cmd/targ/main.go` - `extractTargFlags`

**Properties**:

- `--no-binary-cache` disables binary caching
- `--no-cache` still works (deprecation warning)

### 2.4 Remove --keep

**Files**: `cmd/targ/main.go` - Remove `keepBootstrap` handling

---

## Phase 3: Migrate dev/targets.go to Target Builder

Convert existing struct-based targets to function + Target builder pattern:

**Before** (current):

```go
type Coverage struct {
    HTML bool `targ:"flag,desc=Open HTML report"`
}
func (c *Coverage) Run() error { ... }
```

**After** (new):

```go
func Coverage(args CoverageArgs) error { ... }
var coverage = targ.Targ(Coverage).Description("Display coverage report")
```

**Functional check**: `targ coverage` works with new pattern

---

## Phase 4: Execution Features (Deps, Cache, Watch)

Implement core execution features. Add builder methods to Target.

### 4.1 Add Execution Builder Methods

Extend `target.go`:

```go
func (t *Target) Deps(targets ...*Target) *Target
func (t *Target) ParallelDeps(targets ...*Target) *Target
func (t *Target) Cache(patterns ...string) *Target
func (t *Target) Watch(patterns ...string) *Target
```

### 4.2 Deps Execution

Reuse existing `internal/core/deps.go` dep tracking logic.

**Properties**:

```go
// Each dep executes exactly once
rapid.Check(t, func(t *rapid.T) {
    execCounts := make(map[string]int)
    a := Targ(func() { execCounts["a"]++ })
    b := Targ(func() { execCounts["b"]++ }).Deps(a)
    c := Targ(func() { execCounts["c"]++ }).Deps(a, b)

    c.Run(ctx)

    gomega.Expect(execCounts["a"]).To(gomega.Equal(1))
    gomega.Expect(execCounts["b"]).To(gomega.Equal(1))
    gomega.Expect(execCounts["c"]).To(gomega.Equal(1))
})

// Serial deps run in order
// Parallel deps run concurrently (verify via timing)
```

### 4.3 .Cache()

Reuse `file/checksum.go`.

**Properties**:

- Cache hit skips execution
- Cache miss runs execution
- File change invalidates cache
- `--no-cache` bypasses

### 4.4 .Watch()

Reuse `file/watch.go`.

**Properties**:

- File change triggers re-run
- `targ.ResetDeps()` clears dep cache
- Ctrl+C cancels cleanly

### 4.5 Migrate dev/targets.go

Add deps, cache, watch to targets that need them:

```go
var build = targ.Targ(Build).Cache("**/*.go", "go.mod", "go.sum")
var check = targ.Targ(Check).Deps(fmt, lint, test)
```

**Functional check**: `targ check` runs deps in order, caching works

---

## Phase 5: Repetition Features (.Times, .While, .Retry, .Backoff, .Timeout)

Implement repetition and resilience features.

### 5.1 .Times() and .While()

**Properties**:

```go
// Times stops on failure without retry
rapid.Check(t, func(t *rapid.T) {
    n := rapid.IntRange(1, 10).Draw(t, "n")
    failAt := rapid.IntRange(1, n).Draw(t, "failAt")
    count := 0

    target := Targ(func() error {
        count++
        if count == failAt { return errors.New("fail") }
        return nil
    }).Times(n)

    target.Run(ctx)
    gomega.Expect(count).To(gomega.Equal(failAt))
})

// Combined times+while: earliest wins
```

### 5.2 .Retry() and .Backoff()

**Properties**:

```go
// Times completes all with retry
rapid.Check(t, func(t *rapid.T) {
    n := rapid.IntRange(1, 10).Draw(t, "n")
    count := 0

    target := Targ(func() error {
        count++
        return errors.New("always fail")
    }).Times(n).Retry()

    target.Run(ctx)
    gomega.Expect(count).To(gomega.Equal(n))
})

// Backoff delays after failure (verify via timing)
```

### 5.3 .Timeout()

**Properties**:

- Cancels context after duration
- Nested timeouts: inner wins if smaller

### 5.4 Migrate dev/targets.go

Add timeout/retry to flaky targets if any exist.

---

## Phase 6: Runtime Override Flags

Add CLI flags that override compile-time settings.

### 6.1 Add Override Flags

```
--watch "pattern"
--cache "pattern"
--cache-dir "path"
--timeout duration
--deps target1,target2
--dep-mode parallel
--times N
--while "cmd"
--retry
--backoff D,M
```

**Files**: `internal/core/parse.go`

**Behavior**:

- `--cache-dir` sets custom cache directory (default: `.targ-cache`)
- Help output shows cache directory in use

**Properties**:

```go
// Flags parse correctly
rapid.Check(t, func(t *rapid.T) {
    times := rapid.IntRange(1, 100).Draw(t, "times")
    args := []string{"targ", "--times", strconv.Itoa(times), "build"}
    parsed := parseArgs(args)
    gomega.Expect(parsed.times).To(gomega.Equal(times))
})

// Invalid flags error clearly
// Override applies to correct target
```

### 6.2 Ownership Model (targ.Disabled)

**Properties**:

- `.Watch(targ.Disabled)` allows user-defined --watch
- Without Disabled: conflict = error
- User-defined flag receives parsed value

### 6.3 Migrate dev/targets.go

Test override flags on local targets: `targ build --watch "**/*.go"`

---

## Phase 7: Shell Support (targ.Shell + String Targets)

### 7.1 targ.Shell(ctx, cmd, args)

**New function**: `shell.go`

**Properties**:

```go
// $var substitution from struct fields
rapid.Check(t, func(t *rapid.T) {
    name := rapid.StringMatching(`[a-z]+`).Draw(t, "name")
    args := struct{ Name string }{Name: name}

    result := substituteVars("echo $name", args)
    gomega.Expect(result).To(gomega.Equal(fmt.Sprintf("echo %s", name)))
})

// Unknown $var errors
// Context cancellation propagates
```

### 7.2 String Targets: targ.Targ("cmd $var")

**Properties**:

- Infers flags from $var
- Short flags from first letter
- Collision skips short for later args

### 7.3 Migrate dev/targets.go

Convert shell-heavy targets to use `targ.Shell()` or string targets:

```go
var lint = targ.Targ("golangci-lint run ./...").Description("Run linter")
```

---

## Phase 8: --sync Remote Import

```
targ --sync github.com/foo/bar
```

**Properties**:

- Creates/updates import
- Registers exported targets
- Naming conflicts error clearly
- Source tracking in help output

---

## Phase 9: Additional Global Flags

### 9.1 --parallel/-p

```
targ -p build test lint
```

**Properties**: Parallel targets share dep state

### 9.2 --source/-s

```
targ -s ./dev/targs.go build
```

### 9.3 --to-func, --to-string

**Properties**:

- `--to-func` expands string target
- `--to-string` errors if not simple Shell

### 9.4 Migrate dev/targets.go

Test `targ -p fmt lint test` for parallel execution.

---

## Phase 10: Remove Struct Model (LOC: -2,000)

Final cleanup. At this point, dev/targets.go should already be fully migrated to Target builder pattern.

**Prerequisite**: All tests pass with Target-only model, dev/targets.go uses no struct targets

**Files**:

- `internal/core/command.go` - Remove struct parsing (~1,500 LOC)
- `cmd/targ/main.go` - Remove struct wrapper generation (~500 LOC)

**Verification**: `targ check` works, no struct-based targets remain

---

## Critical Files

| File                        | Role                                 |
| --------------------------- | ------------------------------------ |
| `targ.go`                   | Public API (Targ, Group, Shell, Run) |
| `target.go`                 | Target type and builder (new)        |
| `group.go`                  | Group type (new)                     |
| `internal/core/command.go`  | Execution logic, Target integration  |
| `internal/core/run_env.go`  | Testing abstraction (extend)         |
| `internal/core/deps.go`     | Dependency tracking (reuse)          |
| `cmd/targ/main.go`          | Build tool, discovery, flags         |
| `internal/create/create.go` | --create scaffold (new)              |
| `dev/targets.go`            | Local targets to migrate as we go    |

---

## When to Stop and Discuss

Stop and talk through if:

1. **Testing setup proves difficult**: Can't get imptest/rapid working cleanly
2. **Discovery integration is complex**: Mixing struct and Target models harder than expected
3. **Bootstrap generation changes**: More invasive than expected
4. **Struct removal is risky**: Too many places depend on struct model
5. **Any phase takes significantly longer**: Something is harder than expected

---

## Testing Strategy

**All tests are property-based** using rapid/imptest/gomega. No separate "unit tests" - everything is expressed as properties that must hold.

**Fuzz testing** at boundaries where unbounded input is possible:

- CLI argument parsing (user input)
- File pattern matching (user input)
- Shell command parsing (user input)
- Go source parsing (dependency boundary)
- Module resolution (dependency boundary)

If you can't quickly enumerate all input combinations, fuzz it.

**Linter compliance**: After tests pass, run `targ targets check` and address all concerns. Do not apply blanket linter ignore flags (file-level or config-level) without discussing first.

---

## Verification

After each phase:

1. **Property tests pass**: `go test ./...` (all tests are property-based)
2. **Fuzz tests pass**: Boundaries covered
3. **`targ check` passes**: Address any linter/vet concerns (or discuss exceptions)
4. **Functional check**: dev/targets.go changes work
5. **Migration check**: Local targets use new patterns

Final verification:

1. All dev/targets.go targets use Target builder pattern
2. Run via `targ <target>` - all targets work
3. Test execution modifiers: `targ build --watch "**/*.go"`
4. Verify help output shows correct metadata
5. Shell completion works
6. `targ check` passes with no exceptions

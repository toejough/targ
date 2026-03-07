# Fix --help Source Attribution Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix `targ --help` to show clean package paths for remote targets and correct file paths for local string/deps-only targets instead of module cache paths and `(unknown)`.

**Architecture:** Add a `sourceFile` field to `Target` populated via `runtime.Caller` in `Targ()` for non-function targets. In `parseTargetLike()`, prefer `sourcePkg` (for remote targets) over file paths, and fall back to the new `sourceFile` for local string/deps-only targets.

**Tech Stack:** Go 1.25.5, gomega (assertions), rapid (property-based testing)

---

### Task 1: Add `sourceFile` field and `GetSourceFile()` to Target

**Files:**
- Modify: `internal/core/target.go:51-77` (Target struct)
- Modify: `internal/core/target.go:252-255` (near GetSource)
- Test: `internal/core/target_test.go`

**Step 1: Write the failing test**

Add to `internal/core/target_test.go`:

```go
func TestProperty_StringTargetCapturesSourceFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	target := core.Targ("echo hello")
	g.Expect(target.GetSourceFile()).ToNot(BeEmpty(),
		"string targets should capture source file")
	g.Expect(target.GetSourceFile()).To(HaveSuffix("target_test.go"),
		"source file should point to the calling file")
}

func TestProperty_DepsOnlyTargetCapturesSourceFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	target := core.Targ()
	g.Expect(target.GetSourceFile()).ToNot(BeEmpty(),
		"deps-only targets should capture source file")
	g.Expect(target.GetSourceFile()).To(HaveSuffix("target_test.go"),
		"source file should point to the calling file")
}

func TestProperty_FuncTargetHasNoSourceFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	target := core.Targ(func() {})
	g.Expect(target.GetSourceFile()).To(BeEmpty(),
		"function targets should not have sourceFile set (funcSourceFile handles them)")
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run 'TestProperty_StringTargetCapturesSourceFile|TestProperty_DepsOnlyTargetCapturesSourceFile|TestProperty_FuncTargetHasNoSourceFile' -v`
Expected: Compilation error — `GetSourceFile` method does not exist.

**Step 3: Write minimal implementation**

In `internal/core/target.go`, add `sourceFile` field to the Target struct (after line 75, the `sourcePkg` field):

```go
	sourceFile     string // file path captured at Targ() for non-function targets
```

Add `GetSourceFile()` method (after the existing `GetSource()` method around line 254):

```go
// GetSourceFile returns the file path where this target was defined.
// Only set for string and deps-only targets (function targets use funcSourceFile).
func (t *Target) GetSourceFile() string {
	return t.sourceFile
}
```

In `Targ()` (around line 614), capture `runtime.Caller(1)` for string and deps-only cases:

```go
func Targ(fn ...any) *Target {
	if len(fn) == 0 {
		// Deps-only target with no function
		_, file, _, _ := runtime.Caller(1)
		return &Target{sourceFile: file}
	}

	if len(fn) > 1 {
		panic("targ.Targ: expected at most one argument")
	}

	f := fn[0]
	if f == nil {
		panic("targ.Targ: fn cannot be nil")
	}

	// Validate fn is a function or string
	switch v := f.(type) {
	case string:
		if v == "" {
			panic("targ.Targ: shell command cannot be empty")
		}

		_, file, _, _ := runtime.Caller(1)
		return &Target{fn: f, sourceFile: file}
	default:
		fnValue := reflect.ValueOf(f)
		if fnValue.Kind() != reflect.Func {
			panic(fmt.Sprintf("targ.Targ: expected func or string, got %T", f))
		}
	}

	return &Target{fn: f}
}
```

Note: `runtime` is already imported in `target.go` — verify with `grep "runtime" internal/core/target.go`. If not, it needs to be added to the imports. Actually `runtime` is imported in `source.go` (same package), but each file needs its own imports. Check if `target.go` imports `runtime` — if not, add it.

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run 'TestProperty_StringTargetCapturesSourceFile|TestProperty_DepsOnlyTargetCapturesSourceFile|TestProperty_FuncTargetHasNoSourceFile' -v`
Expected: PASS

**Step 5: Run full test suite for the package**

Run: `go test -tags sqlite_fts5 ./internal/core/ -v`
Expected: All tests pass (no regressions)

**Step 6: Commit**

```
feat(core): capture source file in Targ() for string and deps-only targets
```

---

### Task 2: Use `sourcePkg` and `sourceFile` in `parseTargetLike()`

**Files:**
- Modify: `internal/core/command.go:1637-1696` (parseTargetLike function)
- Test: `internal/core/command_test.go`

**Step 1: Write the failing tests**

Add to `internal/core/command_test.go`:

```go
func TestParseTargetLike_RemoteTargetUsesSourcePkg(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	target := core.Targ(func() {}).Name("lint")
	target.SetSourceForTest("github.com/toejough/targ/dev")

	node, err := core.ParseTargetLikeForTest(target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node.SourceFile).To(Equal("github.com/toejough/targ/dev"),
		"remote targets should use sourcePkg as SourceFile")
}

func TestParseTargetLike_LocalStringTargetUsesSourceFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	target := core.Targ("echo hello").Name("hello")

	node, err := core.ParseTargetLikeForTest(target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node.SourceFile).ToNot(BeEmpty(),
		"local string targets should have SourceFile set")
	g.Expect(node.SourceFile).To(HaveSuffix("command_test.go"),
		"local string targets should show defining file")
}

func TestParseTargetLike_LocalDepsOnlyTargetUsesSourceFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	target := core.Targ().Name("all")

	node, err := core.ParseTargetLikeForTest(target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node.SourceFile).ToNot(BeEmpty(),
		"local deps-only targets should have SourceFile set")
	g.Expect(node.SourceFile).To(HaveSuffix("command_test.go"),
		"local deps-only targets should show defining file")
}

func TestParseTargetLike_LocalFuncTargetKeepsExistingSourceFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	target := core.Targ(func() {}).Name("build")

	node, err := core.ParseTargetLikeForTest(target)
	g.Expect(err).ToNot(HaveOccurred())
	// Local function targets have empty sourcePkg (cleared during resolution)
	// and their SourceFile comes from funcSourceFile(), which points to this test file
	g.Expect(node.SourceFile).To(HaveSuffix("command_test.go"),
		"local func targets should keep funcSourceFile path")
}

func TestParseTargetLike_RemoteStringTargetUsesSourcePkg(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	target := core.Targ("kubectl apply").Name("deploy")
	target.SetSourceForTest("github.com/company/infra/dev")

	node, err := core.ParseTargetLikeForTest(target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node.SourceFile).To(Equal("github.com/company/infra/dev"),
		"remote string targets should use sourcePkg as SourceFile")
}

func TestParseTargetLike_RemoteFuncTargetUsesSourcePkg(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	target := core.Targ(func() {}).Name("lint")
	target.SetSourceForTest("github.com/toejough/targ/dev")

	node, err := core.ParseTargetLikeForTest(target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(node.SourceFile).To(Equal("github.com/toejough/targ/dev"),
		"remote func targets should prefer sourcePkg over module cache path")
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run 'TestParseTargetLike_Remote|TestParseTargetLike_Local' -v`
Expected: FAIL — remote targets still show file paths, string/deps-only targets still have empty SourceFile.

**Step 3: Write minimal implementation**

In `internal/core/command.go`, modify `parseTargetLike()`. After the existing block at line 1690-1693 that stores the Target reference, add source file resolution:

```go
	// Store Target reference for dep execution
	if t, ok := target.(*Target); ok {
		node.Target = t

		// Source file resolution:
		// 1. Remote targets (non-empty sourcePkg): use package path as display source
		// 2. Local string/deps-only targets: use sourceFile captured at Targ()
		// 3. Local function targets: keep funcSourceFile path (already set above)
		if src := t.GetSource(); src != "" {
			node.SourceFile = src
		} else if sf := t.GetSourceFile(); sf != "" && node.SourceFile == "" {
			node.SourceFile = sf
		}
	}
```

**Step 4: Run tests to verify they pass**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run 'TestParseTargetLike_Remote|TestParseTargetLike_Local' -v`
Expected: PASS

**Step 5: Run full test suite for the package**

Run: `go test -tags sqlite_fts5 ./internal/core/ -v`
Expected: All tests pass

**Step 6: Commit**

```
fix(core): use sourcePkg for remote target source display in help
```

---

### Task 3: Update test helper and add export if needed

**Files:**
- Modify: `internal/core/export_test.go:30-38` (NewTargetForTest)

**Step 1: Check if `NewTargetForTest` needs a `sourceFile` param**

Read `internal/core/export_test.go`. The existing `NewTargetForTest(name, desc, sourcePkg, nameOverridden)` creates targets for tests that check source-related behavior. If any tests need to set `sourceFile` on test targets, add an optional parameter. However, existing tests that use `NewTargetForTest` only test sourcePkg-related flows, so this is likely unnecessary.

If needed, add a `SetSourceFileForTest` method to `Target` in `target.go`:

```go
// SetSourceFileForTest sets the source file path (for testing only).
func (t *Target) SetSourceFileForTest(file string) {
	t.sourceFile = file
}
```

**Step 2: Run full test suite**

Run: `go test -tags sqlite_fts5 ./internal/core/ -v`
Expected: All tests pass

**Step 3: Run check-for-fail**

Run: `go run -tags targ ./cmd/targ check-for-fail`
Expected: All checks pass

**Step 4: Commit (if changes were made)**

```
test(core): add SetSourceFileForTest helper for source file testing
```

---

### Task 4: Run full validation

**Step 1: Run all tests**

Run: `go test -tags sqlite_fts5 ./...`
Expected: All tests pass

**Step 2: Run check-for-fail**

Run: `go run -tags targ ./cmd/targ check-for-fail`
Expected: All checks pass

**Step 3: Run go vet**

Run: `go vet ./...`
Expected: No issues

**Step 4: Final commit if needed, then done**

No commit needed if all previous commits are clean.

# Dep Group Chaining Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable `Deps()` chaining with mixed serial/parallel groups that execute in order, with back-to-back same-mode coalescing.

**Architecture:** Replace flat `deps []*Target` + `depMode DepMode` with `depGroups []depGroup`. Each group has targets and a mode. Groups execute sequentially; targets within a group run per the group's mode. Coalescing merges consecutive same-mode groups.

**Tech Stack:** Go, gomega, rapid (property-based testing)

---

### Task 1: Add `depGroup` struct and `DepModeMixed` constant

**Files:**
- Modify: `internal/core/target.go:16-25` (DepMode constants), `internal/core/target.go:37-42` (Target struct fields)
- Modify: `internal/core/types.go:115-122` (TargetExecutionLike interface)
- Modify: `targ.go:14-18` (exported constants)

**Step 1: Write the failing test**

Add to `internal/core/target_test.go`:

```go
func TestProperty_DepGroupChaining(t *testing.T) {
	t.Parallel()

	t.Run("SingleSerialGroup", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {})
		b := core.Targ(func() {})
		main := core.Targ(func() {}).Deps(a, b)

		groups := main.GetDepGroups()
		g.Expect(groups).To(HaveLen(1))
		g.Expect(groups[0].Targets).To(Equal([]*core.Target{a, b}))
		g.Expect(groups[0].Mode).To(Equal(core.DepModeSerial))
		g.Expect(main.GetDepMode()).To(Equal(core.DepModeSerial))
	})

	t.Run("SingleParallelGroup", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {})
		b := core.Targ(func() {})
		main := core.Targ(func() {}).Deps(a, b, core.DepModeParallel)

		groups := main.GetDepGroups()
		g.Expect(groups).To(HaveLen(1))
		g.Expect(groups[0].Targets).To(Equal([]*core.Target{a, b}))
		g.Expect(groups[0].Mode).To(Equal(core.DepModeParallel))
		g.Expect(main.GetDepMode()).To(Equal(core.DepModeParallel))
	})

	t.Run("CoalescesSameMode", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {})
		b := core.Targ(func() {})
		main := core.Targ(func() {}).Deps(a).Deps(b)

		groups := main.GetDepGroups()
		g.Expect(groups).To(HaveLen(1))
		g.Expect(groups[0].Targets).To(Equal([]*core.Target{a, b}))
		g.Expect(groups[0].Mode).To(Equal(core.DepModeSerial))
	})

	t.Run("CoalescesSameModeParallel", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {})
		b := core.Targ(func() {})
		main := core.Targ(func() {}).Deps(a, core.DepModeParallel).Deps(b, core.DepModeParallel)

		groups := main.GetDepGroups()
		g.Expect(groups).To(HaveLen(1))
		g.Expect(groups[0].Targets).To(Equal([]*core.Target{a, b}))
		g.Expect(groups[0].Mode).To(Equal(core.DepModeParallel))
	})

	t.Run("MixedModeCreatesMultipleGroups", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {})
		b := core.Targ(func() {})
		c := core.Targ(func() {})
		d := core.Targ(func() {})
		main := core.Targ(func() {}).
			Deps(a).
			Deps(b, c, core.DepModeParallel).
			Deps(d)

		groups := main.GetDepGroups()
		g.Expect(groups).To(HaveLen(3))
		g.Expect(groups[0].Targets).To(Equal([]*core.Target{a}))
		g.Expect(groups[0].Mode).To(Equal(core.DepModeSerial))
		g.Expect(groups[1].Targets).To(Equal([]*core.Target{b, c}))
		g.Expect(groups[1].Mode).To(Equal(core.DepModeParallel))
		g.Expect(groups[2].Targets).To(Equal([]*core.Target{d}))
		g.Expect(groups[2].Mode).To(Equal(core.DepModeSerial))
		g.Expect(main.GetDepMode()).To(Equal(core.DepModeMixed))
	})

	t.Run("GetDepsFlattensAllGroups", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {})
		b := core.Targ(func() {})
		c := core.Targ(func() {})
		main := core.Targ(func() {}).
			Deps(a).
			Deps(b, core.DepModeParallel).
			Deps(c)

		g.Expect(main.GetDeps()).To(Equal([]*core.Target{a, b, c}))
	})

	t.Run("NoDepsReturnsEmptyGroups", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		main := core.Targ(func() {})

		g.Expect(main.GetDepGroups()).To(BeEmpty())
		g.Expect(main.GetDeps()).To(BeEmpty())
		g.Expect(main.GetDepMode()).To(Equal(core.DepModeSerial))
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestProperty_DepGroupChaining -v`
Expected: FAIL — `GetDepGroups` method doesn't exist, `DepModeMixed` undefined.

**Step 3: Write minimal implementation**

In `internal/core/target.go`:

1. Add `DepModeMixed` constant:
```go
const (
	DepModeSerial DepMode = iota
	DepModeParallel
	DepModeMixed
)
```

2. Add `depGroup` struct and `DepGroup` exported view:
```go
type depGroup struct {
	targets []*Target
	mode    DepMode
}

// DepGroup is the exported view of a dependency group.
type DepGroup struct {
	Targets []*Target
	Mode    DepMode
}
```

3. Replace Target struct fields:
```go
// Remove:
//   deps    []*Target
//   depMode DepMode
// Add:
depGroups []depGroup
```

4. Update `String()`:
```go
func (m DepMode) String() string {
	switch m {
	case DepModeParallel:
		return depModeParallelStr
	case DepModeMixed:
		return depModeMixedStr
	default:
		return depModeSerialStr
	}
}
```
Add `depModeMixedStr = "mixed"` to the constants block.

5. Rewrite `Deps()`:
```go
func (t *Target) Deps(args ...any) *Target {
	mode := DepModeSerial
	var targets []*Target

	for _, arg := range args {
		switch v := arg.(type) {
		case *Target:
			targets = append(targets, v)
		case DepMode:
			mode = v
		}
	}

	if len(targets) == 0 {
		return t
	}

	// Coalesce with last group if same mode
	if len(t.depGroups) > 0 && t.depGroups[len(t.depGroups)-1].mode == mode {
		t.depGroups[len(t.depGroups)-1].targets = append(
			t.depGroups[len(t.depGroups)-1].targets, targets...)
	} else {
		t.depGroups = append(t.depGroups, depGroup{targets: targets, mode: mode})
	}

	return t
}
```

6. Update `GetDeps()`:
```go
func (t *Target) GetDeps() []*Target {
	var all []*Target
	for _, g := range t.depGroups {
		all = append(all, g.targets...)
	}
	return all
}
```

7. Update `GetDepMode()`:
```go
func (t *Target) GetDepMode() DepMode {
	if len(t.depGroups) == 0 {
		return DepModeSerial
	}

	mode := t.depGroups[0].mode
	for _, g := range t.depGroups[1:] {
		if g.mode != mode {
			return DepModeMixed
		}
	}
	return mode
}
```

8. Add `GetDepGroups()`:
```go
func (t *Target) GetDepGroups() []DepGroup {
	groups := make([]DepGroup, len(t.depGroups))
	for i, g := range t.depGroups {
		groups[i] = DepGroup{Targets: g.targets, Mode: g.mode}
	}
	return groups
}
```

9. In `internal/core/types.go`, update the interface:
```go
type TargetExecutionLike interface {
	GetDeps() []*Target
	GetDepMode() DepMode
	GetDepGroups() []DepGroup
	GetTimeout() time.Duration
	GetTimes() int
	GetRetry() bool
	GetBackoff() (time.Duration, float64)
}
```

10. In `targ.go`, add exports:
```go
const (
	DepModeParallel = core.DepModeParallel
	DepModeSerial   = core.DepModeSerial
	DepModeMixed    = core.DepModeMixed
	// ...
)

type DepGroup = core.DepGroup
```

11. Fix all internal references to `t.deps` and `t.depMode` — there are references in `runDeps`, `runDepsParallel`, `runDepsSerial`, `Run`, and `command.go:1915`.

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run TestProperty_DepGroupChaining -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS — existing dep tests still work since single-call `Deps()` behavior is preserved.

**Step 6: Commit**

```
feat(core): add dep group chaining with coalescing
```

---

### Task 2: Update execution to iterate dep groups

**Files:**
- Modify: `internal/core/target.go:382-429` (runDeps, runDepsParallel, runDepsSerial)

**Step 1: Write the failing test**

Add to `test/execution_properties_test.go` (in the existing dep execution section near line 260):

```go
t.Run("DependencyChainedGroupExecution", func(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var mu sync.Mutex
	var order []string

	a := targ.Targ(func() {
		mu.Lock()
		order = append(order, "a")
		mu.Unlock()
	}).Name("a")

	b := targ.Targ(func() {
		mu.Lock()
		order = append(order, "b")
		mu.Unlock()
	}).Name("b")

	c := targ.Targ(func() {
		mu.Lock()
		order = append(order, "c")
		mu.Unlock()
	}).Name("c")

	main := targ.Targ(func() {
		mu.Lock()
		order = append(order, "main")
		mu.Unlock()
	}).Deps(a).Deps(b, c, targ.DepModeParallel)

	err := main.Run(context.Background())
	g.Expect(err).NotTo(HaveOccurred())

	// a must come first (serial group 1), then b and c (parallel group 2), then main
	g.Expect(order[0]).To(Equal("a"))
	g.Expect(order[len(order)-1]).To(Equal("main"))
	// b and c are in the middle (parallel, order may vary)
	g.Expect(order[1:3]).To(ConsistOf("b", "c"))
})

t.Run("DependencyChainedGroupError", func(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	executed := false

	a := targ.Targ(func() error {
		return fmt.Errorf("fail in group 1")
	}).Name("a")

	b := targ.Targ(func() {
		executed = true
	}).Name("b")

	main := targ.Targ(func() {}).
		Deps(a).
		Deps(b, targ.DepModeParallel)

	err := main.Run(context.Background())
	g.Expect(err).To(HaveOccurred())
	g.Expect(executed).To(BeFalse(), "group 2 should not run if group 1 fails")
})
```

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./test/ -run "DependencyChainedGroup" -v`
Expected: FAIL — `runDeps` still uses flat `t.deps` field (won't compile after Task 1).

**Step 3: Write minimal implementation**

Replace `runDeps`, `runDepsParallel`, `runDepsSerial` in `internal/core/target.go`:

```go
func (t *Target) runDeps(ctx context.Context) error {
	for _, group := range t.depGroups {
		var err error
		if group.mode == DepModeParallel {
			err = runGroupParallel(ctx, group.targets)
		} else {
			err = runGroupSerial(ctx, group.targets)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func runGroupParallel(ctx context.Context, targets []*Target) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, len(targets))

	for _, dep := range targets {
		go func(d *Target) {
			errs <- d.Run(ctx)
		}(dep)
	}

	var firstErr error
	for range targets {
		err := <-errs
		if err != nil && firstErr == nil {
			firstErr = err
			cancel()
		}
	}

	return firstErr
}

func runGroupSerial(ctx context.Context, targets []*Target) error {
	for _, dep := range targets {
		err := dep.Run(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
```

Update `Run()` at line ~442 to use `depGroups`:
```go
if len(t.depGroups) > 0 {
	err := t.runDeps(ctx)
	...
}
```

Update `command.go:1915` similarly:
```go
if node.Target != nil && len(node.Target.depGroups) > 0 {
```

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./test/ -run "DependencyChainedGroup" -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS

**Step 6: Commit**

```
feat(core): execute dep groups sequentially with per-group mode
```

---

### Task 3: Update help display for chained groups

**Files:**
- Modify: `internal/core/command.go:126` (commandNode struct), `internal/core/command.go:215-226` (appendDepsLine), `internal/core/command.go:1615-1627` (node building)

**Step 1: Write the failing test**

Find the existing help display tests and add a test for chained group formatting. Search for `appendDepsLine` or help display tests to locate the right file.

Add a test that builds a target with chained deps and verifies the help output contains `→` separators. The test should check:
- Single serial group: `Deps: a, b (serial)` — unchanged
- Single parallel group: `Deps: a, b (parallel)` — unchanged
- Mixed groups: `Deps: a → b, c (parallel) → d`

**Step 2: Run test to verify it fails**

Expected: FAIL — current display logic uses flat `Deps []string` + single `DepMode string`.

**Step 3: Write minimal implementation**

1. In `commandNode`, replace:
```go
// Remove:
//   Deps    []string
//   DepMode string
// Add:
DepGroups []DepGroupDisplay
```

Where:
```go
type DepGroupDisplay struct {
	Names []string
	Mode  string
}
```

2. Update node building (command.go:1615-1627):
```go
if execTarget, ok := target.(TargetExecutionLike); ok {
	for _, g := range execTarget.GetDepGroups() {
		var names []string
		for _, d := range g.Targets {
			names = append(names, d.GetName())
		}
		node.DepGroups = append(node.DepGroups, DepGroupDisplay{
			Names: names,
			Mode:  g.Mode.String(),
		})
	}
	// ...
}
```

3. Update `appendDepsLine`:
```go
func appendDepsLine(lines []string, node *commandNode) []string {
	if len(node.DepGroups) == 0 {
		return lines
	}

	// Single group — use original format for backward compatibility
	if len(node.DepGroups) == 1 {
		g := node.DepGroups[0]
		mode := g.Mode
		if mode == "" {
			mode = DepModeSerial.String()
		}
		return append(lines, fmt.Sprintf("Deps: %s (%s)", strings.Join(g.Names, ", "), mode))
	}

	// Multiple groups — use arrow separator
	var parts []string
	for _, g := range node.DepGroups {
		part := strings.Join(g.Names, ", ")
		if g.Mode == DepModeParallel.String() {
			part += " (parallel)"
		}
		parts = append(parts, part)
	}

	return append(lines, "Deps: "+strings.Join(parts, " → "))
}
```

4. Update all other references to `node.Deps` and `node.DepMode` in command.go. Search for these field names and fix each occurrence.

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/core/ -run <test_name> -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS

**Step 6: Commit**

```
feat(help): display chained dep groups with arrow separators
```

---

### Task 4: Apply `--dep-mode` override at runtime

**Files:**
- Modify: `internal/core/command.go:1907-1920` (runTargetWithOverrides)
- Modify: `internal/core/override.go:24` (RuntimeOverrides.DepMode)

**Step 1: Write the failing test**

Add to `test/overrides_properties_test.go` near the existing DepMode tests (around line 430):

```go
t.Run("DepModeOverrideFlattensMixedGroups", func(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var mu sync.Mutex
	var order []string

	a := targ.Targ(func() {
		mu.Lock()
		order = append(order, "a")
		mu.Unlock()
	}).Name("a")

	b := targ.Targ(func() {
		mu.Lock()
		order = append(order, "b")
		mu.Unlock()
	}).Name("b")

	// Normally parallel, but --dep-mode serial should make them serial
	main := targ.Targ(func() {
		mu.Lock()
		order = append(order, "main")
		mu.Unlock()
	}).Name("main").Deps(a, b, targ.DepModeParallel)

	// Run with --dep-mode serial override
	// (Use the CLI integration test pattern from existing override tests)
})
```

Follow the patterns in the existing override tests to invoke with `--dep-mode serial` and verify serial execution order.

**Step 2: Run test to verify it fails**

Expected: FAIL — `--dep-mode` override is not applied at runtime.

**Step 3: Write minimal implementation**

In `runTargetWithOverrides`, before `node.Target.runDeps(ctx)`, check and apply the override:

```go
if node.Target != nil && len(node.Target.depGroups) > 0 {
	target := node.Target

	// Apply --dep-mode override: flatten all groups into one
	if opts.Overrides.DepMode != "" {
		var mode DepMode
		if opts.Overrides.DepMode == "parallel" {
			mode = DepModeParallel
		}
		allDeps := target.GetDeps()
		if len(allDeps) > 0 {
			target.depGroups = []depGroup{{targets: allDeps, mode: mode}}
		}
	}

	err := target.runDeps(ctx)
	if err != nil {
		return err
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./test/ -run "DepModeOverride" -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS

**Step 6: Commit**

```
feat(override): apply --dep-mode override at runtime
```

---

### Task 5: Update runner code generation

**Files:**
- Modify: `internal/runner/runner.go:2128-2144` (deps code gen)

**Step 1: Write the failing test**

Add to `internal/runner/runner_properties_test.go` near the existing `DepModeCodeGen` test (around line 934):

```go
t.Run("DepModeCodeGenChained", func(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test that codegen with chained deps produces multiple .Deps() calls
	// Check the createTargetOpts and formatDepsCall for this case
	// (Follow the pattern of the existing DepModeCodeGen test)
})
```

The codegen currently produces `.Deps(A, B, targ.DepModeParallel)` for a single group. For chained groups, it should produce `.Deps(A).Deps(B, C, targ.DepModeParallel).Deps(D)`.

Note: The runner's `createTargetOpts` struct currently has a single `DepMode string` field. This may need to change to support chained groups in the `targ create` CLI. However, since `targ create` creates simple targets, it may be acceptable to keep it single-group for now. Evaluate whether this needs a full rewrite or just awareness of the new structure.

**Step 2-4:** Standard TDD red/green cycle.

**Step 5: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS

**Step 6: Commit**

```
feat(runner): update code gen for chained dep groups
```

---

### Task 6: Update docs and existing tests

**Files:**
- Modify: `README.md:207,338,486` (dep mode documentation)
- Modify: `docs/design.md` (if dep mode is referenced)
- Modify: `docs/architecture.md` (dep mode references)
- Modify: `docs/requirements.md` (REQ-005)
- Verify: All existing tests pass without modification (should be backward compatible)

**Step 1: Update README examples**

Add chaining examples alongside existing ones:
```go
targ.Targ(ci).Deps(generate).Deps(lint, test, targ.DepModeParallel).Deps(deploy)
```

**Step 2: Update architecture/design/requirements docs**

Add chaining to the feature table and architecture description.

**Step 3: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS

**Step 4: Commit**

```
docs: add dep group chaining documentation
```

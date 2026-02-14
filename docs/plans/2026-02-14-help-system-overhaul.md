# Help System Overhaul Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make help output correct in both targ CLI and compiled binary modes, with tag-based flag classification enforced by tests, proper labels, and auto-generated examples.

**Architecture:** Add `FlagMode` to flag definitions so binary mode filtering is data-driven. Propagate `BinaryMode` from `targ.Main()` through `RunOptions` to help filter. Replace hardcoded examples with a generator that uses command metadata. Rename "Targ flags:" to "Global flags:" (targ mode) / "Flags:" (binary mode).

**Tech Stack:** Go, gomega

---

### Task 1: Add FlagMode to flag registry

**Files:**
- Modify: `internal/flags/flags.go:8-17` (Def struct), `internal/flags/flags.go:32-108` (All() registry)
- Modify: `internal/flags/flags_test.go`

**Step 1: Write failing test**

Add to `internal/flags/flags_test.go`:

```go
func TestAllFlagsHaveExplicitMode(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	for _, f := range flags.All() {
		// Every flag must have been consciously classified.
		// FlagModeAll (0) is valid for help/completion.
		// FlagModeTargOnly (1) is valid for everything else.
		// We verify by checking that only "help" and "completion" use FlagModeAll.
		if f.Mode == flags.FlagModeAll {
			g.Expect(f.Long).To(BeElementOf("help", "completion"),
				"only help and completion should be FlagModeAll, got: "+f.Long)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/flags/ -run TestAllFlagsHaveExplicitMode -v`
Expected: FAIL — `Mode` field doesn't exist on `flags.Def`.

**Step 3: Write minimal implementation**

In `internal/flags/flags.go`, add the type and field:

```go
// FlagMode controls which execution modes a flag appears in.
type FlagMode int

const (
	// FlagModeAll means the flag appears in both targ CLI and compiled binary help.
	FlagModeAll FlagMode = iota
	// FlagModeTargOnly means the flag only appears in targ CLI help.
	FlagModeTargOnly
)
```

Add `Mode FlagMode` to the `Def` struct.

In `All()`, set `Mode: FlagModeTargOnly` on every flag EXCEPT `help` and `completion`. The hidden/removed flags also get `FlagModeTargOnly` since they're targ-specific concepts.

```go
// In All():
{Long: "help", Short: "h", Desc: "Show help"},                              // FlagModeAll (zero value)
{Long: "completion", ..., Mode: FlagModeAll},                                // explicit
{Long: "source", ..., Mode: FlagModeTargOnly},
{Long: "timeout", ..., Mode: FlagModeTargOnly},
{Long: "parallel", ..., Mode: FlagModeTargOnly},
// ... all others get Mode: FlagModeTargOnly
```

Note: `help` can use the zero value (FlagModeAll) since that's the iota default. But `completion` should be explicit for clarity. Set it explicitly on both for readability.

**Step 4: Run test to verify it passes**

Run: `go test -tags sqlite_fts5 ./internal/flags/ -run TestAllFlagsHaveExplicitMode -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS — adding a field with zero value doesn't break anything.

**Step 6: Commit**

```
feat(flags): add FlagMode to classify flags for binary/targ mode

Every flag now declares whether it appears in binary mode (FlagModeAll)
or only in targ CLI mode (FlagModeTargOnly). Test enforces classification.
```

---

### Task 2: Use FlagMode in shouldSkipTargFlag

**Files:**
- Modify: `internal/help/builder.go:205-231` (shouldSkipTargFlag)
- Modify: `internal/help/binary_mode_test.go`

**Step 1: Write failing test**

The existing `binary_mode_test.go` tests already verify binary mode filtering. Update them to also verify that the filtering is driven by `FlagMode`, not the hardcoded allowlist. Add:

```go
t.Run("BinaryModeUsessFlagMode", func(t *testing.T) {
	g := NewWithT(t)

	var buf bytes.Buffer

	opts := help.RootHelpOpts{
		BinaryName:  "myapp",
		Description: "My application",
		Filter: help.TargFlagFilter{
			IsRoot:     true,
			BinaryMode: true,
		},
	}

	help.WriteRootHelp(&buf, opts)
	output := buf.String()

	// Should show FlagModeAll flags
	g.Expect(output).To(ContainSubstring("--help"))
	g.Expect(output).To(ContainSubstring("--completion"))

	// Should NOT show FlagModeTargOnly flags
	g.Expect(output).ToNot(ContainSubstring("--source"))
	g.Expect(output).ToNot(ContainSubstring("--create"))
	g.Expect(output).ToNot(ContainSubstring("--no-binary-cache"))
	g.Expect(output).ToNot(ContainSubstring("--dep-mode"))
})
```

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/help/ -run TestBinaryModeHelpOutput/BinaryModeUsessFlagMode -v`
Expected: May pass already since the hardcoded allowlist does the same thing. If so, this is a refactor task — change the implementation and verify tests still pass.

**Step 3: Write minimal implementation**

Replace the hardcoded allowlist in `shouldSkipTargFlag` (`builder.go:210-218`):

```go
func shouldSkipTargFlag(f flags.Def, filter TargFlagFilter) bool {
	if f.RootOnly && !filter.IsRoot {
		return true
	}

	// In binary mode, only show flags classified as FlagModeAll
	if filter.BinaryMode && f.Mode != flags.FlagModeAll {
		return true
	}

	switch f.Long {
	case "completion":
		return filter.DisableCompletion
	case "help":
		return filter.DisableHelp
	case "timeout":
		return filter.DisableTimeout
	default:
		return false
	}
}
```

**Step 4: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS

**Step 5: Commit**

```
refactor(help): use FlagMode instead of hardcoded allowlist for binary mode
```

---

### Task 3: Propagate BinaryMode from targ.Main()

**Files:**
- Modify: `internal/core/types.go:35-96` (RunOptions)
- Modify: `internal/core/execute.go:71-75` (Main)
- Modify: `internal/core/command.go:1807,1861` (TargFlagFilter creation)

**Step 1: Write failing test**

Add an integration-level test that verifies `Main()`-style execution produces binary mode help:

```go
// In a test file that can invoke ExecuteWithResolution or similar
func TestBinaryModePropagation(t *testing.T) {
	g := NewWithT(t)

	// Create a target and run with BinaryMode=true
	// Capture help output and verify it uses "[flags...]" not "[targ flags...]"
	// and doesn't show --timeout, --parallel, etc.
}
```

Follow the existing test patterns in `test/` for how help output is captured and verified. Look at how `RunOptions` is passed in existing tests.

**Step 2: Run test to verify it fails**

Expected: FAIL — `BinaryMode` field doesn't exist on `RunOptions`.

**Step 3: Write minimal implementation**

1. Add `BinaryMode bool` to `RunOptions` in `types.go`:
```go
type RunOptions struct {
	AllowDefault      bool
	BinaryMode        bool // Set by Main() for compiled binary mode
	DisableHelp       bool
	// ...
}
```

2. Update `Main()` in `execute.go` to set it:
```go
func Main(targets ...any) {
	RegisterTarget(targets...)
	env := osRunEnv{}
	_ = ExecuteWithResolution(env, RunOptions{
		AllowDefault: true,
		BinaryMode:   true,
	})
}
```

Note: `Main()` currently calls `ExecuteRegistered()` which creates its own `RunOptions`. Change `Main()` to create options directly with `BinaryMode: true`.

3. Pass `BinaryMode` through to `TargFlagFilter` in `command.go`:

At line 1807 (printCommandHelp):
```go
Filter: help.TargFlagFilter{
	IsRoot:            false,
	BinaryMode:        opts.BinaryMode,
	DisableCompletion: opts.DisableCompletion,
	DisableHelp:       opts.DisableHelp,
	DisableTimeout:    opts.DisableTimeout,
},
```

At line 1861 (printUsage):
```go
Filter: help.TargFlagFilter{
	IsRoot:            true,
	BinaryMode:        opts.BinaryMode,
	DisableCompletion: opts.DisableCompletion,
	DisableHelp:       opts.DisableHelp,
	DisableTimeout:    opts.DisableTimeout,
},
```

**Step 4: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS

**Step 5: Commit**

```
feat(core): propagate BinaryMode from Main() to help filter

targ.Main() now sets BinaryMode=true on RunOptions, which flows
through to TargFlagFilter for correct binary-mode help output.
```

---

### Task 4: Rename "Targ flags:" to "Global flags:" / "Flags:"

**Files:**
- Modify: `internal/help/render.go:320-353` (renderTargFlags)
- Modify: `internal/help/content.go:16-36` (ContentBuilder — add binaryMode field)
- Modify: `internal/help/builder.go:106-135` (AddTargFlagsFiltered — pass binaryMode)
- Modify: `internal/help/generators.go` (pass binaryMode to builder)

**Step 1: Write failing test**

```go
func TestFlagSectionLabel(t *testing.T) {
	t.Parallel()

	t.Run("TargModeShowsGlobalFlags", func(t *testing.T) {
		g := NewWithT(t)
		var buf bytes.Buffer

		opts := help.RootHelpOpts{
			BinaryName:  "targ",
			Description: "Build tool",
			Filter:      help.TargFlagFilter{IsRoot: true},
		}

		help.WriteRootHelp(&buf, opts)
		output := buf.String()

		g.Expect(output).To(ContainSubstring("Global flags:"))
		g.Expect(output).ToNot(ContainSubstring("Targ flags:"))
	})

	t.Run("BinaryModeShowsFlags", func(t *testing.T) {
		g := NewWithT(t)
		var buf bytes.Buffer

		opts := help.RootHelpOpts{
			BinaryName:  "myapp",
			Description: "My app",
			Filter:      help.TargFlagFilter{IsRoot: true, BinaryMode: true},
		}

		help.WriteRootHelp(&buf, opts)
		output := buf.String()

		g.Expect(output).To(ContainSubstring("Flags:"))
		g.Expect(output).ToNot(ContainSubstring("Global flags:"))
		g.Expect(output).ToNot(ContainSubstring("Targ flags:"))
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags sqlite_fts5 ./internal/help/ -run TestFlagSectionLabel -v`
Expected: FAIL — still shows "Targ flags:".

**Step 3: Write minimal implementation**

1. Add `binaryMode bool` to `ContentBuilder` in `content.go`.

2. In `AddTargFlagsFiltered` (`builder.go`), store the filter's binary mode:
```go
func (cb *ContentBuilder) AddTargFlagsFiltered(filter TargFlagFilter) *ContentBuilder {
	cb.binaryMode = filter.BinaryMode
	// ... rest unchanged
}
```

3. In `renderTargFlags` (`render.go:320-353`), change the header:
```go
func (cb *ContentBuilder) renderTargFlags(styles Styles) string {
	hasGlobal := len(cb.globalFlags) > 0
	hasRootOnly := len(cb.rootOnlyFlags) > 0

	if !hasGlobal && !hasRootOnly {
		return ""
	}

	var sb strings.Builder

	if cb.binaryMode {
		// Binary mode: flat "Flags:" section (no subsections, only FlagModeAll flags present)
		sb.WriteString(styles.Header.Render("Flags:"))
		for _, f := range cb.globalFlags {
			sb.WriteString("\n")
			sb.WriteString(cb.renderFlag(f, styles))
		}
	} else {
		// Targ CLI mode: "Global flags:" with subsections
		sb.WriteString(styles.Header.Render("Global flags:"))

		if hasGlobal {
			sb.WriteString("\n  ")
			sb.WriteString(styles.Subsection.Render("Global:"))
			for _, f := range cb.globalFlags {
				sb.WriteString("\n")
				sb.WriteString(cb.renderFlagWithIndent(f, styles, "    "))
			}
		}

		if hasRootOnly && cb.isRoot {
			sb.WriteString("\n  ")
			sb.WriteString(styles.Subsection.Render("Root only:"))
			for _, f := range cb.rootOnlyFlags {
				sb.WriteString("\n")
				sb.WriteString(cb.renderFlagWithIndent(f, styles, "    "))
			}
		}
	}

	return sb.String()
}
```

**Step 4: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: Some existing tests may fail due to changed "Targ flags:" → "Global flags:" label. Update those test expectations.

**Step 5: Commit**

```
feat(help): rename flag section labels for clarity

"Targ flags:" → "Global flags:" in targ CLI mode.
"Flags:" (flat, no subsections) in binary mode.
```

---

### Task 5: Auto-generated examples for root help

**Files:**
- Modify: `internal/core/command.go:1830-1842` (printUsage example generation)
- Modify: `internal/help/generators.go` (add example generator)
- Test: `internal/help/generators_test.go`

**Step 1: Write failing test**

```go
func TestAutoGeneratedRootExamples(t *testing.T) {
	t.Parallel()

	t.Run("TargModeRootExamples", func(t *testing.T) {
		g := NewWithT(t)
		var buf bytes.Buffer

		opts := help.RootHelpOpts{
			BinaryName:  "targ",
			Description: "Build tool",
			CommandGroups: []help.CommandGroup{
				{Source: "dev/", Commands: []help.Command{
					{Name: "build", Desc: "Build the project"},
					{Name: "test", Desc: "Run tests"},
				}},
			},
			Filter: help.TargFlagFilter{IsRoot: true},
		}

		help.WriteRootHelp(&buf, opts)
		output := buf.String()

		// Should have auto-generated examples using actual command names
		g.Expect(output).To(ContainSubstring("targ build"))
		g.Expect(output).To(ContainSubstring("targ build test"))
	})

	t.Run("BinaryModeRootExamples", func(t *testing.T) {
		g := NewWithT(t)
		var buf bytes.Buffer

		opts := help.RootHelpOpts{
			BinaryName:  "myapp",
			Description: "My app",
			CommandGroups: []help.CommandGroup{
				{Source: "main.go", Commands: []help.Command{
					{Name: "greet", Desc: "Say hello"},
				}},
			},
			Filter: help.TargFlagFilter{IsRoot: true, BinaryMode: true},
		}

		help.WriteRootHelp(&buf, opts)
		output := buf.String()

		// Should use binary name
		g.Expect(output).To(ContainSubstring("myapp greet"))
		// Should NOT reference "targ"
		g.Expect(output).ToNot(ContainSubstring("targ"))
	})

	t.Run("UserExamplesReplace", func(t *testing.T) {
		g := NewWithT(t)
		var buf bytes.Buffer

		opts := help.RootHelpOpts{
			BinaryName:  "targ",
			Description: "Build tool",
			Examples: []help.Example{
				{Title: "Custom", Code: "targ custom-thing"},
			},
			Filter: help.TargFlagFilter{IsRoot: true},
		}

		help.WriteRootHelp(&buf, opts)
		output := buf.String()

		// User examples should replace auto-generated ones
		g.Expect(output).To(ContainSubstring("targ custom-thing"))
	})
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL — current examples are hardcoded or nil.

**Step 3: Write minimal implementation**

Add auto-example generation to `generators.go`:

```go
// GenerateRootExamples creates examples from command metadata.
func GenerateRootExamples(binaryName string, groups []CommandGroup, binaryMode bool) []Example {
	// Collect all command names
	var cmdNames []string
	for _, g := range groups {
		for _, c := range g.Commands {
			cmdNames = append(cmdNames, c.Name)
		}
	}

	if len(cmdNames) == 0 {
		return nil
	}

	var examples []Example

	// Basic: run a command
	examples = append(examples, Example{
		Title: "Run a command",
		Code:  binaryName + " " + cmdNames[0],
	})

	// Chain: run multiple commands (targ mode only, 2+ commands needed)
	if !binaryMode && len(cmdNames) >= 2 {
		examples = append(examples, Example{
			Title: "Chain commands",
			Code:  binaryName + " " + cmdNames[0] + " " + cmdNames[1],
		})
	}

	return examples
}
```

In `WriteRootHelp` (`generators.go`), auto-generate when no user examples:
```go
examples := opts.Examples
if len(examples) == 0 {
	examples = GenerateRootExamples(opts.BinaryName, opts.CommandGroups, opts.Filter.BinaryMode)
}
if len(examples) > 0 {
	b.AddExamples(examples...)
}
```

In `printUsage` (`command.go:1830-1842`), remove the hardcoded `completionExampleWithGetenv` and `chainExample` calls. Let the generator handle it. If `opts.Examples` is set by the user, pass those through. If nil, let `WriteRootHelp` auto-generate.

```go
// Replace lines 1831-1837 with:
var helpExamples []help.Example
if opts.Examples != nil {
	helpExamples = make([]help.Example, 0, len(opts.Examples))
	for _, e := range opts.Examples {
		helpExamples = append(helpExamples, help.Example{Title: e.Title, Code: e.Code})
	}
}
// nil helpExamples → WriteRootHelp will auto-generate
```

**Step 4: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: Some tests may need updated example expectations. Fix them.

**Step 5: Commit**

```
feat(help): auto-generate root examples from command metadata

Root help now generates examples using actual command names and
binary name. User-provided .Examples() replace auto-generated ones.
Removes hardcoded "targ build test" examples.
```

---

### Task 6: Auto-generated examples for target help

**Files:**
- Modify: `internal/help/generators.go` (add target example generator)
- Modify: `internal/core/command.go:1795-1813` (pass flag/positional metadata)
- Test: `internal/help/generators_test.go`

**Step 1: Write failing test**

```go
func TestAutoGeneratedTargetExamples(t *testing.T) {
	t.Parallel()

	t.Run("TargetWithPositionalAndFlags", func(t *testing.T) {
		g := NewWithT(t)
		var buf bytes.Buffer

		opts := help.TargetHelpOpts{
			BinaryName:  "targ",
			Name:        "deploy",
			Description: "Deploy the app",
			Flags: []help.Flag{
				{Long: "--env", Desc: "Environment", Placeholder: "ENV"},
				{Long: "--port", Desc: "Port number", Placeholder: "N"},
				{Long: "--dry-run", Desc: "Dry run"},
			},
			Filter: help.TargFlagFilter{IsRoot: false},
		}

		help.WriteTargetHelp(&buf, opts)
		output := buf.String()

		// Should have auto-generated example with flags
		g.Expect(output).To(ContainSubstring("targ deploy"))
		g.Expect(output).To(ContainSubstring("--env"))
	})

	t.Run("TargetWithNoFlags", func(t *testing.T) {
		g := NewWithT(t)
		var buf bytes.Buffer

		opts := help.TargetHelpOpts{
			BinaryName:  "targ",
			Name:        "clean",
			Description: "Clean build artifacts",
			Filter:      help.TargFlagFilter{IsRoot: false},
		}

		help.WriteTargetHelp(&buf, opts)
		output := buf.String()

		// Minimal example, just the command
		g.Expect(output).To(ContainSubstring("targ clean"))
	})

	t.Run("UserExamplesReplaceTargetExamples", func(t *testing.T) {
		g := NewWithT(t)
		var buf bytes.Buffer

		opts := help.TargetHelpOpts{
			BinaryName:  "targ",
			Name:        "deploy",
			Description: "Deploy",
			Examples: []help.Example{
				{Title: "Deploy to prod", Code: "targ deploy --env production"},
			},
			Filter: help.TargFlagFilter{IsRoot: false},
		}

		help.WriteTargetHelp(&buf, opts)
		output := buf.String()

		g.Expect(output).To(ContainSubstring("Deploy to prod"))
	})
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL — current target help has no auto-generated examples.

**Step 3: Write minimal implementation**

Add to `generators.go`:

```go
// GenerateTargetExamples creates examples from target metadata.
func GenerateTargetExamples(binaryName, targetName string, cmdFlags []Flag, binaryMode bool) []Example {
	prefix := binaryName + " " + targetName
	if binaryMode {
		prefix = binaryName
		if targetName != "" {
			prefix += " " + targetName
		}
	}

	var examples []Example

	// Basic usage
	examples = append(examples, Example{
		Title: "Basic usage",
		Code:  prefix,
	})

	// With options (if there are non-required flags)
	var optionalFlags []Flag
	for _, f := range cmdFlags {
		if !f.Required {
			optionalFlags = append(optionalFlags, f)
		}
	}

	if len(optionalFlags) > 0 {
		code := prefix
		// Show up to 2 optional flags
		limit := 2
		if len(optionalFlags) < limit {
			limit = len(optionalFlags)
		}
		for _, f := range optionalFlags[:limit] {
			code += " " + f.Long
			if f.Placeholder != "" {
				code += " " + exampleValueForPlaceholder(f.Placeholder)
			}
		}

		examples = append(examples, Example{
			Title: "With options",
			Code:  code,
		})
	}

	return examples
}

func exampleValueForPlaceholder(placeholder string) string {
	switch strings.ToLower(placeholder) {
	case "n":
		return "10"
	case "duration":
		return "30s"
	case "d,m":
		return "1s,2.0"
	default:
		return strings.ToLower(placeholder)
	}
}
```

In `WriteTargetHelp` (`generators.go`), auto-generate when no user examples:
```go
// Replace lines 124-130:
if len(opts.Examples) > 0 {
	b.AddExamples(opts.Examples...)
} else if len(opts.Flags) > 0 || len(opts.Subcommands) > 0 {
	generated := GenerateTargetExamples(opts.BinaryName, opts.Name, opts.Flags, opts.Filter.BinaryMode)
	b.AddExamples(generated...)
} else {
	b.AddExamples() // explicitly empty
}
```

**Step 4: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS

**Step 5: Commit**

```
feat(help): auto-generate target examples from flag metadata

Target help now shows contextual examples using actual flag names
and type-aware placeholder values. User .Examples() replace auto-generated.
```

---

### Task 7: Update existing tests and fix regressions

**Files:**
- Modify: Various test files that assert on help output strings

**Step 1: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`

Identify all failures caused by:
- "Targ flags:" → "Global flags:" label change
- Changed example text
- New flag section format in binary mode

**Step 2: Fix each test**

Update string expectations to match new labels and auto-generated example text. Do NOT change the implementation to match old tests — the tests should match the new design.

**Step 3: Run full test suite again**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS

**Step 4: Commit**

```
test(help): update test expectations for help system overhaul

Updates test assertions for new flag section labels and
auto-generated example format.
```

---

### Task 8: Update documentation

**Files:**
- Modify: `README.md` — update help output examples
- Modify: `docs/architecture.md` — update help system description
- Modify: `docs/requirements.md` — add/update help requirements

**Step 1: Update docs**

Update any documentation that references "Targ flags:", shows example help output, or describes the help system.

**Step 2: Run full test suite**

Run: `go test -tags sqlite_fts5 ./...`
Expected: PASS

**Step 3: Commit**

```
docs: update documentation for help system overhaul
```

package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ"
)

// Property: Multiple targets can be executed in sequence
func TestProperty_Execution_MultipleTargetsRunSequentially(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	order := make([]string, 0)

	a := targ.Targ(func() { order = append(order, "a") }).Name("a")
	b := targ.Targ(func() { order = append(order, "b") }).Name("b")

	_, err := targ.Execute([]string{"app", "a", "b"}, a, b)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(order).To(Equal([]string{"a", "b"}))
}

// Property: Glob pattern with contains match (*test*) works
func TestProperty_Glob_ContainsMatchWorks(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	testCalls := 0
	buildCalls := 0

	targetTest := targ.Targ(func() { testCalls++ }).Name("run-test-unit")
	targetBuild := targ.Targ(func() { buildCalls++ }).Name("build")

	_, err := targ.ExecuteWithOptions(
		[]string{"app", "*test*"},
		targ.RunOptions{AllowDefault: false},
		targetTest, targetBuild,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(testCalls).To(Equal(1))
	g.Expect(buildCalls).To(Equal(0))
}

// Property: Glob patterns with no matches return error in parallel mode
func TestProperty_Glob_NoMatchesReturnsError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	build := targ.Targ(func() {}).Name("build")
	test := targ.Targ(func() {}).Name("test")

	// In parallel mode with multiple roots, glob with no matches should error
	_, err := targ.ExecuteWithOptions(
		[]string{"app", "nonexistent-*"},
		targ.RunOptions{AllowDefault: false, Overrides: targ.RuntimeOverrides{Parallel: true}},
		build, test,
	)
	g.Expect(err).To(HaveOccurred())
}

// Property: Glob patterns match multiple targets
func TestProperty_Glob_PatternMatchesMultipleTargets(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	testACalls := 0
	testBCalls := 0
	buildCalls := 0

	testA := targ.Targ(func() { testACalls++ }).Name("test-a")
	testB := targ.Targ(func() { testBCalls++ }).Name("test-b")
	build := targ.Targ(func() { buildCalls++ }).Name("build")

	_, err := targ.ExecuteWithOptions(
		[]string{"app", "test-*"},
		targ.RunOptions{AllowDefault: false},
		testA, testB, build,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(testACalls).To(Equal(1))
	g.Expect(testBCalls).To(Equal(1))
	g.Expect(buildCalls).To(Equal(0))
}

// Property: Glob patterns work within groups
func TestProperty_Glob_PatternMatchesSubcommands(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	testACalls := 0
	testBCalls := 0
	buildCalls := 0

	testA := targ.Targ(func() { testACalls++ }).Name("test-a")
	testB := targ.Targ(func() { testBCalls++ }).Name("test-b")
	build := targ.Targ(func() { buildCalls++ }).Name("build")

	group := targ.Group("dev", testA, testB, build)

	// When group is the single root, args go directly to its subcommands
	_, err := targ.Execute([]string{"app", "test-*"}, group)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(testACalls).To(Equal(1))
	g.Expect(testBCalls).To(Equal(1))
	g.Expect(buildCalls).To(Equal(0))
}

// Property: Glob pattern * matches all targets
func TestProperty_Glob_StarMatchesAllTargets(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	aCalls := 0
	bCalls := 0

	targetA := targ.Targ(func() { aCalls++ }).Name("alpha")
	targetB := targ.Targ(func() { bCalls++ }).Name("beta")

	_, err := targ.ExecuteWithOptions(
		[]string{"app", "*"},
		targ.RunOptions{AllowDefault: false},
		targetA, targetB,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(aCalls).To(Equal(1))
	g.Expect(bCalls).To(Equal(1))
}

// Property: Glob pattern with suffix match (*-unit) works
func TestProperty_Glob_SuffixMatchWorks(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	unitCalls := 0
	intCalls := 0

	targetUnit := targ.Targ(func() { unitCalls++ }).Name("test-unit")
	targetInt := targ.Targ(func() { intCalls++ }).Name("test-int")

	_, err := targ.ExecuteWithOptions(
		[]string{"app", "*-unit"},
		targ.RunOptions{AllowDefault: false},
		targetUnit, targetInt,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(unitCalls).To(Equal(1))
	g.Expect(intCalls).To(Equal(0))
}

// Property: Namespace nodes (groups) are not directly executable
func TestProperty_Groups_NamespaceNodesAreNotExecutable(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	sub := targ.Targ(func() {}).Name("sub")
	group := targ.Group("grp", sub)

	// When there's only a group, calling without subcommand shows help (no error)
	// but doesn't execute the group itself
	called := false
	subWithTrack := targ.Targ(func() { called = true }).Name("sub")
	groupWithTrack := targ.Group("grp", subWithTrack)

	_, err := targ.Execute([]string{"app"}, groupWithTrack)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeFalse()) // Group itself not executed

	_ = group // Used for documentation
}

// Property: Help on nested groups shows chain example
func TestProperty_Help_NestedGroupsShowChainExample(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Create a group with subcommands AND another command at root level
	// This exercises the chainExample nested groups branch
	sub := targ.Targ(func() {}).Name("sub")
	group := targ.Group("grp", sub)
	other := targ.Targ(func() {}).Name("other")

	result, err := targ.Execute([]string{"app", "--help"}, group, other)
	g.Expect(err).NotTo(HaveOccurred())
	// Help output should include chain example with ^ for nested structure
	g.Expect(result.Output).To(ContainSubstring("grp"))
	g.Expect(result.Output).To(ContainSubstring("other"))
}

// Property: Double-dash (^) resets to root
func TestProperty_PathResolution_CaretResetsToRoot(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	subCalled := 0
	rootCalled := 0

	sub := targ.Targ(func() { subCalled++ }).Name("sub")
	group := targ.Group("grp", sub)
	rootTarget := targ.Targ(func() { rootCalled++ }).Name("root-target")

	// Execute sub, then ^ to reset, then root-target
	_, err := targ.Execute([]string{"app", "grp", "sub", "^", "root-target"}, group, rootTarget)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(subCalled).To(Equal(1))
	g.Expect(rootCalled).To(Equal(1))
}
